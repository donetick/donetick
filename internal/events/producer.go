package events

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"go.uber.org/zap"
)

const (
	METHOD_POST       = "POST"
	HEAD_CONTENT_TYPE = "Content-Type"
	CONTENT_TYPE_JSON = "application/json"
)

type EventType string

const (
	EventTypeUnknown EventType = ""
	// EventTypeTaskCreated    EventType = "task.created"
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
	client *http.Client
	queue  chan Event
	logger *zap.SugaredLogger
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
	return &EventsProducer{
		client: &http.Client{
			Timeout: cfg.WebhookConfig.Timeout,
		},
		queue: make(chan Event, cfg.WebhookConfig.QueueSize),
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

	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.logger.Errorw("Failed to marshal webhook event", "error", err)
		return
	}

	// Pring the event and the url:
	p.logger.Debug("Sending event to webhook", "url", event.URL, "event", event)
	p.logger.Debug("Event: ", event)

	req, err := http.NewRequest(METHOD_POST, event.URL, bytes.NewBuffer(eventJSON))
	if err != nil {
		p.logger.Errorw("Failed to create webhook request", "error", err)
		return
	}
	req.Header.Set(HEAD_CONTENT_TYPE, CONTENT_TYPE_JSON)

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Debugw("Failed to send webhook event", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		p.logger.Errorw("Webhook request failed", "status", resp.StatusCode)
		return
	}
}

func (p *EventsProducer) ChoreCompleted(ctx context.Context, webhookURL *string, chore *chModel.Chore, performer *uModel.User) {
	if webhookURL == nil {
		p.logger.Debug("No subscribers for circle, skipping webhook")
		return
	}

	event := Event{
		Type:      EventTypeTaskCompleted,
		URL:       *webhookURL,
		Timestamp: time.Now(),
		Data: ChoreData{Chore: chore,
			Username:    performer.Username,
			DisplayName: performer.DisplayName,
		},
	}
	p.publishEvent(event)
}

func (p *EventsProducer) ChoreSkipped(ctx context.Context, webhookURL *string, chore *chModel.Chore, performer *uModel.User) {
	if webhookURL == nil {
		p.logger.Debug("No Webhook URL for circle, skipping webhook")
		return
	}

	event := Event{
		Type:      EventTypeTaskSkipped,
		URL:       *webhookURL,
		Timestamp: time.Now(),
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
		Timestamp: time.Now(),
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
		Timestamp: time.Now(),
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
		Timestamp: time.Now(),
		Data:      data,
	})
}
