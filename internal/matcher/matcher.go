package matcher

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	FieldGlue           = "+"
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
	// Raw map[string]string
}

type Pattern struct {
	raw      string
	regex    *regexp.Regexp
	idxEpNum int
	idxRes   int
}

// Compile compiles a template string into a regex pattern
func Compile(template string) (*Pattern, error) {
	templateBase := strings.ReplaceAll(template, "."+PlaceholderExt, "")
	templateBase = strings.ReplaceAll(templateBase, PlaceholderExt, "")

	regexStr := regexp.QuoteMeta(templateBase)

	// Replace placeholders with named capture groups
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderSeries), "(?P<Series>.+?)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderEpNum), "(?P<EpNum>\\d+)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderEpName), "(?P<EpName>.+?)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderFiller), "(?P<Filler>.*?)")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderRes), "(?P<Res>\\d{3,4}p|\\d{3,4}x\\d{3,4})")
	regexStr = strings.ReplaceAll(regexStr, regexp.QuoteMeta(PlaceholderAny), "(?P<Any>.*?)")

	regexStr = "^" + regexStr + "$"

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", template, err)
	}

	return &Pattern{
		raw:      template,
		regex:    re,
		idxEpNum: re.SubexpIndex("EpNum"),
		idxRes:   re.SubexpIndex("Res"),
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

	result["Ext"] = strings.TrimPrefix(ext, ".")

	return result
}

// MatchTyped attempts to match a filename and returns a structured result
func (p *Pattern) MatchTyped(filename string) (*MatchResult, bool) {
	ext := filepath.Ext(filename)
	if ext == "" {
		return nil, false
	}

	nameWithoutExt := strings.TrimSuffix(filename, ext)
	match := p.regex.FindStringSubmatch(nameWithoutExt)

	if match == nil {
		return nil, false
	}

	var epNum int
	if p.idxEpNum >= 0 && p.idxEpNum < len(match) {
		valStr := match[p.idxEpNum]
		if val, err := strconv.Atoi(valStr); err == nil {
			epNum = val
		}
	}

	var res string
	if p.idxRes >= 0 && p.idxRes < len(match) {
		res = match[p.idxRes]
	}

	return &MatchResult{
		EpisodeNum: epNum,
		Resolution: res,
		Extension:  strings.TrimPrefix(ext, "."),
	}, true
}

// GenerateFilenameFromFields builds filename from field list
// Fields can be field names (uppercase) or literal strings (must be quoted)
// Returns error if an unknown field is encountered
func GenerateFilenameFromFields(fields []string, separator string, vars TemplateVars, padding int) (string, error) {
	if padding <= 0 {
		padding = 3
	}

	var builder strings.Builder
	first := true
	suppressNextSep := false

	for _, field := range fields {

		// Handle Glue Operator
		if field == FieldGlue {
			suppressNextSep = true
			continue
		}

		value, err := resolveField(field, vars, padding)
		if err != nil {
			return "", err
		}

		// Skip empty values
		if value == "" {
			continue
		}

		if !first {
			if !suppressNextSep {
				builder.WriteString(separator)
			}
		}

		builder.WriteString(value)
		first = false
		suppressNextSep = false // Reset after consuming
	}

	builder.WriteString(".")
	builder.WriteString(vars.Ext)

	return builder.String(), nil
}

func resolveField(field string, vars TemplateVars, padding int) (string, error) {
	switch field {
	case "SERIES":
		return vars.Series, nil
	case "SERIES_EN":
		return vars.SeriesEn, nil
	case "SERIES_JP":
		return vars.SeriesJp, nil
	case "EP_NUM":
		return padNumber(vars.EpNum, padding), nil
	case "EP_NAME":
		return vars.EpName, nil
	case "FILLER":
		return vars.Filler, nil
	case "RES":
		return vars.Res, nil
	}

	// Not a keyword.
	// Check if it's explicitly quoted (to allow using "SERIES" as a literal)
	if len(field) >= 2 && field[0] == '"' && field[len(field)-1] == '"' {
		return field[1 : len(field)-1], nil
	}

	// Otherwise, treat as an implicit literal
	return field, nil
}

// padNumber pads a number string with zeros to width
func padNumber(s string, width int) string {

	if s == "" {
		return ""
	}

	if len(s) >= width {
		return s
	}

	needed := width - len(s)
	return strings.Repeat("0", needed) + s
}
