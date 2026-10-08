package model

import (
	"testing"
	"time"

	cModel "donetick.com/core/internal/circle/model"
)

func circleUser(userID int, role cModel.UserRole, active bool) *cModel.UserCircleDetail {
	return &cModel.UserCircleDetail{UserCircle: cModel.UserCircle{UserID: userID, Role: role, IsActive: active}}
}

func TestCanDelegate(t *testing.T) {
	updatedAt := time.Now().UTC().Add(-time.Minute)
	chore := &Chore{CreatedBy: 1, UpdatedAt: updatedAt}
	circleUsers := []*cModel.UserCircleDetail{
		circleUser(1, "admin", true),
		circleUser(2, "member", true),
		circleUser(3, "member", false),
	}
	now := time.Now().UTC()
	stale := updatedAt.Add(-time.Minute)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		userID    int
		updatedAt *time.Time
		wantErr   bool
	}{
		{"admin", 1, &now, false},
		{"member who is neither creator nor admin", 2, &now, false},
		{"inactive member", 3, &now, true},
		{"not in circle", 4, &now, true},
		{"stale updatedAt", 2, &stale, true},
		{"updatedAt in the future", 2, &future, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := chore.CanDelegate(tt.userID, circleUsers, tt.updatedAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("CanDelegate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
