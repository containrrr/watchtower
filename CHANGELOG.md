# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

- Add `--registry-ca-validate` flag: when supplied with `--registry-ca`, Watchtower can validate the provided CA bundle on startup and fail fast on misconfiguration. Prefer using this over `--insecure-registry` in production.
