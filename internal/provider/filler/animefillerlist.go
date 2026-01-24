// Package filler implements filler source providers.
package filler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mydehq/autotitle/internal/provider"
	"github.com/mydehq/autotitle/internal/types"
	"golang.org/x/net/html"
)

const fillerListURL = "https://www.animefillerlist.com/shows"

// aflURLPatterns are URL patterns that this filler source handles
var aflURLPatterns = []string{
	"animefillerlist.com/shows/",
	"animefillerlist.com/",
}

// AnimeFillerListSource implements FillerSource for AnimeFillerList.com
type AnimeFillerListSource struct {
	client *http.Client
}

// NewAnimeFillerListSource creates a new AnimeFillerList source
func NewAnimeFillerListSource() *AnimeFillerListSource {
	return &AnimeFillerListSource{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the filler source identifier
func (s *AnimeFillerListSource) Name() string {
	return "animefillerlist"
}

// MatchesURL returns true if this source can handle the given URL
func (s *AnimeFillerListSource) MatchesURL(url string) bool {
	for _, pattern := range aflURLPatterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	return false
}

// ExtractSlug extracts the series slug from a filler source URL
func (s *AnimeFillerListSource) ExtractSlug(url string) (string, error) {
	re := regexp.MustCompile(`animefillerlist\.com/shows/([a-z0-9-]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not extract slug from URL: %s", url)
}

// FetchFillers fetches filler episode numbers from AnimeFillerList
func (s *AnimeFillerListSource) FetchFillers(ctx context.Context, slug string) ([]int, error) {
	url := fmt.Sprintf("%s/%s", fillerListURL, slug)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Add User-Agent to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Autotitle/2.0; +https://github.com/mydehq/autotitle)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch filler list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // No filler list found
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.ErrAPIError{
			Service:    "AnimeFillerList",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to fetch filler list for %s", slug),
		}
	}

	return parseFillerHTML(resp.Body)
}

func parseFillerHTML(r io.Reader) ([]int, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var fillers []int
	seen := make(map[int]bool)
	var crawler func(*html.Node)

	crawler = func(node *html.Node) {
		// Look for table rows: <tr class="filler ...">
		if node.Type == html.ElementNode && node.Data == "tr" {
			class := getAttr(node, "class")
			// Capture all filler types:
			// - "filler" - pure filler
			// - "mostly-filler" or "mostly_filler" - mostly filler content
			// - "mixed-filler" or "mixed_filler" - mixed filler/canon
			// Exclude "canon" only entries
			isFiller := strings.Contains(class, "filler") && !strings.HasPrefix(strings.TrimSpace(class), "canon")
			if isFiller {
				// Find <td class="Number">
				if td := findChildByClass(node, "td", "Number"); td != nil {
					text := getText(td)
					var num int
					if _, err := fmt.Sscanf(strings.TrimSpace(text), "%d", &num); err == nil {
						if !seen[num] {
							fillers = append(fillers, num)
							seen[num] = true
						}
					}
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
	// Direct child check
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag && strings.Contains(getAttr(c, "class"), class) {
			return c
		}
	}
	return nil
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

// init registers the AnimeFillerList source
func init() {
	provider.RegisterFillerSource(NewAnimeFillerListSource())
}
