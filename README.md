# WrappErrors

Package wrapperrors provides simple error handling primitives.
Although Go has a good error handling approach, sometimes it is difficult to handle them appropiatly.
This package has been created for such purpose, allowing to define differents errors types and use them in the entire application.

```bash
go get github.com/felipewom/wrapperrors
````

## Defining errors types

First of all, it has to be defined the differents types of errors that are going to be used in the application.
This could be done as global variables somewhere in the project using the function `errors.Define` which receive an internal code and generate a new error type.

In the following example all the error types definitions are being done in a package called `errorsdefinitions`.

```go
package errorsdefinitions

var (
    Internal := wrapperrors.Define("internal")
    NotFound := wrapperrors.Define("not_found")
    InvalidPayload := wrapperrors.Define("invalid_payload")
)
```

## Creating a new error

The funcion `wrapperrors.New` creates a new error based on a previously defined error.
For example, if we want to return a not found error we can do it as follow.

```go
return wrapperrors.New(NotFound, nil, "the resource has not been found")
```

Let us see a more interested example:

```go
type Car struct {
    ID      string
    Name    string
}

// GetCar retrieves the car with the given id
func GetCar(id string) (Car, error) {
    car := Car{}

    err := db.QueryRow("SELECT id, name FROM car WHERE id = ?", id).Scan(&car.ID, &person.Name)
    if err == sql.ErrNoRows {
        return Car{}, wrapperrors.New(errorsdefinitions.NotFound, err, "car has not been found in the database")
    } else if err != nil {
        return Car{}, wrapperrors.New(errorsdefinitions.Internal, err, "get car from database has failed")
    }

    return car, nil
}
```

## Wrapping an error

The function `wrapperrors.Wrap` wraps an error with a new message but keeps the same error type.
In some cases, we actually want to do is to return an error of the same type but with a different message, so for those such cases this function is intented to be used.

```go
func GetPersonEmail(id string) (string, error) {
    person, err := GetPerson(id)
    if err != nil {
        return "", wrapperrors.Wrap(err, "the person could not be retrieve")
    }

    return person.Email, nil 
}
```

## Is

The function `wrapperrors.Is` checks if a given error is an error of some type.

```go
func GetPersonEmail(id string) (string, error) {
    person, err := GetPerson(id)
    if wrapperrors.Is(err, apperrors.NotFound) {
        return "", nil
    } else if err != nil {
        return wrapperrors.Wrap(err, "get person has failed")
    }

    return person.Email, nil
}
```

## Error() and String()

There are two functions that allows us to print the error information: `wrapperrors.Error` and `wrapperrors.String`.
The first one only returns the error message and the second one returns a string with the underlying information such as the internal code and the stacktrace among others.

```go
func main() {
    email, err := GetPersonEmail("123")
    if err != nil {
        log.Error(wrapperrors.String(err))
        println("error: "+err.Error())
    }

    println("the person e-mail is "+email)
}
```

If the person is not found, then the following output will be printed in the console.

```bash
error: the person could not be retrieve
```

And the following log is printed

```bash
the person could not be retrieve | [err_code: not_found] | SRC: .../persons.go:17 | CAUSE: {person has not been found in the database | [err_code: not_found] | SRC: .../db.go:45}
```

## Author

- **Felipe Moura** · [felipewom@gmail.com](mailto:felipewom@gmail.com)

## License

[MIT](https://github.com/felipewom/wrapperrors/blob/master/LICENSE)