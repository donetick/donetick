package webhook

import "testing"

func TestValidateDiscordWebhookURL(t *testing.T) {
	valid := []string{
		"https://discord.com/api/webhooks/123456789/abcdefg",
		"https://discordapp.com/api/webhooks/123456789/abcdefg",
		"https://canary.discord.com/api/webhooks/123/xyz?wait=true",
	}
	for _, raw := range valid {
		if err := ValidateDiscordWebhookURL(raw); err != nil {
			t.Fatalf("expected %q to be accepted, got %v", raw, err)
		}
	}

	invalid := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"plain http", "http://discord.com/api/webhooks/1/x"},
		{"other host", "https://evil.example.com/api/webhooks/1/x"},
		{"lookalike host", "https://discord.com.evil.example.com/api/webhooks/1/x"},
		{"wrong path", "https://discord.com/api/users/@me"},
		{"path traversal", "https://discord.com/api/webhooks/../../internal"},
		{"credentials", "https://user:pass@discord.com/api/webhooks/1/x"},
		{"port", "https://discord.com:8080/api/webhooks/1/x"},
		{"internal address", "https://127.0.0.1/api/webhooks/1/x"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDiscordWebhookURL(tc.url); err == nil {
				t.Fatalf("expected %q to be rejected", tc.url)
			}
		})
	}
}
