<!-- Developer guide: local dev and test commands -->
# Developer Guide — Running tests & race detector

This short guide covers how to run unit tests locally and how to run the race-enabled test suite in a Linux container (recommended for Windows hosts).

## Prerequisites

- Go toolchain (version compatible with project go.mod). To run `go test` locally, ensure `go` is in your PATH.
- Docker (for running a Linux container to execute `-race` with CGO enabled)
- Optional: GitHub CLI `gh` to open PRs from the command line.

## Run unit tests locally

From the repository root:

PowerShell

```powershell
go test ./... -v
```

If you only want to run a package tests, run:

```powershell
go test ./pkg/registry/auth -v
```

## Run race detector (recommended via container on Windows)

The Go race detector requires cgo and a C toolchain. On Linux runners this is usually available; on Windows it's simplest to run tests inside a Linux container.

Example (PowerShell):

```powershell
docker run --rm -v "${PWD}:/work" -w /work -e CGO_ENABLED=1 golang:1.20 bash -lc "apt-get update && apt-get install -y build-essential ; /usr/local/go/bin/go test -race ./... -v"
```

Notes:
- The command mounts the current working directory into the container and installs `build-essential` to provide a C toolchain so `-race` works.
- If you prefer a faster run, run `go test -run TestName ./pkg/yourpkg -race`.

## Render PlantUML diagrams (local)

To render PlantUML into SVG using Docker (no Java/PlantUML install required):

```powershell
docker run --rm -v "${PWD}:/work" -w /work plantuml/plantuml -tsvg docs/diagrams/update-flow.puml
```

Move the generated SVG into the docs assets folder:

```powershell
mkdir docs/assets/images -Force
Move-Item docs/diagrams/update-flow.svg docs/assets/images/update-flow.svg -Force
```

## Create a branch and PR (example)

Example git commands:

```powershell
git checkout -b docs/update-flow
git add docs/update-flow.md docs/diagrams/update-flow.puml docs/developer-guide.md docs/assets/images/update-flow.svg
git commit -m "docs: add update flow docs, diagrams and developer guide"
git push -u origin docs/update-flow
```

If you have the GitHub CLI installed you can open a PR with:

```powershell
gh pr create --title "docs: update flow + diagrams" --body "Adds update flow documentation, a PlantUML diagram and developer guide." --base main
```

If `gh` is not installed you can open a PR via GitHub web UI after pushing the branch.

---

If you'd like, I can push the branch and attempt to open the PR for you now.
