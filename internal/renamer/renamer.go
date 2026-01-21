package renamer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/database"
	"github.com/mydehq/autotitle/internal/fetcher"
	"github.com/mydehq/autotitle/internal/logger"
	"github.com/mydehq/autotitle/internal/matcher"
)

// Renamer orchestrates the file renaming process
type Renamer struct {
	Config    *config.GlobalConfig
	MapConfig *config.MapConfig
	DB        *database.DB
	DryRun    bool
	NoBackup  bool
	Verbose   bool
	Quiet     bool
}

type FileResult struct {
	Original  string
	NewName   string
	Error     error
	IsRenamed bool
}

// Execute performs the renaming on the target directory
func (r *Renamer) Execute(targetPath string) error {
	// 1. Resolve configuration for this target
	// The map config can have multiple targets, we need to find the one matching path
	// But LoadMap was called with 'targetPath' already?
	// If the user runs `autotitle .`, we load ./_autotitle.yml.
	// If the user runs `autotitle /path/to/anime`, we load /path/to/anime/_autotitle.yml.
	// We need to resolve the specific config for the current directory relative to the map file.

	// Assuming targetPath is absolute or relative to CWD.
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}

	targetConfig, err := r.MapConfig.ResolveTarget(".")
	// We are running IN the directory implied by MapConfig loading (usually targetPath).
	// If MapConfig was loaded from targetPath, then "." is the target we want.
	if err != nil {
		return fmt.Errorf("failed to resolve target config: %w", err)
	}

	// 2. Load Database
	// We need the series ID/Slug to load the DB.
	// We use the MALURL from config as the Series Slug identifier now that we use URLs.
	if targetConfig.MALURL == "" {
		return fmt.Errorf("mal_url is missing in configuration")
	}

	// The Database matches the slug logic used in 'db gen'.
	// In 'db gen', we now use 'mal-{ID}'.
	// We extract ID from the configured MAL URL.
	malID := fetcher.ExtractMALID(targetConfig.MALURL)
	if malID == 0 {
		return fmt.Errorf("invalid mal_url in config: could not extract ID")
	}

	seriesSlug := fmt.Sprintf("%d", malID)

	seriesData, err := r.DB.Load(seriesSlug)
	if err != nil {
		return fmt.Errorf("failed to load database for '%s' (ID: %d): %w", seriesSlug, malID, err)
	}
	if seriesData == nil {
		return fmt.Errorf("database not found for '%s'. Run 'autotitle db gen' first.", seriesSlug)
	}

	// 3. Scan Files
	files, err := r.scanFiles(absPath)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		logger.Warn("No video files found in %s", absPath)
		return nil
	}

	// 4. Match and Plan
	var plans []FileResult

	// Compile patterns from config
	patterns := targetConfig.Patterns
	if len(patterns) == 0 {
		patterns = r.Config.Patterns
	}

	// Output template fallback
	defaultOutput := r.Config.Output

	type PatternPair struct {
		Matcher *matcher.Pattern
		Output  string
	}

	var pairs []PatternPair

	for _, p := range patterns {
		out := p.Output

		if out == "" {
			out = defaultOutput
		}

		for _, inputTmpl := range p.Input {
			cp, err := matcher.Compile(inputTmpl)

			if err != nil {
				logger.Error("Pattern error: %v", err)
				continue
			}

			pairs = append(pairs, PatternPair{Matcher: cp, Output: out})
		}
	}

	// Process files
	for _, file := range files {
		// Try to match
		var bestMatch map[string]string
		var outputTmpl string

		matched := false
		for _, pair := range pairs {
			vars := pair.Matcher.Match(filepath.Base(file))
			if vars != nil {
				bestMatch = vars
				outputTmpl = pair.Output
				matched = true
				break
			}
		}

		if !matched {
			if r.Verbose {
				logger.Info("Skipping non-matching file: %s", filepath.Base(file))
			}
			continue
		}

		// Enrich data
		epNumStr := bestMatch["EpNum"]
		epNum, _ := parseEpNum(epNumStr) // Handles "01", "1" etc

		epData, ok := seriesData.Episodes[epNum]
		if !ok {
			if r.Verbose {
				logger.Warn("Episode %d not found in database for file %s", epNum, filepath.Base(file))
			}
		}

		// Use anime title from database
		seriesName := seriesData.Title
		if seriesName == "" {
			seriesName = "Unknown"
		}

		// Construct TemplateVars
		tv := matcher.TemplateVars{
			Series: seriesName,
			EpNum:  epNumStr,
			EpName: epData.Title,
			Res:    bestMatch["Res"],
			Ext:    bestMatch["Ext"],
		}
		// Filler handling
		if epData.Filler {
			tv.Filler = "[F]" // Standardize or config? user might want generic filler tag
		}

		newName := matcher.GenerateFilename(outputTmpl, tv)

		// If no change, skip
		if newName == filepath.Base(file) {
			continue
		}

		plans = append(plans, FileResult{
			Original: file,
			NewName:  filepath.Join(filepath.Dir(file), newName),
		})
	}

	if len(plans) == 0 {
		if !r.Quiet {
			logger.Info("No files to rename.")
		}
		return nil
	}

	// 5. Confirm?? (Not in v1 plan explicitly, but recommended)
	// Plan says .
	// We'll proceed.

	// 6. Perform Rename
	// Backup Dir setup
	backupDir := filepath.Join(absPath, r.Config.Backup.DirName)
	if !r.DryRun && !r.NoBackup {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup dir: %w", err)
		}
	}

	// Undo Log (map of original -> new relative paths? or just rely on backup)
	// Undo command restores from backupDir to parent.
	// Simple copy back.

	for _, plan := range plans {
		dest := plan.NewName

		if r.DryRun {
			if !r.Quiet {
				// Use relative paths for cleaner output
				relOld, _ := filepath.Rel(absPath, plan.Original)
				relNew, _ := filepath.Rel(absPath, dest)
				fmt.Printf("%s -> %s\n", relOld, relNew)
			}
			continue
		}

		// Backup first
		if !r.NoBackup {
			// Copy original to backup
			// We flatten structure or keep it? Flatten is easier for restore if filenames unique.
			// But duplicate filenames? unlikley for same folder.
			// Just copy basename.
			backupPath := filepath.Join(backupDir, filepath.Base(plan.Original))
			if err := copyFile(plan.Original, backupPath); err != nil {
				if !r.Quiet {
					logger.Error("Backup failed for %s: %v", filepath.Base(plan.Original), err)
				}
				// Stop processing this file?
				continue
			}
		}

		// Rename
		if err := os.Rename(plan.Original, dest); err != nil {
			if !r.Quiet {
				logger.Error("Rename failed: %v", err)
			}
		}
	}

	if !r.Quiet {
		logger.Success("Processed %d files.", len(plans))
	}

	return nil
}

// scanFiles returns list of supported video files
func (r *Renamer) scanFiles(root string) ([]string, error) {
	var files []string
	formats := make(map[string]bool)
	for _, f := range r.Config.Formats {
		formats["."+strings.ToLower(f)] = true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if formats[ext] {
			files = append(files, filepath.Join(root, entry.Name()))
		}
	}
	return files, nil
}

// Clean removes the backup directory
func (r *Renamer) Clean(targetPath string) error {
	backupDir := filepath.Join(targetPath, r.Config.Backup.DirName)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("no backup directory found at %s", backupDir)
	}
	return os.RemoveAll(backupDir)
}

// Undo restores files from backup
func (r *Renamer) Undo(targetPath string) error {
	backupDir := filepath.Join(targetPath, r.Config.Backup.DirName)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("no backup directory found to undo from")
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		src := filepath.Join(backupDir, entry.Name())
		dest := filepath.Join(targetPath, entry.Name())

		// Move back (overwrite existing if renamed one is there? yes, restore requests authority)
		// But new file might have different name. We are just putting old file back.
		// What about the renamed file? It becomes orphan.
		// Ideally we delete the renamed file too, but we don't track it without a log.
		// The requirement was "Restores files from the _backup directory".
		// Simple restore is enough for v1.

		if err := os.Rename(src, dest); err != nil {
			logger.Error("Failed to restore %s: %v", entry.Name(), err)
		} else {
			count++
		}
	}

	// Remove empty backup dir?
	os.Remove(backupDir)

	logger.Success("Restored %d files.", count)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func parseEpNum(s string) (int, error) {
	// Atoi handles "01" as 1
	return strconv.Atoi(s)
}

