package types

import (
	"testing"
)

func TestMedia_GetTitle(t *testing.T) {
	tests := []struct {
		name     string
		media    Media
		variant  string
		expected string
	}{
		{
			name: "returns Japanese title when available",
			media: Media{
				Title:   "Shingeki no Kyojin",
				TitleEN: "Attack on Titan",
				TitleJP: "進撃の巨人",
			},
			variant:  "SERIES_JP",
			expected: "進撃の巨人",
		},
		{
			name: "falls back to default when JP empty",
			media: Media{
				Title:   "Attack on Titan",
				TitleEN: "Attack on Titan",
				TitleJP: "",
			},
			variant:  "SERIES_JP",
			expected: "Attack on Titan",
		},
		{
			name: "returns English title when available",
			media: Media{
				Title:   "Shingeki no Kyojin",
				TitleEN: "Attack on Titan",
				TitleJP: "進撃の巨人",
			},
			variant:  "SERIES_EN",
			expected: "Attack on Titan",
		},
		{
			name: "falls back to default when EN empty",
			media: Media{
				Title:   "Shingeki no Kyojin",
				TitleEN: "",
				TitleJP: "進撃の巨人",
			},
			variant:  "SERIES_EN",
			expected: "Shingeki no Kyojin",
		},
		{
			name: "returns default for unknown variant",
			media: Media{
				Title:   "Shingeki no Kyojin",
				TitleEN: "Attack on Titan",
				TitleJP: "進撃の巨人",
			},
			variant:  "SERIES",
			expected: "Shingeki no Kyojin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.media.GetTitle(tt.variant)
			if got != tt.expected {
				t.Errorf("GetTitle(%q) = %q, want %q", tt.variant, got, tt.expected)
			}
		})
	}
}

func TestMedia_GetEpisode(t *testing.T) {
	media := Media{
		Episodes: []Episode{
			{Number: 1, Title: "Episode 1"},
			{Number: 2, Title: "Episode 2"},
			{Number: 3, Title: "Episode 3"},
		},
	}

	t.Run("finds existing episode", func(t *testing.T) {
		ep := media.GetEpisode(2)
		if ep == nil {
			t.Fatal("expected episode 2, got nil")
		}
		if ep.Title != "Episode 2" {
			t.Errorf("expected 'Episode 2', got %q", ep.Title)
		}
	})

	t.Run("returns nil for non-existent episode", func(t *testing.T) {
		ep := media.GetEpisode(999)
		if ep != nil {
			t.Errorf("expected nil, got %v", ep)
		}
	})
}
