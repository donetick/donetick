package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestPasswordAuthDisabledGuard verifies the #438 guard: when applied, any
// password-auth request is rejected with 403 and a clear, SSO-pointing message,
// before the real handler runs.
func TestPasswordAuthDisabledGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handlerCalled := false
	g := r.Group("api/v1/auth")
	g.Use(passwordAuthDisabled())
	g.POST("login", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body=%s)", w.Code, w.Body.String())
	}
	if handlerCalled {
		t.Fatal("login handler ran despite password auth being disabled")
	}
	if !strings.Contains(w.Body.String(), "SSO") {
		t.Errorf("expected an SSO-pointing message, got %s", w.Body.String())
	}
}

// TestPasswordAuthEnabledByDefault confirms the guard is opt-in: without it,
// the password-auth handler runs normally.
func TestPasswordAuthEnabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handlerCalled := false
	g := r.Group("api/v1/auth")
	g.POST("login", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !handlerCalled || w.Code != http.StatusOK {
		t.Fatalf("expected login handler to run (200), got %d called=%v", w.Code, handlerCalled)
	}
}
