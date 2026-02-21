package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MessageValidator handles validation of WebSocket messages
type MessageValidator struct {
	maxMessageSize  int
	maxFieldLength  int
	allowedTypes    map[string]bool
	sanitizedLogger *SanitizedLogger
}

// WebSocketMessage represents the structure of WebSocket messages
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ValidationResult contains validation results and sanitized message
type ValidationResult struct {
	Valid            bool
	SanitizedMessage *WebSocketMessage
	Errors           []string
}

// NewMessageValidator creates a new message validator
func NewMessageValidator(logger *SanitizedLogger) *MessageValidator {
	return &MessageValidator{
		maxMessageSize:  8192, // 8KB max message size
		maxFieldLength:  1024, // 1KB max field length
		sanitizedLogger: logger,
		allowedTypes: map[string]bool{
			"ping":            true,
			"pong":            true,
			"chore_updated":   true,
			"chore_completed": true,
			"chore_skipped":   true,
			"user_joined":     true,
			"user_left":       true,
			"circle_updated":  true,
			"notification":    true,
			"heartbeat":       true,
		},
	}
}

// ValidateMessage validates incoming WebSocket messages
func (mv *MessageValidator) ValidateMessage(rawMessage []byte, userID int, connectionID string) *ValidationResult {
	result := &ValidationResult{
		Valid:  false,
		Errors: make([]string, 0),
	}

	// Check message size
	if len(rawMessage) > mv.maxMessageSize {
		result.Errors = append(result.Errors, fmt.Sprintf("message size %d exceeds limit %d", len(rawMessage), mv.maxMessageSize))
		mv.sanitizedLogger.LogMessageValidation(userID, connectionID, "message_too_large", "unknown")
		return result
	}

	// Check if message is valid UTF-8
	if !utf8.Valid(rawMessage) {
		result.Errors = append(result.Errors, "message contains invalid UTF-8 characters")
		mv.sanitizedLogger.LogMessageValidation(userID, connectionID, "invalid_utf8", "unknown")
		return result
	}

	// Parse JSON
	var message WebSocketMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid JSON: %v", err))
		mv.sanitizedLogger.LogMessageValidation(userID, connectionID, "invalid_json", "unknown")
		return result
	}

	// Validate message structure
	if validationErrors := mv.validateMessageStructure(&message); len(validationErrors) > 0 {
		result.Errors = append(result.Errors, validationErrors...)
		mv.sanitizedLogger.LogMessageValidation(userID, connectionID, "invalid_structure", message.Type)
		return result
	}

	// Sanitize message content
	sanitized := mv.sanitizeMessage(&message)
	result.SanitizedMessage = sanitized
	result.Valid = true

	return result
}

// validateMessageStructure validates the basic structure of a WebSocket message
func (mv *MessageValidator) validateMessageStructure(message *WebSocketMessage) []string {
	var errors []string

	// Validate message type
	if message.Type == "" {
		errors = append(errors, "message type is required")
	} else if !mv.allowedTypes[message.Type] {
		errors = append(errors, fmt.Sprintf("message type '%s' is not allowed", message.Type))
	}

	// Validate message type length
	if len(message.Type) > 50 {
		errors = append(errors, "message type is too long")
	}

	// Validate timestamp format if provided
	if message.Timestamp != "" {
		if _, err := time.Parse(time.RFC3339, message.Timestamp); err != nil {
			errors = append(errors, "invalid timestamp format, use RFC3339")
		}
	}

	// Validate data fields
	if message.Data != nil {
		if dataErrors := mv.validateDataFields(message.Data, ""); len(dataErrors) > 0 {
			errors = append(errors, dataErrors...)
		}
	}

	return errors
}

// validateDataFields recursively validates data fields
func (mv *MessageValidator) validateDataFields(data map[string]interface{}, prefix string) []string {
	var errors []string

	for key, value := range data {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		// Validate key length and content
		if len(key) > 100 {
			errors = append(errors, fmt.Sprintf("field key '%s' is too long", fieldPath))
			continue
		}

		if !mv.isValidFieldName(key) {
			errors = append(errors, fmt.Sprintf("field key '%s' contains invalid characters", fieldPath))
			continue
		}

		// Validate value based on type
		switch v := value.(type) {
		case string:
			if len(v) > mv.maxFieldLength {
				errors = append(errors, fmt.Sprintf("field '%s' value is too long", fieldPath))
			}
			if !utf8.ValidString(v) {
				errors = append(errors, fmt.Sprintf("field '%s' contains invalid UTF-8", fieldPath))
			}
		case map[string]interface{}:
			// Recursively validate nested objects (limit depth)
			if strings.Count(fieldPath, ".") > 3 {
				errors = append(errors, fmt.Sprintf("field '%s' is nested too deeply", fieldPath))
			} else {
				if nestedErrors := mv.validateDataFields(v, fieldPath); len(nestedErrors) > 0 {
					errors = append(errors, nestedErrors...)
				}
			}
		case []interface{}:
			if len(v) > 100 {
				errors = append(errors, fmt.Sprintf("field '%s' array is too large", fieldPath))
			}
			// Validate array elements
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
					if itemErrors := mv.validateDataFields(itemMap, itemPath); len(itemErrors) > 0 {
						errors = append(errors, itemErrors...)
					}
				}
			}
		case float64:
			// Numbers are generally safe, but check for reasonable ranges
			if v > 1e15 || v < -1e15 {
				errors = append(errors, fmt.Sprintf("field '%s' number is out of reasonable range", fieldPath))
			}
		}
	}

	return errors
}

// isValidFieldName checks if a field name contains only safe characters
func (mv *MessageValidator) isValidFieldName(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// sanitizeMessage sanitizes message content to prevent XSS and other attacks
func (mv *MessageValidator) sanitizeMessage(message *WebSocketMessage) *WebSocketMessage {
	sanitized := &WebSocketMessage{
		Type:      mv.sanitizeString(message.Type),
		Timestamp: message.Timestamp, // Timestamp already validated
		Data:      make(map[string]interface{}),
	}

	if message.Data != nil {
		sanitized.Data = mv.sanitizeDataFields(message.Data)
	}

	return sanitized
}

// sanitizeDataFields recursively sanitizes data fields
func (mv *MessageValidator) sanitizeDataFields(data map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range data {
		sanitizedKey := mv.sanitizeString(key)

		switch v := value.(type) {
		case string:
			sanitized[sanitizedKey] = mv.sanitizeString(v)
		case map[string]interface{}:
			sanitized[sanitizedKey] = mv.sanitizeDataFields(v)
		case []interface{}:
			sanitized[sanitizedKey] = mv.sanitizeArray(v)
		default:
			sanitized[sanitizedKey] = value
		}
	}

	return sanitized
}

// sanitizeArray sanitizes array elements
func (mv *MessageValidator) sanitizeArray(arr []interface{}) []interface{} {
	sanitized := make([]interface{}, len(arr))

	for i, item := range arr {
		switch v := item.(type) {
		case string:
			sanitized[i] = mv.sanitizeString(v)
		case map[string]interface{}:
			sanitized[i] = mv.sanitizeDataFields(v)
		default:
			sanitized[i] = item
		}
	}

	return sanitized
}

// sanitizeString removes potentially dangerous characters from strings
func (mv *MessageValidator) sanitizeString(s string) string {
	// Remove null bytes and control characters except tab, newline, carriage return
	var result strings.Builder
	for _, r := range s {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			continue
		}
		result.WriteRune(r)
	}

	cleaned := result.String()

	// Basic XSS prevention - remove script tags and event handlers
	cleaned = strings.ReplaceAll(cleaned, "<script", "&lt;script")
	cleaned = strings.ReplaceAll(cleaned, "</script>", "&lt;/script&gt;")
	cleaned = strings.ReplaceAll(cleaned, "javascript:", "")
	cleaned = strings.ReplaceAll(cleaned, "vbscript:", "")

	return cleaned
}

// ValidateMessageType checks if a message type is allowed
func (mv *MessageValidator) ValidateMessageType(messageType string) error {
	if messageType == "" {
		return errors.New("message type is required")
	}

	if !mv.allowedTypes[messageType] {
		return fmt.Errorf("message type '%s' is not allowed", messageType)
	}

	return nil
}

// AddAllowedMessageType adds a new allowed message type
func (mv *MessageValidator) AddAllowedMessageType(messageType string) {
	mv.allowedTypes[messageType] = true
}

// RemoveAllowedMessageType removes an allowed message type
func (mv *MessageValidator) RemoveAllowedMessageType(messageType string) {
	delete(mv.allowedTypes, messageType)
}

// GetAllowedMessageTypes returns a list of all allowed message types
func (mv *MessageValidator) GetAllowedMessageTypes() []string {
	types := make([]string, 0, len(mv.allowedTypes))
	for t := range mv.allowedTypes {
		types = append(types, t)
	}
	return types
}
