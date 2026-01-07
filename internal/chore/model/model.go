package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cModel "donetick.com/core/internal/circle/model"
	lModel "donetick.com/core/internal/label/model"
	stModel "donetick.com/core/internal/subtask/model"
	tModel "donetick.com/core/internal/thing/model"
)

const MAX_TEMPLATES = 5

type FrequencyType string

const (
	FrequencyTypeOnce          FrequencyType = "once"
	FrequencyTypeDaily         FrequencyType = "daily"
	FrequencyTypeWeekly        FrequencyType = "weekly"
	FrequencyTypeMonthly       FrequencyType = "monthly"
	FrequencyTypeYearly        FrequencyType = "yearly"
	FrequencyTypeAdaptive      FrequencyType = "adaptive"
	FrequencyTypeInterval      FrequencyType = "interval"
	FrequencyTypeDayOfTheWeek  FrequencyType = "days_of_the_week"
	FrequencyTypeDayOfTheMonth FrequencyType = "day_of_the_month"
	FrequencyTypeTrigger       FrequencyType = "trigger"
	FrequencyTypeNoRepeat      FrequencyType = "no_repeat"
)

type AssignmentStrategy string

const (
	AssignmentStrategyRandom                   AssignmentStrategy = "random"
	AssignmentStrategyLeastAssigned            AssignmentStrategy = "least_assigned"
	AssignmentStrategyLeastCompleted           AssignmentStrategy = "least_completed"
	AssignmentStrategyKeepLastAssigned         AssignmentStrategy = "keep_last_assigned"
	AssignmentStrategyRandomExceptLastAssigned AssignmentStrategy = "random_except_last_assigned"
	AssignmentStrategyRoundRobin               AssignmentStrategy = "round_robin"
	AssignmentStrategyNoAssignee               AssignmentStrategy = "no_assignee"
)

type Chore struct {
	ID                     int                   `json:"id" gorm:"primary_key"`
	Name                   string                `json:"name" gorm:"column:name"`                                           // Chore description
	FrequencyType          FrequencyType         `json:"frequencyType" gorm:"column:frequency_type"`                        // "daily", "weekly", "monthly", "yearly", "adaptive",or "custom"
	Frequency              int                   `json:"frequency" gorm:"column:frequency"`                                 // Number of days, weeks, months, or years between chores
	FrequencyMetadata      *string               `json:"-" gorm:"column:frequency_meta"`                                    // TODO: Clean up after v0.1.39
	FrequencyMetadataV2    *FrequencyMetadata    `json:"frequencyMetadata" gorm:"column:frequency_meta_v2;type:json"`       // Additional frequency information for v2 (if used)
	NextDueDate            *time.Time            `json:"nextDueDate" gorm:"column:next_due_date;index"`                     // When the chore is due
	IsRolling              bool                  `json:"isRolling" gorm:"column:is_rolling"`                                // Whether the chore is rolling
	AssignedTo             *int                  `json:"assignedTo" gorm:"column:assigned_to"`                              // Who the chore is assigned to
	Assignees              []ChoreAssignees      `json:"assignees" gorm:"foreignkey:ChoreID;references:ID"`                 // Assignees of the chore
	AssignStrategy         AssignmentStrategy    `json:"assignStrategy" gorm:"column:assign_strategy"`                      // How the chore is assigned
	RotateEvery            *int                  `json:"rotateEvery,omitempty" gorm:"column:rotate_every"`                   // Number of completions before rotating assignee (nil or 0 = rotate every time)
	IsActive               bool                  `json:"isActive" gorm:"column:is_active"`                                  // Whether the chore is active
	Notification           bool                  `json:"notification" gorm:"column:notification"`                           // Whether the chore has notification
	NotificationMetadata   *string               `json:"-" gorm:"column:notification_meta"`                                 // TODO: Clean up after v0.1.39
	NotificationMetadataV2 *NotificationMetadata `json:"notificationMetadata" gorm:"column:notification_meta_v2;type:json"` // Additional notification information
	Labels                 *string               `json:"labels" gorm:"column:labels"`                                       // Labels for the chore
	LabelsV2               *[]Label              `json:"labelsV2" gorm:"many2many:chore_labels"`                            // Labels for the chore
	CircleID               int                   `json:"circleId" gorm:"column:circle_id;index"`                            // The circle this chore is in
	CreatedAt              time.Time             `json:"createdAt" gorm:"column:created_at"`                                // When the chore was created
	UpdatedAt              time.Time             `json:"updatedAt" gorm:"column:updated_at"`                                // When the chore was last updated
	CreatedBy              int                   `json:"createdBy" gorm:"column:created_by"`                                // Who created the chore
	UpdatedBy              int                   `json:"updatedBy" gorm:"column:updated_by"`                                // Who last updated the chore
	ThingChore             *tModel.ThingChore    `json:"thingChore" gorm:"foreignkey:chore_id;references:id;<-:false"`      // ThingChore relationship
	Status                 Status                `json:"status" gorm:"column:status"`
	Priority               int                   `json:"priority" gorm:"column:priority"`
	CompletionWindow       *int                  `json:"completionWindow,omitempty" gorm:"column:completion_window"` // Number seconds before the chore is due that it can be completed
	Points                 *int                  `json:"points,omitempty" gorm:"column:points"`                      // Points for completing the chore
	Description            *string               `json:"description,omitempty" gorm:"type:text;column:description"`  // Description of the chore
	SubTasks               *[]stModel.SubTask    `json:"subTasks,omitempty" gorm:"foreignkey:ChoreID;references:ID"` // Subtasks for the chore
	RequireApproval        bool                  `json:"requireApproval" gorm:"column:require_approval"`             // Whether chore completion requires admin approval
	IsPrivate              bool                  `json:"isPrivate" gorm:"column:is_private;default:false"`           // Whether the chore is private
	DeadlineOffset         *int                  `json:"deadlineOffset,omitempty" gorm:"column:deadline_offset"`     // Seconds after NextDueDate when chore deadline is reached
}

type Status int8

const (
	ChoreStatusNoStatus        Status = 0
	ChoreStatusInProgress      Status = 1
	ChoreStatusPaused          Status = 2
	ChoreStatusPendingApproval Status = 3
)

type ChoreAssignees struct {
	ID      int `json:"-" gorm:"primary_key"`
	ChoreID int `json:"-" gorm:"column:chore_id;uniqueIndex:idx_chore_user"`     // The chore this assignee is for
	UserID  int `json:"userId" gorm:"column:user_id;uniqueIndex:idx_chore_user"` // The user this assignee is for
}
type ChoreHistory struct {
	ID          int                `json:"id" gorm:"primary_key"`                             // Unique identifier
	ChoreID     int                `json:"choreId" gorm:"column:chore_id"`                    // The chore this history is for
	PerformedAt *time.Time         `json:"performedAt" gorm:"column:performed_at"`            // When the chore was performed (completed or skipped)
	CompletedBy int                `json:"completedBy" gorm:"column:completed_by"`            // Who completed the chore
	AssignedTo  *int               `json:"assignedTo" gorm:"column:assigned_to"`              // Who the chore was assigned to
	Note        *string            `json:"notes" gorm:"column:notes"`                         // Notes about the chore
	DueDate     *time.Time         `json:"dueDate" gorm:"column:due_date"`                    // When the chore was due
	UpdatedAt   *time.Time         `json:"updatedAt" gorm:"column:updated_at"`                // When the record was last updated
	CreatedAt   time.Time          `json:"createdAt" gorm:"column:created_at;autoCreateTime"` // When the record was created
	Status      ChoreHistoryStatus `json:"status" gorm:"column:status"`                       // Status of the chore (1=completed, 2=skipped)
	Points      *int               `json:"points,omitempty" gorm:"column:points"`             // Points for completing the chore
	Duration    *int               `json:"duration,omitempty" gorm:"<-:false;-:migration"`    // Duration in seconds calculated from query (read-only, no DB column)
}

type ChoreHistoryStatus int8

const (
	ChoreHistoryStatusStarted         ChoreHistoryStatus = 0
	ChoreHistoryStatusCompleted       ChoreHistoryStatus = 1
	ChoreHistoryStatusSkipped         ChoreHistoryStatus = 2
	ChoreHistoryStatusPendingApproval ChoreHistoryStatus = 3
	ChoreHistoryStatusRejected        ChoreHistoryStatus = 4
	ChoreHistoryStatusMissed          ChoreHistoryStatus = 5
)

type FrequencyMetadata struct {
	Days        []*string    `json:"days,omitempty"`
	Months      []*string    `json:"months,omitempty"`
	Unit        *string      `json:"unit,omitempty"`
	Time        string       `json:"time,omitempty"`
	Timezone    string       `json:"timezone,omitempty"`
	WeekPattern *Weekpattern `json:"weekPattern,omitempty"`
	WeekNumbers []int        `json:"weekNumbers,omitempty"` // DEPRECATED: use Occurrences instead
	Occurrences []*int       `json:"occurrences,omitempty"` // e.g. ["1","3","last"] for 1st, 3rd, and last occurrence of the day
}

type Weekpattern string

const (
	WeekpatternEveryWeek     Weekpattern = "every_week"
	WeekPatternWeekOfMonth   Weekpattern = "week_of_month"   // e.g. ["1","3","last"] for 1st, 3rd, and last occurrence of the day in the month
	WeekPatternWeekOfQuarter Weekpattern = "week_of_quarter" // e.g. ["1","2","3"] for 1st, 2nd, and 3rd occurrence of the day in the quarter
)

type NotificationMetadata struct {
	DueDate       bool                    `json:"dueDate,omitempty"`
	Completion    bool                    `json:"completion,omitempty"`
	Nagging       bool                    `json:"nagging,omitempty"`
	PreDue        bool                    `json:"predue,omitempty"`
	CircleGroup   bool                    `json:"circleGroup,omitempty"`
	CircleGroupID *int64                  `json:"circleGroupID,omitempty"`
	Templates     []*NotificationTemplate `json:"templates,omitempty" validate:"max=5"` // Template for notification
}

type NotificationTemplate struct {
	Value int                      `json:"value"`
	Unit  NotificationTemplateUnit `json:"unit"`
}

type NotificationTemplateUnit string

const (
	NotificationTemplateUnitMinute NotificationTemplateUnit = "m"
	NotificationTemplateUnitHour   NotificationTemplateUnit = "h"
	NotificationTemplateUnitDay    NotificationTemplateUnit = "d"
)

type Tag struct {
	ID   int    `json:"-" gorm:"primary_key"`
	Name string `json:"name" gorm:"column:name;unique"`
}

type ChoreDetail struct {
	ID                  int                `json:"id" gorm:"column:id"`
	Name                string             `json:"name" gorm:"column:name"`
	Description         *string            `json:"description" gorm:"column:description"`
	FrequencyType       string             `json:"frequencyType" gorm:"column:frequency_type"`
	NextDueDate         *time.Time         `json:"nextDueDate" gorm:"column:next_due_date"`
	AssignedTo          *int               `json:"assignedTo" gorm:"column:assigned_to"`
	LastCompletedDate   *time.Time         `json:"lastCompletedDate" gorm:"column:last_completed_date"`
	LastCompletedBy     *int               `json:"lastCompletedBy" gorm:"column:last_completed_by"`
	TotalCompletedCount int                `json:"totalCompletedCount" gorm:"column:total_completed"`
	Priority            int                `json:"priority" gorm:"column:priority"`
	Notes               *string            `json:"notes" gorm:"column:notes"`
	CreatedBy           int                `json:"createdBy" gorm:"column:created_by"`
	CompletionWindow    *int               `json:"completionWindow,omitempty" gorm:"column:completion_window"`
	Subtasks            *[]stModel.SubTask `json:"subTasks,omitempty" gorm:"foreignkey:ChoreID;references:ID"`
	Status              Status             `json:"status" gorm:"column:status"`
	Duration            int                `json:"duration" gorm:"column:duration"` // Total duration in seconds for the chore
	StartTime           *time.Time         `json:"startTime" gorm:"column:start_time"`
	TimerUpdatedAt      *time.Time         `json:"timerUpdatedAt" gorm:"column:timer_updated_at"` // When the chore was last started
	DeadlineOffset      *int               `json:"deadlineOffset,omitempty"`
}

type Label struct {
	ID        int    `json:"id" gorm:"primary_key"`
	Name      string `json:"name" gorm:"column:name"`
	Color     string `json:"color" gorm:"column:color"`
	CircleID  *int   `json:"-" gorm:"column:circle_id"`
	CreatedBy int    `json:"created_by" gorm:"column:created_by"`
}

type ChoreLabels struct {
	ChoreID int `json:"choreId" gorm:"primaryKey;autoIncrement:false;not null"`
	LabelID int `json:"labelId" gorm:"primaryKey;autoIncrement:false;not null"`
	UserID  int `json:"userId" gorm:"primaryKey;autoIncrement:false;not null"`
	Label   Label
}
type ChoreLiteReq struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	ID          int     `json:"id"`
	DueDate     string  `json:"dueDate"`
	CreatedBy   *int    `json:"createdBy"`
}

type ChoreReq struct {
	Name                 string                `json:"name" binding:"required"`
	FrequencyType        FrequencyType         `json:"frequencyType"`
	ID                   int                   `json:"id"`
	DueDate              string                `json:"dueDate"`
	Assignees            []ChoreAssignees      `json:"assignees"`
	AssignStrategy       AssignmentStrategy    `json:"assignStrategy" binding:"required"`
	RotateEvery          *int                  `json:"rotateEvery,omitempty"`
	AssignedTo           *int                  `json:"assignedTo"`
	IsRolling            bool                  `json:"isRolling"`
	IsActive             bool                  `json:"isActive"`
	Frequency            int                   `json:"frequency"`
	FrequencyMetadata    *FrequencyMetadata    `json:"frequencyMetadata"`
	Notification         bool                  `json:"notification"`
	NotificationMetadata *NotificationMetadata `json:"notificationMetadata"`
	Labels               []string              `json:"labels"`
	LabelsV2             *[]lModel.LabelReq    `json:"labelsV2"`
	ThingTrigger         *tModel.ThingTrigger  `json:"thingTrigger"`
	Points               *int                  `json:"points"`
	CompletionWindow     *int                  `json:"completionWindow"`
	Description          *string               `json:"description"`
	Priority             int                   `json:"priority"`
	SubTasks             *[]stModel.SubTask    `json:"subTasks"`
	RequireApproval      bool                  `json:"requireApproval"`
	IsPrivate            bool                  `json:"isPrivate"`
	DeadlineOffset       *int                  `json:"deadlineOffset,omitempty"`
	UpdatedAt            *time.Time            `json:"updatedAt,omitempty"` // For internal use only when syncing a chore updated offline
}

func (c *Chore) GetDeadline() *time.Time {
	if c.DeadlineOffset == nil || c.NextDueDate == nil {
		return nil
	}
	deadline := c.NextDueDate.Add(time.Duration(*c.DeadlineOffset) * time.Second)
	return &deadline
}

func (c *Chore) CanEdit(userID int, circleUsers []*cModel.UserCircleDetail, updatedAt *time.Time) error {
	userHasPermission := false
	choreCanModified := true
	if c.CreatedBy == userID {
		userHasPermission = true
	}
	for _, cu := range circleUsers {
		if cu.UserID == userID && cu.Role == "admin" {
			userHasPermission = true
			break
		}
	}
	if updatedAt != nil {
		// if the chore was updated after the user fetched it for editing, then do not allow editing
		if c.UpdatedAt.After(*updatedAt) {
			// this means the chore was modified by someone
			choreCanModified = false
		} else if updatedAt.After(time.Now()) {
			// if the updatedAt is in the future, then do not allow editing
			choreCanModified = false
			return errors.New("updatedAt is in the future and cannot be used to edit the chore")
		}
	}
	if !userHasPermission {
		return errors.New("user does not have permission to edit this chore")
	}
	if !choreCanModified {
		return errors.New("chore has been modified by another user, please refresh and try again")
	}
	return nil

}
func (c *Chore) CanView(userID int, circleUsers []*cModel.UserCircleDetail) bool {
	// if private then only creator and assignees can view:
	if c.IsPrivate {
		if c.CreatedBy == userID {
			return true
		}

		for _, a := range c.Assignees {
			if a.UserID == userID {
				return true
			}
		}
		return false
	}
	// if public then anyone in the circle can view:
	for _, cu := range circleUsers {
		if cu.UserID == userID {
			return true
		}
	}
	return false
}
func (c *Chore) CanComplete(userID int, circleUsers []*cModel.UserCircleDetail) bool {
	// If using no assignee strategy, allow any circle member to complete
	if c.AssignStrategy == AssignmentStrategyNoAssignee && (c.AssignedTo == nil || *c.AssignedTo == 0) {
		if !c.IsPrivate {
			// For public chores with no assignee, any circle member can complete
			for _, cu := range circleUsers {
				if cu.UserID == userID {
					return true
				}
			}
		} else {
			// For private chores with no assignee, only creator can complete
			if c.CreatedBy == userID {
				return true
			}
		}
		return false
	}

	if !c.IsPrivate {
		if c.AssignedTo != nil && *c.AssignedTo == userID {
			return true
		}
		for _, a := range c.Assignees {
			if a.UserID == userID {
				return true
			}
		}
		// manager and admin can complete any chore in the circle:
		for _, cu := range circleUsers {
			if cu.UserID == userID && cu.IsManagerOrAdmin() {
				return true
			}
		}
	} else {
		// if private then only creator, assigned to and assignees can complete:
		if c.CreatedBy == userID {
			return true
		}
		if c.AssignedTo != nil && *c.AssignedTo == userID {
			return true
		}
		for _, a := range c.Assignees {
			if a.UserID == userID {
				return true
			}
		}
	}
	return false
}

// Implement driver.Valuer to convert the struct to JSON when saving to the database otherwise will
// get `error converting argument $12 type: unsupported type model.NotificationMetadata,  a struct` need
// the `Value()` and `Scan()` methods to store and retrieve the `NotificationMetadata` struct in the database as JSON.
func (n NotificationMetadata) Value() (driver.Value, error) {
	return json.Marshal(n)
}

func (n *NotificationMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var err error
	switch v := value.(type) {
	case []byte:
		err = json.Unmarshal(v, n)
	case string:
		err = json.Unmarshal([]byte(v), n)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	if err != nil {
		return err
	}

	// Validate after unmarshaling from database
	return n.Validate()
}

func (n *NotificationMetadata) Validate() error {
	if n == nil {
		return nil
	}

	if len(n.Templates) > MAX_TEMPLATES {
		return fmt.Errorf("templates cannot exceed %d items (got %d)",
			MAX_TEMPLATES, len(n.Templates))
	}

	return nil
}

func (f FrequencyMetadata) Value() (driver.Value, error) {
	return json.Marshal(f)
}

func (f *FrequencyMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, f)
}

type TimeSession struct {
	ID             int               `json:"id" gorm:"primary_key"`
	ChoreID        int               `json:"choreId" gorm:"column:chore_id;index"`
	ChoreHistoryID int               `json:"choreHistoryId" gorm:"column:chore_history_id;index"` // The chore history this session is for
	StartTime      time.Time         `json:"startTime" gorm:"column:start_time"`
	EndTime        *time.Time        `json:"endTime" gorm:"column:end_time"`
	Duration       int               `json:"duration" gorm:"column:duration"`
	Status         TimeSessionStatus `json:"status" gorm:"column:status"`
	PauseLog       PauseLogEntries   `json:"pauseLog" gorm:"column:pause_log;type:json"` // Track pause/resume times
	UpdateBy       int               `json:"updatedBy" gorm:"column:updated_by"`         // Who last updated the session
	UpdateAt       time.Time         `json:"updatedAt" gorm:"column:updated_at"`         // When the session was last updated
}

type TimeSessionStatus int8

const (
	TimeSessionStatusActive    TimeSessionStatus = 0
	TimeSessionStatusPaused    TimeSessionStatus = 1
	TimeSessionStatusCompleted TimeSessionStatus = 2
)

type PauseLogEntry struct {
	StartTime time.Time  `json:"start"`
	EndTime   *time.Time `json:"end"`
	Duration  int        `json:"duration"`
	UpdateBy  int        `json:"updatedBy"`
}

// PauseLogEntries is a custom type for handling JSON serialization/deserialization
type PauseLogEntries []*PauseLogEntry

// Implement driver.Valuer to convert the slice to JSON when saving to the database
func (p PauseLogEntries) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Implement sql.Scanner to convert JSON from database back to slice
func (p *PauseLogEntries) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		return json.Unmarshal([]byte(v), p)
	default:
		return errors.New("type assertion to []byte or string failed")
	}
}

func (t *TimeSession) Start(UserID int) {
	timeNow := time.Now().UTC()
	t.Status = TimeSessionStatusActive
	if t.StartTime.IsZero() {
		t.StartTime = timeNow
	}

	t.PauseLog = append(t.PauseLog, &PauseLogEntry{
		StartTime: timeNow,
		UpdateBy:  UserID,
	})
	t.UpdateBy = UserID
	t.UpdateAt = timeNow
}

func (t *TimeSession) Pause(UserID int) {
	timeNow := time.Now().UTC()
	duration := 0
	if t.Status == TimeSessionStatusActive {
		t.Status = TimeSessionStatusPaused
	}
	if len(t.PauseLog) > 0 && t.PauseLog[len(t.PauseLog)-1].EndTime == nil {
		// If the last pause entry doesn't have an end time, update it
		t.PauseLog[len(t.PauseLog)-1].EndTime = &timeNow
		duration = int(timeNow.Sub(t.PauseLog[len(t.PauseLog)-1].StartTime).Seconds())
		t.PauseLog[len(t.PauseLog)-1].Duration = duration
		t.Duration += duration
	}
	t.UpdateBy = UserID
	t.UpdateAt = timeNow
}

func (t *TimeSession) Finish(UserID int) {
	timeNow := time.Now().UTC()
	t.Status = TimeSessionStatusCompleted
	if len(t.PauseLog) > 0 && t.PauseLog[len(t.PauseLog)-1].EndTime == nil {
		// If the last pause entry doesn't have an end time, update it
		t.PauseLog[len(t.PauseLog)-1].EndTime = &timeNow
		duration := int(timeNow.Sub(t.PauseLog[len(t.PauseLog)-1].StartTime).Seconds())
		t.PauseLog[len(t.PauseLog)-1].Duration = duration
		t.Duration += duration
	}
	t.EndTime = &timeNow
	t.UpdateBy = UserID
	t.UpdateAt = timeNow
}
