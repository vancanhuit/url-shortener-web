package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func randomDBName() string {
	var dbName [10]byte
	_, _ = rand.Read(dbName[:])
	return fmt.Sprintf("testdb_%x", dbName)
}

func createTestDB(t *testing.T, dbName string) string {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	db, err := openDB(dsn)
	require.NoError(t, err)

	_, err = db.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)
	t.Logf("Test database %q has been created successfully", dbName)
	t.Cleanup(func() {
		_, err := db.Exec("DROP DATABASE " + dbName)
		if err != nil {
			t.Logf("Failed to drop test database %q: %v", dbName, err)
		}
		t.Logf("Test database %q has been dropped successfully", dbName)
	})

	parsedURL, err := url.Parse(dsn)
	require.NoError(t, err)
	parsedURL.Path = dbName

	return parsedURL.String()
}

func newTestApp(t *testing.T) *Application {
	t.Helper()
	dbName := randomDBName()
	dsn := createTestDB(t, dbName)
	db, err := openDB(dsn)
	require.NoError(t, err)
	err = migrate(db)
	require.NoError(t, err)

	app := &Application{
		Repo: &Repo{
			DB: db,
		},
		Logger: slog.New(slog.DiscardHandler),
	}

	server := httptest.NewServer(app.routes())
	app.BaseURL = server.URL

	t.Cleanup(func() {
		server.Close() //nolint:errcheck
		db.Close()     //nolint:errcheck
	})

	return app
}

func TestAPIWithValidInput(t *testing.T) {
	app := newTestApp(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test shortening a URL
	resp, err := client.Post(app.BaseURL+"/api/shorten", echo.MIMEApplicationJSON, bytes.NewBufferString(`{"url":"http://example.com"}`))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, echo.MIMEApplicationJSON, resp.Header.Get(echo.HeaderContentType))

	var response struct {
		Alias    string `json:"alias"`
		ShortURL string `json:"short_url"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Check that the short URL is correctly formatted
	require.True(t, strings.HasPrefix(response.ShortURL, app.BaseURL+"/r/"))

	// Test redirecting to the original URL
	resp, err = client.Get(response.ShortURL)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "http://example.com", resp.Header.Get("Location"))

	alias := response.Alias

	// Test existing URL
	resp, err = client.Post(app.BaseURL+"/api/shorten", echo.MIMEApplicationJSON, bytes.NewBufferString(`{"url":"http://example.com"}`))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, echo.MIMEApplicationJSON, resp.Header.Get(echo.HeaderContentType))
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Check that the alias is the same
	require.Equal(t, alias, response.Alias)
}

func TestRedirectWithNonExistentURL(t *testing.T) {
	app := newTestApp(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(app.BaseURL + "/r/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	var response struct {
		Error string `json:"error"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	require.Equal(t, "requested resource could not be found", response.Error)
}

func TestAPIWithInvalidInput(t *testing.T) {
	// Setup test server
	app := newTestApp(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	b := make([]byte, 1024*1024)
	_, err := rand.Read(b)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		payload    string
		errorMsg   string
		statusCode int
	}{
		{
			name:       "empty body",
			payload:    ``,
			errorMsg:   "body is empty",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "syntax error",
			payload:    `{"url": https://example.com}`,
			errorMsg:   "body contains badly-formed JSON (at position 9)",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "incorrect value type",
			payload:    `{"url": 123}`,
			errorMsg:   "body contains an invalid value for the \"url\" field (at position 11)",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "incorrect json type",
			payload:    `[]`,
			errorMsg:   "body contains an invalid value (at position 1)",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "unexpected EOF",
			payload:    `{"url": "https://example.com`,
			errorMsg:   "body contains an unexpected end of JSON",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "unknown key",
			payload:    `{"url": "https://example.com", "foo": "bar"}`,
			errorMsg:   "body contains unknown key \"foo\"",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "multiple JSON values",
			payload:    `{"url": "https://example.com"}{"url": "https://example.com"}`,
			errorMsg:   "body must only contain a single JSON object",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "missing input",
			payload:    `{}`,
			errorMsg:   "'url' is required",
			statusCode: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid URL",
			payload:    `{"url": "invalid-url"}`,
			errorMsg:   "'url' must be a valid HTTP(S) URL",
			statusCode: http.StatusUnprocessableEntity,
		},
		{
			name:       "more than 500 bytes long",
			payload:    fmt.Sprintf(`{"url": "http://%x"}`, b[:500]),
			errorMsg:   "'url' must be at most 500 characters long",
			statusCode: http.StatusUnprocessableEntity,
		},
		{
			name:       "body limit exceeded",
			payload:    fmt.Sprintf(`{"url": "http://%x"}`, b),
			errorMsg:   "request entity too large",
			statusCode: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(app.BaseURL+"/api/shorten", "application/json", bytes.NewBufferString(tc.payload))
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			require.Equal(t, tc.statusCode, resp.StatusCode)

			var response struct {
				Error string `json:"error"`
			}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			require.Equal(t, tc.errorMsg, response.Error)
		})
	}
}
