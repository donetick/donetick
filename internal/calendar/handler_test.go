package calendar

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"donetick.com/core/config"
	sModel "donetick.com/core/external/payment/model"
	auth "donetick.com/core/internal/auth"
	calRepo "donetick.com/core/internal/calendar/repo"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	testUserID   = 7
	testCircleID = 1
)

// newTestHandler builds a handler backed by a real sqlite database, plus a
// router that authenticates every request as a plus-member testUserID. The
// instance is self-hosted, which the user repo always treats as plus.
func newTestHandler(t *testing.T) (*Handler, *gorm.DB, *gin.Engine) {
	t.Helper()
	return newTestHandlerWithConfig(t, func(cfg *config.Config) {}, true)
}

// newTestHandlerWithConfig builds the test handler with a customized config and
// control over whether the authenticated caller is a plus member.
func newTestHandlerWithConfig(t *testing.T, customize func(*config.Config), callerIsPlus bool) (*Handler, *gorm.DB, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "test_calendar.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.Server.PublicHost = "https://donetick.example.com"
	customize(cfg)

	h := NewHandler(
		chRepo.NewChoreRepository(db, cfg),
		uRepo.NewUserRepository(db, cfg),
		calRepo.NewCalendarRepository(db),
		cfg,
	)

	if err := db.Create(&uModel.User{ID: testUserID, DisplayName: "Test User", Username: "test", CircleID: testCircleID}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// IsPlusMember() reads Expiration, which the real auth middleware fills in
	// from the subscription join (or the self-hosted default).
	identity := &uModel.UserDetails{User: uModel.User{ID: testUserID, CircleID: testCircleID}}
	if callerIsPlus {
		future := time.Now().UTC().Add(24 * time.Hour)
		identity.Expiration = &future
	}

	router := gin.New()
	authed := router.Group("")
	authed.Use(func(c *gin.Context) {
		c.Set("id", identity)
		c.Next()
	}, auth.RequirePlusMemberMiddleware())
	authed.GET("/url", h.GetCalendarURL)
	authed.POST("/url", h.CreateCalendarURL)
	authed.POST("/url/rotate", h.RotateCalendarURL)
	authed.DELETE("/url", h.RevokeCalendarURL)
	router.GET("/feed/:token", h.ServeCalendar)

	return h, db, router
}

// urlResponse is the decoded body of the calendar URL endpoints.
type urlResponse struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

func callJSON(t *testing.T, router *gin.Engine, method, path string) (int, urlResponse) {
	t.Helper()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, nil))

	var body urlResponse
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response %q: %v", w.Body.String(), err)
		}
	}
	return w.Code, body
}

// tokenFromURL extracts the bare token from a generated subscription URL.
func tokenFromURL(t *testing.T, url string) string {
	t.Helper()

	idx := strings.LastIndex(url, "/")
	if idx < 0 {
		t.Fatalf("malformed calendar URL: %q", url)
	}
	return strings.TrimSuffix(url[idx+1:], ".ics")
}

// The authenticated management routes and the public feed route share the
// "api/v1/chores/calendar" prefix, mixing a static segment with a wildcard.
// Gin panics on some conflicting shapes, so assert the real patterns register
// and route as intended.
func TestRoutePatternsDoNotConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()

	managed := router.Group("api/v1/chores/calendar")
	{
		managed.GET("/url", func(c *gin.Context) { c.String(http.StatusOK, "url") })
		managed.POST("/url", func(c *gin.Context) { c.String(http.StatusOK, "create") })
		managed.POST("/url/rotate", func(c *gin.Context) { c.String(http.StatusOK, "rotate") })
		managed.DELETE("/url", func(c *gin.Context) { c.String(http.StatusOK, "revoke") })
	}
	public := router.Group("api/v1/chores/calendar")
	{
		public.GET("/:token", func(c *gin.Context) { c.String(http.StatusOK, "feed:"+c.Param("token")) })
	}

	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{http.MethodGet, "/api/v1/chores/calendar/url", "url"},
		{http.MethodPost, "/api/v1/chores/calendar/url", "create"},
		{http.MethodPost, "/api/v1/chores/calendar/url/rotate", "rotate"},
		{http.MethodDelete, "/api/v1/chores/calendar/url", "revoke"},
		{http.MethodGet, "/api/v1/chores/calendar/abc123.ics", "feed:abc123.ics"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if w.Body.String() != tt.expected {
				t.Errorf("routed to %q, want %q", w.Body.String(), tt.expected)
			}
		})
	}
}

func TestGenerateCalendarToken(t *testing.T) {
	token, err := generateCalendarToken()
	if err != nil {
		t.Fatalf("generateCalendarToken returned error: %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not raw-url base64: %v", err)
	}
	if len(raw) != calendarTokenBytes {
		t.Errorf("expected %d bytes of entropy, got %d", calendarTokenBytes, len(raw))
	}
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token contains characters needing URL escaping: %q", token)
	}
}

func TestGenerateCalendarToken_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		token, err := generateCalendarToken()
		if err != nil {
			t.Fatalf("generateCalendarToken returned error: %v", err)
		}
		if seen[token] {
			t.Fatalf("generated a duplicate token: %q", token)
		}
		seen[token] = true
	}
}

func TestBaseURL_PrefersConfiguredPublicHost(t *testing.T) {
	h := &Handler{publicHost: "https://donetick.example.com"}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/url", nil)
	// A forged Host must not leak into the generated URL.
	c.Request.Host = "attacker.example.net"
	c.Request.Header.Set("X-Forwarded-Proto", "http")

	if got := h.baseURL(c); got != "https://donetick.example.com" {
		t.Errorf("baseURL() = %q, want the configured public host", got)
	}
}

func TestBaseURL_NormalizesPublicHost(t *testing.T) {
	tests := []struct {
		name       string
		publicHost string
		expected   string
	}{
		{"trailing slash trimmed", "https://dt.example.com/", "https://dt.example.com"},
		{"scheme added when missing", "dt.example.com", "https://dt.example.com"},
		{"http preserved", "http://localhost:2021", "http://localhost:2021"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{publicHost: tt.publicHost}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/url", nil)

			if got := h.baseURL(c); got != tt.expected {
				t.Errorf("baseURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBaseURL_FallsBackToRequest(t *testing.T) {
	tests := []struct {
		name     string
		fwdProto string
		expected string
	}{
		{"no header", "", "http://dt.local"},
		{"https", "https", "https://dt.local"},
		{"http", "http", "http://dt.local"},
		{"garbage header ignored", "javascript:", "http://dt.local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{} // public_host not configured
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/url", nil)
			c.Request.Host = "dt.local"
			if tt.fwdProto != "" {
				c.Request.Header.Set("X-Forwarded-Proto", tt.fwdProto)
			}

			if got := h.baseURL(c); got != tt.expected {
				t.Errorf("baseURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCalendarURL_Shape(t *testing.T) {
	h := &Handler{publicHost: "https://dt.example.com"}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/url", nil)

	got := h.calendarURL(c, "tok123")
	want := "https://dt.example.com/api/v1/chores/calendar/tok123.ics"
	if got != want {
		t.Errorf("calendarURL() = %q, want %q", got, want)
	}
}

func TestGetCalendarURL_NotGeneratedYet(t *testing.T) {
	_, _, router := newTestHandler(t)

	code, body := callJSON(t, router, http.MethodGet, "/url")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 before a URL is generated, got %d", code)
	}
	if body.URL != "" {
		t.Errorf("expected no URL in response, got %q", body.URL)
	}
}

func TestCreateCalendarURL_IsIdempotent(t *testing.T) {
	_, _, router := newTestHandler(t)

	code, created := callJSON(t, router, http.MethodPost, "/url")
	if code != http.StatusCreated {
		t.Fatalf("expected 201 on first create, got %d", code)
	}
	if created.URL == "" {
		t.Fatal("expected a URL in the create response")
	}

	code, again := callJSON(t, router, http.MethodPost, "/url")
	if code != http.StatusOK {
		t.Errorf("expected 200 on repeat create, got %d", code)
	}
	if again.URL != created.URL {
		t.Errorf("repeat create changed the URL:\n  first:  %q\n  second: %q", created.URL, again.URL)
	}

	code, fetched := callJSON(t, router, http.MethodGet, "/url")
	if code != http.StatusOK {
		t.Errorf("expected 200 from GET after create, got %d", code)
	}
	if fetched.URL != created.URL {
		t.Errorf("GET returned a different URL than create: %q vs %q", fetched.URL, created.URL)
	}
}

func TestRotateCalendarURL_InvalidatesOldToken(t *testing.T) {
	_, _, router := newTestHandler(t)

	_, created := callJSON(t, router, http.MethodPost, "/url")
	oldToken := tokenFromURL(t, created.URL)

	code, rotated := callJSON(t, router, http.MethodPost, "/url/rotate")
	if code != http.StatusOK {
		t.Fatalf("expected 200 from rotate, got %d", code)
	}
	if rotated.URL == created.URL {
		t.Fatal("rotate returned the same URL")
	}
	newToken := tokenFromURL(t, rotated.URL)

	// The old URL must stop working immediately; the new one must work.
	if code := serveFeed(t, router, oldToken); code != http.StatusForbidden {
		t.Errorf("expected 403 for rotated-away token, got %d", code)
	}
	if code := serveFeed(t, router, newToken); code != http.StatusOK {
		t.Errorf("expected 200 for rotated-in token, got %d", code)
	}
}

func TestRevokeCalendarURL(t *testing.T) {
	_, _, router := newTestHandler(t)

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/url", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from revoke, got %d", w.Code)
	}

	if code := serveFeed(t, router, token); code != http.StatusForbidden {
		t.Errorf("expected 403 for revoked token, got %d", code)
	}
	if code, _ := callJSON(t, router, http.MethodGet, "/url"); code != http.StatusNotFound {
		t.Errorf("expected 404 from GET after revoke, got %d", code)
	}
}

func TestRevokeCalendarURL_NoTokenIsNotAnError(t *testing.T) {
	_, _, router := newTestHandler(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/url", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 when revoking a non-existent token, got %d", w.Code)
	}
}

func TestEscapeICalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello world", "hello world"},
		{"semicolon", "a;b", "a\\;b"},
		{"comma", "a,b", "a\\,b"},
		{"backslash", "a\\b", "a\\\\b"},
		{"newline", "a\nb", "a\\nb"},
		{"crlf", "a\r\nb", "a\\nb"},
		{"carriage return", "a\rb", "ab"},
		{"combined", "hello; world,\nfoo\\bar", "hello\\; world\\,\\nfoo\\\\bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeICalText(tt.input)
			if result != tt.expected {
				t.Errorf("escapeICalText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFoldLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLines int // 0 means don't check line count
	}{
		{"short line", "SHORT:value", 1},
		{"exactly 75", "DESCRIPTION:" + strings.Repeat("x", 63), 1},
		{"76 chars needs folding", "DESCRIPTION:" + strings.Repeat("x", 64), 2},
		{"very long line", "DESCRIPTION:" + strings.Repeat("x", 200), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldLine(tt.input)

			// Must end with \r\n
			if !strings.HasSuffix(result, "\r\n") {
				t.Error("folded line must end with CRLF")
			}

			// Split into lines and verify each
			lines := strings.Split(strings.TrimSuffix(result, "\r\n"), "\r\n")

			if tt.maxLines > 0 && len(lines) != tt.maxLines {
				t.Errorf("expected %d lines, got %d: %q", tt.maxLines, len(lines), result)
			}

			// First line must be <= 75 octets
			if len(lines[0]) > 75 {
				t.Errorf("first line is %d octets, max 75: %q", len(lines[0]), lines[0])
			}

			// Continuation lines must start with space and be <= 75 octets total
			for i := 1; i < len(lines); i++ {
				if lines[i][0] != ' ' {
					t.Errorf("continuation line %d must start with space: %q", i, lines[i])
				}
				if len(lines[i]) > 75 {
					t.Errorf("continuation line %d is %d octets, max 75: %q", i, len(lines[i]), lines[i])
				}
			}
		})
	}
}

func TestFoldLine_DoesNotSplitMultiByteRunes(t *testing.T) {
	// Every rune here is 3 octets, so a naive 75-octet cut lands mid-rune.
	original := "SUMMARY:" + strings.Repeat("日", 60)
	folded := foldLine(original)

	for i, line := range strings.Split(strings.TrimSuffix(folded, "\r\n"), "\r\n") {
		if !utf8.ValidString(line) {
			t.Errorf("line %d is not valid UTF-8: %q", i, line)
		}
		if len(line) > 75 {
			t.Errorf("line %d is %d octets, max 75", i, len(line))
		}
	}

	// Unfolding must still reproduce the input exactly.
	unfolded := strings.TrimSuffix(strings.ReplaceAll(folded, "\r\n ", ""), "\r\n")
	if unfolded != original {
		t.Errorf("roundtrip failed:\n  original: %q\n  unfolded: %q", original, unfolded)
	}
}

func TestFoldLine_Roundtrip(t *testing.T) {
	// Verify that unfolding a folded line gives back the original
	original := "DESCRIPTION:" + strings.Repeat("abcdefghij", 20)
	folded := foldLine(original)

	// Unfold per RFC 5545: remove CRLF followed by a single space
	unfolded := strings.ReplaceAll(folded, "\r\n ", "")
	unfolded = strings.TrimSuffix(unfolded, "\r\n")

	if unfolded != original {
		t.Errorf("roundtrip failed:\n  original: %q\n  unfolded: %q", original, unfolded)
	}
}

func TestMapPriority(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 0},
		{1, 7},
		{2, 5},
		{3, 3},
		{4, 1},
		{5, 0}, // unknown
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("priority_%d", tt.input), func(t *testing.T) {
			result := mapPriority(tt.input)
			if result != tt.expected {
				t.Errorf("mapPriority(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPriorityName(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, ""},
		{1, "Low"},
		{2, "Medium"},
		{3, "High"},
		{4, "Urgent"},
		{5, ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("priority_%d", tt.input), func(t *testing.T) {
			result := priorityName(tt.input)
			if result != tt.expected {
				t.Errorf("priorityName(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no tags", "hello world", "hello world"},
		{"simple tag", "<b>bold</b>", "bold"},
		{"nested tags", "<div><p>text</p></div>", "text"},
		{"self closing", "before<br/>after", "beforeafter"},
		{"with attributes", `<a href="url">link</a>`, "link"},
		{"empty", "", ""},
		{"only tags", "<div></div>", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTMLTags(tt.input)
			if result != tt.expected {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildICalFeed_Empty(t *testing.T) {
	result := buildICalFeed([]*chModel.Chore{}, "Test User", "America/New_York")

	if !strings.Contains(result, "BEGIN:VCALENDAR") {
		t.Error("missing BEGIN:VCALENDAR")
	}
	if !strings.Contains(result, "END:VCALENDAR") {
		t.Error("missing END:VCALENDAR")
	}
	if !strings.Contains(result, "VERSION:2.0") {
		t.Error("missing VERSION:2.0")
	}
	if !strings.Contains(result, "PRODID:") {
		t.Error("missing PRODID")
	}
	if strings.Contains(result, "BEGIN:VEVENT") {
		t.Error("should not contain VEVENT for empty chore list")
	}
	if !strings.Contains(result, "X-WR-CALNAME:Donetick - Test User") {
		t.Error("missing calendar name")
	}
	if !strings.Contains(result, "X-WR-TIMEZONE:America/New_York") {
		t.Error("missing timezone")
	}
}

func TestBuildICalFeed_SkipsChoresWithoutDueDate(t *testing.T) {
	chores := []*chModel.Chore{
		{
			ID:          1,
			Name:        "No Due Date",
			NextDueDate: nil,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	if strings.Contains(result, "BEGIN:VEVENT") {
		t.Error("should not contain VEVENT for chore without due date")
	}
}

func TestBuildICalFeed_BasicChore(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	desc := "Clean the kitchen"
	points := 5

	chores := []*chModel.Chore{
		{
			ID:          42,
			Name:        "Kitchen Cleaning",
			Description: &desc,
			NextDueDate: &dueDate,
			Priority:    3,
			Points:      &points,
			Status:      0, // active/default status
		},
	}
	// Set UpdatedAt manually (it's embedded in gorm.Model)
	chores[0].UpdatedAt = updatedAt

	result := buildICalFeed(chores, "Test User", "UTC")

	checks := []struct {
		name     string
		contains string
	}{
		{"VEVENT start", "BEGIN:VEVENT"},
		{"VEVENT end", "END:VEVENT"},
		{"UID", "UID:chore-42@donetick"},
		{"DTSTART", "DTSTART:20250615T100000Z"},
		{"SUMMARY", "SUMMARY:Kitchen Cleaning"},
		{"PRIORITY", "PRIORITY:3"},
		{"STATUS", "STATUS:NEEDS-ACTION"},
		{"LAST-MODIFIED", "LAST-MODIFIED:20250601T120000Z"},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !strings.Contains(result, check.contains) {
				t.Errorf("missing %q in output:\n%s", check.contains, result)
			}
		})
	}

	// Check description contains the text and metadata
	if !strings.Contains(result, "DESCRIPTION:") {
		t.Error("missing DESCRIPTION")
	}
	if !strings.Contains(result, "Clean the kitchen") {
		t.Error("missing description text")
	}
}

func TestBuildICalFeed_CompletionWindow(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	window := 3 // 3 hours

	chores := []*chModel.Chore{
		{
			ID:               1,
			Name:             "Test",
			NextDueDate:      &dueDate,
			CompletionWindow: &window,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	// DTEND should be 3 hours after DTSTART
	if !strings.Contains(result, "DTEND:20250615T130000Z") {
		t.Errorf("expected DTEND 3 hours after start, got:\n%s", result)
	}
}

func TestBuildICalFeed_DefaultDuration(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	chores := []*chModel.Chore{
		{
			ID:          1,
			Name:        "Test",
			NextDueDate: &dueDate,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	// Default duration is 1 hour
	if !strings.Contains(result, "DTEND:20250615T110000Z") {
		t.Errorf("expected DTEND 1 hour after start (default), got:\n%s", result)
	}
}

func TestBuildICalFeed_InProgressStatus(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	chores := []*chModel.Chore{
		{
			ID:          1,
			Name:        "Test",
			NextDueDate: &dueDate,
			Status:      chModel.ChoreStatusInProgress,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	if !strings.Contains(result, "STATUS:IN-PROCESS") {
		t.Errorf("expected IN-PROCESS status, got:\n%s", result)
	}
}

func TestBuildICalFeed_SpecialCharactersInName(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	chores := []*chModel.Chore{
		{
			ID:          1,
			Name:        "Buy groceries; milk, eggs, bread",
			NextDueDate: &dueDate,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	if !strings.Contains(result, "SUMMARY:Buy groceries\\; milk\\, eggs\\, bread") {
		t.Errorf("special characters not properly escaped in:\n%s", result)
	}
}

func TestBuildICalFeed_HTMLDescription(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	desc := "<p>Clean the <b>kitchen</b> thoroughly</p>"

	chores := []*chModel.Chore{
		{
			ID:          1,
			Name:        "Test",
			Description: &desc,
			NextDueDate: &dueDate,
		},
	}

	result := buildICalFeed(chores, "Test", "")

	if strings.Contains(result, "<p>") || strings.Contains(result, "<b>") {
		t.Errorf("HTML tags should be stripped from description:\n%s", result)
	}
	if !strings.Contains(result, "Clean the kitchen thoroughly") {
		t.Errorf("description text missing after HTML stripping:\n%s", result)
	}
}

func TestBuildICalFeed_MultipleChores(t *testing.T) {
	due1 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	due2 := time.Date(2025, 6, 16, 14, 0, 0, 0, time.UTC)

	chores := []*chModel.Chore{
		{ID: 1, Name: "Chore One", NextDueDate: &due1},
		{ID: 2, Name: "Chore Two", NextDueDate: &due2},
		{ID: 3, Name: "No Date", NextDueDate: nil}, // should be skipped
	}

	result := buildICalFeed(chores, "Test", "")

	eventCount := strings.Count(result, "BEGIN:VEVENT")
	if eventCount != 2 {
		t.Errorf("expected 2 VEVENTs, got %d", eventCount)
	}

	if !strings.Contains(result, "chore-1@donetick") {
		t.Error("missing UID for chore 1")
	}
	if !strings.Contains(result, "chore-2@donetick") {
		t.Error("missing UID for chore 2")
	}
	if strings.Contains(result, "chore-3@donetick") {
		t.Error("chore 3 (no due date) should not be included")
	}
}

func TestBuildICalFeed_CRLFLineEndings(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	chores := []*chModel.Chore{
		{ID: 1, Name: "Test", NextDueDate: &dueDate},
	}

	result := buildICalFeed(chores, "Test", "")

	// Every line should end with \r\n (RFC 5545 requirement)
	lines := strings.Split(result, "\r\n")
	// The last element after split will be empty string (after final \r\n)
	if lines[len(lines)-1] != "" {
		t.Error("file should end with CRLF")
	}

	// Verify no bare \n exists (that isn't part of \r\n)
	withoutCRLF := strings.ReplaceAll(result, "\r\n", "")
	if strings.Contains(withoutCRLF, "\n") {
		t.Error("found bare LF not part of CRLF")
	}
}

func TestBuildChoreDescription_AllFields(t *testing.T) {
	desc := "Do the thing"
	points := 10

	ch := &chModel.Chore{
		Description:   &desc,
		FrequencyType: chModel.FrequencyTypeDaily,
		Points:        &points,
		Priority:      3,
	}

	result := buildChoreDescription(ch)

	if !strings.Contains(result, "Do the thing") {
		t.Error("missing description text")
	}
	if !strings.Contains(result, "Repeats: daily") {
		t.Error("missing frequency info")
	}
	if !strings.Contains(result, "Points: 10") {
		t.Error("missing points")
	}
	if !strings.Contains(result, "Priority: High") {
		t.Error("missing priority")
	}
}

func TestBuildICalFeed_DescriptionUsesICalNewlineEscape(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	desc := "Do the thing"
	points := 10

	chores := []*chModel.Chore{
		{
			ID:            1,
			Name:          "Test",
			Description:   &desc,
			NextDueDate:   &dueDate,
			FrequencyType: chModel.FrequencyTypeDaily,
			Points:        &points,
		},
	}

	// Unfold so the DESCRIPTION property can be inspected as one line.
	result := strings.ReplaceAll(buildICalFeed(chores, "Test", ""), "\r\n ", "")

	if !strings.Contains(result, `Do the thing\nRepeats: daily\nPoints: 10`) {
		t.Errorf("description separators should be the iCal \\n escape, got:\n%s", result)
	}
	// A doubled backslash would render as a literal "\n" in calendar clients.
	if strings.Contains(result, `\\n`) {
		t.Errorf("description newlines are double-escaped:\n%s", result)
	}
}

func TestBuildChoreDescription_OnceFrequencyOmitted(t *testing.T) {
	ch := &chModel.Chore{
		FrequencyType: chModel.FrequencyTypeOnce,
	}

	result := buildChoreDescription(ch)

	if strings.Contains(result, "Repeats") {
		t.Error("once frequency should not show 'Repeats'")
	}
}

func TestBuildChoreDescription_Empty(t *testing.T) {
	ch := &chModel.Chore{}

	result := buildChoreDescription(ch)

	if result != "" {
		t.Errorf("expected empty description, got %q", result)
	}
}

func TestBuildChoreDescription_ZeroPoints(t *testing.T) {
	points := 0
	ch := &chModel.Chore{
		Points: &points,
	}

	result := buildChoreDescription(ch)

	if strings.Contains(result, "Points") {
		t.Error("zero points should not be included")
	}
}

// serveFeed requests the public feed for a token and returns the status code.
func serveFeed(t *testing.T, router *gin.Engine, token string) int {
	t.Helper()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/feed/"+token+".ics", nil))
	return w.Code
}

func TestServeCalendar_UnknownToken(t *testing.T) {
	_, _, router := newTestHandler(t)

	if code := serveFeed(t, router, "totally-made-up-token"); code != http.StatusForbidden {
		t.Errorf("expected status 403 for an unknown token, got %d", code)
	}
}

func TestServeCalendar_ValidTokenServesFeed(t *testing.T) {
	_, db, router := newTestHandler(t)

	dueDate := time.Now().UTC().Add(24 * time.Hour)
	chore := &chModel.Chore{
		Name:        "Water the plants",
		CircleID:    testCircleID,
		CreatedBy:   testUserID,
		IsActive:    true,
		NextDueDate: &dueDate,
	}
	if err := db.Create(chore).Error; err != nil {
		t.Fatalf("failed to seed chore: %v", err)
	}

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/feed/"+token+".ics", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("expected a text/calendar content type, got %q", ct)
	}
	if sniff := w.Header().Get("X-Content-Type-Options"); sniff != "nosniff" {
		t.Errorf("expected nosniff header, got %q", sniff)
	}
	if !strings.Contains(w.Body.String(), "BEGIN:VCALENDAR") {
		t.Errorf("response is not an iCal feed:\n%s", w.Body.String())
	}
}

func TestServeCalendar_WorksWithoutIcsSuffix(t *testing.T) {
	_, _, router := newTestHandler(t)

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/feed/"+token, nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 without the .ics suffix, got %d", w.Code)
	}
}

func TestServeCalendar_DisabledUser(t *testing.T) {
	_, db, router := newTestHandler(t)

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	if err := db.Model(&uModel.User{}).Where("id = ?", testUserID).Update("disabled", true).Error; err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	if code := serveFeed(t, router, token); code != http.StatusForbidden {
		t.Errorf("expected 403 for a disabled user, got %d", code)
	}
}

func TestManagementRoutes_RequirePlusMember(t *testing.T) {
	_, _, router := newTestHandlerWithConfig(t, func(cfg *config.Config) {}, false)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/url"},
		{http.MethodPost, "/url"},
		{http.MethodPost, "/url/rotate"},
		{http.MethodDelete, "/url"},
	}

	for _, req := range requests {
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(req.method, req.path, nil))

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 for a non-plus member, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// On donetick.com, plus status comes from an active subscription row rather
// than the self-hosted default.
func newHostedTestHandler(t *testing.T) (*Handler, *gorm.DB, *gin.Engine) {
	t.Helper()
	return newTestHandlerWithConfig(t, func(cfg *config.Config) {
		cfg.IsDoneTickDotCom = true
	}, true)
}

func TestServeCalendar_NonPlusOwnerIsRejected(t *testing.T) {
	_, _, router := newHostedTestHandler(t)

	// The URL is issued while the caller is a plus member...
	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	// ...but the owner has no active subscription row, so the feed itself must
	// refuse to serve. This is the lapsed-subscription path.
	if code := serveFeed(t, router, token); code != http.StatusForbidden {
		t.Errorf("expected 403 for a non-plus owner, got %d", code)
	}
}

func TestServeCalendar_PlusOwnerIsServed(t *testing.T) {
	_, db, router := newHostedTestHandler(t)

	expires := time.Now().UTC().Add(30 * 24 * time.Hour)
	subscription := &sModel.Subscription{
		ID:        "sub_test",
		UserID:    testUserID,
		CircleID:  testCircleID,
		Status:    "active",
		ExpiresAt: &expires,
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to seed subscription: %v", err)
	}

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	if code := serveFeed(t, router, token); code != http.StatusOK {
		t.Errorf("expected 200 for a plus owner, got %d", code)
	}
}

func TestServeCalendar_RecordsLastUsed(t *testing.T) {
	h, _, router := newTestHandler(t)

	_, created := callJSON(t, router, http.MethodPost, "/url")
	token := tokenFromURL(t, created.URL)

	stored, err := h.calRepo.GetByToken(t.Context(), token)
	if err != nil {
		t.Fatalf("failed to load token: %v", err)
	}
	if stored.LastUsedAt != nil {
		t.Fatal("expected last used to be unset before the feed is fetched")
	}

	if code := serveFeed(t, router, token); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	stored, err = h.calRepo.GetByToken(t.Context(), token)
	if err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if stored.LastUsedAt == nil {
		t.Error("expected last used to be recorded after serving the feed")
	}
}
