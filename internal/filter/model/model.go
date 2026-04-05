package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ConditionType represents the type of condition
type ConditionType string

const (
	ConditionTypeAssignee  ConditionType = "assignee"
	ConditionTypeCreatedBy ConditionType = "createdBy"
	ConditionTypePriority  ConditionType = "priority"
	ConditionTypeStatus    ConditionType = "status"
	ConditionTypeDueDate   ConditionType = "dueDate"
	ConditionTypeLabel     ConditionType = "label"
	ConditionTypeProject   ConditionType = "project"
	ConditionTypePoints    ConditionType = "points"
)

// ConditionOperator represents operators for different condition types
type ConditionOperator string

const (
	// Common operators
	OperatorIs    ConditionOperator = "is"
	OperatorIsNot ConditionOperator = "isNot"
	OperatorOr    ConditionOperator = "or"
	OperatorAnd   ConditionOperator = "and"

	// Comparison operators (for priority, points)
	OperatorEquals             ConditionOperator = "equals"
	OperatorGreaterThan        ConditionOperator = "greaterThan"
	OperatorLessThan           ConditionOperator = "lessThan"
	OperatorGreaterThanOrEqual ConditionOperator = "greaterThanOrEqual"
	OperatorLessThanOrEqual    ConditionOperator = "lessThanOrEqual"

	// Due date specific operators
	OperatorIsOverdue      ConditionOperator = "isOverdue"
	OperatorIsDueToday     ConditionOperator = "isDueToday"
	OperatorIsDueTomorrow  ConditionOperator = "isDueTomorrow"
	OperatorIsDueThisWeek  ConditionOperator = "isDueThisWeek"
	OperatorIsDueThisMonth ConditionOperator = "isDueThisMonth"
	OperatorHasNoDueDate   ConditionOperator = "hasNoDueDate"
	OperatorHasDueDate     ConditionOperator = "hasDueDate"
	OperatorBefore         ConditionOperator = "before"
	OperatorAfter          ConditionOperator = "after"
	OperatorBetween        ConditionOperator = "between"

	// Label/Project specific operators
	OperatorHas         ConditionOperator = "has"
	OperatorDoesNotHave ConditionOperator = "doesNotHave"
)

// LogicalOperator represents how conditions are combined
type LogicalOperator string

const (
	LogicalOperatorAND LogicalOperator = "AND"
	LogicalOperatorOR  LogicalOperator = "OR"
)

// noValueOperators are operators that don't require a value field
var noValueOperators = map[ConditionOperator]bool{
	OperatorIsOverdue:      true,
	OperatorIsDueToday:     true,
	OperatorIsDueTomorrow:  true,
	OperatorIsDueThisWeek:  true,
	OperatorIsDueThisMonth: true,
	OperatorHasNoDueDate:   true,
	OperatorHasDueDate:     true,
}

// FilterCondition represents a single condition in a filter
type FilterCondition struct {
	Type     string      `json:"type" binding:"required,oneof=assignee createdBy priority status dueDate label project points"`
	Operator string      `json:"operator" binding:"required"`
	Value    interface{} `json:"value"`
}

func (fc FilterCondition) Validate() error {
	op := ConditionOperator(fc.Operator)
	if noValueOperators[op] {
		return nil
	}
	if fc.Value == nil {
		return fmt.Errorf("condition type %q with operator %q requires a value", fc.Type, fc.Operator)
	}
	return nil
}

// FilterConditions is a custom type for JSON array storage
type FilterConditions []FilterCondition

// Scan implements the sql.Scanner interface
func (fc *FilterConditions) Scan(value interface{}) error {
	if value == nil {
		*fc = FilterConditions{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to unmarshal FilterConditions")
	}

	return json.Unmarshal(bytes, fc)
}

// Validate checks all conditions are valid
func (fcs FilterConditions) Validate() error {
	for i, fc := range fcs {
		if err := fc.Validate(); err != nil {
			return fmt.Errorf("condition[%d]: %w", i, err)
		}
	}
	return nil
}

// Value implements the driver.Valuer interface
func (fc FilterConditions) Value() (driver.Value, error) {
	if len(fc) == 0 {
		return "[]", nil
	}
	return json.Marshal(fc)
}

type Filter struct {
	ID          int              `json:"id" gorm:"primary_key"`
	Name        string           `json:"name" gorm:"column:name;not null"`
	Description *string          `json:"description" gorm:"column:description"`
	Color       *string          `json:"color" gorm:"column:color"`
	Icon        *string          `json:"icon" gorm:"column:icon"`
	Conditions  FilterConditions `json:"conditions" gorm:"column:conditions;type:json;not null"`
	Operator    LogicalOperator  `json:"operator" gorm:"column:operator;default:'AND';not null"`
	CircleID    int              `json:"circleId" gorm:"column:circle_id;index;not null"`
	CreatedBy   int              `json:"createdBy" gorm:"column:created_by;not null"`
	CreatedAt   time.Time        `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time       `json:"updatedAt,omitempty" gorm:"column:updated_at;autoUpdateTime"`
	IsPinned    bool             `json:"isPinned" gorm:"column:is_pinned;default:false"`
}

type FilterReq struct {
	Name        string           `json:"name" binding:"required,min=1,max=100"`
	Description *string          `json:"description" binding:"omitempty,max=500"`
	Color       *string          `json:"color" binding:"omitempty,hexcolor|rgb|rgba"`
	Icon        *string          `json:"icon" binding:"omitempty,max=50"`
	Conditions  FilterConditions `json:"conditions" binding:"required,min=1,max=20,dive"`
	Operator    *LogicalOperator `json:"operator" binding:"omitempty,oneof=AND OR"`
	IsPinned    bool             `json:"isPinned"`
}

type UpdateFilterReq struct {
	ID int `json:"id" binding:"required"`
	FilterReq
}
