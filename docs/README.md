# Watchtower Documentation Website Setup

[![Documentation](https://img.shields.io/badge/docs-live-blue)](https://nicholas-fedor.github.io/watchtower/)
[![GitHub Actions](https://img.shields.io/github/actions/workflow/status/nicholas-fedor/watchtower/publish-docs.yaml)](https://github.com/nicholas-fedor/watchtower/actions)

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [MkDocs Configuration](#mkdocs-configuration)
  - [Core Setup](#core-setup)
  - [Key Configuration Details](#key-configuration-details)
    - [Theme Features](#theme-features)
    - [Custom Styling](#custom-styling)
    - [Markdown Extensions](#markdown-extensions)
  - [Plugins](#plugins)
    - [Search Plugin (Built-in MkDocs)](#search-plugin-built-in-mkdocs)
    - [Mike Plugin](#mike-plugin)
    - [git-revision-date-localized Plugin](#git-revision-date-localized-plugin)
    - [mkdocs-minify Plugin](#mkdocs-minify-plugin)
  - [Dependencies](#dependencies)
- [Mike Versioning](#mike-versioning)
  - [Mike's Purpose](#mikes-purpose)
  - [Configuration](#configuration)
  - [Usage](#usage)
    - [Using Mike Locally](#using-mike-locally)
  - [Mike Version Management](#mike-version-management)
    - [Version Types](#version-types)
    - [Alias System](#alias-system)
  - [Deployment Strategy](#deployment-strategy)
- [WASM Template Preview](#wasm-template-preview)
  - [WASM Template Preview Overview](#wasm-template-preview-overview)
  - [Implementation](#implementation)
    - [Core Components](#core-components)
    - [Build Process](#build-process)
    - [JavaScript Integration](#javascript-integration)
    - [States and Levels](#states-and-levels)
  - [Usage in Documentation](#usage-in-documentation)
- [GitHub Workflows](#github-workflows)
  - [Documentation Publishing](#documentation-publishing)
    - [Workflow Triggers](#workflow-triggers)
    - [Workflow Process](#workflow-process)
    - [Version Logic](#version-logic)
- [Deployment Instructions](#deployment-instructions)
  - [Prerequisites](#prerequisites)
  - [Local Development](#local-development)
  - [Production Deployment](#production-deployment)
  - [Version Management (Deployment)](#version-management-deployment)
  - [Troubleshooting](#troubleshooting)
    - [Common Issues](#common-issues)
    - [Validation](#validation)

## Overview

The Watchtower documentation website serves as the central hub for user guides, configuration references, and advanced feature documentation.
The site is statically generated and supports multiple versions to accommodate different Watchtower releases.

This is enabled by the use of several key components including:

- **[MkDocs](https://www.mkdocs.org/)**: Static site generator with Material theme
- **[Mike](https://github.com/jimporter/mike)**: Version management for documentation
- **[GitHub Actions](https://github.com/features/actions)**: Automated build and deployment pipelines

[Back to top](#table-of-contents)

## MkDocs Configuration

**Quick Navigation:**

- [Core Setup](#core-setup)
- [Key Configuration Details](#key-configuration-details)
  - [Theme Features](#theme-features)
  - [Custom Styling](#custom-styling)
  - [Markdown Extensions](#markdown-extensions)
- [Plugins](#plugins)
- [Dependencies](#dependencies)

### Core Setup

The documentation is built using MkDocs with the Material theme.
The main configuration file is located at `build/mkdocs/mkdocs.yaml` and defines:

- **Site Information**: Project name, URL, repository links, and metadata
- **Theme Configuration**: Material theme with custom Watchtower color schemes and navigation features
- **Markdown Extensions**: Enhanced formatting including code highlighting, admonitions, and Mermaid diagrams
- **Navigation Structure**: Hierarchical organization of documentation sections
- **Plugins**: Search, versioning, and revision date functionality

### Key Configuration Details

#### Theme Features

- Instant loading and navigation tracking
- Sticky navigation tabs
- Section-based navigation
- Table of contents following
- Search with suggestions and highlighting
- Code copy buttons and improved tooltips

#### Custom Styling

- Watchtower-specific color palette defined in `docs/assets/styles/theme.css`
- Additional styling in `docs/assets/styles/extra.css`
- Custom theme overrides in `docs/overrides/main.html`

#### Markdown Extensions

- Table of contents with permalinks
- Admonition blocks for notes and warnings
- Attribute lists for HTML attributes
- Definition lists
- Code highlighting with line numbers
- Inline code highlighting
- Superfences for nested code blocks
- Mermaid diagram support
- Magic links for GitHub references
- Tabbed content blocks

### Plugins

#### Search Plugin (Built-in MkDocs)

**Purpose**:

- Provides full-text search functionality across the documentation site using the lunr.js library.
- Enables users to quickly find relevant content through keyword searches with instant results and highlighting.

**Configuration**:

- Enabled by default with the `search` plugin entry.
- Search features are controlled through theme features in the Material theme configuration:
  - `search.suggest` - displays suggestions
  - `search.highlight` - highlights terms
  - `search.share` - enables sharing

**Integration**:

- Automatically indexes all Markdown content during build.
- Search index generated as a JSON file that is loaded client-side.

**Customizations**:

- Styling via CSS in `docs/assets/styles/extra.css`
- Integrates with Material theme.

#### Mike Plugin

**Purpose**:

- Enables versioned documentation deployment with user-selectable version dropdown.
- Critical for maintaining documentation across Watchtower releases.

**Configuration**:

| Option           | Value         | Description                                |
|------------------|---------------|--------------------------------------------|
| version_selector | true          | enables the dropdown                       |
| css_dir          | assets/styles | specifies custom CSS location              |
| alias_type       | redirect      | handles version aliases via HTTP redirects |

**Integration**: Works with `extra.version` config, integrates with GitHub Actions for automated deployment.

**Customizations**: CSS styling in `docs/assets/styles/extra.css`, alias management for flexible version pointing (e.g., `latest` alias automatically updates to newest release).

#### git-revision-date-localized Plugin

**Purpose**:

- Displays localized "last updated" timestamps on pages for content freshness.

**Configuration**:

| Option                | Value   | Description                                      |
|-----------------------|---------|--------------------------------------------------|
| enable_creation_date  | true    | shows both creation and modification dates       |
| type                  | timeago | displays relative time (e.g., "2 days ago")      |

**Integration**:

- Parses Git history.
- Requires full repo history.
- Injects into page footers via Material theme.

**Customizations**:

- Date formatting via plugin options.
- Localization options for different languages.
- CSS styling in `docs/assets/styles/extra.css`.

#### mkdocs-minify Plugin

**Purpose**:

- Minifies HTML, CSS, JS, XML files to reduce sizes and improve performance.

**Configuration**:

- Not explicitly configured.
- Uses defaults (minifies all supported file types).

**Integration**:

- Runs during `mkdocs build`.
- Compatible with theme and plugins.

**Customizations**:

- Can be fine-tuned or disabled for debugging by modifying plugin configuration in `mkdocs.yaml`.

### Dependencies

Python dependencies are managed in `build/mkdocs/docs-requirements.txt`:

- **`mkdocs`**:
  - Core static site generator that processes Markdown files and generates HTML documentation sites.
  - Version handling follows semantic versioning with regular updates for security patches and new features. Critical for the entire documentation pipeline.

- **`mkdocs-material`**:
  - Material Design theme providing modern, responsive UI components and extensive customization options.
  - Version must be aligned with MkDocs to ensure compatibility; typically updated alongside MkDocs major releases.

- **`md-toc`**:
  - Generates table of contents for Markdown files, enhancing navigation within documentation pages.
  - Version stability is important to maintain consistent TOC formatting across all pages.

- **`mkdocs-git-revision-date-localized-plugin`**:
  - Plugin that displays localized "last updated" timestamps on documentation pages.
  - Depends on Git repository access and full commit history
  - Version updates should be tested for localization and date formatting changes.

- **`mkdocs-minify-plugin`**:
  - Minifies HTML, CSS, JavaScript, and XML assets to reduce file sizes and improve page load performance.
  - Optional dependency that should be verified for compatibility with theme customizations and other plugins.

- **`mike`**:
  - Handles versioned documentation deployment and management, enabling multiple documentation versions.
  - Critical for multi-version support
  - Version updates may affect deployment workflows and alias management.

[Back to top](#table-of-contents)

## Mike Versioning

**Quick Navigation:**

- [Mike's Purpose](#mikes-purpose)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Using Mike Locally](#using-mike-locally)
- [Version Management](#mike-version-management)
  - [Version Types](#version-types)
  - [Alias System](#alias-system)
- [Deployment Strategy](#deployment-strategy)

### Mike's Purpose

Mike enables versioned documentation, allowing users to access documentation specific to their Watchtower version.
 This is crucial for maintaining accurate references across different releases.

### Configuration

Mike is configured in `build/mkdocs/mkdocs.yaml` with:

- Version selector enabled
- CSS directory for custom styling
- Redirect-based alias handling

### Usage

#### Using Mike Locally

To use Mike for local development and testing of versioned documentation:

1. **Install Mike**:

   Install Mike using pip:

   ```bash
   pip install mike
   ```

2. **Configuration**:

   Ensure Mike is configured in your `mkdocs.yml` file. The plugin should be added to the plugins list:

   ```yaml
   plugins:
     - mike:
         version_selector: true
         css_dir: assets/styles
         alias_type: redirect
   ```

   > [!NOTE]
   > This configuration is already present in `build/mkdocs/mkdocs.yaml`. The `alias_type: redirect` creates HTML redirects for version aliases, which is suitable for static hosting.

3. **Build and Deploy Locally**:

     To build the documentation and commit the built files to a local Git branch (e.g., gh-pages) without pushing to a remote repository:

     ```bash
     mike deploy <version> --config-file build/mkdocs/mkdocs.yaml
     ```

     Replace `<version>` with the desired version identifier (e.g., `1.0.0`, `dev`). You can also specify aliases:

     ```bash
     mike deploy <version> <alias> --config-file build/mkdocs/mkdocs.yaml
     ```

     Use `--update-aliases` to move an alias from another version if needed.

     > [!NOTE]
     > `mike deploy` commits changes locally by default. To push to a remote repository, add the `--push` flag.

4. **Serve Multiple Versions Locally**:

   To serve all deployed versions locally for testing version switching and navigation:

   ```bash
   mike serve --config-file build/mkdocs/mkdocs.yaml
   ```

   > [!NOTE]
   > This serves documentation from the deployed Git branch, so you must run `mike deploy` first to include your changes in the versioned site.

5. **Serve Single Version Locally**:

   For quick previews during development without versioning features:

   ```bash
   mkdocs serve --config-file build/mkdocs/mkdocs.yaml
   ```

   This serves the current documentation state without requiring deployment commits.

This setup allows you to develop with `mkdocs serve`, deploy versions locally with `mike deploy`, and test the full versioned site with `mike serve` before pushing changes.

### Mike Version Management

#### Version Types

- **Latest**: Points to the most recent stable release
- **Dev**: Development version built from main branch
- **Tagged Releases**: Specific version tags (e.g., v1.2.3)

#### Alias System

- `latest` alias automatically updates to point to new releases
- Version-specific aliases for direct access to historical docs

### Deployment Strategy

Versions are deployed automatically via GitHub Actions:

- Release events create new versioned documentation
- Main branch pushes update the `dev` version
- Aliases are managed to ensure `latest` always points to the newest release

[Back to top](#table-of-contents)

## WASM Template Preview

**Quick Navigation:**

- [Overview](#wasm-template-preview-overview)
- [Implementation](#implementation)
  - [Core Components](#core-components)
  - [Build Process](#build-process)
  - [JavaScript Integration](#javascript-integration)
  - [States and Levels](#states-and-levels)
- [Usage in Documentation](#usage-in-documentation)

### WASM Template Preview Overview

- The WASM template preview provides an interactive way for users to test notification templates directly in the browser.
- This feature allows real-time rendering of templates with different container states and log levels.

### Implementation

#### Core Components

- `tools/tplprev/`: Nested `github.com/nicholas-fedor/tplprev` module (CLI + WASM)
- `scripts/build-tplprev.sh`: Build script for WASM compilation

#### Build Process

1. Compiles Go code to WebAssembly using `GOARCH=wasm GOOS=js`
2. Copies `wasm_exec.js` from Go installation or downloads from GitHub
3. Outputs `tplprev.wasm` and `wasm_exec.js` to `docs/assets/`

#### JavaScript Integration

The WASM module exposes a `WATCHTOWER.tplprev` function that accepts:

- Template string
- Array of container states (scanned, updated, failed, etc.)
- Array of log levels (fatal, error, warn, info, debug, trace)

#### States and Levels

- **States**: Scanned (c), Updated (u), Failed (e), Skipped (k), Stale (t), Fresh (f)
- **Levels**: Fatal (f), Error (e), Warn (w), Info (i), Debug (d), Trace (t)

### Usage in Documentation

The template preview is integrated into `docs/notifications/template-preview/index.md` with:

- Interactive HTML form
- JavaScript for WASM execution
- CSS styling for the preview interface

[Back to top](#table-of-contents)

## GitHub Workflows

**Quick Navigation:**

- [Documentation Publishing (`publish-docs.yaml`)](#documentation-publishing)
  - [Triggers (Documentation Publishing)](#workflow-triggers)
  - [Process (Documentation Publishing)](#workflow-process)
  - [Version Logic](#version-logic)

### Documentation Publishing

#### Workflow Triggers

- Manual dispatch
- Pushes to main branch affecting `docs/` directory
- Release publications

#### Workflow Process

1. **Checkout**: Full repository with history for versioning
2. **Go Setup**: Installs Go 1.27.x for WASM compilation
3. **Template Preview Build**: Executes `scripts/build-tplprev.sh`
4. **Python Setup**: Configures Python with pip caching
5. **MkDocs Installation**: Installs dependencies from `docs-requirements.txt`
6. **Version Determination**: Sets version based on trigger type
7. **Deployment**: Uses Mike to build and deploy versioned documentation

#### Version Logic

- Releases: Uses tag name as version, sets `latest` alias
- Main branch: Deploys as `dev` version
- Manual: Deploys as `dev` version

[Back to top](#table-of-contents)

## Deployment Instructions

**Quick Navigation:**

- [Prerequisites](#prerequisites)
- [Local Development](#local-development)
- [Production Deployment](#production-deployment)
- [Version Management](#version-management-deployment)
- [Troubleshooting](#troubleshooting)
  - [Common Issues](#common-issues)
  - [Validation](#validation)

### Prerequisites

- Python 3.x with pip
- Go 1.27.x or later
- Git with full repository history

### Local Development

1. **Clone Repository**:

    ```bash
    git clone https://github.com/nicholas-fedor/watchtower.git
    cd watchtower
    ```

2. **Create and activate Python virtual environment**:

    ```bash
    python -m venv watchtower-docs
    . watchtower-docs/bin/activate
    ```

3. **Install Dependencies**:

    ```bash
    pip install -r build/mkdocs/docs-requirements.txt
    ```

4. **Build Template Preview**:

    ```bash
    chmod +x scripts/build-tplprev.sh
    ./scripts/build-tplprev.sh
    ```

5. **Serve Documentation Locally**:

    ```bash
    mkdocs serve --config-file build/mkdocs/mkdocs.yaml
    ```

### Production Deployment

The documentation is automatically deployed via GitHub Actions. Manual deployment can be triggered through the Actions tab in the repository.

**For Maintainers**:

- Documentation changes are deployed on merge to main
- New releases automatically create versioned docs
- Version aliases are managed automatically
- Manual deployments can be triggered for testing

### Version Management (Deployment)

**Creating New Versions**:

1. Create a GitHub release with appropriate tag
2. The `publish-docs.yaml` workflow will automatically deploy the version
3. Update `latest` alias to point to the new release

**Managing Aliases**:

- `latest` is automatically managed by the deployment workflow
- Custom aliases can be set using Mike commands locally for testing

### Troubleshooting

#### Common Issues

- **WASM Build Failures**: Ensure Go version matches CI (1.27.x)
- **Missing Dependencies**: Clear pip cache and reinstall
- **Version Conflicts**: Check Mike version aliases with `mike list`

#### Validation

- Use `mkdocs build --config-file build/mkdocs/mkdocs.yaml` for local validation
- Check link validation warnings in build output
- Verify WASM functionality in browser developer console

[Back to top](#table-of-contents)
