#!/bin/bash

# get the repo root directory safely
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT" || { echo "Error: Failed to change directory"; exit 1; }

cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./docs/assets/

# build webassembly binary
GOARCH=wasm GOOS=js go build -o ./docs/assets/tplprev.wasm ./tplprev
if [ $? -ne 0 ]; then
    echo "Error: WASM build failed!"
    exit 1
fi

echo "WASM build successful!"