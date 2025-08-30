package main

import (
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
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
	e.Use(middleware.ContextTimeout(60 * time.Second))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:       true,
		LogStatus:    true,
		LogLatency:   true,
		LogProtocol:  true,
		LogRemoteIP:  true,
		LogHost:      true,
		LogMethod:    true,
		LogRequestID: true,
		LogUserAgent: true,
		HandleError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			app.Logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "REQUEST",
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.String("protocol", v.Protocol),
				slog.String("remote_ip", v.RemoteIP),
				slog.String("host", v.Host),
				slog.String("user_agent", v.UserAgent),
				slog.String("request_id", v.RequestID),
				slog.String("latency", v.Latency.String()),
			)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("1M"))
	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Skipper: middleware.DefaultSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(100),
			Burst:     200,
			ExpiresIn: 1 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			ip := c.RealIP()
			return ip, nil
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "too many requests")
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		},
	}))

	e.POST("/api/shorten", app.Shorten)
	e.GET("/r/:alias", app.Redirect)

	return e
}
