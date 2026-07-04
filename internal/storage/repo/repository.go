package repo

import (
	"context"
	"time"

	"donetick.com/core/config"
	errorx "donetick.com/core/internal/error"

	st "donetick.com/core/internal/storage/model"
	uModel "donetick.com/core/internal/user/model"
	"gorm.io/gorm"
)

type StorageRepository struct {
	db             *gorm.DB
	maxUserStorage int
}

func NewStorageRepository(db *gorm.DB, config *config.Config) *StorageRepository {
	return &StorageRepository{db: db, maxUserStorage: config.Storage.MaxUserStorage}
}

func (r *StorageRepository) AddMediaRecord(ctx context.Context, media *st.StorageFile, user *uModel.UserDetails) error {
	if !user.IsPlusMember() {
		return errorx.ErrNotAPlusMember
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check total circle-wide usage against quota (all members share the same pool).
		var circleUsed int64
		if err := tx.Model(&st.StorageUsage{}).
			Select("COALESCE(SUM(used_bytes), 0)").
			Where("circle_id = ?", user.CircleID).
			Scan(&circleUsed).Error; err != nil {
			return err
		}
		if int(circleUsed)+media.SizeBytes > r.maxUserStorage {
			return errorx.ErrNotEnoughSpace
		}

		// Increment only this user's own row.
		res := tx.Model(&st.StorageUsage{}).
			Where("user_id = ?", user.ID).
			Updates(map[string]interface{}{"used_bytes": gorm.Expr("used_bytes + ?", media.SizeBytes)})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errorx.ErrNotEnoughSpace
		}

		if err := tx.Model(&st.StorageFile{}).Create(&media).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *StorageRepository) RemoveFileRecords(ctx context.Context, files []*st.StorageFile, userID int) error {
	// create transaction and increment the storage then save the file:
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		ids := make([]string, len(files))
		filesSize := 0
		for i, file := range files {
			ids[i] = file.FilePath
			filesSize += file.SizeBytes
		}

		// delete the files:
		if err := tx.Model(&st.StorageFile{}).Where("file_path in (?) and user_id = ?", ids, userID).Delete(&st.StorageFile{}).Error; err != nil {
			return err
		}

		// decrement the storage:
		if err := tx.Model(&st.StorageUsage{}).Where("user_id = ? ", userID).Updates(map[string]interface{}{"used_bytes": gorm.Expr("used_bytes - ?", filesSize)}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *StorageRepository) GetAllFilesByOwnerType(ctx context.Context, entityType st.EntityType, entityID int) ([]*st.StorageFile, error) {
	var files []*st.StorageFile
	if err := r.db.WithContext(ctx).Where("entity_type = ? and entity_id = ?", entityType, entityID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *StorageRepository) GetFilesByUser(ctx context.Context, userID int, entityType st.EntityType, entityID int) ([]*st.StorageFile, error) {
	var files []*st.StorageFile
	// we are getting files by user ID, entity type and entity ID, or entity ID = 0 which will get file for this specific entity and anything
	// in purgatory ( file upload without having yet an entity ID )
	if err := r.db.WithContext(ctx).Where("user_id = ? and entity_type = ? and (entity_id = ?  or entity_id = 0)", userID, entityType, entityID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *StorageRepository) GetStorageStats(ctx context.Context, currentUser *uModel.UserDetails) (int, int, error) {
	var totalUsedBytes int64
	if err := r.db.WithContext(ctx).Model(&st.StorageUsage{}).
		Select("COALESCE(SUM(used_bytes), 0)").
		Where("circle_id = ?", currentUser.CircleID).
		Scan(&totalUsedBytes).Error; err != nil {
		return 0, 0, err
	}
	return int(totalUsedBytes), r.maxUserStorage, nil
}

func (r *StorageRepository) RemoveAllFileByEntity(ctx context.Context, entityType st.EntityType, entityID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var files []*st.StorageFile
		if err := tx.Where("entity_type = ? AND entity_id = ?", entityType, entityID).Find(&files).Error; err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}

		// group by user so each user's quota is decremented correctly
		byUser := make(map[int]int)
		paths := make([]string, 0, len(files))
		for _, f := range files {
			byUser[f.UserID] += f.SizeBytes
			paths = append(paths, f.FilePath)
		}

		if err := tx.Where("file_path IN (?)", paths).Delete(&st.StorageFile{}).Error; err != nil {
			return err
		}

		for userID, totalBytes := range byUser {
			if err := tx.Model(&st.StorageUsage{}).
				Where("user_id = ?", userID).
				Updates(map[string]interface{}{"used_bytes": gorm.Expr("used_bytes - ?", totalBytes)}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *StorageRepository) GetFileByPath(ctx context.Context, filePath string, userID int) (*st.StorageFile, error) {
	var file st.StorageFile
	if err := r.db.WithContext(ctx).Where("file_path = ? and user_id = ?", filePath, userID).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *StorageRepository) ReassignDraftAttachments(ctx context.Context, draftID string, userID int, choreID int) error {
	return r.db.WithContext(ctx).Model(&st.StorageFile{}).
		Where("draft_id = ? AND user_id = ? AND entity_type = ?", draftID, userID, st.EntityTypeChoreAttachmentDraft).
		Updates(map[string]interface{}{
			"entity_type": st.EntityTypeChoreAttachment,
			"entity_id":   choreID,
			"draft_id":    "",
		}).Error
}

// GetExpiredDraftFiles returns draft attachment files older than the given unix timestamp.
func (r *StorageRepository) CreateStorageUsage(ctx context.Context, userID, circleID int) error {
	return r.db.WithContext(ctx).Create(&st.StorageUsage{
		UserID:    userID,
		CircleID:  circleID,
		UsedBytes: 0,
		UpdatedAt: time.Now().UTC(),
	}).Error
}

func (r *StorageRepository) GetExpiredDraftFiles(ctx context.Context, olderThanUnix int64) ([]*st.StorageFile, error) {
	var files []*st.StorageFile
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND created_at < ?", st.EntityTypeChoreAttachmentDraft, olderThanUnix).
		Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}
