# Integration Tests

This directory contains end-to-end integration tests that verify how `config`, `matcher`, and `renamer` work together. These tests simulate real-world usage scenarios without relying on external dependencies (databases or APIs are mocked).

## How to Run

Run all integration tests from the project root:

```bash
go test -v ./tests/...
```

## File Structure

*   **`setup_test.go`**: Contains shared helpers like `MockDB` and common setup logic. **Do not put specific test cases here.**
*   **`*_scenario_test.go`**: Each file represents a specific testing scenario (e.g., `anime_scenario_test.go` for complex anime naming patterns).

## Adding a New Test Scenario

1.  Create a new file: `tests/your_scenario_test.go`.
2.  Use the `tests` package (so you can access shared helpers).
3.  Simulate your `_autotitle.yml` config using `config.Target`.
4.  Mock the media data using `types.Media`.
5.  Execute the renamer and verify the output paths.

### Template

```go
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

func TestScenario_NewFeature(t *testing.T) {
	// 1. Setup Environment
	tmpDir := t.TempDir()
	
	// Create dummy input file
	filename := "Series - 01.mkv"
	if _, err := os.Create(filepath.Join(tmpDir, filename)); err != nil {
		t.Fatal(err)
	}

	// 2. Mock Media
	media := &types.Media{
		Title: "Test Series",
		Episodes: []types.Episode{
			{Number: 1, Title: "Episode Title"},
		},
	}

	// 3. Configure Target
	target := &config.Target{
		Path: tmpDir,
		Patterns: []config.Pattern{
			{
				Input: []string{"{{SERIES}} - {{EP_NUM}}"},
				Output: config.OutputConfig{
					Fields:    []string{"SERIES", "EP_NUM", "EP_NAME"},
					Separator: " - ",
				},
			},
		},
	}

	// 4. Execute
	// Uses MockDB from setup_test.go
	mockDB := &MockDB{path: filepath.Join(tmpDir, "db")}
	r := renamer.New(mockDB, types.BackupConfig{Enabled: false}, []string{})

	ops, err := r.Execute(context.Background(), tmpDir, target, media)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// 5. Verify
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	expected := "Test Series - 01 - Episode Title.mkv"
	if filepath.Base(ops[0].TargetPath) != expected {
		t.Errorf("Want %q, Got %q", expected, filepath.Base(ops[0].TargetPath))
	}
}
```
