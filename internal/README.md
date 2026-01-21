# Autotitle - Complete API & Internal Documentation

<details>
<summary><strong>Table of Contents</strong></summary>

### Getting Started

1. [Quick Start](#quick-start)
2. [Installation](#installation)

### Public API

1. [CLI Command Mapping](#cli-command-mapping)
2. [Complete API Reference](#complete-public-api-reference)
   - [Core Functions](#core-functions)
   - [Database Functions](#database-functions)
   - [Pattern Functions](#pattern-functions)
   - [Option Constructors](#option-constructors)
   - [Types](#types)

### Internal Implementation

1. [Internal Packages Overview](#internal-packages-overview)
2. [Package Details](#package-details)
   - [api/](#api)
   - [matcher/](#matcher)
   - [database/](#database)
   - [config/](#config)
   - [renamer/](#renamer)
   - [fetcher/](#fetcher)
   - [logger/](#logger)
   - [util/](#util)

### Advanced Usage

1. [Advanced Internal Imports](#advanced-internal-imports)
2. [Testing](#testing-internal-packages)
3. [Development](#development-guidelines)
4. [See Also](#see-also)

</details>

---

## Quick Start

### Installation

```bash
go get github.com/soymadip/autotitle
```

### Basic Usage

Rename files in a directory:

```go
package main

import (
    "fmt"
    "autotitle"
)

func main() {
    // Preview changes
    err := autotitle.Rename("/path/to/videos",
        autotitle.WithDryRun(),
    )
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Apply rename
    err = autotitle.Rename("/path/to/videos")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
}
```

### Initialize Configuration

```go
// Create config file in directory
err := autotitle.Init("/path/to/videos", autotitle.WithForce())
if err != nil {
    return err
}
```

### Database Operations

```go
// Generate database from MAL
err := autotitle.DBGen(
    "https://myanimelist.net/anime/16498/Shingeki_no_Kyojin",
    func(o *autotitle.DBGenOptions) {
        o.AFLURL = "attack-on-titan"
        o.Force = true
    },
)

// List databases
ids, err := autotitle.DBList("")
for _, id := range ids {
    fmt.Println("Series:", id)
}
```

---

## CLI Command Mapping

This table maps CLI commands to their equivalent package functions:

| CLI Command                          | Package Function(s)                                  | Direct Parameters / Notes                                              |
| ------------------------------------ | ---------------------------------------------------- | ---------------------------------------------------------------------- |
| `autotitle <path>`                   | `Rename(path, opts...)`                              | `WithDryRun`, `WithNoBackup`, `WithVerbose`, `WithQuiet`, `WithConfig` |
| `autotitle init [path]`              | `Init(path, opts...)`                                | `WithForce`, `WithConfig`                                              |
| `autotitle db gen <mal_url\|mal_id>` | `DBGen(malURL, opts...)`                             | Function options object (`DBGenOptions`)                               |
| `autotitle db list`                  | `DBList(outputDir string)`                           | Lists cached DBs with MAL ID and stored title                          |
| `autotitle db info <id\|url\|query>` | `FindSeriesByQuery(query, outputDir)` → `DBInfo(id)` | Accepts MAL ID, MAL URL, or title/slug query                           |
| `autotitle db rm <id\|urlquery>`     | `FindSeriesByQuery(query, outputDir)` → `DBRm(id)`   | Accepts MAL ID, MAL URL, or title/slug query; interactive confirmation |
| `autotitle db rm -a`                 | `DBRm(\"\", outputDir, true)`                        | Remove all databases (requires confirmation)                           |
| `autotitle db path`                  | `DBPath(outputDir string)`                           | Get DB directory path                                                  |
| `autotitle undo <path>`              | `Undo(path)`                                         | Restore from backup                                                    |
| `autotitle clean <path>`             | `Clean(path)`                                        | Remove backup directory                                                |
| Utility                              | `ExtractMALID(url string)`                           | Parse MAL ID from a MAL URL                                            |

---

## Complete Public API Reference

The `autotitle` package provides a stable public API for integrating anime episode renaming into Go applications.

```go
import "autotitle"
```

### Core Functions

#### Rename

Rename anime episodes in the specified directory.

```go
func Rename(path string, opts ...Option) error
```

**Parameters:**

- `path`: Directory containing video files
- `opts`: Functional options (WithDryRun, WithVerbose, etc.)

**Returns:** Error if operation fails

**Example:**

```go
err := autotitle.Rename("/path/to/videos",
    autotitle.WithDryRun(),
    autotitle.WithVerbose(),
)
```

#### Init

Create a new `_autotitle.yml` config file in the specified directory.

```go
func Init(path string, opts ...Option) error
```

**Parameters:**

- `path`: Directory where config will be created
- `opts`: Functional options (WithForce, WithConfig, etc.)

**Returns:** Error if operation fails

**Example:**

```go
err := autotitle.Init("/path/to/videos", autotitle.WithForce())
```

#### Undo

Restore files from the backup directory.

```go
func Undo(path string) error
```

**Parameters:**

- `path`: Directory containing backup

**Returns:** Error if operation fails

**Example:**

```go
err := autotitle.Undo("/path/to/videos")
```

#### Clean

Remove the backup directory.

```go
func Clean(path string) error
```

**Parameters:**

- `path`: Directory containing backup

**Returns:** Error if operation fails

**Example:**

```go
err := autotitle.Clean("/path/to/videos")
```

### Database Functions

#### DBGen

Generate an episode database from MyAnimeList and AnimeFillerList.

The database stores the anime title and all episode data fetched from MyAnimeList. The anime title is then used to replace the `{{SERIES}}` placeholder in rename patterns.

```go
func DBGen(malURL string, opts ...func(*DBGenOptions)) error
```

**Parameters:**

- `malURL`: MyAnimeList anime URL (e.g., "https://myanimelist.net/anime/5114/Title")
- `opts`: Option constructor functions

**Returns:** Error if operation fails

**Data Stored in Database:**

- `anime_title` - Official anime title from MyAnimeList
- `episodes` - Map of episode numbers to episode data (title, filler status, air date)
- `last_update` - When the database was last generated/updated

**Extracted from URL:**

- MAL ID is extracted automatically from the URL using `ExtractMALID()`

**Example:**

```go
err := autotitle.DBGen(
    "https://myanimelist.net/anime/16498/Shingeki_no_Kyojin",
    func(o *autotitle.DBGenOptions) {
        o.AFLURL = "attack-on-titan"
        o.Force = true
    },
)
```

#### DBPath

Get the database directory path.

```go
func DBPath(outputDir string) (string, error)
```

**Parameters:**

- `outputDir`: Custom database directory (empty for default)

**Returns:** Database directory path and error

**Example:**

```go
path, err := autotitle.DBPath("")
```

#### DBList

List all cached database series IDs.

```go
func DBList(outputDir string) ([]string, error)
```

**Parameters:**

- `outputDir`: Custom database directory (empty for default)

**Returns:** Slice of series IDs and error

**Example:**

```go
ids, err := autotitle.DBList("")
```

#### DBInfo

Get information about a specific database.

```go
func DBInfo(seriesID string, outputDir string) (*SeriesData, error)
```

**Parameters:**

- `seriesID`: Series ID to query
- `outputDir`: Custom database directory (empty for default)

**Returns:** SeriesData and error

**Example:**

```go
info, err := autotitle.DBInfo("235", "")
```

#### DBRm

Remove one or more databases.

```go
func DBRm(seriesID string, outputDir string, deleteAll bool) error
```

**Parameters:**

- `seriesID`: Series ID to remove (ignored if deleteAll is true)
- `outputDir`: Custom database directory (empty for default)
- `deleteAll`: If true, remove all databases (seriesID is ignored)

**Returns:** Error if operation fails

**Example:**

```go
// Remove specific database
err := autotitle.DBRm("235", "", false)

// Remove all databases
err := autotitle.DBRm("", "", true)
```

### Utility Functions

#### ExtractMALID

Extract the numeric MAL ID from a MyAnimeList URL.

```go
func ExtractMALID(url string) int
```

**Parameters:**

- `url`: MyAnimeList URL (e.g., "https://myanimelist.net/anime/5114/Title")

**Returns:** Numeric MAL ID, or 0 if extraction fails

**Example:**

```go
id := autotitle.ExtractMALID("https://myanimelist.net/anime/5114/Fullmetal_Alchemist_Brotherhood")
// id = 5114
```

**Use Cases:**

- Extracting ID for database operations
- Parsing user input before calling DBGen
- CLI validation of MAL URLs

### Pattern Functions

#### CompilePattern

Compile a template string into a pattern for matching and generation.

```go
func CompilePattern(template string) (*Pattern, error)
```

**Parameters:**

- `template`: Template string with placeholders

**Returns:** Compiled Pattern and error

**Available Placeholders:**

- `{{SERIES}}` - Anime series name (stored in database from MyAnimeList)
- `{{EP_NUM}}` - Episode number (zero-padded to 3 digits)
- `{{EP_NAME}}` - Episode name/title (from database)
- `{{FILLER}}` - Filler marker (e.g., "[F]" or empty)
- `{{RES}}` - Resolution (e.g., 1080p)
- `{{EXT}}` - File extension
- `{{ANY}}` - Any characters

**Example:**

```go
pattern, err := autotitle.CompilePattern(
    "Episode {{EP_NUM}} {{RES}}.{{EXT}}",
)
```

### Option Constructors

```go
autotitle.WithDryRun()              // Preview mode
autotitle.WithNoBackup()            // Skip backup
autotitle.WithVerbose()             // Verbose output
autotitle.WithQuiet()               // Suppress output
autotitle.WithConfig(path string)   // Custom config
autotitle.WithForce()               // Force operations
```

### Types

```go
type Option func(*Options)

type Options struct {
    DryRun     bool
    NoBackup   bool
    Verbose    bool
    Quiet      bool
    ConfigPath string
    Force      bool
}

type DBGenOptions struct {
    MALURL    string
    AFLURL    string
    OutputDir string
    Force     bool
}

type TemplateVars struct {
    Series string
    EpNum  string
    EpName string
    Filler string
    Res    string
    Ext    string
}

type EpisodeData struct {
    Number  int       // Episode number
    Title   string    // Episode title
    Filler  bool      // Is this a filler episode?
    AirDate time.Time // Air date (optional)
}

type SeriesData struct {
    MALID         string              // MAL ID as string (encoded in filename: {malID}.json)
    Title         string              // Canonical title (Jikan `.data.title`) — used for `{{SERIES}}`
    Slug          string              // Normalized slug of the canonical title (for fast matching)
    Aliases       []string            // Other known titles (english, japanese, synonyms) for matching
    MALURL        string              // Original MAL URL (optional)
    ImageURL      string              // Representative image URL (optional)
    EpisodeCount  int                 // Cached episode count (optional)
    LastUpdate    time.Time           // When database was last updated
    NextCheck     time.Time           // When to check for updates (optional)
    Episodes      map[int]EpisodeData // Episodes keyed by number
    TitleEnglish  string              // Jikan title_english (optional)
    TitleJapanese string              // Jikan title_japanese (optional)
    TitleSynonyms []string            // Jikan title_synonyms (optional)
}

type SearchResult struct {
	MALID        string
	Title        string
	EpisodeCount int
}
```

**Stored in Database File:**

The JSON database file (`~/.cache/autotitle/db/{malID}.json`) now contains rich metadata to support smart matching and display. Top-level fields written by `db gen` include:

- `mal_id` - MAL numeric ID (also encoded in filename)
- `title` - Canonical anime title (used for `{{SERIES}}`)
- `slug` - Normalized slug of the canonical title (used for fast matching)
- `aliases` - Array of alternate titles (english, japanese, synonyms)
- `mal_url` - MyAnimeList URL for the series
- `image_url` - Representative image URL (optional)
- `episode_count` - Cached episode count (optional)
- `last_update` - Timestamp of last update
- `next_check` - Optional next check time
- `episodes` - Array of episode objects (number, title, filler, air_date)

Notes:

- The authoritative identifier is the MAL ID encoded in the filename (`{malID}.json`). The `mal_id` field is written for convenience and clarity.
- The `slug` and `aliases` fields are used by the CLI resolver to match free-form queries (title/slug). Matching prefers slug equality, then exact title/alias equality, then prefix/substring, and finally (optionally) a fuzzy fallback.
- DB files are written atomically (temp file → rename) to avoid corruption.

**Example Database File:**

```json
{
  "anime_title": "Attack on Titan",
  "last_update": "2024-01-15T10:30:00Z",
  "episodes": [
    {
      "number": 1,
      "title": "To Your 2,000 Years Later",
      "filler": false,
      "air_date": "2023-04-03T22:00:00Z"
    }
  ]
}
```

---

## Internal Packages Overview

The `internal/` directory contains the implementation details of autotitle. These packages are **not part of the stable public API** and may change between versions.

> **Note:** Users should prefer the stable public API in the `autotitle` package. Only import from `internal/` for advanced use cases.

### Package Summary

| Package     | Purpose                               |
| ----------- | ------------------------------------- |
| `api/`      | Core business logic, orchestration    |
| `matcher/`  | Pattern matching, filename generation |
| `database/` | Episode data persistence              |
| `config/`   | Configuration file loading            |
| `renamer/`  | File renaming orchestration           |
| `fetcher/`  | External API communication (MAL, AFL) |
| `logger/`   | CLI logging utilities                 |
| `util/`     | Helper utilities                      |

### Stability Notes

Packages in `internal/` should be considered **unstable** and may change:

- Function signatures may be modified
- Types may be reorganized
- APIs may be deprecated or removed
- No semantic versioning guarantees

**If you rely on internal packages:**

1. Pin your dependency to a specific version
2. Test thoroughly with new versions before upgrading
3. Consider opening an issue if you need a stable API for your use case

---

## Package Details

### api/

Core business logic and orchestration for all autotitle operations.

**Key exports:**

- `Rename()` - Rename files in directory
- `Undo()` - Restore from backup
- `Clean()` - Remove backup
- `Init()` - Create config file
- `DBGen()`, `DBPath()`, `DBList()`, `DBInfo()`, `DBRm()` - Database operations
- `CompilePattern()` - Compile pattern templates
- `Option`, `Options`, `DBGenOptions` - Configuration types
- `TemplateVars`, `EpisodeData`, `SeriesData` - Data types

All core logic lives here with **zero dependencies on logging or UI concerns**. This makes the API pure business logic that can be tested and reused independently.

### matcher/

Pattern matching and filename generation.

**Key exports:**

- `GuessPattern(filename string) string` - Detect naming pattern from filename
- `Compile(template string) (*Pattern, error)` - Compile template to regex
- `GenerateFilename(template string, vars TemplateVars) string` - Generate filename from template
- `TemplateVars` struct - Variables for substitution
- `Pattern` type - Compiled pattern for matching

**Template placeholders:**

- `{{SERIES}}` - Anime series name
- `{{EP_NUM}}` - Episode number (zero-padded to 3 digits)
- `{{EP_NAME}}` - Episode title
- `{{FILLER}}` - Filler marker (e.g., `[F]`)
- `{{RES}}` - Resolution (e.g., `1080p`)
- `{{EXT}}` - File extension
- `{{ANY}}` - Match any characters

### database/

Database persistence and retrieval for episode data.

**Key exports:**

- `New(outputDir string) (*DB, error)` - Create database instance
- `Load(seriesID string) (*SeriesData, error)` - Load series data from file
- `List() ([]string, error)` - List all cached series IDs
- `Save(data *SeriesData) error` - Save series data to file
- `Delete(seriesID string) error` - Delete a database file
- `DeleteAll() error` - Delete all database files
- `Exists(seriesID string) bool` - Check if database exists
- `Find(query string) ([]SearchResult, error)` - Fuzzy search for series

**Storage:**

Databases are stored as JSON files in the cache directory (typically `~/.cache/autotitle/db/`):

- Filename format: `{malID}.json` (e.g., `5114.json` for MAL ID 5114)
- JSON contains: anime title, episode data, and metadata
- Series ID encoded in filename, not stored in JSON (DRY principle)

### config/

Configuration file loading and parsing.

**Key exports:**

- `LoadGlobal(customPath string) (*GlobalConfig, error)` - Load global config
- `LoadMap(dir, filename string) (*MapConfig, error)` - Load map file
- `GlobalConfig` struct - Global settings
- `MapConfig` struct - Per-directory config
- `Target` struct - Target configuration with patterns

**Config locations:**

- Global: `~/.config/autotitle/config.yml` or `/etc/autotitle/config.yml`
- Map file: `./_autotitle.yml` (per directory)

### renamer/

File renaming orchestration and backup management.

**Key exports:**

- `Renamer` struct - Orchestrates rename operations
- `Execute(targetPath string) error` - Perform rename
- `Undo(targetPath string) error` - Restore from backup
- `Clean(targetPath string) error` - Remove backup directory

Handles:

- Pattern matching against filenames
- Database lookups for episode data
- Backup creation
- File renaming with error handling

### fetcher/

External API communication (MyAnimeList, AnimeFillerList).

**Key exports:**

- `New(rateLimit, timeout int) *Fetcher` - Create fetcher
- `FetchMAL(malID int) (*SeriesInfo, error)` - Fetch from MAL
- `FetchAFL(slug string) (*FillerInfo, error)` - Fetch from AnimeFillerList
- `ExtractMALID(url string) int` - Parse MAL ID from URL

Handles:

- Rate limiting
- Timeout management
- API error handling
- Data parsing and validation

### logger/

Logging utilities for CLI output.

**Key exports:**

- `Info(format string, args ...interface{})` - Information messages
- `Warn(format string, args ...interface{})` - Warning messages
- `Error(format string, args ...interface{})` - Error messages
- `Success(format string, args ...interface{})` - Success messages
- `Fatal(format string, args ...interface{})` - Fatal error (exits)

**Note:** Core API (`internal/api`) does not use logging. Logging is reserved for CLI and user-facing code.

### util/

Utility functions and helpers.

**Key exports:**

- `ParseRanges(rangeStr string) ([]int, error)` - Parse episode ranges (e.g., "1-10,15,20-30")
- Other helper functions

---

## Advanced Internal Imports

For advanced use cases, you can import internal packages directly.

### Pattern Matching and Generation

The `matcher` package provides pattern detection and filename generation utilities:

```go
import "github.com/soymadip/autotitle/internal/matcher"

// Detect pattern from filename
pattern := matcher.GuessPattern("Episode 01 1080p.mkv")
// Returns: "Episode {{EP_NUM}} {{RES}}.{{EXT}}"

// Compile pattern to regex
compiled, err := matcher.Compile(pattern)
if err != nil {
    return err
}

// Generate new filename
newName := matcher.GenerateFilename(
    "{{SERIES}} {{EP_NUM}} - {{EP_NAME}}.{{EXT}}",
    matcher.TemplateVars{
        Series: "Attack on Titan",
        EpNum:  "1",
        EpName: "The Fall of Shiganshina",
        Res:    "1080p",
        Ext:    "mkv",
    },
)
```

**Caution:** These are internal APIs and may change. Use the stable `autotitle.CompilePattern()` when possible.

### Configuration

The `config` package provides configuration file loading:

```go
import "github.com/soymadip/autotitle/internal/config"

// Load global configuration
globalCfg, err := config.LoadGlobal("")
if err != nil {
    return err
}

// Load map file from directory
mapCfg, err := config.LoadMap("/path/to/anime", "_autotitle.yml")
if err != nil {
    return err
}

// Resolve target configuration
target, err := mapCfg.ResolveTarget(".")
if err != nil {
    return err
}
```

### Database

The `database` package provides direct database access:

```go
import "github.com/soymadip/autotitle/internal/database"

// Create database instance
db, err := database.New("")  // empty string = default directory
if err != nil {
    return err
}

// Load series data
seriesData, err := db.Load("16498")
if err != nil {
    return err
}

// List databases
ids, err := db.List()
if err != nil {
    return err
}

// Delete database
err := db.Delete("16498")
```

**Note:** Prefer the stable `autotitle.DBList()`, `autotitle.DBInfo()`, `autotitle.DBPath()`, and `autotitle.DBRm()` functions when possible.

---

## Testing Internal Packages

Internal packages are designed to be testable. Consider creating tests for your integration:

```go
import (
    "github.com/soymadip/autotitle/internal/matcher"
    "github.com/soymadip/autotitle/internal/config"
)

func TestMyIntegration(t *testing.T) {
    pattern := matcher.GuessPattern("Episode 01.mkv")
    if pattern == "" {
        t.Fatal("Expected pattern detection")
    }
}
```

---

## Development Guidelines

When modifying internal packages:

1. **Maintain separation of concerns** - Each package should have a clear responsibility
2. **Keep API layers pure** - Don't add logging or UI logic to business logic packages
3. **Use meaningful names** - Function and type names should be self-documenting
4. **Add error context** - Wrap errors with additional context using `fmt.Errorf`
5. **Write tests** - Test packages independently
6. **Document public exports** - Add godoc comments to exported functions and types

---

## See Also

- `../README.md` - User guide and CLI documentation
- `../autotitle.go` - Public package wrapper
- `../cmd/autotitle/main.go` - CLI implementation
