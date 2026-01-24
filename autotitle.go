// Package autotitle provides high-level functions for renaming anime episodes
// with proper titles and filler detection.
//
// This package provides a clean API for integrating autotitle into other Go applications.
package autotitle

import (
	"context"

	"github.com/mydehq/autotitle/internal/api"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/provider"
	"github.com/mydehq/autotitle/internal/types"
	"github.com/mydehq/autotitle/internal/version"
)

// Re-export types
type (
	Option          = api.Option
	RenameOperation = types.RenameOperation
	Media           = types.Media
	Episode         = types.Episode
	Event           = types.Event
	EventHandler    = types.EventHandler
	Pattern         = matcher.Pattern
	TemplateVars    = matcher.TemplateVars
)

// Re-export option constructors
var (
	WithDryRun   = api.WithDryRun
	WithNoBackup = api.WithNoBackup
	WithEvents   = api.WithEvents
)

// Rename renames anime episodes in the specified directory
func Rename(ctx context.Context, path string, opts ...Option) ([]RenameOperation, error) {
	return api.Rename(ctx, path, opts...)
}

// Init creates a new map file
func Init(ctx context.Context, path, url, fillerURL string, force bool) error {
	return api.Init(ctx, path, url, fillerURL, force)
}

// DBGen generates a database from a provider URL
func DBGen(ctx context.Context, url, fillerURL string, force bool) (bool, error) {
	return api.DBGen(ctx, url, fillerURL, force)
}

// Undo restores files from backup
func Undo(ctx context.Context, path string) error {
	return api.Undo(ctx, path)
}

// Clean removes the backup for a directory
func Clean(ctx context.Context, path string) error {
	return api.Clean(ctx, path)
}

// CleanAll removes all backups globally
func CleanAll(ctx context.Context) error {
	return api.CleanAll(ctx)
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
