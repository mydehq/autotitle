package database_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mydehq/autotitle/internal/database"
)

func TestNewAndLoadSave(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "autotitle-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := database.New(tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create test series data
	testData := &database.SeriesData{
		MALID:        "12345",
		Title:        "Test Anime",
		Slug:         "test-anime",
		Aliases:      []string{"Test Series", "テストアニメ"},
		EpisodeCount: 12,
		Episodes: map[int]database.EpisodeData{
			1: {Number: 1, Title: "Episode 1", Filler: false},
			2: {Number: 2, Title: "Episode 2", Filler: true},
		},
	}

	// Test Save
	if err := db.Save(testData); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "12345.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Save() did not create file at %s", expectedPath)
	}

	// Test Load
	loaded, err := db.Load("12345")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify loaded data
	if loaded.MALID != testData.MALID {
		t.Errorf("Load() MALID = %q; want %q", loaded.MALID, testData.MALID)
	}
	if loaded.Title != testData.Title {
		t.Errorf("Load() Title = %q; want %q", loaded.Title, testData.Title)
	}
	if len(loaded.Episodes) != len(testData.Episodes) {
		t.Errorf("Load() Episodes count = %d; want %d", len(loaded.Episodes), len(testData.Episodes))
	}
}

func TestFind(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "autotitle-db-find-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := database.New(tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create test data
	series1 := &database.SeriesData{
		MALID:        "235",
		Title:        "Meitantei Conan",
		Slug:         "meitantei-conan",
		Aliases:      []string{"Detective Conan", "名探偵コナン"},
		EpisodeCount: 1000,
		LastUpdate:   time.Now(),
	}

	series2 := &database.SeriesData{
		MALID:        "779",
		Title:        "Meitantei Conan Movie 01: Tokei Jikake no Matenrou",
		Slug:         "meitantei-conan-movie-01-tokei-jikake-no-matenrou",
		Aliases:      []string{"Detective Conan Movie 01"},
		EpisodeCount: 1,
		LastUpdate:   time.Now(),
	}

	if err := db.Save(series1); err != nil {
		t.Fatalf("Save(series1) error = %v", err)
	}
	if err := db.Save(series2); err != nil {
		t.Fatalf("Save(series2) error = %v", err)
	}

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantFirst string // MALID of first result
	}{
		{
			name:      "Exact MAL ID",
			query:     "235",
			wantCount: 1,
			wantFirst: "235",
		},
		{
			name:      "Exact slug",
			query:     "meitantei-conan",
			wantCount: 1,
			wantFirst: "235",
		},
		{
			name:      "Fuzzy title match",
			query:     "conan",
			wantCount: 2, // Both series contain "conan"
		},
		{
			name:      "Case insensitive",
			query:     "CONAN",
			wantCount: 2,
		},
		{
			name:      "Partial title",
			query:     "meitantei",
			wantCount: 2,
		},
		{
			name:      "Alias match",
			query:     "detective",
			wantCount: 2,
		},
		{
			name:      "No match",
			query:     "nonexistent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.Find(tt.query)
			if err != nil {
				t.Fatalf("Find() error = %v", err)
			}

			if len(results) != tt.wantCount {
				t.Errorf("Find(%q) returned %d results; want %d", tt.query, len(results), tt.wantCount)
			}

			if tt.wantFirst != "" && len(results) > 0 {
				if results[0].MALID != tt.wantFirst {
					t.Errorf("Find(%q) first result MALID = %q; want %q", tt.query, results[0].MALID, tt.wantFirst)
				}
			}
		})
	}
}

func TestExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autotitle-db-exists-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := database.New(tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test non-existent
	if db.Exists("99999") {
		t.Error("Exists() returned true for non-existent series")
	}

	// Save a series
	testData := &database.SeriesData{
		MALID:      "235",
		Title:      "Test",
		LastUpdate: time.Now(),
	}
	if err := db.Save(testData); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Test existent
	if !db.Exists("235") {
		t.Error("Exists() returned false for existing series")
	}
}

func TestList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autotitle-db-list-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := database.New(tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Empty list
	ids, err := db.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("List() on empty db returned %d items; want 0", len(ids))
	}

	// Add test data
	for _, id := range []string{"235", "779", "1001"} {
		testData := &database.SeriesData{
			MALID:      id,
			Title:      "Test " + id,
			LastUpdate: time.Now(),
		}
		if err := db.Save(testData); err != nil {
			t.Fatalf("Save(%s) error = %v", id, err)
		}
	}

	// List should return 3 items
	ids, err = db.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("List() returned %d items; want 3", len(ids))
	}
}
