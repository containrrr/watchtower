## Prerequisites

The nicholas-fedor/watchtower fork of Watchtower is intended to help renew efforts into maintaining and improving the Watchtower project.

## Tools

To contribute code changes to this project you will need the following development tools:

* [Go](https://go.dev/doc/install)
* [Docker](https://docs.docker.com/engine/installation/)
* [Task](https://taskfile.dev/installation/) (runs the project tasks defined in `Taskfile.yml`)

Run `task` from the repository root to list the available tasks.

It is highly recommended to have the latest version of Go installed.
You can check for your current Go version as follows:

```bash
go version
```

## Devcontainer

### Overview

[Dev Containers](https://docs.github.com/en/codespaces/setting-up-your-project-for-codespaces/adding-a-dev-container-configuration/introduction-to-dev-containers) are a method of providing a standardized development environment when working on a project.

This repository provides a baseline configuration that is expected to help contributors quickly get started.

### Getting Started

If you are using VSCode and new to using Dev Containers, then review the following documentation to help you get started: [https://code.visualstudio.com/docs/devcontainers/containers](https://code.visualstudio.com/docs/devcontainers/containers)
Documentation for GitHub's implementation, Codespaces, can be found here: [https://docs.github.com/en/codespaces/quickstart](https://docs.github.com/en/codespaces/quickstart)

## Security

You must have GPG signing of Git commits enabled.
GitHub provides excellent resources for how to configure this:

* [Generating a GPG key](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key#generating-a-gpg-key)
* [Configuring Git for GPG signing](https://docs.github.com/en/authentication/managing-commit-signature-verification/telling-git-about-your-signing-key#telling-git-about-your-gpg-key)
* [GPG signing Git commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)

## Checking out the code

Do not place your code in the Go source path.

```bash
git clone git@github.com:<your fork>/watchtower.git
cd watchtower
```

## Linting

Watchtower uses [Golangci-lint](https://golangci-lint.run/) to help maintain code quality.
The configuration file can be found at `build/golangci-lint/golangci-lint.yaml`.
It can be installed locally using the following [instructions](https://golangci-lint.run/docs/welcome/install/local/).

The preferred method of using the linter is the following task:

```bash
task lint
```

This runs Golangci-lint with the project configuration and applies automatic fixes where possible (for example, `modernize` rewrites).
To check for issues without modifying any files, run:

```bash
task lint:check
```

## Formatting

Golangci-lint can also be used to format the codebase:

```bash
task fmt
```

Swagger annotation comments are formatted separately with the [swag](https://github.com/swaggo/swag) CLI, which is pinned as a Go tool in `go.mod`:

```bash
task swag:fmt
```

After changing API annotations, regenerate the Swagger documents in `internal/api/swagger/` and commit them.
CI fails when the committed documents differ from freshly generated ones, which `task swag:check` reproduces locally:

```bash
task swag:gen
task swag:check
```

## Testing

### Mocking

[Mockery](https://vektra.github.io/mockery/latest/) is used to generate mock implementations of interfaces.
It is pinned as a Go tool in `go.mod` and configured using the `build/mockery/mockery.yaml` file.

To generate new mock implementations of Watchtower's interfaces, run the following from the root directory:

```bash
task mocks
```

It removes the generated mocks before regenerating them, so the mock of a removed or renamed interface is deleted with it.
Hand-written fakes in the `mocks` directories are kept.

`task mocks:check` regenerates the mocks and fails if they differ from the committed ones.
It leaves the regenerated files in place so you can review and commit them.

### Executing Unit Tests

To execute Watchtower's unit tests, run the following task from the root directory:

```bash
task test
```

This will run the `go test` command with the following flags:

* `-timeout 30s` - sets the timeout for the tests to 30 seconds
* `-coverprofile coverage.out` - generates a coverage profile
* `-covermode atomic` - sets the cover mode to atomic

Tests written with the standard `testing` package send Watchtower's log output to the test that produced it, so a failing test prints what it logged.
Add `-v` to a `go test` command to see the output of passing tests too.

Two further tasks help catch concurrency and test-isolation bugs:

* `task test:race` - runs the tests with the race detector
* `task test:repeat` - runs every package twice in shuffled order to catch state leaking between tests

### Fuzzing

`task test` runs each fuzz target's seed corpus only.
To fuzz the targets, run:

```bash
task fuzz
```

This fuzzes every target in the module for 10 seconds each, one after another, and stops at the first failing input.
Go saves that input under the package's `testdata/fuzz` directory, where `go test` replays it as a regression case until it is removed.
Three variables adjust the run:

* `FUZZTIME` - how long to fuzz each target, for example `task fuzz FUZZTIME=1m`
* `FUZZPKGS` - the packages to fuzz, for example `task fuzz FUZZPKGS=./internal/registry/...`
* `FUZZPARALLEL` - the number of fuzzing workers, 8 by default

### Golden Files

Some tests compare output against golden files in `testdata/` directories, such as the `--help` output, the flag manifest, and the inventory of environment variables and container labels.
These guard Watchtower's user-facing configuration surface, so a failing golden test means a user-visible change.
If the change is intended, regenerate the files and review the diff before committing:

```bash
task test:update-golden
```

The template preview module in `tools/tplprev` cannot import Watchtower, so it keeps a generated copy of the built-in notification templates, along with generated lists of the container report methods templates can call and the container keys in the JSON output.
A test fails when these copies differ from `pkg/notifications`, and the preview module's own tests check its report type and JSON output against the lists.
After changing a built-in template, the container report, or the JSON output, regenerate the copies with `task tplprev:gen` (or `task test:update-golden`), update the preview module to match, and commit both.

## Building

### Binary and Archives

#### Using go build

```bash
go build                               # compiles and packages an executable binary, watchtower
go test ./...                          # runs the tests
./watchtower                           # runs the application (outside of a container)
```

The equivalent tasks are `task build` (writes `bin/watchtower`) and `task run` (pass flags after `--`, for example `task run -- --help`).

For cross-compiling to other architectures (e.g., amd64, arm64, arm/v7, 386, riscv64), set environment variables like `GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0` before running `go build`. Example for arm/v7:

```bash
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o watchtower-armhf
```

#### Using GoReleaser

To build the Watchtower binary and archives for production releases, use GoReleaser with the `build/goreleaser/stable.yaml` configuration. This handles cross-compilation, versioning, and packaging for multiple architectures (amd64, i386, armhf, arm64v8, riscv64) and OS (Linux, Windows).

The `release-stable.yaml` workflow runs on a tag push (e.g., `v1.2.3`) or manually via GitHub Actions, producing full builds with SBOM and provenance attestations.

For local testing, run GoReleaser in snapshot mode:

```bash
task release:snapshot
```

This runs `goreleaser release --config build/goreleaser/stable.yaml --snapshot --clean`.

This produces binaries in `dist/` (e.g., `dist/watchtower_linux_amd64/watchtower`) and archives (e.g., `watchtower_linux_amd64_1.11.6.tar.gz` if versioned).

### Docker Image

To build Watchtower images, use GoReleaser for multi-architecture support with attestations.

Nightly images are built by the `release-nightly.yaml` workflow, which runs on a daily schedule or manually via GitHub Actions. Locally:

```bash
task release:snapshot CONFIG=nightly
```

To build a Watchtower image of your own, use the self-contained Dockerfiles in /build/docker/:

* `/build/docker/Dockerfile.self-local` will build an image based on your current local Watchtower files.
* `/build/docker/Dockerfile.self-github` will build an image based on current Watchtower's repository on GitHub.

```bash
task docker:build # builds an image named watchtower from local files using Dockerfile.self-local
```

For multi-architecture nightly images (amd64, i386, armhf, arm64v8, riscv64), use Docker Buildx after cross-compiling binaries to `dist/watchtower_linux_{GOARCH}/watchtower` (matching the nightly workflow structure). Alternatively, trigger the `release-nightly.yaml` workflow manually via GitHub Actions for image builds with SBOM and provenance attestations.

For stable images (with binaries/archives), use the stable config as above.

The shared `build/docker/Dockerfile` is used for both, with COPY watchtower /watchtower matching GoReleaser's binary placement.

## Submitting Pull Requests

* Before submitting, ensure you have GPG signed your Git commits.
* All commit messages are expected to follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) guidelines.
* If the pull request is intended to address an issue from either this fork, another fork, or an upstream issue, please ensure to at least add a comment to reference it.
  GitHub automatically generates cross-references, which is incredibly helpful for anyone else maintaining forks of Watchtower or relying upon the upstream repository.
