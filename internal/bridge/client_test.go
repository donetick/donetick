package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, srv *httptest.Server, timeout time.Duration) *Client {
	t.Helper()
	return NewClient(Config{
		Enabled:       true,
		BaseURL:       srv.URL,
		InstanceID:    "inst-1",
		InstanceToken: "dtb_live_test_secret",
		Timeout:       timeout,
	})
}

func TestClient_RegisterDevice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/devices" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dtb_live_test_secret" {
			t.Fatalf("missing/incorrect Authorization header: %q", got)
		}
		var body registerDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Token != "raw-fcm-token" {
			t.Fatalf("unexpected token in request: %q", body.Token)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(registerDeviceResponse{BridgeDeviceID: "bd-123", Active: true})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	res, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{
		LocalDeviceID: "local-1",
		Token:         "raw-fcm-token",
		Platform:      "ios",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BridgeDeviceID != "bd-123" || !res.Active {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestClient_RegisterDevice_DeviceLimitReached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "DEVICE_LIMIT_REACHED", "message": "limit reached", "requestId": "r1"},
		})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{LocalDeviceID: "l", Token: "t", Platform: "ios"})
	if !errors.Is(err, ErrDeviceLimitReached) {
		t.Fatalf("expected ErrDeviceLimitReached, got %v", err)
	}
}

func TestClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := testClient(t, srv, 10*time.Millisecond)
	_, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{LocalDeviceID: "l", Token: "t", Platform: "ios"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable on timeout, got %v", err)
	}
}

func TestClient_BridgeUnavailable_ConnectionRefused(t *testing.T) {
	// A server that is immediately closed leaves the port refusing
	// connections, simulating "Bridge is down".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := NewClient(Config{Enabled: true, BaseURL: url, InstanceToken: "dtb_live_x", Timeout: time.Second})
	_, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{LocalDeviceID: "l", Token: "t", Platform: "ios"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable on connection refused, got %v", err)
	}
}

func TestClient_ServerError_MapsToUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "INTERNAL", "message": "boom", "requestId": "r2"},
		})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{LocalDeviceID: "l", Token: "t", Platform: "ios"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable for 5xx, got %v", err)
	}
}

func TestClient_InvalidTokenDeactivation_SendNotificationResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/notifications/send" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "idem-1" {
			t.Fatalf("missing idempotency key header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(sendNotificationResponse{
			RequestID: "req-1",
			Accepted:  1,
			Succeeded: 0,
			Failed:    1,
			Results: []struct {
				BridgeDeviceID string `json:"bridgeDeviceId"`
				Status         string `json:"status"`
			}{{BridgeDeviceID: "bd-1", Status: "invalid_token"}},
		})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	res, err := c.SendNotification(context.Background(), "idem-1",
		[]DeviceTarget{{BridgeDeviceID: "bd-1", Token: "raw-token"}},
		NotificationPayload{Title: "t", Body: "b"},
		NotificationOptions{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Results) != 1 || res.Results[0].Status != StatusInvalidToken {
		t.Fatalf("expected invalid_token target status, got %+v", res.Results)
	}
}

func TestClient_SendNotification_RequiresIdempotencyKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called without an idempotency key")
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.SendNotification(context.Background(), "", nil, NotificationPayload{}, NotificationOptions{})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestClient_Disabled(t *testing.T) {
	c := NewClient(Config{Enabled: false})
	_, err := c.RegisterDevice(context.Background(), RegisterDeviceInput{})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestClient_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "UNAUTHORIZED", "message": "authentication required", "requestId": "r3"},
		})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.InstanceStatus(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestIdempotencyKey_DeterministicAndStable(t *testing.T) {
	k1 := IdempotencyKey("chore-42", "2026-09-30T00:00:00Z")
	k2 := IdempotencyKey("chore-42", "2026-09-30T00:00:00Z")
	k3 := IdempotencyKey("chore-42", "2026-10-01T00:00:00Z")
	if k1 != k2 {
		t.Fatalf("expected same inputs to produce same key, got %q vs %q", k1, k2)
	}
	if k1 == k3 {
		t.Fatalf("expected different inputs to produce different keys")
	}
	if len(k1) == 0 {
		t.Fatalf("expected non-empty key")
	}
}

func TestClient_InstanceStatus_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instance/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"instanceId":   "i1",
			"instanceName": "Home",
			"enabled":      true,
			"owner":        map[string]any{"emailMasked": "m***@example.com", "emailVerified": true},
			"plan":         map[string]any{"name": "free", "subscriptionStatus": "active"},
			"usage": map[string]any{
				"notificationsUsed": 1, "notificationsLimit": 500,
				"periodStart": "2026-09-01T00:00:00Z", "periodEnd": "2026-10-01T00:00:00Z",
				"activeDevices": 1, "deviceLimit": 5,
			},
		})
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	status, err := c.InstanceStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.InstanceName != "Home" || status.PlanName != "free" || status.ActiveDevices != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
}
