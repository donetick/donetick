package chore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	"donetick.com/core/internal/events"
	lModel "donetick.com/core/internal/label/model"
	lRepo "donetick.com/core/internal/label/repo"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/storage"
	storageRepo "donetick.com/core/internal/storage/repo"
	stRepo "donetick.com/core/internal/subtask/repo"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
)

func TestDueDatesDiffer(t *testing.T) {
	dueDate := time.Date(2026, time.July, 28, 3, 59, 0, 0, time.UTC)
	sameInstantInAnotherLocation := dueDate.In(time.FixedZone("offset", -4*60*60))
	differentDueDate := dueDate.Add(time.Minute)

	tests := []struct {
		name       string
		oldDueDate *time.Time
		newDueDate *time.Time
		want       bool
	}{
		{name: "both nil", want: false},
		{name: "old nil", newDueDate: &dueDate, want: true},
		{name: "new nil", oldDueDate: &dueDate, want: true},
		{name: "same pointer", oldDueDate: &dueDate, newDueDate: &dueDate, want: false},
		{name: "equal values at different addresses", oldDueDate: &dueDate, newDueDate: &sameInstantInAnotherLocation, want: false},
		{name: "different values", oldDueDate: &dueDate, newDueDate: &differentDueDate, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dueDatesDiffer(test.oldDueDate, test.newDueDate); got != test.want {
				t.Fatalf("dueDatesDiffer() = %v, want %v", got, test.want)
			}
		})
	}
}

func newCreateChoreTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection, so the notification planner goroutine sees the same in-memory database.
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.Migration(db))
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: 1, CircleID: 1, Role: cModel.UserRoleAdmin, IsActive: true}).Error)

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	circleRepository := cRepo.NewCircleRepository(db)
	eventProducer := events.NewEventsProducer(cfg)
	eventProducer.Start(context.Background())
	h := &Handler{
		choreRepo:     chRepo.NewChoreRepository(db, cfg),
		circleRepo:    circleRepository,
		nPlanner:      nps.NewNotificationPlanner(nRepo.NewNotificationRepository(db), circleRepository),
		eventProducer: eventProducer,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: 1, CircleID: 1}})
		c.Next()
	})
	router.POST("/chores", h.CreateChore)
	return router, db
}

func TestCreateChoreFirstDueDate(t *testing.T) {
	tests := []struct {
		name          string
		frequencyType chModel.FrequencyType
		metadata      *chModel.FrequencyMetadata
		nextDueDate   time.Time
		want          time.Time
	}{
		{
			name:          "friday start date moves to saturday",
			frequencyType: chModel.FrequencyTypeDayOfTheWeek,
			metadata:      &chModel.FrequencyMetadata{Days: []*string{jsonPtr("wednesday"), jsonPtr("saturday")}},
			nextDueDate:   time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC),
			want:          time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC),
		},
		{
			name:          "start date on a selected day is kept",
			frequencyType: chModel.FrequencyTypeDayOfTheWeek,
			metadata:      &chModel.FrequencyMetadata{Days: []*string{jsonPtr("wednesday"), jsonPtr("saturday")}},
			nextDueDate:   time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC),
			want:          time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC),
		},
		{
			name:          "sunday start date wraps to wednesday",
			frequencyType: chModel.FrequencyTypeDayOfTheWeek,
			metadata:      &chModel.FrequencyMetadata{Days: []*string{jsonPtr("wednesday")}},
			nextDueDate:   time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC),
			want:          time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC),
		},
		{
			name:          "weekday is checked in the chore timezone",
			frequencyType: chModel.FrequencyTypeDayOfTheWeek,
			metadata:      &chModel.FrequencyMetadata{Days: []*string{jsonPtr("saturday")}, Timezone: "Asia/Tokyo"},
			nextDueDate:   time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC), // Fri 00:00 in Tokyo
			want:          time.Date(2026, 9, 18, 15, 0, 0, 0, time.UTC), // Sat 00:00 in Tokyo
		},
		{
			name:          "weekly chore keeps its start date",
			frequencyType: chModel.FrequencyTypeWeekly,
			nextDueDate:   time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC),
			want:          time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, db := newCreateChoreTestRouter(t)
			body, err := json.Marshal(map[string]any{
				"name":              test.name,
				"frequencyType":     test.frequencyType,
				"frequencyMetadata": test.metadata,
				"nextDueDate":       test.nextDueDate,
				"assignStrategy":    chModel.AssignmentStrategyRandom,
			})
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/chores", bytes.NewReader(body)))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			var response struct {
				Res int `json:"res"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			var chore chModel.Chore
			require.NoError(t, db.First(&chore, response.Res).Error)
			require.NotNil(t, chore.NextDueDate)
			require.True(t, chore.NextDueDate.Equal(test.want), "first due date = %v, want %v", chore.NextDueDate.UTC(), test.want)
		})
	}
}

func newEditChoreTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// A file rather than ":memory:": editing opens a transaction while other
	// queries run, so it needs more than one connection to the same database.
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "edit.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: 1, CircleID: 1, Role: cModel.UserRoleAdmin, IsActive: true}).Error)

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.Storage.BasePath = t.TempDir()
	circleRepository := cRepo.NewCircleRepository(db)
	eventProducer := events.NewEventsProducer(cfg)
	eventProducer.Start(context.Background())
	h := &Handler{
		choreRepo:     chRepo.NewChoreRepository(db, cfg),
		circleRepo:    circleRepository,
		lRepo:         lRepo.NewLabelRepository(db, cfg),
		stRepo:        stRepo.NewSubTasksRepository(db),
		tRepo:         tRepo.NewThingRepository(db, cfg),
		storage:       storage.NewLocalStorage(cfg),
		storageRepo:   storageRepo.NewStorageRepository(db, cfg),
		nPlanner:      nps.NewNotificationPlanner(nRepo.NewNotificationRepository(db), circleRepository),
		eventProducer: eventProducer,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: 1, CircleID: 1}})
		c.Next()
	})
	router.PUT("/chores", h.EditChore)
	return router, db
}

// Clients that don't send labelsV2 (for example older apps or API scripts) must
// be able to update a chore without touching its labels.
func TestEditChoreWithoutLabelsV2KeepsLabels(t *testing.T) {
	router, db := newEditChoreTestRouter(t)
	circleID := 1
	label := lModel.Label{Name: "garden", Color: "#4caf50", CircleID: &circleID, CreatedBy: 1}
	require.NoError(t, db.Create(&label).Error)
	chore := chModel.Chore{Name: "water plants", FrequencyType: chModel.FrequencyTypeOnce,
		AssignStrategy: chModel.AssignmentStrategyRandom, CircleID: 1, CreatedBy: 1, IsActive: true}
	require.NoError(t, db.Create(&chore).Error)
	require.NoError(t, db.Create(&chModel.ChoreLabels{ChoreID: chore.ID, LabelID: label.ID, UserID: 1}).Error)

	edit := func(body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/chores", bytes.NewReader(raw)))
		return recorder
	}
	labelCount := func() int64 {
		t.Helper()
		var count int64
		require.NoError(t, db.Model(&chModel.ChoreLabels{}).Where("chore_id = ?", chore.ID).Count(&count).Error)
		return count
	}
	body := map[string]any{
		"id":             chore.ID,
		"name":           "water the plants",
		"frequencyType":  chModel.FrequencyTypeOnce,
		"assignStrategy": chModel.AssignmentStrategyRandom,
	}

	recorder := edit(body)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 1, labelCount(), "an update without labelsV2 keeps the labels")
	var updated chModel.Chore
	require.NoError(t, db.First(&updated, chore.ID).Error)
	require.Equal(t, "water the plants", updated.Name)

	body["labelsV2"] = []map[string]int{}
	recorder = edit(body)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Zero(t, labelCount(), "an empty labelsV2 still removes the labels")
}
