package chore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
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
