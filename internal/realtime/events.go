package realtime

import (
	"encoding/json"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	uModel "donetick.com/core/internal/user/model"
)

// EventType represents the type of real-time event
type EventType string

const (
	// Chore events
	EventTypeChoreCreated         EventType = "chore.created"
	EventTypeChoreUpdated         EventType = "chore.updated"
	EventTypeChoreDeleted         EventType = "chore.deleted"
	EventTypeChoreCompleted       EventType = "chore.completed"
	EventTypeChoreSkipped         EventType = "chore.skipped"
	EventTypeChoreAssigneeChanged EventType = "chore.assignee_changed"
	EventTypeChoreStatus          EventType = "chore.status"
	EventTypeChoreDueDateChanged  EventType = "chore.due_date_changed"
	EventTypeChoreArchived        EventType = "chore.archived"

	// Subtask events
	EventTypeSubtaskUpdated   EventType = "subtask.updated"
	EventTypeSubtaskCompleted EventType = "subtask.completed"

	// System events
	EventTypeConnectionEstablished EventType = "connection.established"
	EventTypeHeartbeat             EventType = "heartbeat"
	EventTypeError                 EventType = "error"
)

// Event represents a real-time event to be sent to clients
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	CircleID  int         `json:"circleId"`
	Data      interface{} `json:"data"`
	ID        string      `json:"id,omitempty"`
}

// ChoreEventData contains data for chore-related events
type ChoreEventData struct {
	Chore   *chModel.Chore         `json:"chore"`
	User    *uModel.User           `json:"user"`
	Changes map[string]interface{} `json:"changes,omitempty"`
	History *chModel.ChoreHistory  `json:"history,omitempty"`
	Note    *string                `json:"note,omitempty"`
}

// ChoreCreatedData contains data for chore creation events
type ChoreCreatedData struct {
	Chore *chModel.Chore `json:"chore"`
	User  *uModel.User   `json:"user"`
}

// ChoreDeletedData contains data for chore deletion events
type ChoreDeletedData struct {
	ChoreID   int          `json:"choreId"`
	ChoreName string       `json:"choreName"`
	CircleID  int          `json:"circleId"`
	User      *uModel.User `json:"user"`
}

// SubtaskEventData contains data for subtask events
type SubtaskEventData struct {
	ChoreID     int          `json:"choreId"`
	SubtaskID   int          `json:"subtaskId"`
	CompletedAt *time.Time   `json:"completedAt"`
	User        *uModel.User `json:"user"`
}

// ConnectionEstablishedData contains data sent when connection is established
type ConnectionEstablishedData struct {
	ConnectionID string    `json:"connectionId"`
	CircleID     int       `json:"circleId"`
	UserID       int       `json:"userId"`
	Timestamp    time.Time `json:"timestamp"`
}

// HeartbeatData contains heartbeat ping data
type HeartbeatData struct {
	Timestamp time.Time `json:"timestamp"`
	ServerID  string    `json:"serverId,omitempty"`
}

// ErrorData contains error information
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ToJSON serializes the event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// NewEvent creates a new event with current timestamp
func NewEvent(eventType EventType, circleID int, data interface{}) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		CircleID:  circleID,
		Data:      data,
	}
}

// NewChoreCreatedEvent creates a chore creation event
func NewChoreCreatedEvent(chore *chModel.Chore, user *uModel.User) *Event {
	return NewEvent(EventTypeChoreCreated, chore.CircleID, &ChoreCreatedData{
		Chore: chore,
		User:  user,
	})
}

// NewChoreUpdatedEvent creates a chore update event
func NewChoreUpdatedEvent(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}, note *string) *Event {
	return NewEvent(EventTypeChoreUpdated, chore.CircleID, &ChoreEventData{
		Chore:   chore,
		User:    user,
		Changes: changes,
		Note:    note,
	})
}

// NewChoreDeletedEvent creates a chore deletion event
func NewChoreDeletedEvent(choreID int, choreName string, circleID int, user *uModel.User) *Event {
	return NewEvent(EventTypeChoreDeleted, circleID, &ChoreDeletedData{
		ChoreID:   choreID,
		ChoreName: choreName,
		CircleID:  circleID,
		User:      user,
	})
}

// NewChoreCompletedEvent creates a chore completion event
func NewChoreCompletedEvent(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) *Event {
	return NewEvent(EventTypeChoreCompleted, chore.CircleID, &ChoreEventData{
		Chore:   chore,
		User:    user,
		History: history,
		Note:    note,
	})
}

func NewChoreStatusChangedEvent(chore *chModel.Chore, user *uModel.User, changes map[string]interface{}, note *string) *Event {
	return NewEvent(EventTypeChoreStatus, chore.CircleID, &ChoreEventData{
		Chore:   chore,
		User:    user,
		Changes: changes,
		Note:    note,
	})
}

// NewChoreSkippedEvent creates a chore skip event
func NewChoreSkippedEvent(chore *chModel.Chore, user *uModel.User, history *chModel.ChoreHistory, note *string) *Event {
	return NewEvent(EventTypeChoreSkipped, chore.CircleID, &ChoreEventData{
		Chore:   chore,
		User:    user,
		History: history,
		Note:    note,
	})
}

// NewSubtaskUpdatedEvent creates a subtask update event
func NewSubtaskUpdatedEvent(choreID, subtaskID int, completedAt *time.Time, user *uModel.User, circleID int) *Event {
	return NewEvent(EventTypeSubtaskUpdated, circleID, &SubtaskEventData{
		ChoreID:     choreID,
		SubtaskID:   subtaskID,
		CompletedAt: completedAt,
		User:        user,
	})
}

// NewSubtaskCompletedEvent creates a subtask completion event
func NewSubtaskCompletedEvent(choreID, subtaskID int, completedAt *time.Time, user *uModel.User, circleID int) *Event {
	return NewEvent(EventTypeSubtaskCompleted, circleID, &SubtaskEventData{
		ChoreID:     choreID,
		SubtaskID:   subtaskID,
		CompletedAt: completedAt,
		User:        user,
	})
}

// NewConnectionEstablishedEvent creates a connection established event
func NewConnectionEstablishedEvent(connectionID string, circleID, userID int) *Event {
	return NewEvent(EventTypeConnectionEstablished, circleID, &ConnectionEstablishedData{
		ConnectionID: connectionID,
		CircleID:     circleID,
		UserID:       userID,
		Timestamp:    time.Now().UTC(),
	})
}

// NewHeartbeatEvent creates a heartbeat event
func NewHeartbeatEvent(circleID int) *Event {
	return NewEvent(EventTypeHeartbeat, circleID, &HeartbeatData{
		Timestamp: time.Now().UTC(),
	})
}

// NewErrorEvent creates an error event
func NewErrorEvent(circleID int, code, message string) *Event {
	return NewEvent(EventTypeError, circleID, &ErrorData{
		Code:    code,
		Message: message,
	})
}
