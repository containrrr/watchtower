# Git Monitoring

## Git Enable

Sets the process-wide default for Git ref change detection.
When true, associated containers are watched unless a container [`com.centurylinklabs.watchtower.git-watch`](../../advanced-features/git-monitoring/index.md#labels) label overrides it.

```text
            Argument: --git-enable
Environment Variable: WATCHTOWER_GIT_ENABLE
                Type: Boolean
             Default: false
```

!!! Note
    A repository URL does not enable the watcher by itself.
    See [Git Monitoring](../../advanced-features/git-monitoring/index.md#behavior).

## Git Image

Associates an image name with a Git repository.
Repeatable.
Not comma-glued.
Format: `image=repo[#ref][@policy]`.

```text
            Argument: --git-image
Environment Variable: WATCHTOWER_GIT_IMAGE
                Type: String array
             Default: (empty)
```

!!! Note
    See [Git Monitoring](../../advanced-features/git-monitoring/index.md#per-image_mapping).

## Git Auth Token

HTTPS authentication token for private repositories.
Takes priority over username/password and SSH.

```text
            Argument: --git-auth-token
Environment Variable: WATCHTOWER_GIT_AUTH_TOKEN
                Type: String
             Default: (empty)
```

!!! Note
    Supports a file path for [Docker Secrets](../../getting-started/docker-secrets/index.md) (for example `/run/secrets/git_auth_token`).
    Prefer Docker Secrets over putting the token in the environment.
    Tokens never appear in labels, `/v1/config`, or events.

## Git Username

Basic-auth username for private repositories.

```text
            Argument: --git-username
Environment Variable: WATCHTOWER_GIT_USERNAME
                Type: String
             Default: (empty)
```

## Git Password

Basic-auth password for private repositories.

```text
            Argument: --git-password
Environment Variable: WATCHTOWER_GIT_PASSWORD
                Type: String
             Default: (empty)
```

!!! Note
    Supports a file path for [Docker Secrets](../../getting-started/docker-secrets/index.md) (for example `/run/secrets/git_password`).
    Prefer Docker Secrets over putting the password in the environment.
    Passwords never appear in labels, `/v1/config`, or events.

## Git SSH Key Path

Path to an SSH private key used for clone, ls-remote, and local path project checkout.

```text
            Argument: --git-ssh-key-path
Environment Variable: WATCHTOWER_GIT_SSH_KEY_PATH
                Type: String
             Default: (empty)
```

## Git SSH Known Hosts

Path to an SSH `known_hosts` file used to verify Git hosts.
Required for SSH remotes in the scratch runtime image, which has no default host keys.

```text
            Argument: --git-ssh-known-hosts
Environment Variable: WATCHTOWER_GIT_SSH_KNOWN_HOSTS
                Type: String
             Default: (empty)
```

!!! Note
    See [Git Monitoring](../../advanced-features/git-monitoring/index.md#authentication).

## Git Timeout

Timeout for Git network operations (for example `30s` or `1m`).

```text
            Argument: --git-timeout
Environment Variable: WATCHTOWER_GIT_TIMEOUT
                Type: Duration
             Default: 30s
```

## Git CA Bundle

Path to a PEM file of extra CA certificates for Git HTTPS.
Applied to clone, ls-remote, REST probes, and local path project checkout.

```text
            Argument: --git-ca-bundle
Environment Variable: WATCHTOWER_GIT_CA_BUNDLE
                Type: String
             Default: (empty)
```

## Git Insecure Skip TLS

Skip TLS verification for Git HTTPS.
Applied to clone, ls-remote, REST probes, and local path project checkout.

```text
            Argument: --git-insecure-skip-tls
Environment Variable: WATCHTOWER_GIT_INSECURE_SKIP_TLS
                Type: Boolean
             Default: false
```

## Git Dockerfile

Default Dockerfile path relative to the build context.
Used when a container does not set `com.centurylinklabs.watchtower.git-dockerfile`.
Empty means `Dockerfile` in the build context.

```text
            Argument: --git-dockerfile
Environment Variable: WATCHTOWER_GIT_DOCKERFILE
                Type: String
             Default: (empty)
```

!!! Note
    See [Git Monitoring](../../advanced-features/git-monitoring/index.md#labels).
    Watchtower's own image uses `build/docker/Dockerfile` with the repository root as the context.

## Git Context

Default subdirectory of a Git URL context sent to the Docker daemon as the build context.
Used when a container does not set `com.centurylinklabs.watchtower.git-context`.
Empty means the repository root.

```text
            Argument: --git-context
Environment Variable: WATCHTOWER_GIT_CONTEXT
                Type: String
             Default: (empty)
```

!!! Note
    See [Git Monitoring](../../advanced-features/git-monitoring/index.md#labels).
    Keep the context at the repository root when the Dockerfile `COPY`s files from outside its directory.

## Git Compose Stash

Save local files in a Compose project directory, check out the monitored commit, then write those files back.
Use this when the project has local files that are not gitignored, such as an extra `.env`.
When false, a dirty worktree aborts Compose apply.
Compose interpolates that project's `.env`. It does not interpolate Watchtower's process environment.

```text
            Argument: --git-compose-stash
Environment Variable: WATCHTOWER_GIT_COMPOSE_STASH
                Type: Boolean
             Default: false
```

## Compose Project

Maps a Docker Compose project name to a directory inside Watchtower.
Format: `name=/path`. Repeatable, newline-separated.

This is an explicit opt-in for a [local path context](../../advanced-features/git-monitoring/index.md#local_path_context), together with `com.centurylinklabs.watchtower.compose-dir`.
Watchtower does not infer a project from `com.docker.compose.project.working_dir`.
Each applied container needs `com.docker.compose.service`.

!!! Note
    See [Local path context](../../advanced-features/git-monitoring/index.md#local_path_context).

```text
            Argument: --compose-project
Environment Variable: WATCHTOWER_COMPOSE_PROJECT
                Type: List
             Default: (empty)
```
