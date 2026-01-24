package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mydehq/autotitle/internal/database"
	"github.com/mydehq/autotitle/internal/types"
)

func TestRepository_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := database.NewRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	ctx := context.Background()
	media := &types.Media{
		ID:           "12345",
		Provider:     "mal",
		Title:        "Test Anime",
		Slug:         "test-anime",
		EpisodeCount: 12,
		Episodes: []types.Episode{
			{Number: 1, Title: "Ep 1"},
		},
		LastUpdate: time.Now(),
	}

	// Test Save
	if err := repo.Save(ctx, media); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file existence: {tmp}/{provider}/{id}@{slug}.json
	expectedPath := filepath.Join(tmpDir, "mal", "12345@test-anime.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file at %s, but not found", expectedPath)
	}

	// Test Load
	loaded, err := repo.Load(ctx, "mal", "12345")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.Title != media.Title {
		t.Errorf("Expected title %q, got %q", media.Title, loaded.Title)
	}
	if len(loaded.Episodes) != 1 {
		t.Errorf("Expected 1 episode, got %d", len(loaded.Episodes))
	}
}

func TestRepository_Search(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := database.NewRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	ctx := context.Background()
	media1 := &types.Media{ID: "1", Provider: "mal", Title: "Naruto", Slug: "naruto"}
	media2 := &types.Media{ID: "2", Provider: "mal", Title: "Naruto Shippuden", Slug: "naruto-shippuden"}
	media3 := &types.Media{ID: "3", Provider: "tmdb", Title: "Bleach", Slug: "bleach"}

	repo.Save(ctx, media1)
	repo.Save(ctx, media2)
	repo.Save(ctx, media3)

	tests := []struct {
		query     string
		wantCount int
	}{
		{"naruto", 2},
		{"shippuden", 1},
		{"bleach", 1},
		{"one piece", 0},
		{"", 3}, // Empty query should return all
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := repo.Search(ctx, tt.query)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("Search(%q) returned %d results, want %d", tt.query, len(results), tt.wantCount)
			}
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := database.NewRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	ctx := context.Background()
	media := &types.Media{ID: "1", Provider: "mal", Title: "Test"}
	repo.Save(ctx, media)

	if !repo.Exists("mal", "1") {
		t.Fatal("Exists returned false before delete")
	}

	if err := repo.Delete(ctx, "mal", "1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if repo.Exists("mal", "1") {
		t.Error("Exists returned true after delete")
	}
}
