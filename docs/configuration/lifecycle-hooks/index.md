# Lifecycle Hooks

## Enable Lifecycle Hooks

Enables lifecycle hook execution.
Lifecycle hooks are disabled by default.

```text
            Argument: --enable-lifecycle-hooks
Environment Variable: WATCHTOWER_LIFECYCLE_HOOKS
                Type: Boolean
             Default: false
```

## Lifecycle UID

Sets the default user ID to run lifecycle hooks as when no container-specific UID is specified.

```text
            Argument: --lifecycle-uid
Environment Variable: WATCHTOWER_LIFECYCLE_UID
                Type: Integer
              Default: None
```

!!! Note
    Container-specific labels (`com.centurylinklabs.watchtower.lifecycle.uid`) take precedence over this global setting.

    See [Lifecycle Hooks](../../advanced-features/lifecycle-hooks/index.md).

## Lifecycle GID

Sets the default group ID to run lifecycle hooks as when no container-specific GID is specified.

```text
            Argument: --lifecycle-gid
Environment Variable: WATCHTOWER_LIFECYCLE_GID
                Type: Integer
              Default: None
```

!!! Note
    Container-specific labels (`com.centurylinklabs.watchtower.lifecycle.gid`) take precedence over this global setting.

    See [Lifecycle Hooks](../../advanced-features/lifecycle-hooks/index.md).
