package chore

import (
	"context"
	"testing"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Integration tests for the repository layer. They run against a real (in-memory)
// SQLite database migrated with the production migration system — no mocks — so
// they exercise real GORM behaviour and SQL. This is the reference pattern for
// DB-backed tests in this repo. See TESTING.md.
//
// We use SQLite in-memory here for fast feedback; the production Postgres path
// is worth covering with testcontainers as a follow-up, since GORM/SQL can
// differ across drivers.

func setupTestDB(t *testing.T) (*ChoreRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory database")

	require.NoError(t, database.Migration(db), "failed to run migrations")

	cfg := &config.Config{Database: config.DatabaseConfig{Type: "sqlite"}}
	return NewChoreRepository(db, cfg), db
}

func newOneTimeChore(name string, circleID, createdBy int) *chModel.Chore {
	now := time.Now().UTC()
	due := now.Add(24 * time.Hour)
	return &chModel.Chore{
		Name:          name,
		FrequencyType: chModel.FrequencyTypeOnce,
		NextDueDate:   &due,
		CircleID:      circleID,
		CreatedBy:     createdBy,
		AssignedTo:    &createdBy,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestChoreRepository_CreateAndGet(t *testing.T) {
	repo, _ := setupTestDB(t)
	ctx := context.Background()

	const userID, circleID = 1, 1
	chore := newOneTimeChore("Take out the trash", circleID, userID)

	id, err := repo.CreateChore(ctx, chore)
	require.NoError(t, err)
	assert.NotZero(t, id, "created chore should have a non-zero id")

	got, err := repo.GetChore(ctx, id, userID, circleID)
	require.NoError(t, err)
	assert.Equal(t, "Take out the trash", got.Name)
	assert.True(t, got.IsActive)
	assert.Positive(t, got.SyncVersion, "CreateChore should assign a sync version")
}

func TestChoreRepository_GetChore_WrongCircleIsNotVisible(t *testing.T) {
	repo, _ := setupTestDB(t)
	ctx := context.Background()

	id, err := repo.CreateChore(ctx, newOneTimeChore("Circle 1 chore", 1, 1))
	require.NoError(t, err)

	// A user querying under a different circle must not see the chore.
	_, err = repo.GetChore(ctx, id, 1, 2)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestChoreRepository_CompleteChore_RecordsHistoryAndDeactivates(t *testing.T) {
	repo, _ := setupTestDB(t)
	ctx := context.Background()

	const userID, circleID = 1, 1
	chore := newOneTimeChore("Water the plants", circleID, userID)
	id, err := repo.CreateChore(ctx, chore)
	require.NoError(t, err)
	chore.ID = id

	completedAt := time.Now().UTC()
	// A one-time chore has no next due date; applyPoints=false keeps this focused
	// on the completion + history behaviour.
	err = repo.CompleteChore(ctx, chore, nil, userID, nil, &completedAt, nil, false)
	require.NoError(t, err)

	// History should now contain exactly one completed record.
	history, err := repo.GetChoreHistory(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, chModel.ChoreHistoryStatusCompleted, history[0].Status)
	assert.Equal(t, userID, history[0].CompletedBy)

	// A completed one-time chore should be deactivated.
	got, err := repo.GetChore(ctx, id, userID, circleID)
	require.NoError(t, err)
	assert.False(t, got.IsActive, "one-time chore should be inactive after completion")
}

func TestChoreRepository_CompleteChore_AwardsPoints(t *testing.T) {
	repo, db := setupTestDB(t)
	ctx := context.Background()

	const userID, circleID = 1, 1

	// The user must belong to the circle for points to accrue.
	require.NoError(t, db.Create(&cModel.UserCircle{
		UserID: userID, CircleID: circleID, IsActive: true, Points: 0,
	}).Error)

	points := 5
	chore := newOneTimeChore("Do the dishes", circleID, userID)
	chore.Points = &points
	id, err := repo.CreateChore(ctx, chore)
	require.NoError(t, err)
	chore.ID = id

	completedAt := time.Now().UTC()
	err = repo.CompleteChore(ctx, chore, nil, userID, nil, &completedAt, nil, true /* applyPoints */)
	require.NoError(t, err)

	var uc cModel.UserCircle
	require.NoError(t, db.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&uc).Error)
	assert.Equal(t, 5, uc.Points, "completing a chore worth 5 points should award 5 points")
}
