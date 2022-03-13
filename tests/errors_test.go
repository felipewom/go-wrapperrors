package tests

import (
	"database/sql"
	"errors"
	"github.com/felipewom/go-wrapperrors/wrapperrors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestNewError_Empty(t *testing.T) {
	expected1 := "code: [testing_error]"
	wrappedError := wrapperrors.New("testing_error", nil).
		WithStatus(0).
		WithMessage("message a").
		WithMessage("message b").
		Error()
	assert.Equal(t, expected1, wrappedError)
}

func TestNewErrorFromRaw(t *testing.T) {
	expected := "{\"code\": [\"testing_error\"], \"message\": [\"message a\"], \"status\": [{\"message\": \"Internal Server Error\", \"code\": 500}], \"cause\": \"testing error\"}"
	wrappedError := wrapperrors.New("testing_error", errors.New("testing error")).
		WithStatus(http.StatusInternalServerError).
		WithMessage("message a").
		String()
	assert.Equal(t, expected, wrappedError)
}

func TestNewErrorFromDefinition(t *testing.T) {
	notFound := wrapperrors.Define("not_found", http.StatusNotFound)
	errMsg := notFound.FromDefinition(sql.ErrNoRows).WithMessage("car has not been found in the database")
	assert.EqualValues(t, "cause: [sql: no rows in result set]; code: [not_found]", errMsg.Error())
	assert.EqualValues(t, "{\"code\": [\"not_found\"], \"message\": [\"car has not been found in the database\"], \"status\": [{\"message\": \"Not Found\", \"code\": 404}], \"cause\": \"sql: no rows in result set\"}", errMsg.String())
}

func TestExpectedNewError(t *testing.T) {
	notFound := wrapperrors.Define("not_found", http.StatusNotFound)
	assert.Error(t, notFound)
}
