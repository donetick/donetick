package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
)

func setAuthCookieHeader(t *testing.T, cfg *config.Config, maxAge int) (string, *http.Cookie) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	NewTokenService(nil, nil, cfg).SetAuthCookie(c, "refresh_token", "token-value", maxAge)

	header := rec.Header().Get("Set-Cookie")
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d (Set-Cookie: %q)", len(cookies), header)
	}
	return header, cookies[0]
}

func TestAuthCookieSecureByDefault(t *testing.T) {
	header, cookie := setAuthCookieHeader(t, &config.Config{}, 3600)

	if !cookie.Secure || !strings.Contains(header, "Secure") {
		t.Errorf("expected Secure cookie by default, got %q", header)
	}
	if !cookie.HttpOnly {
		t.Errorf("expected HttpOnly cookie, got %q", header)
	}
	if cookie.Path != "/" {
		t.Errorf("expected Path=/, got %q", header)
	}
	if cookie.Name != "refresh_token" || cookie.Value != "token-value" {
		t.Errorf("unexpected name/value, got %q", header)
	}
	if !strings.Contains(header, "Max-Age=3600") {
		t.Errorf("expected Max-Age=3600, got %q", header)
	}
}

func TestAuthCookieInsecureWhenConfigured(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{InsecureCookies: true}}
	header, cookie := setAuthCookieHeader(t, cfg, -1)

	if cookie.Secure || strings.Contains(header, "Secure") {
		t.Errorf("expected no Secure flag with server.insecure_cookies, got %q", header)
	}
	if !cookie.HttpOnly {
		t.Errorf("expected HttpOnly cookie, got %q", header)
	}
	if cookie.Path != "/" {
		t.Errorf("expected Path=/, got %q", header)
	}
	if !strings.Contains(header, "Max-Age=0") {
		t.Errorf("expected Max-Age=0 for maxAge -1, got %q", header)
	}
}
