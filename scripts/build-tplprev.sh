#!/bin/bash

cd $(git rev-parse --show-toplevel)

cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./docs/assets/

GOARCH=wasm GOOS=js go build -o ./docs/assets/tplprev.wasm ./tplprev