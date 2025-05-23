# WrappErrors

Package `wrapperrors` provides primitives for creating and handling errors in Go. It enhances standard error handling by allowing errors to carry detailed, structured information like multiple codes, messages, and HTTP status codes, while remaining fully compatible with Go's standard error functions (`errors.Is`, `errors.As`, `errors.Unwrap`). It also offers custom formatting options for detailed logging.

```bash
go get github.com/felipewom/go-wrapperrors
```

## Features

*   Define reusable error types with associated codes and default HTTP status codes.
*   Create error instances with detailed context including multiple messages and status codes.
*   Wrap existing errors (standard or `ErrorWrapper` types) to add context while preserving the original error chain.
*   Full compatibility with standard Go error handling:
    *   `errors.Is(err, target)`
    *   `errors.As(err, &targetInterfaceOrType)`
    *   `errors.Unwrap(err)`
*   Custom error formatting using `fmt.Formatter`:
    *   `%s`, `%v`: Standard error message.
    *   `%+v`: Detailed, JSON-like structured output.
*   Access structured error details programmatically (`GetCodes()`, `GetMessages()`, `GetStatuses()`, `Json()`).

## Defining Error Types

First, define the types of errors your application will use. This is typically done as global variables using `wrapperrors.Define(code string, defaultStatus int)`.

**Example:** Create a package `apperrors` for your application's error definitions.

```go
package errorsdefinitions

import (
	"github.com/felipewom/go-wrapperrors/wrapperrors"
	"net/http"
)

var (
	Internal       = wrapperrors.Define("internal_error", http.StatusInternalServerError)
	NotFound       = wrapperrors.Define("not_found", http.StatusNotFound)
	InvalidPayload = wrapperrors.Define("invalid_payload", http.StatusBadRequest)
	Unauthorized   = wrapperrors.Define("unauthorized_access", http.StatusUnauthorized)
)
```

## Creating and Augmenting Errors

Create instances from your predefined errors and add context using `WithCause(error)` and `WithMessage(string)`.

**Example:** Returning a `NotFound` error if a database record isn't found.

```go
// file: services/carservice.go
package services

import (
	"database/sql"
	"errors" // Standard errors package
	"fmt" 
	"github.com/your_username/your_project/apperrors" // Your error definitions
)

type Car struct {
	ID   string
	Name string
}

// GetCar retrieves a car by its ID.
func GetCar(id string) (Car, error) {
	var car Car
	// Simulating database call
	// err := db.QueryRow("SELECT id, name FROM car WHERE id = ?", id).Scan(&car.ID, &car.Name)
	err := sql.ErrNoRows // Simulate error for example

	if err == nil {
		return car, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		// Create an instance of apperrors.NotFound, wrap sql.ErrNoRows, and add a descriptive message.
		return Car{}, apperrors.NotFound.WithCause(err).WithMessage(fmt.Sprintf("Car with ID '%s' not found in the database", id))
	}

	// For other unexpected database errors, wrap with a general internal error.
	return Car{}, apperrors.Internal.WithCause(err).WithMessage("Failed to retrieve car from database due to an unexpected issue")
}
```

## Adding Context to Existing Errors

You can add more context to an existing error. If it's an `ErrorWrapper`, you can use its methods. If it's a standard error, you'll wrap it.

```go
func ProcessCar(carID string) error {
	car, err := GetCar(carID)
	if err != nil {
        // Assuming GetCar returns an ErrorWrapper compatible error
        if ew, ok := err.(wrapperrors.ErrorWrapper); ok {
		    // Add more specific context to the error returned by GetCar
		    return ew.WithMessage(fmt.Sprintf("Processing failed for car ID %s", carID))
        }
        // If it's not an ErrorWrapper, but you want to make it one:
        return apperrors.Internal.WithCause(err).WithMessage(fmt.Sprintf("Processing failed for car ID %s", carID))
	}
	// ... process car
	return nil
}

// If you need to wrap a standard error or change the fundamental cause:
func AnotherProcess(id string) error {
    stdErr := errors.New("a standard library error occurred")
    // some condition ...
    if stdErr != nil { 
        // Wrap the standard error with one of your defined types
        return apperrors.InvalidPayload.WithCause(stdErr).WithMessage("Standard error occurred during processing")
    }
    return nil
}
```

## Checking Error Types with `errors.Is`

Use `errors.Is` from the standard Go library to check if an error matches one of your predefined types or any error in its wrap chain. `wrapperrors.ErrorWrapper` is designed to work seamlessly with it.

```go
// file: main.go
package main

import (
	"errors" // Standard errors package
	"fmt"
	"log" // Standard log package
	"github.com/your_username/your_project/apperrors"  // Your error definitions
	"github.com/your_username/your_project/services" // Your services
)

func main() {
	_, err := services.GetCar("123") // Example call
	if err != nil {
		if errors.Is(err, apperrors.NotFound) {
			fmt.Println("Handler: The requested car was not found.")
			// Handle NotFound specifically (e.g., return 404 to client)
		} else if errors.Is(err, apperrors.Internal) {
			fmt.Println("Handler: An unexpected internal error occurred.")
			// Handle Internal specifically
		} else {
			fmt.Println("Handler: An unknown error occurred.")
		}
		// Log the detailed error
		log.Printf("Detailed error for logging: %+v\n", err) 
	}
}
```
The package-level `wrapperrors.Is(err, target)` is also available but using `errors.Is` directly is generally recommended for consistency with standard Go practices.

## Standard Error Handling (`errors.As`, `errors.Unwrap`)

`ErrorWrapper` implements the `Unwrap() error` method, making it fully compatible with `errors.As` for retrieving a specific error type from the chain and `errors.Unwrap` for unwrapping layers of errors.

```go
type CustomDatabaseError struct {
    Query string
    OriginalError error
}
func (cde *CustomDatabaseError) Error() string { return fmt.Sprintf("db error with query '%s': %v", cde.Query, cde.OriginalError) }
func (cde *CustomDatabaseError) Unwrap() error { return cde.OriginalError }

// ...
// var someErr error 
// Assume 'someErr' is an error chain that might include an ErrorWrapper,
// which in turn wraps a *CustomDatabaseError.
// For example:
// originalDBErr := &CustomDatabaseError{Query: "SELECT *", OriginalError: errors.New("connection failed")}
// someErr = apperrors.Internal.WithCause(originalDBErr).WithMessage("Operation failed")


var cde *CustomDatabaseError
// if errors.As(someErr, &cde) {
//    fmt.Printf("Custom DB Error (Query: %s) found in chain!\n", cde.Query)
// }

// Unwrapping layer by layer
// unwrapped := errors.Unwrap(someErr)
// if unwrapped != nil {
//    fmt.Printf("Unwrapped error: %v\n", unwrapped)
// }
```

## Error Formatting and Accessing Details

`ErrorWrapper` provides multiple ways to access error information:

1.  **`err.Error()` or `fmt.Sprintf("%s", err)` or `fmt.Sprintf("%v", err)`**:
    *   Returns the standard error message. If the error wraps other errors using `WithCause`, their messages are included in a chain (e.g., "wrapper message: cause message: deeper cause message").
    *   Example: `codes:not_found,messages:Car with ID '123' not found in the database: sql: no rows in result set`

2.  **`err.String()` or `fmt.Sprintf("%+v", err)`**:
    *   Returns a detailed, JSON-like string representation, including codes, messages, statuses, and a recursively formatted cause. Useful for verbose logging.
    *   Example: `{"code": ["not_found"], "message": ["Car with ID '123' not found in the database"], "status": [{"message": "Not Found", "code": 404}], "cause": "sql: no rows in result set"}`
    *   (If the cause were also an `ErrorWrapper`, its details would be nested similarly.)

3.  **`err.Json() map[string]interface{}`**:
    *   Returns a map representation of the error, suitable for structured logging or programmatic access.
    *   Example structure:
        ```json
        {
          "code": ["not_found"],
          "message": ["Car with ID '123' not found in the database"],
          "status": [
            {"message": "Not Found", "code": 404}
          ],
          "cause": "sql: no rows in result set" 
        }
        ```
        (If the cause is an `ErrorWrapper`, `cause` will be a nested map; otherwise, it's the cause's error string.)

4.  **Accessor Methods**:
    *   `ew.GetCodes() []string`: Returns a slice of all codes (e.g., `[]string{"not_found"}`).
    *   `ew.GetMessages() []string`: Returns a slice of all messages (e.g., `[]string{"Car with ID '123' not found"}`).
    *   `ew.GetStatuses() []statusCode`: Returns a slice of `statusCode` structs (each with `Message` string and `Code` int fields).

**Example Usage (assuming `err` is an `ErrorWrapper`):**

```go
// import "github.com/felipewom/go-wrapperrors/wrapperrors"
// import "log" // Standard log package
// ... inside a function where 'err' is an ErrorWrapper instance
// var err error = // ... some error
// if ew, ok := err.(wrapperrors.ErrorWrapper); ok {
//    log.Printf("Standard error: %s\n", ew.Error()) 
//    log.Printf("Detailed error: %+v\n", ew)      
//
//    errMap := ew.Json()
//    log.Printf("Structured error: %v\n", errMap) // This will print the map using default formatting
//
//    if codes := ew.GetCodes(); len(codes) > 0 {
//        log.Printf("Primary error code: %s\n", codes[0])
//    }
// } else if err != nil {
//    log.Printf("Standard error (non-ErrorWrapper): %s\n", err.Error())
// }
```

## Deprecated Functions

*   `wrapperrors.New(code string, cause error)`: Use `Define(code, status).WithCause(cause).WithMessage(message)` or `PredefinedError.WithCause(cause).WithMessage(message)` instead for clearer error construction and explicit status definition.
*   `wrapperrors.Wrap(e error, message string)`: To add a message to an existing `ErrorWrapper`, use `existingErrorWrapper.WithMessage(message)`. To wrap a standard error or create a new `ErrorWrapper` with a cause, use `Define(...).WithCause(e).WithMessage(message)` or `PredefinedError.WithCause(e).WithMessage(message)`.

## Author

- **Felipe Moura** · [felipewom@gmail.com](mailto:felipewom@gmail.com)

## License

[MIT](https://github.com/felipewom/wrapperrors/blob/master/LICENSE)