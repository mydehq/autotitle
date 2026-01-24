package matcher

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	PlaceholderSeries   = "{{SERIES}}"
	PlaceholderSeriesEn = "{{SERIES_EN}}"
	PlaceholderSeriesJp = "{{SERIES_JP}}"
	PlaceholderEpNum    = "{{EP_NUM}}"
	PlaceholderEpName   = "{{EP_NAME}}"
	PlaceholderFiller   = "{{FILLER}}"
	PlaceholderRes      = "{{RES}}"
	PlaceholderExt      = "{{EXT}}"
	PlaceholderAny      = "{{ANY}}"
)

type TemplateVars struct {
	Series   string
	SeriesEn string
	SeriesJp string
	EpNum    string
	EpName   string
	Filler   string
	Res      string
	Ext      string
}

// MatchResult contains extracted values from a filename match
type MatchResult struct {
	EpisodeNum int
	Resolution string
	Extension  string
	Raw        map[string]string // All captured groups
}

type Pattern struct {
	raw   string
	regex *regexp.Regexp
}

// Compile compiles a template string into a regex pattern
func Compile(template string) (*Pattern, error) {
	// Remove {{EXT}} from template as matching is done on base filename
	templateBase := strings.ReplaceAll(template, "."+PlaceholderExt, "")
	templateBase = strings.ReplaceAll(templateBase, PlaceholderExt, "")

	regexStr := regexp.QuoteMeta(templateBase)

	// Replace placeholders with named capture groups
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderSeries), "(?P<Series>.+)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderEpNum), "(?P<EpNum>\\d+)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderEpName), "(?P<EpName>.+)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderFiller), "(?P<Filler>.*)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderRes), "(?P<Res>\\d{3,4}p|\\d{3,4}x\\d{3,4})")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderAny), "(?P<Any>.*)")

	// Anchor the regex to match full string
	regexStr = "^" + regexStr + "$"

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", template, err)
	}

	return &Pattern{
		raw:   template,
		regex: re,
	}, nil
}

// Match attempts to match a filename against the compiled pattern
func (p *Pattern) Match(filename string) map[string]string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return nil
	}

	nameWithoutExt := strings.TrimSuffix(filename, ext)
	match := p.regex.FindStringSubmatch(nameWithoutExt)

	if match == nil {
		return nil
	}

	result := make(map[string]string)
	for i, name := range p.regex.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	// Always populate Ext from file extension
	result["Ext"] = strings.TrimPrefix(ext, ".")

	return result
}

// MatchTyped attempts to match a filename and returns a structured result
func (p *Pattern) MatchTyped(filename string) (*MatchResult, bool) {
	raw := p.Match(filename)
	if raw == nil {
		return nil, false
	}

	var epNum int
	if ep, ok := raw["EpNum"]; ok && ep != "" {
		fmt.Sscanf(ep, "%d", &epNum)
	}

	return &MatchResult{
		EpisodeNum: epNum,
		Resolution: raw["Res"],
		Extension:  raw["Ext"],
		Raw:        raw,
	}, true
}

// GenerateFilenameFromFields builds filename from field list
// Fields can be field names (uppercase) or literal strings
// Only adds separator between non-empty values - no cleanup needed
func GenerateFilenameFromFields(fields []string, separator string, vars TemplateVars) string {
	if separator == "" {
		separator = " - "
	}

	fieldValues := map[string]string{
		"SERIES":    vars.Series,
		"SERIES_EN": vars.SeriesEn,
		"SERIES_JP": vars.SeriesJp,
		"EP_NUM":    padNumber(vars.EpNum, 3),
		"EP_NAME":   vars.EpName,
		"FILLER":    vars.Filler,
		"RES":       vars.Res,
	}

	var parts []string
	for _, field := range fields {
		// Check if it's a field name (uppercase)
		if value, ok := fieldValues[field]; ok {
			if value != "" {
				parts = append(parts, value)
			}
		} else {
			// It's a literal string - always include
			parts = append(parts, field)
		}
	}

	return strings.Join(parts, separator) + "." + vars.Ext
}

// padNumber pads a number string with zeros to width
func padNumber(s string, width int) string {
	if s == "" {
		return ""
	}

	// Parse as int to normalize
	var n int

	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return s // Return original if not a number
	}

	// Format with padding
	format := fmt.Sprintf("%%0%dd", width)
	return fmt.Sprintf(format, n)
}
