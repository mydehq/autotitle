package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		shouldError bool
	}{
		{
			name:        "empty targets",
			cfg:         &Config{},
			shouldError: true,
		},
		{
			name: "missing path",
			cfg: &Config{
				Targets: []Target{{URL: "https://mal.net/anime/1"}},
			},
			shouldError: true,
		},
		{
			name: "missing url",
			cfg: &Config{
				Targets: []Target{{Path: "."}},
			},
			shouldError: true,
		},
		{
			name: "missing patterns",
			cfg: &Config{
				Targets: []Target{{Path: ".", URL: "https://mal.net/anime/1"}},
			},
			shouldError: true,
		},
		{
			name: "valid config",
			cfg: &Config{
				Targets: []Target{
					{
						Path: ".",
						URL:  "https://myanimelist.net/anime/1",
						Patterns: []Pattern{
							{
								Input:  []string{"Episode {{EP_NUM}}"},
								Output: OutputConfig{Fields: []string{"SERIES", "EP_NUM"}},
							},
						},
					},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if tt.shouldError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "_autotitle.yml")

	content := `targets:
  - path: "."
    url: "https://myanimelist.net/anime/12345"
    filler_url: "https://animefillerlist.com/shows/test"
    patterns:
      - input:
          - "Episode {{EP_NUM}}"
        output:
          fields: [SERIES, EP_NUM, EP_NAME]
          separator: " - "
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}

	target := cfg.Targets[0]
	if target.URL != "https://myanimelist.net/anime/12345" {
		t.Errorf("unexpected URL: %s", target.URL)
	}
	if target.FillerURL != "https://animefillerlist.com/shows/test" {
		t.Errorf("unexpected FillerURL: %s", target.FillerURL)
	}
}

func TestGenerateDefault(t *testing.T) {
	cfg := GenerateDefault(
		"https://myanimelist.net/anime/12345",
		"https://animefillerlist.com/shows/test",
		"Episode {{EP_NUM}}",
	)

	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}

	target := cfg.Targets[0]
	if target.Path != "." {
		t.Errorf("expected path '.', got %s", target.Path)
	}
	if target.URL != "https://myanimelist.net/anime/12345" {
		t.Errorf("unexpected URL: %s", target.URL)
	}
	if len(target.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(target.Patterns))
	}
}
