package matcher_test

import (
	"log"
	"testing"

	"github.com/mydehq/autotitle/internal/matcher"
)

func TestGuessPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "Standard Format with Brackets",
			filename: "[Sub] Series - 01 [1080p].mkv",
			want:     "[Sub] Series - {{EP_NUM}} [{{RES}}].{{EXT}}",
		},
		{
			name:     "Space Separated",
			filename: "Series 01.mp4",
			want:     "Series {{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "Dot Separated",
			filename: "Series.E01.mkv",
			want:     "Series.E{{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "No Resolution",
			filename: "Series - 01.avi",
			want:     "Series - {{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "Multiple Brackets",
			filename: "[Group][1080p] Series - 01.mkv",
			want:     "[Group][{{RES}}] Series - {{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "SxxExx Format",
			filename: "Series S01E02.mkv",
			want:     "Series S01E{{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "Episode Keyword",
			filename: "Series Episode 12.mkv",
			want:     "Series Episode {{EP_NUM}}.{{EXT}}",
		},
		{
			name:     "CRC masking",
			filename: "[Group] Series - 01 [1A2B3C4D].mkv",
			want:     "[Group] Series - {{EP_NUM}} [{{ANY}}].{{EXT}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.GuessPattern(tt.filename)
			if got != tt.want {
				t.Errorf("GuessPattern(%q) = %q; want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestGenerateFilenameFromFields(t *testing.T) {
	tests := []struct {
		name      string
		fields    []string
		separator string
		vars      matcher.TemplateVars
		want      string
	}{
		{
			name:      "All fields populated",
			fields:    []string{"SERIES", "EP_NUM", "FILLER", "EP_NAME"},
			separator: " - ",
			vars: matcher.TemplateVars{
				Series: "Test Series",
				EpNum:  "1",
				EpName: "The Beginning",
				Filler: "[F]",
				Ext:    "mkv",
			},
			want: "Test Series - 001 - [F] - The Beginning.mkv",
		},
		{
			name:      "Empty FILLER auto-skipped",
			fields:    []string{"SERIES", "EP_NUM", "FILLER", "EP_NAME"},
			separator: " - ",
			vars: matcher.TemplateVars{
				Series: "Test Series",
				EpNum:  "1",
				EpName: "The Beginning",
				Filler: "", // Empty - should be skipped
				Ext:    "mkv",
			},
			want: "Test Series - 001 - The Beginning.mkv",
		},
		{
			name:      "Multiple empty fields skipped",
			fields:    []string{"SERIES", "EP_NUM", "RES", "FILLER", "EP_NAME"},
			separator: " - ",
			vars: matcher.TemplateVars{
				Series: "Test",
				EpNum:  "1",
				EpName: "Episode",
				Filler: "", // Empty
				Res:    "", // Empty
				Ext:    "mkv",
			},
			want: "Test - 001 - Episode.mkv",
		},
		{
			name:      "With literal prefix",
			fields:    []string{"\"DC\"", "EP_NUM", "FILLER", "EP_NAME"},
			separator: " - ",
			vars: matcher.TemplateVars{
				EpNum:  "5",
				EpName: "Title",
				Filler: "[F]",
				Ext:    "mkv",
			},
			want: "DC - 005 - [F] - Title.mkv",
		},
		{
			name:      "Different separator",
			fields:    []string{"SERIES", "EP_NUM", "EP_NAME"},
			separator: "_",
			vars: matcher.TemplateVars{
				Series: "Show",
				EpNum:  "10",
				EpName: "Test",
				Ext:    "mp4",
			},
			want: "Show_010_Test.mp4",
		},
		{
			name:      "Mixed literals and fields",
			fields:    []string{"\"DC\"", "EP_NUM", "\"[Filler]\"", "EP_NAME"},
			separator: " ",
			vars: matcher.TemplateVars{
				EpNum:  "3",
				EpName: "Example",
				Ext:    "mkv",
			},
			want: "DC 003 [Filler] Example.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matcher.GenerateFilenameFromFields(tt.fields, tt.separator, tt.vars, 0)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
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

	p, err := matcher.Compile(template)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	match := p.Match(filename)
	if match == nil {
		t.Fatal("Match() returned nil")
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
	// Greedy matchers (.+) would capture "Subs] [v2" for {{ANY}}
	template := "[{{ANY}}] {{SERIES}} - {{EP_NUM}}.{{EXT}}"
	filename := "[Subs] [v2] My show - 01.mkv"

	p, err := matcher.Compile(template)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	match := p.Match(filename)
	if match == nil {
		t.Fatal("Match() returned nil")
	}

	if got := match["Any"]; got != "Subs" {
		t.Errorf("Match[\"Any\"] = %q; want \"Subs\" (Greedy match detected!)", got)
	}
	if got := match["Series"]; got != "[v2] My show" {
		t.Errorf("Match[\"Series\"] = %q; want \"[v2] My show\"", got)
	}
}
