// Package renamer handles file renaming operations.
package renamer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mydehq/autotitle/internal/backup"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/types"
)

// Renamer handles file renaming operations
type Renamer struct {
	DB            types.DatabaseRepository
	BackupManager types.BackupManager
	Events        types.EventHandler
	DryRun        bool
	NoBackup      bool
	BackupConfig  types.BackupConfig
	Formats       []string
}

// New creates a new Renamer
func New(db types.DatabaseRepository, backupConfig types.BackupConfig, formats []string) *Renamer {
	// Initialize backup manager relative to DB path
	// Assuming DB path is ~/.cache/autotitle/db, we want ~/.cache/autotitle
	dbPath := db.Path()
	cacheRoot := filepath.Dir(dbPath)

	// Import backup inside New to avoid circular deps if any (though types is separate so it's fine)
	// We need to import internal/backup in the file
	bm := backup.New(cacheRoot, backupConfig.DirName)

	// Use defaults if no formats provided
	if len(formats) == 0 {
		formats = config.DefaultFormats
	}

	return &Renamer{
		DB:            db,
		BackupManager: bm,
		BackupConfig:  backupConfig,
		Formats:       formats,
	}
}

// WithEvents sets the event handler
func (r *Renamer) WithEvents(h types.EventHandler) *Renamer {
	r.Events = h
	return r
}

// WithDryRun enables dry-run mode
func (r *Renamer) WithDryRun() *Renamer {
	r.DryRun = true
	return r
}

// WithNoBackup disables backup creation
func (r *Renamer) WithNoBackup() *Renamer {
	r.NoBackup = true
	return r
}

// Execute performs the rename operation for a target
func (r *Renamer) Execute(ctx context.Context, dir string, target *config.Target, media *types.Media) ([]types.RenameOperation, error) {
	// ... (Get list of files logic) ...
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var operations []types.RenameOperation
	renameMappings := make(map[string]string) // oldName -> newName

	// Compile patterns
	var patterns []*matcher.Pattern
	for _, p := range target.Patterns {
		for _, input := range p.Input {
			compiled, err := matcher.Compile(input)
			if err != nil {
				r.emit(types.Event{Type: types.EventWarning, Message: fmt.Sprintf("Invalid pattern: %s", input)})
				continue
			}
			patterns = append(patterns, compiled)
		}
	}

	if len(patterns) == 0 {
		return nil, fmt.Errorf("no valid patterns found")
	}

	// Get output config from first pattern
	outputCfg := target.Patterns[0].Output
	separator := outputCfg.Separator
	if separator == "" {
		separator = " - "
	}

	// Process each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := filepath.Ext(filename)
		if !r.isVideoFile(ext) {
			continue
		}

		// Try to match against patterns
		var matchResult *matcher.MatchResult
		for _, p := range patterns {
			if result, ok := p.MatchTyped(filename); ok {
				matchResult = result
				break
			}
		}

		if matchResult == nil {
			r.emit(types.Event{Type: types.EventWarning, Message: fmt.Sprintf("No pattern matched: %s", filename)})
			continue
		}

		// Get episode data
		ep := media.GetEpisode(matchResult.EpisodeNum)
		if ep == nil {
			r.emit(types.Event{Type: types.EventWarning, Message: fmt.Sprintf("Episode %d not found in database", matchResult.EpisodeNum)})
			continue
		}

		// Build template vars
		vars := matcher.TemplateVars{
			Series:   media.GetTitle("SERIES"),
			SeriesEn: media.GetTitle("SERIES_EN"),
			SeriesJp: media.GetTitle("SERIES_JP"),
			EpNum:    fmt.Sprintf("%d", ep.Number),
			EpName:   ep.Title,
			Res:      matchResult.Resolution,
			Ext:      matchResult.Extension,
		}
		if ep.IsFiller {
			vars.Filler = "[F]"
		}

		// Generate new filename
		newFilename := matcher.GenerateFilenameFromFields(outputCfg.Fields, separator, vars)
		sourcePath := filepath.Join(dir, filename)
		targetPath := filepath.Join(dir, newFilename)

		op := types.RenameOperation{
			SourcePath: sourcePath,
			TargetPath: targetPath,
			Episode:    ep,
			Status:     types.StatusPending,
		}

		if sourcePath == targetPath {
			op.Status = types.StatusSkipped
			r.emit(types.Event{Type: types.EventInfo, Message: fmt.Sprintf("Skipped (unchanged): %s", filename)})
		} else {
			// Add to mappings for backup
			renameMappings[filename] = newFilename

			if r.DryRun {
				r.emit(types.Event{Type: types.EventInfo, Message: fmt.Sprintf("[DRY-RUN] %s → %s", filename, newFilename)})
			} else {
				// We defer actual rename execution until backup is done
			}
		}

		operations = append(operations, op)
	}

	// Perform backup if needed
	// Logic:
	// 1. If DryRun -> No backup
	// 2. If CLI --no-backup (NoBackup=true) -> No backup
	// 3. If Global Backup.Enabled=false -> No backup
	shouldBackup := !r.DryRun && !r.NoBackup && r.BackupConfig.Enabled

	if shouldBackup && len(renameMappings) > 0 {
		r.emit(types.Event{Type: types.EventInfo, Message: "Creating backup..."})
		if err := r.BackupManager.Backup(ctx, dir, renameMappings); err != nil {
			return nil, fmt.Errorf("backup failed: %w", err)
		}
	}

	// Execute renames
	for i, op := range operations {
		if op.Status == types.StatusSkipped {
			continue
		}

		if r.DryRun {
			continue
		}

		// Re-check paths in case something changed (unlikely)
		if err := os.Rename(op.SourcePath, op.TargetPath); err != nil {
			operations[i].Status = types.StatusFailed
			operations[i].Error = err.Error()
			r.emit(types.Event{Type: types.EventError, Message: fmt.Sprintf("Failed: %s: %v", filepath.Base(op.SourcePath), err)})
		} else {
			operations[i].Status = types.StatusSuccess
			r.emit(types.Event{Type: types.EventSuccess, Message: fmt.Sprintf("Renamed: %s → %s", filepath.Base(op.SourcePath), filepath.Base(op.TargetPath))})
		}
	}

	return operations, nil
}

func (r *Renamer) emit(e types.Event) {
	if r.Events != nil {
		r.Events(e)
	}
}

func (r *Renamer) isVideoFile(ext string) bool {
	ext = strings.ToLower(ext)
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:] // Remove leading dot
	}
	for _, f := range r.Formats {
		if ext == f {
			return true
		}
	}
	return false
}
