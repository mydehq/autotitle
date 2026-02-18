package renamer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/types"
)

// MockDB implements types.DatabaseRepository for testing
type MockDB struct {
	types.DatabaseRepository
}

func (m *MockDB) Path() string {
	return "/tmp/test"
}

func TestRenamer_Offset(t *testing.T) {
	// Setup
	media := &types.Media{
		Title: "Test Series",
		Episodes: []types.Episode{
			{Number: 1, Title: "Episode 1"},
			{Number: 11, Title: "Episode 11"},
		},
	}

	target := &config.Target{
		Patterns: []config.Pattern{
			{
				Input: []string{"{{SERIES}} - {{EP_NUM}}"}, // "Test Series - 01" (ext stripped)
				Output: config.OutputConfig{
					Fields:    []string{"SERIES", "EP_NUM", "EP_NAME"},
					Separator: " - ",
				},
			},
		},
	}

	// Create temp dir
	tmpDir := t.TempDir()

	// Create dummy file: "Test Series - 01.mkv"
	// 01 + 10 (offset) = 11 -> matches Episode 11
	filename := "Test Series - 01.mkv"
	f, err := os.Create(filepath.Join(tmpDir, filename))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Init renamer
	r := New(&MockDB{}, types.BackupConfig{Enabled: false}, []string{"mkv"})
	r.WithOffset(10)
	r.WithDryRun()

	ops, err := r.Execute(context.Background(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Episode.Number != 11 {
		t.Errorf("Expected matched episode number 11, got %d", op.Episode.Number)
	}

	// Check target filename contains "11"
	// Template uses "EpNum" which is from op.Episode.Number
	// expected: "Test Series - 011 - Episode 11.mkv" (based on fields: SERIES, EP_NUM, EP_NAME)
	// Output fields: SERIES, EP_NUM, EP_NAME
	// Default separator " - "
	// Auto-padding: Max ep is 11 -> 2 digits.
	expected := "Test Series - 11 - Episode 11.mkv"
	if filepath.Base(op.TargetPath) != expected {
		t.Errorf("Expected target path %s, got %s", expected, filepath.Base(op.TargetPath))
	}
}

func TestRenamer_OffsetZeroOverride(t *testing.T) {
	// Setup
	media := &types.Media{
		Title: "Test Series",
		Episodes: []types.Episode{
			{Number: 1, Title: "Episode 1"},
			{Number: 11, Title: "Episode 11"},
		},
	}

	target := &config.Target{
		Patterns: []config.Pattern{
			{
				Input: []string{"{{SERIES}} - {{EP_NUM}}"},
				Output: config.OutputConfig{
					Fields:    []string{"SERIES", "EP_NUM", "EP_NAME"},
					Separator: " - ",
					Offset:    10, // Configured offset
				},
			},
		},
	}

	tmpDir := t.TempDir()
	filename := "Test Series - 01.mkv"
	f, err := os.Create(filepath.Join(tmpDir, filename))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	r := New(&MockDB{}, types.BackupConfig{Enabled: false}, []string{"mkv"})
	r.WithOffset(0) // Explicitly set to 0 to override config
	r.WithDryRun()

	ops, err := r.Execute(context.Background(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	// Expect 1 (override), not 11 (config)
	if op.Episode.Number != 1 {
		t.Errorf("Expected matched episode number 1, got %d", op.Episode.Number)
	}
}
