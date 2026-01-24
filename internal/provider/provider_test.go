package provider

import (
	"testing"
)

func TestMALProvider_MatchesURL(t *testing.T) {
	p := NewMALProvider(nil)

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://myanimelist.net/anime/16498/Shingeki_no_Kyojin", true},
		{"https://myanimelist.com/anime/16498/Shingeki_no_Kyojin", true},
		{"https://anilist.co/anime/16498/Shingeki-no-Kyojin", false},
		{"https://themoviedb.org/tv/1234", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := p.MatchesURL(tt.url)
			if got != tt.expected {
				t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestMALProvider_ExtractID(t *testing.T) {
	p := NewMALProvider(nil)

	tests := []struct {
		url         string
		expectedID  string
		shouldError bool
	}{
		{"https://myanimelist.net/anime/16498/Shingeki_no_Kyojin", "16498", false},
		{"https://myanimelist.com/anime/1/Cowboy_Bebop", "1", false},
		{"https://myanimelist.net/anime/12345", "12345", false},
		{"https://anilist.co/anime/16498", "", true},
		{"invalid-url", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			id, err := p.ExtractID(tt.url)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if id != tt.expectedID {
					t.Errorf("ExtractID(%q) = %q, want %q", tt.url, id, tt.expectedID)
				}
			}
		})
	}
}

func TestGetProviderForURL(t *testing.T) {
	tests := []struct {
		url          string
		expectedName string
		shouldError  bool
	}{
		{"https://myanimelist.net/anime/16498/Shingeki_no_Kyojin", "mal", false},
		{"https://themoviedb.org/tv/1234", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			p, err := GetProviderForURL(tt.url)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if p.Name() != tt.expectedName {
					t.Errorf("expected provider %q, got %q", tt.expectedName, p.Name())
				}
			}
		})
	}
}
