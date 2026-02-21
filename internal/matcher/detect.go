package matcher

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reCRC       = regexp.MustCompile(`\[[A-Fa-f0-9]{8}\]`)
	reRes       = regexp.MustCompile(`(?i)\b(\d{3,4}p|\d{3,4}x\d{3,4})\b`)
	reSxxExx    = regexp.MustCompile(`(?i)(S\s*\d+\s*[Ex]\s*)(\d+)`)
	rePrefix    = regexp.MustCompile(`( - | Episode | Ep\.? | E\s+)(\d+)`)
	reNumber    = regexp.MustCompile(`\d+`)
	reBracketed = regexp.MustCompile(`\[([^\]]+)\]`)
)

// GuessPattern auto-detects a pattern from a filename
func GuessPattern(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if len(ext) > 0 {
		ext = strings.TrimPrefix(ext, ".")
	}

	pattern := base

	pattern = reCRC.ReplaceAllString(pattern, `[{{ANY}}]`)

	if loc := reRes.FindStringIndex(pattern); loc != nil {
		pattern = pattern[:loc[0]] + "{{RES}}" + pattern[loc[1]:]
	}

	// Mask leading tags: [Group] [v2] -> [{{ANY}}] [{{ANY}}]
	tagOffset := 0
	for {
		loc := reBracketed.FindStringSubmatchIndex(pattern[tagOffset:])
		if loc == nil {
			break
		}

		start, end := loc[0]+tagOffset, loc[1]+tagOffset
		content := pattern[loc[2]+tagOffset : loc[3]+tagOffset]

		// If it's not at the very start (ignoring leading metadata), stop
		prefix := strings.TrimSpace(pattern[:start])
		if prefix != "" {
			// Check if prefix is entirely made of {{ANY}} or {{RES}} blocks
			isAgnostic := true
			for _, part := range strings.Fields(prefix) {
				if part != "[{{ANY}}]" && part != "[{{RES}}]" {
					isAgnostic = false
					break
				}
			}
			if !isAgnostic {
				break
			}
		}
		if content == "{{ANY}}" || content == "{{RES}}" {
			tagOffset = end
			continue
		}

		// Replace tag content with {{ANY}}
		pattern = pattern[:start+1] + "{{ANY}}" + pattern[end-1:]
		tagOffset = start + len("[{{ANY}}]")
	}

	// SxxExx format
	if startEnd := reSxxExx.FindStringSubmatchIndex(pattern); startEnd != nil {
		prefixEnd := startEnd[3]
		numEnd := startEnd[5]
		pattern = pattern[:prefixEnd] + "{{EP_NUM}}" + pattern[numEnd:]
		goto Finalize
	}

	// Prefix patterns like " - 01" or " Episode 01"
	{
		if startEnd := rePrefix.FindStringSubmatchIndex(pattern); startEnd != nil {
			numStart, numEnd := startEnd[4], startEnd[5]
			pattern = pattern[:numStart] + "{{EP_NUM}}" + pattern[numEnd:]
			goto Finalize
		}
	}

	// Find last number, filtering out version/codec/year numbers
	{
		matches := reNumber.FindAllStringIndex(pattern, -1)

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
	// Mask episode title if present after the episode number
	pattern = maskTrailer(pattern)

	if len(ext) > 0 {
		return pattern + ".{{EXT}}"
	}
	return pattern
}

func maskTrailer(pattern string) string {
	idx := strings.Index(pattern, "{{EP_NUM}}")
	if idx == -1 {
		return pattern
	}

	trailer := pattern[idx+len("{{EP_NUM}}"):]
	if trailer == "" {
		return pattern
	}

	// Find the first occurrence of a separator block.
	// A separator block is a sequence of non-alphanumeric separator symbols
	// optionally surrounded by whitespace, OR just a sequence of whitespace.
	reSepBlock := regexp.MustCompile(`([ \t]*[-_.:|—]+[ \t]*|[ \t]+)`)
	loc := reSepBlock.FindStringIndex(trailer)

	if loc != nil {
		sIdx, eIdx := loc[0], loc[1]
		separator := trailer[sIdx:eIdx]

		// Found a separator. Check if there's significant content after it
		// and before any remaining metadata like [{{RES}}] or [{{ANY}}]
		remaining := trailer[eIdx:]

		// Find first metadata block [{{ANY}}] or [{{RES}}]
		metaRe := regexp.MustCompile(`\[\{\{[A-Z_]+\}\}\]`)
		m := metaRe.FindStringIndex(remaining)

		if m == nil {
			// No metadata, mask the whole remaining part if it's not empty
			if strings.TrimSpace(remaining) != "" {
				return pattern[:idx+len("{{EP_NUM}}")] + trailer[:sIdx] + separator + "{{ANY}}"
			}
		} else {
			// Mask only up to the metadata block
			titlePart := remaining[:m[0]]
			if strings.TrimSpace(titlePart) != "" {
				return pattern[:idx+len("{{EP_NUM}}")] + trailer[:sIdx] + separator + "{{ANY}} " + remaining[m[0]:]
			}
		}
	}
	return pattern
}
