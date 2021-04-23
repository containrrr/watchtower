#!/usr/bin/env bash

go test -v -coverprofile coverage.out -covermode atomic ./...

# Requires CODECOV_TOKEN to be set
bash <(curl -s https://codecov.io/bash)