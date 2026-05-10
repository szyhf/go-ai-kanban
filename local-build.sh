#!/bin/bash

set -e  # Exit on any error

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case "$ARCH" in
  x86_64)
    ARCH="x64"
    GOARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    GOARCH="arm64"
    ;;
  *)
    echo "Warning: Unknown architecture $ARCH, using as-is"
    GOARCH="$ARCH"
    ;;
esac

# Map OS names
case "$OS" in
  linux)
    OS="linux"
    GOOS="linux"
    ;;
  darwin)
    OS="macos"
    GOOS="darwin"
    ;;
  *)
    echo "Warning: Unknown OS $OS, using as-is"
    GOOS="$OS"
    ;;
esac

PLATFORM="${OS}-${ARCH}"

echo "Detected platform: $PLATFORM"

echo "Cleaning previous builds..."
rm -rf npx-cli/dist
mkdir -p npx-cli/dist/$PLATFORM

echo "Building web app..."
(cd packages/local-web && npm run build)

echo "Building Go backend..."
CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o server ./cmd/server

echo "Creating distribution package..."
zip -q vibe-kanban.zip server
rm -f server
mv vibe-kanban.zip npx-cli/dist/$PLATFORM/vibe-kanban.zip

echo "CLI build complete!"
echo "Files created:"
echo "   - npx-cli/dist/$PLATFORM/vibe-kanban.zip"

echo ""
echo "Installing npx-cli dependencies..."
(cd npx-cli && npm ci)

echo ""
echo "Building npx-cli TypeScript..."
(cd npx-cli && npm run build)

echo ""
echo "To test locally, run:"
echo "   cd npx-cli && node bin/cli.js"
