package chore

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	tModel "donetick.com/core/internal/thing/model"
	"github.com/stretchr/testify/require"
)

func TestCompletionActions(t *testing.T) {
	for _, tc := range []struct{ name, kind, state, operation, value, want string }{
		{"boolean reset", "boolean", "true", "set", "false", "false"},
		{"counter increment", "number", "5", "add", "2", "7"},
		{"counter decrement", "number", "5", "add", "-2", "3"},
		{"text clear", "text", "ready", "set", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newTestChoreRepo(t)
			thing := tModel.Thing{UserID: testOwnerID, Type: tc.kind, State: tc.state}
			require.NoError(t, db.Create(&thing).Error)
			chore := createChore(t, db, "action", testOwnerID, false, nil)
			chore.CompletionActions = []chModel.CompletionAction{{ThingID: thing.ID, Operation: tc.operation, Value: tc.value}}
			require.NoError(t, db.Save(chore).Error)
			var loaded chModel.Chore
			require.NoError(t, db.First(&loaded, chore.ID).Error)
			now := time.Now().UTC()
			require.NoError(t, r.CompleteChore(context.Background(), &loaded, nil, testOwnerID, nil, &now, nil, false))
			require.NoError(t, db.First(&thing, thing.ID).Error)
			require.Equal(t, tc.want, thing.State)
			var histories []tModel.ThingHistory
			require.NoError(t, db.Where("thing_id = ?", thing.ID).Find(&histories).Error)
			require.Len(t, histories, 1)
			require.Equal(t, tc.want, histories[0].State)
		})
	}
}

func TestCompletionActionsRollbackAndSkip(t *testing.T) {
	r, db := newTestChoreRepo(t)
	thing := tModel.Thing{UserID: testOwnerID, Type: "number", State: "5"}
	require.NoError(t, db.Create(&thing).Error)
	chore := createChore(t, db, "action", testOwnerID, false, nil)
	chore.CompletionActions = []chModel.CompletionAction{
		{ThingID: thing.ID, Operation: "add", Value: "2"},
		{ThingID: thing.ID, Operation: "set", Value: "invalid"},
	}
	require.NoError(t, db.Save(chore).Error)
	now := time.Now().UTC()
	require.Error(t, r.CompleteChore(context.Background(), chore, nil, testOwnerID, nil, &now, nil, false))
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "5", thing.State)
	var loaded chModel.Chore
	require.NoError(t, db.First(&loaded, chore.ID).Error)
	require.True(t, loaded.IsActive)
	var count int64
	require.NoError(t, db.Model(&chModel.ChoreHistory{}).Where("chore_id = ?", chore.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, r.SkipChore(context.Background(), chore, testOwnerID, nil, nil, &now))
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "5", thing.State)
}

func TestCompletionActionsTriggerAnotherTask(t *testing.T) {
	r, db := newTestChoreRepo(t)
	thing := tModel.Thing{UserID: testOwnerID, Type: "boolean", State: "false"}
	require.NoError(t, db.Create(&thing).Error)
	target := createChore(t, db, "triggered", testOwnerID, false, nil)
	target.FrequencyType = "trigger"
	target.IsActive = false
	require.NoError(t, db.Save(target).Error)
	require.NoError(t, db.Create(&tModel.ThingChore{ThingID: thing.ID, ChoreID: target.ID, TriggerState: "true"}).Error)
	chore := createChore(t, db, "source", testOwnerID, false, nil)
	chore.CompletionActions = []chModel.CompletionAction{{ThingID: thing.ID, Operation: "set", Value: "true"}}
	require.NoError(t, db.Save(chore).Error)
	now := time.Now().UTC()
	require.NoError(t, r.CompleteChore(context.Background(), chore, nil, testOwnerID, nil, &now, nil, false))
	require.NoError(t, db.First(target, target.ID).Error)
	require.True(t, target.IsActive)
	require.NotNil(t, target.NextDueDate)
}

func TestCompletionActionsCannotChangeAnotherOwnersThing(t *testing.T) {
	r, db := newTestChoreRepo(t)
	thing := tModel.Thing{UserID: testOtherID, Type: "boolean", State: "true"}
	require.NoError(t, db.Create(&thing).Error)
	chore := createChore(t, db, "source", testOwnerID, false, nil)
	chore.CompletionActions = []chModel.CompletionAction{{ThingID: thing.ID, Operation: "set", Value: "false"}}
	require.NoError(t, db.Save(chore).Error)
	now := time.Now().UTC()
	require.Error(t, r.CompleteChore(context.Background(), chore, nil, testOwnerID, nil, &now, nil, false))
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "true", thing.State)
}

func TestCompletionActionsWaitForApproval(t *testing.T) {
	r, db := newTestChoreRepo(t)
	thing := tModel.Thing{UserID: testOwnerID, Type: "number", State: "5"}
	require.NoError(t, db.Create(&thing).Error)
	chore := createChore(t, db, "approval", testOwnerID, false, nil)
	chore.RequireApproval = true
	chore.Status = chModel.ChoreStatusPendingApproval
	chore.CompletionActions = []chModel.CompletionAction{{ThingID: thing.ID, Operation: "add", Value: "1"}}
	require.NoError(t, db.Save(chore).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&chModel.ChoreHistory{ChoreID: chore.ID, CompletedBy: testOwnerID, PerformedAt: &now, Status: chModel.ChoreHistoryStatusPendingApproval}).Error)
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "5", thing.State)
	require.NoError(t, r.ApproveChore(context.Background(), chore, testOwnerID, nil, nil, false))
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "6", thing.State)
	require.Error(t, r.ApproveChore(context.Background(), chore, testOwnerID, nil, nil, false))
	require.NoError(t, db.First(&thing, thing.ID).Error)
	require.Equal(t, "6", thing.State)
}
