package xp

import (
	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	user "donetick.com/core/internal/user/model"
	userRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoutesAcceptJWTAndAPIToken(t *testing.T) {
	h, db, u := fixture(t)
	require.NoError(t, db.Model(&user.User{}).Where("id = ?", 1).Update("circle_id", 1).Error)
	u.Username = "owner"
	cfg := &config.Config{}
	cfg.Jwt.Secret = "local-test-signing-key"
	cfg.Jwt.SessionTime = time.Hour
	repo := userRepo.NewUserRepository(db, cfg)
	jwtMiddleware, err := auth.NewAuthMiddleware(cfg, repo, nil)
	require.NoError(t, err)
	token, _, err := jwtMiddleware.TokenGenerator(u)
	require.NoError(t, err)
	require.NoError(t, db.Create(&user.APIToken{UserID: 1, Name: "Local test", Token: "local-fixture-api-token"}).Error)
	r := gin.New()
	Routes(r, h, auth.NewMultiAuthMiddleware(jwtMiddleware, repo))
	for _, header := range []struct{ name, value string }{{"Authorization", "Bearer " + token}, {"secretkey", "local-fixture-api-token"}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/users/1/xp", nil)
		req.Header.Set(header.name, header.value)
		r.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code, w.Body.String())
	}
	for _, token := range []string{"", "invalid-fixture-token"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/users/1/xp", nil)
		req.Header.Set("secretkey", token)
		r.ServeHTTP(w, req)
		require.Equal(t, 401, w.Code)
	}
}
