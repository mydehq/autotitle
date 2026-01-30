package config

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/mydehq/autotitle/internal/matcher"
)

// ScanResult holds the results of directory scanning
type ScanResult struct {
	DetectedPatterns []string
	HasMedia         bool
	TotalFiles       int
}

// Scan scans a directory for media files and guesses renaming patterns.
// It uses the provided formats list to identify relevant files.
func Scan(dir string, formats []string) (*ScanResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		TotalFiles: len(entries),
	}

	seenPatterns := make(map[string]bool)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := filepath.Ext(e.Name())
		if len(ext) > 0 {
			ext = ext[1:] // Remove leading dot
		}

		if slices.Contains(formats, ext) {
			result.HasMedia = true
			p := matcher.GuessPattern(e.Name())
			if p != "" && !seenPatterns[p] {
				result.DetectedPatterns = append(result.DetectedPatterns, p)
				seenPatterns[p] = true
			}
		}
	}

	return result, nil
}
