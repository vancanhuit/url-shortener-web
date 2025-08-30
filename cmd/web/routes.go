package main

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (app *Application) routes() http.Handler {
	e := echo.New()

	e.JSONSerializer = &CustomJSONSerializer{}
	e.HTTPErrorHandler = app.CustomHTTPErrorHandler

	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	e.Validator = &CustomValidator{Validator: validate}

	e.Use(middleware.RequestID())
	e.Use(middleware.BodyLimit("1M"))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/api/shorten", app.Shorten)
	e.GET("/r/:alias", app.Redirect)

	return e
}
