// Package backup handles file backup and restore operations.
package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mydehq/autotitle/internal/types"
)

const (
	RegistryFileName = "backup_registry.json"
	MappingsFileName = "mappings.json"
	DefaultDirName   = ".autotitle_backup"
)

// Manager handles backup operations
type Manager struct {
	registryPath string // ~/.cache/autotitle/backup_registry.json
	dirName      string // Backup dir name (from config)
	Events       types.EventHandler
}

// New creates a new BackupManager
func New(cacheRoot string, dirName string) *Manager {
	if dirName == "" {
		dirName = DefaultDirName
	}
	return &Manager{
		registryPath: filepath.Join(cacheRoot, RegistryFileName),
		dirName:      dirName,
	}
}

// WithEvents sets the event handler
func (m *Manager) WithEvents(h types.EventHandler) types.BackupManager {
	m.Events = h
	return m
}

func (m *Manager) emit(t types.EventType, msg string) {
	if m.Events != nil {
		m.Events(types.Event{Type: t, Message: msg})
	}
}

// Backup creates a backup of files before renaming
// mappings is a map of oldName -> newName
func (m *Manager) Backup(ctx context.Context, dir string, mappings map[string]string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve source dir: %w", err)
	}

	// Clean any previous backup for this directory first
	_ = m.Clean(ctx, dir)

	// Create backup directory inside the input directory
	backupPath := filepath.Join(absDir, m.dirName)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Copy original files to backup
	for oldName := range mappings {
		src := filepath.Join(absDir, oldName)
		dst := filepath.Join(backupPath, oldName)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to backup file %s: %w", oldName, err)
		}
		m.emit(types.EventInfo, fmt.Sprintf("Backed up: %s", oldName))
	}

	// Write mappings.json
	mappingsPath := filepath.Join(backupPath, MappingsFileName)
	mappingsData, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mappings: %w", err)
	}
	if err := os.WriteFile(mappingsPath, mappingsData, 0644); err != nil {
		return fmt.Errorf("failed to write mappings file: %w", err)
	}

	// Add to global registry
	record := types.BackupRecord{
		Path:      backupPath,
		SourceDir: absDir,
		Timestamp: time.Now(),
	}
	return m.addRegistry(record)
}

// Restore restores files from backup (undo rename)
func (m *Manager) Restore(ctx context.Context, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve dir: %w", err)
	}

	backupPath := filepath.Join(absDir, m.dirName)

	// Read mappings
	mappingsPath := filepath.Join(backupPath, MappingsFileName)
	data, err := os.ReadFile(mappingsPath)
	if err != nil {
		return fmt.Errorf("no backup found for directory: %w", err)
	}

	var mappings map[string]string
	if err := json.Unmarshal(data, &mappings); err != nil {
		return fmt.Errorf("failed to parse mappings: %w", err)
	}

	for oldName, newName := range mappings {
		src := filepath.Join(backupPath, oldName)
		dst := filepath.Join(absDir, oldName)
		renamedPath := filepath.Join(absDir, newName)

		// Restore original first
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to restore file %s: %w", oldName, err)
		}

		// Only remove renamed file IF it's different from the original
		if oldName != newName {
			if _, err := os.Stat(renamedPath); err == nil {
				_ = os.Remove(renamedPath)
			}
		}
		m.emit(types.EventSuccess, fmt.Sprintf("Restored: %s → %s", newName, oldName))
	}

	// Clean up backup after successful restore
	return m.Clean(ctx, dir)
}

// Clean removes backup for a specific directory
func (m *Manager) Clean(ctx context.Context, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve dir: %w", err)
	}

	backupPath := filepath.Join(absDir, m.dirName)

	// Remove backup directory
	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("failed to remove backup dir: %w", err)
	}

	// Remove from registry
	return m.removeFromRegistry(absDir)
}

// CleanAll removes all backups globally using registry
func (m *Manager) CleanAll(ctx context.Context) error {
	records, err := m.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, r := range records {
		_ = os.RemoveAll(r.Path) // Ignore individual errors
	}

	// Clear registry
	return m.saveRegistry([]types.BackupRecord{})
}

// ListAll returns all backup records from global registry
func (m *Manager) ListAll(ctx context.Context) ([]types.BackupRecord, error) {
	data, err := os.ReadFile(m.registryPath)
	if os.IsNotExist(err) {
		return []types.BackupRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}

	var records []types.BackupRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return []types.BackupRecord{}, nil
	}
	return records, nil
}

func (m *Manager) addRegistry(r types.BackupRecord) error {
	records, _ := m.ListAll(context.Background())
	records = append(records, r)
	return m.saveRegistry(records)
}

func (m *Manager) removeFromRegistry(sourceDir string) error {
	records, _ := m.ListAll(context.Background())
	var kept []types.BackupRecord
	for _, r := range records {
		if r.SourceDir != sourceDir {
			kept = append(kept, r)
		}
	}
	return m.saveRegistry(kept)
}

func (m *Manager) saveRegistry(records []types.BackupRecord) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(m.registryPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.registryPath, data, 0644)
}

func copyFile(src, dst string) error {

	// Try hard link first
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	// Fallback to bitwise copy
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
