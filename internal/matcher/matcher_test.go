package matcher_test

import (
	"log"
	"testing"

	"github.com/soymadip/autotitle/internal/matcher"
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

func TestGenerateFilename(t *testing.T) {
	vars := matcher.TemplateVars{
		Series: "Test Series",
		EpNum:  "1",
		EpName: "Test Episode",
		Filler: "",
		Res:    "1080p",
		Ext:    "mkv",
	}

	tests := []struct {
		name     string
		template string
		vars     matcher.TemplateVars
		want     string
	}{
		{
			name:     "Standard Template",
			template: "{{SERIES}} - {{EP_NUM}} - {{EP_NAME}}.{{EXT}}",
			vars:     vars,
			want:     "Test Series - 001 - Test Episode.mkv",
		},
		{
			name:     "With Filler",
			template: "{{SERIES}} - {{EP_NUM}} {{FILLER}}.{{EXT}}",
			vars: matcher.TemplateVars{
				Series: "Test Series",
				EpNum:  "2",
				Filler: "[F]",
				Ext:    "mkv",
			},
			want: "Test Series - 002 [F].mkv",
		},
		{
			name:     "With Resolution",
			template: "[{{RES}}] {{SERIES}} - {{EP_NUM}}.{{EXT}}",
			vars:     vars,
			want:     "[1080p] Test Series - 001.mkv",
		},
		{
			name:     "Clean Spacing",
			template: "{{SERIES}} {{EP_NUM}} {{FILLER}}.{{EXT}}", // Filler empty
			vars:     vars,
			want:     "Test Series 001 .mkv", // Double space reduced to single space, but space before . remains
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.GenerateFilename(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("GenerateFilename() = %q; want %q", got, tt.want)
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
