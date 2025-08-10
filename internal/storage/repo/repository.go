package repo

import (
	"context"

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
	// create transaction and increment the storage then save the file:
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// confirm is the user have enough space and increment the storage:
		res := tx.Model(&st.StorageUsage{}).Where("(user_id = ? OR circle_id = ?)and used_bytes <= ? ", user.ID, user.CircleID, r.maxUserStorage-media.SizeBytes).Updates(map[string]interface{}{"used_bytes": gorm.Expr("used_bytes + ?", media.SizeBytes)})
		if res.RowsAffected == 0 {
			return errorx.ErrNotEnoughSpace
		}
		if res.Error != nil {
			return res.Error
		}
		// save the media record:
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
	// delete all files by entity type and entity ID:
	if err := r.db.WithContext(ctx).Where("entity_type = ? and entity_id = ?", entityType, entityID).Delete(&st.StorageFile{}).Error; err != nil {
		return err
	}
	return nil
}
