#!/bin/bash

VERSION=$(git describe)
echo "Building $VERSION..."
go build -o watchtower -ldflags "-X github.com/containrrr/watchtower/cmd.version=$VERSION"