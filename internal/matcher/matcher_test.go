package matcher

import (
	"log"
	"testing"
)

func TestGuessPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"Standard Format with Brackets", "[Sub] Series - 01 [1080p].mkv", "[{{ANY}}] Series - {{EP_NUM}} [{{RES}}].{{EXT}}"},
		{"Space Separated", "Series - 01.mkv", "Series - {{EP_NUM}}.{{EXT}}"},
		{"Dot Separated", "Series.01.mkv", "Series.{{EP_NUM}}.{{EXT}}"},
		{"SxxExx Format", "Series S01E01.mkv", "Series S01E{{EP_NUM}}.{{EXT}}"},
		{"Episode Keyword", "Series Episode 01.mkv", "Series Episode {{EP_NUM}}.{{EXT}}"},
		{"CRC masking", "[Group] Series - 01 [1A2B3C4D].mkv", "[{{ANY}}] Series - {{EP_NUM}} [{{ANY}}].{{EXT}}"},
		{"Hyphen Separated Title", "S01E01-Title.mkv", "S01E{{EP_NUM}}-{{ANY}}.{{EXT}}"},
		{"Underscore Separated Title", "ss_ep1_lsjflsjfsl.mkv", "ss_ep{{EP_NUM}}_{{ANY}}.{{EXT}}"},
		{"Dot Separated Title", "Series.S01E01.Title.mkv", "Series.S01E{{EP_NUM}}.{{ANY}}.{{EXT}}"},
		{"Space Separated Title", "S01E01 Title.mkv", "S01E{{EP_NUM}} {{ANY}}.{{EXT}}"},
		{"Double Underscore", "S01E01__Title.mkv", "S01E{{EP_NUM}}__{{ANY}}.{{EXT}}"},
		{"Spaced Hyphen", "S01E01 - Title.mkv", "S01E{{EP_NUM}} - {{ANY}}.{{EXT}}"},
		{"Triple Hyphen", "S01E01---Title.mkv", "S01E{{EP_NUM}}---{{ANY}}.{{EXT}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuessPattern(tt.filename); got != tt.want {
				t.Errorf("GuessPattern(%q) = %q; want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestGenerateFilenameFromFields(t *testing.T) {
	vars := TemplateVars{
		Series: "Test Series",
		EpNum:  "1",
		EpName: "Episode Title",
		Res:    "1080p",
		Ext:    "mkv",
	}

	tests := []struct {
		name      string
		fields    []string
		separator string
		padding   int
		want      string
	}{
		{
			"All fields populated",
			[]string{"SERIES", " - ", "EP_NUM", " - ", "EP_NAME"},
			"",
			3,
			"Test Series - 001 - Episode Title.mkv",
		},
		{
			"Empty FILLER auto-skipped",
			[]string{"SERIES", " ", "FILLER", " - ", "EP_NUM"},
			"",
			2,
			"Test Series  - 01.mkv",
		},
		{
			"Multiple empty fields skipped",
			[]string{"SERIES", "FILLER", "RES", " - ", "EP_NUM"},
			"",
			3,
			"Test Series1080p - 001.mkv",
		},
		{
			"With literal prefix and glue",
			[]string{"\"S1\"", "+", "EP_NUM", " - ", "EP_NAME"},
			"",
			2,
			"S101 - Episode Title.mkv",
		},
		{
			"Different separator",
			[]string{"SERIES", "EP_NUM"},
			".",
			2,
			"Test Series.01.mkv",
		},
		{
			"Mixed literals and fields",
			[]string{"\"[Draft]\"", "SERIES", " - ", "EP_NUM"},
			" ",
			3,
			"[Draft] Test Series  -  001.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateFilenameFromFields(tt.fields, tt.separator, vars, tt.padding)
			if err != nil {
				t.Fatalf("GenerateFilenameFromFields() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GenerateFilenameFromFields() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestCompileAndMatch(t *testing.T) {
	template := "{{SERIES}} - {{EP_NUM}} [{{RES}}].{{EXT}}"
	filename := "Test Anime - 01 [1080p].mkv"

	p, err := Compile(template)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	match := p.Match(filename)
	if match == nil {
		t.Fatalf("Match() returned nil. Regex: %s", p.String())
	}

	log.Printf("Match: %v", match)

	tests := []struct {
		key  string
		want string
	}{
		{"Series", "Test Anime"},
		{"EpNum", "01"},
		{"Res", "1080p"},
		{"Ext", "mkv"},
	}

	for _, tt := range tests {
		if got := match[tt.key]; got != tt.want {
			t.Errorf("Match[%q] = %q; want %q", tt.key, got, tt.want)
		}
	}
}

func TestNonGreedyMatch(t *testing.T) {
	template := "[{{ANY}}] {{SERIES}} - {{EP_NUM}}.{{EXT}}"
	filename := "[Subs] [v2] My show - 01.mkv"

	p, err := Compile(template)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	match := p.Match(filename)
	if match == nil {
		t.Fatalf("Match() returned nil. Regex: %s", p.String())
	}

	if match["Any"] != "Subs" {
		t.Errorf("Match[\"Any\"] = %q; want \"Subs\"", match["Any"])
	}
	if got := match["Series"]; got != "[v2] My show" {
		t.Errorf("Match[\"Series\"] = %q; want \"[v2] My show\"", got)
	}
}

func TestMultiplePlaceholders(t *testing.T) {
	template := "[{{ANY}}] [{{ANY}}] {{SERIES}} - {{EP_NUM}}.{{EXT}}"
	filename := "[Subs] [v2] My show - 01.mkv"

	p, err := Compile(template)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	match := p.Match(filename)
	if match == nil {
		t.Fatalf("Match() returned nil. Regex: %s", p.String())
	}

	if match["Any_1"] != "Subs" {
		t.Errorf("Any_1 = %q, want %q", match["Any_1"], "Subs")
	}
	if match["Any_2"] != "v2" {
		t.Errorf("Any_2 = %q, want %q", match["Any_2"], "v2")
	}
	if match["Series"] != "My show" {
		t.Errorf("Series = %q, want %q", match["Series"], "My show")
	}
}
