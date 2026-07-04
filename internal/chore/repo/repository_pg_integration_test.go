//go:build integration

package chore

import (
	"context"
	"testing"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Integration tests that run against a REAL PostgreSQL instance via
// testcontainers. Production uses Postgres, and GORM/SQL behavior can differ
// from SQLite (e.g. the sync-cursor uses `ON CONFLICT ... RETURNING`), so these
// guard the parts that unit-level SQLite tests can't. See be/TESTING.md.
//
// These are gated behind the `integration` build tag and require Docker, so the
// default `go test ./...` stays fast and Docker-free. Run them with:
//
//	go test -tags integration ./internal/chore/repo/
//
// CI runs them in a dedicated job (see backend go-build workflow).

func setupPostgresRepo(t *testing.T) *ChoreRepository {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("donetick_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")
	testcontainers.CleanupContainer(t, pg)

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "failed to connect to postgres")

	require.NoError(t, database.Migration(db), "failed to run migrations")

	cfg := &config.Config{Database: config.DatabaseConfig{Type: "postgres"}}
	return NewChoreRepository(db, cfg)
}

func TestChoreRepository_Postgres_CreateCompleteAwardsPoints(t *testing.T) {
	repo := setupPostgresRepo(t)
	ctx := context.Background()

	const userID, circleID = 1, 1
	// GetChore/points depend on real Postgres semantics; exercise the full
	// create -> complete -> points path end to end.
	require.NoError(t, repo.db.Create(&cModel.UserCircle{
		UserID: userID, CircleID: circleID, IsActive: true, Points: 0,
	}).Error)

	points := 7
	chore := newOneTimeChore("Vacuum living room", circleID, userID)
	chore.Points = &points
	id, err := repo.CreateChore(ctx, chore)
	require.NoError(t, err)
	assert.NotZero(t, id)
	chore.ID = id

	// SyncVersion comes from the Postgres-specific ON CONFLICT ... RETURNING path.
	got, err := repo.GetChore(ctx, id, userID, circleID)
	require.NoError(t, err)
	assert.Positive(t, got.SyncVersion)

	completedAt := time.Now().UTC()
	require.NoError(t, repo.CompleteChore(ctx, chore, nil, userID, nil, &completedAt, nil, true))

	history, err := repo.GetChoreHistory(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, chModel.ChoreHistoryStatusCompleted, history[0].Status)

	var uc cModel.UserCircle
	require.NoError(t, repo.db.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&uc).Error)
	assert.Equal(t, 7, uc.Points)
}
