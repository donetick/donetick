package thing

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/project/model"
	tModel "donetick.com/core/internal/thing/model"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
)

// Things are created without a circle (Thing.CircleID stays 0). A trigger must
// still reschedule the owner's chores, and must skip chores the owner can no
// longer see, e.g. after being removed from a shared project.
func TestThingTriggerUsesOwnersCircleAndSkipsInvisibleChores(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "things.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	h := &Handler{choreRepo: chRepo.NewChoreRepository(db, cfg), tRepo: tRepo.NewThingRepository(db, cfg)}

	const circleID, thingOwner, projectOwner = 1, 2, 1
	for _, id := range []int{projectOwner, thingOwner} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: circleID, IsActive: true}).Error)
	}
	hidden := &pModel.Project{Name: "not shared with the thing owner", CircleID: circleID, CreatedBy: projectOwner, IsPrivate: true}
	require.NoError(t, db.Create(hidden).Error)

	visible := &chModel.Chore{Name: "water plants", CircleID: circleID, CreatedBy: thingOwner, IsActive: false}
	invisible := &chModel.Chore{Name: "secret", CircleID: circleID, CreatedBy: thingOwner, ProjectID: &hidden.ID, IsPrivate: true, IsActive: false}
	require.NoError(t, db.Create(visible).Error)
	require.NoError(t, db.Create(invisible).Error)

	thing := &tModel.Thing{Name: "sensor", UserID: thingOwner, Type: "boolean", State: "false"} // CircleID 0, like CreateThing leaves it
	require.NoError(t, db.Create(thing).Error)
	for _, choreID := range []int{visible.ID, invisible.ID} {
		require.NoError(t, db.Create(&tModel.ThingChore{ThingID: thing.ID, ChoreID: choreID, TriggerState: "true"}).Error)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("PUT", "/", nil)
	c.Set("id", &uModel.UserDetails{User: uModel.User{ID: thingOwner, CircleID: circleID}})
	thing.State = "true"
	require.False(t, EvaluateTriggerAndScheduleDueDate(h, c, thing))

	var got chModel.Chore
	require.NoError(t, db.First(&got, visible.ID).Error)
	require.True(t, got.IsActive, "the owner's visible chore is rescheduled")
	require.NotNil(t, got.NextDueDate)
	var untouched chModel.Chore
	require.NoError(t, db.First(&untouched, invisible.ID).Error)
	require.False(t, untouched.IsActive, "a chore the owner cannot see is left alone")
	require.Nil(t, untouched.NextDueDate)
}
