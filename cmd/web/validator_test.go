package main

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func newTestValidator() *CustomValidator {
	return &CustomValidator{
		Validator: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func TestValidatorValid(t *testing.T) {
	cv := newTestValidator()
	input := struct {
		URL string `validate:"required,http_url,max=500"`
	}{
		URL: "https://example.com",
	}

	err := cv.Validate(input)
	require.NoError(t, err)
}

func TestValidatorMissingURL(t *testing.T) {
	cv := newTestValidator()
	input := struct {
		URL string `validate:"required,http_url,max=500"`
	}{}

	err := cv.Validate(input)
	require.Error(t, err)
	require.Equal(t, "missing url", err.Error())
}

func TestValidatorInvalidURL(t *testing.T) {
	cv := newTestValidator()
	input := struct {
		URL string `validate:"required,http_url,max=500"`
	}{
		URL: "not-a-url",
	}

	err := cv.Validate(input)
	require.Error(t, err)
	require.Equal(t, "must be a valid HTTP(S) URL", err.Error())
}

func TestValidatorTooLong(t *testing.T) {
	cv := newTestValidator()
	longURL := "https://example.com/" + strings.Repeat("a", 500)
	input := struct {
		URL string `validate:"required,http_url,max=500"`
	}{
		URL: longURL,
	}

	err := cv.Validate(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be at most 500 characters long")
}

func TestValidatorFallbackError(t *testing.T) {
	cv := newTestValidator()
	input := struct {
		Email string `validate:"required,email"`
	}{
		Email: "not-an-email",
	}

	err := cv.Validate(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation failed for")
}
