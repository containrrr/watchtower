# Watchtower's Documentation Website Build Configuration

## Overview

This directory contains the configurations for building Watchtower's documentation website.
The website is statically generated using [MkDocs](https://www.mkdocs.org/) with the Material theme.
It is then deployed to GitHub Pages.

## Directory Structure

- `build/mkdocs/`: Contains the MkDocs configuration files
- `docs/`: Contains the source documentation files
- `overrides/`: Contains custom theme overrides
- `scripts/`: Contains scripts for generating documentation

## Key Files

- `docs-requirements.txt`: Python dependencies for documentation build
- `mkdocs.yaml`: Main MkDocs configuration file
- `scripts/build-tplprev.sh`: Builds `tplprev.wasm` and copies `wasm_exec.js` into `docs/assets/`

## Build Process

The documentation build process involves:

1. Setting up a Python virtual environment
2. Installing dependencies from `docs-requirements.txt`
3. Generating the template preview by running `scripts/build-tplprev.sh`, which builds `tplprev.wasm` and copies `wasm_exec.js` into `docs/assets/`
4. Building the static site using MkDocs

## Versioning

The documentation uses the Mike plugin for MkDocs to manage multiple versions of the documentation.
This allows users to access different versions of the documentation based on the Watchtower version they're using.

## Deployment

The built documentation is deployed to GitHub Pages.
The deployment process uses GitHub Actions to automatically build and deploy the documentation on each release.

## Customization

The documentation uses custom theme overrides in the `overrides/` directory to customize the Material theme for Watchtower's needs.

## Contributing

Contributions to the documentation are welcome.
Please follow the standard GitHub pull request process for versioning.
