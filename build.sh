#!/bin/bash

# check if `go` is installed or not
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go before running this script."
    exit 1
fi

BINFILE=watchtower
if [ -n "$MSYSTEM" ]; then
    BINFILE=watchtower.exe
fi
VERSION=$(git describe --tags)
echo "Building $VERSION..."
go build -o $BINFILE -ldflags "-X github.com/containrrr/watchtower/internal/meta.Version=$VERSION"

if [ $? -ne 0 ]; then
    echo "Error: Build failed!"
    exit 1
fi

echo "Build successful!"