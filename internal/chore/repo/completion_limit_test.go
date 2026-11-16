package chore

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/require"
)

func newLimitedChore(t *testing.T, limit int) (*ChoreRepository, *chModel.Chore) {
	t.Helper()

	r, db := newTestChoreRepo(t)
	chore := createChore(t, db, "limited", testOwnerID, false, nil)
	chore.FrequencyType = chModel.FrequencyTypeDaily
	chore.FrequencyMetadataV2 = &chModel.FrequencyMetadata{
		ScheduleBounds: &chModel.ScheduleBounds{
			StopRepeating: &chModel.StopRepeating{Type: chModel.StopCompletions, CompletionLimit: &limit},
		},
	}
	require.NoError(t, db.Save(chore).Error)
	return r, chore
}

func reloadChore(t *testing.T, r *ChoreRepository, id int) *chModel.Chore {
	t.Helper()

	chore, err := r.GetChore(context.Background(), id, testOwnerID, testCircleID)
	require.NoError(t, err)
	return chore
}

func TestCompleteChoreStopsAtCompletionLimit(t *testing.T) {
	r, chore := newLimitedChore(t, 2)
	next := time.Now().UTC().AddDate(0, 0, 1)
	now := time.Now().UTC()

	require.NoError(t, r.CompleteChore(context.Background(), chore, nil, testOwnerID, &next, &now, nil, false))
	chore = reloadChore(t, r, chore.ID)
	require.NotNil(t, chore.NextDueDate, "the first completion is under the limit")
	require.True(t, chore.IsActive)

	require.NoError(t, r.CompleteChore(context.Background(), chore, nil, testOwnerID, &next, &now, nil, false))
	chore = reloadChore(t, r, chore.ID)
	require.Nil(t, chore.NextDueDate, "the completion that meets the limit ends the chore")
	require.False(t, chore.IsActive)
}

func TestCompleteChoreLimitIgnoresSkips(t *testing.T) {
	r, chore := newLimitedChore(t, 2)
	next := time.Now().UTC().AddDate(0, 0, 1)
	now := time.Now().UTC()

	require.NoError(t, r.SkipChore(context.Background(), chore, testOwnerID, &next, nil, &now))
	require.NoError(t, r.SkipChore(context.Background(), reloadChore(t, r, chore.ID), testOwnerID, &next, nil, &now))
	require.NoError(t, r.CompleteChore(context.Background(), reloadChore(t, r, chore.ID), nil, testOwnerID, &next, &now, nil, false))

	chore = reloadChore(t, r, chore.ID)
	require.NotNil(t, chore.NextDueDate, "skips must not count toward the limit")
	require.True(t, chore.IsActive)
}

func TestApproveChoreStopsAtCompletionLimit(t *testing.T) {
	r, chore := newLimitedChore(t, 1)
	now := time.Now().UTC()
	next := now.AddDate(0, 0, 1)

	require.NoError(t, r.SetChorePendingApproval(context.Background(), chore, nil, testOwnerID, &now))
	require.NoError(t, r.ApproveChore(context.Background(), chore, testOwnerID, &next, nil, false))

	chore = reloadChore(t, r, chore.ID)
	require.Nil(t, chore.NextDueDate, "approving the completion that meets the limit ends the chore")
	require.False(t, chore.IsActive)
}
