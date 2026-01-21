// Package api provides the core implementation for autotitle operations.
// This package is used by both the CLI and the public library API.
package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soymadip/autotitle/internal/config"
	"github.com/soymadip/autotitle/internal/database"
	"github.com/soymadip/autotitle/internal/fetcher"
	"github.com/soymadip/autotitle/internal/matcher"
	"github.com/soymadip/autotitle/internal/renamer"
)

// Option is a functional option for configuring operations
type Option func(*Options)

// Options holds configuration for autotitle operations
type Options struct {
	DryRun     bool
	NoBackup   bool
	Verbose    bool
	Quiet      bool
	ConfigPath string
	Force      bool
	Anime      string
	Filler     string
	RateLimit  int
}

// WithDryRun enables dry-run mode (preview changes without applying)
func WithDryRun() Option {
	return func(o *Options) { o.DryRun = true }
}

// WithNoBackup disables backup creation before renaming
func WithNoBackup() Option {
	return func(o *Options) { o.NoBackup = true }
}

// WithVerbose enables verbose output
func WithVerbose() Option {
	return func(o *Options) { o.Verbose = true }
}

// WithQuiet suppresses all output except errors
func WithQuiet() Option {
	return func(o *Options) { o.Quiet = true }
}

// WithConfig specifies a custom config file path
func WithConfig(path string) Option {
	return func(o *Options) { o.ConfigPath = path }
}

// WithForce enables force mode (overwrite existing files)
func WithForce() Option {
	return func(o *Options) { o.Force = true }
}

// WithAnime sets the anime name or MAL URL
func WithAnime(anime string) Option {
	return func(o *Options) { o.Anime = anime }
}

// WithFiller sets the anime filler list URL or slug
func WithFiller(filler string) Option {
	return func(o *Options) { o.Filler = filler }
}

// WithRateLimit sets the API rate limit (requests per second)
func WithRateLimit(limit int) Option {
	return func(o *Options) { o.RateLimit = limit }
}

// Rename renames anime episodes in the specified directory
func Rename(path string, opts ...Option) error {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	// Load global config
	globalCfg, err := config.LoadGlobal(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load map config
	mapCfg, err := config.LoadMap(path, globalCfg.MapFile)

	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load map configuration: %w", err)
		}

		return fmt.Errorf("no map file found at %s. Run 'autotitle init' first.", filepath.Join(path, globalCfg.MapFile))
	}

	// Create renamer
	r := &renamer.Renamer{
		Config:    globalCfg,
		MapConfig: mapCfg,
		DryRun:    options.DryRun,
		NoBackup:  options.NoBackup,
		Verbose:   options.Verbose,
		Quiet:     options.Quiet,
	}

	// Initialize DB in renamer
	db, err := database.New("") // Use default cache dir
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	r.DB = db

	// Execute rename
	return r.Execute(path)
}

// Undo restores files from the backup directory
func Undo(path string) error {
	globalCfg, err := config.LoadGlobal("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	r := &renamer.Renamer{Config: globalCfg}
	if err := r.Undo(path); err != nil {
		return fmt.Errorf("failed to undo rename: %w", err)
	}
	return nil
}

// Clean removes the backup directory
func Clean(path string) error {
	globalCfg, err := config.LoadGlobal("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	r := &renamer.Renamer{Config: globalCfg}
	if err := r.Clean(path); err != nil {
		return fmt.Errorf("failed to clean backup: %w", err)
	}
	return nil
}

// DBGenOptions holds options for database generation
type DBGenOptions struct {
	MALURL     string
	AFLURL     string
	OutputDir  string
	Force      bool
	RateLimit  int
	ConfigPath string
}

// DBGen generates an episode database from MAL and AnimeFillerList
func DBGen(malURL string, opts ...func(*DBGenOptions)) error {
	options := &DBGenOptions{
		MALURL: malURL,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Load global config to check for default rate limit
	globalCfg, _ := config.LoadGlobal(options.ConfigPath) // Ignore error, as defaults will be used

	// Create fetcher
	// Use flag value if set, otherwise use config value, otherwise use hard default (1)
	rateLimit := 1

	if globalCfg != nil && globalCfg.API.RateLimit > 0 {
		rateLimit = globalCfg.API.RateLimit
	}

	if options.RateLimit > 0 {
		rateLimit = options.RateLimit
	}
	f := fetcher.New(rateLimit, 30)

	// Extract MAL ID from URL
	malID := fetcher.ExtractMALID(options.MALURL)
	if malID == 0 {
		return fmt.Errorf("failed to extract MAL ID from URL: %s", options.MALURL)
	}

	// Fetch episodes
	episodes, err := f.FetchEpisodes(malID)
	if err != nil {
		return fmt.Errorf("failed to fetch episodes: %w", err)
	}

	// Fetch fillers if AFL URL provided
	var fillers map[int]bool
	if options.AFLURL != "" {
		fillerList, err := f.FetchFillers(options.AFLURL)
		if err != nil {
			// Silently ignore filler fetch errors - fillers are optional
		} else if fillerList != nil {
			fillers = make(map[int]bool)
			for _, ep := range fillerList {
				fillers[ep] = true
			}
		}
	}

	// Fetch anime info
	animeInfo, err := f.FetchAnimeInfo(malID)
	if err != nil {
		return fmt.Errorf("failed to fetch anime info: %w", err)
	}

	// Create database
	db, err := database.New(options.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Check if database already exists
	seriesID := fmt.Sprintf("%d", malID)
	if db.Exists(seriesID) && !options.Force {
		return fmt.Errorf("database already exists for series %s (use Force option to overwrite)", seriesID)
	}

	// Build series data
	aliases := animeInfo.TitleSynonyms
	if animeInfo.TitleEnglish != "" && animeInfo.TitleEnglish != animeInfo.Title {
		aliases = append(aliases, animeInfo.TitleEnglish)
	}
	if animeInfo.TitleJapanese != "" && animeInfo.TitleJapanese != animeInfo.Title {
		aliases = append(aliases, animeInfo.TitleJapanese)
	}

	seriesData := database.SeriesData{
		MALID:         seriesID,
		Title:         animeInfo.Title,
		Slug:          fetcher.GenerateSlug(animeInfo.Title),
		Aliases:       aliases,
		MALURL:        animeInfo.URL,
		ImageURL:      animeInfo.ImageURL,
		EpisodeCount:  len(episodes),
		Episodes:      make(map[int]database.EpisodeData),
		TitleEnglish:  animeInfo.TitleEnglish,
		TitleJapanese: animeInfo.TitleJapanese,
		TitleSynonyms: animeInfo.TitleSynonyms,
	}

	for num, ep := range episodes {
		seriesData.Episodes[num] = database.EpisodeData{
			Number:  num,
			Title:   ep.Title,
			Filler:  fillers[num],
			AirDate: ep.AirDate,
		}
	}

	// Save to database
	if err := db.Save(&seriesData); err != nil {
		return fmt.Errorf("failed to save database: %w", err)
	}

	return nil
}

// Creates a new map file in the specified directory
func Init(path string, opts ...Option) error {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load global config to get map file name
	globalCfg, err := config.LoadGlobal(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	configPath := filepath.Join(absPath, globalCfg.MapFile)

	// Check if exists
	if _, err := os.Stat(configPath); err == nil && !options.Force {
		return fmt.Errorf("Config file already exists at %s (use WithForce to overwrite)", configPath)
	}

	// Detect pattern from first video file
	var detectedPattern string

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("Failed to read directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext == ".mkv" || ext == ".mp4" || ext == ".avi" || ext == ".webm" {
			detectedPattern = matcher.GuessPattern(name)
			break
		}
	}

	// Generate config content
	inputPattern := detectedPattern
	if inputPattern == "" {
		inputPattern = "Episode {{EP_NUM}} {{RES}}.{{EXT}}"
	}

	malURL := options.Anime
	if malURL == "" {
		malURL = "https://myanimelist.net/anime/XXXXX/Series_Name"
	}

	aflURL := options.Filler
	if aflURL == "" {
		aflURL = "https://www.animefillerlist.com/shows/series-name"
	}

	content := fmt.Sprintf(`# Autotitle Map File
targets:
  - path: "."
    mal_url: "%s"   # Replace with actual MAL page URL
    afl_url: "%s" # Replace with animeFilterList page url or 'null' if not found
    patterns:
      - input:
          - "%s"      # AUTO GENERATED, VERIFY
        output: "%s"  # Default output format
`, malURL, aflURL, inputPattern, globalCfg.Output)

	// Write file
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}

	return nil
}

// DBPath returns the path to the database directory
func DBPath(outputDir string) (string, error) {
	db, err := database.New(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}

	return db.Dir, nil
}

// DBList returns a list of all cached database series IDs
func DBList(outputDir string) ([]string, error) {
	db, err := database.New(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	ids, err := db.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	return ids, nil
}

// DBInfo returns information about a specific database
func DBInfo(seriesID string, outputDir string) (*SeriesData, error) {
	db, err := database.New(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	sd, err := db.Load(seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to load database for series %s: %w", seriesID, err)
	}
	if sd == nil {
		return nil, fmt.Errorf("database not found for series %s", seriesID)
	}

	return sd, nil
}

// DBRm removes one or more databases
func DBRm(seriesID string, outputDir string, deleteAll bool) error {
	db, err := database.New(outputDir)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if deleteAll {
		if err := db.DeleteAll(); err != nil {
			return fmt.Errorf("failed to delete all databases: %w", err)
		}
		return nil
	}

	if seriesID == "" {
		return fmt.Errorf("specify a series ID or use deleteAll to delete all")
	}

	if err := db.Delete(seriesID); err != nil {
		return fmt.Errorf("failed to delete database for series %s: %w", seriesID, err)
	}

	return nil
}

func FindSeriesByQuery(query string, outputDir string) ([]database.SearchResult, error) {
	db, err := database.New(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	return db.Find(query)
}

// Re-export commonly used types and functions from subpackages
type (
	Pattern      = matcher.Pattern
	TemplateVars = matcher.TemplateVars
	EpisodeData  = database.EpisodeData
	SeriesData   = database.SeriesData
)

// Re-export pattern utilities
var (
	CompilePattern   = matcher.Compile
	GuessPattern     = matcher.GuessPattern
	GenerateFilename = matcher.GenerateFilename
)
