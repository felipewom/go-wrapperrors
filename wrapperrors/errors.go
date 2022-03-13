// Package wrapperrors provides easy to use error handling primitives.
package wrapperrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	InternalError = Define("internal_error", http.StatusInternalServerError)
	UnknownError  = Define("unknown_error", http.StatusInternalServerError)
)

type ErrorWrapper interface {
	Error() string
	String() string
	Json() map[string]interface{}
	WithMessage(message string) ErrorWrapper
	WithStatus(status int) ErrorWrapper
	WithCause(err error) ErrorWrapper
	FromDefinition(cause error) ErrorWrapper
	Is(target error) bool
}

type wrapper struct {
	code    []string
	message []string
	status  []statusCode
	cause   error
	*sync.RWMutex
}

type statusCode struct {
	message string
	code    int
}

func (e wrapper) Error() string {
	parts := make([]string, 0)
	if e.cause != nil {
		parts = append(parts, fmt.Sprintf("cause: [%s]", e.cause.Error()))
	}
	if len(e.code) > 0 {
		codeStr := e.codeString()
		codeErr := strings.ReplaceAll(codeStr, "\"", "")
		parts = append(parts, fmt.Sprintf("code: %s", codeErr))
	}
	joinedParts := strings.Join(parts[:], "; ")
	return errors.New(fmt.Sprintf("%s", joinedParts)).Error()
}

// String returns an string containing all the internal information about the given error.
func (e *wrapper) String() string {
	if e == nil {
		return ""
	}
	parts := make([]string, 0)
	if len(e.code) > 0 {
		parts = append(parts, fmt.Sprintf("\"code\": %s", e.codeString()))
	}
	if len(e.message) > 0 {
		parts = append(parts, fmt.Sprintf("\"message\": %s", e.messageString()))
	}
	if len(e.status) > 0 {
		parts = append(parts, fmt.Sprintf("\"status\": %s", e.statusString()))
	}
	if e.cause != nil {
		parts = append(parts, fmt.Sprintf("\"cause\": \"%s\"", e.cause.Error()))
	}
	joinedParts := strings.Join(parts[:], ", ")
	return fmt.Sprintf("{%s}", joinedParts)
}

func (e *wrapper) Json() map[string]interface{} {
	jsonMap := make(map[string]interface{})
	if e == nil {
		return jsonMap
	}
	err := json.Unmarshal([]byte(e.Error()), &jsonMap)
	if err != nil {
		log.New(os.Stderr, "ERROR", 0).Println("error parsing wrapperrors map: %s", err.Error())
	}
	return jsonMap
}

func (e *wrapper) WithMessage(message string) ErrorWrapper {
	e.Lock()
	defer e.Unlock()
	e.message = wrapMessage(message, e)
	return e
}

func (e *wrapper) WithStatus(status int) ErrorWrapper {
	e.Lock()
	defer e.Unlock()
	e.status = wrapStatus(status, e)
	return e
}

func (e *wrapper) WithCause(err error) ErrorWrapper {
	e.Lock()
	defer e.Unlock()
	e.cause = wrapCause(err, e)
	return e
}

func (e wrapper) codeString() string {
	return joinToString(e.code)
}

func (e wrapper) messageString() string {
	return joinToString(e.message)
}

func (e wrapper) statusString() string {
	s := make([]interface{}, len(e.status))
	for i, v := range e.status {
		s[i] = v
	}
	return mapToString(s, func(item interface{}) string {
		status := item.(statusCode)
		return fmt.Sprintf("{\"message\": \"%s\", \"code\": %d}", status.message, status.code)
	})
}

// Is verify if a given error has the same time of the given target error.
// The target parameter should be an error previously defined with the Define function.
func (e wrapper) Is(target error) bool {
	if targetErr, ok := target.(wrapper); ok {
		return strings.Join(e.code[:], "; ") == strings.Join(targetErr.code[:], ";")
	}
	return e.Error() == target.Error()
}

// Is verify if a given error has the same time of the given target error.
// The target parameter should be an error previously defined with the Define function.
func Is(e error, target error) bool {
	err, eOk := e.(wrapper)
	targetErr, tOk := target.(wrapper)
	if eOk && tOk {
		return err.String() == targetErr.String()
	}
	return e == target
}

// Define define a new error base model.
func Define(code string, status int) ErrorWrapper {
	return &wrapper{
		code: []string{code},
		status: []statusCode{
			{
				message: getStatusText(status),
				code:    status,
			},
		},
		cause: nil,
	}
}

// New creates a new error from a given message and raw error.
func New(code string, cause error) ErrorWrapper {
	return newError(code, cause)
}

// FromDefinition creates a new error from a given pre-definition.
func (e wrapper) FromDefinition(cause error) ErrorWrapper {
	wp := newError(Code(e), cause)
	for _, status := range e.status {
		wp.WithStatus(status.code)
	}
	return wp
}

// Wrap wraps an error with a message.
func Wrap(e error, message string) ErrorWrapper {
	return wrap(e, message)
}

// Code retrieves the error internal code of a given error.
func Code(e error) string {
	if err, ok := e.(wrapper); ok {
		return strings.Join(err.code[:], "; ")
	}

	return ""
}

// Message retrieves the error internal message of a given error.
func Message(e error) string {
	if err, ok := e.(wrapper); ok {
		return strings.Join(err.message[:], "; ")
	}

	return ""
}

// Status retrieves the error internal status of a given error.
func Status(e error) string {
	if wp, ok := e.(wrapper); ok {
		return wp.statusString()
	}

	return ""
}

func newError(code string, cause error) ErrorWrapper {
	return &wrapper{
		code:    []string{code},
		cause:   cause,
		RWMutex: &sync.RWMutex{},
	}
}

func wrap(e error, message string) ErrorWrapper {
	if err, ok := e.(wrapper); ok {
		return err.WithMessage(message).WithCause(e)
	}

	return UnknownError.WithCause(e).WithMessage(message)
}

func wrapMessage(message string, e *wrapper) []string {
	return append(e.message, message)
}

func wrapStatus(status int, e *wrapper) []statusCode {
	newStatus := statusCode{
		message: getStatusText(status),
		code:    status,
	}
	if len(e.status) == 0 {
		return []statusCode{newStatus}
	}
	return append(e.status, newStatus)
}

func wrapCause(err error, e *wrapper) error {
	if e.cause == nil {
		return err
	}
	return errors.New(fmt.Sprintf("%v; %v;", e.cause.Error(), err.Error()))
}

func getStatusText(status int) string {
	statusText := http.StatusText(status)
	if statusText == "" {
		return strconv.Itoa(status)
	}
	return statusText
}

func joinToString(arr []string) string {
	s := make([]interface{}, len(arr))
	for i, v := range arr {
		s[i] = v
	}
	return mapToString(s, func(item interface{}) string {
		return fmt.Sprintf("\"%s\"", item)
	})
}

func mapToString(arr []interface{}, mapFn func(item interface{}) string) string {
	buff := strings.Builder{}
	buff.WriteString("[")
	parts := make([]string, 0)
	for _, item := range arr {
		parts = append(parts, mapFn(item))
	}
	buff.WriteString(strings.Join(parts[:], ", "))
	buff.WriteString("]")
	return buff.String()
}
