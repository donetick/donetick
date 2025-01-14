package points

import (
	"context"

	pModel "donetick.com/core/internal/points"
	"gorm.io/gorm"
)

type PointsRepository struct {
	db *gorm.DB
}

func NewPointsRepository(db *gorm.DB) *PointsRepository {
	return &PointsRepository{db}
}

func (r *PointsRepository) CreatePointsHistory(c context.Context, tx *gorm.DB, pointsHistory *pModel.PointsHistory) error {
	if tx != nil {
		return tx.Model(&pModel.PointsHistory{}).Save(pointsHistory).Error
	}

	return r.db.WithContext(c).Save(pointsHistory).Error
}
