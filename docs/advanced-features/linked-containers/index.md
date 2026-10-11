# Linked Containers

## Overview

Watchtower, by default, ensures that interdependent containers are updated in the correct order to maintain application stability.
It automatically detects container dependencies through various mechanisms and uses topological sorting to determine the optimal update sequence.

When containers depend-upon each other (such as a web application depending on a database), updating them in the wrong order can cause service disruptions.

Watchtower addresses this by:

- **Detecting dependencies** through Docker links, labels, and network configurations
- **Performing topological sorting** to determine the correct update order
- **Stopping containers in reverse dependency order** (dependents first - i.e., web app before database)
- **Restarting containers in dependency order** (dependencies first - i.e., database before web app)

This ensures that dependent services are stopped before their dependencies are updated, and dependencies are available when dependents restart.

## How It Works

### Dependency Detection

Watchtower detects container dependencies through multiple mechanisms, checked in the following priority order:

1. **Watchtower depends-on label** (`com.centurylinklabs.watchtower.depends-on`)
2. **Docker Compose depends_on label** (`com.docker.compose.depends_on`)
3. **Docker links and network mode** (legacy Docker linking and `network_mode: service:container`)

#### Watchtower Depends-On Label

The `com.centurylinklabs.watchtower.depends-on` label allows explicit declaration of dependencies:

```dockerfile title="Explicit dependency declaration"
LABEL com.centurylinklabs.watchtower.depends-on="database,redis"
```

This label accepts a comma-separated list of container names that must be available before this container starts.
This supports referencing containers from other Docker Compose projects or stacks, enabling cross-project dependency management.

An entry may also name a Docker Compose service, such as `db` or `my-database`, or a project-qualified service, such as `myproject-db`.
A service name resolves to every replica of that service.

A service name used in more than one Compose project, such as `db`, resolves to the service in the labeled container's own project when that project has one.
Otherwise it resolves to the service in every project that has it, so the container restarts when any of them is updated.
When the service in its own project is unmonitored, the entry is left alone instead (see [Dependencies on Unmonitored Containers](#dependencies_on_unmonitored_containers)).
Use the container name or a project-qualified service name, such as `myproject-db`, to depend on only one of them.

!!! Warning
    An entry that names no container is ignored and logged as a warning.

#### Docker Compose Depends-On Label

Watchtower automatically recognizes Docker Compose's `depends_on` relationships:

```yaml title="Docker Compose dependency"
services:
  web:
    image: nginx
    depends_on:
      - database
  database:
    image: postgres
```

The `com.docker.compose.depends_on` label is automatically set by Docker Compose and parsed by Watchtower to extract service names and support implicitly restarting linked services.

<!-- markdownlint-disable MD046 -->
!!! Important "Docker Compose depends_on Restart Behavior"
    Docker Compose's [`depends_on`](https://docs.docker.com/reference/compose-file/services/#depends_on){target="_blank" rel="noopener noreferrer"} has a `restart` attribute in the long-form syntax:

    ```yaml
    services:
      web:
        depends_on:
          database:
            condition: service_healthy
            restart: true
    ```

    Watchtower does not support using this to control implicit restarts, because this is an explicit, opt-in feature that defaults to `false` when omitted, such as when using the short-form syntax.
<!-- markdownlint-enable MD046 -->

#### Docker Links and Network Mode

For legacy Docker setups using links or `network_mode: service:container`, Watchtower treats these as implicit dependencies:

```dockerfile title="Legacy Docker linking"
# Container with explicit link
docker run --link database:db nginx

# Container using service network mode
docker run --network container:database nginx
```

### Topological Sorting

Watchtower uses topological sorting to determine the correct update order. This algorithm:

- Builds a dependency graph from all detected relationships
- Detects cycles and sets aside every container in one
- Produces a linear ordering where dependencies precede dependents

!!! Warning
    Containers in a circular dependency are skipped and reported as skipped, while the rest of the update continues.
    Ensure your dependency graph is acyclic so those containers are updated.

### Update Order

When updates are needed, Watchtower follows this sequence:

1. **Identify all containers requiring updates**
2. **Expand the set** to include all containers in the dependency chain
3. **Sort containers** using topological order (dependencies first)
4. **Stop containers** in reverse topological order (dependents first)
5. **Update and restart containers** in topological order (dependencies first)

This ensures that:

- Dependent services are stopped before their dependencies change
- Dependencies are fully restarted before dependents attempt to connect

### Restarting Dependent Containers

When a container is updated, Watchtower also restarts the containers that depend on it, directly or through other restarted containers, without updating their images.
Dependents are found with the same dependency graph that orders the update, so the containers restarted and the update order always agree.

- A Watchtower container restarts when a container it depends on restarts, such as a VPN container it shares a network with.
- A container that names a Watchtower container exactly, by container name or ID, restarts when that Watchtower container is updated. The name without its replica number (such as `infra-watchtower` for `infra-watchtower-1`) also works when exactly one Watchtower container has it, so it names nothing when the service runs several replicas.
- Containers in a circular dependency are neither updated nor restarted.

### Dependencies on Unmonitored Containers

A container that Watchtower does not monitor is left alone, including when a monitored container depends on it.
Containers are excluded from monitoring by the [container selection](../../configuration/container-selection/index.md) options, such as the enable label, the disable lists, image filters, and scopes.

- A dependency on an unmonitored container is ignored when ordering updates and restarting dependents.
- A dependency that names an unmonitored container exactly never resolves to a different, monitored container with a similar name.
- A service name whose service in the container's own Compose project is unmonitored never resolves to the same service in another project.

## Configuration

### Automatic Detection

In most cases, no additional configuration is required. Watchtower automatically detects dependencies from:

- Docker Compose `depends_on` declarations
- Existing Docker links
- `network_mode: service:container` configurations

!!! Note "Docker Compose Considerations"
    When using Docker Compose, Watchtower leverages `depends_on` declarations for dependency detection. Dependencies are resolved using service names, not container names. Ensure your `depends_on` references service names correctly.

!!! Warning "Rolling restart is currently not supported when used in combination with linked-containers."
    This limitation exists because linked-containers require coordinated updates across dependency chains, which conflicts with the incremental nature of rolling restarts.

### Disable Docker Compose Depends-On

If you want to disable automatic dependency detection from the Docker Compose `depends_on` configuration while preserving other dependency sources, use the following configuration:

<!-- markdownlint-disable MD046 -->
=== "Docker Compose"
    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower
        environment:
          - WATCHTOWER_USE_COMPOSE_DEPENDS_ON=false
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --use-compose-depends-on=false
    ```
<!-- markdownlint-enable MD046 -->

This disables parsing of the `com.docker.compose.depends_on` label while still honoring:

- Watchtower's explicit `com.centurylinklabs.watchtower.depends-on` label
- Legacy Docker links
- Network mode dependencies (`network_mode: service:container`)

### Explicit Dependencies

For cases where automatic detection is insufficient, use the Watchtower depends-on label:

<!-- markdownlint-disable MD046 -->
=== "Dockerfile"
    ```dockerfile
    FROM nginx:latest

    # Declare dependencies on database and cache services
    LABEL com.centurylinklabs.watchtower.depends-on="postgres,redis"
    ```
=== "Docker Compose"
    ```yaml
    services:
      web:
        image: nginx
        labels:
          - com.centurylinklabs.watchtower.depends-on=postgres,redis
      postgres:
        image: postgres
      redis:
        image: redis
    ```
<!-- markdownlint-enable MD046 -->

### Advanced Scenarios

#### Multiple Dependencies

```dockerfile title="Multiple dependencies"
LABEL com.centurylinklabs.watchtower.depends-on="database,cache,queue"
```

#### Complex Dependency Chains

```yaml title="Complex dependency chain"
services:
  web:
    image: nginx
    depends_on:
      - api
  api:
    image: myapi
    depends_on:
      - database
      - cache
  database:
    image: postgres
  cache:
    image: redis
```

In this scenario, Watchtower will update containers in the order: `cache`, `database`, `api`, `web`.

## Examples

### MySQL-WordPress Scenario

Consider a classic WordPress setup with MySQL database:

<!-- markdownlint-disable MD046 -->
=== "Docker Compose"
    ```yaml
    services:
      wordpress:
        image: wordpress:latest
        depends_on:
          - mysql
        environment:
          WORDPRESS_DB_HOST: mysql
      mysql:
        image: mysql:8.0
        environment:
          MYSQL_ROOT_PASSWORD: example
    ```
=== "Update Process"
    When Watchtower detects a MySQL update:

    1. **Dependency Detection**: Identifies that `wordpress` depends on `mysql`
    2. **Stop Order**: Stops `wordpress` first, then `mysql`
    3. **Update Order**: Updates and restarts `mysql` first, then `wordpress`

    This prevents WordPress from losing database connectivity during the update.
<!-- markdownlint-enable MD046 -->

### Microservices Architecture

For complex applications with multiple services:

<!-- markdownlint-disable MD046 -->
=== "Docker Compose"

    ```yaml
    services:
      api-gateway:
        image: nginx
        depends_on:
          - auth-service
          - user-service
      auth-service:
        image: auth-service
        depends_on:
          - redis
      user-service:
        image: user-service
        depends_on:
          - postgres
      redis:
        image: redis
      postgres:
        image: postgres
    ```

=== "Dependency Graph"

    ```
    postgres → user-service
    redis → auth-service
    auth-service → api-gateway
    user-service → api-gateway
    ```

    Update order: `postgres`, `redis`, `user-service`, `auth-service`, `api-gateway`
<!-- markdownlint-enable MD046 -->

### Legacy Docker Links

For applications using traditional Docker linking:

<!-- markdownlint-disable MD046 -->
=== "Docker Run Commands"

    ```bash
    # Start database
    docker run -d --name mysql mysql:8.0

    # Start web app with link
    docker run -d --name webapp --link mysql:db nginx
    ```

=== "Watchtower Behavior"

    Watchtower automatically detects the link and ensures `webapp` is stopped before `mysql` updates, and `mysql` restarts before `webapp`.

### Network Mode Dependencies

Containers using `network_mode: service:container`:

=== "Docker Compose"

    ```yaml
    services:
      sidecar:
        image: sidecar
        network_mode: service:main-app
      main-app:
        image: main-app
    ```

=== "Watchtower Behavior"

    The `sidecar` container is treated as dependent on `main-app`, ensuring proper update sequencing.
<!-- markdownlint-enable MD046 -->

## Troubleshooting

### Common Issues

#### Containers Skipped Due to Circular Dependencies

!!! Warning
    If you see "Skipping container update (circular dependency)" warnings, check your dependency declarations for cycles.

**Solution**: Review and remove circular dependencies. For example, if A depends on B and B depends on A, remove one of the dependencies or restructure your services.

#### Ignored Depends-On Entries

!!! Warning
    If you see "Ignoring depends-on entry that names no container" warnings, the named `com.centurylinklabs.watchtower.depends-on` entry matched no container.

**Solution**: Use the exact container name, a Compose service name, or a project-qualified service name such as `myproject-db`.

#### Containers Not Updating in Expected Order

**Check**:

- Verify dependency labels are correctly formatted
- Ensure container names match exactly
- Check Docker Compose service names vs container names

#### Missing Dependencies

**Symptoms**: Containers update out of order or fail to connect after updates.

**Debug**: Enable debug logging to see detected dependencies:

```bash
watchtower --debug
```

Look for log messages like:

- "Retrieved links from watchtower depends-on label"
- "Retrieved links from compose depends-on label"
- "Completed dependency sort"

### Debugging Commands

Enable verbose logging to inspect dependency detection:

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  nickfedor/watchtower \
  --debug \
  --run-once
```

Check container labels:

```bash
docker inspect container_name | jq '.[0].Config.Labels'
```
