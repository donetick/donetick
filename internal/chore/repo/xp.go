package chore

import (
	"errors"
	"fmt"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	xpRepo "donetick.com/core/internal/points/repo"
	"gorm.io/gorm"
)

var ErrChoreChanged = errors.New("task changed; refresh and try again")

// A lock alone does not make an already loaded task current. In particular, a
// second completion must not award the same occurrence after waiting for one.
// Always tasks may be completed again using their newly loaded sync version.
func lockCompletionXP(tx *gorm.DB, chore *chModel.Chore) error {
	if err := lockChoreXP(tx, chore.ID, chore.CircleID); err != nil {
		return err
	}
	var current chModel.Chore
	if err := tx.Select("sync_version", "status", "is_active").First(&current, chore.ID).Error; err != nil {
		return err
	}
	if current.SyncVersion != chore.SyncVersion || !current.IsActive || current.Status == chModel.ChoreStatusPendingApproval {
		return ErrChoreChanged
	}
	return nil
}

// Serializes completion/approval/rejection/undo for the same chore on both
// PostgreSQL and SQLite, before reading the history whose status will change.
func lockChoreXP(tx *gorm.DB, choreID, circleID int) error {
	r := tx.Model(&chModel.Chore{}).Where("id = ? AND circle_id = ?", choreID, circleID).UpdateColumn("status", gorm.Expr("status"))
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func snapshotCompletionXP(tx *gorm.DB, chore *chModel.Chore, userID int, history *chModel.ChoreHistory) error {
	history.CompletedBy = userID
	history.XPCategoryID = chore.XPCategoryID
	history.XPValue = chore.Points
	history.XPSnapshot = true
	if chore.Points == nil || *chore.Points <= 0 {
		return nil
	}
	if err := xpRepo.LockMember(tx, chore.CircleID, userID); err != nil {
		return err
	}
	var member cModel.UserCircle
	if err := tx.Where("circle_id = ? AND user_id = ? AND is_active = ?", chore.CircleID, userID, true).First(&member).Error; err != nil {
		return err
	}
	history.XPMembershipID = member.ID
	return nil
}

func awardCompletionXP(tx *gorm.DB, chore *chModel.Chore, history *chModel.ChoreHistory) error {
	if history.Points == nil || *history.Points <= 0 {
		return nil
	}
	var member cModel.UserCircle
	if err := tx.Where("circle_id = ? AND user_id = ? AND is_active = ?", chore.CircleID, history.CompletedBy, true).First(&member).Error; err != nil {
		return err
	}
	if (history.XPMembershipID != 0 && history.XPMembershipID != member.ID) || (history.XPMembershipID == 0 && member.CreatedAt.After(history.CreatedAt)) {
		return xpRepo.ErrMembershipChanged
	}
	return xpRepo.ChangeEarned(tx, chore.CircleID, history.CompletedBy, *history.Points, history.XPCategoryID, fmt.Sprintf("chore-award:%d:%d", history.ID, history.CreatedAt.UnixNano()), member.ID)
}

func reverseCompletionXP(tx *gorm.DB, choreID, circleID int, history *chModel.ChoreHistory) error {
	var member cModel.UserCircle
	err := tx.Where("circle_id = ? AND user_id = ? AND is_active = ?", circleID, history.CompletedBy, true).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// Removed memberships have no surviving balance. Never subtract their old
	// awards from a fresh membership created when the same user rejoins.
	if (history.XPMembershipID != 0 && history.XPMembershipID != member.ID) || (history.XPMembershipID == 0 && member.CreatedAt.After(history.CreatedAt)) {
		return nil
	}
	return xpRepo.ChangeEarned(tx, circleID, history.CompletedBy, -*history.Points, history.XPCategoryID, fmt.Sprintf("chore-undo:%d:%d", history.ID, history.CreatedAt.UnixNano()), member.ID)
}
