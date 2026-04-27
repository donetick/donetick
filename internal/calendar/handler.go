package calendar

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

const (
	calendarTokenPrefix = "dtcal"
	calendarProdID      = "-//Donetick//Chores Calendar//EN"
)

// Handler handles iCal calendar endpoints
type Handler struct {
	choreRepo *chRepo.ChoreRepository
	userRepo  *uRepo.UserRepository
	secret    string
}

// NewHandler creates a new calendar handler
func NewHandler(cr *chRepo.ChoreRepository, ur *uRepo.UserRepository, cfg *config.Config) *Handler {
	return &Handler{
		choreRepo: cr,
		userRepo:  ur,
		secret:    cfg.Jwt.Secret,
	}
}

// generateCalendarToken creates an HMAC-signed token for a user's calendar URL.
func (h *Handler) generateCalendarToken(userID int) string {
	payload := fmt.Sprintf("%s:%d", calendarTokenPrefix, userID)
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%d-%s", userID, sig)
}

// parseCalendarToken validates a calendar token and returns the user ID.
func (h *Handler) parseCalendarToken(token string) (int, error) {
	parts := strings.SplitN(token, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid token format")
	}

	userID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in token")
	}

	expectedToken := h.generateCalendarToken(userID)
	if !hmac.Equal([]byte(token), []byte(expectedToken)) {
		return 0, fmt.Errorf("invalid token signature")
	}

	return userID, nil
}

// GetCalendarURL godoc
//
//	@Summary		Get calendar subscription URL
//	@Description	Returns a personal iCal calendar subscription URL for the current user
//	@Tags			chores
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string]string	"url: calendar subscription URL"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Router			/chores/calendar/url [get]
func (h *Handler) GetCalendarURL(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	token := h.generateCalendarToken(currentUser.ID)

	scheme := "https"
	if c.Request.TLS == nil {
		if fwdProto := c.GetHeader("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	calendarURL := fmt.Sprintf("%s://%s/api/v1/chores/calendar/%s.ics", scheme, host, token)

	c.JSON(http.StatusOK, gin.H{
		"url": calendarURL,
	})
}

// ServeCalendar godoc
//
//	@Summary		Serve iCal calendar feed
//	@Description	Serves an iCal (.ics) calendar file containing the user's chores. Authenticated via HMAC token in URL.
//	@Tags			chores
//	@Produce		text/calendar
//	@Param			token	path		string	true	"Calendar token (obtained from /chores/calendar/url)"
//	@Success		200		{string}	string	"iCal calendar data (VTODO)"
//	@Failure		403		{string}	string	"Invalid calendar token"
//	@Failure		500		{string}	string	"Failed to generate calendar"
//	@Router			/chores/calendar/{token} [get]
func (h *Handler) ServeCalendar(c *gin.Context) {
	logger := logging.FromContext(c)

	rawToken := c.Param("token")
	rawToken = strings.TrimSuffix(rawToken, ".ics")

	userID, err := h.parseCalendarToken(rawToken)
	if err != nil {
		logger.Error("Invalid calendar token", "error", err)
		c.String(http.StatusForbidden, "Invalid calendar token")
		return
	}

	user, err := h.userRepo.GetUserByID(c.Request.Context(), userID)
	if err != nil || user.Disabled {
		logger.Error("Calendar user not found or disabled", "userID", userID, "error", err)
		c.String(http.StatusForbidden, "Invalid calendar token")
		return
	}

	chores, err := h.choreRepo.GetChores(c, user.CircleID, userID, false)
	if err != nil {
		logger.Error("Failed to retrieve chores for calendar", "error", err, "userID", userID)
		c.String(http.StatusInternalServerError, "Failed to generate calendar")
		return
	}

	ical := buildICalFeed(chores, user.DisplayName, user.Timezone)

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"donetick-chores.ics\"")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
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

		b.WriteString("BEGIN:VTODO\r\n")

		uid := fmt.Sprintf("chore-%d@donetick", ch.ID)
		b.WriteString(foldLine(fmt.Sprintf("UID:%s", uid)))
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))

		due := ch.NextDueDate.UTC().Format("20060102T150405Z")
		b.WriteString(fmt.Sprintf("DUE:%s\r\n", due))

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
		b.WriteString("END:VTODO\r\n")
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

	return strings.Join(parts, "\\n")
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

// foldLine implements RFC 5545 line folding (content lines must be <= 75 octets).
func foldLine(line string) string {
	const maxLen = 75
	line = strings.TrimRight(line, "\r\n")

	if len(line) <= maxLen {
		return line + "\r\n"
	}

	var b strings.Builder
	b.WriteString(line[:maxLen])
	b.WriteString("\r\n")
	remaining := line[maxLen:]

	for len(remaining) > 0 {
		chunkLen := 74 // continuation lines start with a space, leaving 74 for content
		if chunkLen > len(remaining) {
			chunkLen = len(remaining)
		}
		b.WriteByte(' ')
		b.WriteString(remaining[:chunkLen])
		b.WriteString("\r\n")
		remaining = remaining[chunkLen:]
	}

	return b.String()
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
func Routes(router *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	// Authenticated endpoint to get the user's personal calendar URL
	calRoutes := router.Group("api/v1/chores/calendar")
	calRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		calRoutes.GET("/url", h.GetCalendarURL)
	}

	// Public endpoint (token-authenticated via HMAC) to serve the .ics file
	// Calendar apps cannot send JWT headers, so this uses token-in-URL auth
	router.GET("api/v1/chores/calendar/:token", h.ServeCalendar)
}
