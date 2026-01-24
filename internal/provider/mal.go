// Package provider implements data providers for fetching media information.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mydehq/autotitle/internal/types"
)

const (
	jikanAPIURL = "https://api.jikan.moe/v4"
)

// malURLPatterns are URL patterns that this provider handles
var malURLPatterns = []string{
	"myanimelist.net/anime/",
	"myanimelist.com/anime/",
}

// MALProvider implements the Provider interface for MyAnimeList
type MALProvider struct {
	client    *http.Client
	rateLimit time.Duration
}

// NewMALProvider creates a new MAL provider
func NewMALProvider(cfg *types.APIConfig) *MALProvider {
	timeout := 30 * time.Second
	rateLimit := time.Second / 2

	if cfg != nil {
		if cfg.Timeout > 0 {
			timeout = time.Duration(cfg.Timeout) * time.Second
		}
		if cfg.RateLimit > 0 {
			// Convert requests/sec to duration between requests
			// 2 req/s = 500ms
			rateLimit = time.Duration(float64(time.Second) / cfg.RateLimit)
		}
	}

	return &MALProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		rateLimit: rateLimit,
	}
}

// Name returns the provider identifier
func (p *MALProvider) Name() string {
	return "mal"
}

// Configure updates provider settings
func (p *MALProvider) Configure(cfg *types.APIConfig) {
	if cfg == nil {
		return
	}
	if cfg.Timeout > 0 {
		p.client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.RateLimit > 0 {
		p.rateLimit = time.Duration(float64(time.Second) / cfg.RateLimit)
	}
}

// Type returns the media type this provider handles
func (p *MALProvider) Type() types.MediaType {
	return types.MediaTypeAnime
}

// MatchesURL returns true if this provider can handle the given URL
func (p *MALProvider) MatchesURL(url string) bool {
	for _, pattern := range malURLPatterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

// ExtractID extracts the MAL ID from a URL
func (p *MALProvider) ExtractID(url string) (string, error) {
	re := regexp.MustCompile(`myanimelist\.(?:net|com)/anime/(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not extract MAL ID from URL: %s", url)
}

// FetchMedia fetches anime data from MyAnimeList via Jikan API
func (p *MALProvider) FetchMedia(ctx context.Context, id string) (*types.Media, error) {
	malID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid MAL ID: %s", id)
	}

	// Fetch anime info
	info, err := p.fetchAnimeInfo(ctx, malID)
	if err != nil {
		return nil, err
	}

	// Fetch episodes
	episodes, err := p.fetchEpisodes(ctx, malID)
	if err != nil {
		return nil, err
	}

	// Calculate next episode air date
	var nextEpisodeAirDate *string
	now := time.Now()

	for _, ep := range episodes {
		if ep.AirDate != "" {
			// Jikan format: "2006-04-04T00:00:00+00:00"
			t, err := time.Parse(time.RFC3339, ep.AirDate)
			if err == nil && t.After(now) {
				dateStr := ep.AirDate
				nextEpisodeAirDate = &dateStr
				break // Found the first future episode
			}
		}
	}

	return &types.Media{
		ID:                 id,
		Provider:           p.Name(),
		Title:              info.Title,
		TitleEN:            info.TitleEN,
		TitleJP:            info.TitleJP,
		Slug:               generateSlug(info.Title),
		Aliases:            info.Aliases,
		Type:               types.MediaTypeAnime,
		Status:             info.Status,
		NextEpisodeAirDate: nextEpisodeAirDate,
		Episodes:           episodes,
		EpisodeCount:       len(episodes),
		LastUpdate:         time.Now(),
	}, nil
}

type animeInfoResponse struct {
	Title   string
	TitleEN string
	TitleJP string
	Aliases []string
	Status  string
}

func (p *MALProvider) fetchAnimeInfo(ctx context.Context, malID int) (*animeInfoResponse, error) {
	p.sleep()

	url := fmt.Sprintf("%s/anime/%d", jikanAPIURL, malID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		// Rate limited, wait and retry
		time.Sleep(2 * time.Second)
		return p.fetchAnimeInfo(ctx, malID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.ErrAPIError{
			Service:    "Jikan",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to fetch anime %d", malID),
		}
	}

	var result struct {
		Data struct {
			Title         string   `json:"title"`
			TitleEnglish  string   `json:"title_english"`
			TitleJapanese string   `json:"title_japanese"`
			TitleSynonyms []string `json:"title_synonyms"`
			Status        string   `json:"status"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse anime info: %w", err)
	}

	return &animeInfoResponse{
		Title:   result.Data.Title,
		TitleEN: result.Data.TitleEnglish,
		TitleJP: result.Data.TitleJapanese,
		Aliases: result.Data.TitleSynonyms,
		Status:  result.Data.Status,
	}, nil
}

func (p *MALProvider) fetchEpisodes(ctx context.Context, malID int) ([]types.Episode, error) {
	var episodes []types.Episode
	page := 1

	for {
		p.sleep()

		url := fmt.Sprintf("%s/anime/%d/episodes?page=%d", jikanAPIURL, malID, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch episodes: %w", err)
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, types.ErrAPIError{
				Service:    "Jikan",
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("failed to fetch episodes for anime %d", malID),
			}
		}

		var result struct {
			Data []struct {
				MalID int    `json:"mal_id"`
				Title string `json:"title"`
				Aired string `json:"aired"`
			} `json:"data"`
			Pagination struct {
				HasNextPage bool `json:"has_next_page"`
			} `json:"pagination"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse episodes: %w", err)
		}
		resp.Body.Close()

		for _, ep := range result.Data {
			episodes = append(episodes, types.Episode{
				Number:  ep.MalID,
				Title:   ep.Title,
				AirDate: ep.Aired,
			})
		}

		if !result.Pagination.HasNextPage {
			break
		}
		page++
	}

	return episodes, nil
}

func (p *MALProvider) sleep() {
	time.Sleep(p.rateLimit)
}

// generateSlug converts a title to a URL-safe slug
func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(slug, "")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

// init registers the MAL provider
func init() {
	RegisterProvider(NewMALProvider(nil))
}
