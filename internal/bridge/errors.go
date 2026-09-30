package bridge

import "errors"

// Sentinel errors returned by Client methods. Callers should use
// errors.Is against these rather than matching on error strings, since the
// underlying message is sanitized (no token/content) but may still vary.
var (
	// ErrUnavailable means the request could not reach Bridge at all, or
	// Bridge returned a server-side error (5xx) or timed out. Callers
	// should treat this as "try again later" and must not block local
	// operations (e.g. login, chore actions) on it.
	ErrUnavailable = errors.New("bridge: service unavailable")

	// ErrUnauthorized means the configured instance token was rejected
	// (missing, malformed, revoked, or rotated away). This is not
	// necessarily permanent (token may be reconfigured) but Core should
	// stop retrying with the same credential and surface a clear "token
	// invalid/revoked" state rather than treating it as transient.
	ErrUnauthorized = errors.New("bridge: instance credential rejected")

	// ErrValidation means Bridge rejected the request shape/contents
	// (e.g. bad platform, oversized payload). This indicates a Core bug,
	// not a transient condition.
	ErrValidation = errors.New("bridge: request rejected as invalid")

	// ErrDeviceLimitReached means the owner's active-device plan limit
	// was reached.
	ErrDeviceLimitReached = errors.New("bridge: device limit reached")

	// ErrQuotaExceeded means the owner's monthly notification quota was
	// reached.
	ErrQuotaExceeded = errors.New("bridge: notification quota exceeded")

	// ErrRateLimited means Bridge's abuse-prevention rate limiter
	// rejected the request. Transient; safe to retry after a delay.
	ErrRateLimited = errors.New("bridge: rate limited")

	// ErrConflict means a request with the same idempotency key is
	// already being processed by Bridge.
	ErrConflict = errors.New("bridge: request already in progress")

	// ErrNotFound means the referenced device/instance does not exist
	// (from Bridge's point of view).
	ErrNotFound = errors.New("bridge: not found")

	// ErrDisabled is returned by Client methods when the bridge is not
	// enabled in config, so callers can distinguish "nothing to do" from
	// a real failure without checking config themselves.
	ErrDisabled = errors.New("bridge: not enabled")
)

// errorCodeToErr maps Bridge's stable {"error":{"code":...}} envelope code
// (see donetick-bridge internal/httpapi/errors.go) to a sentinel error.
func errorCodeToErr(code string, status int) error {
	switch code {
	case "UNAUTHORIZED":
		return ErrUnauthorized
	case "VALIDATION_ERROR", "BAD_REQUEST":
		return ErrValidation
	case "DEVICE_LIMIT_REACHED":
		return ErrDeviceLimitReached
	case "QUOTA_EXCEEDED":
		return ErrQuotaExceeded
	case "RATE_LIMITED":
		return ErrRateLimited
	case "REQUEST_IN_PROGRESS", "CONFLICT":
		return ErrConflict
	case "NOT_FOUND":
		return ErrNotFound
	default:
		if status >= 500 {
			return ErrUnavailable
		}
		return ErrValidation
	}
}

// ErrorCategory classifies an error returned by a Client method into a
// small, loggable/displayable category -- safe to persist as
// devices.bridge_sync_status or surface in Core's own settings API,
// matching plan §15 "Optional bridge_sync_status/last error category".
type ErrorCategory string

const (
	CategoryNone          ErrorCategory = ""
	CategoryUnavailable   ErrorCategory = "unavailable"
	CategoryUnauthorized  ErrorCategory = "unauthorized"
	CategoryValidation    ErrorCategory = "validation_error"
	CategoryDeviceLimit   ErrorCategory = "device_limit_reached"
	CategoryQuotaExceeded ErrorCategory = "quota_exceeded"
	CategoryRateLimited   ErrorCategory = "rate_limited"
	CategoryConflict      ErrorCategory = "conflict"
	CategoryNotFound      ErrorCategory = "not_found"
	CategoryDisabled      ErrorCategory = "disabled"
	CategoryUnknown       ErrorCategory = "unknown"
)

// Classify returns a sanitized category for err, for logging/status display.
// It never includes any message text from err.
func Classify(err error) ErrorCategory {
	switch {
	case err == nil:
		return CategoryNone
	case errors.Is(err, ErrDisabled):
		return CategoryDisabled
	case errors.Is(err, ErrUnavailable):
		return CategoryUnavailable
	case errors.Is(err, ErrUnauthorized):
		return CategoryUnauthorized
	case errors.Is(err, ErrValidation):
		return CategoryValidation
	case errors.Is(err, ErrDeviceLimitReached):
		return CategoryDeviceLimit
	case errors.Is(err, ErrQuotaExceeded):
		return CategoryQuotaExceeded
	case errors.Is(err, ErrRateLimited):
		return CategoryRateLimited
	case errors.Is(err, ErrConflict):
		return CategoryConflict
	case errors.Is(err, ErrNotFound):
		return CategoryNotFound
	default:
		return CategoryUnknown
	}
}
