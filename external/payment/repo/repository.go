package database

import (
	"context"
	"errors"
	"time"

	pModel "donetick.com/core/external/payment/model"

	// "forward.in/internal/metric"

	"donetick.com/core/logging"

	"gorm.io/gorm"
)

func NewStripeDB(db *gorm.DB) StripeDB {
	return StripeDB{db: db}
}

func NewRevenueCatDB(db *gorm.DB) RevenueCatDB {
	return RevenueCatDB{db: db}
}

func NewSubscriptionDB(db *gorm.DB) SubscriptionDB {
	return SubscriptionDB{db: db}
}

type StripeDB struct {
	db *gorm.DB
}

type RevenueCatDB struct {
	db *gorm.DB
}

type SubscriptionDB struct {
	db *gorm.DB
}

func (a *StripeDB) SaveCustomer(ctx context.Context, customer *pModel.StripeCustomer) (*pModel.StripeCustomer, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("customer.db.Save", "customer", customer)

	if err := a.db.WithContext(ctx).Create(customer).Error; err != nil {
		logger.Error("customer.db.Save failed to save", "err", err)
		return nil, err
	}
	return customer, nil
}

func (a *StripeDB) SaveSubscription(ctx context.Context, subscription *pModel.StripeSubscription) (*pModel.StripeSubscription, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("subscription.db.Save", "subscription", subscription)

	if err := a.db.WithContext(ctx).Save(subscription).Error; err != nil {
		logger.Error("subscription.db.Save failed to save", "err", err)

		return nil, err
	}
	return subscription, nil
}

func (a *StripeDB) SaveSession(ctx context.Context, session *pModel.StripeSession) (*pModel.StripeSession, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("session.db.Save", "session", session)

	if err := a.db.WithContext(ctx).Save(session).Error; err != nil {
		logger.Error("session.db.Save failed to save", "err", err)

		return nil, err
	}
	return session, nil
}

func (a *StripeDB) GetCustomer(ctx context.Context, accountID int) (*pModel.StripeCustomer, error) {
	logger := logging.FromContext(ctx)

	var customer pModel.StripeCustomer
	if err := a.db.WithContext(ctx).Where("user_id = ?", accountID).First(&customer).Error; err != nil {
		logger.Error("customer.db.Get failed to get", "err", err)
		return nil, err
	}
	return &customer, nil
}

func (a *StripeDB) GetCustomerByCustomerID(ctx context.Context, customerID string) (*pModel.StripeCustomer, error) {
	logger := logging.FromContext(ctx)

	var customer pModel.StripeCustomer
	if err := a.db.WithContext(ctx).Where("customer_id = ?", customerID).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.Error("customer.db.GetByCustomerID failed to get", "err", err)
		return nil, err
	}
	return &customer, nil
}

func (a *StripeDB) GetSubscriptionByID(ctx context.Context, subscriptionID string) (*pModel.StripeSubscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.StripeSubscription
	if err := a.db.WithContext(ctx).Where("subscription_id = ?", subscriptionID).First(&subscription).Error; err != nil {
		logger.Error("subscription.db.Get failed to get", "err", err)
		return nil, err
	}
	return &subscription, nil
}

func (a *StripeDB) GetSubscriptionByAccountID(ctx context.Context, userID int) (*pModel.StripeSubscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.StripeSubscription
	// join subscriptions and customers table and get the subscription by account_id
	if err := a.db.WithContext(ctx).Table("stripe_subscriptions").Select("stripe_subscriptions.*").Joins("left join stripe_customers on stripe_customers.customer_id = stripe_subscriptions.customer_id").Where("stripe_customers.user_id = ?", userID).First(&subscription).Error; err != nil {
		logger.Error("subscription.db.Get failed to get", "err", err)
		return nil, err
	}

	// if err := a.db.WithContext(ctx).Where("account_id", accountID).First(&subscription).Error; err != nil {
	// 	logger.Error("subscription.db.Get failed to get", "err", err)
	// 	return nil, err
	// }
	return &subscription, nil
}

func (a *StripeDB) GetSession(ctx context.Context, SessionID string) (*pModel.StripeSession, error) {
	logger := logging.FromContext(ctx)

	var session pModel.StripeSession
	if err := a.db.WithContext(ctx).Where("session_id = ?", SessionID).First(&session).Error; err != nil {
		logger.Error("session.db.Get failed to get", "err", err)
		return nil, err
	}
	return &session, nil
}

func (a *StripeDB) DeleteSession(ctx context.Context, sessionID string) error {
	logger := logging.FromContext(ctx)

	if err := a.db.WithContext(ctx).Where("session_id", sessionID).Delete(&pModel.StripeSession{}).Error; err != nil {
		logger.Error("session.db.Delete failed to delete", "err", err)
		return err
	}
	return nil
}

func (a *StripeDB) CancelSubscription(ctx context.Context, subscriptionID string) error {
	logger := logging.FromContext(ctx)

	// existed, err := a.GetSubscriptionByID(ctx, subscriptionID)
	// if err != nil {
	// 	logger.Error("subscription.db.Delete failed to delete", "err", err)
	// 	return err
	// }

	err := a.db.WithContext(ctx).Model(&pModel.StripeSubscription{}).Where("subscription_id = ?", subscriptionID).Updates(map[string]interface{}{
		"status":     "canceled",
		"updated_at": time.Now().UTC(),
	}).Error

	if err != nil {
		logger.Error("subscription.db.Delete failed to delete", "err", err)
		return err
	}
	return nil
}

func (a *StripeDB) UpdateSession(ctx context.Context, sessionID string, status string) error {
	logger := logging.FromContext(ctx)

	// set status where session_id = sessionID
	if err := a.db.WithContext(ctx).Model(&pModel.StripeSession{}).Where("session_id = ?", sessionID).Update("status", status).Error; err != nil {
		logger.Error("session.db.Update failed to update", "err", err)
		return err
	}
	return nil
}

func (a *StripeDB) SaveInvoice(ctx context.Context, invoice *pModel.StripeInvoice) (*pModel.StripeInvoice, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("invoice.db.Save", "invoice", invoice)

	if err := a.db.WithContext(ctx).Create(invoice).Error; err != nil {
		logger.Error("invoice.db.Save failed to save", "err", err)

		return nil, err
	}
	return invoice, nil
}

func (a *StripeDB) UpdateSubscription(ctx context.Context, subscription *pModel.StripeSubscription) error {
	logger := logging.FromContext(ctx)

	if subscription.SubscriptionID == "" {
		return errors.New("subscription id is required")
	}
	// only update the status and expired_at
	if err := a.db.WithContext(ctx).Model(&pModel.StripeSubscription{}).Where("subscription_id = ?", subscription.SubscriptionID).Updates(map[string]interface{}{
		"status":     subscription.Status,
		"expired_at": subscription.ExpiredAt,
		"updated_at": subscription.UpdatedAt,
	}).Error; err != nil {
		logger.Error("subscription.db.Update failed to update", "err", err)
		return err
	}

	return nil
}

// RevenueCat database operations
func (r *RevenueCatDB) SaveSubscription(ctx context.Context, subscription *pModel.RevenueCatSubscription) (*pModel.RevenueCatSubscription, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("revenuecat.subscription.db.Save", "subscription", subscription)

	if err := r.db.WithContext(ctx).Save(subscription).Error; err != nil {
		logger.Error("revenuecat.subscription.db.Save failed to save", "err", err)
		return nil, err
	}
	return subscription, nil
}

func (r *RevenueCatDB) GetSubscriptionByAppUserID(ctx context.Context, appUserID string) (*pModel.RevenueCatSubscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.RevenueCatSubscription
	if err := r.db.WithContext(ctx).Where("app_user_id = ? AND status = ?", appUserID, "active").First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.Error("revenuecat.subscription.db.Get failed to get", "err", err)
		return nil, err
	}
	return &subscription, nil
}

func (r *RevenueCatDB) GetSubscriptionByOriginalTransactionID(ctx context.Context, originalTransactionID string) (*pModel.RevenueCatSubscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.RevenueCatSubscription
	if err := r.db.WithContext(ctx).Where("original_transaction_id = ?", originalTransactionID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.Error("revenuecat.subscription.db.Get failed to get", "err", err)
		return nil, err
	}
	return &subscription, nil
}

func (r *RevenueCatDB) UpdateSubscription(ctx context.Context, subscription *pModel.RevenueCatSubscription) error {
	logger := logging.FromContext(ctx)

	if subscription.OriginalTransactionID == "" {
		return errors.New("original transaction id is required")
	}

	now := time.Now().UTC()
	subscription.UpdatedAt = &now

	if err := r.db.WithContext(ctx).Model(&pModel.RevenueCatSubscription{}).Where("original_transaction_id = ?", subscription.OriginalTransactionID).Updates(subscription).Error; err != nil {
		logger.Error("revenuecat.subscription.db.Update failed to update", "err", err)
		return err
	}

	return nil
}

func (r *RevenueCatDB) SaveEvent(ctx context.Context, event *pModel.RevenueCatEvent) (*pModel.RevenueCatEvent, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("revenuecat.event.db.Save", "event", event)

	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		logger.Error("revenuecat.event.db.Save failed to save", "err", err)
		return nil, err
	}
	return event, nil
}

func (r *RevenueCatDB) EventExists(ctx context.Context, eventID string) (bool, error) {
	logger := logging.FromContext(ctx)

	var count int64
	if err := r.db.WithContext(ctx).Model(&pModel.RevenueCatEvent{}).Where("event_id = ?", eventID).Count(&count).Error; err != nil {
		logger.Error("revenuecat.event.db.Exists failed to check", "err", err)
		return false, err
	}
	return count > 0, nil
}

// Unified Subscription Database Operations
func (s *SubscriptionDB) SaveSubscription(ctx context.Context, subscription *pModel.Subscription) (*pModel.Subscription, error) {
	logger := logging.FromContext(ctx)

	logger.Debugw("subscription.db.Save", "subscription", subscription)

	if err := s.db.WithContext(ctx).Save(subscription).Error; err != nil {
		logger.Error("subscription.db.Save failed to save", "err", err)
		return nil, err
	}
	return subscription, nil
}

func (s *SubscriptionDB) GetSubscriptionByUserID(ctx context.Context, userID int) (*pModel.Subscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.Subscription
	if err := s.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, "active").
		Order("created_at DESC").First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.Error("subscription.db.Get failed to get", "err", err)
		return nil, err
	}
	return &subscription, nil
}

func (s *SubscriptionDB) GetSubscriptionByID(ctx context.Context, id string) (*pModel.Subscription, error) {
	logger := logging.FromContext(ctx)

	var subscription pModel.Subscription
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.Error("subscription.db.Get failed to get", "err", err)
		return nil, err
	}
	return &subscription, nil
}

func (s *SubscriptionDB) UpdateSubscription(ctx context.Context, subscription *pModel.Subscription) error {
	logger := logging.FromContext(ctx)

	if subscription.ID == "" {
		return errors.New("subscription id is required")
	}

	subscription.UpdatedAt = time.Now().UTC()

	if err := s.db.WithContext(ctx).Model(&pModel.Subscription{}).Where("id = ?", subscription.ID).Updates(subscription).Error; err != nil {
		logger.Error("subscription.db.Update failed to update", "err", err)
		return err
	}

	return nil
}
