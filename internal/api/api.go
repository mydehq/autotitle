// Package api provides the main entry points for autotitle operations.
package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mydehq/autotitle/internal/backup"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/database"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/provider"
	_ "github.com/mydehq/autotitle/internal/provider/filler" // Register filler sources
	"github.com/mydehq/autotitle/internal/renamer"
	"github.com/mydehq/autotitle/internal/types"
)

// Option is a functional option for configuring operations
type Option func(*Options)

// Options holds configuration for autotitle operations
type Options struct {
	DryRun   bool
	NoBackup bool
	Events   types.EventHandler
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

// Rename renames anime episodes in the specified directory
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

	// Try to update database if needed (smart cache check)
	// If it fails (e.g. offline), we'll try to use the cached version if available
	_, genErr := DBGen(ctx, target.URL, target.FillerURL, false)
	if genErr != nil {
		// Log error but continue to see if we have valid cache
		// If we had an event handler here we could warn the user
		fmt.Printf("Warning: Failed to update database: %v\n", genErr)
	}

	// Load media from database
	media, err := db.Load(ctx, prov.Name(), id)
	if err != nil {
		return nil, err
	}

	if media == nil {
		// If loading failed and generation also failed, returns the generation error
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate database: %w", genErr)
		}
		// If generation "succeeded" (shouldn't happen if media is nil) or skipped,
		// but media is still nil, return valid error
		return nil, types.ErrDatabaseNotFound{Provider: prov.Name(), ID: id}
	}

	// Load global config
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		// Just warn, don't fail, use defaults
		fmt.Printf("Warning: Failed to load global config: %v\n", err)
		globalCfg = &config.GlobalConfig{
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

	// Execute rename
	return r.Execute(ctx, path, target, media)
}

// Init creates a new map file in the specified directory
func Init(ctx context.Context, path string, url, fillerURL string, force bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load global config for map file name, formats, and default patterns
	globalCfg, _ := config.LoadGlobal() // Ignore error, use defaults

	mapFileName := config.DefaultMapFileName
	formats := config.DefaultFormats
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
		if !force {
			return fmt.Errorf("map file already exists: %s", mapPath)
		}
	}

	// Try to detect pattern from files using global formats
	var detectedPattern string
	entries, _ := os.ReadDir(absPath)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if len(ext) > 0 {
			ext = ext[1:] // Remove leading dot
		}
		// Check if extension is in formats list
		for _, f := range formats {
			if ext == f {
				detectedPattern = matcher.GuessPattern(e.Name())
				break
			}
		}
		if detectedPattern != "" {
			break
		}
	}

	if url == "" {
		url = "https://myanimelist.net/anime/XXXXX/Series_Name"
	}
	if fillerURL == "" {
		fillerURL = "https://www.animefillerlist.com/shows/series-name"
	}

	// Use global patterns if detection failed
	var cfg *config.Config
	if detectedPattern == "" && globalCfg != nil && len(globalCfg.Patterns) > 0 {
		// Use patterns from global config
		cfg = &config.Config{
			Targets: []config.Target{
				{
					Path:      ".",
					URL:       url,
					FillerURL: fillerURL,
					Patterns:  globalCfg.Patterns,
				},
			},
		}
	} else {
		cfg = config.GenerateDefault(url, fillerURL, detectedPattern)
	}

	return config.Save(mapPath, cfg)
}

// DBGen generates a database from a provider URL
// Returns true if database was generated, false if it already existed
func DBGen(ctx context.Context, url string, fillerURL string, force bool) (bool, error) {
	// Load global config to configure provider
	globalCfg, _ := config.LoadGlobal() // Ignore error, use defaults if fails (nil is handled by Configure)

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
	if !force && db.Exists(prov.Name(), id) {
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
			// If load fails despite Exists returning true, assume valid (or corrupted, forcing overwrite might be safer?
			// But for now, let's respect the "cached" behavior if we can't inspect it, or maybe fetch if we can't read it.
			// Let's assume if Exists is true, we skip unless we have a reason to fetch.
			// Actually, if we can't read it, we should probably fetch.
			// But let's stick to the simple contract: if it exists, use it, UNLESS we know it's expired.
			return false, nil
		}
	}

	// Fetch media
	media, err := prov.FetchMedia(ctx, id)
	if err != nil {
		return false, err
	}

	// Fetch filler if URL provided
	if fillerURL != "" {
		fillerSource, err := provider.GetFillerSourceForURL(fillerURL)
		if err == nil {
			slug, err := fillerSource.ExtractSlug(fillerURL)
			if err == nil {
				fillers, err := fillerSource.FetchFillers(ctx, slug)
				if err == nil {
					for i := range media.Episodes {
						for _, f := range fillers {
							if media.Episodes[i].Number == f {
								media.Episodes[i].IsFiller = true
								break
							}
						}
					}
					media.FillerSource = fillerSource.Name()
				}
			}
		}
	}

	// Save to database
	// db variable already exists from earlier check
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

// Clean removes the backup for a specific directory
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
