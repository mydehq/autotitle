// Package renamer handles file renaming operations.
package renamer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mydehq/autotitle/internal/backup"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/tagger"
	"github.com/mydehq/autotitle/internal/types"
)

// Renamer handles file renaming operations
type Renamer struct {
	DB            types.DatabaseRepository
	BackupManager types.BackupManager
	Events        types.EventHandler
	DryRun        bool
	NoBackup      bool
	Tag           bool
	BackupConfig  types.BackupConfig
	Formats       []string
	Offset        *int
}

// New creates a new Renamer
func New(db types.DatabaseRepository, backupConfig types.BackupConfig, formats []string) *Renamer {
	dbPath := db.Path()
	cacheRoot := filepath.Dir(dbPath)

	bm := backup.New(cacheRoot, backupConfig.DirName)

	if len(formats) == 0 {
		formats = config.GetDefaults().Formats
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

// WithTagging enables post-rename MKV metadata embedding via mkvpropedit.
func (r *Renamer) WithTagging(enabled bool) *Renamer {
	r.Tag = enabled
	return r
}

// WithOffset sets the episode number offset
func (r *Renamer) WithOffset(offset int) *Renamer {
	r.Offset = &offset
	return r
}

// Execute performs the rename operation for a target
func (r *Renamer) Execute(ctx context.Context, dir string, target *types.Target, media *types.Media) ([]types.RenameOperation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	patterns, err := r.compilePatterns(target)
	if err != nil {
		r.emit(types.Event{Type: types.EventWarning, Message: err.Error()})
		if len(patterns) == 0 {
			return nil, fmt.Errorf("no valid patterns found")
		}
	}

	smartPadding := r.calculatePadding(media)

	var operations []types.RenameOperation
	renameMappings := make(map[string]string)

	usedTargets := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := filepath.Ext(filename)
		if !r.isVideoFile(ext) {
			continue
		}

		var matchResult *matcher.MatchResult
		var matchPattern *types.Pattern

		patIdx := 0
		found := false
		for i := range target.Patterns {
			for range target.Patterns[i].Input {
				if patIdx < len(patterns) {
					p := patterns[patIdx]
					if result, ok := p.MatchTyped(filename); ok {
						matchResult = result
						matchPattern = &target.Patterns[i]
						found = true
						break
					}
				}
				patIdx++
			}
			if found {
				break
			}
		}

		if matchResult == nil {
			r.emit(types.Event{Type: types.EventWarning, Message: fmt.Sprintf("No pattern matched: %s", filename)})
			continue
		}

		outputCfg := matchPattern.Output

		padding := outputCfg.Padding
		if padding == 0 {
			padding = smartPadding
		}

		// Calculate Offset
		offset := MatchResultOffset(r.Offset, matchPattern)

		// Get Episode
		episodeNum := matchResult.EpisodeNum + offset
		ep := media.GetEpisode(episodeNum)
		if ep == nil {
			msg := fmt.Sprintf("Episode %d not found in database", matchResult.EpisodeNum)
			if offset != 0 {
				msg = fmt.Sprintf("Episode %d (mapped to %d) not found in database", matchResult.EpisodeNum, episodeNum)
			}
			r.emit(types.Event{Type: types.EventWarning, Message: msg})
			continue
		}

		// Build Variables
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

		// Generate Filename
		separator := outputCfg.Separator

		newFilename, err := matcher.GenerateFilenameFromFields(outputCfg.Fields, separator, vars, padding)
		if err != nil {
			r.emit(types.Event{Type: types.EventError, Message: fmt.Sprintf("Failed to generate filename: %v", err)})
			continue
		}

		sourcePath := filepath.Join(dir, filename)
		targetPath := filepath.Join(dir, newFilename)

		// Check for target collision
		if usedTargets[targetPath] {
			r.emit(types.Event{Type: types.EventError, Message: fmt.Sprintf("Collision detected: %s and another file both want to rename to %s", filename, newFilename)})
			continue
		}
		usedTargets[targetPath] = true

		op := types.RenameOperation{
			SourcePath: sourcePath,
			TargetPath: targetPath,
			Episode:    ep,
			Series:     media.Title,
			Status:     types.StatusPending,
		}

		if sourcePath == targetPath {
			op.Status = types.StatusSkipped
			r.emit(types.Event{Type: types.EventInfo, Message: fmt.Sprintf("Skipped (unchanged): %s", filename)})
		} else {
			renameMappings[filename] = newFilename
			if r.DryRun {
				r.emit(types.Event{Type: types.EventInfo, Message: fmt.Sprintf("[DRY-RUN] %s → %s", filename, newFilename)})
			}
		}

		operations = append(operations, op)
	}

	// Perform Backup
	if err := r.performBackup(ctx, dir, renameMappings); err != nil {
		return nil, err
	}

	// Perform Rename
	r.performRenames(operations)

	return operations, nil
}

func (r *Renamer) compilePatterns(target *types.Target) ([]*matcher.Pattern, error) {
	var patterns []*matcher.Pattern
	var errs []string

	for _, p := range target.Patterns {
		for _, input := range p.Input {
			compiled, err := matcher.Compile(input)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Invalid pattern '%s': %v", input, err))
				continue
			}
			patterns = append(patterns, compiled)
		}
	}

	if len(errs) > 0 {
		return patterns, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return patterns, nil
}

func (r *Renamer) calculatePadding(media *types.Media) int {
	smartPadding := 2
	maxEp := media.EpisodeCount

	if len(media.Episodes) > 0 {
		if max := slices.MaxFunc(media.Episodes, func(a, b types.Episode) int {
			return a.Number - b.Number
		}); max.Number > maxEp {
			maxEp = max.Number
		}
	}

	digits := len(fmt.Sprintf("%d", maxEp))
	if digits > smartPadding {
		smartPadding = digits
	}
	return smartPadding
}

func MatchResultOffset(globalOffset *int, pattern *types.Pattern) int {
	if globalOffset != nil {
		return *globalOffset
	}
	if pattern != nil {
		return pattern.Output.Offset
	}
	return 0
}

func (r *Renamer) performBackup(ctx context.Context, dir string, mappings map[string]string) error {
	shouldBackup := !r.DryRun && !r.NoBackup && r.BackupConfig.Enabled
	if shouldBackup && len(mappings) > 0 {
		r.emit(types.Event{Type: types.EventInfo, Message: "Creating backup..."})
		if err := r.BackupManager.Backup(ctx, dir, mappings); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	return nil
}

func (r *Renamer) performRenames(ops []types.RenameOperation) {
	for i, op := range ops {
		if op.Status == types.StatusSkipped {
			continue
		}
		if r.DryRun {
			continue
		}

		if err := os.Rename(op.SourcePath, op.TargetPath); err != nil {
			ops[i].Status = types.StatusFailed
			ops[i].Error = err.Error()
			r.emit(types.Event{Type: types.EventError, Message: fmt.Sprintf("Failed: %s: %v", filepath.Base(op.SourcePath), err)})
		} else {
			ops[i].Status = types.StatusSuccess
			r.emit(types.Event{Type: types.EventSuccess, Message: fmt.Sprintf("Renamed: %s → %s", filepath.Base(op.SourcePath), filepath.Base(op.TargetPath))})

			if r.Tag && op.Episode != nil {
				r.tagFile(op.TargetPath, op.Episode, ops[i].Series)
			}
		}
	}
}

func (r *Renamer) tagFile(path string, ep *types.Episode, show string) {
	info := tagger.TagInfo{
		Title:       ep.Title,
		Show:        show,
		EpisodeID:   fmt.Sprintf("%d", ep.Number),
		EpisodeSort: ep.Number,
		AirDate:     ep.AirDate,
	}
	if err := tagger.TagFile(context.Background(), path, info); err != nil {
		r.emit(types.Event{Type: types.EventWarning, Message: fmt.Sprintf("Tagging failed for %s: %v", filepath.Base(path), err)})
	} else {
		r.emit(types.Event{Type: types.EventInfo, Message: fmt.Sprintf("Tagged: %s", filepath.Base(path))})
	}
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

	return slices.Contains(r.Formats, ext)
}
