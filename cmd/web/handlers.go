package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (app *Application) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", nil)
}

func (app *Application) Shorten(c echo.Context) error {
	var request struct {
		URL string `json:"url" validate:"required,http_url,max=500"`
	}
	err := c.Bind(&request)
	if err != nil {
		return err
	} else if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "body is empty")
	}

	if err := c.Validate(request); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	}

	requestID := c.Request().Header.Get(echo.HeaderXRequestID)

	alias := GenerateAlias(request.URL, requestID)

	err = app.Repo.Insert(request.URL, alias)
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
	var originalURL string
	originalURL, err := app.Repo.GetOriginalURL(alias)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "requested resource could not be found"})
		}
		return err
	}

	return c.Redirect(http.StatusSeeOther, originalURL)
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
	c.JSON(he.Code, map[string]string{"error": fmt.Sprintf("%v", he.Message)}) //nolint:errcheck
}
