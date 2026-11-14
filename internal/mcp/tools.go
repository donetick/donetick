package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// tool describes one MCP tool and how a call to it translates into a request
// to the external API.
type tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	InputSchema schema          `json:"inputSchema"`
	Annotations toolAnnotations `json:"annotations"`

	// build turns the arguments of a call into an API request. An error it
	// returns is shown to the model, so it should say how to fix the call.
	build func(arguments json.RawMessage) (apiCall, error)
}

// toolAnnotations are hints clients use to decide, for example, whether a call
// needs the user's confirmation. All four are always sent because the
// protocol's defaults assume the worst.
type toolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

var (
	readOnly    = toolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	additive    = toolAnnotations{}
	overwriting = toolAnnotations{DestructiveHint: true, IdempotentHint: true}
)

// apiCall is a request to one of Donetick's HTTP API routes.
type apiCall struct {
	method string
	path   string
	query  url.Values
	body   any // sent as JSON when not nil
}

type schema map[string]any

func object(properties schema, required ...string) schema {
	s := schema{"type": "object", "additionalProperties": false}
	if len(properties) > 0 {
		s["properties"] = properties
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func property(kind, description string) schema {
	return schema{"type": kind, "description": description}
}

const (
	choreIDHelp = "Numeric id of the chore, as returned by list_chores."
	thingIDHelp = "Numeric id of the thing, as returned by list_things."
	dueDateHelp = "Due date as an RFC 3339 timestamp (2026-01-31T18:00:00Z) or a plain date (2026-01-31)."
)

// tools is the list served by tools/list, in a fixed order so clients can
// cache it. Every tool maps to exactly one external API (eAPI) route.
var tools = []tool{
	{
		Name:        "list_chores",
		Title:       "List chores",
		Description: "List the active chores (tasks) in the user's circle, with their due dates, assignees, recurrence and labels.",
		InputSchema: object(schema{
			"includeSubtasks": property("boolean", "Also return each chore's subtasks. Defaults to false."),
		}),
		Annotations: readOnly,
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				IncludeSubtasks bool `json:"includeSubtasks"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			call := apiCall{method: http.MethodGet, path: "/eapi/v1/chore"}
			if args.IncludeSubtasks {
				call.query = url.Values{"includeSubtasks": {"true"}}
			}
			return call, nil
		},
	},
	{
		Name:        "create_chore",
		Title:       "Create chore",
		Description: "Create a one-time chore assigned to its creator. Returns the created chore.",
		InputSchema: object(schema{
			"name":        property("string", "Short name of the chore."),
			"description": property("string", "Longer description or notes."),
			"dueDate":     property("string", dueDateHelp),
			"createdBy":   property("integer", "User id of the circle member to create the chore for, as returned by list_circle_members. Defaults to the token's owner."),
			"projectId":   property("integer", "Id of the project to add the chore to."),
		}, "name"),
		Annotations: additive,
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				Name        string  `json:"name"`
				Description *string `json:"description,omitempty"`
				DueDate     string  `json:"dueDate,omitempty"`
				CreatedBy   *int    `json:"createdBy,omitempty"`
				ProjectID   *int    `json:"projectId,omitempty"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if strings.TrimSpace(args.Name) == "" {
				return apiCall{}, errors.New("name is required")
			}
			return apiCall{method: http.MethodPost, path: "/eapi/v1/chore", body: args}, nil
		},
	},
	{
		Name:        "update_chore",
		Title:       "Update chore",
		Description: "Change the name, description or due date of a chore. Fields that are left out keep their current value. Only the chore's creator or a circle admin or manager may update it.",
		InputSchema: object(schema{
			"id":             property("integer", choreIDHelp),
			"name":           property("string", "New name."),
			"description":    property("string", "New description."),
			"dueDate":        property("string", dueDateHelp),
			"forceUnarchive": property("boolean", "Make an archived chore active again."),
		}, "id"),
		Annotations: overwriting,
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				ID             *int    `json:"id,omitempty"`
				Name           string  `json:"name,omitempty"`
				Description    *string `json:"description,omitempty"`
				DueDate        string  `json:"dueDate,omitempty"`
				ForceUnarchive bool    `json:"forceUnarchive,omitempty"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			if args.Name == "" && args.Description == nil && args.DueDate == "" && !args.ForceUnarchive {
				return apiCall{}, errors.New("nothing to update: give at least one of name, description, dueDate or forceUnarchive")
			}
			path := fmt.Sprintf("/eapi/v1/chore/%d", *args.ID)
			args.ID = nil
			return apiCall{method: http.MethodPut, path: path, body: args}, nil
		},
	},
	{
		Name:        "complete_chore",
		Title:       "Complete chore",
		Description: "Mark a chore as done. A recurring chore is rescheduled to its next due date and handed to its next assignee. Returns the updated chore.",
		InputSchema: object(schema{
			"id":          property("integer", choreIDHelp),
			"completedBy": property("integer", "User id of the circle member who did the chore, as returned by list_circle_members. Defaults to the token's owner."),
		}, "id"),
		Annotations: toolAnnotations{DestructiveHint: true},
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				ID          *int `json:"id"`
				CompletedBy *int `json:"completedBy"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			call := apiCall{method: http.MethodPost, path: fmt.Sprintf("/eapi/v1/chore/%d/complete", *args.ID)}
			if args.CompletedBy != nil {
				call.query = url.Values{"completedBy": {strconv.Itoa(*args.CompletedBy)}}
			}
			return call, nil
		},
	},
	{
		Name:        "delete_chore",
		Title:       "Delete chore",
		Description: "Permanently delete a chore. Only the chore's creator or a circle admin or manager may delete it.",
		InputSchema: object(schema{"id": property("integer", choreIDHelp)}, "id"),
		Annotations: overwriting,
		build: func(arguments json.RawMessage) (apiCall, error) {
			id, err := decodeID(arguments)
			if err != nil {
				return apiCall{}, err
			}
			return apiCall{method: http.MethodDelete, path: fmt.Sprintf("/eapi/v1/chore/%d", id)}, nil
		},
	},
	{
		Name:        "list_circle_members",
		Title:       "List circle members",
		Description: "List the members of the user's circle (household or team) with their user ids, names, roles and points.",
		InputSchema: object(nil),
		Annotations: readOnly,
		build: func(arguments json.RawMessage) (apiCall, error) {
			if err := decodeArguments(arguments, &struct{}{}); err != nil {
				return apiCall{}, err
			}
			return apiCall{method: http.MethodGet, path: "/eapi/v1/circle/members"}, nil
		},
	},
	{
		Name:        "list_things",
		Title:       "List things",
		Description: "List the user's things. A thing is a named value (a number, a boolean or text) that can complete or reschedule chores when it changes.",
		InputSchema: object(nil),
		Annotations: readOnly,
		build: func(arguments json.RawMessage) (apiCall, error) {
			if err := decodeArguments(arguments, &struct{}{}); err != nil {
				return apiCall{}, err
			}
			return apiCall{method: http.MethodGet, path: "/eapi/v1/things/"}, nil
		},
	},
	{
		Name:        "get_thing",
		Title:       "Get thing",
		Description: "Get one thing, including its current state and the chores linked to it.",
		InputSchema: object(schema{"id": property("integer", thingIDHelp)}, "id"),
		Annotations: readOnly,
		build: func(arguments json.RawMessage) (apiCall, error) {
			id, err := decodeID(arguments)
			if err != nil {
				return apiCall{}, err
			}
			return apiCall{method: http.MethodGet, path: fmt.Sprintf("/eapi/v1/things/%d", id)}, nil
		},
	},
	{
		Name:        "set_thing_state",
		Title:       "Set thing state",
		Description: "Set the state of a thing. Chores linked to the thing are completed or rescheduled if the new state matches their trigger.",
		InputSchema: object(schema{
			"id": property("integer", thingIDHelp),
			"state": schema{
				"type":        []string{"string", "number", "boolean"},
				"description": "New state: a whole number for a number thing, true or false for a boolean thing, any text for a text thing.",
			},
		}, "id", "state"),
		Annotations: overwriting,
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				ID    *int `json:"id"`
				State any  `json:"state"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			state, err := scalarString(args.State)
			if err != nil {
				return apiCall{}, fmt.Errorf("state %w", err)
			}
			return apiCall{
				method: http.MethodGet,
				path:   fmt.Sprintf("/eapi/v1/things/%d/state", *args.ID),
				query:  url.Values{"state": {state}},
			}, nil
		},
	},
	{
		Name:        "increment_thing",
		Title:       "Increment thing",
		Description: "Add an amount to a number thing and return its new state. Chores linked to the thing are completed or rescheduled if the new state matches their trigger.",
		InputSchema: object(schema{
			"id":     property("integer", thingIDHelp),
			"amount": property("integer", "Amount to add; negative to subtract. Defaults to 1."),
		}, "id"),
		Annotations: toolAnnotations{DestructiveHint: true},
		build: func(arguments json.RawMessage) (apiCall, error) {
			var args struct {
				ID     *int `json:"id"`
				Amount *int `json:"amount"`
			}
			if err := decodeArguments(arguments, &args); err != nil {
				return apiCall{}, err
			}
			if args.ID == nil {
				return apiCall{}, errors.New("id is required")
			}
			amount := 1
			if args.Amount != nil {
				amount = *args.Amount
			}
			return apiCall{
				method: http.MethodGet,
				path:   fmt.Sprintf("/eapi/v1/things/%d/state/change", *args.ID),
				query:  url.Values{"op": {strconv.Itoa(amount)}},
			}, nil
		},
	},
}

func findTool(name string) (tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return tool{}, false
}

// decodeArguments reads tool arguments strictly: a misspelled or made-up
// argument is reported back instead of being silently ignored.
func decodeArguments(arguments json.RawMessage, into any) error {
	if len(arguments) == 0 || string(arguments) == "null" {
		arguments = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	err := decoder.Decode(into)
	var wrongType *json.UnmarshalTypeError
	switch {
	case err == nil:
		return nil
	case !errors.As(err, &wrongType):
		return errors.New(strings.TrimPrefix(err.Error(), "json: "))
	case wrongType.Field == "":
		return errors.New("arguments must be an object")
	}
	want := "a " + wrongType.Type.Kind().String()
	switch wrongType.Type.Kind() {
	case reflect.Int:
		want = "a whole number"
	case reflect.Bool:
		want = "true or false"
	}
	return fmt.Errorf("%s must be %s, not a %s", wrongType.Field, want, wrongType.Value)
}

func decodeID(arguments json.RawMessage) (int, error) {
	var args struct {
		ID *int `json:"id"`
	}
	if err := decodeArguments(arguments, &args); err != nil {
		return 0, err
	}
	if args.ID == nil {
		return 0, errors.New("id is required")
	}
	return *args.ID, nil
}

// scalarString renders a JSON string, number or boolean the way the things
// API expects to receive a state.
func scalarString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", errors.New("must not be empty")
		}
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", errors.New("is required")
	}
	return "", errors.New("must be a string, a number or a boolean")
}
