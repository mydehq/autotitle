package fetcher_test

import (
	"testing"

	"github.com/soymadip/autotitle/internal/fetcher"
)

func TestExtractMALID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{
			name: "Standard MAL URL",
			url:  "https://myanimelist.net/anime/235/Detective_Conan",
			want: 235,
		},
		{
			name: "MAL URL with trailing slash",
			url:  "https://myanimelist.net/anime/779/",
			want: 779,
		},
		{
			name: "HTTP URL",
			url:  "http://myanimelist.net/anime/12345/Test",
			want: 12345,
		},
		{
			name: "Invalid URL - no ID",
			url:  "https://myanimelist.net/anime/",
			want: 0,
		},
		{
			name: "Invalid URL - not MAL",
			url:  "https://example.com/anime/123",
			want: 0,
		},
		{
			name: "Empty string",
			url:  "",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetcher.ExtractMALID(tt.url)
			if got != tt.want {
				t.Errorf("ExtractMALID(%q) = %d; want %d", tt.url, got, tt.want)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "Simple title",
			title: "Detective Conan",
			want:  "detective-conan",
		},
		{
			name:  "Title with special characters",
			title: "Attack on Titan!",
			want:  "attack-on-titan",
		},
		{
			name:  "Title with multiple spaces",
			title: "One  Piece   Adventure",
			want:  "one-piece-adventure",
		},
		{
			name:  "Title with parentheses",
			title: "Fullmetal Alchemist (2003)",
			want:  "fullmetal-alchemist-2003",
		},
		{
			name:  "Title with colons",
			title: "Re:Zero - Starting Life in Another World",
			want:  "rezero-starting-life-in-another-world",
		},
		{
			name:  "Title already lowercase",
			title: "death note",
			want:  "death-note",
		},
		{
			name:  "Japanese characters removed",
			title: "名探偵コナン Detective Conan",
			want:  "detective-conan",
		},
		{
			name:  "Mixed case and numbers",
			title: "Code Geass R2",
			want:  "code-geass-r2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetcher.GenerateSlug(tt.title)
			if got != tt.want {
				t.Errorf("GenerateSlug(%q) = %q; want %q", tt.title, got, tt.want)
			}
		})
	}
}
