package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"reflect"
	"slices"
	"strings"
)

// tool describes one MCP tool and how a call to it is carried out through
// Donetick's HTTP API.
type tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	InputSchema schema          `json:"inputSchema"`
	Annotations toolAnnotations `json:"annotations"`

	// run carries out a call and returns the API response to show the model.
	// A plain error means the arguments were wrong; its text is shown to the
	// model too, so it should say how to fix the call.
	run runner
}

type runner func(s *session, arguments json.RawMessage) (apiResponse, error)

// tools is the list served by tools/list, in a fixed order so clients can
// cache it.
var tools = slices.Concat(choreTools, labelTools, projectTools, filterTools, thingTools)

func findTool(name string) (tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return tool{}, false
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
	oneWay      = toolAnnotations{DestructiveHint: true}
)

// session is what a tool has to work with while it runs: the owner of the API
// token, and a way to send requests to the API as that user.
type session struct {
	userID int
	call   func(apiCall) (apiResponse, error)
}

// apiCall is a request to one of Donetick's HTTP API routes.
type apiCall struct {
	method string
	path   string
	query  url.Values
	body   any // sent as JSON when not nil
}

type apiResponse struct {
	status int
	body   []byte
}

func (r apiResponse) ok() bool {
	return r.status >= 200 && r.status < 300
}

// refusal ends a tool call with an error response from the API, for tools
// that need more than one request and are turned down part-way.
type refusal struct {
	response apiResponse
}

func (r *refusal) Error() string {
	return fmt.Sprintf("the API answered with HTTP %d", r.response.status)
}

// notFound says that a record a tool was asked to change is not among the
// ones the caller can see.
type notFound struct {
	kind string
	id   int
}

func (e *notFound) Error() string {
	return fmt.Sprintf("there is no %s with id %d", e.kind, e.id)
}

// internalError marks a failure that is the server's fault, not the caller's.
// It is logged and reported without details.
type internalError struct {
	err error
}

func (e *internalError) Error() string { return e.err.Error() }
func (e *internalError) Unwrap() error { return e.err }

// object is a JSON object whose members are kept exactly as they were
// received, so it can be sent back without losing the ones this package does
// not know about.
type object = map[string]json.RawMessage

func raw(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err) // only called with values that always encode
	}
	return encoded
}

// request makes a tool out of a single API request.
func request(build func(arguments json.RawMessage) (apiCall, error)) runner {
	return func(s *session, arguments json.RawMessage) (apiResponse, error) {
		call, err := build(arguments)
		if err != nil {
			return apiResponse{}, err
		}
		return s.call(call)
	}
}

// fixed makes a tool that takes no arguments and always sends the same request.
func fixed(method, path string) runner {
	return request(func(arguments json.RawMessage) (apiCall, error) {
		_, err := decodeArguments(arguments, &struct{}{})
		return apiCall{method: method, path: path}, err
	})
}

// byID makes a tool whose only argument is the id in the route.
func byID(method, pathFormat string) runner {
	return request(func(arguments json.RawMessage) (apiCall, error) {
		var args struct {
			ID *int `json:"id"`
		}
		if _, err := decodeArguments(arguments, &args); err != nil {
			return apiCall{}, err
		}
		if args.ID == nil {
			return apiCall{}, errors.New("id is required")
		}
		return apiCall{method: method, path: fmt.Sprintf(pathFormat, *args.ID)}, nil
	})
}

// fetch gets the object a route returns, unwrapping the "res" envelope most
// routes use. Updates start from it: the API replaces records whole, so a
// tool that changes one field has to send the others back as they are.
func (s *session) fetch(path string) (object, error) {
	found, err := s.call(apiCall{method: "GET", path: path})
	if err != nil {
		return nil, err
	}
	if !found.ok() {
		return nil, &refusal{found}
	}
	var record object
	if err := json.Unmarshal(unwrap(found.body), &record); err != nil || record == nil {
		return nil, &internalError{fmt.Errorf("GET %s did not return an object", path)}
	}
	return record, nil
}

// find gets one record out of a route that only lists them.
func (s *session) find(path, kind string, id int) (object, error) {
	found, err := s.call(apiCall{method: "GET", path: path})
	if err != nil {
		return nil, err
	}
	if !found.ok() {
		return nil, &refusal{found}
	}
	var records []object
	if err := json.Unmarshal(unwrap(found.body), &records); err != nil {
		return nil, &internalError{fmt.Errorf("GET %s did not return a list", path)}
	}
	for _, record := range records {
		var recordID int
		if json.Unmarshal(record["id"], &recordID) == nil && recordID == id {
			return record, nil
		}
	}
	return nil, &notFound{kind, id}
}

// unwrap returns the value of "res" when body is an object that has one, and
// body itself otherwise.
func unwrap(body []byte) []byte {
	var envelope struct {
		Res json.RawMessage `json:"res"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Res != nil {
		return envelope.Res
	}
	return body
}

// pick copies the named members of a record that are present.
func pick(record object, names ...string) object {
	picked := object{}
	for _, name := range names {
		if value, ok := record[name]; ok {
			picked[name] = value
		}
	}
	return picked
}

// overlay writes the arguments the caller sent over a record, leaving the
// rest of it alone. A null argument is written too: that is how a field is
// cleared.
func overlay(record, given object, names ...string) {
	for _, name := range names {
		if value, ok := given[name]; ok {
			record[name] = value
		}
	}
}

// decodeArguments reads tool arguments strictly: a misspelled or made-up
// argument is reported back instead of being silently ignored. It also
// returns the arguments as they were sent, which tells an argument that was
// left out from one that was set to null.
func decodeArguments(arguments json.RawMessage, into any) (object, error) {
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
		given := object{}
		_ = json.Unmarshal(arguments, &given)
		return given, checkNames(arguments, reflect.TypeOf(into))
	case !errors.As(err, &wrongType):
		return nil, errors.New(strings.TrimPrefix(err.Error(), "json: "))
	case wrongType.Field == "":
		return nil, errors.New("arguments must be an object")
	}
	return nil, fmt.Errorf("%s must be %s, not a %s", wrongType.Field, describeKind(wrongType.Type), wrongType.Value)
}

// checkNames turns down members whose name matches an argument in everything
// but case. encoding/json lets "Name" stand for "name", but the tools copy
// arguments by their exact name, so such a member would be accepted and then
// quietly left out.
func checkNames(value json.RawMessage, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice:
		var elements []json.RawMessage
		_ = json.Unmarshal(value, &elements)
		for _, element := range elements {
			if err := checkNames(element, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		var members object
		_ = json.Unmarshal(value, &members)
		fields := map[string]reflect.Type{}
		for i := range t.NumField() {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			fields[name] = t.Field(i).Type
		}
		for _, name := range slices.Sorted(maps.Keys(members)) {
			field, known := fields[name]
			if !known {
				return fmt.Errorf("unknown field %q", name)
			}
			if err := checkNames(members[name], field); err != nil {
				return err
			}
		}
	}
	return nil
}

func describeKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int:
		return "a whole number"
	case reflect.Bool:
		return "true or false"
	case reflect.Slice:
		return "a list"
	case reflect.Struct, reflect.Map:
		return "an object"
	}
	return "a " + t.Kind().String()
}

// schema is a JSON Schema, written as the map it serializes to.
type schema map[string]any

func inputs(properties schema, required ...string) schema {
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

// nullable is a property that can also be set to null to clear it.
func nullable(kind, description string) schema {
	return schema{"type": []string{kind, "null"}, "description": description}
}

func oneOf(description string, values ...string) schema {
	return schema{"type": "string", "enum": values, "description": description}
}

func listOf(kind, description string) schema {
	return schema{"type": "array", "items": schema{"type": kind}, "description": description}
}

// with returns a copy of properties with more added, so tools can share a set
// of properties without sharing the map.
func (s schema) with(more schema) schema {
	merged := schema{}
	for name, value := range s {
		merged[name] = value
	}
	for name, value := range more {
		merged[name] = value
	}
	return merged
}
