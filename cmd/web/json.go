package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type CustomJSONSerializer struct{}

func (d CustomJSONSerializer) Serialize(c echo.Context, i any, indent string) error {
	enc := json.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

func (d CustomJSONSerializer) Deserialize(c echo.Context, i any) error {
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(i)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body contains badly-formed JSON (at position %d)", syntaxError.Offset)).SetInternal(err)
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field == "" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body contains an invalid value (at position %d)", unmarshalTypeError.Offset)).SetInternal(err)
			}
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)).SetInternal(err)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body contains unknown key %s", fieldName))
		case errors.Is(err, io.ErrUnexpectedEOF):
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body contains an unexpected end of JSON")).SetInternal(err)
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("body must only contain a single JSON object")).SetInternal(err)
	}

	return nil
}
