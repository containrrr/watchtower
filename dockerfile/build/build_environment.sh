#!/bin/bash

files=$(ls -1 /src | wc -l)
if [ "$files" == "0" ];
then
  echo "Error: Must mount Go source code into /src directory"
  exit 990
fi

# Grab Go package name
pkgName="$(go list -e -f '{{.ImportComment}}' 2>/dev/null || true)"

if [ -z "$pkgName" ];
then
  echo "Error: Must add canonical import path to root package"
  exit 992
fi

# Grab just first path listed in GOPATH
goPath="${GOPATH%%:*}"

# Construct Go package path
pkgPath="$goPath/src/$pkgName"

# Set-up src directory tree in GOPATH
mkdir -p "$(dirname "$pkgPath")"

# Link source dir into GOPATH
ln -sf /src "$pkgPath"
cd "$pkgPath"

echo "Restoring dependencies..."
if [ -e glide.yaml ];
then
  # Install dependencies with glide...
  glide install
else
  # Get all package dependencies
  go get -t -d -v ./...
fi