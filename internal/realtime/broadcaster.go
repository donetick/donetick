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

// broadcastChoreEvent enforces the same visibility boundary as the API. A
// private project's owner is its sole recipient; standalone private chores go
// only to their creator and assignees. Public chores remain circle-wide.
func (b *EventBroadcaster) broadcastChoreEvent(chore *chModel.Chore, event *Event) {
	// A caller that did not load the project cannot safely determine recipients.
	// Fail closed rather than treating an unknown private project as a public one.
	if chore.ProjectID != nil && chore.Project == nil {
		return
	}
	if chore.Project != nil && chore.Project.IsPrivate {
		b.service.BroadcastToUsers(chore.CircleID, []int{chore.Project.CreatedBy}, event)
		return
	}
	if chore.IsPrivate {
		recipients := []int{chore.CreatedBy}
		if chore.AssignedTo != nil {
			recipients = append(recipients, *chore.AssignedTo)
		}
		for _, assignee := range chore.Assignees {
			recipients = append(recipients, assignee.UserID)
		}
		b.service.BroadcastToUsers(chore.CircleID, recipients, event)
		return
	}
	b.service.BroadcastToCircle(chore.CircleID, event)
}

// BroadcastChoreCreated broadcasts a chore creation event
func (b *EventBroadcaster) BroadcastChoreCreated(chore *chModel.Chore, user *uModel.User) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreCreatedEvent(chore, user)
	event.ID = b.generateEventID()
	event.SyncVersion = chore.SyncVersion

	b.broadcastChoreEvent(chore, event)
}

// BroadcastChoreUpdated broadcasts a chore update event
func (b *EventBroadcaster) BroadcastChoreUpdated(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreUpdatedEvent(chore, user, changes, note)
	event.ID = b.generateEventID()
	event.SyncVersion = chore.SyncVersion

	b.broadcastChoreEvent(chore, event)
}

// BroadcastChoreDeleted broadcasts a chore deletion event
func (b *EventBroadcaster) BroadcastChoreDeleted(chore *chModel.Chore, user *uModel.User, syncVersion int64) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreDeletedEvent(chore.ID, chore.Name, chore.CircleID, user)
	event.ID = b.generateEventID()
	event.SyncVersion = syncVersion

	b.broadcastChoreEvent(chore, event)
}

// BroadcastChoreCompleted broadcasts a chore completion event
func (b *EventBroadcaster) BroadcastChoreCompleted(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreCompletedEvent(chore, user, history, note)
	event.ID = b.generateEventID()
	event.SyncVersion = chore.SyncVersion
	if history != nil && history.SyncVersion > event.SyncVersion {
		event.SyncVersion = history.SyncVersion
	}

	b.broadcastChoreEvent(chore, event)
}

// BroadcastChoreStarted broadcasts a chore start event
func (b *EventBroadcaster) BroadcastChoreStatus(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreStatusChangedEvent(chore, user, changes, nil)
	event.ID = b.generateEventID()
	event.SyncVersion = chore.SyncVersion

	b.broadcastChoreEvent(chore, event)
}

// BroadcastChoreSkipped broadcasts a chore skip event
func (b *EventBroadcaster) BroadcastChoreSkipped(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) {
	if !b.service.config.Enabled {
		return
	}

	event := NewChoreSkippedEvent(chore, user, history, note)
	event.ID = b.generateEventID()
	event.SyncVersion = chore.SyncVersion
	if history != nil && history.SyncVersion > event.SyncVersion {
		event.SyncVersion = history.SyncVersion
	}

	b.broadcastChoreEvent(chore, event)
}

// BroadcastSubtaskUpdated broadcasts a subtask update event
func (b *EventBroadcaster) BroadcastSubtaskUpdated(chore *chModel.Chore, subtaskID int, completedAt *time.Time, user *uModel.User, syncVersion int64) {
	if !b.service.config.Enabled {
		return
	}

	event := NewSubtaskUpdatedEvent(chore.ID, subtaskID, completedAt, user, chore.CircleID)
	event.ID = b.generateEventID()
	event.SyncVersion = syncVersion

	b.broadcastChoreEvent(chore, event)
}

// BroadcastSubtaskCompleted broadcasts a subtask completion event
func (b *EventBroadcaster) BroadcastSubtaskCompleted(chore *chModel.Chore, subtaskID int, completedAt *time.Time, user *uModel.User, syncVersion int64) {
	if !b.service.config.Enabled {
		return
	}

	event := NewSubtaskCompletedEvent(chore.ID, subtaskID, completedAt, user, chore.CircleID)
	event.ID = b.generateEventID()
	event.SyncVersion = syncVersion

	b.broadcastChoreEvent(chore, event)
}

// generateEventID generates a unique event ID
func (b *EventBroadcaster) generateEventID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
