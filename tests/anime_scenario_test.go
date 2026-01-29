package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/renamer"
	"github.com/mydehq/autotitle/internal/types"
)

func TestScenario_SampleAnime(t *testing.T) {
	// 1. Setup Environment
	tmpDir := t.TempDir()

	// Create dummy files with different naming patterns
	files := []string{
		"[Sub] My Anime - 01.mkv", // Pattern A
		"My Anime E02.mp4",        // Pattern B
	}
	for _, f := range files {
		if _, err := os.Create(filepath.Join(tmpDir, f)); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Construct Mock Media
	episodes := []types.Episode{}
	for i := 1; i <= 24; i++ {
		title := "Generic Episode Title"
		if i == 13 {
			title = "The Beginning of Season 2"
		}
		if i == 14 {
			title = "The Continuation"
		}
		episodes = append(episodes, types.Episode{
			Number: i,
			Title:  title,
		})
	}
	media := &types.Media{
		Title:        "My Anime",
		EpisodeCount: 24,
		Episodes:     episodes,
	}

	// 3. Configure Target
	target := &config.Target{
		Path: tmpDir,
		Patterns: []config.Pattern{
			{
				Input: []string{
					"[Sub] My Anime - {{EP_NUM}}.{{EXT}}",
					"My Anime E{{EP_NUM}}.{{EXT}}",
				},
				Output: config.OutputConfig{
					Fields:    []string{"S2", "+", "EP_NUM", " - ", "EP_NAME"},
					Separator: "", // Explicitly empty
					Offset:    12,
				},
			},
		},
	}

	// 4. Execute
	mockDB := &MockDB{path: filepath.Join(tmpDir, "db")}
	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{"mkv", "mp4"})

	ops, err := r.Execute(context.Background(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 5. Verify
	expected := map[string]string{
		"[Sub] My Anime - 01.mkv": "S213 - The Beginning of Season 2.mkv",
		"My Anime E02.mp4":        "S214 - The Continuation.mp4",
	}

	if len(ops) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(ops))
	}

	for _, op := range ops {
		baseSource := filepath.Base(op.SourcePath)
		baseTarget := filepath.Base(op.TargetPath)

		want, ok := expected[baseSource]
		if !ok {
			t.Errorf("Unexpected source file processed: %s", baseSource)
			continue
		}

		if baseTarget != want {
			t.Errorf("Wrong rename for %s:\nGot:  %s\nWant: %s", baseSource, baseTarget, want)
		}
	}
}
