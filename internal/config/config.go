// Package config handles autotitle configuration loading and validation.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mydehq/autotitle/internal/types"
	"gopkg.in/yaml.v3"
)

// Config represents the autotitle configuration file
type Config struct {
	Targets []Target `yaml:"targets"`
	BaseDir string   `yaml:"-"`
}

// Target represents a rename target in the configuration
type Target struct {
	Path      string    `yaml:"path"`
	URL       string    `yaml:"url"`                  // Provider URL (MAL, TMDB, etc.)
	FillerURL string    `yaml:"filler_url,omitempty"` // Optional filler source URL
	Patterns  []Pattern `yaml:"patterns"`
}

// Pattern represents input/output pattern configuration
type Pattern struct {
	Input  []string     `yaml:"input"`
	Output OutputConfig `yaml:"output"`
}

// OutputConfig represents output format configuration
type OutputConfig struct {
	Fields    []string `yaml:"fields"`
	Separator string   `yaml:"separator,omitempty"`
}

// GlobalConfig represents the global configuration file (~/.config/autotitle/config.yml)
type GlobalConfig struct {
	MapFile  string             `yaml:"map_file"`
	Patterns []Pattern          `yaml:"patterns"`
	Formats  []string           `yaml:"formats"`
	API      types.APIConfig    `yaml:"api"`
	Backup   types.BackupConfig `yaml:"backup"`
}

// DefaultMapFileName is the default name for the map file
const DefaultMapFileName = "_autotitle.yml"
const GlobalConfigFileName = "config.yml"

// DefaultFormats is the default list of video extensions
var DefaultFormats = []string{"mkv", "mp4", "avi", "webm", "m4v", "ts", "flv"}

// Load loads configuration from a directory
func Load(dir string) (*Config, error) {
	// Try to get map file name from global config
	mapFileName := DefaultMapFileName
	if globalCfg, err := LoadGlobal(); err == nil && globalCfg.MapFile != "" {
		mapFileName = globalCfg.MapFile
	}

	// Try primary path first
	path := filepath.Join(dir, mapFileName)
	if _, err := os.Stat(path); err == nil {
		return LoadFile(path)
	}

	// Try alternate extension (.yml <-> .yaml)
	altPath := swapYAMLExtension(path)
	if _, err := os.Stat(altPath); err == nil {
		return LoadFile(altPath)
	}

	// Return error for primary path
	return LoadFile(path)
}

// swapYAMLExtension swaps .yml to .yaml and vice versa
func swapYAMLExtension(path string) string {
	if strings.HasSuffix(path, ".yml") {
		return strings.TrimSuffix(path, ".yml") + ".yaml"
	}
	if strings.HasSuffix(path, ".yaml") {
		return strings.TrimSuffix(path, ".yaml") + ".yml"
	}
	return path
}

// LoadFile loads configuration from a specific file path
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read map file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse map file: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}
	cfg.BaseDir = filepath.Dir(absPath)

	return &cfg, nil
}

// LoadGlobal loads the global configuration
func LoadGlobal() (*GlobalConfig, error) {
	// Paths to check in order
	paths := []string{}

	// 1. ~/.config/autotitle/config.yml (and .yaml)
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "autotitle", "config.yml"))
		paths = append(paths, filepath.Join(home, ".config", "autotitle", "config.yaml"))
	}

	// 2. /etc/autotitle/config.yml (and .yaml) (Linux/Unix)
	paths = append(paths, filepath.Join("/etc", "autotitle", "config.yml"))
	paths = append(paths, filepath.Join("/etc", "autotitle", "config.yaml"))

	var configPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}

	// Default values
	cfg := &GlobalConfig{
		MapFile: DefaultMapFileName,
		Formats: DefaultFormats,
		Patterns: []Pattern{
			{
				Input: []string{"{{EP_NUM}}.{{EXT}}", "Episode {{EP_NUM}}.{{EXT}}", "E{{EP_NUM}}.{{EXT}}"},
				Output: OutputConfig{
					Fields:    []string{"SERIES", "EP_NUM", "FILLER", "EP_NAME"},
					Separator: " - ",
				},
			},
		},
		API: types.APIConfig{
			RateLimit: 2.0, // Default 2 requests/sec
			Timeout:   30,  // Default 30s
		},
		Backup: types.BackupConfig{
			Enabled: true,
			DirName: "backups",
		},
	}

	if configPath == "" {
		return cfg, nil // Return defaults if no config found
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global config: %w", err)
	}

	return cfg, nil
}

// Save saves configuration to a file
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write map file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func Validate(cfg *Config) error {
	if len(cfg.Targets) == 0 {
		return fmt.Errorf("config must have at least one target")
	}

	for i, target := range cfg.Targets {
		if target.Path == "" {
			return fmt.Errorf("target %d: path is required", i)
		}
		if target.URL == "" {
			return fmt.Errorf("target %d: url is required", i)
		}
		if len(target.Patterns) == 0 {
			return fmt.Errorf("target %d: at least one pattern is required", i)
		}

		for j, pattern := range target.Patterns {
			if len(pattern.Input) == 0 {
				return fmt.Errorf("target %d, pattern %d: at least one input pattern is required", i, j)
			}
			if len(pattern.Output.Fields) == 0 {
				return fmt.Errorf("target %d, pattern %d: output fields are required", i, j)
			}
		}
	}

	return nil
}

// ResolveTarget finds the target configuration for a given path
func (c *Config) ResolveTarget(path string) (*Target, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	for i := range c.Targets {
		targetPath := c.Targets[i].Path
		if !filepath.IsAbs(targetPath) {
			// Resolve relative to map file location
			targetPath = filepath.Join(c.BaseDir, targetPath)
		}

		// Check if paths resolve to the same location
		tAbs, err := filepath.Abs(targetPath)
		if err == nil && tAbs == absPath {
			return &c.Targets[i], nil
		}

		// Also support "." exact match logic if targetPath resolution is funky
		if targetPath == "." && absPath == c.BaseDir {
			return &c.Targets[i], nil
		}
	}

	return nil, fmt.Errorf("no target found for path: %s", path)
}

// GenerateDefault creates a default config with auto-detected pattern
func GenerateDefault(url, fillerURL, inputPattern string) *Config {
	if inputPattern == "" {
		inputPattern = "Episode {{EP_NUM}} {{RES}}"
	}

	return &Config{
		Targets: []Target{
			{
				Path:      ".",
				URL:       url,
				FillerURL: fillerURL,
				Patterns: []Pattern{
					{
						Input: []string{inputPattern},
						Output: OutputConfig{
							Fields:    []string{"SERIES", "EP_NUM", "FILLER", "EP_NAME"},
							Separator: " - ",
						},
					},
				},
			},
		},
	}
}
