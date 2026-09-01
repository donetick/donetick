package calendar

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    chModel "donetick.com/core/internal/chore/model"
    "github.com/gin-gonic/gin"
)

func TestGenerateAndParseCalendarToken(t *testing.T) {
    h := &Handler{secret: "test-secret-key"}

    userID := 42
    token := h.generateCalendarToken(userID)

    // Token should contain the user ID
    if !strings.HasPrefix(token, "42-") {
        t.Errorf("token should start with '42-', got %q", token)
    }

    // Parse should return the same user ID
    parsedID, err := h.parseCalendarToken(token)
    if err != nil {
        t.Fatalf("parseCalendarToken returned error: %v", err)
    }
    if parsedID != userID {
        t.Errorf("expected userID %d, got %d", userID, parsedID)
    }
}

func TestParseCalendarToken_InvalidFormat(t *testing.T) {
    h := &Handler{secret: "test-secret-key"}

    tests := []struct {
        name  string
        token string
    }{
        {"empty", ""},
        {"no dash", "abc123"},
        {"non-numeric user ID", "abc-def"},
        {"wrong signature", "42-invalidsignature"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := h.parseCalendarToken(tt.token)
            if err == nil {
                t.Error("expected error, got nil")
            }
        })
    }
}

func TestParseCalendarToken_WrongSecret(t *testing.T) {
    h1 := &Handler{secret: "secret-one"}
    h2 := &Handler{secret: "secret-two"}

    token := h1.generateCalendarToken(42)

    _, err := h2.parseCalendarToken(token)
    if err == nil {
        t.Error("expected error when parsing token with different secret, got nil")
    }
}

func TestParseCalendarToken_DifferentUserIDs(t *testing.T) {
    h := &Handler{secret: "test-secret-key"}

    token1 := h.generateCalendarToken(1)
    token2 := h.generateCalendarToken(2)

    if token1 == token2 {
        t.Error("tokens for different users should be different")
    }

    // Try to tamper: take signature from user 1 and put user 2's ID
    parts := strings.SplitN(token1, "-", 2)
    tampered := fmt.Sprintf("2-%s", parts[1])

    _, err := h.parseCalendarToken(tampered)
    if err == nil {
        t.Error("expected error for tampered token, got nil")
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

func TestServeCalendar_InvalidToken(t *testing.T) {
    h := &Handler{secret: "test-secret"}

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Params = gin.Params{{Key: "token", Value: "invalid-token.ics"}}
    c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/chores/calendar/invalid-token.ics", nil)

    h.ServeCalendar(c)

    if w.Code != http.StatusForbidden {
        t.Errorf("expected status 403, got %d", w.Code)
    }
}

func TestServeCalendar_TamperedToken(t *testing.T) {
    h := &Handler{secret: "test-secret"}

    // Generate valid token for user 1, then change user ID to 2
    token := h.generateCalendarToken(1)
    parts := strings.SplitN(token, "-", 2)
    tampered := fmt.Sprintf("2-%s.ics", parts[1])

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Params = gin.Params{{Key: "token", Value: tampered}}
    c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/chores/calendar/"+tampered, nil)

    h.ServeCalendar(c)

    if w.Code != http.StatusForbidden {
        t.Errorf("expected status 403, got %d", w.Code)
    }
}
