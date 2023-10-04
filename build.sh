#!/bin/bash

BINFILE=watchtower
if [ -n "$MSYSTEM" ]; then
    BINFILE=watchtower.exe
fi
VERSION=$(git describe --tags)
echo "Building $VERSION..."
go build -o $BINFILE -ldflags "-X github.com/nicholas-fedor/watchtower/internal/meta.Version=$VERSION"
