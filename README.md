# Autotitle

A CLI tool & Go library for automatically renaming anime episodes with proper titles and detecting filler episodes.

## Features

- 🎯 **Automatic Episode Renaming** - Pattern-based filename matching and generation
- 🎨 **Flexible Pattern Matching** - Support for multiple filename formats with `{{TEMPLATE}}` variables
- 🔖 **Filler Detection** - Automatically marks filler episodes with `[F]` tag
- 📚 **Episode Database** - Caches episode data from MyAnimeList and AnimeFillerList
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
    mal_url: "https://myanimelist.net/anime/XXXXX/Series_Name"
    afl_url: "https://www.animefillerlist.com/shows/series-name"
    patterns:
      - input:
          - "Episode {{EP_NUM}} {{RES}}.{{EXT}}"
        output:
          fields: [SERIES, EP_NUM, FILLER, EP_NAME]
          separator: " - " # Optional, defaults to " - "
```

### Output Format

The field-based output format supports:

- **Field names** (uppercase): SERIES, EP_NUM, FILLER, EP_NAME, RES
- **Literal strings** (any text): "DC", "[v2]", "S01"
- **Optional separator**: Defaults to `-` if not specified
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

## mal_url & afl_url

- To get `mal_url`, visit [MyAnimeList](https://myanimelist.net/) and find the series, copy the URL.
- To get `afl_url`, visit [AnimeFillerList](https://www.animefillerlist.com/) and find the series, copy the URL. In case the series is not listed/no filler, just use `null`.

### Available Fields

These fields can be used in the output configuration.

| Field Name | Description                       | Example                   | Notes                        |
| ---------- | --------------------------------- | ------------------------- | ---------------------------- |
| `SERIES`   | Anime series name (from database) | `Attack on Titan`         | Auto-populated               |
| `EP_NUM`   | Episode number (padded to 3)      | `001`, `123`              | Auto-populated               |
| `EP_NAME`  | Episode title (from database)     | `The Fall of Shiganshina` | Auto-populated               |
| `FILLER`   | Filler marker                     | `[F]` or empty            | Auto-skipped if empty        |
| `RES`      | Resolution                        | `1080p`, `720p`           | From input, skipped if empty |
| `EXT`      | File extension                    | `mkv`, `mp4`              | Always included              |

**Input Pattern Matching:** Use `{{FIELD_NAME}}` placeholders in input patterns to match filenames. `{{ANY}}` matches arbitrary text.

## CLI Commands

### Main Commands

```bash
autotitle <path>              # Rename files in directory
autotitle init [path]         # Create _autotitle.yml config file
autotitle undo <path>         # Restore from backup
autotitle clean <path>        # Remove backup directory
```

### Database Commands

```bash
autotitle db gen <mal_url|mal_id>    # Generate database (writes extended DB; prints title/episode count)
autotitle db path                    # Show database directory
autotitle db list                    # List all cached databases (shows MAL ID and stored title)
autotitle db info <id|url|query>     # Show database info; searches by fuzzy title/slug (interactive if ambiguous)
autotitle db rm <id|url|query>       # Remove specific database; searches by fuzzy title/slug (interactive if ambiguous)
autotitle db rm -a                   # Remove all databases
```

### Flags

| Flag          | Short | Description                      |
| ------------- | ----- | -------------------------------- |
| `--dry-run`   | `-d`  | Preview changes without applying |
| `--no-backup` | `-n`  | Skip backup creation             |
| `--verbose`   | `-v`  | Show detailed output             |
| `--quiet`     | `-q`  | Suppress output except errors    |
| `--config`    | `-c`  | Custom config file path          |
| `--force`     | `-f`  | Overwrite existing files/configs |

## Global Configuration

The global config file is located at:

- Linux/macOS: `~/.config/autotitle/config.yml`

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
    ├── fetcher/             ← External APIs
    ├── logger/              ← CLI logging
    └── util/                ← Utilities
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
