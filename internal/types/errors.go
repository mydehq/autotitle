// Package types defines custom error types for autotitle.
package types

import "fmt"

// ErrPatternNotMatched indicates a filename didn't match any pattern
type ErrPatternNotMatched struct {
	Filename string
}

func (e ErrPatternNotMatched) Error() string {
	return fmt.Sprintf("no pattern matched filename: %s", e.Filename)
}

// ErrEpisodeNotFound indicates an episode number wasn't in the database
type ErrEpisodeNotFound struct {
	Number int
}

func (e ErrEpisodeNotFound) Error() string {
	return fmt.Sprintf("episode not found: %d", e.Number)
}

// ErrDatabaseNotFound indicates a media database doesn't exist
type ErrDatabaseNotFound struct {
	Provider string
	ID       string
}

func (e ErrDatabaseNotFound) Error() string {
	return fmt.Sprintf("database not found: %s/%s", e.Provider, e.ID)
}

// ErrConfigInvalid indicates a configuration error
type ErrConfigInvalid struct {
	Path   string
	Reason string
}

func (e ErrConfigInvalid) Error() string {
	return fmt.Sprintf("invalid config %s: %s", e.Path, e.Reason)
}

// ErrConfigNotFound indicates a configuration file doesn't exist
type ErrConfigNotFound struct {
	Path string
}

func (e ErrConfigNotFound) Error() string {
	return fmt.Sprintf("configuration file not found: %s", e.Path)
}

// ErrProviderNotFound indicates no provider matches the given URL
type ErrProviderNotFound struct {
	URL string
}

func (e ErrProviderNotFound) Error() string {
	return fmt.Sprintf("no provider found for URL: %s", e.URL)
}

// ErrFillerSourceNotFound indicates no filler source matches the given URL
type ErrFillerSourceNotFound struct {
	URL string
}

func (e ErrFillerSourceNotFound) Error() string {
	return fmt.Sprintf("no filler source found for URL: %s", e.URL)
}

// ErrAPIError indicates an error from an external API
type ErrAPIError struct {
	Service    string
	StatusCode int
	Message    string
}

func (e ErrAPIError) Error() string {
	return fmt.Sprintf("%s API error (%d): %s", e.Service, e.StatusCode, e.Message)
}

// ErrBackupNotFound indicates no backup exists for the directory
type ErrBackupNotFound struct {
	Directory string
}

func (e ErrBackupNotFound) Error() string {
	return fmt.Sprintf("no backup found for: %s", e.Directory)
}
