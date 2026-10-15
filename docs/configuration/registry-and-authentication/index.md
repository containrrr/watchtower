# Registry & Authentication

### REPO_USER

Sets the username for authenticating with a private registry, such as Docker Hub.

```text
            Argument: None
Environment Variable: REPO_USER
                Type: String
             Default: None
```

!!! Note
    Must be used with `REPO_PASS` to provide valid credentials.
    Suitable for simple username/password authentication.

    For Docker Hub, the registry is implicitly `https://index.docker.io/v1/`.

### REPO_PASS

Sets the password for authenticating with a private registry, such as Docker Hub.

```text
            Argument: None
Environment Variable: REPO_PASS
                Type: String
             Default: None
```

!!! Note
    Must be used with `REPO_USER`.

    Can be a password or a personal access token for registries requiring 2FA (e.g., Docker Hub).

    Use Docker secrets (e.g., `WATCHTOWER_PASS=/run/secrets/repo_pass`) or environment files to avoid exposing sensitive data in command lines.

### DOCKER_CONFIG

Specifies the directory containing the Docker configuration file (`config.json`) for registry authentication.

```text
            Argument: None
Environment Variable: DOCKER_CONFIG
                Type: String
             Default: `/`
```

!!! Note
    Useful for registries requiring complex authentication (e.g., 2FA on Docker Hub) or credential helpers (e.g., AWS ECR).

    Mount the `config.json` file to the container (e.g., `-v ~/.docker/config.json:/config.json`) and set this variable to the directory containing the file (e.g., `/`).

    Changes to the mounted file may require a symlink to ensure updates propagate.

    See [Usage](../../getting-started/usage/index.md) and [Private Registries](../../advanced-features/private-registries/index.md).

    Public `lscr.io` images are hosted on GitHub Container Registry. Watchtower remaps them to `ghcr.io` for digest checks. Authenticate to **`ghcr.io`** in this config (not only `lscr.io`) to separate your checks from anonymous throttling and restore parallel GHCR checks. Pulls of a public org can still share that org's bucket, which is empty for the first minute or two of each hour.

### Skip Registry TLS Verification

Disables TLS certificate verification for registry connections, useful for self-signed certificates or insecure registries.

```text
            Argument: --registry-tls-skip
Environment Variable: WATCHTOWER_REGISTRY_TLS_SKIP
                Type: Boolean
             Default: false
```

!!! Warning
    Use cautiously, as it reduces security.
    Suitable for testing or private registries.

### Minimum Registry TLS Version

Sets the minimum TLS version for registry connections, overriding the default (TLS 1.2).

```text
            Argument: --registry-tls-min-version
Environment Variable: WATCHTOWER_REGISTRY_TLS_MIN_VERSION
     Possible Values: TLS1.0, TLS1.1, TLS1.2, TLS1.3
             Default: TLS1.2
```

!!! Warning
    Using older versions of TLS not recommended for security reasons.

### Proxy Configuration

Watchtower supports HTTP/HTTPS proxies for registry connections by respecting standard environment variables.
Set these in the Watchtower container to route requests (e.g., to Docker Hub or private registries) through a proxy.
This is useful in environments without direct internet access.

Proxy settings are read from the following variables (uppercase and lowercase variants are supported for compatibility):

```text
            Argument: None
Environment Variable: HTTP_PROXY / http_proxy
                Type: String (e.g., "http://proxy.example.com:3128")
             Default: None
```

```text
            Argument: None
Environment Variable: HTTPS_PROXY / https_proxy
                Type: String (e.g., "http://proxy.example.com:3128")
             Default: None
```

```text
            Argument: None
Environment Variable: NO_PROXY / no_proxy
                Type: Comma-separated string (e.g., "localhost,127.0.0.1,internal.example.com")
             Default: None
```

!!! Note
    Proxies may require authentication.
    Include it in the URL (e.g., `http://user:pass@proxy.example.com:3128`), but avoid exposing credentials in the command line by using Docker secrets or environment files instead.

    If your proxy uses a self-signed certificate, combine with `--registry-tls-skip` to disable TLS verification (use cautiously).

For details on how Go handles these variables, see the [net/http.ProxyFromEnvironment](https://pkg.go.dev/net/http#ProxyFromEnvironment){target="_blank" rel="noopener noreferrer"} documentation.

### Warn on HEAD Failure

Controls warnings for failed HEAD requests to registries.
When a HEAD request fails, Watchtower falls back to a full pull, which counts against the registry's pull rate limit.
`Auto` warns for registries known to support HEAD requests and rate-limit pulls (docker.io and ghcr.io).

The warning is written to the log only and is not sent as a notification, because the fallback pull decides the outcome and reports its own failures.

```text
            Argument: --warn-on-head-failure
Environment Variable: WATCHTOWER_WARN_ON_HEAD_FAILURE
     Possible Values: always, auto, never
             Default: auto
```
