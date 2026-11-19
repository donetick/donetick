package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

const (
	choreIDHelp   = "Numeric id of the chore, as returned by list_chores."
	timestampHelp = "RFC 3339 timestamp, for example 2026-01-31T18:00:00Z or 2026-01-31T18:00:00+05:45."
)

var choreTools = []tool{
	{
		Name:        "list_chores",
		Title:       "List chores",
		Description: "List the chores (tasks) the user can see in their circle, with due dates, assignees, recurrence, labels and projects. People appear as numeric user ids.",
		InputSchema: inputs(schema{
			"includeArchived": property("boolean", "Also return archived chores. Defaults to false."),
			"includeSubtasks": property("boolean", "Also return each chore's subtasks. Defaults to false."),
		}),
		Annotations: readOnly,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				IncludeArchived bool `json:"includeArchived"`
				IncludeSubtasks bool `json:"includeSubtasks"`
			}
			if _, err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			query := url.Values{}
			if args.IncludeArchived {
				query.Set("includeArchived", "true")
			}
			if args.IncludeSubtasks {
				query.Set("includeSubtasks", "true")
			}
			return apiCall{method: http.MethodGet, path: "/api/v1/chores/", query: query}, nil
		}),
	},
	{
		Name:        "get_chore",
		Title:       "Get chore",
		Description: "Get one chore in full, including its subtasks, labels and project.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: readOnly,
		run:         byID(http.MethodGet, "/api/v1/chores/%d"),
	},
	{
		Name:        "create_chore",
		Title:       "Create chore",
		Description: "Create a chore. Unless told otherwise it is a one-time chore assigned to the owner of the API token. Returns the id of the new chore.",
		InputSchema: inputs(choreProperties, "name"),
		Annotations: additive,
		run:         createChore,
	},
	{
		Name:        "update_chore",
		Title:       "Update chore",
		Description: "Change a chore. Only the fields that are given change; the rest keep their current value. Only the chore's creator or a circle admin may update it, and a chore triggered by a thing only the owner of that thing.",
		InputSchema: inputs(choreProperties.with(schema{"id": property("integer", choreIDHelp)}), "id"),
		Annotations: overwriting,
		run:         updateChore,
	},
	{
		Name:        "complete_chore",
		Title:       "Complete chore",
		Description: "Mark a chore as done. A recurring chore is rescheduled to its next due date and handed to its next assignee. Returns the updated chore.",
		InputSchema: inputs(schema{
			"id":            property("integer", choreIDHelp),
			"notes":         property("string", "A note to record with the completion."),
			"completedBy":   property("integer", "User id of the circle member who did the chore. Only circle admins and managers may complete a chore for somebody else. Defaults to the owner of the API token."),
			"completedTime": property("string", "When the chore was done, if not now. "+timestampHelp),
		}, "id"),
		Annotations: oneWay,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				ID            *int    `json:"id,omitempty"`
				Notes         *string `json:"notes,omitempty"`
				CompletedBy   *int    `json:"completedBy,omitempty"`
				CompletedTime *string `json:"completedTime,omitempty"`
			}
			if _, err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			if args.Notes != nil && *args.Notes == "" {
				args.Notes = nil
			}
			if args.CompletedTime != nil {
				if err := checkTimestamp("completedTime", *args.CompletedTime); err != nil {
					return apiCall{}, err
				}
			}
			path := fmt.Sprintf("/api/v1/chores/%d/do", *args.ID)
			args.ID = nil
			return apiCall{method: http.MethodPost, path: path, body: args}, nil
		}),
	},
	{
		Name:        "skip_chore",
		Title:       "Skip chore",
		Description: "Skip the current occurrence of a chore without completing it. A recurring chore moves on to its next due date. Returns the updated chore.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: oneWay,
		run:         byID(http.MethodPost, "/api/v1/chores/%d/skip"),
	},
	{
		Name:        "archive_chore",
		Title:       "Archive chore",
		Description: "Archive a chore: it is kept, with its history, but no longer listed as active.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodPut, "/api/v1/chores/%d/archive"),
	},
	{
		Name:        "unarchive_chore",
		Title:       "Unarchive chore",
		Description: "Make an archived chore active again.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodPut, "/api/v1/chores/%d/unarchive"),
	},
	{
		Name:        "delete_chore",
		Title:       "Delete chore",
		Description: "Permanently delete a chore and its history. Only the chore's creator or a circle admin or manager may delete it. Use archive_chore to keep the history.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodDelete, "/api/v1/chores/%d"),
	},
	{
		Name:        "get_chore_history",
		Title:       "Get chore history",
		Description: "List when one chore was completed, skipped or rescheduled, and by whom.",
		InputSchema: inputs(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: readOnly,
		run:         byID(http.MethodGet, "/api/v1/chores/%d/history"),
	},
	{
		Name:        "list_recent_activity",
		Title:       "List recent activity",
		Description: "List what was completed, skipped or rescheduled across all chores in the last few days.",
		InputSchema: inputs(schema{
			"days":           property("integer", "How many days to look back. Defaults to 7."),
			"includeMembers": property("boolean", "Include the activity of other circle members, not only the user's own. Defaults to false."),
		}),
		Annotations: readOnly,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				Days           *int `json:"days"`
				IncludeMembers bool `json:"includeMembers"`
			}
			if _, err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			query := url.Values{}
			if args.Days != nil {
				if *args.Days < 1 {
					return apiCall{}, errors.New("days must be at least 1")
				}
				query.Set("limit", strconv.Itoa(*args.Days))
			}
			if args.IncludeMembers {
				query.Set("members", "true")
			}
			return apiCall{method: http.MethodGet, path: "/api/v1/chores/history", query: query}, nil
		}),
	},
}

// choreProperties are the fields create_chore and update_chore accept. Apart
// from dueDate, assignees and labels they carry the names the API uses, so a
// model can copy them from what list_chores and get_chore return.
var choreProperties = schema{
	"name":        property("string", "Short name of the chore."),
	"description": nullable("string", "Longer description or notes."),
	"dueDate":     property("string", "When the chore is next due. "+timestampHelp),
	"frequencyType": oneOf("How the chore repeats: once (it does not), daily, weekly, monthly, yearly, "+
		"interval (every `frequency` units of frequencyMetadata.unit), "+
		"days_of_the_week (on the weekdays in frequencyMetadata.days) or "+
		"day_of_the_month (on day `frequency` of the months in frequencyMetadata.months). Defaults to once.",
		"once", "daily", "weekly", "monthly", "yearly", "interval", "days_of_the_week", "day_of_the_month"),
	"frequency": property("integer", "For interval: how many units between occurrences. For day_of_the_month: the day, 1 to 31."),
	"frequencyMetadata": schema{
		"type":                 []string{"object", "null"},
		"additionalProperties": false,
		"description":          "Details of the recurrence, for the frequency types that need them. On an update, the details that are left out keep their value.",
		"properties": schema{
			"unit":        oneOf("For interval: the unit `frequency` counts.", "hours", "days", "weeks", "months", "years"),
			"days":        listOf("string", "For days_of_the_week: lower-case weekday names, for example [\"monday\", \"thursday\"]."),
			"months":      listOf("string", "For day_of_the_month: lower-case month names, for example [\"january\", \"july\"]."),
			"weekPattern": oneOf("For days_of_the_week: every_week (the default), or only certain weeks of the month or quarter, chosen with occurrences.", "every_week", "week_of_month", "week_of_quarter"),
			"occurrences": listOf("integer", "For week_of_month and week_of_quarter: which occurrences of the weekday count, for example [1, 3]; -1 is the last one."),
			"time":        property("string", "Time of day the chore is due, taken from this timestamp. Follows dueDate unless given. "+timestampHelp),
			"timezone":    property("string", "IANA time zone the schedule follows, for example Europe/Berlin. Defaults to UTC."),
		},
	},
	"isRolling":       property("boolean", "Count the next occurrence from when the chore was completed instead of from when it was due."),
	"assignees":       listOf("integer", "User ids of the circle members who take turns doing the chore. An empty list means anyone in the circle."),
	"assignedTo":      nullable("integer", "User id of the member who does the chore next. Must be one of assignees. On a new chore it defaults to the owner of the API token if they are an assignee, and to the first assignee otherwise."),
	"assignStrategy":  oneOf("How the next assignee is chosen when a recurring chore is completed. Defaults to random.", "random", "round_robin", "least_assigned", "least_completed", "keep_last_assigned", "random_except_last_assigned", "no_assignee"),
	"priority":        property("integer", "0 for none, or 1 (most urgent) to 4 (least urgent)."),
	"points":          nullable("integer", "Points awarded for completing the chore."),
	"labels":          listOf("integer", "Ids of the labels on the chore, as returned by list_labels. Replaces the current labels."),
	"projectId":       nullable("integer", "Id of the project the chore belongs to, as returned by list_projects."),
	"isPrivate":       property("boolean", "Show the chore only to its creator and assignees. A chore in a project is visible to whoever can see the project instead."),
	"requireApproval": property("boolean", "Completions must be approved by a circle admin or manager before they count."),
}

// choreArguments are the arguments of create_chore and update_chore. They are
// decoded for validation; what is sent on is the arguments as given.
type choreArguments struct {
	ID                *int               `json:"id"`
	Name              *string            `json:"name"`
	Description       *string            `json:"description"`
	DueDate           *string            `json:"dueDate"`
	FrequencyType     *string            `json:"frequencyType"`
	Frequency         *int               `json:"frequency"`
	FrequencyMetadata *frequencyMetadata `json:"frequencyMetadata"`
	IsRolling         *bool              `json:"isRolling"`
	Assignees         *[]int             `json:"assignees"`
	AssignedTo        *int               `json:"assignedTo"`
	AssignStrategy    *string            `json:"assignStrategy"`
	Priority          *int               `json:"priority"`
	Points            *int               `json:"points"`
	Labels            *[]int             `json:"labels"`
	ProjectID         *int               `json:"projectId"`
	IsPrivate         *bool              `json:"isPrivate"`
	RequireApproval   *bool              `json:"requireApproval"`
}

type frequencyMetadata struct {
	Unit        *string  `json:"unit,omitempty"`
	Days        []string `json:"days,omitempty"`
	Months      []string `json:"months,omitempty"`
	WeekPattern *string  `json:"weekPattern,omitempty"`
	Occurrences []int    `json:"occurrences,omitempty"`
	Time        string   `json:"time,omitempty"`
	Timezone    string   `json:"timezone,omitempty"`
}

// check looks at the arguments on their own, before anything is asked of the
// API. Only the arguments the schema marks as nullable may be null, which
// clears the field.
func (a *choreArguments) check(given object) error {
	err := notNull(given, "name", "dueDate", "frequencyType", "frequency", "isRolling", "assignees",
		"assignStrategy", "priority", "labels", "isPrivate", "requireApproval")
	if err == nil && a.DueDate != nil {
		err = checkTimestamp("dueDate", *a.DueDate)
	}
	return err
}

// apply writes the arguments that were given into a chore request.
func (a *choreArguments) apply(chore, given object) {
	overlay(chore, given,
		"name", "description", "frequencyType", "frequency", "isRolling",
		"assignedTo", "assignStrategy", "priority", "points", "projectId", "isPrivate", "requireApproval")

	// The details of a recurrence are changed one by one, like the chore
	// itself: naming new weekdays must not reset the time zone.
	if details, ok := given["frequencyMetadata"]; ok {
		merged := object{}
		if a.FrequencyMetadata != nil {
			_ = json.Unmarshal(chore["frequencyMetadata"], &merged)
			var changed object
			_ = json.Unmarshal(details, &changed)
			maps.Copy(merged, changed)
			details = raw(merged)
		}
		chore["frequencyMetadata"] = details
	}
	if a.DueDate != nil {
		chore["nextDueDate"] = raw(*a.DueDate)
	}
	if a.Assignees != nil {
		assignees := make([]map[string]int, 0, len(*a.Assignees))
		for _, userID := range *a.Assignees {
			assignees = append(assignees, map[string]int{"userId": userID})
		}
		chore["assignees"] = raw(assignees)
	}
	if a.Labels != nil {
		labels := make([]map[string]int, 0, len(*a.Labels))
		for _, labelID := range *a.Labels {
			labels = append(labels, map[string]int{"id": labelID})
		}
		chore["labelsV2"] = raw(labels)
	}
}

func checkTimestamp(name, value string) error {
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return fmt.Errorf("%s must be an RFC 3339 timestamp such as 2026-01-31T18:00:00Z", name)
	}
	return nil
}

func createChore(s *session, arguments json.RawMessage) (apiResponse, error) {
	var args choreArguments
	given, err := decodeArguments(arguments, &args)
	if err == nil {
		err = args.check(given)
	}
	if err != nil {
		return apiResponse{}, err
	}
	if args.Name == nil || *args.Name == "" {
		return apiResponse{}, errors.New("name is required")
	}
	if args.ID != nil {
		return apiResponse{}, errors.New("id cannot be chosen; use update_chore to change an existing chore")
	}

	chore := object{"frequencyType": raw("once"), "assignStrategy": raw("random")}
	args.apply(chore, given)
	if _, chosen := given["assignedTo"]; !chosen && string(chore["assignStrategy"]) != `"no_assignee"` {
		switch {
		case args.Assignees == nil:
			// A chore nobody was named for goes to the person asking, which
			// is what "add a task" nearly always means.
			chore["assignees"] = raw([]map[string]int{{"userId": s.userID}})
			chore["assignedTo"] = raw(s.userID)
		case slices.Contains(*args.Assignees, s.userID):
			chore["assignedTo"] = raw(s.userID)
		case len(*args.Assignees) > 0:
			// The API leaves a chore with assignees unassigned, and an
			// unassigned chore reminds nobody.
			chore["assignedTo"] = raw((*args.Assignees)[0])
		}
	}
	if err := checkRecurrence(chore); err != nil {
		return apiResponse{}, err
	}
	args.followDueDate(chore)
	return s.call(apiCall{method: http.MethodPost, path: "/api/v1/chores/", body: chore})
}

func updateChore(s *session, arguments json.RawMessage) (apiResponse, error) {
	var args choreArguments
	given, err := decodeArguments(arguments, &args)
	if err == nil {
		err = args.check(given)
	}
	if err != nil {
		return apiResponse{}, err
	}
	if args.ID == nil {
		return apiResponse{}, errors.New("id is required")
	}
	if len(given) < 2 {
		return apiResponse{}, errors.New("nothing to update: give at least one field to change")
	}

	// The API replaces a chore whole, so start from the chore as it is. Its
	// updatedAt goes back with it, which makes the API turn the update down
	// if somebody else changed the chore in the meantime.
	chore, err := s.fetch(fmt.Sprintf("/api/v1/chores/%d", *args.ID))
	if err != nil {
		return apiResponse{}, err
	}
	// These three are left alone by the API when they are not sent.
	delete(chore, "labelsV2")
	delete(chore, "subTasks")
	delete(chore, "completionActions")
	// The link to a thing is read under one name and written under another;
	// without it the API would unlink the thing.
	var link struct {
		ThingID      int    `json:"thingId"`
		TriggerState string `json:"triggerState"`
		Condition    string `json:"condition"`
	}
	if json.Unmarshal(chore["thingChore"], &link) == nil && link.ThingID != 0 {
		// The API only links a chore to a thing of the caller's own, and it
		// finds out after it has saved the chore and removed the old link.
		// So an update by anyone else has to stop here.
		var missing *notFound
		if _, err := s.find("/api/v1/things", "thing", link.ThingID); errors.As(err, &missing) {
			return apiResponse{}, errors.New("this chore is triggered by a thing that belongs to another user, and only that user can update it without losing the link")
		} else if err != nil {
			return apiResponse{}, err
		}
		chore["thingTrigger"] = raw(map[string]any{
			"thingID": link.ThingID, "triggerState": link.TriggerState, "condition": link.Condition,
		})
	}

	args.apply(chore, given)
	if args.FrequencyType != nil || args.Frequency != nil || args.FrequencyMetadata != nil {
		if err := checkRecurrence(chore); err != nil {
			return apiResponse{}, err
		}
	}
	args.followDueDate(chore)
	return s.call(apiCall{method: http.MethodPut, path: "/api/v1/chores/", body: chore})
}

// checkRecurrence turns down recurrences the scheduler cannot work with, such
// as an interval without a unit. The API stores them as they are and fails
// later, when the chore is completed.
func checkRecurrence(chore object) error {
	var recurrence struct {
		Type     string             `json:"frequencyType"`
		Count    *int               `json:"frequency"`
		Metadata *frequencyMetadata `json:"frequencyMetadata"`
	}
	if err := json.Unmarshal(raw(chore), &recurrence); err != nil {
		return &internalError{fmt.Errorf("unexpected chore: %w", err)}
	}
	metadata := recurrence.Metadata
	if metadata == nil {
		metadata = &frequencyMetadata{}
	}

	switch recurrence.Type {
	case "interval":
		if recurrence.Count == nil || *recurrence.Count < 1 || metadata.Unit == nil {
			return errors.New(`frequencyType "interval" needs frequency (how many, at least 1) and frequencyMetadata.unit (hours, days, weeks, months or years)`)
		}
	case "days_of_the_week":
		if len(metadata.Days) == 0 {
			return errors.New(`frequencyType "days_of_the_week" needs frequencyMetadata.days, for example ["monday", "thursday"]`)
		}
		if pattern := metadata.WeekPattern; pattern != nil && *pattern != "every_week" && len(metadata.Occurrences) == 0 {
			return fmt.Errorf("frequencyMetadata.weekPattern %q needs frequencyMetadata.occurrences, for example [1, 3]", *pattern)
		}
	case "day_of_the_month":
		if recurrence.Count == nil || *recurrence.Count < 1 || *recurrence.Count > 31 || len(metadata.Months) == 0 {
			return errors.New(`frequencyType "day_of_the_month" needs frequency (the day, 1 to 31) and frequencyMetadata.months, for example ["january", "july"]`)
		}
	}
	return nil
}

// followDueDate keeps the time of day in step with the due date. Chores that
// repeat at an interval, on weekdays or on a day of the month are scheduled at
// the time in their recurrence details, not at the time of the due date, so
// the web app sets both together; so does this, unless a time was given.
func (a *choreArguments) followDueDate(chore object) {
	var recurrence struct {
		Type     string            `json:"frequencyType"`
		Metadata frequencyMetadata `json:"frequencyMetadata"`
		DueDate  *string           `json:"nextDueDate"`
	}
	if json.Unmarshal(raw(chore), &recurrence) != nil || recurrence.DueDate == nil ||
		!slices.Contains([]string{"interval", "days_of_the_week", "day_of_the_month"}, recurrence.Type) {
		return
	}
	scheduleGiven := a.FrequencyType != nil || a.Frequency != nil || a.FrequencyMetadata != nil
	timeGiven := a.FrequencyMetadata != nil && a.FrequencyMetadata.Time != ""
	if timeGiven || (a.DueDate == nil && (!scheduleGiven || recurrence.Metadata.Time != "")) {
		return
	}
	details := object{}
	_ = json.Unmarshal(chore["frequencyMetadata"], &details)
	details["time"] = raw(*recurrence.DueDate)
	chore["frequencyMetadata"] = raw(details)
}
