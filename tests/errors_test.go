package tests

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/felipewom/go-wrapperrors/wrapperrors"
	"github.com/stretchr/testify/assert"
)

// TestRefactoredNewError_Empty replaces TestNewError_Empty
func TestRefactoredNewError_Empty(t *testing.T) {
	err := wrapperrors.Define("testing_error", 0). // Define creates a base error
							WithMessage("message a").
							WithMessage("message b")

	assert.Equal(t, []string{"testing_error"}, err.GetCodes())
	assert.Equal(t, []string{"message a", "message b"}, err.GetMessages())
	assert.Empty(t, err.GetStatuses()) // Status 0 in Define means no status
	assert.Nil(t, errors.Unwrap(err))

	// Check Error() output for key parts
	errorStr := err.Error()
	assert.Contains(t, errorStr, "codes:testing_error")
	assert.Contains(t, errorStr, "messages:message a,message b")
}

// TestRefactoredNewErrorFromRaw replaces TestNewErrorFromRaw
func TestRefactoredNewErrorFromRaw(t *testing.T) {
	cause := errors.New("testing cause")
	err := wrapperrors.Define("testing_error_raw", http.StatusInternalServerError).
		WithMessage("message a").
		WithCause(cause)

	assert.Equal(t, []string{"testing_error_raw"}, err.GetCodes())
	assert.Equal(t, []string{"message a"}, err.GetMessages())
	if assert.Len(t, err.GetStatuses(), 1) {
		assert.Equal(t, http.StatusInternalServerError, err.GetStatuses()[0].Code)
		assert.Equal(t, "Internal Server Error", err.GetStatuses()[0].Message)
	}

	assert.Equal(t, cause, errors.Unwrap(err)) // Check that the cause is correctly wrapped

	// Check Error() output
	errorStr := err.Error()
	assert.Contains(t, errorStr, "codes:testing_error_raw")
	assert.Contains(t, errorStr, "messages:message a")
	assert.Contains(t, errorStr, cause.Error()) // The cause's message should be part of the wrapper's Error()
}

// TestRefactoredNewErrorFromDefinition replaces TestNewErrorFromDefinition
func TestRefactoredNewErrorFromDefinition(t *testing.T) {
	notFoundDef := wrapperrors.Define("not_found_def", http.StatusNotFound)
	errMsg := notFoundDef.FromDefinition(sql.ErrNoRows).WithMessage("car has not been found")

	assert.Equal(t, []string{"not_found_def"}, errMsg.GetCodes())
	assert.Contains(t, errMsg.GetMessages(), "car has not been found") // FromDefinition copies messages too
	if assert.Len(t, errMsg.GetStatuses(), 1) {
		assert.Equal(t, http.StatusNotFound, errMsg.GetStatuses()[0].Code)
	}
	assert.Equal(t, sql.ErrNoRows, errors.Unwrap(errMsg))

	errorStr := errMsg.Error()
	assert.Contains(t, errorStr, "codes:not_found_def")
	assert.Contains(t, errorStr, "messages:car has not been found")
	assert.Contains(t, errorStr, sql.ErrNoRows.Error())
}

func TestExpectedNewError(t *testing.T) {
	notFound := wrapperrors.Define("not_found_test", http.StatusNotFound)
	assert.Error(t, notFound) // Check it fulfills the error interface
}

func TestErrorsIs(t *testing.T) {
	def1 := wrapperrors.Define("def1_code", http.StatusInternalServerError)
	def2 := wrapperrors.Define("def2_code", http.StatusNotFound)
	def1_variant := wrapperrors.Define("def1_code", http.StatusBadRequest) // Same code, different status

	errDef1WithMessage := def1.WithMessage("instance of def1")
	assert.True(t, errors.Is(errDef1WithMessage, def1), "Instance should be Is def1")
	assert.False(t, errors.Is(errDef1WithMessage, def2), "Instance should not be Is def2")
	assert.True(t, errors.Is(errDef1WithMessage, def1_variant), "Instance should be Is def1_variant due to same codes")

	stdErr := errors.New("standard root cause")
	wrappedStdErr := def1.WithCause(stdErr)

	assert.True(t, errors.Is(wrappedStdErr, def1), "Wrapped stdErr should be Is def1")
	assert.True(t, errors.Is(wrappedStdErr, stdErr), "Wrapped stdErr should be Is stdErr (the cause)")
	assert.False(t, errors.Is(wrappedStdErr, def2), "Wrapped stdErr should not be Is def2")

	// Test with a globally defined error
	internalErrInstance := wrapperrors.InternalError.WithMessage("specific internal issue")
	assert.True(t, errors.Is(internalErrInstance, wrapperrors.InternalError), "Instance of InternalError should be Is InternalError")

	// Nested wrapping
	// w2 (def3) -> w1 (def1) -> stdErr (def2)
	// Note: WithCause creates a new error instance, so we build it up carefully
	w1 := def1.FromDefinition(def2.WithMessage("def2 message")) // w1 is def1, cause is def2
	w2 := def2.FromDefinition(w1.WithMessage("def1 message"))   // w2 is def2, cause is w1

	assert.True(t, errors.Is(w2, def2), "Nested: w2 should be Is def2 (itself)")
	assert.True(t, errors.Is(w2, def1), "Nested: w2 should be Is def1 (via w1)")

	// Test errors.Is against the direct cause of w1, which is def2
	unwrappedW1 := errors.Unwrap(w1)
	assert.True(t, errors.Is(unwrappedW1, def2), "Unwrapped w1 should be def2")

	// Test case where the error being checked IS the definition itself
	assert.True(t, errors.Is(def1, def1), "Definition should be Is itself")

	// Test against a non-ErrorWrapper error that is not in the chain
	someOtherStdErr := errors.New("some other standard error")
	assert.False(t, errors.Is(wrappedStdErr, someOtherStdErr))
}

type MyCustomError struct {
	Msg string
}

func (mce *MyCustomError) Error() string {
	return mce.Msg
}

func TestErrorsAs(t *testing.T) {
	// Case 1: As ErrorWrapper
	definedErr := wrapperrors.Define("my_as_code", http.StatusBadRequest).WithMessage("details for as")
	errWithCause := definedErr.WithCause(errors.New("original cause for as"))

	var targetEW wrapperrors.ErrorWrapper
	if assert.True(t, errors.As(errWithCause, &targetEW), "Should be able to 'As' ErrorWrapper") {
		assert.Equal(t, []string{"my_as_code"}, targetEW.GetCodes())
		assert.Contains(t, targetEW.GetMessages(), "details for as")
		// Note: errors.As assigns the first matching error in the chain.
		// Here, errWithCause itself is an ErrorWrapper.
		// So targetEW will be errWithCause.
		assert.Equal(t, errWithCause.Error(), targetEW.Error())
	}

	// Case 2: As custom error type
	customErr := &MyCustomError{Msg: "I am a custom error"}
	wrappedCustomErr := wrapperrors.Define("wrapper_for_custom", http.StatusInternalServerError).WithCause(customErr)

	var targetCustom *MyCustomError
	if assert.True(t, errors.As(wrappedCustomErr, &targetCustom), "Should be able to 'As' MyCustomError") {
		assert.Equal(t, "I am a custom error", targetCustom.Msg)
		assert.Equal(t, customErr, targetCustom) // Should be the exact same instance
	}

	// Case 3: errors.As where target is not in the chain
	var anotherTarget *MyCustomError
	assert.False(t, errors.As(errWithCause, &anotherTarget), "Should not 'As' an unrelated type")
}

func TestJsonOutput(t *testing.T) {
	causeErr := wrapperrors.Define("json_cause_code", http.StatusConflict).WithMessage("json_cause_message")
	err := wrapperrors.Define("json_code", http.StatusBadRequest).
		WithMessage("json_message1").
		WithMessage("json_message2").
		WithStatus(http.StatusPaymentRequired). // Add another status
		WithCause(causeErr)

	jsonMap := err.Json()

	// Check codes
	codes, ok := jsonMap["code"].([]string)
	assert.True(t, ok, "JSON 'code' should be []string")
	assert.Equal(t, []string{"json_code"}, codes)

	// Check messages
	messages, ok := jsonMap["message"].([]string)
	assert.True(t, ok, "JSON 'message' should be []string")
	assert.Equal(t, []string{"json_message1", "json_message2"}, messages)

	// Check statuses
	statuses, ok := jsonMap["status"].([]map[string]interface{})
	assert.True(t, ok, "JSON 'status' should be []map[string]interface{}")
	if assert.Len(t, statuses, 2) { // Defined with 400, added 402
		// Order might not be guaranteed, so check for presence
		found400 := false
		found402 := false
		for _, s := range statuses {
			codeVal, ok := s["code"].(int)
			assert.True(t, ok, "status code should be an int")
			if codeVal == http.StatusBadRequest {
				found400 = true
				assert.Equal(t, "Bad Request", s["message"])
			}
			if codeVal == http.StatusPaymentRequired {
				found402 = true
				assert.Equal(t, "Payment Required", s["message"])
			}
		}
		assert.True(t, found400, "Status 400 not found in JSON")
		assert.True(t, found402, "Status 402 not found in JSON")
	}

	// Check cause
	causeMap, ok := jsonMap["cause"].(map[string]interface{})
	assert.True(t, ok, "JSON 'cause' should be a map for ErrorWrapper cause")

	causeCodes, _ := causeMap["code"].([]string)
	assert.Equal(t, []string{"json_cause_code"}, causeCodes)

	causeMessages, _ := causeMap["message"].([]string)
	assert.Equal(t, []string{"json_cause_message"}, causeMessages)
}

func TestFmtFormatter(t *testing.T) {
	cause := errors.New("fmt_root_cause")
	err := wrapperrors.Define("fmt_code", http.StatusInternalServerError).
		WithMessage("fmt_msg1").
		WithStatus(http.StatusForbidden).
		WithCause(cause)

	// Test %s and %v (should be the same, standard error message)
	sVerb := fmt.Sprintf("%s", err)
	vVerb := fmt.Sprintf("%v", err)

	expectedErrorString := "codes:fmt_code messages:fmt_msg1: fmt_root_cause" // Based on current Error() impl
	assert.Equal(t, expectedErrorString, sVerb, "Output of %s verb")
	assert.Equal(t, expectedErrorString, vVerb, "Output of %v verb")

	assert.Contains(t, sVerb, "fmt_code")
	assert.Contains(t, sVerb, "fmt_msg1")
	assert.Contains(t, sVerb, "fmt_root_cause")

	// Test %+v (detailed, structured format)
	plusVVerb := fmt.Sprintf("%+v", err)

	assert.Contains(t, plusVVerb, "\"code\": [\"fmt_code\"]")
	assert.Contains(t, plusVVerb, "\"message\": [\"fmt_msg1\"]")

	// Check for status details (order independent)
	assert.Contains(t, plusVVerb, "\"status\": [")
	assert.Contains(t, plusVVerb, "\"message\": \"Internal Server Error\", \"code\": 500")
	assert.Contains(t, plusVVerb, "\"message\": \"Forbidden\", \"code\": 403")

	assert.Contains(t, plusVVerb, "\"cause\": \"fmt_root_cause\"") // Cause is a standard error
}

func TestEdgeCasesAndAPIHelpers(t *testing.T) {
	// Define with status = 0
	defZeroStatus := wrapperrors.Define("code_zero_status", 0)
	assert.Empty(t, defZeroStatus.GetStatuses(), "Define with status 0 should have no statuses")
	jsonZeroStatus := defZeroStatus.Json()
	_, statusExists := jsonZeroStatus["status"]
	assert.False(t, statusExists, "JSON output should not have 'status' field if defined with 0")

	// FromDefinition with nil cause
	defForNilCause := wrapperrors.Define("def_nil_cause", http.StatusTeapot)
	instanceFromNilCause := defForNilCause.FromDefinition(nil)
	assert.Equal(t, defForNilCause.GetCodes(), instanceFromNilCause.GetCodes())
	assert.Equal(t, defForNilCause.GetMessages(), instanceFromNilCause.GetMessages()) // Messages are copied
	assert.Equal(t, defForNilCause.GetStatuses(), instanceFromNilCause.GetStatuses()) // Statuses are copied
	// errors.Unwrap(instanceFromNilCause) should be nil as per current fmt.Errorf behavior
	// if the message for fmt.Errorf doesn't include %w, or if errors.New is used.
	// Our FromDefinition when cause is nil uses errors.New(primaryMessage) for its .cause field.
	assert.Nil(t, errors.Unwrap(instanceFromNilCause.Unwrap()), "Unwrap(Unwrap()) should be nil for FromDefinition(nil)")
	assert.Contains(t, instanceFromNilCause.Error(), "codes:def_nil_cause", "Error message for FromDefinition(nil)")

	// Helpers with non-ErrorWrapper errors
	stdErr := errors.New("std error for helpers")
	assert.Empty(t, wrapperrors.Code(stdErr), "Code() of stdErr should be empty")
	assert.Equal(t, stdErr.Error(), wrapperrors.Message(stdErr), "Message() of stdErr should be its Error()")
	assert.Equal(t, 0, wrapperrors.Status(stdErr), "Status() of stdErr should be 0")

	// Helpers with nil error
	assert.Empty(t, wrapperrors.Code(nil), "Code(nil) should be empty")
	assert.Empty(t, wrapperrors.Message(nil), "Message(nil) should be empty")
	assert.Zero(t, wrapperrors.Status(nil), "Status(nil) should be 0")

	// WithStatus(0) no-op
	errWithStatus := wrapperrors.Define("status_zero_code", http.StatusAccepted) // Has one status initially
	numStatusesBefore := len(errWithStatus.GetStatuses())
	assert.Equal(t, 1, numStatusesBefore)
	errWithStatus.WithStatus(0) // Should be no-op
	assert.Len(t, errWithStatus.GetStatuses(), numStatusesBefore, "WithStatus(0) should not add a status")
	errWithStatus.WithStatus(http.StatusContinue) // Add a valid one
	assert.Len(t, errWithStatus.GetStatuses(), numStatusesBefore+1, "WithStatus(non-zero) should add a status")

	// Define with empty code string
	emptyCodeDef := wrapperrors.Define("", http.StatusAlreadyReported)
	assert.Equal(t, []string{""}, emptyCodeDef.GetCodes(), "Define with empty code string")
	assert.Contains(t, emptyCodeDef.Error(), "codes:") // Should still contain "codes:" part
	assert.NotContains(t, emptyCodeDef.Error(), "codes:,", "Error string should not have hanging comma for empty code")

	// WithMessage with empty string
	withEmptyMsg := emptyCodeDef.WithMessage("")
	assert.Equal(t, []string{""}, withEmptyMsg.GetMessages(), "WithMessage with empty string")
	assert.Contains(t, withEmptyMsg.Error(), "messages:")
	assert.NotContains(t, withEmptyMsg.Error(), "messages:,", "Error string should not have hanging comma for empty message")

	// Chaining WithCause(nil)
	errChainedNilCause := wrapperrors.Define("chain_nil_cause", 500).WithCause(nil)
	// WithCause(nil) is like fmt.Errorf("message: %w", nil), which results in "message: <nil>"
	// errors.Unwrap will return nil.
	assert.Nil(t, errors.Unwrap(errChainedNilCause), "WithCause(nil) should result in a nil cause after unwrap")
	assert.Contains(t, errChainedNilCause.Error(), "<nil>", "Error string for WithCause(nil) should contain <nil>")
}

func TestJsonOutputWithNilCause(t *testing.T) {
	err := wrapperrors.Define("json_nil_cause", http.StatusPartialContent).WithCause(nil)
	jsonMap := err.Json()

	_, causeExists := jsonMap["cause"]
	assert.False(t, causeExists, "JSON 'cause' should not exist if WithCause(nil) was used")

	errWithNoCauseAtAll := wrapperrors.Define("json_no_cause", http.StatusAccepted)
	jsonMapNoCause := errWithNoCauseAtAll.Json()
	_, causeExistsNoCause := jsonMapNoCause["cause"]
	assert.False(t, causeExistsNoCause, "JSON 'cause' should not exist if no cause was ever added")
}

func TestJsonWithStandardErrorCause(t *testing.T) {
	stdCause := errors.New("standard cause for json")
	err := wrapperrors.Define("json_std_cause", http.StatusOK).WithCause(stdCause)
	jsonMap := err.Json()

	causeVal, ok := jsonMap["cause"].(string)
	assert.True(t, ok, "jsonMap[cause] should be a string for standard error cause")
	assert.Equal(t, stdCause.Error(), causeVal)
}

// Ensure all public API methods are safe with nil receiver (though not idiomatic to call them on nil)
// Go itself will panic if methods are called on nil pointers unless the method has a nil check.
// Our ErrorWrapper is an interface, so the concrete *wrapper type is what matters.
// The functions Code, Message, Status are package-level and already handle nil.
// Methods on *wrapper will panic if 'e' is nil and not checked (e.g. e.RLock()).
// This is standard Go behavior. We can test the package-level functions.

func TestHelperFunctionsWithNilError(t *testing.T) {
	assert.Equal(t, "", wrapperrors.Code(nil))
	assert.Equal(t, "", wrapperrors.Message(nil))
	assert.Equal(t, 0, wrapperrors.Status(nil))
	// wrapperrors.Is(nil, nil) should be true by errors.Is semantics
	assert.True(t, wrapperrors.Is(nil, nil), "Is(nil, nil) should be true")
	assert.False(t, wrapperrors.Is(wrapperrors.Define("d", 0), nil), "Is(err, nil) should be false")
	assert.False(t, wrapperrors.Is(nil, wrapperrors.Define("d", 0)), "Is(nil, err) should be false")
}
