package device

import (
	"context"
	"fmt"
	"testing"

	"donetick.com/core/config"
	errorx "donetick.com/core/internal/error"
	uModel "donetick.com/core/internal/user/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newDeviceTestRepository(t *testing.T) *DeviceRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.UserDeviceToken{}))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return NewDeviceRepository(db, cfg)
}

func TestRegisterDeviceTokenReRegistersSameDeviceWithoutConflict(t *testing.T) {
	repo := newDeviceTestRepository(t)
	c := context.Background()

	token := &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-1",
		Token:    "fcm-token-a",
		Platform: "ios",
	}
	require.NoError(t, repo.RegisterDeviceToken(c, token))

	// Re-registering the same device (e.g. app restart, token refresh) used to
	// hit a UNIQUE constraint on (user_id, device_id) because the old code
	// deactivated the existing row instead of updating it in place.
	reReg := &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-1",
		Token:    "fcm-token-b",
		Platform: "ios",
	}
	require.NoError(t, repo.RegisterDeviceToken(c, reReg))

	tokens, err := repo.GetUserDeviceTokens(c, 1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, "fcm-token-b", tokens[0].Token)
	require.True(t, tokens[0].IsActive)
}

func TestRegisterDeviceTokenDeactivatesStaleDeviceIDForSameToken(t *testing.T) {
	repo := newDeviceTestRepository(t)
	c := context.Background()

	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-old",
		Token:    "fcm-token-shared",
		Platform: "android",
	}))

	// Same push token shows up under a new device_id (e.g. reinstall).
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-new",
		Token:    "fcm-token-shared",
		Platform: "android",
	}))

	active, err := repo.GetActiveDeviceTokens(c, 1)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, "device-new", active[0].DeviceID)
}

func TestRegisterDeviceTokenEnforcesDeviceLimit(t *testing.T) {
	repo := newDeviceTestRepository(t)
	c := context.Background()

	for i := 0; i < MaxDevicesPerUser; i++ {
		require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
			UserID:   1,
			DeviceID: fmt.Sprintf("device-%d", i),
			Token:    fmt.Sprintf("fcm-token-%d", i),
			Platform: "ios",
		}))
	}

	err := repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-overflow",
		Token:    "fcm-token-overflow",
		Platform: "ios",
	})
	require.ErrorIs(t, err, errorx.ErrDeviceLimitExceeded)

	// Re-registering an already-active device must still succeed at the limit.
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-0",
		Token:    "fcm-token-0-refreshed",
		Platform: "ios",
	}))
}
