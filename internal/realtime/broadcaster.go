package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	uModel "donetick.com/core/internal/user/model"
)

// EventBroadcaster handles broadcasting events to appropriate connections
type EventBroadcaster struct {
	service *RealTimeService
	config  *config.Config
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster(service *RealTimeService, config *config.Config) *EventBroadcaster {
	return &EventBroadcaster{
		service: service,
		config:  config,
	}
}

// BroadcastChoreCreated broadcasts a chore creation event
func (b *EventBroadcaster) BroadcastChoreCreated(chore *chModel.Chore, user *uModel.User) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreCreatedEvent(chore, user)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastChoreUpdated broadcasts a chore update event
func (b *EventBroadcaster) BroadcastChoreUpdated(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreUpdatedEvent(chore, user, changes, note)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastChoreDeleted broadcasts a chore deletion event
func (b *EventBroadcaster) BroadcastChoreDeleted(choreID int, choreName string, circleID int, user *uModel.User) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreDeletedEvent(choreID, choreName, circleID, user)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(circleID, event)
}

// BroadcastChoreCompleted broadcasts a chore completion event
func (b *EventBroadcaster) BroadcastChoreCompleted(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreCompletedEvent(chore, user, history, note)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastChoreStarted broadcasts a chore start event
func (b *EventBroadcaster) BroadcastChoreStatus(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreStatusChangedEvent(chore, user, changes, nil)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastChoreSkipped broadcasts a chore skip event
func (b *EventBroadcaster) BroadcastChoreSkipped(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreSkippedEvent(chore, user, history, note)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastSubtaskUpdated broadcasts a subtask update event
func (b *EventBroadcaster) BroadcastSubtaskUpdated(choreID, subtaskID int, completedAt *time.Time, user *uModel.User, circleID int) {
	if !b.service.config.Enabled {
		return
	}

	event := NewSubtaskUpdatedEvent(choreID, subtaskID, completedAt, user, circleID)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(circleID, event)
}

// BroadcastSubtaskCompleted broadcasts a subtask completion event
func (b *EventBroadcaster) BroadcastSubtaskCompleted(choreID, subtaskID int, completedAt *time.Time, user *uModel.User, circleID int) {
	if !b.service.config.Enabled {
		return
	}

	event := NewSubtaskCompletedEvent(choreID, subtaskID, completedAt, user, circleID)
	event.ID = b.generateEventID()

	b.service.BroadcastToCircle(circleID, event)
}

// generateEventID generates a unique event ID
func (b *EventBroadcaster) generateEventID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
