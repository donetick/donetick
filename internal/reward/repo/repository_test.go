package repo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	rModel "donetick.com/core/internal/reward/model"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_reward.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	return db
}

// seedMember inserts a user_circles row directly (bypassing the circle
// package) so tests don't need a full user/circle signup flow.
func seedMember(t *testing.T, db *gorm.DB, userID, circleID, points, pointsRedeemed int) {
	t.Helper()
	member := &cModel.UserCircle{
		UserID:         userID,
		CircleID:       circleID,
		Role:           cModel.UserRoleMember,
		IsActive:       true,
		Points:         points,
		PointsRedeemed: pointsRedeemed,
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("failed to seed member: %v", err)
	}
}

func seedReward(t *testing.T, db *gorm.DB, circleID, pointsCost int) *rModel.Reward {
	t.Helper()
	reward := &rModel.Reward{
		CircleID:   circleID,
		Name:       "Movie Night",
		PointsCost: pointsCost,
		IsActive:   true,
	}
	if err := db.Create(reward).Error; err != nil {
		t.Fatalf("failed to seed reward: %v", err)
	}
	return reward
}

func TestRequestRedemption_InsufficientPoints(t *testing.T) {
	db := setupTestDB(t)
	r := NewRewardRepository(db, nil)
	ctx := context.Background()

	seedMember(t, db, 1, 1, 10, 0) // 10 available
	reward := seedReward(t, db, 1, 50)

	_, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	if err != ErrInsufficientPoints {
		t.Fatalf("expected ErrInsufficientPoints, got %v", err)
	}
}

func TestRequestRedemption_Success(t *testing.T) {
	db := setupTestDB(t)
	r := NewRewardRepository(db, nil)
	ctx := context.Background()

	seedMember(t, db, 1, 1, 100, 0) // 100 available
	reward := seedReward(t, db, 1, 50)

	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if redemption.Status != rModel.RedemptionStatusPending {
		t.Fatalf("expected status Pending, got %v", redemption.Status)
	}
	if redemption.PointsCost != 50 {
		t.Fatalf("expected snapshot cost 50, got %d", redemption.PointsCost)
	}
}

func TestApproveRedemption_DebitsPoints(t *testing.T) {
	db := setupTestDB(t)
	r := NewRewardRepository(db, nil)
	ctx := context.Background()

	seedMember(t, db, 1, 1, 100, 0)
	reward := seedReward(t, db, 1, 50)
	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if err := r.ApproveRedemption(ctx, redemption.ID, 1, 999); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	var member cModel.UserCircle
	if err := db.Where("user_id = ? AND circle_id = ?", 1, 1).First(&member).Error; err != nil {
		t.Fatalf("failed to load member: %v", err)
	}
	if member.PointsRedeemed != 50 {
		t.Fatalf("expected points_redeemed=50, got %d", member.PointsRedeemed)
	}
	if member.Points != 100 {
		t.Fatalf("expected points unchanged at 100, got %d", member.Points)
	}

	var updated rModel.RewardRedemption
	if err := db.First(&updated, redemption.ID).Error; err != nil {
		t.Fatalf("failed to load redemption: %v", err)
	}
	if updated.Status != rModel.RedemptionStatusApproved {
		t.Fatalf("expected status Approved, got %v", updated.Status)
	}
	if updated.ResolvedBy == nil || *updated.ResolvedBy != 999 {
		t.Fatalf("expected resolvedBy=999, got %v", updated.ResolvedBy)
	}
}

func TestApproveRedemption_NotPending(t *testing.T) {
	db := setupTestDB(t)
	r := NewRewardRepository(db, nil)
	ctx := context.Background()

	seedMember(t, db, 1, 1, 100, 0)
	reward := seedReward(t, db, 1, 50)
	redemption, _ := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)

	if err := r.ApproveRedemption(ctx, redemption.ID, 1, 999); err != nil {
		t.Fatalf("first approve failed: %v", err)
	}
	if err := r.ApproveRedemption(ctx, redemption.ID, 1, 999); err != ErrRedemptionNotPending {
		t.Fatalf("expected ErrRedemptionNotPending on second approve, got %v", err)
	}
}

func TestRejectRedemption_DoesNotTouchPoints(t *testing.T) {
	db := setupTestDB(t)
	r := NewRewardRepository(db, nil)
	ctx := context.Background()

	seedMember(t, db, 1, 1, 100, 0)
	reward := seedReward(t, db, 1, 50)
	redemption, _ := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)

	note := "not this week"
	if err := r.RejectRedemption(ctx, redemption.ID, 1, 999, &note); err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	var member cModel.UserCircle
	db.Where("user_id = ? AND circle_id = ?", 1, 1).First(&member)
	if member.PointsRedeemed != 0 {
		t.Fatalf("expected points_redeemed unchanged at 0, got %d", member.PointsRedeemed)
	}

	var updated rModel.RewardRedemption
	db.First(&updated, redemption.ID)
	if updated.Status != rModel.RedemptionStatusRejected {
		t.Fatalf("expected status Rejected, got %v", updated.Status)
	}
	if updated.Note == nil || *updated.Note != note {
		t.Fatalf("expected note to be saved, got %v", updated.Note)
	}
}
