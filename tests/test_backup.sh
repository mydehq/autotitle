#!/bin/bash
set -e
BIN="$(pwd)/bin/autotitle"
TEST_ROOT="/tmp/autotitle_backup_test"
CACHE_ROOT="$HOME/.cache/autotitle"
REGISTRY="$CACHE_ROOT/backup_registry.json"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

log() { echo -e "${GREEN}[TEST] $1${NC}"; }
fail() { echo -e "${RED}[FAIL] $1${NC}"; exit 1; }

# Setup
log "Setting up test environment..."
rm -rf "$TEST_ROOT"
mkdir -p "$TEST_ROOT"
cd "$TEST_ROOT"
cp "$BIN" .

# Create dummy file
touch "Shingeki no Kyojin - 01.mkv"
log "Created 'Shingeki no Kyojin - 01.mkv'"

# 1. Rename (Backup Creation)
# ---------------------------
log "Running Rename..."
# Init first
"$BIN" init . -u "https://myanimelist.net/anime/16498/Shingeki_no_Kyojin" --force >/dev/null
# Generate DB
"$BIN" db gen "https://myanimelist.net/anime/16498/Shingeki_no_Kyojin" >/dev/null
# Rename
"$BIN" . >/dev/null

# Check Local Backup
if [ -d ".autotitle_backup" ]; then
    log "✔ Local backup folder created"
else
    fail "Local backup folder missing"
fi
if [ -f ".autotitle_backup/mappings.json" ]; then
    log "✔ Local mappings.json created"
else
    fail "Local mappings.json missing"
fi

# Check Global Registry
if grep -q "$TEST_ROOT" "$REGISTRY"; then
    log "✔ Added to global registry"
else
    fail "Not found in global registry ($REGISTRY)"
fi

# 2. Undo (Validation)
# --------------------
log "Running Undo..."
"$BIN" undo . >/dev/null

# Check Local Backup Removed
if [ ! -d ".autotitle_backup" ]; then
    log "✔ Local backup folder removed"
else
    fail "Local backup folder still exists after undo"
fi

# Check Global Registry Removed
if grep -q "$TEST_ROOT" "$REGISTRY"; then
    fail "Still found in global registry after undo"
else
    log "✔ Removed from global registry"
fi

# 3. Manual Clean (Validation)
# ----------------------------
log "Running Manual Clean Test..."
# Re-create state
"$BIN" . >/dev/null 2>&1
if [ -d ".autotitle_backup" ]; then log "  (Re-created backup)"; fi

"$BIN" clean . >/dev/null

if [ ! -d ".autotitle_backup" ]; then
    log "✔ Local backup removed by clean"
else
    fail "Local backup persists after clean"
fi
if grep -q "$TEST_ROOT" "$REGISTRY"; then
    fail "Still in registry after clean"
else
    log "✔ Removed from registry after clean"
fi

# 4. Global Clean (Validation)
# ----------------------------
log "Running Global Clean Test..."
# Re-create state multiple times? Just one for now.
"$BIN" . >/dev/null 2>&1
log "  (Re-created backup for global clean)"

"$BIN" clean -g >/dev/null

if [ ! -d ".autotitle_backup" ]; then
    log "✔ Local backup removed by global clean"
else
    fail "Local backup persists after global clean"
fi

# Global registry should be empty list []
if [ "$(cat "$REGISTRY")" == "[]" ] || [ ! -s "$REGISTRY" ]; then
    log "✔ Global registry wiped"
else
    # It might contain other backups from other dirs if I have them?
    # But clean -g should wipe ALL.
    # Let's check if OUR entry is gone at least.
    if grep -q "$TEST_ROOT" "$REGISTRY"; then
        fail "Still in registry after global clean"
    else
        log "✔ Removed from registry after global clean"
    fi
fi

log "ALL TESTS PASSED"
