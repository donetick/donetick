package events

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/internal/webhook"
	"donetick.com/core/logging"
	"go.uber.org/zap"
)

// maxWebhookSendTimeout bounds a single delivery even if the configured
// client timeout is generous, so one slow receiver cannot stall the queue.
const maxWebhookSendTimeout = 30 * time.Second

type EventType string

const (
	EventTypeUnknown      EventType = ""
	EventTypeTaskCreated  EventType = "task.created"
	EventTypeTaskReminder EventType = "task.reminder"
	// EventTypeTaskUpdated    EventType = "task.updated"
	EventTypeTaskCompleted    EventType = "task.completed"
	EventTypeSubTaskCompleted EventType = "subtask.completed"
	// EventTypeTaskReassigned EventType = "task.reassigned"
	EventTypeTaskSkipped  EventType = "task.skipped"
	EventTypeThingChanged EventType = "thing.changed"
)

type Event struct {
	Type      EventType   `json:"type"`
	URL       string      `json:"-"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type ChoreData struct {
	Chore       *chModel.Chore `json:"chore"`
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name"`
	Note        string         `json:"note"`
}

type EventsProducer struct {
	client        *webhook.Client
	policy        webhook.Policy
	throttle      *webhook.Throttle
	signingSecret string
	queue         chan Event
	logger        *zap.SugaredLogger
}

func (p *EventsProducer) Start(ctx context.Context) {

	p.logger = logging.FromContext(ctx)

	go func() {
		for event := range p.queue {
			p.processEvent(event)
		}
	}()
}

func NewEventsProducer(cfg *config.Config) *EventsProducer {
	policy := webhook.PolicyFromConfig(cfg)
	return &EventsProducer{
		client:        webhook.NewClient(policy, cfg.WebhookConfig.Timeout, cfg.WebhookConfig.MaxResponseBytes),
		policy:        policy,
		throttle:      webhook.NewThrottle(cfg.WebhookConfig.RateLimitPerMinute, cfg.WebhookConfig.FailureThreshold, cfg.WebhookConfig.FailureCooldown),
		signingSecret: cfg.WebhookConfig.SigningSecret,
		queue:         make(chan Event, cfg.WebhookConfig.QueueSize),
	}
}

func (p *EventsProducer) publishEvent(event Event) {
	select {
	case p.queue <- event:
		// Successfully added to queue
	default:
		log.Println("Webhook queue is full, dropping event")
	}
}

func (p *EventsProducer) processEvent(event Event) {
	p.logger.Debugw("Sending webhook event", "type", event.Type, "url", event.URL)

	// A URL is checked before it is stored, but it may predate that check
	// or have been loosened since, and the host behind it can change. The
	// syntax check is cheap; the address behind the name is re-checked by
	// the client when it connects.
	if err := p.policy.ValidateURLSyntax(event.URL); err != nil {
		p.logger.Warnw("Skipping webhook event, destination is not allowed",
			"type", event.Type, "url", event.URL, "error", err)
		return
	}

	if allowed, reason := p.throttle.Allow(event.URL); !allowed {
		p.logger.Warnw("Dropping webhook event", "type", event.Type, "url", event.URL, "reason", reason)
		return
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.logger.Errorw("Failed to marshal webhook event", "error", err)
		return
	}

	headers := map[string]string{
		webhook.HeaderEvent:    string(event.Type),
		webhook.HeaderDelivery: webhook.NewDeliveryID(),
	}
	if p.signingSecret != "" {
		now := time.Now().UTC()
		headers[webhook.HeaderTimestamp] = strconv.FormatInt(now.Unix(), 10)
		headers[webhook.HeaderSignature] = webhook.Sign(p.signingSecret, now, eventJSON)
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxWebhookSendTimeout)
	defer cancel()

	resp, err := p.client.PostJSON(ctx, event.URL, eventJSON, headers)
	if err != nil {
		p.recordFailure(event, "error", err)
		return
	}

	if resp.StatusCode >= 400 {
		p.recordFailure(event, "status", resp.StatusCode)
		return
	}

	p.throttle.RecordSuccess(event.URL)
}

// recordFailure logs a failed delivery and parks the destination once it has
// failed often enough in a row.
func (p *EventsProducer) recordFailure(event Event, detailKey string, detail interface{}) {
	p.logger.Debugw("Failed to send webhook event", "type", event.Type, "url", event.URL, detailKey, detail)
	if p.throttle.RecordFailure(event.URL) {
		p.logger.Warnw("Pausing webhook deliveries after repeated failures", "url", event.URL)
	}
}

func (p *EventsProducer) ChoreCreated(ctx context.Context, webhookURL *string, chore *chModel.Chore, creator *uModel.User) {
	if webhookURL == nil || choreHiddenFromCircle(chore) {
		p.logger.Debug("No subscribers for circle, skipping webhook")
		return
	}

	event := Event{
		Type:      EventTypeTaskCreated,
		URL:       *webhookURL,
		Timestamp: time.Now().UTC(),
		Data: ChoreData{
			Chore:       chore,
			Username:    creator.Username,
			DisplayName: creator.DisplayName,
		},
	}
	p.publishEvent(event)
}

func (p *EventsProducer) ChoreCompleted(ctx context.Context, webhookURL *string, chore *chModel.Chore, performer *uModel.User) {
	if webhookURL == nil || choreHiddenFromCircle(chore) {
		p.logger.Debug("No subscribers for circle, skipping webhook")
		return
	}

	event := Event{
		Type:      EventTypeTaskCompleted,
		URL:       *webhookURL,
		Timestamp: time.Now().UTC(),
		Data: ChoreData{Chore: chore,
			Username:    performer.Username,
			DisplayName: performer.DisplayName,
		},
	}
	p.publishEvent(event)
}

func (p *EventsProducer) ChoreSkipped(ctx context.Context, webhookURL *string, chore *chModel.Chore, performer *uModel.User) {
	if webhookURL == nil || choreHiddenFromCircle(chore) {
		p.logger.Debug("No Webhook URL for circle, skipping webhook")
		return
	}

	event := Event{
		Type:      EventTypeTaskSkipped,
		URL:       *webhookURL,
		Timestamp: time.Now().UTC(),
		Data: ChoreData{Chore: chore,
			Username:    performer.Username,
			DisplayName: performer.DisplayName,
		},
	}
	p.publishEvent(event)
}

func (p *EventsProducer) NotificationEvent(ctx context.Context, url string, event interface{}) {
	// print the event and the url :
	p.logger.Debug("Sending notification event")

	p.publishEvent(Event{
		URL:       url,
		Type:      EventTypeTaskReminder,
		Timestamp: time.Now().UTC(),
		Data:      event,
	})
}

func (p *EventsProducer) ThingsUpdated(ctx context.Context, url *string, data interface{}) {
	if url == nil {
		p.logger.Debug("No subscribers for circle, skipping webhook")
		return
	}
	p.publishEvent(Event{
		URL:       *url,
		Type:      EventTypeThingChanged,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
}

func (p *EventsProducer) SubtaskUpdated(ctx context.Context, url *string, data interface{}) {
	if url == nil {
		p.logger.Debug("No subscribers for circle, skipping webhook")
		return
	}
	p.publishEvent(Event{
		URL:       *url,
		Type:      EventTypeSubTaskCompleted,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
}

// choreHiddenFromCircle reports whether a chore lives in a project that is not
// visible to the whole circle. The circle webhook is a circle-wide integration, so
// such chores are never sent to it. An unloaded project counts as hidden.
func choreHiddenFromCircle(chore *chModel.Chore) bool {
	if chore == nil {
		return true
	}
	if chore.ProjectID != nil && chore.Project == nil {
		return true
	}
	return chore.Project != nil && chore.Project.IsPrivate
}
