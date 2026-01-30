// Package autotitle provides high-level functions for renaming media files
// with proper titles and filler detection.
//
// This package provides a clean API for integrating autotitle into other Go applications.
package autotitle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mydehq/autotitle/internal/backup"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/database"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/provider"
	_ "github.com/mydehq/autotitle/internal/provider/filler" // Register filler sources
	"github.com/mydehq/autotitle/internal/renamer"
	"github.com/mydehq/autotitle/internal/types"
	"github.com/mydehq/autotitle/internal/version"
)

// Re-export types
type (
	RenameOperation = types.RenameOperation
	Media           = types.Media
	Episode         = types.Episode
	Event           = types.Event
	EventHandler    = types.EventHandler
	Pattern         = matcher.Pattern
	TemplateVars    = matcher.TemplateVars
)

// Event Types & Status
const (
	EventInfo     = types.EventInfo
	EventSuccess  = types.EventSuccess
	EventWarning  = types.EventWarning
	EventError    = types.EventError
	EventProgress = types.EventProgress

	StatusPending = types.StatusPending
	StatusSuccess = types.StatusSuccess
	StatusSkipped = types.StatusSkipped
	StatusFailed  = types.StatusFailed
)

// Option is a functional option for configuring operations
type Option func(*Options)

// Options holds configuration for autotitle operations
type Options struct {
	DryRun   bool
	NoBackup bool

	Events types.EventHandler
	Offset *int

	// Init options
	URL       string
	FillerURL string
	Separator string
	Padding   int
	Force     bool
}

func (o *Options) emit(t types.EventType, msg string) {
	if o.Events != nil {
		o.Events(types.Event{Type: t, Message: msg})
	} else if t == types.EventWarning || t == types.EventError {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	}
}

// WithDryRun enables dry-run mode
func WithDryRun() Option {
	return func(o *Options) { o.DryRun = true }
}

// WithNoBackup disables backup creation
func WithNoBackup() Option {
	return func(o *Options) { o.NoBackup = true }
}

// WithEvents sets the event handler for progress updates
func WithEvents(h types.EventHandler) Option {
	return func(o *Options) { o.Events = h }
}

// WithOffset sets the episode number offset
func WithOffset(offset int) Option {
	return func(o *Options) { o.Offset = &offset }
}

// WithURL sets the provider URL for Init
func WithURL(url string) Option {
	return func(o *Options) { o.URL = url }
}

// WithFiller sets the filler list URL for Init
func WithFiller(url string) Option {
	return func(o *Options) { o.FillerURL = url }
}

// WithSeparator sets the separator for Init
func WithSeparator(sep string) Option {
	return func(o *Options) { o.Separator = sep }
}

// WithPadding sets the episode padding for Init
func WithPadding(p int) Option {
	return func(o *Options) { o.Padding = p }
}

// WithForce enables overwriting existing config for Init
func WithForce() Option {
	return func(o *Options) { o.Force = true }
}

// Rename renames media files in the specified directory
func Rename(ctx context.Context, path string, opts ...Option) ([]types.RenameOperation, error) {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	// Load config
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}

	// Resolve target
	target, err := cfg.ResolveTarget(path)
	if err != nil {
		return nil, err
	}

	// Get provider for URL
	prov, err := provider.GetProviderForURL(target.URL)
	if err != nil {
		return nil, err
	}

	// Extract ID
	id, err := prov.ExtractID(target.URL)
	if err != nil {
		return nil, err
	}

	// Initialize database
	db, err := database.NewRepository("")
	if err != nil {
		return nil, err
	}

	// If local options specify a FillerURL, prefer that over the config file
	fillerURL := target.FillerURL
	if options.FillerURL != "" {
		fillerURL = options.FillerURL
	}

	force := false
	if options.Force {
		force = true
	}

	dbGenOpts := []Option{
		WithFiller(fillerURL),
	}
	if force {
		dbGenOpts = append(dbGenOpts, WithForce())
	}

	_, genErr := DBGen(ctx, target.URL, dbGenOpts...)
	if genErr != nil {
		options.emit(types.EventWarning, fmt.Sprintf("Failed to update database: %v", genErr))
	}

	// Load media from database
	media, err := db.Load(ctx, prov.Name(), id)
	if err != nil {
		return nil, err
	}

	if media == nil {
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate database: %w", genErr)
		}
		return nil, types.ErrDatabaseNotFound{Provider: prov.Name(), ID: id}
	}

	// Load global config
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		options.emit(types.EventWarning, fmt.Sprintf("Failed to load global config: %v", err))
		globalCfg = &types.GlobalConfig{
			API:    types.APIConfig{RateLimit: 2.0, Timeout: 30},
			Backup: types.BackupConfig{Enabled: true, DirName: "backups"},
		}
	}

	// Create renamer
	r := renamer.New(db, globalCfg.Backup, globalCfg.Formats)
	if options.DryRun {
		r.WithDryRun()
	}
	if options.NoBackup {
		r.WithNoBackup()
	}
	if options.Events != nil {
		r.WithEvents(options.Events)
	}

	if options.Offset != nil {
		r.WithOffset(*options.Offset)
	}

	// Execute rename
	return r.Execute(ctx, path, target, media)
}

// Init creates a new map file in the specified directory
func Init(ctx context.Context, path string, opts ...Option) error {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load global config
	globalCfg, _ := config.LoadGlobal()
	defaults := config.GetDefaults()

	mapFileName := defaults.MapFile
	formats := defaults.Formats
	if globalCfg != nil {
		if globalCfg.MapFile != "" {
			mapFileName = globalCfg.MapFile
		}
		if len(globalCfg.Formats) > 0 {
			formats = globalCfg.Formats
		}
	}

	mapPath := filepath.Join(absPath, mapFileName)
	if _, err := os.Stat(mapPath); err == nil {
		if !options.Force {
			return fmt.Errorf("map file already exists: %s", mapPath)
		}
		// Warning when overriding
		options.emit(types.EventWarning, fmt.Sprintf("Overwriting existing map file: %s", mapPath))
	}

	// Analyze directory for patterns and media presence
	scanResult, err := config.Scan(absPath, formats)
	if err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}

	if !scanResult.HasMedia && scanResult.TotalFiles == 0 {
		if !options.Force {
			return fmt.Errorf("no files found in directory")
		}
		options.emit(types.EventWarning, "No files found in directory. Use standard configuration.")
	} else if !scanResult.HasMedia && scanResult.TotalFiles > 0 {
		if !options.Force {
			return fmt.Errorf("no media files found in directory (use --force to initialize anyway)")
		}
		options.emit(types.EventWarning, "No media files found. Use standard configuration.")
	}

	// Build configuration
	url := options.URL
	if url == "" {
		url = "https://myanimelist.net/anime/XXXXX/Series_Name"
	}
	fillerURL := options.FillerURL
	if fillerURL == "" {
		fillerURL = "https://www.animefillerlist.com/shows/series-name"
	}

	offset := 0
	if options.Offset != nil {
		offset = *options.Offset
	}

	// Generate default config
	cfg := config.GenerateDefault(url, fillerURL, scanResult.DetectedPatterns, options.Separator, offset, options.Padding)

	// If detection failed but we have global patterns, prefer those over hardcoded defaults
	if len(scanResult.DetectedPatterns) == 0 && globalCfg != nil && len(globalCfg.Patterns) > 0 {
		cfg.Targets[0].Patterns = globalCfg.Patterns
		// Apply overrides to these global patterns
		for i := range cfg.Targets[0].Patterns {
			if offset != 0 {
				cfg.Targets[0].Patterns[i].Output.Offset = offset
			}
			if options.Separator != "" {
				cfg.Targets[0].Patterns[i].Output.Separator = options.Separator
			}
			if options.Padding > 0 {
				cfg.Targets[0].Patterns[i].Output.Padding = options.Padding
			}
		}
	}

	return config.Save(mapPath, cfg)
}

// DBGen generates a database from a provider URL
// Returns true if database was generated, false if it already existed
func DBGen(ctx context.Context, url string, opts ...Option) (bool, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// Load global config to configure provider
	globalCfg, _ := config.LoadGlobal()

	// Get provider
	prov, err := provider.GetProviderForURL(url)
	if err != nil {
		return false, err
	}

	// Configure provider with global settings
	if globalCfg != nil {
		prov.Configure(&globalCfg.API)
	}

	// Extract ID
	id, err := prov.ExtractID(url)
	if err != nil {
		return false, err
	}

	// Initialize database repository
	db, err := database.NewRepository("")
	if err != nil {
		return false, err
	}

	// Check if exists
	if !options.Force && db.Exists(prov.Name(), id) {
		// Load existing data to check expiration
		existing, err := db.Load(ctx, prov.Name(), id)
		if err == nil && existing != nil {
			// If finished airing, no new episodes will come
			if existing.Status == "Finished Airing" {
				return false, nil // Skip
			}

			// If next episode is known and in the future, wait
			if existing.NextEpisodeAirDate != nil {
				t, err := time.Parse(time.RFC3339, *existing.NextEpisodeAirDate)
				if err == nil && t.After(time.Now()) {
					return false, nil // Skip
				}
			}
		} else {
			return false, nil
		}
	}

	// Fetch media
	media, err := prov.FetchMedia(ctx, id)
	if err != nil {
		return false, err
	}

	// Fetch filler if URL provided
	if options.FillerURL != "" {
		fillerSource, err := provider.GetFillerSourceForURL(options.FillerURL)
		if err == nil {
			slug, err := fillerSource.ExtractSlug(options.FillerURL)
			if err == nil {
				fillers, err := fillerSource.FetchFillers(ctx, slug)
				if err == nil {
					for i := range media.Episodes {
						if slices.Contains(fillers, media.Episodes[i].Number) {
							media.Episodes[i].IsFiller = true
						}
					}
					media.FillerSource = fillerSource.Name()
				}
			}
		}
	}

	// Save to database
	if err := db.Save(ctx, media); err != nil {
		return false, err
	}

	return true, nil
}

// DBList lists all cached databases
func DBList(ctx context.Context, providerFilter string) ([]types.MediaSummary, error) {
	db, err := database.NewRepository("")
	if err != nil {
		return nil, err
	}
	return db.List(ctx, providerFilter)
}

// DBInfo returns information about a specific database entry
func DBInfo(ctx context.Context, prov, id string) (*types.Media, error) {
	db, err := database.NewRepository("")
	if err != nil {
		return nil, err
	}
	return db.Load(ctx, prov, id)
}

// DBDelete removes a database entry
func DBDelete(ctx context.Context, prov, id string) error {
	db, err := database.NewRepository("")
	if err != nil {
		return err
	}
	return db.Delete(ctx, prov, id)
}

// DBDeleteAll removes all database entries
func DBDeleteAll(ctx context.Context) error {
	db, err := database.NewRepository("")
	if err != nil {
		return err
	}
	return db.DeleteAll(ctx)
}

// DBPath returns the database directory path
func DBPath() (string, error) {
	db, err := database.NewRepository("")
	if err != nil {
		return "", err
	}
	return db.Path(), nil
}

// Undo restores files from backup
func Undo(ctx context.Context, path string) error {
	db, err := database.NewRepository("")
	if err != nil {
		return err
	}
	cacheRoot := filepath.Dir(db.Path())

	globalCfg, _ := config.LoadGlobal()
	dirName := backup.DefaultDirName
	if globalCfg != nil && globalCfg.Backup.DirName != "" {
		dirName = globalCfg.Backup.DirName
	}

	bm := backup.New(cacheRoot, dirName)
	return bm.Restore(ctx, path)
}

// Clean removes the backup for a directory
func Clean(ctx context.Context, path string) error {
	db, err := database.NewRepository("")
	if err != nil {
		return err
	}
	cacheRoot := filepath.Dir(db.Path())

	globalCfg, _ := config.LoadGlobal()
	dirName := backup.DefaultDirName
	if globalCfg != nil && globalCfg.Backup.DirName != "" {
		dirName = globalCfg.Backup.DirName
	}

	bm := backup.New(cacheRoot, dirName)
	return bm.Clean(ctx, path)
}

// CleanAll removes all backups globally
func CleanAll(ctx context.Context) error {
	db, err := database.NewRepository("")
	if err != nil {
		return err
	}
	cacheRoot := filepath.Dir(db.Path())

	globalCfg, _ := config.LoadGlobal()
	dirName := backup.DefaultDirName
	if globalCfg != nil && globalCfg.Backup.DirName != "" {
		dirName = globalCfg.Backup.DirName
	}

	bm := backup.New(cacheRoot, dirName)
	return bm.CleanAll(ctx)
}

// Version returns the version string
func Version() string {
	return version.String()
}

// Provider registry functions
var (
	GetProviderForURL     = provider.GetProviderForURL
	GetFillerSourceForURL = provider.GetFillerSourceForURL
	ListProviders         = provider.ListProviders
	ListFillerSources     = provider.ListFillerSources
)

// Pattern utilities
var (
	CompilePattern             = matcher.Compile
	GuessPattern               = matcher.GuessPattern
	GenerateFilenameFromFields = matcher.GenerateFilenameFromFields
)
