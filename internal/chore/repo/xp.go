package chore

import (
	chModel "donetick.com/core/internal/chore/model"
	circle "donetick.com/core/internal/circle/model"
	xp "donetick.com/core/internal/xp/model"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidXP = errors.New("points must be between 0 and 1000 and category must be active in this circle")

func validateXP(tx *gorm.DB, chore *chModel.Chore, previous *chModel.Chore) error {
	if chore.Points != nil && (*chore.Points < 0 || *chore.Points > 1000) {
		return ErrInvalidXP
	}
	if chore.XPCategoryID == nil {
		return nil
	}
	if *chore.XPCategoryID <= 0 {
		return ErrInvalidXP
	}
	var category xp.Category
	// Share lock makes validation and a concurrent archive serialize on PostgreSQL.
	q := tx
	if tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := q.Where("id = ? AND circle_id = ?", *chore.XPCategoryID, chore.CircleID).First(&category).Error; err != nil {
		return ErrInvalidXP
	}
	unchanged := previous != nil && previous.XPCategoryID != nil && *previous.XPCategoryID == *chore.XPCategoryID
	if category.Archived && !unchanged {
		return ErrInvalidXP
	}
	return nil
}

func claimCompletion(tx *gorm.DB, chore *chModel.Chore, nextVersion int64) error {
	result := tx.Model(&chModel.Chore{}).Where("id = ? AND circle_id = ? AND sync_version = ?", chore.ID, chore.CircleID, chore.SyncVersion).Update("sync_version", nextVersion)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("task changed concurrently; reload before completing")
	}
	return nil
}

func recordXPAward(tx *gorm.DB, chore *chModel.Chore, history *chModel.ChoreHistory) error {
	if chore.XPCategoryID == nil || history.Points == nil || *history.Points <= 0 {
		return nil
	}
	var membership circle.UserCircle
	if err := tx.Where("circle_id = ? AND user_id = ?", chore.CircleID, history.CompletedBy).First(&membership).Error; err != nil {
		return err
	}
	return tx.Create(&xp.Award{MembershipID: membership.ID, HistoryID: history.ID, CircleID: chore.CircleID, UserID: history.CompletedBy, CategoryID: *chore.XPCategoryID, Points: *history.Points}).Error
}
