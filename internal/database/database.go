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
	path := filepath.Join(db.Dir, seriesID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
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
	path := filepath.Join(db.Dir, sd.MALID+".json")
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
	path := filepath.Join(db.Dir, seriesID+".json")
	_, err := os.Stat(path)
	return err == nil
}

func (db *DB) Delete(seriesID string) error {
	path := filepath.Join(db.Dir, seriesID+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete database file for series %s: %w", seriesID, err)
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

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			ids = append(ids, entry.Name()[:len(entry.Name())-5])
		}
	}
	return ids, nil
}

func (db *DB) Find(query string) ([]SearchResult, error) {
	ids, err := db.List()
	if err != nil {
		return nil, err
	}

	var matches []SearchResult
	querySlug := fetcher.GenerateSlug(query)

	for _, id := range ids {
		sd, err := db.Load(id)
		if err != nil {
			continue // Skip malformed db files
		}

		// 1. Exact matches (ID or Slug)
		if sd.MALID == query || sd.Slug == querySlug {
			matches = append(matches, SearchResult{
				MALID:        sd.MALID,
				Title:        sd.Title,
				EpisodeCount: sd.EpisodeCount,
			})
			continue
		}

		// 2. Fuzzy/Partial matches on Title or Aliases
		// Check Main Title
		if strings.Contains(strings.ToLower(sd.Title), strings.ToLower(query)) {
			matches = append(matches, SearchResult{
				MALID:        sd.MALID,
				Title:        sd.Title,
				EpisodeCount: sd.EpisodeCount,
			})
			continue
		}

		// Check Aliases
		for _, alias := range sd.Aliases {
			if strings.Contains(strings.ToLower(alias), strings.ToLower(query)) {
				matches = append(matches, SearchResult{
					MALID:        sd.MALID,
					Title:        sd.Title,
					EpisodeCount: sd.EpisodeCount,
				})
				break
			}
		}
	}

	return matches, nil
}

func ParseRanges(s string) ([]int, error) {
	return util.ParseRanges(s)
}
