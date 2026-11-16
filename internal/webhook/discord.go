package webhook

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// discordWebhookHosts are the hosts Discord issues webhook URLs on.
var discordWebhookHosts = map[string]struct{}{
	"discord.com":        {},
	"discordapp.com":     {},
	"ptb.discord.com":    {},
	"canary.discord.com": {},
}

// ValidateDiscordWebhookURL checks that a Discord notification target really
// is a Discord webhook. The target is user-supplied and the server posts to
// it, so without this it is just another way to aim the server at an
// arbitrary host, and no policy flag would make that one legitimate.
func ValidateDiscordWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%w: Discord webhook URL is empty", ErrInvalidURL)
	}
	if len(raw) > maxURLLength {
		return fmt.Errorf("%w: URL is longer than %d characters", ErrInvalidURL, maxURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidURL, err.Error())
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w: a Discord webhook URL must start with https://", ErrInvalidURL)
	}
	if u.User != nil {
		return fmt.Errorf("%w: credentials in the URL are not allowed", ErrInvalidURL)
	}
	if _, ok := discordWebhookHosts[strings.ToLower(u.Hostname())]; !ok {
		return fmt.Errorf("%w: expected a discord.com webhook URL", ErrInvalidURL)
	}
	if u.Port() != "" {
		return fmt.Errorf("%w: a Discord webhook URL has no port", ErrInvalidURL)
	}
	escaped := u.EscapedPath()
	// Compare against the cleaned path too, so "/api/webhooks/../admin"
	// cannot pass the prefix check and then be normalized by the receiver.
	if !strings.HasPrefix(escaped, "/api/webhooks/") || path.Clean(escaped) != strings.TrimSuffix(escaped, "/") {
		return fmt.Errorf("%w: expected a Discord webhook URL under /api/webhooks/", ErrInvalidURL)
	}
	return nil
}
