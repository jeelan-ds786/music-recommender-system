package profile

import (
	"strings"
	"time"

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

	case "max":
		return &ValidationError{
			Error: strings.ToUpper(field) + "_TOO_LONG",
			Field: field,
		}

	case "url":
		return &ValidationError{
			Error: "AVATAR_URL_INVALID",
			Field: field,
		}

	case "len", "iso3166_1_alpha2":
		return &ValidationError{
			Error: "COUNTRY_INVALID",
			Field: field,
		}
	}

	return &ValidationError{
		Error: "VALIDATION_ERROR",
		Field: field,
	}
}

// ValidateBirthYear is checked separately from ValidateStruct: no
// go-playground/validator tag can bound against the current year (tags are
// static strings, and a hardcoded max would go stale), so this is a plain
// bounds check instead of a registered custom validator function. Callers
// must only invoke it when year is non-nil — nil means the field was
// omitted, not "year 0".
func ValidateBirthYear(year *int16) *ValidationError {
	if year == nil {
		return nil
	}

	y := int(*year)
	if y < 1900 || y > time.Now().Year() {
		return &ValidationError{
			Error: "BIRTH_YEAR_INVALID",
			Field: "birth_year",
		}
	}

	return nil
}
