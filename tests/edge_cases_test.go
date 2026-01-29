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

func TestScenario_EdgeCases(t *testing.T) {
	t.Run("FillerEpisodes", testFillerEpisodes)
	t.Run("OffsetOutOfRange", testOffsetOutOfRange)
	t.Run("UnmatchedFiles", testUnmatchedFiles)
}

func testFillerEpisodes(t *testing.T) {
	tmpDir := t.TempDir()
	// File matches Ep 5
	files := []string{"Series - 05.mkv", "Series - 06.mkv"}
	for _, f := range files {
		os.Create(filepath.Join(tmpDir, f))
	}

	mockDB := &MockDB{path: reqPath(tmpDir)}

	// Ep 5 is filler, Ep 6 is canon
	media := &types.Media{
		Title: "Naruto",
		Episodes: []types.Episode{
			{Number: 5, Title: "Filler Arc Start", IsFiller: true},
			{Number: 6, Title: "Canon Arc Start", IsFiller: false},
		},
	}

	target := &config.Target{
		Patterns: []config.Pattern{{
			Input: []string{"{{SERIES}} - {{EP_NUM}}"},
			Output: config.OutputConfig{
				Fields:    []string{"SERIES", "EP_NUM", "FILLER", "EP_NAME"},
				Separator: " - ",
			},
		}},
	}

	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{})
	ops, err := r.Execute(context.TODO(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := map[string]string{
		"Series - 05.mkv": "Naruto - 05 - [F] - Filler Arc Start.mkv",
		"Series - 06.mkv": "Naruto - 06 - Canon Arc Start.mkv", // Empty filler field skipped
	}

	for _, op := range ops {
		base := filepath.Base(op.SourcePath)
		want := expected[base]
		got := filepath.Base(op.TargetPath)
		if got != want {
			t.Errorf("File %s: Want %q, Got %q", base, want, got)
		}
	}
}

func testOffsetOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	os.Create(filepath.Join(tmpDir, "Series - 01.mkv"))

	mockDB := &MockDB{path: reqPath(tmpDir)}
	media := &types.Media{
		Title: "Mini Series",
		Episodes: []types.Episode{
			{Number: 1, Title: "Ep 1"},
		},
	}

	target := &config.Target{
		Patterns: []config.Pattern{{
			Input: []string{"{{SERIES}} - {{EP_NUM}}"},
			Output: config.OutputConfig{
				Fields: []string{"SERIES", "EP_NUM"},
				Offset: 100, // Maps Ep 1 -> 101 (Which doesn't exist)
			},
		}},
	}

	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{})
	ops, err := r.Execute(context.TODO(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should produce NO operations or Skipped/Failed?
	// If episode not found, Renamer currently emits a warning and skips adding to operations??
	// Let's check logic: if ep == nil { emit warning; continue }
	// So ops should be empty.
	if len(ops) != 0 {
		t.Errorf("Expected 0 operations for out-of-range episode, got %d", len(ops))
	}
}

func testUnmatchedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	os.Create(filepath.Join(tmpDir, "Series - 01.mkv"))
	os.Create(filepath.Join(tmpDir, "readme.txt"))       // Should be ignored
	os.Create(filepath.Join(tmpDir, "Unknown - 99.avi")) // Should be ignored (pattern mismatch)

	mockDB := &MockDB{path: reqPath(tmpDir)}
	media := &types.Media{
		Title:    "Series",
		Episodes: []types.Episode{{Number: 1, Title: "Ep 1"}},
	}

	target := &config.Target{
		Patterns: []config.Pattern{{
			Input:  []string{"Series - {{EP_NUM}}"}, // Strict matching
			Output: config.OutputConfig{Fields: []string{"SERIES", "EP_NUM"}},
		}},
	}

	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{})
	ops, err := r.Execute(context.TODO(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(ops) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(ops))
	}
	if len(ops) > 0 && filepath.Base(ops[0].SourcePath) != "Series - 01.mkv" {
		t.Errorf("Unexpected file matched: %s", filepath.Base(ops[0].SourcePath))
	}
}

func reqPath(s string) string { return filepath.Join(s, "db") }
