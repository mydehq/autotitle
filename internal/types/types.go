// Package types defines core domain types used throughout autotitle.
package types

import "time"

// MediaType represents the type of media content
type MediaType string

const (
	MediaTypeAnime  MediaType = "anime"
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tvshow"
)

// Episode represents a single episode in a series
type Episode struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	IsFiller bool   `json:"is_filler,omitempty"`
	IsMixed  bool   `json:"is_mixed,omitempty"`
	AirDate  string `json:"air_date,omitempty"`
}

// Media is the unified type for all content (anime, movies, TV shows)
type Media struct {
	ID                 string    `json:"id"`
	Provider           string    `json:"provider"`
	Title              string    `json:"title"`
	TitleEN            string    `json:"title_en,omitempty"`
	TitleJP            string    `json:"title_jp,omitempty"`
	Slug               string    `json:"slug,omitempty"`
	Aliases            []string  `json:"aliases,omitempty"`
	Type               MediaType `json:"type"`
	Status             string    `json:"status,omitempty"`
	NextEpisodeAirDate *string   `json:"next_episode_air_date,omitempty"`
	EpisodeCount       int       `json:"episode_count,omitempty"`
	FillerSource       string    `json:"filler_source,omitempty"`
	LastUpdate         time.Time `json:"last_update"`
	Episodes           []Episode `json:"episodes,omitempty"`
}

// APIConfig holds API-related settings
type APIConfig struct {
	RateLimit float64 `yaml:"rate_limit"` // Requests per second
	Timeout   int     `yaml:"timeout"`    // Seconds
}

// BackupConfig holds backup-related settings
type BackupConfig struct {
	Enabled bool   `yaml:"enabled"`
	DirName string `yaml:"dir_name"`
}

// GetTitle returns the requested title variant with fallback to default
func (m *Media) GetTitle(variant string) string {
	switch variant {
	case "SERIES_JP", "JP":
		if m.TitleJP != "" {
			return m.TitleJP
		}
	case "SERIES_EN", "EN":
		if m.TitleEN != "" {
			return m.TitleEN
		}
	}
	return m.Title
}

// GetEpisode returns an episode by number, or nil if not found
func (m *Media) GetEpisode(num int) *Episode {
	for i := range m.Episodes {
		if m.Episodes[i].Number == num {
			return &m.Episodes[i]
		}
	}
	return nil
}

// OperationStatus represents the status of a rename operation
type OperationStatus string

const (
	StatusPending OperationStatus = "pending"
	StatusSuccess OperationStatus = "success"
	StatusSkipped OperationStatus = "skipped"
	StatusFailed  OperationStatus = "failed"
)

// RenameOperation represents a planned or completed file rename
type RenameOperation struct {
	SourcePath string          `json:"source_path"`
	TargetPath string          `json:"target_path"`
	Episode    *Episode        `json:"episode,omitempty"`
	Status     OperationStatus `json:"status"`
	Error      string          `json:"error,omitempty"`
}

// BackupRecord tracks a backup in the global registry
type BackupRecord struct {
	Path      string    `json:"path"`       // Full path to backup dir
	SourceDir string    `json:"source_dir"` // Original directory
	Timestamp time.Time `json:"timestamp"`
}

// EventType represents the type of progress event
type EventType string

const (
	EventInfo     EventType = "info"
	EventProgress EventType = "progress"
	EventSuccess  EventType = "success"
	EventWarning  EventType = "warning"
	EventError    EventType = "error"
)

// Event represents a progress event during operations
type Event struct {
	Type    EventType `json:"type"`
	Message string    `json:"message"`
	Data    any       `json:"data,omitempty"`
}

// EventHandler receives progress events during operations
type EventHandler func(Event)
