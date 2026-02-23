package label

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lModel "donetick.com/core/internal/label/model"
	lRepo "donetick.com/core/internal/label/repo"
	uModel "donetick.com/core/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&lModel.Label{}))
	return db
}

func TestCreateLabelPersistsCircleID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupHandlerTestDB(t)
	repo := lRepo.NewLabelRepository(db, nil)
	handler := NewHandler(repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"name":"Shared","color":"#fff"}`
	req, err := http.NewRequest(http.MethodPost, "/api/v1/labels", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	user := &uModel.UserDetails{
		User: uModel.User{
			ID:       1,
			CircleID: 42,
		},
	}
	c.Set("id", user)

	handler.createLabel(c)

	require.Equal(t, http.StatusOK, w.Code)

	var created lModel.Label
	require.NoError(t, db.First(&created).Error)
	require.NotNil(t, created.CircleID)
	require.Equal(t, 42, *created.CircleID)
	require.Equal(t, 1, created.CreatedBy)
}
