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

// Type aliases for easier usage
type (
	Config       = types.Config
	Target       = types.Target
	Pattern      = types.Pattern
	OutputConfig = types.OutputConfig
	GlobalConfig = types.GlobalConfig
)

const GlobalConfigFileName = "config.yml"

// defaults holds the default global configuration values
var defaults = types.GlobalConfig{
	MapFile: "_autotitle.yml",
	Formats: []string{"mkv", "mp4", "avi", "webm", "m4v", "ts", "flv"},
	Patterns: []types.Pattern{
		{
			Input: []string{"{{EP_NUM}}.{{EXT}}", "Episode {{EP_NUM}}.{{EXT}}", "E{{EP_NUM}}.{{EXT}}"},
			Output: types.OutputConfig{
				Fields:    []string{"E", "+", "EP_NUM", "FILLER", "-", "EP_NAME"},
				Separator: " ",
				Offset:    0,
				Padding:   0, // 0 means auto-detect
			},
		},
	},
	API: types.APIConfig{
		RateLimit: 2.0,
		Timeout:   30,
	},
	Backup: types.BackupConfig{
		Enabled: true,
		DirName: ".autotitle_backup",
	},
}

// defaultMapFile holds the default configuration for _autotitle.yml
var defaultMapFile = types.Config{
	Targets: []types.Target{
		{
			Path:      ".",
			URL:       "",
			FillerURL: "",
			Patterns:  defaults.Patterns,
		},
	},
}

// GetDefaults returns a deep copy of the default global configuration
func GetDefaults() types.GlobalConfig {
	return defaults.Clone()
}

// Load loads configuration from a directory
func Load(dir string) (*types.Config, error) {
	// Try to get map file name from global config
	mapFileName := defaults.MapFile
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
func LoadFile(path string) (*types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, types.ErrConfigNotFound{Path: path}
		}
		return nil, fmt.Errorf("failed to read map file: %w", err)
	}

	var cfg types.Config
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
func LoadGlobal() (*types.GlobalConfig, error) {
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
	cfg := &types.GlobalConfig{}
	*cfg = GetDefaults()

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
func Save(path string, cfg *types.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write map file: %w", err)
	}

	return nil
}

// SaveToDir saves configuration to the default map file in the specified directory
func SaveToDir(dir string, cfg *types.Config) error {
	path := filepath.Join(dir, defaults.MapFile)
	return Save(path, cfg)
}

// Validate validates the configuration
func Validate(cfg *types.Config) error {
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

// GenerateDefault creates a default config with auto-detected pattern
func GenerateDefault(url, fillerURL string, inputPatterns []string, separator string, offset, padding int) *types.Config {

	// Create a deep copy of defaultMapFile to avoid mutating globals
	cfg := defaultMapFile.Clone()
	target := &cfg.Targets[0]

	// Override with provided values
	if url != "" {
		target.URL = url
	}

	target.FillerURL = fillerURL

	// If input patterns are provided, we only want those.
	if len(inputPatterns) > 0 {
		templateOutput := target.Patterns[0].Output
		target.Patterns = []types.Pattern{
			{
				Input:  inputPatterns,
				Output: templateOutput,
			},
		}
	}

	// Apply settings to all patterns
	for i := range target.Patterns {
		if separator != "" {
			target.Patterns[i].Output.Separator = separator
		}
		if offset != 0 {
			target.Patterns[i].Output.Offset = offset
		}
		if padding > 0 {
			target.Patterns[i].Output.Padding = padding
		}
	}

	return cfg
}
