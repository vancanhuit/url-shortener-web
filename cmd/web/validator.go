package main

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type CustomValidator struct {
	Validator *validator.Validate
}

func (cv *CustomValidator) Validate(i any) error {
	err := cv.Validator.Struct(i)
	if err != nil {
		if err, ok := err.(*validator.InvalidValidationError); ok {
			panic(err)
		}

		e := err.(validator.ValidationErrors)[0]
		if e.Tag() == "required" && e.Field() == "URL" {
			return fmt.Errorf("missing url")
		}
		if e.Tag() == "http_url" {
			return fmt.Errorf("must be a valid HTTP(S) URL")
		}
		if e.Tag() == "max" {
			return fmt.Errorf("must be at most %s characters long", e.Param())
		}

		return fmt.Errorf("validation failed for '%s': %s", e.Field(), e.Tag())
	}
	return nil
}
