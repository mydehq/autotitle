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

func TestScenario_SpaceSeparator(t *testing.T) {
	// Setup
	media := &types.Media{
		Title: "Test Series",
		Episodes: []types.Episode{
			{Number: 1, Title: "Episode 1"},
		},
	}

	target := &config.Target{
		Patterns: []config.Pattern{
			{
				Input: []string{"{{SERIES}} - {{EP_NUM}}"},
				Output: config.OutputConfig{
					Fields:    []string{"SERIES", "EP_NUM", "EP_NAME"},
					Separator: "  ", // Double space
				},
			},
		},
	}

	tmpDir := t.TempDir()
	filename := "Test Series - 01.mkv"
	if _, err := os.Create(filepath.Join(tmpDir, filename)); err != nil {
		t.Fatal(err)
	}

	mockDB := &MockDB{path: filepath.Join(tmpDir, "db")}
	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{})
	r.WithDryRun()

	ops, err := r.Execute(context.TODO(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	// Expected: "Test Series  01  Episode 1.mkv"
	expected := "Test Series  01  Episode 1.mkv"
	if filepath.Base(ops[0].TargetPath) != expected {
		t.Errorf("Wrong rename:\nGot:  %q\nWant: %q", filepath.Base(ops[0].TargetPath), expected)
	}
}
