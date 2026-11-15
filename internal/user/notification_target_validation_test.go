package user

import (
	"strings"
	"testing"

	nModel "donetick.com/core/internal/notifier/model"
)

func TestValidateNotificationTarget(t *testing.T) {
	cases := []struct {
		name     string
		platform nModel.NotificationPlatform
		target   string
		wantErr  bool
	}{
		{"telegram chat id", nModel.NotificationPlatformTelegram, "123456789", false},
		{"pushover key", nModel.NotificationPlatformPushover, "uQiRzpo4DXghDmr9QzzfQu27cmVRsG", false},
		{"discord webhook", nModel.NotificationPlatformDiscord, "https://discord.com/api/webhooks/123/abc", false},
		{"empty", nModel.NotificationPlatformTelegram, "", true},
		{"whitespace", nModel.NotificationPlatformTelegram, "   ", true},
		{"too long", nModel.NotificationPlatformTelegram, strings.Repeat("a", maxNotificationTargetLength+1), true},
		// The Discord target is posted to by the server, so it has to be a
		// Discord webhook and not an arbitrary internal URL.
		{"discord pointing elsewhere", nModel.NotificationPlatformDiscord, "https://evil.example.com/api/webhooks/1/x", true},
		{"discord pointing internal", nModel.NotificationPlatformDiscord, "http://127.0.0.1:2375/containers/json", true},
		{"discord not a url", nModel.NotificationPlatformDiscord, "123456789", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNotificationTarget(tc.platform, tc.target)
			if tc.wantErr && err == nil {
				t.Fatalf("expected %q to be rejected", tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected %q to be accepted, got %v", tc.target, err)
			}
		})
	}
}
