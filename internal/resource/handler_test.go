package resource

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
)

// TestResourceExposesDisablePasswordAuth verifies the resource endpoint
// surfaces the disable_password_auth flag so the client can hide password
// login (#438).
func TestResourceExposesDisablePasswordAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, want := range []bool{true, false} {
		h := NewHandler(&config.Config{DisablePasswordAuth: want})
		r := gin.New()
		r.GET("api/v1/resource", h.getResource)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("got %d", w.Code)
		}
		var body struct {
			DisablePasswordAuth bool `json:"disable_password_auth"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.DisablePasswordAuth != want {
			t.Errorf("disable_password_auth = %v, want %v", body.DisablePasswordAuth, want)
		}
	}
}
