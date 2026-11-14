package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	fModel "donetick.com/core/internal/filter/model"
	lModel "donetick.com/core/internal/label/model"
	pModel "donetick.com/core/internal/project/model"
	tModel "donetick.com/core/internal/thing/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// toolRoutes lists, for every tool, a valid call and the API requests it
// makes. A tool that is not listed here fails TestToolsUseOnlyTheirOwnRoutes,
// so adding one means stating which routes it may reach.
var toolRoutes = map[string]struct {
	arguments string
	requests  []string
}{
	"list_chores":          {`{}`, []string{"GET /api/v1/chores/"}},
	"get_chore":            {`{"id":7}`, []string{"GET /api/v1/chores/7"}},
	"create_chore":         {`{"name":"Dust"}`, []string{"POST /api/v1/chores/"}},
	"update_chore":         {`{"id":7,"name":"Dust"}`, []string{"GET /api/v1/chores/7", "PUT /api/v1/chores/"}},
	"complete_chore":       {`{"id":7}`, []string{"POST /api/v1/chores/7/do"}},
	"skip_chore":           {`{"id":7}`, []string{"POST /api/v1/chores/7/skip"}},
	"archive_chore":        {`{"id":7}`, []string{"PUT /api/v1/chores/7/archive"}},
	"unarchive_chore":      {`{"id":7}`, []string{"PUT /api/v1/chores/7/unarchive"}},
	"delete_chore":         {`{"id":7}`, []string{"DELETE /api/v1/chores/7"}},
	"get_chore_history":    {`{"id":7}`, []string{"GET /api/v1/chores/7/history"}},
	"list_recent_activity": {`{}`, []string{"GET /api/v1/chores/history"}},
	"list_labels":          {`{}`, []string{"GET /api/v1/labels"}},
	"create_label":         {`{"name":"Kitchen"}`, []string{"POST /api/v1/labels"}},
	"update_label":         {`{"id":7,"name":"Kitchen"}`, []string{"GET /api/v1/labels", "PUT /api/v1/labels"}},
	"delete_label":         {`{"id":7}`, []string{"DELETE /api/v1/labels/7"}},
	"list_projects":        {`{}`, []string{"GET /api/v1/projects"}},
	"create_project":       {`{"name":"Garden"}`, []string{"POST /api/v1/projects"}},
	"update_project":       {`{"id":7,"name":"Garden"}`, []string{"GET /api/v1/projects", "PUT /api/v1/projects/7"}},
	"delete_project":       {`{"id":7}`, []string{"DELETE /api/v1/projects/7"}},
	"list_filters":         {`{}`, []string{"GET /api/v1/filters"}},
	"create_filter":        {`{"name":"Late","conditions":[{"type":"dueDate","operator":"isOverdue"}]}`, []string{"POST /api/v1/filters"}},
	"update_filter":        {`{"id":7,"name":"Late"}`, []string{"GET /api/v1/filters/7", "PUT /api/v1/filters/7"}},
	"delete_filter":        {`{"id":7}`, []string{"DELETE /api/v1/filters/7"}},
	"list_things":          {`{}`, []string{"GET /api/v1/things"}},
	"create_thing":         {`{"name":"Loads","type":"number","state":0}`, []string{"POST /api/v1/things"}},
	"update_thing":         {`{"id":7,"name":"Loads"}`, []string{"GET /api/v1/things", "PUT /api/v1/things"}},
	"set_thing_state":      {`{"id":7,"state":3}`, []string{"PUT /api/v1/things/7/state"}},
	"delete_thing":         {`{"id":7}`, []string{"DELETE /api/v1/things/7"}},
}

// recorder is a session that answers every request itself and remembers what
// was asked.
func recorder(requests *[]string) *session {
	return &session{userID: alex, call: func(call apiCall) (apiResponse, error) {
		*requests = append(*requests, call.method+" "+call.path)
		if regexp.MustCompile(`/\d+$`).MatchString(call.path) {
			return apiResponse{status: http.StatusOK, body: []byte(`{"id":7,"name":"n"}`)}, nil
		}
		return apiResponse{status: http.StatusOK, body: []byte(`[{"id":7,"name":"n"}]`)}, nil
	}}
}

// The maintainers want chores, labels, projects, filters and things within
// reach of a model, and nothing that manages users, circles or tokens.
func TestToolsUseOnlyTheirOwnRoutes(t *testing.T) {
	allowed := regexp.MustCompile(`^(GET|POST|PUT|DELETE) /api/v1/(chores|labels|projects|filters|things)(/|$)`)
	require.Len(t, toolRoutes, len(tools), "every tool must be listed in toolRoutes")

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			routes, listed := toolRoutes[tool.Name]
			require.True(t, listed, "tool is missing from toolRoutes")

			var requests []string
			_, err := tool.run(recorder(&requests), json.RawMessage(routes.arguments))
			require.NoError(t, err)
			require.Equal(t, routes.requests, requests)
			for _, request := range requests {
				require.Regexp(t, allowed, request)
			}
		})
	}
}

func TestToolDefinitions(t *testing.T) {
	validName := regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	seen := map[string]bool{}
	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			require.Regexp(t, validName, tool.Name)
			require.False(t, seen[tool.Name], "duplicate tool name")
			seen[tool.Name] = true
			require.NotEmpty(t, tool.Title)
			require.NotEmpty(t, tool.Description)
			_, err := json.Marshal(tool)
			require.NoError(t, err)

			require.Equal(t, "object", tool.InputSchema["type"])
			properties, _ := tool.InputSchema["properties"].(schema)
			required, _ := tool.InputSchema["required"].([]string)
			for _, name := range required {
				require.Contains(t, properties, name, "required argument is not declared")
			}
			for name, property := range properties {
				require.NotEmpty(t, property.(schema)["description"], "argument %s has no description", name)
			}

			// Calls with a missing or made-up argument are turned down before
			// anything is sent to the API.
			var requests []string
			if len(required) > 0 {
				_, err := tool.run(recorder(&requests), json.RawMessage(`{}`))
				require.Error(t, err)
			}
			_, err = tool.run(recorder(&requests), json.RawMessage(`{"noSuchArgument":true}`))
			require.ErrorContains(t, err, "noSuchArgument")
			require.Empty(t, requests)

			// Every argument in the schema is one the tool accepts.
			for name := range properties {
				_, err := tool.run(recorder(&requests), json.RawMessage(fmt.Sprintf(`{%q:null}`, name)))
				if err != nil {
					require.NotContains(t, err.Error(), "unknown field", "argument %s is in the schema but not accepted", name)
				}
			}
		})
	}
}

func TestToolCallErrors(t *testing.T) {
	engine, _ := newServer(t)

	// Mistakes in the arguments come back as a result the model can read and
	// act on, not as a protocol error.
	for _, test := range []struct {
		name      string
		tool      string
		arguments map[string]any
		want      string
	}{
		{"required argument missing", "delete_chore", nil, "id is required"},
		{"blank name", "create_chore", map[string]any{"name": ""}, "name is required"},
		{"unknown argument", "list_chores", map[string]any{"archived": true}, `unknown field "archived"`},
		{"text for a number", "get_chore", map[string]any{"id": "one"}, "id must be a whole number, not a string"},
		{"fraction for a number", "get_chore", map[string]any{"id": 1.5}, "id must be a whole number, not a number 1.5"},
		{"number for text", "create_chore", map[string]any{"name": 7}, "name must be a string, not a number"},
		{"text for a flag", "list_chores", map[string]any{"includeSubtasks": "yes"}, "includeSubtasks must be true or false, not a string"},
		{"text for a list", "create_chore", map[string]any{"name": "Dust", "assignees": "sam"}, "assignees must be a list, not a string"},
		{"nothing to update", "update_chore", map[string]any{"id": 1}, "nothing to update"},
		{"nothing to update on a record", "update_label", map[string]any{"id": 1}, "nothing to update: give at least one of name, color, isShared"},
		{"null where a value is needed", "update_chore", map[string]any{"id": 1, "name": nil}, "name cannot be null"},
		{"date without a time", "create_chore", map[string]any{"name": "Dust", "dueDate": "2030-01-15"}, "dueDate must be an RFC 3339 timestamp"},
		{"id on a new chore", "create_chore", map[string]any{"name": "Dust", "id": 4}, "id cannot be chosen"},
		{"state missing", "set_thing_state", map[string]any{"id": 1}, "state is required"},
		{"state of the wrong type", "set_thing_state", map[string]any{"id": 1, "state": []int{1}}, "state must be a string, a number or a boolean"},
		{"filter without conditions", "create_filter", map[string]any{"name": "All"}, "conditions is required"},
		{"record that does not exist", "update_label", map[string]any{"id": 999, "name": "Gone"}, "there is no label with id 999"},
		{"chore that does not exist", "update_chore", map[string]any{"id": 999, "name": "Gone"}, "Donetick refused the request"},
		{"value the API turns down", "create_chore", map[string]any{"name": "Dust", "assignStrategy": "whoever"}, "Invalid request format ("},
		// A name in the wrong case would be read as the argument and then left out.
		{"argument in the wrong case", "create_chore", map[string]any{"Name": "Dust"}, `unknown field "Name"`},
		{"argument in the wrong case on an update", "update_chore", map[string]any{"id": 1, "NAME": "Dust"}, `unknown field "NAME"`},
		{"nested argument in the wrong case", "create_chore", map[string]any{"name": "Dust", "frequencyMetadata": map[string]any{"Unit": "days"}}, `unknown field "Unit"`},
		{"list member in the wrong case", "create_filter", map[string]any{"name": "Late", "conditions": []any{map[string]any{"Type": "dueDate", "operator": "isOverdue"}}}, `unknown field "Type"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Contains(t, mustFail(t, engine, alexToken, test.tool, test.arguments), test.want)
		})
	}

	t.Run("arguments are not an object", func(t *testing.T) {
		r := post(t, engine, alexToken, legacy("tools/call", map[string]any{"name": "list_chores", "arguments": []int{1}}))
		require.Equal(t, true, r.result["isError"], r.body)
		require.Contains(t, r.body, "arguments must be an object")
	})

	for name, params := range map[string]map[string]any{
		"unknown tool": {"name": "format_disk"},
		"no tool name": {"arguments": map[string]any{}},
		// Tools for managing people and access are deliberately absent.
		"member list": {"name": "list_circle_members"},
	} {
		t.Run(name, func(t *testing.T) {
			r := post(t, engine, alexToken, legacy("tools/call", params))
			require.Equal(t, http.StatusOK, r.status, r.body)
			require.Equal(t, codeInvalidParams, r.err.Code, r.body)
		})
	}
}

func TestResultsAreNotHTMLEscaped(t *testing.T) {
	for _, test := range []struct{ name, body, want string }{
		{"ampersand", `{"name":"Wash \u0026 fold"}`, `{"name":"Wash & fold"}`},
		{"angle brackets", `"\u003cb\u003e"`, `"<b>"`},
		{"other escapes are kept", `"line\nbreak \u00e9 \u2028 \"quoted\""`, `"line\nbreak \u00e9 \u2028 \"quoted\""`},
		{"an escaped backslash is not an escape", `"C:\\u0026"`, `"C:\\u0026"`},
		{"an escaped backslash before an escape", `"\\\u0026"`, `"\\&"`},
		{"cut short", `"\u00`, `"\u00`},
		{"trailing backslash", `\u0026\`, `&\`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, string(unescapeHTML([]byte(test.body))))
		})
	}

	engine, _ := newServer(t)
	const name = `Wash & fold <delicates> at "C:\u0026"`
	id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{"name": name})
	text, isError := callTool(t, engine, alexToken, "get_chore", map[string]any{"id": id})
	require.False(t, isError, text)
	require.Contains(t, text, `"name":"Wash & fold <delicates> at \"C:\\u0026\""`)
	require.Equal(t, name, mustCall[chModel.Chore](t, engine, alexToken, "get_chore", map[string]any{"id": id}).Name, "the result is still the same JSON")
}

// storedChore reads a chore straight from the database, with what the tests look at.
func storedChore(t *testing.T, db *gorm.DB, id int) chModel.Chore {
	t.Helper()
	var chore chModel.Chore
	require.NoError(t, db.Preload("Assignees").Preload("LabelsV2").Preload("ThingChore").First(&chore, id).Error)
	return chore
}

func assignees(chore chModel.Chore) []int {
	ids := []int{}
	for _, assignee := range chore.Assignees {
		ids = append(ids, assignee.UserID)
	}
	return ids
}

func labelNames(chore chModel.Chore) []string {
	names := []string{}
	if chore.LabelsV2 != nil {
		for _, label := range *chore.LabelsV2 {
			names = append(names, label.Name)
		}
	}
	return names
}

func TestCreateChore(t *testing.T) {
	engine, db := newServer(t)

	t.Run("a bare chore is a one-time chore for the person asking", func(t *testing.T) {
		id := mustCall[int](t, engine, samToken, "create_chore", map[string]any{"name": "Buy stamps"})
		created := storedChore(t, db, id)
		require.Equal(t, "Buy stamps", created.Name)
		require.Equal(t, chModel.FrequencyTypeOnce, created.FrequencyType)
		require.Equal(t, sam, created.CreatedBy)
		require.Equal(t, sam, *created.AssignedTo)
		require.Equal(t, []int{sam}, assignees(created))
		require.Nil(t, created.NextDueDate)
		require.True(t, created.IsActive)
	})

	t.Run("every field arrives", func(t *testing.T) {
		kitchen := mustCall[lModel.Label](t, engine, alexToken, "create_label", map[string]any{"name": "Kitchen", "isShared": true})
		home := mustCall[pModel.Project](t, engine, alexToken, "create_project", map[string]any{"name": "Home"})
		id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
			"name": "Descale the kettle", "description": "Use citric acid", "dueDate": "2030-01-15T08:30:00+01:00",
			"frequencyType": "interval", "frequency": 6, "frequencyMetadata": map[string]any{"unit": "weeks", "timezone": "Europe/Berlin"},
			"isRolling": true, "assignees": []int{alex, sam}, "assignedTo": sam, "assignStrategy": "round_robin",
			"priority": 2, "points": 5, "labels": []int{kitchen.ID}, "projectId": home.ID, "requireApproval": true,
		})
		created := storedChore(t, db, id)
		require.Equal(t, "Use citric acid", *created.Description)
		require.Equal(t, "2030-01-15T07:30:00Z", created.NextDueDate.UTC().Format(time.RFC3339))
		require.Equal(t, chModel.FrequencyTypeInterval, created.FrequencyType)
		require.Equal(t, 6, created.Frequency)
		require.Equal(t, "weeks", *created.FrequencyMetadataV2.Unit)
		require.Equal(t, "Europe/Berlin", created.FrequencyMetadataV2.Timezone)
		require.Equal(t, "2030-01-15T08:30:00+01:00", created.FrequencyMetadataV2.Time, "the time of day defaults to that of the due date")
		require.True(t, created.IsRolling)
		require.ElementsMatch(t, []int{alex, sam}, assignees(created))
		require.Equal(t, sam, *created.AssignedTo)
		require.Equal(t, chModel.AssignmentStrategyRoundRobin, created.AssignStrategy)
		require.Equal(t, 2, created.Priority)
		require.Equal(t, 5, *created.Points)
		require.Equal(t, []string{"Kitchen"}, labelNames(created))
		require.Equal(t, home.ID, *created.ProjectID)
		require.True(t, created.RequireApproval)
	})

	t.Run("a chore can be left for anyone", func(t *testing.T) {
		id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{"name": "Empty the dishwasher", "assignees": []int{}})
		created := storedChore(t, db, id)
		require.Empty(t, assignees(created))
		require.Nil(t, created.AssignedTo)
	})

	// The API leaves a chore unassigned unless told who is next, and an
	// unassigned chore reminds nobody.
	for _, test := range []struct {
		name      string
		arguments map[string]any
		want      *int
	}{
		{"the first assignee is next", map[string]any{"assignees": []int{sam}}, ptr(sam)},
		{"the person asking is next when they are an assignee", map[string]any{"assignees": []int{sam, alex}}, ptr(alex)},
		{"unless somebody is named", map[string]any{"assignees": []int{sam, alex}, "assignedTo": sam}, ptr(sam)},
		{"or nobody is to be", map[string]any{"assignees": []int{sam, alex}, "assignedTo": nil}, nil},
		{"or the chore has no assignee by design", map[string]any{"assignStrategy": "no_assignee"}, nil},
	} {
		t.Run("who does a new chore next: "+test.name, func(t *testing.T) {
			test.arguments["name"] = "Sweep the porch"
			created := storedChore(t, db, mustCall[int](t, engine, alexToken, "create_chore", test.arguments))
			require.Equal(t, test.want, created.AssignedTo)
		})
	}

	t.Run("the API's own checks apply", func(t *testing.T) {
		text := mustFail(t, engine, alexToken, "create_chore", map[string]any{"name": "Not yours", "assignees": []int{pat}})
		require.Contains(t, text, "Assignee not found in circle")
	})

	// The API stores these as they are and then fails when the chore is completed.
	for name, arguments := range map[string]map[string]any{
		"interval without a unit":      {"frequencyType": "interval", "frequency": 3},
		"interval without a count":     {"frequencyType": "interval", "frequencyMetadata": map[string]any{"unit": "days"}},
		"weekdays without days":        {"frequencyType": "days_of_the_week"},
		"week pattern without weeks":   {"frequencyType": "days_of_the_week", "frequencyMetadata": map[string]any{"days": []string{"monday"}, "weekPattern": "week_of_month"}},
		"day of month without months":  {"frequencyType": "day_of_the_month", "frequency": 15},
		"day of month that is not one": {"frequencyType": "day_of_the_month", "frequency": 32, "frequencyMetadata": map[string]any{"months": []string{"may"}}},
	} {
		t.Run("a recurrence that cannot be scheduled is turned down: "+name, func(t *testing.T) {
			var before, after int64
			require.NoError(t, db.Model(&chModel.Chore{}).Count(&before).Error)
			arguments["name"] = "Broken"
			require.Contains(t, mustFail(t, engine, alexToken, "create_chore", arguments), "needs")
			require.NoError(t, db.Model(&chModel.Chore{}).Count(&after).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestUpdateChore(t *testing.T) {
	engine, db := newServer(t)
	kitchen := mustCall[lModel.Label](t, engine, alexToken, "create_label", map[string]any{"name": "Kitchen", "isShared": true})
	weekly := mustCall[lModel.Label](t, engine, alexToken, "create_label", map[string]any{"name": "Weekly", "isShared": true})
	home := mustCall[pModel.Project](t, engine, alexToken, "create_project", map[string]any{"name": "Home"})
	id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
		"name": "Mop the floor", "description": "Kitchen and hall", "dueDate": "2030-03-04T09:00:00Z",
		"frequencyType": "days_of_the_week", "frequencyMetadata": map[string]any{"days": []string{"monday", "thursday"}},
		"assignees": []int{alex, sam}, "assignedTo": sam, "assignStrategy": "round_robin",
		"priority": 3, "points": 2, "labels": []int{kitchen.ID}, "projectId": home.ID,
	})
	before := storedChore(t, db, id)

	t.Run("fields that are not given keep their value", func(t *testing.T) {
		text, isError := callTool(t, engine, alexToken, "update_chore", map[string]any{"id": id, "name": "Mop the floors"})
		require.False(t, isError, text)

		after := storedChore(t, db, id)
		require.Equal(t, "Mop the floors", after.Name)
		require.Equal(t, "Kitchen and hall", *after.Description)
		require.Equal(t, before.NextDueDate.UTC(), after.NextDueDate.UTC())
		require.Equal(t, before.FrequencyType, after.FrequencyType)
		require.Equal(t, before.FrequencyMetadataV2, after.FrequencyMetadataV2)
		require.ElementsMatch(t, []int{alex, sam}, assignees(after))
		require.Equal(t, sam, *after.AssignedTo)
		require.Equal(t, chModel.AssignmentStrategyRoundRobin, after.AssignStrategy)
		require.Equal(t, 3, after.Priority)
		require.Equal(t, 2, *after.Points)
		require.Equal(t, []string{"Kitchen"}, labelNames(after))
		require.Equal(t, home.ID, *after.ProjectID)
		require.True(t, after.IsActive)
		require.Equal(t, alex, after.CreatedBy)
	})

	t.Run("several fields change at once", func(t *testing.T) {
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
			"id": id, "dueDate": "2030-03-05T18:00:00Z", "frequencyType": "weekly", "frequencyMetadata": nil,
			"assignees": []int{alex}, "assignedTo": alex, "priority": 1, "labels": []int{kitchen.ID, weekly.ID},
		})
		after := storedChore(t, db, id)
		require.Equal(t, "2030-03-05T18:00:00Z", after.NextDueDate.UTC().Format(time.RFC3339))
		require.Equal(t, chModel.FrequencyTypeWeekly, after.FrequencyType)
		require.Equal(t, []int{alex}, assignees(after))
		require.Equal(t, alex, *after.AssignedTo)
		require.Equal(t, 1, after.Priority)
		require.ElementsMatch(t, []string{"Kitchen", "Weekly"}, labelNames(after))
		require.Equal(t, "Mop the floors", after.Name)
	})

	t.Run("null clears a field", func(t *testing.T) {
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
			"id": id, "description": nil, "assignedTo": nil, "points": nil, "projectId": nil, "labels": []int{},
		})
		after := storedChore(t, db, id)
		require.Nil(t, after.Description)
		require.Nil(t, after.AssignedTo)
		require.Nil(t, after.Points)
		require.Nil(t, after.ProjectID)
		require.Empty(t, labelNames(after))
		require.Equal(t, "Mop the floors", after.Name)
	})

	t.Run("a recurrence that cannot be scheduled is turned down", func(t *testing.T) {
		text := mustFail(t, engine, alexToken, "update_chore", map[string]any{"id": id, "frequencyType": "interval"})
		require.Contains(t, text, `frequencyType "interval" needs frequency`)
		require.Equal(t, chModel.FrequencyTypeWeekly, storedChore(t, db, id).FrequencyType)
	})

	t.Run("only the creator or an admin may update", func(t *testing.T) {
		text := mustFail(t, engine, samToken, "update_chore", map[string]any{"id": id, "name": "Mine now"})
		require.Contains(t, text, "HTTP 403")
		require.Contains(t, text, "user does not have permission to edit this chore")
		require.Equal(t, "Mop the floors", storedChore(t, db, id).Name)
	})

	t.Run("chores of another circle are out of reach", func(t *testing.T) {
		mustFail(t, engine, patToken, "update_chore", map[string]any{"id": id, "name": "Mine now"})
		require.Equal(t, "Mop the floors", storedChore(t, db, id).Name)
	})

	t.Run("recurrence details change one by one", func(t *testing.T) {
		weekdays := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
			"name": "Bins out", "dueDate": "2030-03-04T09:00:00+01:00", "frequencyType": "days_of_the_week",
			"frequencyMetadata": map[string]any{"days": []string{"monday"}, "timezone": "Europe/Berlin"},
		})
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
			"id": weekdays, "frequencyMetadata": map[string]any{"days": []string{"monday", "friday"}},
		})
		details := storedChore(t, db, weekdays).FrequencyMetadataV2
		require.Equal(t, []string{"monday", "friday"}, []string{*details.Days[0], *details.Days[1]})
		require.Equal(t, "Europe/Berlin", details.Timezone, "the time zone was reset")
		require.Equal(t, "2030-03-04T09:00:00+01:00", details.Time, "the time of day was reset")
	})

	t.Run("the time of day follows a new due date", func(t *testing.T) {
		daily := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
			"name": "Feed the cat", "dueDate": "2030-01-19T18:00:00Z",
			"frequencyType": "interval", "frequency": 1, "frequencyMetadata": map[string]any{"unit": "days"},
		})
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{"id": daily, "dueDate": "2030-01-20T07:00:00Z"})
		require.Equal(t, "2030-01-20T07:00:00Z", storedChore(t, db, daily).FrequencyMetadataV2.Time)
		completed := mustCall[chModel.Chore](t, engine, alexToken, "complete_chore", map[string]any{"id": daily, "completedTime": "2030-01-20T07:05:00Z"})
		require.Equal(t, "2030-01-21T07:00:00Z", completed.NextDueDate.UTC().Format(time.RFC3339), "the next occurrence went back to the old time of day")

		// A time that is given outright is kept, and other updates leave it alone.
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
			"id": daily, "dueDate": "2030-01-22T07:00:00Z", "frequencyMetadata": map[string]any{"time": "2030-01-22T20:30:00Z"},
		})
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{"id": daily, "name": "Feed the cats"})
		require.Equal(t, "2030-01-22T20:30:00Z", storedChore(t, db, daily).FrequencyMetadataV2.Time)
	})

	t.Run("a chore triggered by somebody else's thing is left alone", func(t *testing.T) {
		// The API would save the change, drop the link and then refuse.
		thing := mustCall[tModel.Thing](t, engine, samToken, "create_thing", map[string]any{"name": "Laundry basket is full", "type": "boolean", "state": false})
		triggered := mustCall[int](t, engine, samToken, "create_chore", map[string]any{"name": "Do the laundry"})
		require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", triggered).Update("frequency_type", chModel.FrequencyTypeTrigger).Error)
		require.NoError(t, db.Create(&tModel.ThingChore{ThingID: thing.ID, ChoreID: triggered, TriggerState: "true", Condition: "eq"}).Error)

		text := mustFail(t, engine, alexToken, "update_chore", map[string]any{"id": triggered, "name": "Do all the laundry"})
		require.Contains(t, text, "belongs to another user")
		after := storedChore(t, db, triggered)
		require.Equal(t, "Do the laundry", after.Name)
		require.NotNil(t, after.ThingChore, "the link to the thing was lost")

		mustSucceed(t, engine, samToken, "update_chore", map[string]any{"id": triggered, "name": "Do all the laundry"})
		after = storedChore(t, db, triggered)
		require.Equal(t, "Do all the laundry", after.Name)
		require.NotNil(t, after.ThingChore, "the link to the thing was lost")
	})

	t.Run("a chore triggered by a thing stays linked to it", func(t *testing.T) {
		thing := mustCall[tModel.Thing](t, engine, alexToken, "create_thing", map[string]any{"name": "Bin is full", "type": "boolean", "state": false})
		triggered := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{"name": "Take out the bin"})
		require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", triggered).Update("frequency_type", chModel.FrequencyTypeTrigger).Error)
		require.NoError(t, db.Create(&tModel.ThingChore{ThingID: thing.ID, ChoreID: triggered, TriggerState: "true", Condition: "eq"}).Error)

		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{"id": triggered, "name": "Take out the bins"})
		after := storedChore(t, db, triggered)
		require.Equal(t, "Take out the bins", after.Name)
		require.NotNil(t, after.ThingChore, "the link to the thing was lost")
		require.Equal(t, thing.ID, after.ThingChore.ThingID)
		require.Equal(t, "true", after.ThingChore.TriggerState)
		require.Equal(t, "eq", after.ThingChore.Condition)
	})
}

func TestChoreLifecycle(t *testing.T) {
	engine, db := newServer(t)
	id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
		"name": "Water the plants", "dueDate": "2030-01-15T08:00:00Z",
		"frequencyType": "interval", "frequency": 3, "frequencyMetadata": map[string]any{"unit": "days"},
		"assignees": []int{alex, sam}, "assignedTo": alex, "assignStrategy": "keep_last_assigned",
	})
	dueDate := func() string { return storedChore(t, db, id).NextDueDate.UTC().Format(time.RFC3339) }

	listed := mustCall[[]chModel.Chore](t, engine, samToken, "list_chores", nil)
	require.Len(t, listed, 1)
	require.Equal(t, "Water the plants", listed[0].Name)
	require.Empty(t, mustCall[[]chModel.Chore](t, engine, patToken, "list_chores", nil), "chores stay inside their circle")
	require.Equal(t, "Water the plants", mustCall[chModel.Chore](t, engine, alexToken, "get_chore", map[string]any{"id": id}).Name)

	t.Run("complete", func(t *testing.T) {
		completed := mustCall[chModel.Chore](t, engine, alexToken, "complete_chore", map[string]any{
			"id": id, "notes": "Ferns were dry", "completedTime": "2030-01-15T10:00:00Z",
		})
		require.Equal(t, "2030-01-18T08:00:00Z", completed.NextDueDate.UTC().Format(time.RFC3339), "the next occurrence is three days on")

		var history chModel.ChoreHistory
		require.NoError(t, db.Where("chore_id = ?", id).First(&history).Error)
		require.Equal(t, alex, history.CompletedBy)
		require.Equal(t, "Ferns were dry", *history.Note)
		require.Equal(t, "2030-01-15T10:00:00Z", history.PerformedAt.UTC().Format(time.RFC3339))
	})

	t.Run("only an admin or manager completes for somebody else", func(t *testing.T) {
		text := mustFail(t, engine, samToken, "complete_chore", map[string]any{"id": id, "completedBy": alex})
		require.Contains(t, text, "HTTP 403")
		require.Equal(t, "2030-01-18T08:00:00Z", dueDate())

		mustSucceed(t, engine, alexToken, "complete_chore", map[string]any{"id": id, "completedBy": sam, "completedTime": "2030-01-18T09:00:00Z"})
		var history chModel.ChoreHistory
		require.NoError(t, db.Where("chore_id = ?", id).Order("id desc").First(&history).Error)
		require.Equal(t, sam, history.CompletedBy)
		require.Equal(t, "2030-01-21T08:00:00Z", dueDate())
	})

	t.Run("skip", func(t *testing.T) {
		mustCall[chModel.Chore](t, engine, alexToken, "skip_chore", map[string]any{"id": id})
		require.Equal(t, "2030-01-24T08:00:00Z", dueDate())
	})

	t.Run("history", func(t *testing.T) {
		history := mustCall[[]chModel.ChoreHistory](t, engine, alexToken, "get_chore_history", map[string]any{"id": id})
		require.Len(t, history, 3)
		mustFail(t, engine, patToken, "get_chore_history", map[string]any{"id": id})

		text, isError := callTool(t, engine, alexToken, "list_recent_activity", map[string]any{"days": 30, "includeMembers": true})
		require.False(t, isError, text)
	})

	t.Run("archive and unarchive", func(t *testing.T) {
		mustSucceed(t, engine, alexToken, "archive_chore", map[string]any{"id": id})
		require.False(t, storedChore(t, db, id).IsActive)
		require.Empty(t, mustCall[[]chModel.Chore](t, engine, alexToken, "list_chores", nil))
		require.Len(t, mustCall[[]chModel.Chore](t, engine, alexToken, "list_chores", map[string]any{"includeArchived": true}), 1)

		mustSucceed(t, engine, alexToken, "unarchive_chore", map[string]any{"id": id})
		require.True(t, storedChore(t, db, id).IsActive)
	})

	t.Run("delete", func(t *testing.T) {
		text := mustFail(t, engine, samToken, "delete_chore", map[string]any{"id": id})
		require.Contains(t, text, "You are not allowed to delete this chore")

		mustSucceed(t, engine, alexToken, "delete_chore", map[string]any{"id": id})
		require.ErrorIs(t, db.First(&chModel.Chore{}, id).Error, gorm.ErrRecordNotFound)
	})
}

func TestLabelTools(t *testing.T) {
	engine, db := newServer(t)
	label := func(id int) lModel.Label {
		var label lModel.Label
		require.NoError(t, db.First(&label, id).Error)
		return label
	}

	created := mustCall[lModel.Label](t, engine, alexToken, "create_label", map[string]any{"name": "Kitchen", "color": "#4caf50", "isShared": true})
	require.Equal(t, lModel.Label{ID: created.ID, Name: "Kitchen", Color: "#4caf50", IsShared: true, CreatedBy: alex}, withoutCircle(label(created.ID)))
	private := mustCall[lModel.Label](t, engine, alexToken, "create_label", map[string]any{"name": "Secret"})
	require.False(t, label(private.ID).IsShared)

	require.Len(t, mustCall[[]lModel.Label](t, engine, alexToken, "list_labels", nil), 2)
	require.Len(t, mustCall[[]lModel.Label](t, engine, samToken, "list_labels", nil), 1, "an unshared label is its creator's alone")
	require.Empty(t, mustCall[[]lModel.Label](t, engine, patToken, "list_labels", nil))

	mustSucceed(t, engine, alexToken, "update_label", map[string]any{"id": created.ID, "name": "Kitchen & pantry"})
	require.Equal(t, lModel.Label{ID: created.ID, Name: "Kitchen & pantry", Color: "#4caf50", IsShared: true, CreatedBy: alex}, withoutCircle(label(created.ID)))
	mustSucceed(t, engine, alexToken, "update_label", map[string]any{"id": created.ID, "color": "#ff9800", "isShared": false})
	require.Equal(t, lModel.Label{ID: created.ID, Name: "Kitchen & pantry", Color: "#ff9800", CreatedBy: alex}, withoutCircle(label(created.ID)))

	mustSucceed(t, engine, alexToken, "update_label", map[string]any{"id": created.ID, "isShared": true})
	mustFail(t, engine, samToken, "update_label", map[string]any{"id": created.ID, "name": "Mine now"})
	mustFail(t, engine, samToken, "delete_label", map[string]any{"id": created.ID})
	require.Equal(t, "Kitchen & pantry", label(created.ID).Name)

	mustSucceed(t, engine, alexToken, "delete_label", map[string]any{"id": created.ID})
	require.ErrorIs(t, db.First(&lModel.Label{}, created.ID).Error, gorm.ErrRecordNotFound)
}

func ptr[T any](value T) *T { return &value }

func withoutCircle(label lModel.Label) lModel.Label {
	label.CircleID = nil
	return label
}

func TestProjectTools(t *testing.T) {
	engine, db := newServer(t)
	project := func(id int) pModel.Project {
		var project pModel.Project
		require.NoError(t, db.First(&project, id).Error)
		return project
	}

	created := mustCall[pModel.Project](t, engine, alexToken, "create_project", map[string]any{
		"name": "Garden", "description": "Everything outdoors", "color": "#4caf50",
	})
	require.Equal(t, "Garden", project(created.ID).Name)
	require.False(t, project(created.ID).IsPrivate, "new projects are visible to the circle")
	icon := "leaf"
	require.NoError(t, db.Model(&pModel.Project{}).Where("id = ?", created.ID).Update("icon", icon).Error)

	listed := func(token string) []string {
		names := []string{}
		for _, p := range mustCall[[]pModel.Project](t, engine, token, "list_projects", nil) {
			names = append(names, p.Name)
		}
		return names
	}
	require.Contains(t, listed(samToken), "Garden")
	require.NotContains(t, listed(patToken), "Garden")

	mustSucceed(t, engine, alexToken, "update_project", map[string]any{"id": created.ID, "name": "Garden & balcony"})
	after := project(created.ID)
	require.Equal(t, "Garden & balcony", after.Name)
	require.Equal(t, "Everything outdoors", *after.Description, "fields that are not given keep their value")
	require.Equal(t, "#4caf50", *after.Color)
	require.Equal(t, icon, *after.Icon)
	require.False(t, after.IsPrivate)

	mustSucceed(t, engine, alexToken, "update_project", map[string]any{"id": created.ID, "description": nil})
	require.Nil(t, project(created.ID).Description)
	require.Equal(t, "Garden & balcony", project(created.ID).Name)

	mustFail(t, engine, samToken, "update_project", map[string]any{"id": created.ID, "name": "Mine now"})
	mustFail(t, engine, samToken, "delete_project", map[string]any{"id": created.ID})
	require.Contains(t, mustFail(t, engine, patToken, "update_project", map[string]any{"id": created.ID, "name": "Mine now"}), "there is no project with id")
	require.Equal(t, "Garden & balcony", project(created.ID).Name)

	choreID := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{"name": "Mow the lawn", "projectId": created.ID})
	mustSucceed(t, engine, alexToken, "delete_project", map[string]any{"id": created.ID})
	require.ErrorIs(t, db.First(&pModel.Project{}, created.ID).Error, gorm.ErrRecordNotFound)
	require.Nil(t, storedChore(t, db, choreID).ProjectID, "the chores of a deleted project are kept")
}

func TestFilterTools(t *testing.T) {
	engine, db := newServer(t)
	filter := func(id int) fModel.Filter {
		var filter fModel.Filter
		require.NoError(t, db.First(&filter, id).Error)
		return filter
	}
	overdue := map[string]any{"type": "dueDate", "operator": "isOverdue"}
	urgent := map[string]any{"type": "priority", "operator": "is", "value": []int{1, 2}}

	created := mustCall[fModel.Filter](t, engine, alexToken, "create_filter", map[string]any{
		"name": "Late and urgent", "description": "Do these first", "color": "#f44336",
		"conditions": []any{overdue, urgent}, "isPinned": true,
	})
	stored := filter(created.ID)
	require.Equal(t, "Late and urgent", stored.Name)
	require.Equal(t, fModel.LogicalOperatorAND, stored.Operator)
	require.True(t, stored.IsPinned)
	require.Len(t, stored.Conditions, 2)
	require.Equal(t, fModel.FilterCondition{Type: "dueDate", Operator: "isOverdue"}, stored.Conditions[0])
	require.Equal(t, "priority", stored.Conditions[1].Type)
	require.Equal(t, []any{1.0, 2.0}, stored.Conditions[1].Value)

	require.Len(t, mustCall[[]fModel.Filter](t, engine, alexToken, "list_filters", nil), 1)
	require.Empty(t, mustCall[[]fModel.Filter](t, engine, samToken, "list_filters", nil), "an unshared filter is its creator's alone")

	mustSucceed(t, engine, alexToken, "update_filter", map[string]any{"id": created.ID, "name": "Late & urgent", "operator": "OR"})
	stored = filter(created.ID)
	require.Equal(t, "Late & urgent", stored.Name)
	require.Equal(t, fModel.LogicalOperatorOR, stored.Operator)
	require.Equal(t, "Do these first", *stored.Description, "fields that are not given keep their value")
	require.Equal(t, "#f44336", *stored.Color)
	require.True(t, stored.IsPinned)
	require.Len(t, stored.Conditions, 2)

	mustSucceed(t, engine, alexToken, "update_filter", map[string]any{"id": created.ID, "conditions": []any{overdue}, "description": nil})
	stored = filter(created.ID)
	require.Len(t, stored.Conditions, 1)
	require.Nil(t, stored.Description)
	require.Equal(t, "Late & urgent", stored.Name)

	require.Contains(t, mustFail(t, engine, alexToken, "create_filter", map[string]any{
		"name": "Broken", "conditions": []any{map[string]any{"type": "priority", "operator": "is"}},
	}), "requires a value")
	mustFail(t, engine, samToken, "update_filter", map[string]any{"id": created.ID, "name": "Mine now"})
	mustFail(t, engine, samToken, "delete_filter", map[string]any{"id": created.ID})
	require.Equal(t, "Late & urgent", filter(created.ID).Name)

	mustSucceed(t, engine, alexToken, "delete_filter", map[string]any{"id": created.ID})
	require.ErrorIs(t, db.First(&fModel.Filter{}, created.ID).Error, gorm.ErrRecordNotFound)
}

func TestThingTools(t *testing.T) {
	engine, db := newServer(t)
	thing := func(id int) tModel.Thing {
		var thing tModel.Thing
		require.NoError(t, db.First(&thing, id).Error)
		return thing
	}

	loads := mustCall[tModel.Thing](t, engine, alexToken, "create_thing", map[string]any{"name": "Dishwasher loads", "type": "number", "state": 3})
	require.Equal(t, "3", thing(loads.ID).State)
	require.Equal(t, alex, thing(loads.ID).UserID)
	away := mustCall[tModel.Thing](t, engine, alexToken, "create_thing", map[string]any{"name": "On holiday", "type": "boolean", "state": false})
	require.Equal(t, "false", thing(away.ID).State)
	require.Contains(t, mustFail(t, engine, alexToken, "create_thing", map[string]any{"name": "Broken", "type": "number", "state": "many"}), "Invalid state")

	require.Len(t, mustCall[[]tModel.Thing](t, engine, alexToken, "list_things", nil), 2)
	require.Empty(t, mustCall[[]tModel.Thing](t, engine, samToken, "list_things", nil), "things belong to their owner")

	require.Equal(t, "42", mustCall[tModel.Thing](t, engine, alexToken, "set_thing_state", map[string]any{"id": loads.ID, "state": 42}).State)
	require.Equal(t, "42", thing(loads.ID).State)
	mustCall[tModel.Thing](t, engine, alexToken, "set_thing_state", map[string]any{"id": away.ID, "state": true})
	require.Equal(t, "true", thing(away.ID).State)
	require.Contains(t, mustFail(t, engine, alexToken, "set_thing_state", map[string]any{"id": loads.ID, "state": "many"}), "Invalid state")
	require.Contains(t, mustFail(t, engine, samToken, "set_thing_state", map[string]any{"id": loads.ID, "state": 1}), "HTTP 403")
	require.Equal(t, "42", thing(loads.ID).State)

	mustSucceed(t, engine, alexToken, "update_thing", map[string]any{"id": loads.ID, "name": "Loads of dishes"})
	require.Equal(t, "Loads of dishes", thing(loads.ID).Name)
	require.Equal(t, "number", thing(loads.ID).Type, "fields that are not given keep their value")
	require.Equal(t, "42", thing(loads.ID).State)
	require.Contains(t, mustFail(t, engine, alexToken, "update_thing", map[string]any{"id": loads.ID, "type": "boolean"}), "state is required when the type changes")
	require.Equal(t, "number", thing(loads.ID).Type)
	mustSucceed(t, engine, alexToken, "update_thing", map[string]any{"id": loads.ID, "type": "number", "name": "Loads of dishes"})
	mustSucceed(t, engine, alexToken, "update_thing", map[string]any{"id": loads.ID, "type": "text", "state": "plenty"})
	require.Equal(t, "text", thing(loads.ID).Type)
	require.Equal(t, "plenty", thing(loads.ID).State)
	require.Contains(t, mustFail(t, engine, samToken, "update_thing", map[string]any{"id": loads.ID, "name": "Mine now"}), "there is no thing with id")

	require.Contains(t, mustFail(t, engine, samToken, "delete_thing", map[string]any{"id": loads.ID}), "HTTP 403")
	mustSucceed(t, engine, alexToken, "delete_thing", map[string]any{"id": loads.ID})
	require.ErrorIs(t, db.First(&tModel.Thing{}, loads.ID).Error, gorm.ErrRecordNotFound)
}

// A state change runs the chores linked to the thing, which is the reason
// things exist; it has to keep working through the tool.
func TestSettingAThingRunsItsChores(t *testing.T) {
	engine, db := newServer(t)
	bin := mustCall[tModel.Thing](t, engine, alexToken, "create_thing", map[string]any{"name": "Bin is full", "type": "boolean", "state": false})
	id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{"name": "Take out the bin"})
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", id).Update("frequency_type", chModel.FrequencyTypeTrigger).Error)
	require.NoError(t, db.Create(&tModel.ThingChore{ThingID: bin.ID, ChoreID: id, TriggerState: "true", Condition: "eq"}).Error)
	require.Nil(t, storedChore(t, db, id).NextDueDate)

	mustCall[tModel.Thing](t, engine, alexToken, "set_thing_state", map[string]any{"id": bin.ID, "state": true})
	due := storedChore(t, db, id).NextDueDate
	require.NotNil(t, due, "the chore linked to the thing was not scheduled")
	require.WithinDuration(t, time.Now().UTC(), *due, time.Minute)
	require.False(t, strings.Contains(storedChore(t, db, id).Name, "bins"))
}
