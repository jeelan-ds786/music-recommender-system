package preference

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

type ValidationError struct {
	Error string
	Field string
}

var validate = validator.New()

func ValidateStruct(v any) *ValidationError {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}

	validationErrors := err.(validator.ValidationErrors)

	ve := validationErrors[0]

	field := strings.ToLower(ve.Field())

	switch ve.Tag() {

	case "required":
		return &ValidationError{
			Error: strings.ToUpper(field) + "_REQUIRED",
			Field: field,
		}

	case "max":
		return &ValidationError{
			Error: strings.ToUpper(field) + "_TOO_LONG",
			Field: field,
		}
	}

	return &ValidationError{
		Error: "VALIDATION_ERROR",
		Field: field,
	}
}
