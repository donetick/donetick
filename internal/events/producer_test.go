package events

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/webhook"
	"donetick.com/core/logging"
)

func TestRewardRedeemApprovedPayload(t *testing.T) {
	srv, captured := receiver(t, http.StatusOK)
	p := testProducer(t, nil)
	data := RewardRedemptionData{RedemptionID: 7, RewardID: 42, RecipientUserID: 2}
	p.RewardRedeemApproved(context.Background(), &srv.URL, data)
	select {
	case event := <-p.queue:
		p.processEvent(event)
	default:
		t.Fatal("no approval event queued")
	}
	reqs := captured()
	if len(reqs) != 1 {
		t.Fatalf("expected one delivery, got %d", len(reqs))
	}
	var body struct {
		Type      EventType            `json:"type"`
		Timestamp time.Time            `json:"timestamp"`
		Data      RewardRedemptionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(reqs[0].body), &body); err != nil {
		t.Fatal(err)
	}
	if body.Type != EventTypeRewardRedeemApproved || reqs[0].event != string(EventTypeRewardRedeemApproved) || body.Timestamp.IsZero() || body.Data != data {
		t.Fatalf("unexpected approval webhook: %s", reqs[0].body)
	}
}

func TestRewardRedeemApprovedWithoutWebhook(t *testing.T) {
	p := testProducer(t, nil)
	empty := ""
	for _, url := range []*string{nil, &empty} {
		p.RewardRedeemApproved(context.Background(), url, RewardRedemptionData{})
	}
	select {
	case event := <-p.queue:
		t.Fatalf("unexpected event without webhook URL: %+v", event)
	default:
	}
}

type capturedRequest struct {
	body      string
	signature string
	timestamp string
	event     string
	delivery  string
}

// receiver is a stand-in webhook endpoint. It listens on loopback, so tests
// that expect a delivery have to configure the producer to allow loopback.
func receiver(t *testing.T, status int) (*httptest.Server, func() []capturedRequest) {
	t.Helper()

	var (
		mu   sync.Mutex
		reqs []capturedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		reqs = append(reqs, capturedRequest{
			body:      string(body),
			signature: r.Header.Get(webhook.HeaderSignature),
			timestamp: r.Header.Get(webhook.HeaderTimestamp),
			event:     r.Header.Get(webhook.HeaderEvent),
			delivery:  r.Header.Get(webhook.HeaderDelivery),
		})
		mu.Unlock()

		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedRequest(nil), reqs...)
	}
}

func testProducer(t *testing.T, mutate func(*config.WebhookConfig)) *EventsProducer {
	t.Helper()

	cfg := &config.Config{WebhookConfig: config.WebhookConfig{
		Timeout:            2 * time.Second,
		QueueSize:          10,
		AllowLoopback:      true,
		MaxResponseBytes:   4096,
		RateLimitPerMinute: 60,
		FailureThreshold:   3,
		FailureCooldown:    time.Minute,
	}}
	if mutate != nil {
		mutate(&cfg.WebhookConfig)
	}

	p := NewEventsProducer(cfg)
	p.logger = logging.DefaultLogger()
	return p
}

func TestProcessEventDelivers(t *testing.T) {
	srv, captured := receiver(t, http.StatusOK)
	p := testProducer(t, func(c *config.WebhookConfig) { c.SigningSecret = "s3cret" })

	p.processEvent(Event{Type: EventTypeTaskCreated, URL: srv.URL, Timestamp: time.Now().UTC()})

	reqs := captured()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(reqs))
	}
	got := reqs[0]
	if got.event != string(EventTypeTaskCreated) {
		t.Errorf("event header = %q", got.event)
	}
	if got.delivery == "" {
		t.Error("missing delivery id header")
	}
	if got.timestamp == "" || !strings.HasPrefix(got.signature, "t=") || !strings.Contains(got.signature, ",v1=") {
		t.Errorf("signature = %q, timestamp = %q", got.signature, got.timestamp)
	}
	if !strings.Contains(got.body, `"type":"task.created"`) {
		t.Errorf("body = %q", got.body)
	}
}

func TestProcessEventDoesNotSignWithoutSecret(t *testing.T) {
	srv, captured := receiver(t, http.StatusOK)
	p := testProducer(t, nil)

	p.processEvent(Event{Type: EventTypeTaskCreated, URL: srv.URL, Timestamp: time.Now().UTC()})

	reqs := captured()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(reqs))
	}
	if reqs[0].signature != "" {
		t.Errorf("expected no signature header, got %q", reqs[0].signature)
	}
}

func TestProcessEventSkipsBlockedDestination(t *testing.T) {
	srv, captured := receiver(t, http.StatusOK)
	// Loopback is not allowed here, and the test server is on loopback.
	p := testProducer(t, func(c *config.WebhookConfig) { c.AllowLoopback = false })

	p.processEvent(Event{Type: EventTypeTaskCreated, URL: srv.URL, Timestamp: time.Now().UTC()})
	p.processEvent(Event{Type: EventTypeTaskCreated, URL: "http://169.254.169.254/latest/meta-data/", Timestamp: time.Now().UTC()})
	p.processEvent(Event{Type: EventTypeTaskCreated, URL: "file:///etc/passwd", Timestamp: time.Now().UTC()})

	if reqs := captured(); len(reqs) != 0 {
		t.Fatalf("expected no deliveries, got %d", len(reqs))
	}
}

func TestProcessEventRespectsRateLimit(t *testing.T) {
	srv, captured := receiver(t, http.StatusOK)
	p := testProducer(t, func(c *config.WebhookConfig) { c.RateLimitPerMinute = 2 })

	for i := 0; i < 5; i++ {
		p.processEvent(Event{Type: EventTypeTaskCreated, URL: srv.URL, Timestamp: time.Now().UTC()})
	}

	if reqs := captured(); len(reqs) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(reqs))
	}
}

func TestProcessEventParksFailingDestination(t *testing.T) {
	srv, captured := receiver(t, http.StatusInternalServerError)
	p := testProducer(t, func(c *config.WebhookConfig) { c.FailureThreshold = 2 })

	for i := 0; i < 5; i++ {
		p.processEvent(Event{Type: EventTypeTaskCreated, URL: srv.URL, Timestamp: time.Now().UTC()})
	}

	// Two attempts fail, then the destination is parked for the cooldown.
	if reqs := captured(); len(reqs) != 2 {
		t.Fatalf("expected 2 attempts before the cooldown, got %d", len(reqs))
	}
}
