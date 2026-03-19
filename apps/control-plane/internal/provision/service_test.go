package provision

import (
	"strings"
	"testing"
)

func TestNormalizeInstallPath_ValidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty becomes root", input: "", want: "/"},
		{name: "root stays root", input: "/", want: "/"},
		{name: "trimmed empty becomes root", input: "   ", want: "/"},
		{name: "plain segment gets leading slash", input: "blog", want: "/blog"},
		{name: "trailing slash removed", input: "/blog/", want: "/blog"},
		{name: "nested segments", input: "blog/news", want: "/blog/news"},
		{name: "multiple slashes normalized", input: "/blog//news///", want: "/blog/news"},
		{name: "whitespace trimmed", input: "  /shop/deals/  ", want: "/shop/deals"},
		{name: "windows separators normalized", input: `\blog\news`, want: "/blog/news"},
		{name: "underscore and dash allowed", input: "/my_site/v2-blog", want: "/my_site/v2-blog"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeInstallPath(tt.input)
			if err != nil {
				t.Fatalf("normalizeInstallPath(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeInstallPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeInstallPath_RejectsTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "dot segment", input: "./blog"},
		{name: "parent segment", input: "../blog"},
		{name: "embedded parent segment", input: "/shop/../admin"},
		{name: "windows parent segment", input: `\shop\..\admin`},
		{name: "only parent segment", input: ".."},
		{name: "parent at end", input: "/shop/.."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeInstallPath(tt.input)
			if err == nil {
				t.Fatalf("normalizeInstallPath(%q) expected error, got nil", tt.input)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "traversal") {
				t.Fatalf("normalizeInstallPath(%q) error = %q, expected traversal-related error", tt.input, err.Error())
			}
		})
	}
}

func TestNormalizeInstallPath_RejectsInvalidCharacters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"/blog?x=1",
		"/blog/.well-known",
		"/my blog",
		"/blog#anchor",
		"/café",
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeInstallPath(input)
			if err == nil {
				t.Fatalf("normalizeInstallPath(%q) expected error, got nil", input)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "invalid characters") {
				t.Fatalf("normalizeInstallPath(%q) error = %q, expected invalid characters error", input, err.Error())
			}
		})
	}
}
