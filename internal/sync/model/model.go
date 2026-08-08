package model

type SyncCursor struct {
	ID         int        `json:"id" gorm:"primaryKey;autoIncrement"`
	CircleID   int        `json:"circleId" gorm:"column:circle_id;not null;uniqueIndex:idx_sync_cursors_circle_entity"`
	EntityType EntityType `json:"entityType" gorm:"column:entity_type;not null;uniqueIndex:idx_sync_cursors_circle_entity"`
	MaxVersion int64      `json:"maxVersion" gorm:"column:max_version;not null;default:0"`
}

type EntityType int8

const (
	EntityTypeChore        EntityType = 1
	EntityTypeChoreHistory EntityType = 2
)

type Tombstone struct {
	ID          int        `json:"id" gorm:"primaryKey;autoIncrement"`
	CircleID    int        `json:"circleId" gorm:"column:circle_id;not null;index:idx_tombstone_sync;index:idx_tombstone_retrieval,priority:1"`
	EntityType  EntityType `json:"entityType" gorm:"column:entity_type;not null;index:idx_tombstone_sync;index:idx_tombstone_retrieval,priority:2"`
	EntityID    int        `json:"entityId" gorm:"column:entity_id;not null;index:idx_tombstone_sync"`
	UserID      *int       `json:"-" gorm:"column:user_id;index:idx_tombstone_retrieval,priority:3"`
	SyncVersion int64      `json:"syncVersion" gorm:"column:sync_version;not null;index:idx_tombstone_sync;index:idx_tombstone_retrieval,priority:4"`
}
