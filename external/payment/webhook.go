package payment

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"

	"donetick.com/core/config"
	"donetick.com/core/external/payment/model"
	pDB "donetick.com/core/external/payment/repo"
	stripeService "donetick.com/core/external/payment/service"
	uRepo "donetick.com/core/internal/user/repo"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
)

type Webhook struct {
	stripeDB         pDB.StripeDB
	revenueCatDB     pDB.RevenueCatDB
	subscriptionDB   pDB.SubscriptionDB
	whitelistIPs     map[string]bool
	stripe           *stripeService.StripeService
	prices           []config.StripePrices
	revenueCatConfig config.RevenueCatConfig
	userRepo         *uRepo.UserRepository
}

func NewWebhook(stripeDB pDB.StripeDB,
	revenueCatDB pDB.RevenueCatDB,
	subscriptionDB pDB.SubscriptionDB,
	stripeService *stripeService.StripeService,
	uRepo *uRepo.UserRepository,
	config *config.Config) *Webhook {

	whitelistIPs := make(map[string]bool)
	for _, ip := range config.StripeConfig.WhitelistedIPs {
		whitelistIPs[ip] = true
	}

	return &Webhook{
		stripeDB:         stripeDB,
		revenueCatDB:     revenueCatDB,
		subscriptionDB:   subscriptionDB,
		whitelistIPs:     whitelistIPs,
		stripe:           stripeService,
		userRepo:         uRepo,
		prices:           config.StripeConfig.Prices,
		revenueCatConfig: config.RevenueCatConfig,
	}
}

// webhook handler
func (h *Webhook) isIPWhitelisted(ip string) bool {
	_, ok := h.whitelistIPs[ip]
	return ok
}

// stripeSubscriptionExpiry reads the paid-through date from Stripe. Webhook
// payloads don't expand the subscription, and the plan interval isn't knowable
// from the event, so the period end has to come from the API rather than being
// derived by adding a fixed interval.
func (h *Webhook) stripeSubscriptionExpiry(c *gin.Context, subscriptionID string) (time.Time, error) {
	logger := logging.FromContext(c)

	sub, err := h.stripe.GetSubscription(subscriptionID)
	if err != nil {
		logger.Errorw("payment.webhook.stripeSubscriptionExpiry failed to get subscription",
			"subscription_id", subscriptionID, "err", err)
		return time.Time{}, err
	}

	if sub.CurrentPeriodEnd == 0 {
		logger.Errorw("payment.webhook.stripeSubscriptionExpiry subscription has no current period end",
			"subscription_id", subscriptionID)
		return time.Time{}, errors.New("stripe subscription has no current_period_end")
	}

	return time.Unix(sub.CurrentPeriodEnd, 0).UTC(), nil
}

func (h *Webhook) StripeWebhook(c *gin.Context) {

	logger := logging.FromContext(c)

	ip := c.ClientIP()
	if !h.isIPWhitelisted(ip) {
		logger.Errorw("payment.webhook.Webhook failed to validate ip", "ip", ip)
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}

	event := stripe.Event{}
	if err := c.BindJSON(&event); err != nil {
		logger.Errorw("payment.webhook.Webhook failed to bind json", "err", err)
		return
	}
	switch event.Type {
	case "checkout.session.completed":
		// get the subscription id from the event:

		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			logger.Errorw("payment.webhook.Webhook failed to unmarshal session", "err", err)
			c.JSON(http.StatusInternalServerError, "Internal Server Error")
			return
		}

		if s.PaymentStatus != "paid" || s.Status != "complete" {
			logger.Errorw("payment.webhook.Webhook failed to get payment status", "event", event.Type)
			c.JSON(http.StatusOK, nil)

			return
		}

		if s.Subscription == nil || s.Subscription.ID == "" {
			logger.Errorw("payment.webhook.Webhook failed to get subscription id", "event", event.Type)
			c.JSON(http.StatusOK, nil)
			return
		}

		if s.Customer == nil || s.Customer.ID == "" {
			logger.Errorw("payment.webhook.Webhook session has no customer", "event", event.Type)
			c.JSON(http.StatusOK, nil)
			return
		}

		// Get user ID from stripe customer
		customer, err := h.stripeDB.GetCustomerByCustomerID(c, s.Customer.ID)
		if err != nil {
			logger.Errorw("payment.webhook.Webhook failed to get customer", "customer_id", s.Customer.ID, "err", err)
			return
		}

		expiredAt, err := h.stripeSubscriptionExpiry(c, s.Subscription.ID)
		if err != nil {
			// Returning an error lets Stripe retry rather than storing a guessed
			// expiration.
			c.JSON(http.StatusInternalServerError, "Internal Server Error")
			return
		}

		// Save to legacy table for backward compatibility
		h.stripeDB.SaveSubscription(c, &model.StripeSubscription{
			SubscriptionID: s.Subscription.ID,
			CustomerID:     s.Customer.ID,
			Status:         "active",
			ExpiredAt:      &expiredAt,
		})

		// Save to unified subscription table
		if customer != nil {
			unifiedSub := &model.Subscription{
				ID:                     s.Subscription.ID,
				UserID:                 int(customer.UserID),
				CircleID:               customer.CircleID,
				Provider:               model.SubscriptionProviderStripe,
				ExternalSubscriptionID: s.Subscription.ID,
				ExternalCustomerID:     s.Customer.ID,
				Status:                 "active",
				ExpiresAt:              &expiredAt,
				CreatedAt:              time.Now().UTC(),
				UpdatedAt:              time.Now().UTC(),
			}
			h.subscriptionDB.SaveSubscription(c, unifiedSub)
		}

		h.stripeDB.UpdateSession(c, event.ID, string(s.Status))

	case "invoice.payment_succeeded":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			logger.Errorw("payment.webhook.Webhook failed to unmarshal invoice", "err", err)
			c.JSON(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		// One-off invoices carry no subscription and don't affect entitlements.
		if inv.Subscription == nil || inv.Subscription.ID == "" {
			logger.Debugw("payment.webhook.Webhook ignoring non-subscription invoice", "invoice_id", inv.ID)
			c.JSON(http.StatusOK, nil)
			return
		}

		if inv.Customer == nil || inv.Customer.ID == "" {
			logger.Errorw("payment.webhook.Webhook invoice has no customer", "invoice_id", inv.ID)
			c.JSON(http.StatusOK, nil)
			return
		}

		mInvoice := &model.StripeInvoice{
			InvoiceID:      inv.ID,
			CustomerID:     inv.Customer.ID,
			Status:         "paid",
			PeriodStart:    time.Unix(int64(inv.PeriodStart), 0),
			PeriodEnd:      time.Unix(int64(inv.PeriodEnd), 0),
			SubscriptionID: inv.Subscription.ID,
			ReceivedAt:     time.Now().UTC(),
		}
		// The invoice's own period is degenerate on the first payment (start ==
		// end), so take the period end from the subscription itself.
		expiredAt, err := h.stripeSubscriptionExpiry(c, inv.Subscription.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, "Internal Server Error")
			return
		}

		h.stripeDB.SaveInvoice(c, mInvoice)
		now := time.Now().UTC()

		// Update legacy table
		h.stripeDB.UpdateSubscription(c, &model.StripeSubscription{
			SubscriptionID: inv.Subscription.ID,
			UpdatedAt:      &now,
			Status:         "active",
			ExpiredAt:      &expiredAt,
		})

		// Update unified subscription table
		if unifiedSub, err := h.subscriptionDB.GetSubscriptionByID(c, inv.Subscription.ID); err == nil && unifiedSub != nil {
			unifiedSub.Status = "active"
			unifiedSub.ExpiresAt = &expiredAt
			h.subscriptionDB.UpdateSubscription(c, unifiedSub)
		}

		// get the subscription id from the event:

		// dealing with subscriptions and recurring payments, you'll likely be dealing with invoice.payment_succeeded events
	// handle user canceling subscription:
	// case "checkout.session.async_payment_failed":
	// case "payment_intent.succeeded":

	// if payment fail we can send email to the user to update the payment method
	// case "payment_intent.payment_failed":

	// do somethi

	default:
		logger.Debugw("payment.webhook.Webhook failed to handle event", "event", event.Type)

		return
	}
	logger.Debugw("payment.webhook.Webhook handled event", "event", event.Type)

}

// RevenueCat webhook event structure
type RevenueCatWebhookEvent struct {
	APIVersion string          `json:"api_version"`
	Event      RevenueCatEvent `json:"event"`
}

// RevenueCat sends every transaction field at the top level of `event`; there
// is no nested transactions array.
type RevenueCatEvent struct {
	ID                        string                         `json:"id"`
	Type                      string                         `json:"type"`
	EventTimestampMs          int64                          `json:"event_timestamp_ms"`
	AppUserID                 string                         `json:"app_user_id"`
	OriginalAppUserID         string                         `json:"original_app_user_id"`
	ProductID                 string                         `json:"product_id"`
	EntitlementIDs            []string                       `json:"entitlement_ids"`
	Store                     string                         `json:"store"`
	TransactionID             string                         `json:"transaction_id"`
	OriginalTransactionID     string                         `json:"original_transaction_id"`
	PeriodType                string                         `json:"period_type"`
	PurchasedAtMs             *int64                         `json:"purchased_at_ms"`
	ExpirationAtMs            *int64                         `json:"expiration_at_ms"`
	Price                     *float64                       `json:"price"`
	Currency                  string                         `json:"currency"`
	Entitlements              map[string]interface{}         `json:"entitlements"`
	Subscriber                RevenueCatSubscriberAttributes `json:"subscriber_attributes"`
	Environment               string                         `json:"environment"`
	AppID                     string                         `json:"app_id"`
	Aliases                   []string                       `json:"aliases"`
	TransferredFrom           []string                       `json:"transferred_from"`
	TransferredTo             []string                       `json:"transferred_to"`
	GracePeriodExpirationAtMs *int64                         `json:"grace_period_expiration_at_ms"`
}

type RevenueCatSubscriberAttributes struct {
	UserID struct {
		Value     string `json:"value"`
		UpdatedAt int64  `json:"updated_at_ms"`
	} `json:"$userId"`
}

// subscriptionID is the canonical key for the unified subscription row. It
// stays stable across the whole lifecycle of a store subscription, unlike
// transaction_id which changes on every renewal.
func (e RevenueCatEvent) subscriptionID() string {
	if e.OriginalTransactionID != "" {
		return e.OriginalTransactionID
	}
	return e.AppUserID
}

func (e RevenueCatEvent) expiresAt() *time.Time {
	if e.ExpirationAtMs == nil {
		return nil
	}
	expiresAt := time.UnixMilli(*e.ExpirationAtMs).UTC()
	return &expiresAt
}

func (h *Webhook) RevenueCatWebhook(c *gin.Context) {
	logger := logging.FromContext(c)

	// Without a configured secret every comparison below would succeed against
	// an absent header, leaving the endpoint open, so fail closed instead.
	if h.revenueCatConfig.AuthSecret == "" {
		logger.Error("revenuecat.webhook.RevenueCatWebhook no auth secret configured, rejecting request")
		c.JSON(http.StatusServiceUnavailable, "Webhook not configured")
		return
	}

	authHeader := c.GetHeader("Authorization")
	if subtle.ConstantTimeCompare([]byte(authHeader), []byte(h.revenueCatConfig.AuthSecret)) != 1 {
		logger.Error("revenuecat.webhook.RevenueCatWebhook failed to validate auth header")
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}

	var webhookEvent RevenueCatWebhookEvent
	if err := c.BindJSON(&webhookEvent); err != nil {
		logger.Errorw("revenuecat.webhook.RevenueCatWebhook failed to bind json", "err", err)
		c.JSON(http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Check if event already processed (idempotency)
	exists, err := h.revenueCatDB.EventExists(c, webhookEvent.Event.ID)
	if err != nil {
		logger.Errorw("revenuecat.webhook.RevenueCatWebhook failed to check event existence", "err", err)
		c.JSON(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if exists {
		logger.Debugw("revenuecat.webhook.RevenueCatWebhook event already processed", "event_id", webhookEvent.Event.ID)
		c.JSON(http.StatusOK, "Event already processed")
		return
	}

	// Process based on event type
	var handlerErr error
	switch webhookEvent.Event.Type {
	case "INITIAL_PURCHASE":
		handlerErr = h.handleInitialPurchase(c, webhookEvent.Event)
	case "RENEWAL":
		handlerErr = h.handleRenewal(c, webhookEvent.Event)
	case "CANCELLATION":
		handlerErr = h.handleCancellation(c, webhookEvent.Event)

	case "EXPIRATION":
		handlerErr = h.handleExpiration(c, webhookEvent.Event)

	case "BILLING_ISSUE":
		handlerErr = h.handleBillingIssue(c, webhookEvent.Event)

	case "PRODUCT_CHANGE":
		handlerErr = h.handleProductChange(c, webhookEvent.Event)
	case "NON_RENEWING_PURCHASE":
		h.handleNonRenewingPurchase(c, webhookEvent.Event)
	case "TRANSFER":
		handlerErr = h.handleTransfer(c, webhookEvent.Event)
	case "VIRTUAL_CURRENCY_TRANSACTION":
		h.handleVirtualCurrencyTransaction(c, webhookEvent.Event)
	default:
		logger.Debugw("revenuecat.webhook.RevenueCatWebhook unhandled event type", "event_type", webhookEvent.Event.Type)
	}

	// Leave the event unrecorded so RevenueCat's retry gets another chance to
	// apply it; the idempotency check above would otherwise skip it forever.
	if handlerErr != nil {
		logger.Errorw("revenuecat.webhook.RevenueCatWebhook failed to process event",
			"event_id", webhookEvent.Event.ID, "event_type", webhookEvent.Event.Type, "err", handlerErr)
		c.JSON(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Save event to prevent duplicate processing
	eventModel := &model.RevenueCatEvent{
		EventID:           webhookEvent.Event.ID,
		EventType:         webhookEvent.Event.Type,
		AppUserID:         webhookEvent.Event.AppUserID,
		OriginalAppUserID: webhookEvent.Event.OriginalAppUserID,
		ProductID:         webhookEvent.Event.ProductID,
		Store:             webhookEvent.Event.Store,
		EventTimestamp:    time.Unix(webhookEvent.Event.EventTimestampMs/1000, 0),
		ProcessedAt:       time.Now().UTC(),
	}

	if _, err := h.revenueCatDB.SaveEvent(c, eventModel); err != nil {
		logger.Errorw("revenuecat.webhook.RevenueCatWebhook failed to save event", "err", err)
	}

	c.JSON(http.StatusOK, "OK")
}

func (h *Webhook) handleInitialPurchase(c *gin.Context, event RevenueCatEvent) error {
	logger := logging.FromContext(c)

	// Extract user ID from subscriber attributes or app_user_id (assume it's the user ID)
	userID := h.extractUserIDFromEvent(event)
	if userID == 0 {
		logger.Errorw("revenuecat.webhook.handleInitialPurchase failed to extract user ID", "app_user_id", event.AppUserID)
		return nil
	}

	subscriptionID := event.subscriptionID()
	if subscriptionID == "" {
		logger.Errorw("revenuecat.webhook.handleInitialPurchase event has no subscription identifier", "event_id", event.ID)
		return nil
	}

	user, err := h.userRepo.GetUserByID(c, userID)
	if err != nil {
		logger.Errorw("revenuecat.webhook.handleInitialPurchase failed to get user", "user_id", userID, "err", err)
		// An app_user_id that maps to no user will never resolve, so don't ask
		// RevenueCat to retry it forever.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	now := time.Now().UTC()
	subscription := &model.Subscription{
		ID:                     subscriptionID,
		UserID:                 userID,
		CircleID:               user.CircleID,
		Provider:               model.SubscriptionProviderRevenueCat,
		ExternalSubscriptionID: subscriptionID,
		ExternalCustomerID:     event.AppUserID,
		ProductID:              event.ProductID,
		Status:                 "active",
		ExpiresAt:              event.expiresAt(),
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if _, err := h.subscriptionDB.SaveSubscription(c, subscription); err != nil {
		logger.Errorw("revenuecat.webhook.handleInitialPurchase failed to save subscription", "err", err)
		return err
	}
	return nil
}

// findSubscription resolves the unified subscription row for an event. Rows are
// keyed on original_transaction_id, but ones written before that was parsed are
// keyed on app_user_id, so fall back to it rather than creating a duplicate.
func (h *Webhook) findSubscription(c *gin.Context, event RevenueCatEvent) (*model.Subscription, error) {
	seen := make(map[string]bool, 2)
	for _, id := range []string{event.OriginalTransactionID, event.AppUserID} {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		subscription, err := h.subscriptionDB.GetSubscriptionByID(c, id)
		if err != nil {
			return nil, err
		}
		if subscription != nil {
			return subscription, nil
		}
	}
	return nil, nil
}

// applyStatus is the shared path for events that only move the subscription
// status: resolve the row, set the status, persist.
func (h *Webhook) applyStatus(c *gin.Context, event RevenueCatEvent, handler, status string) error {
	logger := logging.FromContext(c)

	subscription, err := h.findSubscription(c, event)
	if err != nil {
		logger.Errorw("revenuecat.webhook."+handler+" failed to get subscription", "err", err)
		return err
	}

	if subscription == nil {
		logger.Errorw("revenuecat.webhook."+handler+" subscription not found",
			"original_transaction_id", event.OriginalTransactionID, "app_user_id", event.AppUserID)
		return nil
	}

	subscription.Status = status

	if err := h.subscriptionDB.UpdateSubscription(c, subscription); err != nil {
		logger.Errorw("revenuecat.webhook."+handler+" failed to update subscription", "err", err)
		return err
	}
	return nil
}

// extractUserIDFromEvent tries to extract user ID from RevenueCat event
// You may need to adjust this based on how you set up the app_user_id mapping
func (h *Webhook) extractUserIDFromEvent(event RevenueCatEvent) int {
	// Option 1: If app_user_id is the actual user ID
	if userID, err := strconv.Atoi(event.AppUserID); err == nil && userID > 0 {
		return userID
	}

	// Option 2: Check subscriber attributes for $userId
	if event.Subscriber.UserID.Value != "" {
		if userID, err := strconv.Atoi(event.Subscriber.UserID.Value); err == nil && userID > 0 {
			return userID
		}
	}

	return 0
}

func (h *Webhook) handleRenewal(c *gin.Context, event RevenueCatEvent) error {
	logger := logging.FromContext(c)

	subscription, err := h.findSubscription(c, event)
	if err != nil {
		logger.Errorw("revenuecat.webhook.handleRenewal failed to get subscription", "err", err)
		return err
	}

	if subscription == nil {
		// Create new subscription if not found
		return h.handleInitialPurchase(c, event)
	}

	// Update subscription
	subscription.Status = "active"
	subscription.ProductID = event.ProductID
	if expiresAt := event.expiresAt(); expiresAt != nil {
		subscription.ExpiresAt = expiresAt
	}

	if err := h.subscriptionDB.UpdateSubscription(c, subscription); err != nil {
		logger.Errorw("revenuecat.webhook.handleRenewal failed to update subscription", "err", err)
		return err
	}
	return nil
}

func (h *Webhook) handleCancellation(c *gin.Context, event RevenueCatEvent) error {
	// Canceling auto-renew does not revoke the paid entitlement. EXPIRATION
	// (including refunds/revocations) ends access; ExpiresAt also bounds it.
	return nil
}

func (h *Webhook) handleExpiration(c *gin.Context, event RevenueCatEvent) error {
	return h.applyStatus(c, event, "handleExpiration", "expired")
}

func (h *Webhook) handleBillingIssue(c *gin.Context, event RevenueCatEvent) error {
	// Billing failures do not revoke access during the store's grace period.
	// Never reactivate an expired subscription on a delayed billing event.
	if event.GracePeriodExpirationAtMs == nil {
		return nil
	}
	sub, err := h.findSubscription(c, event)
	if err != nil || sub == nil {
		return err
	}
	graceEnd := time.UnixMilli(*event.GracePeriodExpirationAtMs).UTC()
	if sub.Status != "active" || sub.ExpiresAt == nil || !graceEnd.After(*sub.ExpiresAt) {
		return nil
	}
	sub.ExpiresAt = &graceEnd
	return h.subscriptionDB.UpdateSubscription(c, sub)
}

func (h *Webhook) handleProductChange(c *gin.Context, event RevenueCatEvent) error {
	logger := logging.FromContext(c)

	subscription, err := h.findSubscription(c, event)
	if err != nil {
		logger.Errorw("revenuecat.webhook.handleProductChange failed to get subscription", "err", err)
		return err
	}

	if subscription == nil {
		logger.Errorw("revenuecat.webhook.handleProductChange subscription not found",
			"original_transaction_id", event.OriginalTransactionID, "app_user_id", event.AppUserID)
		return nil
	}

	// Update product ID and other relevant fields
	subscription.ProductID = event.ProductID
	if expiresAt := event.expiresAt(); expiresAt != nil {
		subscription.ExpiresAt = expiresAt
	}

	if err := h.subscriptionDB.UpdateSubscription(c, subscription); err != nil {
		logger.Errorw("revenuecat.webhook.handleProductChange failed to update subscription", "err", err)
		return err
	}
	return nil
}

func (h *Webhook) handleNonRenewingPurchase(c *gin.Context, event RevenueCatEvent) {
	logger := logging.FromContext(c)
	logger.Debugw("revenuecat.webhook.handleNonRenewingPurchase processing non-renewing purchase", "app_user_id", event.AppUserID, "product_id", event.ProductID)
	// Non-renewing purchases don't create subscriptions, just log the event
}

func (h *Webhook) handleTransfer(c *gin.Context, event RevenueCatEvent) error {
	logger := logging.FromContext(c)

	// TRANSFER has alias arrays, not app_user_id or transaction fields.
	// Skip anonymous aliases and resolve a registered destination account.
	for _, alias := range event.TransferredTo {
		userID, err := strconv.Atoi(alias)
		if err != nil || userID <= 0 {
			continue
		}
		user, err := h.userRepo.GetUserByID(c, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		return h.subscriptionDB.TransferRevenueCatSubscriptions(c, event.TransferredFrom, alias, user.ID, user.CircleID)
	}
	logger.Warnw("revenuecat.webhook.handleTransfer no registered destination", "event_id", event.ID)
	return nil
}

func (h *Webhook) handleVirtualCurrencyTransaction(c *gin.Context, event RevenueCatEvent) {
	logger := logging.FromContext(c)
	logger.Debugw("revenuecat.webhook.handleVirtualCurrencyTransaction processing virtual currency transaction", "app_user_id", event.AppUserID, "product_id", event.ProductID)
	// Virtual currency transactions are logged but don't affect subscriptions
}

func Webhooks(cfg *config.Config, w *Webhook, r *gin.Engine, auth *jwt.GinJWTMiddleware) {

	paymentsWithoutAuth := r.Group("webhooks")

	paymentsWithoutAuth.Use(utils.TimeoutMiddleware(cfg.Server.WebhookTimeout))
	{
		paymentsWithoutAuth.POST("stripe", w.StripeWebhook)
		paymentsWithoutAuth.POST("revenuecat", w.RevenueCatWebhook)
	}

}
