package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (app *Application) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", map[string]any{"version": version})
}

func (app *Application) Shorten(c echo.Context) error {
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "body is empty")
	}

	var request struct {
		URL string `json:"url" validate:"required,http_url,max=500"`
	}
	if err := c.Bind(&request); err != nil {
		return err
	}

	if err := c.Validate(request); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	}

	alias := GenerateAlias(request.URL)

	alias, err := app.Repo.Insert(c.Request().Context(), request.URL, alias)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"alias":     alias,
		"short_url": fmt.Sprintf("%s/r/%s", app.BaseURL, alias),
	})
}

func (app *Application) Redirect(c echo.Context) error {
	alias := c.Param("alias")
	if len(alias) != 11 || !isValidAlias(alias) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "requested resource could not be found"})
	}

	originalURL, err := app.Repo.GetOriginalURL(c.Request().Context(), alias)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "requested resource could not be found"})
		}
		return err
	}

	return c.Redirect(http.StatusSeeOther, originalURL)
}

func isValidAlias(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func (app *Application) CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var he *echo.HTTPError
	if !errors.As(err, &he) {
		he = echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	app.Logger.Error(err.Error())
	if he.Code == http.StatusRequestEntityTooLarge {
		he = echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request entity too large")
	}

	// Send JSON response
	if jsonErr := c.JSON(he.Code, map[string]string{"error": fmt.Sprintf("%v", he.Message)}); jsonErr != nil {
		app.Logger.Error("failed to send error response", "error", jsonErr)
	}
}
