// Package database implements media database persistence with provider subdirectories.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mydehq/autotitle/internal/types"
)

// Repository implements types.DatabaseRepository
type Repository struct {
	baseDir string
}

// NewRepository creates a new database repository
func NewRepository(customDir string) (*Repository, error) {
	dir := customDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dir = filepath.Join(home, ".cache", "autotitle", "db")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	return &Repository{baseDir: dir}, nil
}

// Save saves media data to the database
func (r *Repository) Save(ctx context.Context, media *types.Media) error {

	// Create provider subdirectory
	providerDir := filepath.Join(r.baseDir, media.Provider)
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("failed to create provider directory: %w", err)
	}

	// Delete old files with same ID (handles slug changes)
	pattern := filepath.Join(providerDir, media.ID+"@*.json")
	if oldMatches, _ := filepath.Glob(pattern); len(oldMatches) > 0 {
		for _, oldPath := range oldMatches {
			_ = os.Remove(oldPath)
		}
	}

	// Truncate slug if filename would exceed 255 chars
	slug := media.Slug
	maxSlugLen := 255 - len(media.ID) - len("@") - len(".json")
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}

	path := filepath.Join(providerDir, media.ID+"@"+slug+".json")

	data, err := json.MarshalIndent(media, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal media data: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write database file: %w", err)
	}

	return nil
}

// Load loads media data from the database
func (r *Repository) Load(ctx context.Context, provider, id string) (*types.Media, error) {
	providerDir := filepath.Join(r.baseDir, provider)
	pattern := filepath.Join(providerDir, id+"@*.json")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for media: %w", err)
	}

	if len(matches) == 0 {
		return nil, nil // Not found
	}

	// Use first match (or most recent if multiple)
	filePath := matches[0]
	if len(matches) > 1 {
		filePath = r.newestFile(matches)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database file: %w", err)
	}

	var media types.Media
	if err := json.Unmarshal(data, &media); err != nil {
		return nil, fmt.Errorf("failed to parse database file: %w", err)
	}

	return &media, nil
}

// Exists checks if a database entry exists
func (r *Repository) Exists(provider, id string) bool {
	providerDir := filepath.Join(r.baseDir, provider)
	pattern := filepath.Join(providerDir, id+"@*.json")
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}

// Delete removes a database entry
func (r *Repository) Delete(ctx context.Context, provider, id string) error {
	providerDir := filepath.Join(r.baseDir, provider)
	pattern := filepath.Join(providerDir, id+"@*.json")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to search for media: %w", err)
	}

	if len(matches) == 0 {
		return types.ErrDatabaseNotFound{Provider: provider, ID: id}
	}

	for _, path := range matches {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to delete database file: %w", err)
		}
	}

	return nil
}

// DeleteAll removes all database entries
func (r *Repository) DeleteAll(ctx context.Context) error {
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read database directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(r.baseDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to delete provider directory: %w", err)
			}
		}
	}

	return nil
}

// List returns all database entries for a provider (or all if empty)
func (r *Repository) List(ctx context.Context, provider string) ([]types.MediaSummary, error) {
	var summaries []types.MediaSummary

	// If provider specified, only list that provider
	var providers []string
	if provider != "" {
		providers = []string{provider}
	} else {
		// List all provider directories
		entries, err := os.ReadDir(r.baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to read database directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				providers = append(providers, entry.Name())
			}
		}
	}

	for _, prov := range providers {
		providerDir := filepath.Join(r.baseDir, prov)
		entries, err := os.ReadDir(providerDir)
		if err != nil {
			continue
		}

		// Track unique IDs (in case of duplicates)
		seen := make(map[string]bool)

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			// Parse {ID}@{slug}.json
			name := entry.Name()
			name = name[:len(name)-5] // Remove .json
			id, _, _ := strings.Cut(name, "@")
			if seen[id] {
				continue
			}
			seen[id] = true

			// Load to get title and episode count
			media, err := r.Load(ctx, prov, id)
			if err != nil || media == nil {
				continue
			}

			summaries = append(summaries, types.MediaSummary{
				Provider:     prov,
				ID:           id,
				Title:        media.Title,
				EpisodeCount: len(media.Episodes),
			})
		}
	}

	return summaries, nil
}

// Search finds entries matching a query
func (r *Repository) Search(ctx context.Context, query string) ([]types.MediaSummary, error) {
	all, err := r.List(ctx, "")
	if err != nil {
		return nil, err
	}

	if query == "" {
		return all, nil
	}

	queryLower := strings.ToLower(query)
	var results []types.MediaSummary

	for _, s := range all {
		// Match by ID or title
		if s.ID == query || strings.Contains(strings.ToLower(s.Title), queryLower) {
			results = append(results, s)
		}
	}

	// Sort by title
	slices.SortFunc(results, func(a, b types.MediaSummary) int {
		return strings.Compare(a.Title, b.Title)
	})

	return results, nil
}

// Path returns the base database directory
func (r *Repository) Path() string {
	return r.baseDir
}

func (r *Repository) newestFile(files []string) string {
	var newest string
	var newestTime int64

	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && info.ModTime().Unix() > newestTime {
			newestTime = info.ModTime().Unix()
			newest = f
		}
	}

	if newest == "" {
		return files[0]
	}
	return newest
}
