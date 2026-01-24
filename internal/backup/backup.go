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

// Backup creates a backup of files before renaming
// mappings is a map of oldName -> newName
func (m *Manager) Backup(ctx context.Context, dir string, mappings map[string]string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve source dir: %w", err)
	}

	// Clean any previous backup for this directory first
	_ = m.Clean(ctx, dir) // Ignore error - might not exist

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

	// For each mapping:
	// 1. Delete the renamed file (newName)
	// 2. Copy original file from backup back to source dir
	for oldName, newName := range mappings {
		// Remove renamed file
		renamedPath := filepath.Join(absDir, newName)
		if _, err := os.Stat(renamedPath); err == nil {
			os.Remove(renamedPath)
		}

		// Restore original
		src := filepath.Join(backupPath, oldName)
		dst := filepath.Join(absDir, oldName)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to restore file %s: %w", oldName, err)
		}
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
		// Remove each backup directory
		os.RemoveAll(r.Path) // Ignore individual errors
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
		return []types.BackupRecord{}, nil // Auto-heal if corrupted
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

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
