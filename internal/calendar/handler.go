package calendar

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/auth"
	calModel "donetick.com/core/internal/calendar/model"
	calRepo "donetick.com/core/internal/calendar/repo"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"gorm.io/gorm"
)

const (
	calendarProdID = "-//Donetick//Chores Calendar//EN"
	// calendarTokenBytes is the entropy of a calendar token. The token is a
	// bearer credential sitting in a URL, so it must be infeasible to guess.
	calendarTokenBytes = 32
)

// Handler handles iCal calendar endpoints
type Handler struct {
	choreRepo *chRepo.ChoreRepository
	userRepo  *uRepo.UserRepository
	calRepo   *calRepo.CalendarRepository
	// publicHost is the operator-configured externally reachable base URL. It is
	// preferred over request headers, which a client can forge.
	publicHost string
}

// NewHandler creates a new calendar handler
func NewHandler(cr *chRepo.ChoreRepository, ur *uRepo.UserRepository, calr *calRepo.CalendarRepository, cfg *config.Config) *Handler {
	return &Handler{
		choreRepo:  cr,
		userRepo:   ur,
		calRepo:    calr,
		publicHost: cfg.Server.PublicHost,
	}
}

// generateCalendarToken returns a fresh random calendar token.
func generateCalendarToken() (string, error) {
	buf := make([]byte, calendarTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Raw URL encoding: no padding and no characters needing escaping in a path.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// calendarURL renders the subscription URL for a token.
func (h *Handler) calendarURL(c *gin.Context, token string) string {
	return fmt.Sprintf("%s/api/v1/chores/calendar/%s.ics", h.baseURL(c), token)
}

// baseURL resolves the public base URL for generated links. The configured
// public_host wins: the Host and X-Forwarded-Proto headers are client
// controlled, and a poisoned value would hand the user a calendar URL -- token
// included -- pointing at someone else's origin. The request is only used as a
// fallback for instances that never configured public_host.
func (h *Handler) baseURL(c *gin.Context) string {
	if h.publicHost != "" {
		base := strings.TrimRight(h.publicHost, "/")
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			base = "https://" + base
		}
		return base
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	// Only accept the two values this header is allowed to carry.
	if fwdProto := c.GetHeader("X-Forwarded-Proto"); fwdProto == "http" || fwdProto == "https" {
		scheme = fwdProto
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// tokenResponse is the JSON shape returned by the calendar URL endpoints.
func (h *Handler) tokenResponse(c *gin.Context, token *calModel.CalendarToken) gin.H {
	return gin.H{
		"url":        h.calendarURL(c, token.Token),
		"createdAt":  token.CreatedAt,
		"lastUsedAt": token.LastUsedAt,
	}
}

// issueToken generates and persists a new token, replacing any existing one.
func (h *Handler) issueToken(c *gin.Context, userID int) (*calModel.CalendarToken, error) {
	token, err := generateCalendarToken()
	if err != nil {
		return nil, err
	}
	return h.calRepo.Replace(c.Request.Context(), userID, token)
}

// GetCalendarURL godoc
//
//	@Summary		Get calendar subscription URL
//	@Description	Returns the current user's existing iCal calendar subscription URL. Returns 404 if none has been generated yet.
//	@Tags			chores
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string]string	"url: calendar subscription URL"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only plus members can access this endpoint"
//	@Failure		404	{object}	map[string]string	"error: No calendar URL generated"
//	@Router			/chores/calendar/url [get]
func (h *Handler) GetCalendarURL(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	token, err := h.calRepo.GetByUserID(c.Request.Context(), currentUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "No calendar URL has been generated"})
		return
	}
	if err != nil {
		logger.Error("Failed to load calendar token", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load calendar URL"})
		return
	}

	c.JSON(http.StatusOK, h.tokenResponse(c, token))
}

// CreateCalendarURL godoc
//
//	@Summary		Generate calendar subscription URL
//	@Description	Generates a calendar subscription URL for the current user. Idempotent: returns the existing URL if one was already generated.
//	@Tags			chores
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string]string	"url: existing calendar subscription URL"
//	@Success		201	{object}	map[string]string	"url: newly generated calendar subscription URL"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only plus members can access this endpoint"
//	@Router			/chores/calendar/url [post]
func (h *Handler) CreateCalendarURL(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	existing, err := h.calRepo.GetByUserID(c.Request.Context(), currentUser.ID)
	if err == nil {
		c.JSON(http.StatusOK, h.tokenResponse(c, existing))
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("Failed to load calendar token", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate calendar URL"})
		return
	}

	token, err := h.issueToken(c, currentUser.ID)
	if err != nil {
		logger.Error("Failed to generate calendar token", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate calendar URL"})
		return
	}

	logger.Info("Generated calendar URL", "userID", currentUser.ID)
	c.JSON(http.StatusCreated, h.tokenResponse(c, token))
}

// RotateCalendarURL godoc
//
//	@Summary		Rotate calendar subscription URL
//	@Description	Issues a new calendar subscription URL and immediately invalidates the previous one.
//	@Tags			chores
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string]string	"url: new calendar subscription URL"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only plus members can access this endpoint"
//	@Router			/chores/calendar/url/rotate [post]
func (h *Handler) RotateCalendarURL(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	token, err := h.issueToken(c, currentUser.ID)
	if err != nil {
		logger.Error("Failed to rotate calendar token", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate calendar URL"})
		return
	}

	logger.Info("Rotated calendar URL", "userID", currentUser.ID)
	c.JSON(http.StatusOK, h.tokenResponse(c, token))
}

// RevokeCalendarURL godoc
//
//	@Summary		Revoke calendar subscription URL
//	@Description	Revokes the current user's calendar subscription URL. Existing subscriptions stop working immediately.
//	@Tags			chores
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		204	{string}	string	"No content"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only plus members can access this endpoint"
//	@Router			/chores/calendar/url [delete]
func (h *Handler) RevokeCalendarURL(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	if err := h.calRepo.DeleteByUserID(c.Request.Context(), currentUser.ID); err != nil {
		logger.Error("Failed to revoke calendar token", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke calendar URL"})
		return
	}

	logger.Info("Revoked calendar URL", "userID", currentUser.ID)
	c.Status(http.StatusNoContent)
}

// ServeCalendar godoc
//
//	@Summary		Serve iCal calendar feed
//	@Description	Serves an iCal (.ics) calendar file containing the user's chores. Authenticated via a revocable token in the URL.
//	@Tags			chores
//	@Produce		text/calendar
//	@Param			token	path		string	true	"Calendar token (obtained from /chores/calendar/url)"
//	@Success		200		{string}	string	"iCal calendar data"
//	@Failure		403		{string}	string	"Invalid calendar token, or the owner is not a plus member"
//	@Failure		500		{string}	string	"Failed to generate calendar"
//	@Router			/chores/calendar/{token} [get]
func (h *Handler) ServeCalendar(c *gin.Context) {
	logger := logging.FromContext(c)
	ctx := c.Request.Context()

	rawToken := strings.TrimSuffix(c.Param("token"), ".ics")
	if rawToken == "" {
		c.String(http.StatusForbidden, "Invalid calendar token")
		return
	}

	// Unauthenticated endpoint: failures here are routine (revoked tokens,
	// scanners) so they are logged below error level to avoid log flooding.
	token, err := h.calRepo.GetByToken(ctx, rawToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("Unknown or revoked calendar token")
		} else {
			logger.Error("Failed to look up calendar token", "error", err)
		}
		c.String(http.StatusForbidden, "Invalid calendar token")
		return
	}

	user, err := h.userRepo.GetUserByID(ctx, token.UserID)
	if err != nil || user.Disabled {
		logger.Warn("Calendar user not found or disabled", "userID", token.UserID, "error", err)
		c.String(http.StatusForbidden, "Invalid calendar token")
		return
	}

	// The plus check has to happen here too, not just on the management routes:
	// this endpoint authenticates by token, so a subscription that lapses after
	// the URL was issued would otherwise keep serving the feed forever.
	if !user.IsPlusMember() {
		logger.Warn("Calendar feed requested by non-plus member", "userID", token.UserID)
		c.String(http.StatusForbidden, "Calendar sync requires a plus subscription")
		return
	}

	chores, err := h.choreRepo.GetChores(ctx, user.CircleID, token.UserID, false, nil, false)
	if err != nil {
		logger.Error("Failed to retrieve chores for calendar", "error", err, "userID", token.UserID)
		c.String(http.StatusInternalServerError, "Failed to generate calendar")
		return
	}

	ical := buildICalFeed(chores, user.DisplayName, user.Timezone)

	// Best effort: a failed bookkeeping write must not fail the feed.
	if err := h.calRepo.TouchLastUsed(ctx, token.ID); err != nil {
		logger.Warn("Failed to update calendar token last used", "error", err, "tokenID", token.ID)
	}

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"donetick-chores.ics\"")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("X-Content-Type-Options", "nosniff")
	c.String(http.StatusOK, ical)
}

// buildICalFeed generates a full VCALENDAR string from a list of chores.
func buildICalFeed(chores []*chModel.Chore, calendarName string, userTimezone string) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString(foldLine(fmt.Sprintf("PRODID:%s", calendarProdID)))
	b.WriteString(foldLine(fmt.Sprintf("X-WR-CALNAME:Donetick - %s", escapeICalText(calendarName))))
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")

	if userTimezone != "" {
		b.WriteString(foldLine(fmt.Sprintf("X-WR-TIMEZONE:%s", userTimezone)))
	}

	now := time.Now().UTC()
	dtstamp := now.Format("20060102T150405Z")

	for _, ch := range chores {
		if ch.NextDueDate == nil {
			continue
		}

		b.WriteString("BEGIN:VEVENT\r\n")

		uid := fmt.Sprintf("chore-%d@donetick", ch.ID)
		b.WriteString(foldLine(fmt.Sprintf("UID:%s", uid)))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))

		dtstart := ch.NextDueDate.UTC().Format("20060102T150405Z")
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", dtstart))

		// Use completion window (hours) as event duration, default 1 hour
		duration := time.Hour
		if ch.CompletionWindow != nil && *ch.CompletionWindow > 0 {
			duration = time.Duration(*ch.CompletionWindow) * time.Hour
		}
		dtend := ch.NextDueDate.Add(duration).UTC().Format("20060102T150405Z")
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", dtend))

		b.WriteString(foldLine(fmt.Sprintf("SUMMARY:%s", escapeICalText(ch.Name))))

		desc := buildChoreDescription(ch)
		if desc != "" {
			b.WriteString(foldLine(fmt.Sprintf("DESCRIPTION:%s", escapeICalText(desc))))
		}

		icalPriority := mapPriority(ch.Priority)
		if icalPriority > 0 {
			b.WriteString(fmt.Sprintf("PRIORITY:%d\r\n", icalPriority))
		}

		switch ch.Status {
		case chModel.ChoreStatusInProgress:
			b.WriteString("STATUS:IN-PROCESS\r\n")
		default:
			b.WriteString("STATUS:NEEDS-ACTION\r\n")
		}

		b.WriteString(fmt.Sprintf("LAST-MODIFIED:%s\r\n", ch.UpdatedAt.UTC().Format("20060102T150405Z")))
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// buildChoreDescription creates a plain-text description for the calendar event.
func buildChoreDescription(ch *chModel.Chore) string {
	var parts []string

	if ch.Description != nil && *ch.Description != "" {
		parts = append(parts, stripHTMLTags(*ch.Description))
	}

	if ch.FrequencyType != "" && ch.FrequencyType != chModel.FrequencyTypeOnce {
		parts = append(parts, fmt.Sprintf("Repeats: %s", ch.FrequencyType))
	}

	if ch.Points != nil && *ch.Points > 0 {
		parts = append(parts, fmt.Sprintf("Points: %d", *ch.Points))
	}

	pName := priorityName(ch.Priority)
	if pName != "" {
		parts = append(parts, fmt.Sprintf("Priority: %s", pName))
	}

	// Real newlines: escapeICalText turns these into the RFC 5545 "\n" escape.
	// Pre-escaping here would get double-escaped into a literal backslash-n.
	return strings.Join(parts, "\n")
}

// escapeICalText escapes special characters per RFC 5545.
func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// foldLine implements RFC 5545 line folding (content lines must be <= 75
// octets). Folds land on UTF-8 boundaries: RFC 5545 forbids splitting a
// multi-octet character across a fold.
func foldLine(line string) string {
	const maxLen = 75
	const contLen = 74 // continuation lines start with a space

	line = strings.TrimRight(line, "\r\n")
	if len(line) <= maxLen {
		return line + "\r\n"
	}

	var b strings.Builder
	cut := foldBoundary(line, maxLen)
	b.WriteString(line[:cut])
	b.WriteString("\r\n")
	remaining := line[cut:]

	for len(remaining) > 0 {
		cut = foldBoundary(remaining, contLen)
		b.WriteByte(' ')
		b.WriteString(remaining[:cut])
		b.WriteString("\r\n")
		remaining = remaining[cut:]
	}

	return b.String()
}

// foldBoundary returns the largest cut index <= max that does not fall inside a
// UTF-8 sequence.
func foldBoundary(s string, max int) int {
	if len(s) <= max {
		return len(s)
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		// A single rune wider than the limit; cut at the limit rather than loop.
		return max
	}
	return cut
}

// mapPriority converts Donetick priority (0-4) to iCal priority (1-9).
func mapPriority(p int) int {
	switch p {
	case 4:
		return 1 // urgent -> highest
	case 3:
		return 3 // high
	case 2:
		return 5 // medium
	case 1:
		return 7 // low
	default:
		return 0 // undefined
	}
}

// priorityName returns a human-readable priority name.
func priorityName(p int) string {
	switch p {
	case 4:
		return "Urgent"
	case 3:
		return "High"
	case 2:
		return "Medium"
	case 1:
		return "Low"
	default:
		return ""
	}
}

// stripHTMLTags removes HTML tags from a string for plain-text output.
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Routes registers calendar-specific routes
func Routes(router *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware, limiter *limiter.Limiter) {
	// Authenticated endpoints to manage the user's personal calendar URL.
	// Calendar sync is a plus feature; self-hosted instances are always plus.
	calRoutes := router.Group("api/v1/chores/calendar")
	calRoutes.Use(
		multiAuthMiddleware.MiddlewareFunc(),
		utils.RateLimitMiddleware(limiter),
		auth.RequirePlusMemberMiddleware(),
	)
	{
		calRoutes.GET("/url", h.GetCalendarURL)
		calRoutes.POST("/url", h.CreateCalendarURL)
		calRoutes.POST("/url/rotate", h.RotateCalendarURL)
		calRoutes.DELETE("/url", h.RevokeCalendarURL)
	}

	// Public endpoint (token-authenticated) to serve the .ics file. Calendar
	// apps cannot send auth headers, so this uses token-in-URL auth; it is rate
	// limited because it is reachable without credentials.
	publicRoutes := router.Group("api/v1/chores/calendar")
	publicRoutes.Use(utils.RateLimitMiddleware(limiter))
	{
		publicRoutes.GET("/:token", h.ServeCalendar)
	}
}
