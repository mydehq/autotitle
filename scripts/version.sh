#!/usr/bin/env bash

export MODULE=$(grep "^module" go.mod | awk '{print $2}')
export LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
export VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")
export COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
export DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

export LDFLAGS_BASE="-X \"$MODULE/internal/version.Version=$VERSION\" -X \"$MODULE/internal/version.Commit=$COMMIT\" -X \"$MODULE/internal/version.Date=$DATE\""
export LDFLAGS_RELEASE="-s -w -X \"$MODULE/internal/version.Version=$VERSION\" -X \"$MODULE/internal/version.Commit=$COMMIT\" -X \"$MODULE/internal/version.Date=$DATE\""
