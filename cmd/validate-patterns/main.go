package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mydehq/autotitle/internal/matcher"
)

func main() {
	root := "/home/soymadip/MF/Media/Oregairu S01-S03+OVA/S02/Series"
	formats := []string{".mkv", ".mp4", ".avi", ".webm"}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		isMedia := false
		for _, f := range formats {
			if strings.EqualFold(ext, f) {
				isMedia = true
				break
			}
		}

		if isMedia {
			filename := filepath.Base(path)
			pattern := matcher.GuessPattern(filename)
			fmt.Printf("File: %s\nGUESS: %s\n\n", filename, pattern)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking path: %v\n", err)
	}
}
