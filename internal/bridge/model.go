// Package bridge is a client for the Donetick Bridge push-notification relay
// (see ../../../donetick-bridge/IMPLEMENTATION_PLAN.md). It never logs the
// instance token, raw FCM tokens, or notification content -- only status
// codes, sanitized error categories, and non-sensitive identifiers (see
// plan §5/§17, mirrored here for Core's own logging conventions).
package bridge

import "time"

// DeviceTarget is one send destination: a Bridge device ID plus the
// caller's current raw FCM token (Bridge recomputes and matches the
// fingerprint server-side; it never persists the token).
type DeviceTarget struct {
	BridgeDeviceID string
	Token          string
}

// NotificationPayload is the allow-listed notification content forwarded to
// Bridge. It exists only for the duration of one SendNotification call and
// must never be logged.
type NotificationPayload struct {
	Title string
	Body  string
	Data  map[string]string
}

// NotificationOptions mirrors Bridge's allow-listed send options.
type NotificationOptions struct {
	AndroidChannelID string
	Sound            string
	Badge            *int
}

// RegisterDeviceInput is the request body for POST /v1/devices.
type RegisterDeviceInput struct {
	LocalDeviceID string
	Token         string
	Platform      string
	AppVersion    string
	DeviceModel   string
}

// RegisterDeviceResult is the sanitized response from POST /v1/devices.
type RegisterDeviceResult struct {
	BridgeDeviceID string
	Active         bool
}

// UpdateDeviceInput is the request body for PUT /v1/devices/{id}. Nil
// fields are omitted from the request (matching Bridge's partial-update
// semantics).
type UpdateDeviceInput struct {
	Token       *string
	AppVersion  *string
	DeviceModel *string
	Active      *bool
}

// TargetStatus is one sanitized per-target outcome from a notification
// send. Status values used by Bridge today: "sent", "not_found",
// "inactive", "token_mismatch", "unknown", or a firebase.ErrorCategory
// string such as "invalid_token", "unavailable", "permission_denied",
// "resource_exhausted", "timeout", "malformed_request".
type TargetStatus struct {
	BridgeDeviceID string
	Status         string
}

// StatusInvalidToken is the status string Bridge returns for a target
// whose token Firebase reported as permanently unregistered/invalid. Core
// should deactivate the corresponding local device token when it sees this.
const StatusInvalidToken = "invalid_token"

// SendResult is the sanitized response from POST /v1/notifications/send.
type SendResult struct {
	RequestID string
	Accepted  int
	Succeeded int
	Failed    int
	Results   []TargetStatus
	QuotaUsed int
	QuotaMax  int
}

// InstanceStatus is the sanitized response from GET /v1/instance/status.
type InstanceStatus struct {
	InstanceID   string
	InstanceName string
	Enabled      bool

	OwnerEmailMasked   string
	OwnerEmailVerified bool

	PlanName           string
	SubscriptionStatus string

	NotificationsUsed  int
	NotificationsLimit int
	PeriodStart        time.Time
	PeriodEnd          time.Time
	ActiveDevices      int
	DeviceLimit        int
}
