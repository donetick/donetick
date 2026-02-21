package realtime

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

// SanitizedLogger provides structured logging with sensitive data sanitization
type SanitizedLogger struct {
	logger *zap.SugaredLogger
}

// NewSanitizedLogger creates a new sanitized logger
func NewSanitizedLogger(logger *zap.SugaredLogger) *SanitizedLogger {
	return &SanitizedLogger{logger: logger}
}

// sanitizeToken masks sensitive parts of JWT tokens for logging
func (sl *SanitizedLogger) sanitizeToken(token string) string {
	if token == "" {
		return ""
	}

	// Only show first 8 and last 4 characters of token
	if len(token) <= 12 {
		return strings.Repeat("*", len(token))
	}

	return token[:8] + strings.Repeat("*", len(token)-12) + token[len(token)-4:]
}

// sanitizeUserData removes or masks sensitive user information
func (sl *SanitizedLogger) sanitizeUserData(fields map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range fields {
		switch strings.ToLower(key) {
		case "token", "jwt", "password", "secret", "key":
			if str, ok := value.(string); ok {
				sanitized[key] = sl.sanitizeToken(str)
			} else {
				sanitized[key] = "***"
			}
		case "username", "email":
			if str, ok := value.(string); ok && str != "" {
				// Show first character and domain for emails, first 2 chars for usernames
				if strings.Contains(str, "@") {
					parts := strings.Split(str, "@")
					if len(parts) == 2 {
						sanitized[key] = string(parts[0][0]) + "***@" + parts[1]
					} else {
						sanitized[key] = "***"
					}
				} else {
					if len(str) <= 2 {
						sanitized[key] = strings.Repeat("*", len(str))
					} else {
						sanitized[key] = str[:2] + strings.Repeat("*", len(str)-2)
					}
				}
			} else {
				sanitized[key] = value
			}
		default:
			sanitized[key] = value
		}
	}

	return sanitized
}

// LogAuthSuccess logs successful authentication events
func (sl *SanitizedLogger) LogAuthSuccess(userID int, circleID int, remoteAddr string, userAgent string) {
	sl.logger.Infow("WebSocket authentication successful",
		"user_id", userID,
		"circle_id", circleID,
		"remote_addr", remoteAddr,
		"user_agent", userAgent,
		"timestamp", time.Now().UTC(),
	)
}

// LogAuthFailure logs authentication failures with sanitized data
func (sl *SanitizedLogger) LogAuthFailure(reason string, remoteAddr string, userAgent string, token string) {
	sl.logger.Warnw("WebSocket authentication failed",
		"reason", reason,
		"remote_addr", remoteAddr,
		"user_agent", userAgent,
		"token_preview", sl.sanitizeToken(token),
		"timestamp", time.Now().UTC(),
	)
}

// LogConnectionEvent logs WebSocket connection events
func (sl *SanitizedLogger) LogConnectionEvent(event string, userID int, circleID int, connectionID string, remoteAddr string) {
	sl.logger.Infow("WebSocket connection event",
		"event", event,
		"user_id", userID,
		"circle_id", circleID,
		"connection_id", connectionID,
		"remote_addr", remoteAddr,
		"timestamp", time.Now().UTC(),
	)
}

// LogSecurityEvent logs security-related events
func (sl *SanitizedLogger) LogSecurityEvent(event string, userID int, remoteAddr string, details map[string]interface{}) {
	sanitizedDetails := sl.sanitizeUserData(details)

	sl.logger.Warnw("Security event detected",
		"event", event,
		"user_id", userID,
		"remote_addr", remoteAddr,
		"details", sanitizedDetails,
		"timestamp", time.Now().UTC(),
	)
}

// LogRateLimitEvent logs rate limiting events
func (sl *SanitizedLogger) LogRateLimitEvent(userID int, remoteAddr string, limit int, current int) {
	sl.logger.Warnw("Rate limit exceeded",
		"user_id", userID,
		"remote_addr", remoteAddr,
		"limit", limit,
		"current_requests", current,
		"timestamp", time.Now().UTC(),
	)
}

// LogMessageValidation logs message validation failures
func (sl *SanitizedLogger) LogMessageValidation(userID int, connectionID string, reason string, messageType string) {
	sl.logger.Warnw("WebSocket message validation failed",
		"user_id", userID,
		"connection_id", connectionID,
		"reason", reason,
		"message_type", messageType,
		"timestamp", time.Now().UTC(),
	)
}

// LogPerformanceMetric logs performance-related metrics
func (sl *SanitizedLogger) LogPerformanceMetric(metric string, value interface{}, tags map[string]interface{}) {
	sanitizedTags := sl.sanitizeUserData(tags)

	sl.logger.Infow("Performance metric",
		"metric", metric,
		"value", value,
		"tags", sanitizedTags,
		"timestamp", time.Now().UTC(),
	)
}

// LogError logs errors with context sanitization
func (sl *SanitizedLogger) LogError(message string, err error, context map[string]interface{}) {
	sanitizedContext := sl.sanitizeUserData(context)

	sl.logger.Errorw(message,
		"error", err.Error(),
		"context", sanitizedContext,
		"timestamp", time.Now().UTC(),
	)
}
