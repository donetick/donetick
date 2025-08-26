package model

import "time"

type StripeCustomer struct {
	CustomerID string     `json:"customer_id" gorm:"column:customer_id;not null;index"`
	UserID     uint64     `json:"user_id" gorm:"column:user_id;not null;index"`
	CircleID   int        `json:"circle_id" gorm:"column:circle_id;index"`
	CreatedAt  *time.Time `json:"created_at" gorm:"column:created_at;not null"`
	UpdatedAt  *time.Time `json:"updated_at" gorm:"column:updated_at"`
}

type StripeSession struct {
	SessionID  string     `json:"session_id" gorm:"column:session_id;not null;primaryKey"`
	CustomerID string     `json:"customer_id" gorm:"column:customer_id;not null"`
	UserID     uint64     `json:"user_id" gorm:"column:user_id;not null"`
	CreatedAt  *time.Time `json:"created_at" gorm:"column:created_at;not null"`
	UpdatedAt  *time.Time `json:"updated_at" gorm:"column:updated_at"`
	Status     string     `json:"status" gorm:"column:status;not null"`
}

type StripeSubscription struct {
	SubscriptionID string     `json:"subscription_id" gorm:"column:subscription_id;primaryKey;not null"`
	CustomerID     string     `json:"customer_id" gorm:"column:customer_id;not null;index"`
	CreatedAt      *time.Time `json:"created_at" gorm:"column:created_at;not null"`
	UpdatedAt      *time.Time `json:"updated_at" gorm:"column:updated_at"`
	Status         string     `json:"status" gorm:"column:status;not null"`
	ExpiredAt      *time.Time `json:"expired_at" gorm:"column:expired_at"`
}

type StripeInvoice struct {
	InvoiceID      string    `json:"invoice_id" gorm:"column:invoice_id;not null;primaryKey"`
	Amount         int       `json:"amount" gorm:"column:amount;not null"`
	ReceivedAt     time.Time `json:"received_at" gorm:"column:received_at;not null"`
	Status         string    `json:"status" gorm:"column:status;not null"`
	SubscriptionID string    `json:"subscription_id" gorm:"column:subscription_id;not null"`
	CustomerID     string    `json:"customer_id" gorm:"column:customer_id;not null"`
	PeriodStart    time.Time `json:"period_start" gorm:"column:period_start;not null"`
	PeriodEnd      time.Time `json:"period_end" gorm:"column:period_end;not null"`
}

// RevenueCat models for IAP subscriptions
type RevenueCatSubscription struct {
	AppUserID             string     `json:"app_user_id" gorm:"column:app_user_id;not null;index"`
	OriginalAppUserID     string     `json:"original_app_user_id" gorm:"column:original_app_user_id;not null;index"`
	ProductID             string     `json:"product_id" gorm:"column:product_id;not null"`
	OriginalTransactionID string     `json:"original_transaction_id" gorm:"column:original_transaction_id;primaryKey;not null"`
	Store                 string     `json:"store" gorm:"column:store;not null"`    // app_store, play_store
	Status                string     `json:"status" gorm:"column:status;not null"`  // active, expired, cancelled, billing_issue, etc.
	PeriodType            string     `json:"period_type" gorm:"column:period_type"` // normal, trial, intro
	ExpiresDate           *time.Time `json:"expires_date" gorm:"column:expires_date"`
	AutoRenewStatus       bool       `json:"auto_renew_status" gorm:"column:auto_renew_status"`
	IsTrialPeriod         bool       `json:"is_trial_period" gorm:"column:is_trial_period"`
	CreatedAt             *time.Time `json:"created_at" gorm:"column:created_at;not null"`
	UpdatedAt             *time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// Unified subscription table for all payment providers
type Subscription struct {
	ID                     string               `json:"id" gorm:"column:id;primaryKey;not null"`                                                                                          // subscription_id or original_transaction_id
	UserID                 int                  `json:"user_id" gorm:"column:user_id;not null;index:idx_subscriptions_user_id"`                                                           // direct link to users table
	CircleID               int                  `json:"circle_id" gorm:"column:circle_id;index"`                                                                                          // optional circle association
	Provider               SubscriptionProvider `json:"provider" gorm:"column:provider;not null;index:idx_subscriptions_provider_status;uniqueIndex:idx_subscriptions_provider_external"` // 'stripe', 'revenuecat'
	ExternalSubscriptionID string               `json:"external_subscription_id" gorm:"column:external_subscription_id;uniqueIndex:idx_subscriptions_provider_external"`                  // provider's subscription ID
	ExternalCustomerID     string               `json:"external_customer_id" gorm:"column:external_customer_id"`                                                                          // stripe customer_id or app_user_id
	ProductID              string               `json:"product_id" gorm:"column:product_id"`                                                                                              // product/price identifier
	Status                 string               `json:"status" gorm:"column:status;not null;index:idx_subscriptions_provider_status"`                                                     // active, expired, cancelled, billing_issue
	ExpiresAt              *time.Time           `json:"expires_at" gorm:"column:expires_at;index:idx_subscriptions_expires_at"`                                                           // unified expiration field
	CreatedAt              time.Time            `json:"created_at" gorm:"column:created_at;not null"`
	UpdatedAt              time.Time            `json:"updated_at" gorm:"column:updated_at;not null"`
	ProviderData           string               `json:"provider_data" gorm:"column:provider_data;type:text"` // JSON for provider-specific fields
}

type SubscriptionProvider int // Enum for subscription providers
const (
	SubscriptionProviderStripe SubscriptionProvider = iota
	SubscriptionProviderRevenueCat
)

type RevenueCatEvent struct {
	EventID           string    `json:"event_id" gorm:"column:event_id;primaryKey;not null"`
	EventType         string    `json:"event_type" gorm:"column:event_type;not null"`
	AppUserID         string    `json:"app_user_id" gorm:"column:app_user_id;not null;index"`
	OriginalAppUserID string    `json:"original_app_user_id" gorm:"column:original_app_user_id;not null;index"`
	ProductID         string    `json:"product_id" gorm:"column:product_id;not null"`
	Store             string    `json:"store" gorm:"column:store;not null"`
	EventTimestamp    time.Time `json:"event_timestamp" gorm:"column:event_timestamp;not null"`
	ProcessedAt       time.Time `json:"processed_at" gorm:"column:processed_at;not null"`
}
