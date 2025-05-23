// Package wrapperrors provides primitives for creating and manipulating errors
// that are compatible with standard Go error handling (errors.Is, errors.As, errors.Unwrap)
// and provide detailed, structured information (codes, messages, statuses, cause)
// for logging and programmatic access.
package wrapperrors

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var (
	// InternalError is a predefined error for internal server errors.
	InternalError = Define("internal_error", http.StatusInternalServerError)
	// UnknownError is a predefined error for unknown errors.
	UnknownError = Define("unknown_error", http.StatusInternalServerError)
)

// ErrorWrapper defines the interface for custom errors that support
// detailed structured information and standard Go error handling.
type ErrorWrapper interface {
	error         // Embed standard error interface for basic error message.
	fmt.Formatter // For custom formatting with fmt.Printf (e.g., "%+v" for details).

	// String returns a detailed, JSON-like string representation of the error,
	// including codes, messages, statuses, and a recursively formatted cause.
	// Suitable for detailed logging.
	String() string

	// Json returns a map[string]interface{} representation of the error,
	// allowing structured access to codes, messages, statuses, and cause.
	// The cause is also represented as a map if it's an ErrorWrapper.
	Json() map[string]interface{}

	// WithMessage appends a new message to the error's message list.
	// If the error already has a cause, the error is re-wrapped using `fmt.Errorf` with "%w"
	// and the existing cause, thus maintaining the error chain. The error's own codes and
	// newly augmented messages form the prefix of the error string.
	WithMessage(message string) ErrorWrapper

	// WithStatus appends a new status (code and message) to the error's status list.
	// If the provided status code is 0, this method is a no-op and returns the error unchanged.
	WithStatus(status int) ErrorWrapper

	// WithCause sets or replaces the underlying cause of the error.
	// It uses `fmt.Errorf` with the "%w" verb to wrap the given `err`, making it
	// available to `errors.Unwrap`. The current error's codes and messages (its primary message)
	// form the prefix of the resulting error message.
	WithCause(err error) ErrorWrapper

	// FromDefinition creates a new error instance based on the current error's
	// definition (codes, messages, statuses are copied), but with a new cause.
	// This is useful for creating specific instances from predefined error types.
	FromDefinition(cause error) ErrorWrapper

	// Is checks if the current error instance is equivalent to the target error.
	// This method is called by the standard `errors.Is` function.
	// If `target` is an `ErrorWrapper`, this method primarily compares their codes.
	// This design allows `errors.Is(err, PredefinedErrorDefinition)` to work as expected,
	// determining if `err` (or an error it wraps) is of the type defined by `PredefinedErrorDefinition`
	// based on matching codes.
	Is(target error) bool

	// Unwrap returns the underlying error that was wrapped by this error instance
	// (e.g., via `WithCause`). This makes the error compatible with `errors.Unwrap`
	// and thus with the standard `errors.Is` and `errors.As` functions.
	Unwrap() error

	// GetCodes returns a slice of codes associated with this error.
	GetCodes() []string
	// GetMessages returns a slice of messages associated with this error.
	GetMessages() []string
	// GetStatuses returns a slice of statusCodes (message and code) associated with this error.
	GetStatuses() []statusCode
}

// wrapper is the concrete implementation of ErrorWrapper.
type wrapper struct {
	codes    []string
	messages []string
	statuses []statusCode
	cause    error // Stores the wrapped error, created by fmt.Errorf with %w or errors.New.
	*sync.RWMutex
}

// statusCode holds a message and an integer code for an error status.
type statusCode struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// buildPrimaryMessage constructs the main message string from codes and messages for the current layer.
// This message is used when creating a new error or re-wrapping an existing one.
func (e *wrapper) buildPrimaryMessage() string {
	if e == nil {
		return ""
	}
	e.RLock()
	defer e.RUnlock()
	return e.buildPrimaryMessageWithoutLock()
}

// buildPrimaryMessageWithoutLock is an internal helper that doesn't acquire locks.
// Assumes lock is already held by the caller.
func (e *wrapper) buildPrimaryMessageWithoutLock() string {
	msgParts := []string{}
	if len(e.codes) > 0 {
		msgParts = append(msgParts, "codes:"+strings.Join(e.codes, ","))
	}
	if len(e.messages) > 0 {
		msgParts = append(msgParts, "messages:"+strings.Join(e.messages, ","))
	}
	if len(msgParts) == 0 {
		return "error" // Fallback, should not be reached if Define is used.
	}
	return strings.Join(msgParts, " ")
}

// Error returns the error message. If a cause is wrapped (via WithCause),
// the message will include the wrapper's primary message and the cause's message,
// as per fmt.Errorf's "%w" verb behavior.
// If no cause is set (e.g., after Define() and before WithCause()),
// it returns the primary message of this error layer.
func (e *wrapper) Error() string {
	if e == nil {
		return ""
	}
	e.RLock()
	defer e.RUnlock()

	if e.cause != nil {
		return e.cause.Error()
	}
	return e.buildPrimaryMessageWithoutLock()
}

// String returns a detailed, JSON-like string representation of the error.
// This is suitable for logging and includes codes, messages, statuses, and
// a recursively formatted cause.
func (e *wrapper) String() string {
	if e == nil {
		return ""
	}
	e.RLock()
	defer e.RUnlock()

	var sb strings.Builder
	e.formatDetailed(&sb)
	return sb.String()
}

// formatDetailed is a helper for String() and Format's "%+v" case.
// It constructs the detailed string representation. Assumes lock is held by the caller.
func (e *wrapper) formatDetailed(sb *strings.Builder) {
	sb.WriteString("{")
	parts := make([]string, 0, 4)
	if len(e.codes) > 0 {
		parts = append(parts, fmt.Sprintf("\"code\": %s", e.codeString()))
	}
	if len(e.messages) > 0 {
		parts = append(parts, fmt.Sprintf("\"message\": %s", e.messageString()))
	}
	if len(e.statuses) > 0 {
		parts = append(parts, fmt.Sprintf("\"status\": %s", e.statusString()))
	}

	originalCause := errors.Unwrap(e.cause)
	if originalCause != nil {
		causeStr := ""
		if causeWrapper, ok := originalCause.(ErrorWrapper); ok {
			causeStr = causeWrapper.String()
		} else {
			escapedErrStr := strings.ReplaceAll(originalCause.Error(), "\"", "\\\"")
			causeStr = fmt.Sprintf("\"%s\"", escapedErrStr)
		}
		parts = append(parts, fmt.Sprintf("\"cause\": %s", causeStr))
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString("}")
}

// Format implements fmt.Formatter, allowing custom formatting with package fmt.
// - %s, %v: Output the standard error message via e.Error().
// - %+v: Output a detailed, structured (JSON-like) representation of the error.
func (e *wrapper) Format(f fmt.State, verb rune) {
	if e == nil {
		fmt.Fprint(f, "<nil>")
		return
	}
	e.RLock() 
	
	errStr := "" 
	detailedStr := ""

	if verb == 'v' && f.Flag('+') { 
		var sb strings.Builder
		e.formatDetailed(&sb) 
		detailedStr = sb.String()
	} else { 
		errStr = e.Error() 
	}
	
	e.RUnlock() 

	if detailedStr != "" {
		io.WriteString(f, detailedStr)
	} else {
		io.WriteString(f, errStr)
	}
}

// Json returns a map[string]interface{} representation of the error,
// suitable for structured logging or marshalling.
func (e *wrapper) Json() map[string]interface{} {
	if e == nil {
		return make(map[string]interface{})
	}
	e.RLock()
	defer e.RUnlock()

	jsonMap := make(map[string]interface{})

	if len(e.codes) > 0 {
		jsonMap["code"] = e.codes
	}
	if len(e.messages) > 0 {
		jsonMap["message"] = e.messages
	}
	if len(e.statuses) > 0 {
		statusMaps := make([]map[string]interface{}, len(e.statuses))
		for i, s := range e.statuses {
			statusMaps[i] = map[string]interface{}{"message": s.Message, "code": s.Code}
		}
		jsonMap["status"] = statusMaps
	}

	originalCause := errors.Unwrap(e.cause)
	if originalCause != nil {
		if causeWrapper, ok := originalCause.(ErrorWrapper); ok {
			jsonMap["cause"] = causeWrapper.Json()
		} else {
			jsonMap["cause"] = originalCause.Error()
		}
	}
	return jsonMap
}

// WithMessage appends a message to the error's message list.
// If a cause already exists, it re-wraps the error using `fmt.Errorf` with "%w",
// ensuring the original cause is preserved and the error chain is maintained.
// The error's own codes and newly augmented messages form the prefix of the error string.
func (e *wrapper) WithMessage(message string) ErrorWrapper {
	e.Lock()
	defer e.Unlock()

	e.messages = append(e.messages, message)

	currentMsg := e.buildPrimaryMessageWithoutLock()
	originalCause := errors.Unwrap(e.cause) 
	if originalCause != nil {
		e.cause = fmt.Errorf("%s: %w", currentMsg, originalCause)
	} else {
		e.cause = errors.New(currentMsg)
	}
	return e
}

// WithStatus appends a status (code and descriptive message) to the error.
// If the input status code is 0, this method is a no-op and returns the error unchanged.
// Otherwise, a new statusCode entry is added to the error's status list.
func (e *wrapper) WithStatus(status int) ErrorWrapper {
	if status == 0 {
		return e // No-op for status code 0
	}

	e.Lock()
	defer e.Unlock()

	newStatus := statusCode{
		Message: getStatusText(status),
		Code:    status,
	}
	e.statuses = append(e.statuses, newStatus)
	return e
}

// WithCause wraps the given error `err`, making it the underlying cause of this error.
// It uses `fmt.Errorf` with the "%w" verb, so the wrapped error is available via `errors.Unwrap`.
// The current error's codes and messages (its primary message) will prefix the `err.Error()` string.
// If `err` is nil, `WithCause` will effectively store "<nil>" as the cause message string part.
func (e *wrapper) WithCause(err error) ErrorWrapper {
	e.Lock()
	defer e.Unlock()
	wrapperMessage := e.buildPrimaryMessageWithoutLock()
	e.cause = fmt.Errorf("%s: %w", wrapperMessage, err)
	return e
}

// Unwrap returns the underlying error previously wrapped by WithCause.
// This method makes the ErrorWrapper compatible with `errors.Unwrap`,
// `errors.Is`, and `errors.As` from the standard library.
func (e *wrapper) Unwrap() error {
	if e == nil {
		return nil
	}
	e.RLock()
	defer e.RUnlock()
	return errors.Unwrap(e.cause)
}

// GetCodes returns a copy of the error's internal codes.
func (e *wrapper) GetCodes() []string {
	if e == nil {
		return nil
	}
	e.RLock()
	defer e.RUnlock()
	codesCopy := make([]string, len(e.codes))
	copy(codesCopy, e.codes)
	return codesCopy
}

// GetMessages returns a copy of the error's internal messages.
func (e *wrapper) GetMessages() []string {
	if e == nil {
		return nil
	}
	e.RLock()
	defer e.RUnlock()
	messagesCopy := make([]string, len(e.messages))
	copy(messagesCopy, e.messages)
	return messagesCopy
}

// GetStatuses returns a copy of the error's internal status codes and messages.
func (e *wrapper) GetStatuses() []statusCode {
	if e == nil {
		return nil
	}
	e.RLock()
	defer e.RUnlock()
	statusesCopy := make([]statusCode, len(e.statuses))
	copy(statusesCopy, e.statuses)
	return statusesCopy
}

// codeString returns a string representation of codes, assuming lock is held.
func (e *wrapper) codeString() string {
	return joinToStringSlice(e.codes)
}

// messageString returns a string representation of messages, assuming lock is held.
func (e *wrapper) messageString() string {
	return joinToStringSlice(e.messages)
}

// statusString returns a string representation of statuses, assuming lock is held.
func (e *wrapper) statusString() string {
	s := make([]interface{}, len(e.statuses))
	for i, v := range e.statuses {
		s[i] = v
	}
	return mapToString(s, func(item interface{}) string {
		status := item.(statusCode)
		return fmt.Sprintf("{\"message\": \"%s\", \"code\": %d}", strings.ReplaceAll(status.Message, "\"", "\\\""), status.Code)
	})
}

// Is checks if the current error instance (`e`) is equivalent to the `target` error.
// This method is invoked by `errors.Is` from the standard library.
// If `target` is also an `ErrorWrapper`, this method compares their codes.
// This allows `errors.Is(errInstance, PredefinedError)` to work by checking if `errInstance`
// has the same codes as the `PredefinedError` definition.
// If `target` is not an `ErrorWrapper`, or if codes do not match, it returns `false`,
// and `errors.Is` will then proceed to unwrap `e` and compare the cause.
func (e *wrapper) Is(target error) bool {
	if e == nil && target == nil {
		return true
	}
	if e == nil || target == nil {
		return false
	}

	if e == target { // Pointer equality
		return true
	}
	
	targetWrapper, ok := target.(ErrorWrapper)
	if !ok {
		return false // Target is not an ErrorWrapper, codes cannot be compared here.
	}

	// Both are ErrorWrapper types. Compare their codes for equivalence.
	eCodes := e.GetCodes() 
	targetCodes := targetWrapper.GetCodes()

	if len(eCodes) > 0 && len(targetCodes) > 0 {
		if strings.Join(eCodes, ";") == strings.Join(targetCodes, ";") {
			return true
		}
	}
	
	return false // Codes do not match, or one has no codes.
}

// Is is a package-level convenience function that uses `errors.Is`
// from the standard library to check if error `e` is (or wraps) `target`.
// It is recommended to use `errors.Is` directly.
func Is(e error, target error) bool {
	return errors.Is(e, target)
}

// Define creates a new error definition (an ErrorWrapper).
// `code` is a machine-readable string identifying the error class.
// `status` is an HTTP status code to associate with this error type.
// If `status` is 0, no statusCode entry is created for this error definition,
// meaning the error will not have an explicit HTTP status unless one is added later
// via `WithStatus`.
func Define(code string, status int) ErrorWrapper {
	statuses := []statusCode{}
	if status != 0 { // Only add status if it's non-zero
		statuses = append(statuses, statusCode{
			Message: getStatusText(status),
			Code:    status,
		})
	}

	w := &wrapper{
		codes:    []string{code},
		messages: []string{}, // Messages are typically added to instances, not definitions.
		statuses: statuses,
		RWMutex:  &sync.RWMutex{},
	}
	w.cause = errors.New(w.buildPrimaryMessageWithoutLock())
	return w
}

// New creates a new error.
// Deprecated: Prefer using `Define(code, status).WithCause(cause).WithMessage(message)`
// or `PredefinedError.WithCause(cause).WithMessage(message)` for clearer error construction.
func New(code string, cause error) ErrorWrapper {
	def := Define(code, http.StatusInternalServerError) 
	if cause != nil {
		def = def.WithCause(cause)
	}
	return def
}

// FromDefinition creates a new error instance from a base error definition `e`.
// The new instance inherits codes, messages, and statuses from `e`.
// The provided `cause` becomes the underlying cause of the new error instance.
// If `cause` is nil, the new error's cause will be based on its own primary message.
func (e *wrapper) FromDefinition(cause error) ErrorWrapper {
	if e == nil {
		unknown := Define("unknown_from_nil_definition", http.StatusInternalServerError)
		if cause != nil {
			return unknown.WithCause(cause)
		}
		return unknown
	}

	e.RLock() 
	newWrapper := &wrapper{
		codes:    append([]string{}, e.codes...),
		messages: append([]string{}, e.messages...), 
		statuses: append([]statusCode{}, e.statuses...),
		RWMutex:  &sync.RWMutex{},
	}
	e.RUnlock() 
	
	if cause != nil {
		return newWrapper.WithCause(cause) 
	}
	newWrapper.cause = errors.New(newWrapper.buildPrimaryMessageWithoutLock())
	return newWrapper
}

// Wrap wraps an error with a new message.
// Deprecated: Prefer `existingError.WithMessage("additional context")` if `existingError`
// is an `ErrorWrapper`. If wrapping a standard error or creating a new one, use
// `Define(...).WithCause(err).WithMessage(message)` or
// `PredefinedError.WithCause(err).WithMessage(message)`.
func Wrap(e error, message string) ErrorWrapper {
	if e == nil {
		return UnknownError.WithMessage(message) 
	}
	if ew, ok := e.(ErrorWrapper); ok {
		return ew.WithMessage(message)
	}
	return UnknownError.WithCause(e).WithMessage(message)
}

// Code retrieves the primary (first) error code from an ErrorWrapper.
// Returns an empty string if the error is not an ErrorWrapper or has no codes.
func Code(e error) string {
	if ew, ok := e.(ErrorWrapper); ok {
		codes := ew.GetCodes()
		if len(codes) > 0 {
			return codes[0]
		}
	}
	return ""
}

// Message retrieves messages from an ErrorWrapper, joined by "; ".
// If `e` is not an ErrorWrapper but implements `error`, it returns `e.Error()`.
// Returns an empty string if `e` is `nil` or if an `ErrorWrapper` has no messages.
func Message(e error) string {
	if ew, ok := e.(ErrorWrapper); ok {
		messages := ew.GetMessages()
        if len(messages) > 0 {
		    return strings.Join(messages, "; ")
        }
        return "" // ErrorWrapper with no messages returns empty for this helper
	}
	if e != nil {
		return e.Error() 
	}
	return ""
}

// Status retrieves the HTTP status code of the first status entry from an ErrorWrapper.
// Returns 0 if the error is not an ErrorWrapper, has no statuses, or is `nil`.
func Status(e error) int {
	if wp, ok := e.(ErrorWrapper); ok {
		statuses := wp.GetStatuses()
		if len(statuses) > 0 {
			return statuses[0].Code
		}
	}
	return 0 
}

// getStatusText returns the HTTP status text for a code, or a generic "Status X" string.
func getStatusText(status int) string {
	if status == 0 { 
		return "Status 0" 
	}
	statusText := http.StatusText(status)
	if statusText == "" {
		return "Status " + strconv.Itoa(status)
	}
	return statusText
}

// joinToStringSlice converts a slice of strings to a JSON-array-like string.
// Assumes lock is held by caller.
func joinToStringSlice(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[\"")
	sb.WriteString(strings.Join(arr, "\", \""))
	sb.WriteString("\"]")
	return sb.String()
}

// mapToString converts a slice of interfaces to a JSON-array-like string using a mapping function.
// Assumes lock is held by caller.
func mapToString(arr []interface{}, mapFn func(item interface{}) string) string {
	if len(arr) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, item := range arr {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(mapFn(item))
	}
	sb.WriteString("]")
	return sb.String()
}
