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
          offset: 10
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
	if len(target.Patterns) == 0 {
		t.Fatal("expected at least one pattern")
	}
	if target.Patterns[0].Output.Offset != 10 {
		t.Errorf("unexpected Offset: %d", target.Patterns[0].Output.Offset)
	}
}

func TestGenerateDefault(t *testing.T) {
	cfg := GenerateDefault(
		"https://myanimelist.net/anime/12345",
		"https://animefillerlist.com/shows/test",
		[]string{"Episode {{EP_NUM}}"},
		"",
		0,
		0,
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
	if target.FillerURL != "https://animefillerlist.com/shows/test" {
		t.Errorf("unexpected FillerURL: %s", target.FillerURL)
	}
	if len(target.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(target.Patterns))
	}
}

func TestGenerateDefaultEmptyFiller(t *testing.T) {
	cfg := GenerateDefault(
		"https://myanimelist.net/anime/12345",
		"",
		[]string{"Episode {{EP_NUM}}"},
		"",
		0,
		0,
	)

	if cfg.Targets[0].FillerURL != "" {
		t.Errorf("expected empty FillerURL, got %s", cfg.Targets[0].FillerURL)
	}
}

func TestGenerateDefaultSideEffects(t *testing.T) {
	// Capture initial state of globals
	initialURL := defaultMapFile.Targets[0].URL
	initialPatterns := len(defaults.Patterns)
	initialSeparator := defaults.Patterns[0].Output.Separator

	// Call GenerateDefault with overrides
	cfg1 := GenerateDefault("https://override-url.com", "", nil, "|", 5, 3)

	// Verify cfg1 is correct
	if cfg1.Targets[0].URL != "https://override-url.com" {
		t.Errorf("cfg1 URL not overridden: %s", cfg1.Targets[0].URL)
	}
	if cfg1.Targets[0].Patterns[0].Output.Separator != "|" {
		t.Errorf("cfg1 separator not overridden: %s", cfg1.Targets[0].Patterns[0].Output.Separator)
	}

	// Verify globals are UNCHANGED
	if defaultMapFile.Targets[0].URL != initialURL {
		t.Errorf("defaultMapFile.URL mutated! expected %s, got %s", initialURL, defaultMapFile.Targets[0].URL)
	}
	if len(defaults.Patterns) != initialPatterns {
		t.Errorf("defaults.Patterns length mutated! expected %d, got %d", initialPatterns, len(defaults.Patterns))
	}
	if defaults.Patterns[0].Output.Separator != initialSeparator {
		t.Errorf("defaults.Patterns separator mutated! expected %s, got %s", initialSeparator, defaults.Patterns[0].Output.Separator)
	}

	// Second call with different overrides
	cfg2 := GenerateDefault("https://another-url.com", "", nil, ":", 0, 0)
	if cfg2.Targets[0].URL != "https://another-url.com" {
		t.Errorf("cfg2 URL not overridden: %s", cfg2.Targets[0].URL)
	}
	if cfg2.Targets[0].Patterns[0].Output.Separator != ":" {
		t.Errorf("cfg2 separator not overridden: %s", cfg2.Targets[0].Patterns[0].Output.Separator)
	}

	// Ensure cfg1 was not affected by cfg2
	if cfg1.Targets[0].URL != "https://override-url.com" {
		t.Errorf("cfg1 URL modified by cfg2: %s", cfg1.Targets[0].URL)
	}
	if cfg1.Targets[0].Patterns[0].Output.Separator != "|" {
		t.Errorf("cfg1 separator modified by cfg2: %s", cfg1.Targets[0].Patterns[0].Output.Separator)
	}
}

func TestGenerateDefaultFieldsSideEffects(t *testing.T) {
	// Call GenerateDefault
	cfg1 := GenerateDefault("", "", nil, "", 0, 0)

	// Modify fields in cfg1
	cfg1.Targets[0].Patterns[0].Output.Fields[0] = "MODIFIED"

	// Call GenerateDefault again
	cfg2 := GenerateDefault("", "", nil, "", 0, 0)

	// Verify cfg2 has original fields, not MODIFIED
	if cfg2.Targets[0].Patterns[0].Output.Fields[0] == "MODIFIED" {
		t.Error("cfg2 affected by cfg1 modification! Fields slice was not deep-copied.")
	}

	// Verify global defaultMapFile is unchanged
	if defaultMapFile.Targets[0].Patterns[0].Output.Fields[0] == "MODIFIED" {
		t.Error("defaultMapFile affected by cfg1 modification! Global Fields slice was mutated.")
	}
}
