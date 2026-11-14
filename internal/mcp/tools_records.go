package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The tools in this file manage the records chores refer to: labels,
// projects, saved filters and things.

const colorHelp = "Color as a hex code, for example #4caf50."

// changes reads the arguments of an update tool: the id of the record and the
// fields to change. The API replaces these records whole, so the tools start
// from the record as it is and write only the given fields over it.
func changes(arguments json.RawMessage, into any, fields ...string) (id int, given object, err error) {
	given, err = decodeArguments(arguments, into)
	if err != nil {
		return 0, nil, err
	}
	if json.Unmarshal(given["id"], &id) != nil {
		return 0, nil, errors.New("id is required")
	}
	given = pick(given, fields...)
	if len(given) == 0 {
		return 0, nil, fmt.Errorf("nothing to update: give at least one of %s", strings.Join(fields, ", "))
	}
	return id, given, nil
}

// notNull turns down a null for arguments that cannot be cleared.
func notNull(given object, names ...string) error {
	for _, name := range names {
		if string(given[name]) == "null" {
			return fmt.Errorf("%s cannot be null", name)
		}
	}
	return nil
}

func requireName(given object) error {
	var name string
	if json.Unmarshal(given["name"], &name) != nil || strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return nil
}

var labelProperties = schema{
	"name":     property("string", "Name of the label."),
	"color":    property("string", colorHelp),
	"isShared": property("boolean", "Share the label with the whole circle. A label that is not shared is visible only to its creator."),
}

type labelArguments struct {
	ID       *int    `json:"id"`
	Name     *string `json:"name"`
	Color    *string `json:"color"`
	IsShared *bool   `json:"isShared"`
}

var labelTools = []tool{
	{
		Name:        "list_labels",
		Title:       "List labels",
		Description: "List the labels the user can put on chores: their own and the ones shared with the circle.",
		InputSchema: inputs(nil),
		Annotations: readOnly,
		run:         fixed(http.MethodGet, "/api/v1/labels"),
	},
	{
		Name:        "create_label",
		Title:       "Create label",
		Description: "Create a label. Returns the new label; put it on chores with the labels argument of create_chore or update_chore.",
		InputSchema: inputs(labelProperties, "name"),
		Annotations: additive,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			given, err := decodeArguments(arguments, &labelArguments{})
			if err == nil {
				err = errors.Join(notNull(given, "color", "isShared"), requireName(given))
			}
			return apiCall{method: http.MethodPost, path: "/api/v1/labels", body: pick(given, "name", "color", "isShared")}, err
		}),
	},
	{
		Name:        "update_label",
		Title:       "Update label",
		Description: "Rename or recolor a label, or change whether it is shared. Fields that are left out keep their value. Only the label's creator may update it.",
		InputSchema: inputs(labelProperties.with(schema{"id": property("integer", "Id of the label, as returned by list_labels.")}), "id"),
		Annotations: overwriting,
		run: func(s *session, arguments json.RawMessage) (apiResponse, error) {
			id, given, err := changes(arguments, &labelArguments{}, "name", "color", "isShared")
			if err == nil {
				err = notNull(given, "name", "color", "isShared")
			}
			if err != nil {
				return apiResponse{}, err
			}
			label, err := s.find("/api/v1/labels", "label", id)
			if err != nil {
				return apiResponse{}, err
			}
			// isShared is not carried over: the API keeps it when it is not sent.
			body := pick(label, "id", "name", "color")
			overlay(body, given, "name", "color", "isShared")
			return s.call(apiCall{method: http.MethodPut, path: "/api/v1/labels", body: body})
		},
	},
	{
		Name:        "delete_label",
		Title:       "Delete label",
		Description: "Delete a label and take it off every chore that has it. Only the label's creator may delete it.",
		InputSchema: inputs(schema{"id": property("integer", "Id of the label, as returned by list_labels.")}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodDelete, "/api/v1/labels/%d"),
	},
}

var projectProperties = schema{
	"name":        property("string", "Name of the project."),
	"description": nullable("string", "What the project is for."),
	"color":       nullable("string", colorHelp),
}

type projectArguments struct {
	ID          *int    `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

var projectTools = []tool{
	{
		Name:        "list_projects",
		Title:       "List projects",
		Description: "List the projects the user can see. A project groups chores; put a chore in one with the projectId argument of create_chore or update_chore.",
		InputSchema: inputs(nil),
		Annotations: readOnly,
		run:         fixed(http.MethodGet, "/api/v1/projects"),
	},
	{
		Name:        "create_project",
		Title:       "Create project",
		Description: "Create a project that everyone in the circle can see. Returns the new project.",
		InputSchema: inputs(projectProperties, "name"),
		Annotations: additive,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			given, err := decodeArguments(arguments, &projectArguments{})
			if err == nil {
				err = requireName(given)
			}
			return apiCall{method: http.MethodPost, path: "/api/v1/projects", body: pick(given, "name", "description", "color")}, err
		}),
	},
	{
		Name:        "update_project",
		Title:       "Update project",
		Description: "Change the name, description or color of a project. Fields that are left out keep their value. Only the project's creator may update it. Who can see the project is not changed.",
		InputSchema: inputs(projectProperties.with(schema{"id": property("integer", "Id of the project, as returned by list_projects.")}), "id"),
		Annotations: overwriting,
		run: func(s *session, arguments json.RawMessage) (apiResponse, error) {
			id, given, err := changes(arguments, &projectArguments{}, "name", "description", "color")
			if err == nil {
				err = notNull(given, "name")
			}
			if err != nil {
				return apiResponse{}, err
			}
			project, err := s.find("/api/v1/projects", "project", id)
			if err != nil {
				return apiResponse{}, err
			}
			// Only these four are sent: the API keeps who can see the project
			// when it is not told otherwise.
			body := pick(project, "name", "description", "color", "icon")
			overlay(body, given, "name", "description", "color")
			return s.call(apiCall{method: http.MethodPut, path: fmt.Sprintf("/api/v1/projects/%d", id), body: body})
		},
	},
	{
		Name:        "delete_project",
		Title:       "Delete project",
		Description: "Delete a project. Its chores are kept, without a project. Only the project's creator may delete it.",
		InputSchema: inputs(schema{"id": property("integer", "Id of the project, as returned by list_projects.")}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodDelete, "/api/v1/projects/%d"),
	},
}

var filterProperties = schema{
	"name":        property("string", "Name of the filter."),
	"description": nullable("string", "What the filter shows."),
	"color":       nullable("string", colorHelp),
	"conditions": schema{
		"type":     "array",
		"minItems": 1,
		"maxItems": 20,
		"description": "What a chore has to match. Each condition has a type, an operator and, for most operators, a value:\n" +
			"- assignee, createdBy: operator is or isNot; value is a user id, a list of user ids, or \"me\".\n" +
			"- priority: is or isNot with a priority (0 to 4) or a list of them; greaterThan or lessThan with one priority.\n" +
			"- status: is or isNot; value is 0 (not started), 1 (in progress), 2 (paused) or 3 (waiting for approval), or a list of them.\n" +
			"- dueDate: isOverdue, isDueToday, isDueTomorrow, isDueThisWeek, isDueThisMonth, hasDueDate or hasNoDueDate, without a value; " +
			"before or after with a date; between with a list of two dates.\n" +
			"- label: has or doesNotHave; value is a label id or a list of them.\n" +
			"- project: is or isNot; value is a project id or a list of them.\n" +
			"- points: equals, greaterThan, lessThan, greaterThanOrEqual or lessThanOrEqual; value is a number.",
		"items": schema{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"type", "operator"},
			"properties": schema{
				"type":     oneOf("What the condition looks at.", "assignee", "createdBy", "priority", "status", "dueDate", "label", "project", "points"),
				"operator": property("string", "How the chore is compared with the value."),
				"value":    schema{"description": "What to compare with. Left out for operators that need no value."},
			},
		},
	},
	"operator": oneOf("Whether a chore has to match every condition (AND, the default) or any one of them (OR).", "AND", "OR"),
	"isPinned": property("boolean", "Pin the filter so it is shown first."),
	"isShared": property("boolean", "Share the filter with the whole circle. A filter that is not shared is visible only to its creator."),
}

type filterArguments struct {
	ID          *int    `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Conditions  *[]struct {
		Type     *string `json:"type"`
		Operator *string `json:"operator"`
		Value    any     `json:"value"`
	} `json:"conditions"`
	Operator *string `json:"operator"`
	IsPinned *bool   `json:"isPinned"`
	IsShared *bool   `json:"isShared"`
}

var filterFields = []string{"name", "description", "color", "conditions", "operator", "isPinned", "isShared"}

var filterTools = []tool{
	{
		Name:        "list_filters",
		Title:       "List filters",
		Description: "List the user's saved filters. A filter is a saved set of conditions the Donetick app uses to show a selection of chores.",
		InputSchema: inputs(nil),
		Annotations: readOnly,
		run:         fixed(http.MethodGet, "/api/v1/filters"),
	},
	{
		Name:        "create_filter",
		Title:       "Create filter",
		Description: "Save a filter for the Donetick app to show. Returns the new filter. To answer a question about chores, read them with list_chores instead.",
		InputSchema: inputs(filterProperties, "name", "conditions"),
		Annotations: additive,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args filterArguments
			given, err := decodeArguments(arguments, &args)
			if err == nil {
				err = errors.Join(notNull(given, "operator", "isPinned", "isShared"), requireName(given))
			}
			if err == nil && (args.Conditions == nil || len(*args.Conditions) == 0) {
				err = errors.New("conditions is required: give at least one condition")
			}
			return apiCall{method: http.MethodPost, path: "/api/v1/filters", body: pick(given, filterFields...)}, err
		}),
	},
	{
		Name:        "update_filter",
		Title:       "Update filter",
		Description: "Change a saved filter. Fields that are left out keep their value; conditions, when given, replace all current conditions. Only the filter's creator may update it.",
		InputSchema: inputs(filterProperties.with(schema{"id": property("integer", "Id of the filter, as returned by list_filters.")}), "id"),
		Annotations: overwriting,
		run: func(s *session, arguments json.RawMessage) (apiResponse, error) {
			id, given, err := changes(arguments, &filterArguments{}, filterFields...)
			if err == nil {
				err = notNull(given, "name", "conditions", "operator", "isPinned", "isShared")
			}
			if err != nil {
				return apiResponse{}, err
			}
			filter, err := s.fetch(fmt.Sprintf("/api/v1/filters/%d", id))
			if err != nil {
				return apiResponse{}, err
			}
			// isShared is not carried over: the API keeps it when it is not sent.
			body := pick(filter, "name", "description", "color", "icon", "conditions", "operator", "isPinned")
			overlay(body, given, filterFields...)
			return s.call(apiCall{method: http.MethodPut, path: fmt.Sprintf("/api/v1/filters/%d", id), body: body})
		},
	},
	{
		Name:        "delete_filter",
		Title:       "Delete filter",
		Description: "Delete a saved filter. Only the filter's creator may delete it. No chores are affected.",
		InputSchema: inputs(schema{"id": property("integer", "Id of the filter, as returned by list_filters.")}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodDelete, "/api/v1/filters/%d"),
	},
}

const thingIDHelp = "Numeric id of the thing, as returned by list_things."

var thingProperties = schema{
	"name": property("string", "Name of the thing."),
	"type": oneOf("What kind of value the thing holds.", "number", "boolean", "text"),
	"state": schema{
		"type":        []string{"string", "number", "boolean"},
		"description": "The value: a whole number for a number thing, true or false for a boolean thing, any text for a text thing.",
	},
}

type thingArguments struct {
	ID    *int    `json:"id"`
	Name  *string `json:"name"`
	Type  *string `json:"type"`
	State any     `json:"state"`
}

// state renders the state argument the way the things API stores it: as text.
func (a *thingArguments) state() (string, error) {
	switch state := a.State.(type) {
	case string:
		return state, nil
	case json.Number:
		return state.String(), nil
	case bool:
		if state {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "", errors.New("state is required")
	}
	return "", errors.New("state must be a string, a number or a boolean")
}

var thingTools = []tool{
	{
		Name:        "list_things",
		Title:       "List things",
		Description: "List the user's things. A thing is a named value (a number, a boolean or text) that chores can be linked to, so that they become due when it changes.",
		InputSchema: inputs(nil),
		Annotations: readOnly,
		run:         fixed(http.MethodGet, "/api/v1/things"),
	},
	{
		Name:        "create_thing",
		Title:       "Create thing",
		Description: "Create a thing. Returns the new thing.",
		InputSchema: inputs(thingProperties, "name", "type", "state"),
		Annotations: additive,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args thingArguments
			given, err := decodeArguments(arguments, &args)
			if err == nil {
				err = requireName(given)
			}
			if err == nil && args.Type == nil {
				err = errors.New("type is required: number, boolean or text")
			}
			if err != nil {
				return apiCall{}, err
			}
			state, err := args.state()
			return apiCall{method: http.MethodPost, path: "/api/v1/things", body: map[string]any{
				"name": args.Name, "type": args.Type, "state": state,
			}}, err
		}),
	},
	{
		Name:        "update_thing",
		Title:       "Update thing",
		Description: "Rename a thing or change its type. Fields that are left out keep their value, but a new type needs a state that fits it. To change only the value, use set_thing_state, which also makes the chores linked to the thing due.",
		InputSchema: inputs(thingProperties.with(schema{"id": property("integer", thingIDHelp)}), "id"),
		Annotations: overwriting,
		run: func(s *session, arguments json.RawMessage) (apiResponse, error) {
			var args thingArguments
			id, given, err := changes(arguments, &args, "name", "type", "state")
			if err == nil {
				err = notNull(given, "name", "type", "state")
			}
			if err != nil {
				return apiResponse{}, err
			}
			thing, err := s.find("/api/v1/things", "thing", id)
			if err != nil {
				return apiResponse{}, err
			}
			// The API checks a state against the type only when it is sent one.
			if _, ok := given["state"]; !ok && args.Type != nil && string(thing["type"]) != string(raw(*args.Type)) {
				return apiResponse{}, errors.New("state is required when the type changes: the current value may not fit the new type")
			}
			body := pick(thing, "id", "name", "type")
			overlay(body, given, "name", "type")
			if _, ok := given["state"]; ok {
				state, err := args.state()
				if err != nil {
					return apiResponse{}, err
				}
				body["state"] = raw(state)
			}
			return s.call(apiCall{method: http.MethodPut, path: "/api/v1/things", body: body})
		},
	},
	{
		Name:        "set_thing_state",
		Title:       "Set thing state",
		Description: "Set the value of a thing. A chore linked to the thing becomes due now if the new value matches its trigger and it has no due date yet. Returns the thing.",
		InputSchema: inputs(schema{"id": property("integer", thingIDHelp), "state": thingProperties["state"]}, "id", "state"),
		Annotations: overwriting,
		run: request(func(arguments json.RawMessage) (apiCall, error) {
			var args thingArguments
			if _, err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			if args.Name != nil || args.Type != nil {
				return apiCall{}, errors.New("set_thing_state only takes id and state; use update_thing to change the name or type")
			}
			state, err := args.state()
			if err == nil && state == "" {
				err = errors.New("state must not be empty")
			}
			return apiCall{
				method: http.MethodPut,
				path:   fmt.Sprintf("/api/v1/things/%d/state", *args.ID),
				query:  url.Values{"value": {state}},
			}, err
		}),
	},
	{
		Name:        "delete_thing",
		Title:       "Delete thing",
		Description: "Delete a thing. A thing that chores are linked to cannot be deleted.",
		InputSchema: inputs(schema{"id": property("integer", thingIDHelp)}, "id"),
		Annotations: overwriting,
		run:         byID(http.MethodDelete, "/api/v1/things/%d"),
	},
}
