package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the system-wide or user-specific global configuration.
type GlobalConfig struct {
	MapFile  string       `yaml:"map_file"`
	Patterns []Pattern    `yaml:"patterns"`
	Output   string       `yaml:"output"` // Fallback output pattern if no patterns match
	Formats  []string     `yaml:"formats"`
	API      APIConfig    `yaml:"api"`
	Backup   BackupConfig `yaml:"backup"`
}

type APIConfig struct {
	RateLimit int `yaml:"rate_limit"`
	Timeout   int `yaml:"timeout"`
}

type BackupConfig struct {
	DirName string `yaml:"dir_name"`
}

// MapConfig represents the per-directory configuration file.
type MapConfig struct {
	Targets []Target `yaml:"targets"`
}

type Target struct {
	Path     string    `yaml:"path"`
	ID       string    `yaml:"id"`
	Extends  string    `yaml:"extends"`
	MALURL   string    `yaml:"mal_url"`
	AFLURL   string    `yaml:"afl_url"`
	Patterns []Pattern `yaml:"patterns"`
}

type Pattern struct {
	Input  []string `yaml:"input"`
	Output string   `yaml:"output"`
}

// DefaultGlobalConfig returns the hardcoded default configuration.
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		MapFile: "_autotitle.yml",
		Formats: []string{"mkv", "mp4", "avi", "webm", "m4v", "ts", "flv"},
		Output:  "{{SERIES}} {{EP_NUM}} {{FILLER}} - {{EP_NAME}}.{{EXT}}",
		API: APIConfig{
			RateLimit: 1,
			Timeout:   30,
		},
		Backup: BackupConfig{
			DirName: ".autotitle_backup",
		},
	}
}

// LoadGlobal loads the global configuration from the specified path or standard locations.
func LoadGlobal(customPath string) (*GlobalConfig, error) {
	cfg := DefaultGlobalConfig()

	// 1. Determine config path
	path := customPath
	if path == "" {
		path = findGlobalConfig()
	}

	if path == "" {
		// No config found, return defaults
		return &cfg, nil
	}

	// 2. Read and parse
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config at %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global config at %s: %w", path, err)
	}

	return &cfg, nil
}

// findGlobalConfig searches for the global config file in standard locations.
func findGlobalConfig() string {
	// XDG_CONFIG_HOME
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			xdgConfig = filepath.Join(home, ".config")
		}
	}

	if xdgConfig != "" {
		path := filepath.Join(xdgConfig, "autotitle", "config.yml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// /etc fallback
	etcPath := "/etc/autotitle/config.yml"
	if _, err := os.Stat(etcPath); err == nil {
		return etcPath
	}

	return ""
}

// LoadMap loads the map file from the specified directory.
// It also checks for legacy single-target map files and converts them if necessary.
func LoadMap(dir, filename string) (*MapConfig, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		// Try .yaml extension if .yml not found (or vice-versa logic implicit if filename comes from config)
		// Actually config says explicit filename, so we just try that.
		// Use os.IsNotExist to return nil, nil if it's just missing?
		// No, caller needs to know it's missing to possibly run 'init' logic or error out.
		return nil, err
	}

	var mapCfg MapConfig
	// First try unmarshalling as the new multi-target format
	if err := yaml.Unmarshal(data, &mapCfg); err == nil && len(mapCfg.Targets) > 0 {
		return &mapCfg, nil
	}

	// If that failed or resulted in empty targets, try legacy/single-target format
	// This supports the user's initial desire for a simple file
	var singleTarget struct {
		MALURL   string    `yaml:"mal_url"`
		AFLURL   string    `yaml:"afl_url"`
		Patterns []Pattern `yaml:"patterns"`
	}

	if err := yaml.Unmarshal(data, &singleTarget); err != nil {
		return nil, fmt.Errorf("failed to parse map file at %s: %w", path, err)
	}

	// Convert to Target
	t := Target{
		Path:     ".",
		MALURL:   singleTarget.MALURL,
		AFLURL:   singleTarget.AFLURL,
		Patterns: singleTarget.Patterns,
	}

	// Check for required URL
	if t.MALURL == "" {
		return nil, fmt.Errorf("invalid map file: missing mal_url")
	}

	mapCfg.Targets = []Target{t}
	return &mapCfg, nil
}

// ResolveTarget returns the configuration for a specific directory by resolving inheritance.
func (mc *MapConfig) ResolveTarget(dir string) (*Target, error) {
	// 1. Find the target for this path (handling relative paths)
	// For now, simple string matching on "path". A more robust implementation involves checking filepath.Rel
	var target *Target
	for i := range mc.Targets {
		// Clean paths for comparison
		tPath := filepath.Clean(mc.Targets[i].Path)
		dPath := filepath.Clean(dir)

		// If path is ".", it matches the base dir where the map file is loaded from
		// We are assuming 'dir' passed here is relative to map file location?
		// Actually, LoadMap took 'dir'. So 'Path' in target is relative to 'dir'.

		if tPath == dPath || (tPath == "." && (dPath == "." || dPath == "")) {
			target = &mc.Targets[i]
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("no target config found for directory: %s", dir)
	}

	// 2. Handle matches
	if target.Extends != "" {
		parent := mc.findTargetByID(target.Extends)
		if parent == nil {
			return nil, fmt.Errorf("target '%s' extends unknown id '%s'", target.Path, target.Extends)
		}
		// 4. Merge fields (target overrides parent)
		merged := Target{
			Path:     target.Path,
			ID:       target.ID,
			Extends:  target.Extends,
			MALURL:   parent.MALURL,
			AFLURL:   parent.AFLURL,
			Patterns: parent.Patterns, // Start with parent patterns
		}

		// Override MALURL and AFLURL if target has non-empty values
		if target.MALURL != "" {
			merged.MALURL = target.MALURL
		}
		if target.AFLURL != "" {
			merged.AFLURL = target.AFLURL
		}

		// Override patterns if provided (not merge, full replace)
		if len(target.Patterns) > 0 {
			merged.Patterns = target.Patterns
		}

		merged.Path = target.Path
		merged.Extends = target.Extends

		return &merged, nil
	}

	return target, nil
}

func (mc *MapConfig) findTargetByID(id string) *Target {
	for i := range mc.Targets {
		if mc.Targets[i].ID == id {
			return &mc.Targets[i]
		}
	}
	return nil
}
