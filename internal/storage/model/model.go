package model

import "time"

type StorageFile struct {
	FilePath   string     `json:"file_path" gorm:"column:file_path;primaryKey"`
	FileName   string     `json:"file_name" gorm:"column:file_name"`
	SizeBytes  int        `json:"size_bytes" gorm:"column:size_bytes"`
	UserID     int        `json:"user_id" gorm:"column:user_id;index:idx_storage_file_user_id"`
	EntityID   int        `json:"entity_id" gorm:"column:entity_id;index:idx_entity"`
	EntityType EntityType `json:"entity_type" gorm:"column:entity_type;index:idx_entity"`
	DraftID    string     `json:"draft_id,omitempty" gorm:"column:draft_id;index:idx_draft_id"`
	CreatedAt  int        `json:"created_at" gorm:"column:created_at"`
	// Sign is a fetchable (signed) URL populated at serialization time; never stored.
	Sign string `json:"sign,omitempty" gorm:"-"`
}

type StorageUsage struct {
	UserID    int       `gorm:"column:user_id;primaryKey;index:idx_storage_usage_user_id"`
	CircleID  int       `gorm:"column:circle_id;primaryKey;index:idx_circle"`
	UsedBytes int       `gorm:"column:used_bytes"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

type EntityType int // OwnerType represents the type of the parent entity
const (
	EntityTypeUnknown EntityType = iota
	EntityTypeChoreDescription
	EntityTypeChoreHistory
	EntityTypeThing
	EntityTypeChoreAttachment
	EntityTypeChoreAttachmentDraft
	// EntityTypeChoreDescriptionDraft is a description image uploaded before the
	// chore exists; promoted to EntityTypeChoreDescription on chore creation.
	// Append new values only — these are persisted as integers.
	EntityTypeChoreDescriptionDraft
)
