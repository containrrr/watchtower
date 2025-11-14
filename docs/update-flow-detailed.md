# Watchtower — Detailed Update Flow & Data Shapes

This file provides a precise, developer-oriented mapping of the update call chain and full data-shape details with file references to help maintenance and debugging.

Note: file paths are relative to the repository root.

## Entry points

- `main()` — `main.go`
  - Sets default log level and calls `cmd.Execute()`.

- `cmd.Execute()` / Cobra root command — `cmd/root.go`
  - `PreRun` configures flags, creates `container.Client`, sets registry flags (`registry.InsecureSkipVerify`, `registry.RegistryCABundle`) and may validate CA bundle.
  - `runUpdatesWithNotifications` constructs `types.UpdateParams` and calls `internal/actions.Update`.

## Primary orchestration

- `internal/actions.Update(client container.Client, params types.UpdateParams) (types.Report, error)` — `internal/actions/update.go`
  - High level steps:
    1. Optional pre-checks: `pkg/lifecycle.ExecutePreChecks(client, params)` if `params.LifecycleHooks`.
    2. Container discovery: `client.ListContainers(params.Filter)` (wrapper in `pkg/container/client.go`).
    3. For each container:
       - `client.IsContainerStale(container, params)` — defined in `pkg/container/client.go`.
         - Pull logic: `client.PullImage(ctx, container)` (may skip via `container.IsNoPull(params)`).
         - Digest optimization: `pkg/registry/digest.CompareDigest(container, registryAuth)`.
           - Token flow: `pkg/registry/auth.GetToken` → `GetBearerHeader` → `GetAuthURL`.
           - Token cache: see `pkg/registry/auth/auth.go` (`getCachedToken`, `storeToken`).
           - HEAD request: `pkg/registry/digest.GetDigest` constructs `http.Client` with `digest.newTransport()`.
       - `client.HasNewImage(ctx, container)` compares local and remote image IDs.
       - `container.VerifyConfiguration()` to ensure image/container metadata is sufficient to recreate the container.
       - Mark progress via `session.Progress` (`AddScanned`, `AddSkipped`), call `containers[i].SetStale(stale)`.
    4. Sort by dependencies: `sorter.SortByDependencies(containers)`.
    5. `UpdateImplicitRestart(containers)` sets `LinkedToRestarting` flags for dependent containers.
    6. Build `containersToUpdate` (non-monitor-only) and mark for update in `Progress`.
    7. Update execution:
       - Rolling restart (`params.RollingRestart`): `performRollingRestart` stops and restarts each marked container in reverse order.
       - Normal: `stopContainersInReversedOrder` then `restartContainersInSortedOrder`.
         - Stop: `stopStaleContainer` optionally runs `lifecycle.ExecutePreUpdateCommand` and `client.StopContainer`.
         - Restart: `restartStaleContainer` may `client.RenameContainer` (if self), `client.StartContainer`, then `lifecycle.ExecutePostUpdateCommand`.
    8. Optional `cleanupImages(client, imageIDs)` when `params.Cleanup`.
    9. Optional post-checks: `pkg/lifecycle.ExecutePostChecks(client, params)`.
    10. Return `progress.Report()`.

## File-level locations (key functions)

- `internal/actions/update.go`
  - `Update`, `performRollingRestart`, `stopContainersInReversedOrder`, `stopStaleContainer`, `restartContainersInSortedOrder`, `restartStaleContainer`, `UpdateImplicitRestart`.

- `pkg/container/client.go`
  - `dockerClient.IsContainerStale`, `PullImage`, `HasNewImage`, `ListContainers`, `GetContainer`, `StopContainer`, `StartContainer`, `RenameContainer`, `RemoveImageByID`, `ExecuteCommand`.

- `pkg/container/container.go`
  - Concrete `Container` struct and implementation of `types.Container`.

- `pkg/registry/auth/auth.go`
  - `GetToken`, `GetBearerHeader`, token cache functions `getCachedToken` and `storeToken`.

- `pkg/registry/digest/digest.go`
  - `CompareDigest`, `GetDigest`, `newTransport` (transport respects `registry.InsecureSkipVerify` and `registry.GetRegistryCertPool()`), `NewTransportForTest`.

- `pkg/registry/registry.go`
  - `InsecureSkipVerify` (bool), `RegistryCABundle` (string), and `GetRegistryCertPool()`.

- `pkg/lifecycle/lifecycle.go`
  - `ExecutePreChecks`, `ExecutePostChecks`, `ExecutePreUpdateCommand`, `ExecutePostUpdateCommand`.

- `pkg/session/progress.go` and `pkg/session/container_status.go`
  - `Progress` (map) and `ContainerStatus` with fields and state enum.

## Data shapes — full details

Below are the main data shapes used in the update flow with fields and brief descriptions.

### types.UpdateParams (file: `pkg/types/update_params.go`)
```go
type UpdateParams struct {
    Filter          Filter       // Filter applied to container selection
    Cleanup         bool         // Whether to remove old images after update
    NoRestart       bool         // Skip restarting containers
    Timeout         time.Duration// Timeout used when stopping containers / exec
    MonitorOnly     bool         // Global monitor-only flag
    NoPull          bool         // Global no-pull flag
    LifecycleHooks  bool         // Enable lifecycle hook commands
    RollingRestart  bool         // Use rolling restart strategy
    LabelPrecedence bool         // Prefers container labels over CLI flags
}
```

### container.Client interface (file: `pkg/container/client.go`)
Methods (signatures):
- `ListContainers(Filter) ([]types.Container, error)` — discover containers
- `GetContainer(containerID types.ContainerID) (types.Container, error)` — inspect container
- `StopContainer(types.Container, time.Duration) error`
- `StartContainer(types.Container) (types.ContainerID, error)`
- `RenameContainer(types.Container, string) error`
- `IsContainerStale(types.Container, types.UpdateParams) (bool, types.ImageID, error)`
- `ExecuteCommand(containerID types.ContainerID, command string, timeout int) (SkipUpdate bool, err error)`
- `RemoveImageByID(types.ImageID) error`
- `WarnOnHeadPullFailed(types.Container) bool`

### types.Container interface (file: `pkg/types/container.go`)
Key methods used during update: (method signatures only)
- `ContainerInfo() *types.ContainerJSON`
- `ID() ContainerID`
- `IsRunning() bool`
- `Name() string`
- `ImageID() ImageID`
- `SafeImageID() ImageID`
- `ImageName() string`
- `Enabled() (bool, bool)`
- `IsMonitorOnly(UpdateParams) bool`
- `Scope() (string, bool)`
- `Links() []string`
- `ToRestart() bool`
- `IsWatchtower() bool`
- `StopSignal() string`
- `HasImageInfo() bool`
- `ImageInfo() *types.ImageInspect`
- `GetLifecyclePreCheckCommand() string`
- `GetLifecyclePostCheckCommand() string`
- `GetLifecyclePreUpdateCommand() string`
- `GetLifecyclePostUpdateCommand() string`
- `VerifyConfiguration() error`
- `SetStale(bool)` / `IsStale() bool`
- `IsNoPull(UpdateParams) bool`
- `SetLinkedToRestarting(bool)` / `IsLinkedToRestarting() bool`
- `PreUpdateTimeout() int` / `PostUpdateTimeout() int`
- `IsRestarting() bool`
- `GetCreateConfig() *dockercontainer.Config` / `GetCreateHostConfig() *dockercontainer.HostConfig`

Concrete `Container` fields (file: `pkg/container/container.go`):
- `LinkedToRestarting bool`
- `Stale bool`
- `containerInfo *types.ContainerJSON`
- `imageInfo *types.ImageInspect`

### session.ContainerStatus (file: `pkg/session/container_status.go`)
Fields:
- `containerID types.ContainerID`
- `oldImage types.ImageID`
- `newImage types.ImageID`
- `containerName string`
- `imageName string`
- `error` (embedded error)
- `state session.State` (enum: Skipped/Scanned/Updated/Failed/Fresh/Stale)

`session.Progress` is `map[types.ContainerID]*ContainerStatus` and exposes helper methods: `AddScanned`, `AddSkipped`, `MarkForUpdate`, `UpdateFailed`, and `Report()` which returns a `types.Report`.

### types.TokenResponse (used by `pkg/registry/auth`) — inferred fields
- `Token string`
- `ExpiresIn int` (seconds)

### Registry TLS configuration (file: `pkg/registry/registry.go`)
- `var InsecureSkipVerify bool` — when true, `digest.newTransport()` sets `tls.Config{InsecureSkipVerify: true}`
- `var RegistryCABundle string` — path to PEM bundle; `GetRegistryCertPool()` reads/merges it into system roots

### Token cache (file: `pkg/registry/auth/auth.go`)
Implementation details:
- `type cachedToken struct { token string; expiresAt time.Time }`
- `var tokenCache = map[string]cachedToken{}` protected by `tokenCacheMu *sync.Mutex`
- `var now = time.Now` (overridable in tests)
- `getCachedToken(key string) string` returns token if present and not expired (deletes expired entries)
- `storeToken(key, token string, ttl int)` stores token with TTL (seconds), ttl<=0 => no expiry
- Cache key: full auth URL string (realm+service+scope)

## Transport behavior for digest HEAD & token requests

- `pkg/registry/digest.newTransport()` builds a `*http.Transport` that:
  - Uses `http.ProxyFromEnvironment` and sane defaults for timeouts and connection pooling.
  - If `registry.InsecureSkipVerify` is true, sets `TLSClientConfig = &tls.Config{InsecureSkipVerify: true}`.
  - Else, if `registry.GetRegistryCertPool()` returns a non-nil pool, sets `TLSClientConfig = &tls.Config{RootCAs: pool}` (merges system roots + bundle).

## Edge cases and behavior notes

- If `container.VerifyConfiguration()` fails, container is marked skipped with the error logged and the update continues for other containers.
- If `lifecycle.ExecutePreUpdateCommand` returns `skipUpdate` (exit code 75), the container update is skipped.
- Watchtower self-update: the current watchtower container is renamed before starting the new container so the new container can reclaim the original name.
- Digest HEAD failures fall back to full `docker pull` and may log at `Warn` depending on `WarnOnHeadPullFailed`.
- Tokens are scoped per `repository:<path>:pull` — this prevents accidental reuse across repositories.

## How to use this doc

- Use the file references above to jump to implementations when changing behavior (e.g., token caching or TLS transport changes).
- For any change that affects pull/token behavior, update `pkg/registry/auth` tests and `pkg/registry/digest` tests, and run race-enabled tests.

If you want, I can also open a PR body (title + description + checklist) for you to paste into GitHub, or generate a patch file containing these new docs for you to push from your machine.
