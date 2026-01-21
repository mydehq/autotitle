package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mydehq/autotitle/internal/logger"
	"github.com/mydehq/autotitle/internal/util"
	"golang.org/x/net/html"
)

const JikanAPIURL = "https://api.jikan.moe/v4"
const FillerListURL = "https://www.animefillerlist.com/shows"

type Fetcher struct {
	Client    *http.Client
	RateLimit time.Duration
}

// New creates a new Fetcher instance
func New(rateLimit int, timeout int) *Fetcher {
	return &Fetcher{
		Client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		RateLimit: time.Second / time.Duration(rateLimit),
	}
}

type EpisodeInfo struct {
	Title   string
	AirDate time.Time
}

type AnimeInfo struct {
	Title         string   `json:"title"`
	TitleEnglish  string   `json:"title_english"`
	TitleJapanese string   `json:"title_japanese"`
	TitleSynonyms []string `json:"title_synonyms"`
	URL           string   `json:"url"`
	ImageURL      string   `json:"image_url"`
}

// Fetches detailed anime information from the Jikan API
func (f *Fetcher) FetchAnimeInfo(malID int) (*AnimeInfo, error) {
	f.sleep()
	url := fmt.Sprintf("%s/anime/%d", JikanAPIURL, malID)

	resp, err := f.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime info from Jikan API for MAL ID %d: %w", malID, err)
	}

	if resp.StatusCode == 429 {
		logger.Warn("Rate limited by Jikan API, retrying with longer delay...")
		time.Sleep(2 * time.Second)
		return f.FetchAnimeInfo(malID)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Jikan API returned status %s for MAL ID %d", resp.Status, malID)
	}

	var result struct {
		Data struct {
			Title         string   `json:"title"`
			TitleEnglish  string   `json:"title_english"`
			TitleJapanese string   `json:"title_japanese"`
			TitleSynonyms []string `json:"title_synonyms"`
			URL           string   `json:"url"`
			Images        struct {
				Webp struct {
					LargeImageURL string `json:"large_image_url"`
				} `json:"webp"`
			} `json:"images"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to parse Jikan API response for MAL ID %d: %w", malID, err)
	}
	resp.Body.Close()

	return &AnimeInfo{
		Title:         result.Data.Title,
		TitleEnglish:  result.Data.TitleEnglish,
		TitleJapanese: result.Data.TitleJapanese,
		TitleSynonyms: result.Data.TitleSynonyms,
		URL:           result.Data.URL,
		ImageURL:      result.Data.Images.Webp.LargeImageURL,
	}, nil
}

// Fetches all episodes for a given MAL ID
func (f *Fetcher) FetchEpisodes(malID int) (map[int]EpisodeInfo, error) {
	episodes := make(map[int]EpisodeInfo)
	page := 1
	lastPage := 1

	for page <= lastPage {
		f.sleep()
		url := fmt.Sprintf("%s/anime/%d/episodes?page=%d", JikanAPIURL, malID, page)

		resp, err := f.Client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch episodes from Jikan API for MAL ID %d (page %d): %w", malID, page, err)
		}

		if resp.StatusCode == 429 {
			logger.Warn("Rate limited by Jikan API, retrying with longer delay...")
			time.Sleep(2 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("Jikan API returned status %s for MAL ID %d (page %d)", resp.Status, malID, page)
		}

		var result struct {
			Data []struct {
				MalID int     `json:"mal_id"`
				Title string  `json:"title"`
				Aired *string `json:"aired"`
			} `json:"data"`
			Pagination struct {
				LastVisiblePage int  `json:"last_visible_page"`
				HasNextPage     bool `json:"has_next_page"`
			} `json:"pagination"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("Failed to parse Jikan API response for MAL ID %d (page %d): %w", malID, page, err)
		}
		resp.Body.Close()

		for _, ep := range result.Data {
			var airDate time.Time

			if ep.Aired != nil {
				// Try RFC3339 first, then simplified date format
				t, err := time.Parse(time.RFC3339, *ep.Aired)
				if err != nil {
					t, err = time.Parse("2006-01-02", *ep.Aired)
					if err == nil {
						airDate = t
					}
				} else {
					airDate = t
				}
			}
			episodes[ep.MalID] = EpisodeInfo{
				Title:   ep.Title,
				AirDate: airDate,
			}
		}

		lastPage = result.Pagination.LastVisiblePage

		if lastPage == 0 {
			lastPage = page
		}
		if !result.Pagination.HasNextPage && page >= lastPage {
			break
		}
		page++
	}

	return episodes, nil
}

// Fetches filler episode numbers from AnimeFillerList
func (f *Fetcher) FetchFillers(slug string) ([]int, error) {
	url := fmt.Sprintf("%s/%s", FillerListURL, slug)

	resp, err := f.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch filler list from AnimeFillerList for slug %s: %w", slug, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Warn("No filler list found for slug: %s (404)", slug)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AnimeFillerList returned status %s for slug %s", resp.Status, slug)
	}

	return parseFillerHTML(resp.Body)
}

func (f *Fetcher) sleep() {
	time.Sleep(f.RateLimit)
}

func parseFillerHTML(r io.Reader) ([]int, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AnimeFillerList HTML: %w", err)
	}

	var fillers []int
	var crawler func(*html.Node)

	crawler = func(node *html.Node) {

		if node.Type == html.ElementNode && node.Data == "div" {
			class := getAttr(node, "class")

			if strings.Contains(class, "filler") && !strings.Contains(class, "canon") {

				// Found filler div, extract episode numbers
				if span := findChildByClass(node, "span", "Episodes"); span != nil {
					text := getText(span)
					nums, _ := util.ParseRanges(text)
					fillers = append(fillers, nums...)
				}
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			crawler(c)
		}
	}

	crawler(doc)
	return fillers, nil
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findChildByClass(n *html.Node, tag, class string) *html.Node {
	var result *html.Node
	var search func(*html.Node)
	search = func(node *html.Node) {
		if result != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == tag && strings.Contains(getAttr(node, "class"), class) {
			result = node
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			search(c)
		}
	}
	search(n)
	return result
}

func getText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += getText(c)
	}

	return text
}

// Converts a title to a URL-safe slug
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)

	// Remove special characters
	slug = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(slug, "")

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Normalize consecutive hyphens
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")

	return strings.Trim(slug, "-")
}

// Extracts the numeric ID from a MyAnimeList URL
func ExtractMALID(url string) int {
	re := regexp.MustCompile(`myanimelist\.net/anime/(\d+)`)
	matches := re.FindStringSubmatch(url)

	if len(matches) > 1 {
		id, _ := strconv.Atoi(matches[1])
		return id
	}

	return 0
}
