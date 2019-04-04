#!/bin/bash -e

if [ -z "$1" ]
then
    echo "No argument supplied, please either supply 'release' for a release build or 'ci' for ci build."
    exit 1
fi

source /build_environment.sh

# Grab the last segment from the package name
name=${pkgName##*/}

echo "Running Tests $pkgName..."
(
  go test -v $(glide novendor) || exit 1
)

if [ "$1" == "release" ]
then
  echo "Release Building $pkgName..."
  CGO_ENABLED=${CGO_ENABLED:-0} \
  goreleaser
else
  echo "Snapshot Building $pkgName..."
  CGO_ENABLED=${CGO_ENABLED:-0} \
  goreleaser --snapshot --skip-publish
fi