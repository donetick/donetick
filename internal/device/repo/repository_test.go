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

func TestRegisterDeviceTokenAllowsTokenReplacementAtDeviceLimit(t *testing.T) {
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

	// Reusing a token under a new device ID replaces one active row, so it does
	// not consume a sixth slot and must remain allowed at the limit.
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "replacement-device",
		Token:    "fcm-token-0",
		Platform: "ios",
	}))

	active, err := repo.GetActiveDeviceTokens(c, 1)
	require.NoError(t, err)
	require.Len(t, active, MaxDevicesPerUser)

	var oldDeviceActive, replacementActive bool
	for _, token := range active {
		oldDeviceActive = oldDeviceActive || token.DeviceID == "device-0"
		replacementActive = replacementActive || token.DeviceID == "replacement-device"
	}
	require.False(t, oldDeviceActive)
	require.True(t, replacementActive)
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

func TestRegisterDeviceTokenDeactivatesSameTokenForOtherUsers(t *testing.T) {
	repo := newDeviceTestRepository(t)
	c := context.Background()

	// User 1 signs in on a phone and never unregisters cleanly (expired
	// session, app force-deleted, logout while offline), so their row keeps
	// this device's FCM token.
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "shared-device",
		Token:    "fcm-token-shared",
		Platform: "android",
	}))

	// User 2 then signs in on the same phone. An FCM token addresses one app
	// install, so user 1's row must be deactivated — otherwise both rows stay
	// active with the same token and user 2's phone receives user 1's chore
	// notifications.
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   2,
		DeviceID: "shared-device",
		Token:    "fcm-token-shared",
		Platform: "android",
	}))

	stale, err := repo.GetActiveDeviceTokens(c, 1)
	require.NoError(t, err)
	require.Empty(t, stale, "previous user must no longer be an active push target")

	current, err := repo.GetActiveDeviceTokens(c, 2)
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, "fcm-token-shared", current[0].Token)
}

func TestDeactivateTokensFreesDeviceSlots(t *testing.T) {
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

	// FCM reports two of them as permanently gone (uninstalled apps).
	affected, err := repo.DeactivateTokens(c, []string{"fcm-token-0", "fcm-token-1"})
	require.NoError(t, err)
	require.EqualValues(t, 2, affected)

	count, err := repo.GetActiveDeviceCount(c, 1)
	require.NoError(t, err)
	require.EqualValues(t, MaxDevicesPerUser-2, count)

	// The freed slots are usable again, instead of dead tokens permanently
	// locking the user out of registering a device they actually use.
	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-new",
		Token:    "fcm-token-new",
		Platform: "ios",
	}))

	// Deactivating nothing is a no-op, not an error.
	affected, err = repo.DeactivateTokens(c, nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, affected)
}

func TestUpdateDeviceTokenActivityReportsMissingRegistration(t *testing.T) {
	repo := newDeviceTestRepository(t)
	c := context.Background()

	require.NoError(t, repo.RegisterDeviceToken(c, &uModel.UserDeviceToken{
		UserID:   1,
		DeviceID: "device-1",
		Token:    "fcm-token-a",
		Platform: "ios",
	}))

	require.NoError(t, repo.UpdateDeviceTokenActivity(c, 1, "device-1"))

	// The client uses this as a launch-time heartbeat in place of re-POSTing
	// an unchanged token, so an unknown device must be distinguishable from a
	// successful check-in or the client would assume it is still registered.
	err := repo.UpdateDeviceTokenActivity(c, 1, "device-unknown")
	require.ErrorIs(t, err, errorx.ErrDeviceNotFound)

	// Another user's device is equally not found.
	err = repo.UpdateDeviceTokenActivity(c, 2, "device-1")
	require.ErrorIs(t, err, errorx.ErrDeviceNotFound)
}
