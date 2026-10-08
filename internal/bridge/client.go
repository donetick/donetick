package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"
)

// Config is the subset of Core's bridge.* configuration a Client needs.
// Kept independent of donetick.com/core/config so this package has no
// import-cycle risk and is trivially unit testable.
type Config struct {
	Enabled       bool
	BaseURL       string
	InstanceID    string
	InstanceToken string
	Timeout       time.Duration
}

// Client talks to a Donetick Bridge instance over HTTP. It is safe for
// concurrent use. It never logs the instance token, raw FCM tokens, or
// notification content -- callers passing those values are only ever used
// to build the HTTP request body/headers, never written to a logger.
type Client struct {
	cfg   Config
	http  *http.Client
	nowFn func() time.Time
}

// NewClient builds a Client from cfg. If cfg.Timeout is zero, a
// conservative default is used so a hung Bridge instance can never block a
// Core request indefinitely.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		cfg:   cfg,
		http:  &http.Client{Timeout: timeout},
		nowFn: time.Now,
	}
}

// Enabled reports whether this client is configured to talk to Bridge.
func (c *Client) Enabled() bool {
	return c.cfg.Enabled && c.cfg.BaseURL != "" && c.cfg.InstanceToken != ""
}

type apiErrorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

// doJSON performs an authenticated JSON request against Bridge and decodes
// a successful response into out (if non-nil). It classifies failures into
// the sentinel errors in errors.go. It never logs the request/response
// body -- the body may contain notification content or a fresh instance
// token, both of which are privacy-sensitive.
func (c *Client) doJSON(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}

	var reqBody bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%w: encode request: %v", ErrValidation, err)
		}
		reqBody = *bytes.NewReader(b)
	}

	url := c.cfg.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, &reqBody)
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrValidation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.InstanceToken)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Covers timeouts, connection refused/reset, DNS failure, and
		// context cancellation/deadline -- all "Bridge unreachable right
		// now" from Core's point of view.
		return fmt.Errorf("%w: %v", ErrUnavailable, classifyTransportErr(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		if resp.StatusCode == http.StatusNoContent {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("%w: decode response: %v", ErrUnavailable, err)
		}
		return nil
	}

	var env apiErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env) // best-effort; body may not be the error envelope
	return errorCodeToErr(env.Error.Code, resp.StatusCode)
}

// classifyTransportErr returns a short, safe-to-log transport-error
// description without leaking request details (e.g. it drops the full URL
// that net/http errors normally embed, since the URL could theoretically
// carry sensitive query data on some future endpoint).
func classifyTransportErr(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return "connection_error"
}

// --- Device endpoints --------------------------------------------------

type registerDeviceRequest struct {
	LocalDeviceID string `json:"localDeviceId"`
	Token         string `json:"token"`
	Platform      string `json:"platform"`
	AppVersion    string `json:"appVersion"`
	DeviceModel   string `json:"deviceModel"`
}

type registerDeviceResponse struct {
	BridgeDeviceID string `json:"bridgeDeviceId"`
	Active         bool   `json:"active"`
}

// RegisterDevice implements POST /v1/devices.
func (c *Client) RegisterDevice(ctx context.Context, in RegisterDeviceInput) (*RegisterDeviceResult, error) {
	var resp registerDeviceResponse
	err := c.doJSON(ctx, http.MethodPost, "/v1/devices", registerDeviceRequest{
		LocalDeviceID: in.LocalDeviceID,
		Token:         in.Token,
		Platform:      in.Platform,
		AppVersion:    in.AppVersion,
		DeviceModel:   in.DeviceModel,
	}, "", &resp)
	if err != nil {
		return nil, err
	}
	return &RegisterDeviceResult{BridgeDeviceID: resp.BridgeDeviceID, Active: resp.Active}, nil
}

type updateDeviceRequest struct {
	Token       *string `json:"token,omitempty"`
	AppVersion  *string `json:"appVersion,omitempty"`
	DeviceModel *string `json:"deviceModel,omitempty"`
	Active      *bool   `json:"active,omitempty"`
}

// UpdateDevice implements PUT /v1/devices/{bridgeDeviceId}.
func (c *Client) UpdateDevice(ctx context.Context, bridgeDeviceID string, in UpdateDeviceInput) error {
	path := "/v1/devices/" + bridgeDeviceID
	return c.doJSON(ctx, http.MethodPut, path, updateDeviceRequest{
		Token: in.Token, AppVersion: in.AppVersion, DeviceModel: in.DeviceModel, Active: in.Active,
	}, "", nil)
}

// DeactivateDevice implements DELETE /v1/devices/{bridgeDeviceId}.
func (c *Client) DeactivateDevice(ctx context.Context, bridgeDeviceID string) error {
	path := "/v1/devices/" + bridgeDeviceID
	return c.doJSON(ctx, http.MethodDelete, path, nil, "", nil)
}

// --- Notification send ---------------------------------------------------

type sendNotificationRequest struct {
	Targets      []sendTarget `json:"targets"`
	Notification sendPayload  `json:"notification"`
	Options      sendOptions  `json:"options"`
}

type sendTarget struct {
	BridgeDeviceID string `json:"bridgeDeviceId"`
	Token          string `json:"token"`
}

type sendPayload struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data"`
}

type sendOptions struct {
	AndroidChannelID string `json:"androidChannelId"`
	Sound            string `json:"sound"`
	Badge            *int   `json:"badge"`
}

type sendNotificationResponse struct {
	RequestID string `json:"requestId"`
	Accepted  int    `json:"accepted"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	Results   []struct {
		BridgeDeviceID string `json:"bridgeDeviceId"`
		Status         string `json:"status"`
	} `json:"results"`
	Quota struct {
		Used  int `json:"used"`
		Limit int `json:"limit"`
	} `json:"quota"`
}

// SendNotification implements POST /v1/notifications/send. idempotencyKey
// must be stable across retries of the *same* logical send (see
// notifier.go's scheduler-retry requirement) -- callers must not generate a
// fresh random key per retry attempt, or each retry consumes additional
// Bridge quota.
func (c *Client) SendNotification(ctx context.Context, idempotencyKey string, targets []DeviceTarget, payload NotificationPayload, opts NotificationOptions) (*SendResult, error) {
	if idempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency key required", ErrValidation)
	}
	req := sendNotificationRequest{
		Notification: sendPayload{Title: payload.Title, Body: payload.Body, Data: payload.Data},
		Options: sendOptions{
			AndroidChannelID: opts.AndroidChannelID,
			Sound:            opts.Sound,
			Badge:            opts.Badge,
		},
	}
	for _, t := range targets {
		req.Targets = append(req.Targets, sendTarget{BridgeDeviceID: t.BridgeDeviceID, Token: t.Token})
	}

	var resp sendNotificationResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/notifications/send", req, idempotencyKey, &resp); err != nil {
		return nil, err
	}

	out := &SendResult{
		RequestID: resp.RequestID,
		Accepted:  resp.Accepted,
		Succeeded: resp.Succeeded,
		Failed:    resp.Failed,
		QuotaUsed: resp.Quota.Used,
		QuotaMax:  resp.Quota.Limit,
	}
	for _, r := range resp.Results {
		out.Results = append(out.Results, TargetStatus{BridgeDeviceID: r.BridgeDeviceID, Status: r.Status})
	}
	return out, nil
}

// --- Instance status -------------------------------------------------------

type instanceStatusResponse struct {
	InstanceID   string `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	Enabled      bool   `json:"enabled"`
	Owner        struct {
		EmailMasked   string `json:"emailMasked"`
		EmailVerified bool   `json:"emailVerified"`
	} `json:"owner"`
	Plan struct {
		Name               string `json:"name"`
		SubscriptionStatus string `json:"subscriptionStatus"`
	} `json:"plan"`
	Usage struct {
		NotificationsUsed  int       `json:"notificationsUsed"`
		NotificationsLimit int       `json:"notificationsLimit"`
		PeriodStart        time.Time `json:"periodStart"`
		PeriodEnd          time.Time `json:"periodEnd"`
		ActiveDevices      int       `json:"activeDevices"`
		DeviceLimit        int       `json:"deviceLimit"`
	} `json:"usage"`
}

// InstanceStatus implements GET /v1/instance/status -- the main call
// Core's own settings endpoint (internal/bridge/settings, see
// cmd wiring) uses to report connection/plan/usage state to the frontend.
func (c *Client) InstanceStatus(ctx context.Context) (*InstanceStatus, error) {
	var resp instanceStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/instance/status", nil, "", &resp); err != nil {
		return nil, err
	}
	return &InstanceStatus{
		InstanceID:         resp.InstanceID,
		InstanceName:       resp.InstanceName,
		Enabled:            resp.Enabled,
		OwnerEmailMasked:   resp.Owner.EmailMasked,
		OwnerEmailVerified: resp.Owner.EmailVerified,
		PlanName:           resp.Plan.Name,
		SubscriptionStatus: resp.Plan.SubscriptionStatus,
		NotificationsUsed:  resp.Usage.NotificationsUsed,
		NotificationsLimit: resp.Usage.NotificationsLimit,
		PeriodStart:        resp.Usage.PeriodStart,
		PeriodEnd:          resp.Usage.PeriodEnd,
		ActiveDevices:      resp.Usage.ActiveDevices,
		DeviceLimit:        resp.Usage.DeviceLimit,
	}, nil
}

// IdempotencyKey deterministically derives a Bridge idempotency key for one
// logical notification send, so scheduler retries of the same logical send
// reuse the same key and avoid another quota charge. Inputs should uniquely
// identify the logical send: e.g. a chore ID/notification-target ID plus
// the scheduled time. A monotonically-increasing attemptEpoch may be
// included by callers that intentionally want a *new* logical send (e.g. a
// recurring chore's next due date) to get a new key, while retries of the
// *same* attempt reuse it -- see internal/notifier's caller for how these
// are combined.
func IdempotencyKey(parts ...string) string {
	h := fnv.New64a()
	for i, p := range parts {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(p))
	}
	return "core_" + strconv.FormatUint(h.Sum64(), 36)
}
