package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/project/model"
	pRepo "donetick.com/core/internal/project/repo"
	uModel "donetick.com/core/internal/user/model"
)

// PUT /projects/:id is the only endpoint that returns a single project, and its
// guard is the ownership check inside UpdateProjectWithAccess. A request that
// sets no field must not slip past that check and hand the project back to
// somebody who cannot even see it.
func TestUpdateProjectDoesNotLeakAProjectTheCallerCannotSee(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test_projects.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	h := NewHandler(pRepo.NewProjectRepository(db, cfg, chRepo.NewChoreRepository(db, cfg)), chRepo.NewChoreRepository(db, cfg), nil, nil)

	const (
		circleID   = 1
		ownerID    = 1
		outsiderID = 2
	)
	secret := &pModel.Project{Name: "secret plans", CircleID: circleID, CreatedBy: ownerID, IsPrivate: true}
	if err := db.Create(secret).Error; err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/projects/1", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("id", &uModel.UserDetails{User: uModel.User{ID: outsiderID, CircleID: circleID}})

	h.updateProject(c)

	if w.Code == 200 {
		var body map[string]json.RawMessage
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		t.Fatalf("a user who cannot see the project got 200 and its contents: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret plans") {
		t.Fatalf("response leaked the project name: %s", w.Body.String())
	}
}

// The owner's own update still goes through, and a view-only request from
// somebody who can see the project keeps working.
func TestUpdateProjectStillWorksForTheOwnerAndForViewOnlyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test_projects.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	h := NewHandler(pRepo.NewProjectRepository(db, cfg, chRepo.NewChoreRepository(db, cfg)), chRepo.NewChoreRepository(db, cfg), nil, nil)

	const (
		circleID = 1
		ownerID  = 1
	)
	project := &pModel.Project{Name: "shared", CircleID: circleID, CreatedBy: ownerID}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	call := func(userID int, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/projects/1", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: userID, CircleID: circleID}})
		h.updateProject(c)
		return w
	}

	if w := call(ownerID, `{"name":"renamed"}`); w.Code != 200 {
		t.Fatalf("owner rename: got %d, body %s", w.Code, w.Body.String())
	}
	// A circle project is visible to the whole circle, so a non-owner may store
	// their own layout for it.
	if w := call(2, `{"view":"list"}`); w.Code != 200 {
		t.Fatalf("view-only request by a viewer: got %d, body %s", w.Code, w.Body.String())
	}
	if w := call(ownerID, `{}`); w.Code != 400 {
		t.Fatalf("empty request: got %d, want 400, body %s", w.Code, w.Body.String())
	}
}
