# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

- Add `--registry-ca-validate` flag: when supplied with `--registry-ca`, Watchtower can validate the provided CA bundle on startup and fail fast on misconfiguration. Prefer using this over `--insecure-registry` in production.
 
- Security: registry TLS verification is now secure-by-default for internal HEAD/token requests; `--insecure-registry` is opt-in for testing.
- Registry CA support: add `--registry-ca` to provide a PEM bundle merged into system roots, and `--registry-ca-validate` to fail-fast on invalid bundles.
- Registry token caching: in-memory, concurrent-safe token cache added for registry auth tokens (honors `expires_in`), with deterministic and concurrency unit tests.
- Testability: refactored registry transport construction and exposed test helpers; added an injectable `now` variable for deterministic time-dependent tests.
- Docs: added detailed update flow docs, diagrams, and a developer guide (`docs/update-flow*.md`, PlantUML, and rendered SVG).
- CI: added a GitHub Actions workflow to run `go test -race ./...` with CGO enabled; recommended containerized `-race` run steps added to the developer guide.
