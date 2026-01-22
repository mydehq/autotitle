package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mydehq/autotitle/internal/fetcher"
	"github.com/mydehq/autotitle/internal/util"
)

type SearchResult struct {
	MALID        string
	Title        string
	EpisodeCount int
}

type EpisodeData struct {
	Number  int       `json:"number"`
	Title   string    `json:"title"`
	Filler  bool      `json:"filler"`
	AirDate time.Time `json:"air_date,omitempty"`
}

type SeriesData struct {
	MALID         string              `json:"mal_id"`
	Title         string              `json:"title"`
	Slug          string              `json:"slug"`
	Aliases       []string            `json:"aliases,omitempty"`
	MALURL        string              `json:"mal_url,omitempty"`
	ImageURL      string              `json:"image_url,omitempty"`
	EpisodeCount  int                 `json:"episode_count,omitempty"`
	LastUpdate    time.Time           `json:"last_update"`
	NextCheck     time.Time           `json:"next_check,omitempty"`
	Episodes      map[int]EpisodeData `json:"-"`
	TitleEnglish  string              `json:"title_english,omitempty"`
	TitleJapanese string              `json:"title_japanese,omitempty"`
	TitleSynonyms []string            `json:"title_synonyms,omitempty"`
}

type seriesDataJSON struct {
	MALID         string        `json:"mal_id"`
	Title         string        `json:"title"`
	Slug          string        `json:"slug"`
	Aliases       []string      `json:"aliases,omitempty"`
	MALURL        string        `json:"mal_url,omitempty"`
	ImageURL      string        `json:"image_url,omitempty"`
	EpisodeCount  int           `json:"episode_count,omitempty"`
	LastUpdate    time.Time     `json:"last_update"`
	NextCheck     time.Time     `json:"next_check,omitempty"`
	Episodes      []EpisodeData `json:"episodes"`
	TitleEnglish  string        `json:"title_english,omitempty"`
	TitleJapanese string        `json:"title_japanese,omitempty"`
	TitleSynonyms []string      `json:"title_synonyms,omitempty"`
}

func (sd *SeriesData) MarshalJSON() ([]byte, error) {
	episodes := make([]EpisodeData, 0, len(sd.Episodes))
	for num, ep := range sd.Episodes {
		ep.Number = num
		episodes = append(episodes, ep)
	}

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].Number < episodes[j].Number
	})

	return json.Marshal(seriesDataJSON{
		MALID:         sd.MALID,
		Title:         sd.Title,
		Slug:          sd.Slug,
		Aliases:       sd.Aliases,
		MALURL:        sd.MALURL,
		ImageURL:      sd.ImageURL,
		EpisodeCount:  sd.EpisodeCount,
		LastUpdate:    sd.LastUpdate,
		NextCheck:     sd.NextCheck,
		Episodes:      episodes,
		TitleEnglish:  sd.TitleEnglish,
		TitleJapanese: sd.TitleJapanese,
		TitleSynonyms: sd.TitleSynonyms,
	})
}

func (sd *SeriesData) UnmarshalJSON(data []byte) error {
	var aux seriesDataJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	sd.MALID = aux.MALID
	sd.Title = aux.Title
	sd.Slug = aux.Slug
	sd.Aliases = aux.Aliases
	sd.MALURL = aux.MALURL
	sd.ImageURL = aux.ImageURL
	sd.EpisodeCount = aux.EpisodeCount
	sd.LastUpdate = aux.LastUpdate
	sd.NextCheck = aux.NextCheck
	sd.Episodes = make(map[int]EpisodeData)
	sd.TitleEnglish = aux.TitleEnglish
	sd.TitleJapanese = aux.TitleJapanese
	sd.TitleSynonyms = aux.TitleSynonyms

	for _, ep := range aux.Episodes {
		sd.Episodes[ep.Number] = ep
	}
	return nil
}

type DB struct {
	Dir string
}

func New(customDir string) (*DB, error) {
	dir := customDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dir = filepath.Join(home, ".cache", "autotitle", "db")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	return &DB{Dir: dir}, nil
}

func (db *DB) Load(seriesID string) (*SeriesData, error) {

	// Find file matching pattern: {ID}@*.json
	pattern := filepath.Join(db.Dir, seriesID+"@*.json")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for series %s: %w", seriesID, err)
	}

	if len(matches) == 0 {
		return nil, nil // Not found
	}

	// If multiple matches, select the most recent file
	filePath := matches[0]

	if len(matches) > 1 {
		var newestTime time.Time

		for _, path := range matches {
			info, err := os.Stat(path)

			if err == nil && info.ModTime().After(newestTime) {
				newestTime = info.ModTime()
				filePath = path
			}
		}
	}

	data, err := os.ReadFile(filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read database file for series %s: %w", seriesID, err)
	}

	var sd SeriesData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("failed to parse database file for series %s: %w", seriesID, err)
	}

	if sd.Episodes == nil {
		sd.Episodes = make(map[int]EpisodeData)
	}

	return &sd, nil
}

func (db *DB) Save(sd *SeriesData) error {
	// Delete old files with same ID (handles slug changes)
	pattern := filepath.Join(db.Dir, sd.MALID+"@*.json")
	if oldMatches, err := filepath.Glob(pattern); err == nil {
		for _, oldPath := range oldMatches {
			os.Remove(oldPath) // Ignore errors, file might not exist
		}
	}

	// Truncate slug if filename would exceed 255 chars
	slug := sd.Slug
	maxSlugLen := 255 - len(sd.MALID) - len("@") - len(".json")
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}

	path := filepath.Join(db.Dir, sd.MALID+"@"+slug+".json")
	sd.LastUpdate = time.Now()

	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal series data for series %s: %w", sd.MALID, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write database file for series %s: %w", sd.MALID, err)
	}
	return nil
}

func (db *DB) Exists(seriesID string) bool {
	pattern := filepath.Join(db.Dir, seriesID+"@*.json")
	matches, err := filepath.Glob(pattern)
	return err == nil && len(matches) > 0
}

func (db *DB) Delete(seriesID string) error {

	pattern := filepath.Join(db.Dir, seriesID+"@*.json")
	matches, err := filepath.Glob(pattern)

	if err != nil {
		return fmt.Errorf("failed to search for series %s: %w", seriesID, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("database file not found for series %s", seriesID)
	}

	// Delete all matching files (in case of duplicates)
	for _, path := range matches {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to delete database file %s: %w", path, err)
		}
	}
	return nil
}

func (db *DB) DeleteAll() error {
	if err := os.RemoveAll(db.Dir); err != nil {
		return fmt.Errorf("failed to delete all database files: %w", err)
	}
	return nil
}

func (db *DB) List() ([]string, error) {
	entries, err := os.ReadDir(db.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list database files: %w", err)
	}

	idsMap := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			// Parse {ID}@{slug}.json format
			namePart := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			parts := strings.Split(namePart, "@")
			if len(parts) >= 1 {
				idsMap[parts[0]] = true
			}
		}
	}

	// Convert map to slice for deduplication
	var ids []string
	for id := range idsMap {
		ids = append(ids, id)
	}
	return ids, nil
}

type searchResultWithScore struct {
	result SearchResult
	score  int // Higher score = better match
}

func (db *DB) Find(query string) ([]SearchResult, error) {
	querySlug := fetcher.GenerateSlug(query)
	queryWords := strings.Fields(strings.ToLower(query))
	var matches []searchResultWithScore

	// Fast path: Try to find files by slug in filename
	if querySlug != "" {
		pattern := filepath.Join(db.Dir, "*@*"+querySlug+"*.json")
		fileMatches, err := filepath.Glob(pattern)
		if err == nil && len(fileMatches) > 0 {
			// Found matches in filenames, load only those
			for _, filePath := range fileMatches {
				baseName := filepath.Base(filePath)
				namePart := baseName[:len(baseName)-5] // Remove .json
				parts := strings.Split(namePart, "@")
				if len(parts) < 1 {
					continue
				}
				id := parts[0]

				sd, err := db.Load(id)
				if err != nil || sd == nil {
					continue
				}

				score := db.calculateMatchScore(sd, query, queryWords)
				matches = append(matches, searchResultWithScore{
					result: SearchResult{
						MALID:        sd.MALID,
						Title:        sd.Title,
						EpisodeCount: sd.EpisodeCount,
					},
					score: score,
				})
			}

			if len(matches) > 0 {
				// Sort by score (lowest first = best matches at bottom for selection)
				sort.Slice(matches, func(i, j int) bool {
					return matches[i].score < matches[j].score
				})
				results := make([]SearchResult, len(matches))
				for i, m := range matches {
					results[i] = m.result
				}
				return results, nil // Fast path succeeded
			}
		}
	}

	// Fallback: Search all files by content (original behavior)
	ids, err := db.List()
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		sd, err := db.Load(id)
		if err != nil || sd == nil {
			continue // Skip malformed db files
		}

		// Calculate match score based on how well title/aliases match
		score := db.calculateMatchScore(sd, query, queryWords)
		if score > 0 {
			matches = append(matches, searchResultWithScore{
				result: SearchResult{
					MALID:        sd.MALID,
					Title:        sd.Title,
					EpisodeCount: sd.EpisodeCount,
				},
				score: score,
			})
		}
	}

	// Sort by score (lowest first = best matches at bottom for selection)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score < matches[j].score
	})

	results := make([]SearchResult, len(matches))
	for i, m := range matches {
		results[i] = m.result
	}
	return results, nil
}

func (db *DB) calculateMatchScore(sd *SeriesData, query string, queryWords []string) int {
	score := 0

	// Handle empty query - match everything with low score
	if query == "" || len(queryWords) == 0 {
		return 1
	}

	// 1. Exact ID match: +1000
	if sd.MALID == query {
		return 1000
	}

	// 2. Exact slug match: +900
	querySlug := fetcher.GenerateSlug(query)
	if querySlug != "" && sd.Slug == querySlug {
		return 900
	}

	// 3. Count matching words in title
	titleLower := strings.ToLower(sd.Title)
	for _, word := range queryWords {
		if strings.Contains(titleLower, word) {
			score += 100
		}
	}

	// 4. Count matching words in aliases
	for _, alias := range sd.Aliases {
		aliasLower := strings.ToLower(alias)
		for _, word := range queryWords {
			if strings.Contains(aliasLower, word) {
				score += 50
			}
		}
	}

	return score
}

func ParseRanges(s string) ([]int, error) {
	return util.ParseRanges(s)
}
