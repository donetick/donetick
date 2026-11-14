package mcp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

// Protocol revisions the endpoint serves, newest first.
//
// Revision 2026-07-28 made MCP stateless: every request names its protocol
// version in _meta and is answered on its own. Earlier revisions agree on a
// version once, in an initialize handshake. The specification calls the two
// styles "modern" and "legacy" and lets one endpoint answer both, which is
// what this package does. Nothing is remembered between requests either way.
const modernVersion = "2026-07-28"

var (
	legacyVersions    = []string{"2025-11-25", "2025-06-18", "2025-03-26"}
	supportedVersions = append([]string{modernVersion}, legacyVersions...)
)

const (
	headerProtocolVersion = "MCP-Protocol-Version"
	headerMethod          = "Mcp-Method"
	headerName            = "Mcp-Name"

	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// JSON-RPC error codes: the standard ones, plus those MCP defines.
const (
	codeParseError                 = -32700
	codeInvalidRequest             = -32600
	codeMethodNotFound             = -32601
	codeInvalidParams              = -32602
	codeInternalError              = -32603
	codeHeaderMismatch             = -32020
	codeUnsupportedProtocolVersion = -32022
)

// response is a JSON-RPC response. The id is left out when the request could
// not be read far enough to learn it.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// failure is a JSON-RPC error together with the HTTP status that carries it.
type failure struct {
	status  int
	code    int
	message string
	data    any
}

func badRequest(code int, message string) *failure {
	return &failure{status: http.StatusBadRequest, code: code, message: message}
}

func unsupportedVersion(requested string) *failure {
	return &failure{
		status:  http.StatusBadRequest,
		code:    codeUnsupportedProtocolVersion,
		message: "Unsupported protocol version",
		data:    map[string]any{"supported": supportedVersions, "requested": requested},
	}
}

// message is a JSON-RPC request or notification sent by a client.
type message struct {
	id     json.RawMessage // empty for a notification
	method string
	params map[string]json.RawMessage
	meta   map[string]json.RawMessage
}

func (m *message) isNotification() bool {
	return len(m.id) == 0
}

// decodeMessage reads one JSON-RPC message. Members are looked up by exact
// name: encoding/json would also accept "Method" for "method", and a proxy
// inspecting the body must not be able to see a different request than the
// one that runs.
func decodeMessage(body []byte) (*message, *failure) {
	if !json.Valid(body) {
		return nil, badRequest(codeParseError, "Parse error: the request body is not valid JSON")
	}
	// Batches are turned down. Only revision 2025-03-26 had them, the next
	// revision took them out again, and one request standing for many would
	// multiply what a client gets out of the rate limit.
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil || members == nil {
		return nil, badRequest(codeInvalidRequest, "Invalid request: expected a single JSON-RPC message object (batches are not supported)")
	}

	msg := &message{}
	if id, ok := members["id"]; ok {
		if !isRequestID(id) {
			return nil, badRequest(codeInvalidRequest, "Invalid request: id must be a string or a number")
		}
		msg.id = id
	}
	if string(members["jsonrpc"]) != `"2.0"` {
		return msg, badRequest(codeInvalidRequest, `Invalid request: jsonrpc must be "2.0"`)
	}
	if err := json.Unmarshal(members["method"], &msg.method); err != nil || msg.method == "" {
		return msg, badRequest(codeInvalidRequest, "Invalid request: method is required")
	}
	if params, ok := members["params"]; ok && string(params) != "null" {
		if err := json.Unmarshal(params, &msg.params); err != nil {
			return msg, badRequest(codeInvalidRequest, "Invalid request: params must be an object")
		}
	}
	if meta, ok := msg.params["_meta"]; ok {
		if err := json.Unmarshal(meta, &msg.meta); err != nil || msg.meta == nil {
			return msg, badRequest(codeInvalidParams, "Invalid params: _meta must be an object")
		}
	}
	return msg, nil
}

func isRequestID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return raw[0] == '"' || raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')
}

// negotiate works out which style of the protocol a request uses, and for
// modern requests checks that the headers mirroring the body agree with it.
func negotiate(header http.Header, msg *message) (modern bool, fail *failure) {
	fromHeader := header.Get(headerProtocolVersion)

	if msg.method == "initialize" {
		// The handshake carries the client's version in its params.
		return false, nil
	}

	rawVersion, declared := msg.meta[metaProtocolVersion]
	if !declared {
		switch {
		case fromHeader == "", slices.Contains(legacyVersions, fromHeader):
			// Clients older than 2025-06-18 do not send the header at all.
			return false, nil
		case fromHeader == modernVersion:
			return false, badRequest(codeInvalidParams, "Invalid params: _meta."+metaProtocolVersion+" is required")
		default:
			return false, unsupportedVersion(fromHeader)
		}
	}

	var version string
	if err := json.Unmarshal(rawVersion, &version); err != nil {
		return false, badRequest(codeInvalidParams, "Invalid params: _meta."+metaProtocolVersion+" must be a string")
	}
	if fromHeader == "" {
		return false, badRequest(codeHeaderMismatch, "Header mismatch: the "+headerProtocolVersion+" header is required")
	}
	if fromHeader != version {
		return false, badRequest(codeHeaderMismatch, "Header mismatch: "+headerProtocolVersion+" header value '"+fromHeader+"' does not match body value '"+version+"'")
	}
	if version != modernVersion {
		return false, unsupportedVersion(version)
	}
	if _, ok := msg.meta[metaClientCapabilities]; !ok {
		return false, badRequest(codeInvalidParams, "Invalid params: _meta."+metaClientCapabilities+" is required")
	}
	if fail := checkMirroredHeader(header, headerMethod, msg.method, false); fail != nil {
		return false, fail
	}
	if msg.method == "tools/call" {
		var name string
		_ = json.Unmarshal(msg.params["name"], &name)
		if fail := checkMirroredHeader(header, headerName, name, true); fail != nil {
			return false, fail
		}
	}
	return true, nil
}

// checkMirroredHeader compares a header with the part of the body it mirrors.
// Names of tools may arrive Base64-wrapped; methods may not, so that a proxy
// reading the header sees the method as it is.
func checkMirroredHeader(header http.Header, name, bodyValue string, mayBeEncoded bool) *failure {
	value, ok := header.Get(name), true
	if mayBeEncoded {
		value, ok = decodeHeaderValue(value)
	}
	switch {
	case header.Get(name) == "":
		return badRequest(codeHeaderMismatch, "Header mismatch: the "+name+" header is required")
	case !ok:
		return badRequest(codeHeaderMismatch, "Header mismatch: the "+name+" header is not valid Base64")
	case value != bodyValue:
		return badRequest(codeHeaderMismatch, "Header mismatch: "+name+" header value '"+value+"' does not match body value '"+bodyValue+"'")
	}
	return nil
}

// decodeHeaderValue undoes the "=?base64?...?=" wrapping clients apply to
// header values that are not plain ASCII.
func decodeHeaderValue(value string) (string, bool) {
	const prefix, suffix = "=?base64?", "?="
	if len(value) < len(prefix)+len(suffix) || !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return value, true
	}
	decoded, err := base64.StdEncoding.DecodeString(value[len(prefix) : len(value)-len(suffix)])
	if err != nil {
		return "", false
	}
	return string(decoded), true
}
