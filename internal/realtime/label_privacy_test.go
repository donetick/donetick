package realtime

import (
	"context"
	"path/filepath"
	"testing"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	lModel "donetick.com/core/internal/label/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEventsHidePrivateLabelsPerRecipient(t *testing.T) {
	labels := []lModel.Label{{ID: 1, CreatedBy: 1}, {ID: 2, CreatedBy: 1, IsShared: true}}
	chore := &chModel.Chore{ID: 1, LabelsV2: &labels}
	for _, event := range []*Event{
		NewChoreCreatedEvent(chore, nil),
		NewChoreUpdatedEvent(chore, nil, nil, nil),
	} {
		other := event.forUser(2)
		var visible *chModel.Chore
		switch data := other.Data.(type) {
		case *ChoreCreatedData:
			visible = data.Chore
		case *ChoreEventData:
			visible = data.Chore
		}
		require.Len(t, *visible.LabelsV2, 1)
		require.Equal(t, 2, (*visible.LabelsV2)[0].ID)
		require.Same(t, event, event.forUser(1))
		require.Len(t, *chore.LabelsV2, 2, "redacting one recipient must not mutate another's payload")
	}
}

func TestBroadcastRestoresRecipientPrivateLabels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "broadcast.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	circleID := 1
	labels := []lModel.Label{
		{Name: "Alice private", CreatedBy: 1, CircleID: &circleID},
		{Name: "Bob private", CreatedBy: 2, CircleID: &circleID},
		{Name: "Shared", CreatedBy: 2, CircleID: &circleID, IsShared: true},
	}
	require.NoError(t, db.Create(&labels).Error)
	chore := chModel.Chore{Name: "Shared task", CircleID: circleID, CreatedBy: 2, IsActive: true}
	require.NoError(t, db.Create(&chore).Error)
	for _, label := range labels {
		require.NoError(t, db.Create(&chModel.ChoreLabels{ChoreID: chore.ID, LabelID: label.ID, UserID: label.CreatedBy}).Error)
	}
	cfg := &config.Config{}
	cfg.RealTimeConfig.Enabled = true
	cfg.RealTimeConfig.MaxConnections = 10
	cfg.RealTimeConfig.MaxConnectionsPerUser = 10
	repository := chRepo.NewChoreRepository(db, cfg)
	pool := NewConnectionPool(circleID, &cfg.RealTimeConfig)
	service := &RealTimeService{
		config: &cfg.RealTimeConfig, started: true, stats: &ServiceStats{},
		connectionPools: map[int]*ConnectionPool{circleID: pool},
	}
	broadcaster := NewEventBroadcaster(service, cfg, repository)
	connections := make([]*Connection, 3)
	for i := range connections {
		connections[i] = NewConnection(string(rune('a'+i)), circleID, i+1, nil, nil, nil)
		require.NoError(t, pool.AddConnection(connections[i]))
	}
	actorChore, err := repository.GetChore(context.Background(), chore.ID, 2, circleID)
	require.NoError(t, err)
	require.Len(t, *actorChore.LabelsV2, 2)
	broadcaster.BroadcastChoreUpdated(actorChore, nil, nil, nil)
	for i, conn := range connections {
		select {
		case event := <-conn.Send:
			visible := event.Data.(*ChoreEventData).Chore
			for _, label := range *visible.LabelsV2 {
				require.True(t, label.IsShared || label.CreatedBy == i+1, "private label leaked")
			}
			if i < 2 {
				require.Len(t, *visible.LabelsV2, 2)
			} else {
				require.Len(t, *visible.LabelsV2, 1)
			}
		default:
			t.Fatal("missing broadcast")
		}
	}
	require.Len(t, *actorChore.LabelsV2, 2, "broadcast must not mutate API response")
	for _, label := range *actorChore.LabelsV2 {
		require.True(t, label.IsShared || label.CreatedBy == 2)
	}

	// If loading fails, do not send an incomplete or unredacted fallback payload.
	require.NoError(t, db.Migrator().DropTable(&lModel.Label{}))
	broadcaster.BroadcastChoreUpdated(actorChore, nil, nil, nil)
	for _, conn := range connections {
		require.Empty(t, conn.Send)
	}
}
