package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAlias(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "basic URL",
			url:  "https://example.com",
		},
		{
			name: "URL with path",
			url:  "https://example.com/some/long/path?q=1&r=2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alias := GenerateAlias(tc.url)
			require.Len(t, alias, 11)
			require.True(t, isValidAlias(alias))

			alias2 := GenerateAlias(tc.url)
			require.Equal(t, alias, alias2)
		})
	}
}

func TestGenerateAliasDifferentURLs(t *testing.T) {
	alias1 := GenerateAlias("https://example.com")
	alias2 := GenerateAlias("https://other.com")
	require.NotEqual(t, alias1, alias2)
}

func TestIsValidAlias(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		valid bool
	}{
		{"valid alphanumeric", "abcABC12345", true},
		{"valid with dash", "abc-def-123", true},
		{"valid with underscore", "abc_def_123", true},
		{"contains space", "abc def 123", false},
		{"contains slash", "abc/def/123", false},
		{"contains special char", "abc!def@123", false},
		{"empty string", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.valid, isValidAlias(tc.alias))
		})
	}
}
