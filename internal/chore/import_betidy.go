package chore

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	auth "donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	lModel "donetick.com/core/internal/label/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

// BeTidy import.
//
// Accepts a data bundle exported from the BeTidy Android app (io.betidy.BeTidy)
// via the community tool https://github.com/mschabhuettl/betidy-export and creates
// chores for the current user's circle. BeTidy's interval/date recurrence model is
// mapped onto Donetick's frequency model, BeTidy rooms are matched to labels (created
// on demand) and BeTidy profiles are matched to circle members by name. Stored due
// dates (often in the past) are rolled forward to the next occurrence.

const betidyDueHour = 8

type betidyImportRequest struct {
	Timezone        string       `json:"timezone"` // IANA timezone for due dates; default UTC
	IncludeInactive bool         `json:"includeInactive"`
	User            betidyUser   `json:"user"`
	Tasks           []betidyTask `json:"tasks"`
}

type betidyUser struct {
	Rooms    string `json:"rooms"`    // JSON-encoded array of {id,name}
	Profiles string `json:"profiles"` // JSON-encoded array of {id,name}
}

type betidyTask struct {
	Title         string   `json:"title"`
	Active        bool     `json:"active"`
	Type          string   `json:"type"` // "INTERVAL" (recurring) or "DATE" (one-time)
	RoomID        string   `json:"roomId"`
	IntervalUnit  string   `json:"intervalUnit"` // day | week | month
	IntervalCount int      `json:"intervalCount"`
	TodoDate      string   `json:"todoDate"`
	LastTodoDate  string   `json:"lastTodoDate"`
	Days          []int    `json:"days"` // weekly weekdays, 1=Mon .. 7=Sun
	Important     int      `json:"important"`
	Effort        int      `json:"effort"`
	Description   string   `json:"description"`
	Assigned      []string `json:"assigned"` // BeTidy profile ids
}

type betidyNamed struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ImportBeTidy godoc
//
//	@Summary		Import chores from a BeTidy export bundle
//	@Description	Creates chores for the current circle from a BeTidy export (see betidy-export).
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/chores/import/betidy [post]
func (h *Handler) ImportBeTidy(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	var req betidyImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	loc, err := time.LoadLocation(req.Timezone)
	if err != nil || req.Timezone == "" {
		loc = time.UTC
	}
	tz := loc.String()

	// BeTidy stores rooms and profiles as JSON-encoded strings inside the user record.
	roomName := map[string]string{}
	var rooms []betidyNamed
	if json.Unmarshal([]byte(betidyEmptyToArray(req.User.Rooms)), &rooms) == nil {
		for _, r := range rooms {
			roomName[r.ID] = r.Name
		}
	}
	profileName := map[string]string{}
	var profiles []betidyNamed
	if json.Unmarshal([]byte(betidyEmptyToArray(req.User.Profiles)), &profiles) == nil {
		for _, p := range profiles {
			profileName[p.ID] = p.Name
		}
	}

	// Circle members -> user id, matched by (lower-cased) display name / username / first name.
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get circle members"})
		return
	}
	memberID := map[string]int{}
	addMember := func(name string, uid int) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return
		}
		if _, exists := memberID[name]; !exists {
			memberID[name] = uid
		}
		if fields := strings.Fields(name); len(fields) > 0 {
			if _, exists := memberID[fields[0]]; !exists {
				memberID[fields[0]] = uid
			}
		}
	}
	for _, u := range circleUsers {
		addMember(u.DisplayName, u.UserID)
		addMember(u.Username, u.UserID)
	}

	// Existing labels by name; room labels are created on demand.
	existingLabels, err := h.lRepo.GetUserLabels(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get labels"})
		return
	}
	labelID := map[string]int{}
	for _, l := range existingLabels {
		labelID[l.Name] = l.ID
	}

	today := time.Now().UTC()
	imported, skipped, labelsCreated := 0, 0, 0
	importErrors := []string{}

	for _, t := range req.Tasks {
		if strings.TrimSpace(t.Title) == "" || (!t.Active && !req.IncludeInactive) {
			skipped++
			continue
		}

		ftype, freq, meta := betidyFrequency(t, tz, loc)
		due := betidyNextDue(t, today, loc)

		room := roomName[t.RoomID]
		if room == "" {
			room = t.RoomID
		}

		var assignees []chModel.ChoreAssignees
		seen := map[int]bool{}
		for _, pid := range t.Assigned {
			name := strings.ToLower(strings.TrimSpace(profileName[pid]))
			if uid, found := memberID[name]; found && !seen[uid] {
				assignees = append(assignees, chModel.ChoreAssignees{UserID: uid})
				seen[uid] = true
			}
		}
		if len(assignees) == 0 {
			assignees = []chModel.ChoreAssignees{{UserID: currentUser.ID}}
		}
		assignedTo := assignees[0].UserID

		description := betidyDescription(t, room, profileName)
		var points *int
		if t.Effort > 0 {
			e := t.Effort
			points = &e
		}

		chore := &chModel.Chore{
			Name:                t.Title,
			FrequencyType:       ftype,
			Frequency:           freq,
			FrequencyMetadataV2: meta,
			NextDueDate:         due,
			AssignStrategy:      chModel.AssignmentStrategyRandom,
			AssignedTo:          &assignedTo,
			Assignees:           assignees,
			IsActive:            true,
			Priority:            betidyPriority(t.Important),
			Points:              points,
			Description:         &description,
			CircleID:            currentUser.CircleID,
			CreatedBy:           currentUser.ID,
			CreatedAt:           time.Now().UTC(),
		}

		id, err := h.choreRepo.CreateChore(c, chore)
		if err != nil {
			importErrors = append(importErrors, t.Title+": "+err.Error())
			continue
		}

		if room != "" {
			lid, found := labelID[room]
			if !found {
				circleID := currentUser.CircleID
				newLabel := &lModel.Label{Name: room, Color: "#4b7bec", CreatedBy: currentUser.ID, CircleID: &circleID}
				if err := h.lRepo.CreateLabels(c, []*lModel.Label{newLabel}); err == nil {
					lid = newLabel.ID
					labelID[room] = lid
					labelsCreated++
				}
			}
			if lid > 0 {
				if err := h.lRepo.AssignLabelsToChore(c, id, currentUser.ID, currentUser.CircleID, []int{lid}, nil); err != nil {
					log.Debugw("betidy import: could not assign room label", "chore", id, "room", room, "error", err)
				}
			}
		}
		imported++
	}

	log.Infow("betidy import complete", "imported", imported, "skipped", skipped, "labelsCreated", labelsCreated)
	c.JSON(http.StatusOK, gin.H{"res": gin.H{
		"imported":      imported,
		"skipped":       skipped,
		"labelsCreated": labelsCreated,
		"errors":        importErrors,
	}})
}

func betidyEmptyToArray(s string) string {
	if strings.TrimSpace(s) == "" {
		return "[]"
	}
	return s
}

// betidyPriority maps BeTidy `important` (0 none, 1 important, 2 very) to a Donetick
// priority where P1 is highest.
func betidyPriority(important int) int {
	switch important {
	case 2:
		return 1
	case 1:
		return 2
	default:
		return 0
	}
}

func betidyUnit(u string) string {
	switch u {
	case "day":
		return "days"
	case "week":
		return "weeks"
	case "month":
		return "months"
	default:
		return "days"
	}
}

func betidyFrequency(t betidyTask, tz string, loc *time.Location) (chModel.FrequencyType, int, *chModel.FrequencyMetadata) {
	refTime := time.Date(2025, 1, 1, betidyDueHour, 0, 0, 0, loc).Format(time.RFC3339)
	if t.Type == "DATE" {
		return chModel.FrequencyTypeOnce, 1, &chModel.FrequencyMetadata{Time: refTime, Timezone: tz}
	}
	unit := betidyUnit(t.IntervalUnit)
	count := t.IntervalCount
	if count < 1 {
		count = 1
	}
	return chModel.FrequencyTypeInterval, count, &chModel.FrequencyMetadata{Unit: &unit, Time: refTime, Timezone: tz}
}

func betidyParseDate(s string) (time.Time, bool) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if d, err := time.Parse(layout, s); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

// betidyNextDue rolls BeTidy's stored date forward by the interval to the next
// occurrence >= today, snapping weekly tasks to their intended weekday.
func betidyNextDue(t betidyTask, today time.Time, loc *time.Location) *time.Time {
	anchor, ok := betidyParseDate(t.TodoDate)
	if !ok {
		anchor, ok = betidyParseDate(t.LastTodoDate)
	}
	if !ok {
		anchor = today
	}
	d := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), betidyDueHour, 0, 0, 0, loc)
	todayD := time.Date(today.Year(), today.Month(), today.Day(), betidyDueHour, 0, 0, 0, loc)

	if t.Type == "DATE" {
		if d.Before(todayD) {
			d = todayD
		}
		u := d.UTC()
		return &u
	}

	count := t.IntervalCount
	if count < 1 {
		count = 1
	}
	for i := 0; d.Before(todayD) && i < 2000; i++ {
		switch t.IntervalUnit {
		case "day":
			d = d.AddDate(0, 0, count)
		case "week":
			d = d.AddDate(0, 0, 7*count)
		case "month":
			d = d.AddDate(0, count, 0)
		default:
			i = 2000
		}
	}
	if t.IntervalUnit == "week" && len(t.Days) > 0 {
		targets := map[time.Weekday]bool{}
		for _, x := range t.Days {
			targets[time.Weekday(x%7)] = true // BeTidy 1=Mon..7=Sun -> Go Sun=0..Sat=6
		}
		for i := 0; i < 7 && !targets[d.Weekday()]; i++ {
			d = d.AddDate(0, 0, 1)
		}
	}
	u := d.UTC()
	return &u
}

func betidyRepeatText(t betidyTask) string {
	if t.Type == "DATE" {
		return "once"
	}
	count := t.IntervalCount
	if count < 1 {
		count = 1
	}
	if count == 1 {
		return "every " + t.IntervalUnit
	}
	return "every " + strconv.Itoa(count) + " " + t.IntervalUnit + "s"
}

func betidyDescription(t betidyTask, room string, profileName map[string]string) string {
	var b strings.Builder
	if strings.TrimSpace(t.Description) != "" {
		b.WriteString(strings.TrimSpace(t.Description))
		b.WriteString("\n")
	}
	b.WriteString("[BeTidy] Room: ")
	b.WriteString(room)
	b.WriteString(" · Repeats: ")
	b.WriteString(betidyRepeatText(t))

	names := []string{}
	for _, pid := range t.Assigned {
		if n := profileName[pid]; n != "" {
			names = append(names, n)
		}
	}
	if len(names) > 0 {
		b.WriteString(" · Assignee: ")
		b.WriteString(strings.Join(names, ", "))
	}
	if t.Effort > 0 {
		b.WriteString(" · Effort: ")
		b.WriteString(strconv.Itoa(t.Effort))
	}
	return b.String()
}
