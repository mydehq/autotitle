#!/bin/bash

set -e

NEW_VERSION="${1:-$CZ_PRE_NEW_VERSION}"

# Check if version was passed
if [ -z "$NEW_VERSION" ]; then
    echo "Error: No version argument provided."
    exit 1
fi

echo -e "\033[38;5;244m -> Bumping PKGBUILD to ${NEW_VERSION}...\033[0m"

# 1. Update pkgver
sed -i "s/^ *pkgver=.*/pkgver=${NEW_VERSION}/" PKGBUILD

# 2. Reset pkgrel to 1
sed -i "s/^ *pkgrel=.*/pkgrel=1/" PKGBUILD

# 4. Stage the file for the release commit
git add PKGBUILD

echo -e "\033[0;32m -> PKGBUILD updated and staged.\033[0m"
