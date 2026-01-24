package matcher

import (
	"path/filepath"
	"regexp"
	"strings"
)

// GuessPattern auto-detects a pattern from a filename
func GuessPattern(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if len(ext) > 0 {
		ext = strings.TrimPrefix(ext, ".")
	}

	pattern := base

	// Mask CRCs: [8 hex chars] -> [{{ANY}}]
	reCRC := regexp.MustCompile(`\[[A-Fa-f0-9]{8}\]`)
	pattern = reCRC.ReplaceAllString(pattern, `[{{ANY}}]`)

	// Identify Resolution: \d{3,4}p or \d+x\d+
	reRes := regexp.MustCompile(`(?i)\b(\d{3,4}p|\d{3,4}x\d{3,4})\b`)
	if loc := reRes.FindStringIndex(pattern); loc != nil {
		pattern = pattern[:loc[0]] + "{{RES}}" + pattern[loc[1]:]
	}

	// Heuristic A: SxxExx format
	reSxxExx := regexp.MustCompile(`(?i)(S\d+E)(\d+)`)
	if startEnd := reSxxExx.FindStringSubmatchIndex(pattern); startEnd != nil {
		prefixEnd := startEnd[3]
		numEnd := startEnd[5]
		pattern = pattern[:prefixEnd] + "{{EP_NUM}}" + pattern[numEnd:]
		goto Finalize
	}

	// Heuristic B: Prefix patterns like " - 01" or " Episode 01"
	{
		rePrefix := regexp.MustCompile(`( - | Episode | Ep\.? )(\d+)`)
		if startEnd := rePrefix.FindStringSubmatchIndex(pattern); startEnd != nil {
			numStart, numEnd := startEnd[4], startEnd[5]
			pattern = pattern[:numStart] + "{{EP_NUM}}" + pattern[numEnd:]
			goto Finalize
		}
	}

	// Heuristic C: Find last number, filtering out version/codec/year numbers
	{
		reNum := regexp.MustCompile(`\d+`)
		matches := reNum.FindAllStringIndex(pattern, -1)

		var bestMatch []int

		for _, m := range matches {
			start, end := m[0], m[1]
			val := pattern[start:end]

			// Skip version numbers (v264, h265)
			if start > 0 && (pattern[start-1] == 'v' || pattern[start-1] == 'V') {
				continue
			}
			if start > 0 && (pattern[start-1] == 'x' || pattern[start-1] == 'h') {
				if val == "264" || val == "265" {
					continue
				}
			}

			// Skip year-like numbers (1990-2029)
			if len(val) == 4 && (strings.HasPrefix(val, "19") || strings.HasPrefix(val, "20")) {
				continue
			}

			bestMatch = m
		}

		if bestMatch != nil {
			pattern = pattern[:bestMatch[0]] + "{{EP_NUM}}" + pattern[bestMatch[1]:]
		}
	}

Finalize:

	if len(ext) > 0 {
		return pattern + ".{{EXT}}"
	}
	return pattern
}
