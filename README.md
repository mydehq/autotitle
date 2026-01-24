# Autotitle

A CLI tool & Go library for automatically renaming anime episodes with proper titles and detecting filler episodes.

## Features

- 🎯 **Automatic Episode Renaming** - Pattern-based filename matching and generation
- 🎨 **Flexible Pattern Matching** - Support for multiple filename formats with `{{TEMPLATE}}` variables
- 🔖 **Filler Detection** - Automatically marks filler episodes with `[F]` tag
- 📚 **Episode Database** - Caches episode data from MyAnimeList and AnimeFillerList
- 🧠 **Smart Updates** - Auto-updates database when new episodes air (checks release dates)
- 💾 **Smart Backups** - Automatic backup before renaming with restore capability
- 📦 **Library & CLI** - Use as standalone tool or import as Go package
- 🏗️ **Clean Architecture** - Pure business logic API with zero UI dependencies

## Installation

### As CLI Tool

#### From [`my-repo`](https://mydehq.github.io/my-repo/) Repo:

```bash
  curl -sL https://mydehq.github.io/my-repo/install | bash
  sudo pacman -S autotitle
```

#### Or From AUR

```bash
paru -S autotitle
```

#### Or Build Manually:

```bash
  git clone https://github.com/mydehq/autotitle.git && cd autotitle
  make install
```

### As Library

```bash
go get github.com/mydehq/autotitle
```

## Quick Start

### CLI Usage

```bash
# Navigate to your anime directory
cd /path/to/videos

# Initialize configuration
autotitle init

# open _autotitle.yml and make changes.

# Preview changes (dry-run)
autotitle --dry-run .

# Perform rename
autotitle .

# Restore from backup if needed
autotitle undo .

# Clean backup directory
autotitle clean .
```

### Library Usage

```go
package main

import (
	"fmt"
	"autotitle"
)

func main() {

  // Initialize config
  err := autotitle.Init("/path/to/videos")
  if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
  }

  // open _autotitle.yml and make changes.

	// Rename files in directory
	err := autotitle.Rename("/path/to/videos")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

}
```

## Configuration

When you run `autotitle init`, it creates `_autotitle.yml` with auto-detected patterns:

```yaml
# Autotitle Map File
targets:
  - path: "."
    url: "https://myanimelist.net/anime/XXXXX/Series_Name"
    filler_url: "https://www.animefillerlist.com/shows/series-name"
    patterns:
      - input:
          - "Episode {{EP_NUM}} {{RES}}"
        output:
          fields: [SERIES, EP_NUM, FILLER, EP_NAME]
```

### Output Format

The field-based output format supports:

- **Field names** (uppercase): SERIES, EP_NUM, FILLER, EP_NAME, RES
- **Literal strings** (any text): "DC", "[v2]", "S01"
- **Optional separator**: Defaults to ` - ` (space-dash-space) if not specified
- **Auto-skip empty fields**: Empty fields are automatically excluded from output

**Example:**

```yaml
output:
  fields: [SERIES, EP_NUM, FILLER, EP_NAME] # Standard format
  separator: " - " # Optional, defaults to " - "
```

**With literal strings:**

```yaml
output:
  fields: ["DC", EP_NUM, FILLER, EP_NAME] # Adds "DC" prefix
  separator: " - "
```

**Different separator:**

```yaml
output:
  fields: [SERIES, EP_NUM, EP_NAME]
  separator: "_" # Underscore separator
```

## url & filler_url

- To get `url`, visit [MyAnimeList](https://myanimelist.net/) and find the series, copy the URL.
- To get `filler_url`, visit [AnimeFillerList](https://www.animefillerlist.com/) and find the series, copy the URL. If the series has no fillers, use `null` or omit this field.

### Available Fields

These fields can be used in the output configuration.

| Field Name  | Description                       | Example                   | Notes                 |
| ----------- | --------------------------------- | ------------------------- | --------------------- |
| `SERIES`    | Anime series name (from database) | `Attack on Titan`         | Auto-populated        |
| `SERIES_EN` | English series name               | `Attack on Titan`         | Auto-populated        |
| `SERIES_JP` | Japanese series name              | `進撃の巨人`              | Auto-populated        |
| `EP_NUM`    | Episode number (padded to 3)      | `001`, `123`              | Auto-populated        |
| `EP_NAME`   | Episode title (from database)     | `The Fall of Shiganshina` | Auto-populated        |
| `FILLER`    | Filler marker                     | `[F]` or empty            | Auto-skipped if empty |

**Input Pattern Matching:** Use `{{FIELD_NAME}}` placeholders in input patterns to match filenames. `{{ANY}}` matches arbitrary text.

## CLI Commands

### Main Commands

```bash
autotitle <path>              # Rename files in directory
autotitle init [path]         # Create _autotitle.yml map file
autotitle undo <path>         # Restore from backup
autotitle clean <path>        # Remove backup directory
```

### Database Commands

```bash
autotitle db gen <mal_url> --filler <afl_url>  # Generate database with filler data
autotitle db gen <mal_url> --force             # Force regenerate (ignore cache)
autotitle db path                              # Show database directory
autotitle db list                              # List all cached databases
autotitle db info <id|url|query>               # Show database info
autotitle db rm <id|url|query>                 # Remove specific database
autotitle db rm -a                             # Remove all databases
```

### Flags

| Flag          | Short | Description                      |
| ------------- | ----- | -------------------------------- |
| `--dry-run`   | `-d`  | Preview changes without applying |
| `--no-backup` | `-n`  | Skip backup creation             |
| `--verbose`   | `-v`  | Show detailed output             |
| `--quiet`     | `-q`  | Suppress output except errors    |
| `--config`    | `-c`  | Custom map file path             |
| `--force`     | `-f`  | Force overwrite/update (config, database) |

## Global Configuration

The global config file is located at:

- Linux/macOS: `~/.config/autotitle/config.yml` or `config.yaml`

```yaml
# Global Config Example
map_file: _autotitle.yml        # Custom map file name
formats: [mkv, mp4, avi, webm]  # Video extensions to scan
api:
  rate_limit: 2                 # API requests per second
  timeout: 30                   # HTTP timeout in seconds
backup:
  enabled: true                 # Enable/disable backups
  dir_name: .autotitle_backup   # Backup directory name
```

The local episode database files are stored in the cache directory:

- Default: `~/.cache/autotitle/db/`

## Project Architecture

```
autotitle/
├── cmd/autotitle/main.go    ← CLI orchestration
├── autotitle.go             ← Public package wrapper
└── internal/
    ├── api/                 ← Core business logic
    ├── database/            ← Data persistence
    ├── config/              ← Configuration loading
    ├── matcher/             ← Pattern matching
    ├── renamer/             ← File operations
    ├── provider/            ← External APIs (MAL, etc.)
    ├── provider/filler/     ← Filler sources (AFL, etc.)
    ├── backup/              ← Backup/restore operations
    └── types/               ← Shared types and interfaces
```

**Key Principle:** All business logic lives in `internal/api` with zero dependencies on logging or UI concerns. This makes the code testable and reusable.

## Documentation

- **[internal/README.md](internal/README.md)** - Complete API reference and internal package documentation
  - Quick Start guide
  - Full API documentation
  - CLI to package function mapping
  - Internal package details
  - Advanced usage examples
  - Notes: `db gen` writes extended DB files (title, slug, aliases). Public API exposes `ExtractMALID()` and `FindSeriesByQuery()` to help resolve MAL URLs/IDs and title/slug queries.

## Data Sources

- **MyAnimeList (MAL)** - Episode titles and metadata
- **AnimeFillerList** - Filler/mixed episode detection
- **Local Cache** - Episodes cached for offline use

## License

MIT
