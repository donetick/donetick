package chore

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/require"
)

// Chores without a next due date must stay active only when they are "always"
// chores, regardless of whether they were completed, approved or skipped.
func TestChoresWithoutDueDateStayActiveOnlyForAlways(t *testing.T) {
	cases := []struct {
		frequency  chModel.FrequencyType
		wantActive bool
	}{
		{chModel.FrequencyTypeAlways, true},
		{chModel.FrequencyTypeTrigger, false},
		{chModel.FrequencyTypeOnce, false},
	}

	actions := map[string]func(t *testing.T, r *ChoreRepository, chore *chModel.Chore, now time.Time) error{
		"complete": func(t *testing.T, r *ChoreRepository, chore *chModel.Chore, now time.Time) error {
			return r.CompleteChore(context.Background(), chore, nil, testOwnerID, nil, &now, nil, false)
		},
		"approve": func(t *testing.T, r *ChoreRepository, chore *chModel.Chore, now time.Time) error {
			chore.RequireApproval = true
			chore.Status = chModel.ChoreStatusPendingApproval
			require.NoError(t, r.db.Save(chore).Error)
			require.NoError(t, r.db.Create(&chModel.ChoreHistory{
				ChoreID:     chore.ID,
				CompletedBy: testOtherID,
				PerformedAt: &now,
				Status:      chModel.ChoreHistoryStatusPendingApproval,
			}).Error)
			return r.ApproveChore(context.Background(), chore, testOwnerID, nil, nil, false)
		},
		"skip": func(t *testing.T, r *ChoreRepository, chore *chModel.Chore, now time.Time) error {
			return r.SkipChore(context.Background(), chore, testOwnerID, nil, nil, &now)
		},
	}

	for actionName, action := range actions {
		for _, tc := range cases {
			t.Run(actionName+"/"+string(tc.frequency), func(t *testing.T) {
				r, db := newTestChoreRepo(t)
				chore := createChore(t, db, "chore", testOwnerID, false, nil, testOwnerID)
				chore.FrequencyType = tc.frequency
				require.NoError(t, db.Save(chore).Error)

				require.NoError(t, action(t, r, chore, time.Now().UTC()))

				var loaded chModel.Chore
				require.NoError(t, db.First(&loaded, chore.ID).Error)
				require.Equal(t, tc.wantActive, loaded.IsActive)
			})
		}
	}
}
