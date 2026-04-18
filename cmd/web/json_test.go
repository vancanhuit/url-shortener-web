package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSerialize(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	err := s.Serialize(c, map[string]string{"key": "value"}, "")
	require.NoError(t, err)
	require.Contains(t, rec.Body.String(), `"key":"value"`)
}

func TestSerializeWithIndent(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	err := s.Serialize(c, map[string]string{"key": "value"}, "  ")
	require.NoError(t, err)
	require.Contains(t, rec.Body.String(), "  \"key\": \"value\"")
}

func TestDeserializeValid(t *testing.T) {
	e := echo.New()
	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.NoError(t, err)
	require.Equal(t, "https://example.com", target.URL)
}

func TestDeserializeSyntaxError(t *testing.T) {
	e := echo.New()
	body := `{"url": invalid}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "badly-formed JSON")
}

func TestDeserializeTypeMismatch(t *testing.T) {
	e := echo.New()
	body := `{"url": 123}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "invalid value for the \"url\" field")
}

func TestDeserializeTypeMismatchNoField(t *testing.T) {
	e := echo.New()
	body := `[]`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "invalid value")
}

func TestDeserializeUnknownField(t *testing.T) {
	e := echo.New()
	body := `{"url":"https://example.com","foo":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "unknown key")
}

func TestDeserializeUnexpectedEOF(t *testing.T) {
	e := echo.New()
	body := `{"url": "https://example.com`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "unexpected end of JSON")
}

func TestDeserializeMultipleJSONValues(t *testing.T) {
	e := echo.New()
	body := `{"url":"https://example.com"}{"url":"https://other.com"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := CustomJSONSerializer{}
	var target struct {
		URL string `json:"url"`
	}
	err := s.Deserialize(c, &target)
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusBadRequest, he.Code)
	require.Contains(t, he.Message.(error).Error(), "single JSON object")
}
