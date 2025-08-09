package payment

import (
	"net/http"

	auth "donetick.com/core/internal/auth"
	"donetick.com/core/logging"

	"donetick.com/core/config"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"donetick.com/core/external/payment/model"
	pDB "donetick.com/core/external/payment/repo"
	stripeService "donetick.com/core/external/payment/service"
)

type Handler struct {
	stripeDB       pDB.StripeDB
	subscriptionDB pDB.SubscriptionDB
	whitelistIPs   map[string]bool
	stripe         *stripeService.StripeService
	prices         []config.StripePrices
}

func NewHandler(stripeDB pDB.StripeDB, subscriptionDB pDB.SubscriptionDB, stripeService *stripeService.StripeService, config *config.Config) *Handler {

	whitelistIPs := make(map[string]bool)
	for _, ip := range config.StripeConfig.WhitelistedIPs {
		whitelistIPs[ip] = true
	}

	return &Handler{
		stripeDB:       stripeDB,
		subscriptionDB: subscriptionDB,
		whitelistIPs:   whitelistIPs,
		stripe:         stripeService,
		prices:         config.StripeConfig.Prices,
	}
}

func (h *Handler) CreateSubscription(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser := auth.MustCurrentUser(c)
	var customer *model.StripeCustomer
	var err error
	// TODO: DELETE THIS
	val, _ := h.stripeDB.GetCustomer(c, currentUser.ID)
	if val != nil {
		currentUser.CustomerID = &val.CustomerID
	}

	if currentUser.CustomerID == nil {
		sCustomer, err := h.stripe.CreateCustomer(c, currentUser)
		if err != nil {
			logger.Errorw("payment.handler.CreateSubscription failed to create customer", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer"})
		}
		customer = &model.StripeCustomer{
			UserID:     uint64(currentUser.ID),
			CustomerID: sCustomer.ID,
			CircleID:   currentUser.CircleID,
		}
		customer, err = h.stripeDB.SaveCustomer(c, customer)
		if err != nil {
			logger.Errorw("payment.handler.CreateSubscription failed to save customer", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save customer"})
			return
		}
	} else {
		customer, err = h.stripeDB.GetCustomer(c, currentUser.ID)
		if err != nil {
			logger.Errorw("payment.handler.CreateSubscription failed to get customer", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customer"})
			return
		}
	}
	session, err := h.stripe.CreateSubscriptionCheckoutSession(c, customer.CustomerID, h.prices[0].PriceID)
	if err != nil {
		logger.Errorw("payment.handler.CreateSubscription failed to create checkout session", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create checkout session"})
		return
	}
	h.stripeDB.SaveSession(c, &model.StripeSession{
		CustomerID: customer.CustomerID,
		UserID:     uint64(currentUser.ID),
		SessionID:  session.ID,
		Status:     string(session.PaymentStatus),
	})

	c.JSON(http.StatusOK, gin.H{"sessionURL": session.URL})
	return

}

func (h *Handler) CancelSubscription(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser := auth.MustCurrentUser(c)

	// Get subscription from unified table
	sub, err := h.subscriptionDB.GetSubscriptionByUserID(c, currentUser.ID)
	if err != nil {
		logger.Errorw("payment.handler.CancelSubscription failed to get subscription", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription"})
		return
	}

	if sub == nil {
		logger.Errorw("payment.handler.CancelSubscription no active subscription found", "user_id", currentUser.ID)
		c.JSON(http.StatusNotFound, gin.H{"error": "No active subscription found"})
		return
	}

	// Only cancel Stripe subscriptions via API (RevenueCat is handled via their dashboard/app)
	if sub.Provider == model.SubscriptionProviderStripe {
		subscription, err := h.stripe.CancelSubscription(sub.ExternalSubscriptionID)
		if err != nil {
			logger.Errorw("payment.handler.CancelSubscription failed to cancel stripe subscription", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
			return
		}

		// Update unified subscription table
		if subscription.CancelAtPeriodEnd == true {
			sub.Status = "cancelled"
			if err := h.subscriptionDB.UpdateSubscription(c, sub); err != nil {
				logger.Errorw("payment.handler.CancelSubscription failed to update subscription status", "err", err)
			}

			// Also update legacy table for backward compatibility
			h.stripeDB.CancelSubscription(c, sub.ExternalSubscriptionID)
		}
	} else {
		// For RevenueCat and other providers, just mark as cancelled in our system
		// The actual cancellation should be handled via their respective dashboards
		sub.Status = "cancelled"
		if err := h.subscriptionDB.UpdateSubscription(c, sub); err != nil {
			logger.Errorw("payment.handler.CancelSubscription failed to update subscription status", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Subscription cancelled",
		"provider": sub.Provider,
	})
}

// RouteV1 routes user api given config and gin.Engine
func Routes(cfg *config.Config, h *Handler, r *gin.Engine, auth *jwt.GinJWTMiddleware) {

	paymentsV1 := r.Group("api/v1/payments")

	paymentsV1.Use(auth.MiddlewareFunc())
	{
		paymentsV1.GET("create-subscription", h.CreateSubscription)
		paymentsV1.POST("cancel-subscription", h.CancelSubscription)

	}

}
