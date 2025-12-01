package plugin

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is a collection of validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (v ValidationErrors) Error() string {
	var messages []string
	for _, err := range v.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// GetValidator returns a configured validator instance
func GetValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Register custom tag name function to use json tags for field names
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		jsonTag := fld.Tag.Get("json")
		// Extract just the field name, ignoring modifiers like "omitempty"
		// e.g., "region,omitempty" -> "region"
		if jsonTag == "" {
			return ""
		}
		parts := strings.Split(jsonTag, ",")
		return parts[0]
	})

	return v
}

// ValidateStruct validates a struct and returns user-friendly validation errors
func ValidateStruct(s interface{}) error {
	validate := GetValidator()
	err := validate.Struct(s)

	if err == nil {
		return nil
	}

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	var errors []ValidationError
	for _, e := range validationErrs {
		errors = append(errors, ValidationError{
			Field:   getJSONFieldName(e),
			Message: getErrorMessage(e),
		})
	}

	return ValidationErrors{Errors: errors}
}

// getJSONFieldName extracts the JSON field name from the validation error
func getJSONFieldName(e validator.FieldError) string {
	// Get the full namespace path
	namespace := e.Namespace()

	// Remove the root struct name (everything before the first dot)
	// e.g., "Config.external_links[0].url" -> "external_links[0].url"
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) > 1 {
		return parts[1]
	}

	// Fallback to field name if no namespace
	return e.Field()
}

// getErrorMessage converts validator tag to human-readable message
func getErrorMessage(e validator.FieldError) string {
	// Use the full field path for better error messages
	field := getJSONFieldName(e)

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "required_if":
		return fmt.Sprintf("%s is required", field)
	case "required_with":
		return fmt.Sprintf("%s is required when %s is specified", field, e.Param())
	case "min":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
		}
		return fmt.Sprintf("%s must be at least %s", field, e.Param())
	case "max":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
		}
		return fmt.Sprintf("%s must be at most %s", field, e.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "numeric":
		return fmt.Sprintf("%s must be numeric", field)
	case "alpha":
		return fmt.Sprintf("%s must contain only letters", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and numbers", field)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, e.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, e.Param())
	case "hostname":
		return fmt.Sprintf("%s must be a valid hostname", field)
	case "hostname_port":
		return fmt.Sprintf("%s must be a valid hostname:port", field)
	case "ip":
		return fmt.Sprintf("%s must be a valid IP address", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
