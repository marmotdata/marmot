package user

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	validator "github.com/go-playground/validator/v10"
)

// FieldError is one field that failed validation, named as the request body
// names it.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// InvalidInputError reports which fields were rejected and why, so a request
// missing a required field comes back as a 400 naming the field rather than an
// opaque 500.
type InvalidInputError struct {
	Fields []FieldError
}

func (e *InvalidInputError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+" "+f.Message)
	}
	return ErrInvalidInput.Error() + ": " + strings.Join(parts, ", ")
}

// Unwrap keeps errors.Is(err, ErrInvalidInput) true for callers that only care
// that the input was bad.
func (e *InvalidInputError) Unwrap() error { return ErrInvalidInput }

// newValidator takes field names from the json tags, so an error names the
// field the caller sent (role_names) rather than the Go one (RoleNames).
func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	return v
}

// invalidInput turns a validator failure into an InvalidInputError. Anything
// else is wrapped unchanged, so an internal problem is not relabelled as the
// caller's fault.
func invalidInput(err error) error {
	var failures validator.ValidationErrors
	if !errors.As(err, &failures) {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	fields := make([]FieldError, 0, len(failures))
	for _, f := range failures {
		fields = append(fields, FieldError{Field: f.Field(), Message: ruleMessage(f)})
	}
	return &InvalidInputError{Fields: fields}
}

// ruleMessage renders a failed rule as something the caller can act on.
//
// The cases are the complete set of rules the inputs in this package declare —
// see the validate tags on CreateUserInput and UpdateUserInput: required,
// required_without, min, max and email. omitempty is absent because it skips
// rather than fails. A rule added later falls to the default and should get a
// case of its own.
func ruleMessage(f validator.FieldError) string {
	switch f.Tag() {
	case "required":
		return "is required"
	case "required_without":
		return "is required unless " + f.Param() + " is set"
	case "min":
		switch {
		case !countsEntries(f):
			return "must be at least " + f.Param() + " characters"
		case f.Param() == "1":
			return "must not be empty"
		default:
			return "must have at least " + f.Param() + " entries"
		}
	case "max":
		if countsEntries(f) {
			return "must have at most " + f.Param() + " entries"
		}
		return "must be at most " + f.Param() + " characters"
	case "email":
		return "must be an email address"
	default:
		return "failed the " + f.Tag() + " rule"
	}
}

// countsEntries reports whether a min/max parameter counts entries rather than
// characters: min=1 on role_names means one role, not one character.
func countsEntries(f validator.FieldError) bool {
	switch f.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return true
	default:
		return false
	}
}
