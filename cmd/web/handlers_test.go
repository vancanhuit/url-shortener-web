package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	insertFn         func(ctx context.Context, url, alias string) (string, error)
	getOriginalURLFn func(ctx context.Context, alias string) (string, error)
}

func (m *mockRepo) Insert(ctx context.Context, url, alias string) (string, error) {
	return m.insertFn(ctx, url, alias)
}

func (m *mockRepo) GetOriginalURL(ctx context.Context, alias string) (string, error) {
	return m.getOriginalURLFn(ctx, alias)
}

func newTestEcho() *echo.Echo {
	e := echo.New()
	e.JSONSerializer = &CustomJSONSerializer{}
	e.Validator = &CustomValidator{
		Validator: validator.New(validator.WithRequiredStructEnabled()),
	}
	return e
}

func TestShortenSuccess(t *testing.T) {
	app := &Application{
		BaseURL: "http://localhost:8080",
		Logger:  slog.New(slog.DiscardHandler),
		Repo: &mockRepo{
			insertFn: func(_ context.Context, url, alias string) (string, error) {
				return alias, nil
			},
		},
	}

	e := newTestEcho()
	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("requestID", "test-req-id")

	err := app.Shorten(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["alias"])
	require.Contains(t, resp["short_url"], "http://localhost:8080/r/")
}

func TestShortenEmptyBody(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.ContentLength = 0
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := app.Shorten(c)
	require.Error(t, err)

	var he *echo.HTTPError
	require.True(t, errors.As(err, &he))
	require.Equal(t, http.StatusBadRequest, he.Code)
}

func TestShortenValidationError(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	body := `{"url":"not-a-url"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := app.Shorten(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "must be a valid HTTP(S) URL", resp["error"])
}

func TestShortenRepoError(t *testing.T) {
	app := &Application{
		BaseURL: "http://localhost:8080",
		Logger:  slog.New(slog.DiscardHandler),
		Repo: &mockRepo{
			insertFn: func(_ context.Context, _, _ string) (string, error) {
				return "", fmt.Errorf("db connection failed")
			},
		},
	}

	e := newTestEcho()
	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("requestID", "test-req-id")

	err := app.Shorten(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db connection failed")
}

func TestRedirectSuccess(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
		Repo: &mockRepo{
			getOriginalURLFn: func(_ context.Context, _ string) (string, error) {
				return "https://example.com", nil
			},
		},
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/r/abcdefghijk", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("alias")
	c.SetParamValues("abcdefghijk")

	err := app.Redirect(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "https://example.com", rec.Header().Get("Location"))
}

func TestRedirectInvalidAlias(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/r/short", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("alias")
	c.SetParamValues("short")

	err := app.Redirect(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRedirectNotFound(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
		Repo: &mockRepo{
			getOriginalURLFn: func(_ context.Context, _ string) (string, error) {
				return "", ErrRecordNotFound
			},
		},
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/r/abcdefghijk", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("alias")
	c.SetParamValues("abcdefghijk")

	err := app.Redirect(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "requested resource could not be found", resp["error"])
}

func TestRedirectRepoError(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
		Repo: &mockRepo{
			getOriginalURLFn: func(_ context.Context, _ string) (string, error) {
				return "", fmt.Errorf("db timeout")
			},
		},
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/r/abcdefghijk", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("alias")
	c.SetParamValues("abcdefghijk")

	err := app.Redirect(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db timeout")
}

func TestCustomHTTPErrorHandlerEchoError(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	app.CustomHTTPErrorHandler(echo.NewHTTPError(http.StatusBadRequest, "bad request"), c)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "bad request", resp["error"])
}

func TestCustomHTTPErrorHandlerNonEchoError(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	app.CustomHTTPErrorHandler(fmt.Errorf("unexpected error"), c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "internal server error", resp["error"])
}

func TestCustomHTTPErrorHandlerEntityTooLarge(t *testing.T) {
	app := &Application{
		Logger: slog.New(slog.DiscardHandler),
	}

	e := newTestEcho()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	app.CustomHTTPErrorHandler(echo.NewHTTPError(http.StatusRequestEntityTooLarge, "some internal message"), c)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "request entity too large", resp["error"])
}
