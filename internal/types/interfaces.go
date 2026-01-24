// Package types defines interfaces for autotitle components.
package types

import "context"

// Provider is the core abstraction for data sources (anime, movies, TV, etc.)
type Provider interface {
	// Name returns the provider identifier (e.g., "mal", "tmdb")
	Name() string

	// Type returns the media type this provider handles
	Type() MediaType

	// MatchesURL returns true if this provider can handle the given URL
	MatchesURL(url string) bool

	// ExtractID extracts the media ID from a provider URL
	ExtractID(url string) (string, error)

	// FetchMedia fetches media data from the provider
	FetchMedia(ctx context.Context, id string) (*Media, error)

	// Configure updates provider settings (optional, can be no-op)
	Configure(cfg *APIConfig)
}

// FillerSource is a source for filler episode data (decoupled from providers)
type FillerSource interface {
	// Name returns the filler source identifier
	Name() string

	// MatchesURL returns true if this source can handle the given URL
	MatchesURL(url string) bool

	// ExtractSlug extracts the series slug from a filler source URL
	ExtractSlug(url string) (string, error)

	// FetchFillers returns a list of filler episode numbers
	FetchFillers(ctx context.Context, slug string) ([]int, error)
}

// DatabaseRepository handles media database persistence
type DatabaseRepository interface {
	// Save saves media data to the database
	Save(ctx context.Context, media *Media) error

	// Load loads media data from the database
	Load(ctx context.Context, provider, id string) (*Media, error)

	// Exists checks if a database entry exists
	Exists(provider, id string) bool

	// Delete removes a database entry
	Delete(ctx context.Context, provider, id string) error

	// DeleteAll removes all database entries
	DeleteAll(ctx context.Context) error

	// List returns all database entries for a provider (or all if empty)
	List(ctx context.Context, provider string) ([]MediaSummary, error)

	// Search finds entries matching a query
	Search(ctx context.Context, query string) ([]MediaSummary, error)

	// Path returns the database directory path
	Path() string
}

// MediaSummary is a lightweight summary for database listings
type MediaSummary struct {
	Provider     string `json:"provider"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	EpisodeCount int    `json:"episode_count"`
}

// BackupManager handles file backup/restore operations
type BackupManager interface {
	// Backup creates a backup of files before renaming
	// mappings is oldName -> newName
	Backup(ctx context.Context, dir string, mappings map[string]string) error

	// Restore restores files from the backup
	Restore(ctx context.Context, dir string) error

	// Clean removes the backup for a specific directory
	Clean(ctx context.Context, dir string) error

	// ListAll returns all backup records (global)
	ListAll(ctx context.Context) ([]BackupRecord, error)

	// CleanAll removes all backups globally
	CleanAll(ctx context.Context) error
}

// ConfigRepository handles configuration loading and saving
type ConfigRepository interface {
	// Load loads configuration from a file
	Load(ctx context.Context, path string) (*Config, error)

	// Save saves configuration to a file
	Save(ctx context.Context, path string, cfg *Config) error

	// Validate validates configuration
	Validate(cfg *Config) error
}

// Config represents the autotitle configuration file
type Config struct {
	Targets []Target `yaml:"targets"`
}

// Target represents a rename target in the configuration
type Target struct {
	Path      string    `yaml:"path"`
	URL       string    `yaml:"url"`
	FillerURL string    `yaml:"filler_url,omitempty"`
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
