package service

import (
	"context"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

type StripeService struct {
	Key        string
	SuccessURL string
	CancelURL  string
}

func NewStripeService(config *config.Config) *StripeService {
	return &StripeService{
		Key:        config.StripeConfig.APIKey,
		SuccessURL: config.StripeConfig.SuccessURL,
		CancelURL:  config.StripeConfig.CancelURL,
	}
}

func (s *StripeService) CreateCustomer(c context.Context, user *uModel.UserDetails) (*stripe.Customer, error) {
	stripe.Key = s.Key
	customer, err := customer.New(&stripe.CustomerParams{
		Name:  stripe.String(user.DisplayName),
		Email: stripe.String(user.Email),
	})
	if err != nil {
		return nil, err
	}
	return customer, nil
}

func (s *StripeService) CancelSubscription(subscriptionID string) (*stripe.Subscription, error) {
	stripe.Key = s.Key
	sub, err := subscription.Update(
		subscriptionID,
		&stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(true)},
	)

	// sub, err := subscription.Cancel(subscriptionID, nil)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *StripeService) CreateSubscriptionCheckoutSession(c context.Context, customerID string, priceID string) (*stripe.CheckoutSession, error) {
	logger := logging.FromContext(c)
	logger.Debugw("Creating subscription checkout session", "customerID", customerID, "priceID", priceID)
	stripe.Key = s.Key
	se, err := session.New(&stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{

			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(s.SuccessURL),
		CancelURL:  stripe.String(s.CancelURL),
	})
	if err != nil {
		logger.Errorw("Failed to create subscription checkout session", "err", err)
		return nil, err
	}
	return se, nil
}
