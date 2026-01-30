package types

import (
	"fmt"
	"path/filepath"
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
	Fields    []string `yaml:"fields,flow"`
	Separator string   `yaml:"separator,omitempty"`
	Offset    int      `yaml:"offset,omitempty"`  // Episode number offset
	Padding   int      `yaml:"padding,omitempty"` // Episode number padding (e.g. 2 -> 01, 3 -> 001)
}

// GlobalConfig represents the global configuration file (~/.config/autotitle/config.yml)
type GlobalConfig struct {
	MapFile  string       `yaml:"map_file"`
	Patterns []Pattern    `yaml:"patterns"`
	Formats  []string     `yaml:"formats"`
	API      APIConfig    `yaml:"api"`
	Backup   BackupConfig `yaml:"backup"`
}

// Clone returns a deep copy of the configuration
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	res := *c
	if len(c.Targets) > 0 {
		res.Targets = make([]Target, len(c.Targets))
		for i, t := range c.Targets {
			res.Targets[i] = *t.Clone()
		}
	}
	return &res
}

// Clone returns a deep copy of the target
func (t *Target) Clone() *Target {
	if t == nil {
		return nil
	}
	res := *t
	if len(t.Patterns) > 0 {
		res.Patterns = make([]Pattern, len(t.Patterns))
		for i, p := range t.Patterns {
			res.Patterns[i] = *p.Clone()
		}
	}
	return &res
}

// Clone returns a deep copy of the pattern
func (p *Pattern) Clone() *Pattern {
	if p == nil {
		return nil
	}
	res := *p
	if len(p.Input) > 0 {
		res.Input = make([]string, len(p.Input))
		copy(res.Input, p.Input)
	}
	if len(p.Output.Fields) > 0 {
		res.Output.Fields = make([]string, len(p.Output.Fields))
		copy(res.Output.Fields, p.Output.Fields)
	}
	return &res
}

// Clone returns a deep copy of the global configuration
func (g *GlobalConfig) Clone() GlobalConfig {
	res := *g
	if len(g.Patterns) > 0 {
		res.Patterns = make([]Pattern, len(g.Patterns))
		for i, p := range g.Patterns {
			res.Patterns[i] = *p.Clone()
		}
	}
	if len(g.Formats) > 0 {
		res.Formats = make([]string, len(g.Formats))
		copy(res.Formats, g.Formats)
	}
	return res
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

		// Support "." as an exact match for the base directory
		if targetPath == "." && absPath == c.BaseDir {
			return &c.Targets[i], nil
		}
	}

	return nil, fmt.Errorf("no target found for path: %s", path)
}
