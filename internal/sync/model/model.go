package model

type SyncCursor struct {
	ID         int        `json:"id" gorm:"primaryKey;autoIncrement"`
	CircleID   int        `json:"circleId" gorm:"column:circle_id;not null;uniqueIndex:idx_sync_cursors_circle_entity"`
	EntityType EntityType `json:"entityType" gorm:"column:entity_type;not null;uniqueIndex:idx_sync_cursors_circle_entity"`
	MaxVersion int64      `json:"maxVersion" gorm:"column:max_version;not null;default:0"`
}

type EntityType int8

const (
	EntityTypeChore EntityType = 1
)

type Tombstone struct {
	ID          int        `json:"id" gorm:"primaryKey;autoIncrement"`
	CircleID    int        `json:"circleId" gorm:"column:circle_id;not null;index:idx_tombstone_sync"`
	EntityType  EntityType `json:"entityType" gorm:"column:entity_type;not null;index:idx_tombstone_sync"`
	EntityID    int        `json:"entityId" gorm:"column:entity_id;not null;index:idx_tombstone_sync"`
	SyncVersion int64      `json:"syncVersion" gorm:"column:sync_version;not null;index:idx_tombstone_sync"`
}
