#!/bin/bash
set -euo pipefail

# Build ilnamiqui for multiple platforms
# Usage: ./scripts/build.sh [version]
#   version defaults to "dev"

VERSION="${1:-dev}"
OUTPUT_DIR="./.ilnamiqui"
mkdir -p "$OUTPUT_DIR"

PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64" "windows/amd64")

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    OUTPUT_NAME="ilnamiqui-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    echo "  Building for ${OS}/${ARCH}..."
    GOOS=$OS GOARCH=$ARCH go build \
        -ldflags="-X 'github.com/beabys/ilnamiqui/internal/cli.version=${VERSION}'" \
        -o "${OUTPUT_DIR}/${OUTPUT_NAME}" \
        ./cmd/ilnamiqui/
done

echo ""
echo "  Done. Binaries in ${OUTPUT_DIR}/"
ls -la "${OUTPUT_DIR}/"
