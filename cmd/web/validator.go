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
		if e.Tag() == "required" {
			return fmt.Errorf("'%s' is required", e.Field())
		}
		if e.Tag() == "http_url" {
			return fmt.Errorf("'%s' must be a valid HTTP(S) URL", e.Field())
		}
		if e.Tag() == "max" {
			return fmt.Errorf("'%s' must be at most %s characters long", e.Field(), e.Param())
		}

		return fmt.Errorf("validation failed for '%s': %s", e.Field(), e.Tag())
	}
	return nil
}
