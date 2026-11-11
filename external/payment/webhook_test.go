package payment

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/external/payment/model"
	pDB "donetick.com/core/external/payment/repo"
	cModel "donetick.com/core/internal/circle/model"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newWebhookTest(t *testing.T) (*Webhook, *gorm.DB, *gin.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "payment.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Subscription{}, &uModel.User{}, &uModel.UserNotificationTarget{}, &cModel.Circle{}))
	h := &Webhook{subscriptionDB: pDB.NewSubscriptionDB(db), userRepo: uRepo.NewUserRepository(db, &config.Config{})}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/webhooks/revenuecat", nil)
	return h, db, c
}

func TestCancellationPreservesPaidAccess(t *testing.T) {
	h, db, c := newWebhookTest(t)
	expiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	sub := model.Subscription{ID: "original", Provider: model.SubscriptionProviderRevenueCat, ExternalSubscriptionID: "original", Status: "active", ExpiresAt: &expiry}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, h.handleCancellation(c, RevenueCatEvent{OriginalTransactionID: sub.ID}))
	got, err := h.subscriptionDB.GetSubscriptionByID(c, sub.ID)
	require.NoError(t, err)
	require.Equal(t, "active", got.Status)
	require.True(t, expiry.Equal(*got.ExpiresAt))
	require.NoError(t, h.handleExpiration(c, RevenueCatEvent{OriginalTransactionID: sub.ID}))
	got, err = h.subscriptionDB.GetSubscriptionByID(c, sub.ID)
	require.NoError(t, err)
	require.Equal(t, "expired", got.Status)
}

func TestBillingIssuePreservesAccessAndGracePeriod(t *testing.T) {
	for _, status := range []string{"active", "expired"} {
		t.Run(status, func(t *testing.T) {
			h, db, c := newWebhookTest(t)
			expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
			sub := model.Subscription{ID: "original", Provider: model.SubscriptionProviderRevenueCat, ExternalSubscriptionID: "original", Status: status, ExpiresAt: &expiry}
			require.NoError(t, db.Create(&sub).Error)
			event := RevenueCatEvent{OriginalTransactionID: sub.ID}
			require.NoError(t, h.handleBillingIssue(c, event))
			got, err := h.subscriptionDB.GetSubscriptionByID(c, sub.ID)
			require.NoError(t, err)
			require.Equal(t, status, got.Status)
			require.True(t, expiry.Equal(*got.ExpiresAt))
			grace := expiry.Add(48 * time.Hour).UnixMilli()
			event.GracePeriodExpirationAtMs = &grace
			require.NoError(t, h.handleBillingIssue(c, event))
			got, err = h.subscriptionDB.GetSubscriptionByID(c, sub.ID)
			require.NoError(t, err)
			require.Equal(t, status, got.Status)
			if status == "active" {
				require.Equal(t, grace, got.ExpiresAt.UnixMilli())
			} else {
				require.True(t, expiry.Equal(*got.ExpiresAt))
			}
			// A delayed shorter grace period must not reduce access.
			shorter := expiry.UnixMilli()
			event.GracePeriodExpirationAtMs = &shorter
			require.NoError(t, h.handleBillingIssue(c, event))
			after, err := h.subscriptionDB.GetSubscriptionByID(c, sub.ID)
			require.NoError(t, err)
			require.True(t, got.ExpiresAt.Equal(*after.ExpiresAt))
		})
	}
}

func TestTransferUsesAliasArrays(t *testing.T) {
	h, db, c := newWebhookTest(t)
	require.NoError(t, db.Create(&uModel.User{ID: 42, Username: "destination", CircleID: 7}).Error)
	rows := []model.Subscription{
		{ID: "transaction", Provider: model.SubscriptionProviderRevenueCat, ExternalSubscriptionID: "transaction", ExternalCustomerID: "12", UserID: 12, CircleID: 2, Status: "active"},
		{ID: "$RCAnonymousID:source", Provider: model.SubscriptionProviderRevenueCat, ExternalSubscriptionID: "$RCAnonymousID:source", ExternalCustomerID: "$RCAnonymousID:source", UserID: 12, CircleID: 2, Status: "expired"},
		{ID: "stripe", Provider: model.SubscriptionProviderStripe, ExternalSubscriptionID: "stripe", ExternalCustomerID: "12", UserID: 12, CircleID: 2, Status: "active"},
		{ID: "unrelated", Provider: model.SubscriptionProviderRevenueCat, ExternalSubscriptionID: "unrelated", ExternalCustomerID: "99", UserID: 99, CircleID: 9, Status: "active"},
	}
	require.NoError(t, db.Create(&rows).Error)
	var event RevenueCatEvent
	require.NoError(t, json.Unmarshal([]byte(`{"type":"TRANSFER","transferred_from":["12","$RCAnonymousID:source"],"transferred_to":["$RCAnonymousID:destination","999","42"]}`), &event))
	// Replaying the transfer must be harmless.
	for i := 0; i < 2; i++ {
		require.NoError(t, h.handleTransfer(c, event))
	}
	for i, original := range rows {
		got, err := h.subscriptionDB.GetSubscriptionByID(c, original.ID)
		require.NoError(t, err)
		require.Equal(t, original.Status, got.Status)
		require.Equal(t, original.ExternalSubscriptionID, got.ExternalSubscriptionID)
		if i < 2 {
			require.Equal(t, 42, got.UserID)
			require.Equal(t, 7, got.CircleID)
			require.Equal(t, "42", got.ExternalCustomerID)
		} else {
			require.Equal(t, original.UserID, got.UserID)
			require.Equal(t, original.CircleID, got.CircleID)
			require.Equal(t, original.ExternalCustomerID, got.ExternalCustomerID)
		}
	}
}
