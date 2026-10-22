# Image Cooldown

## Cooldown Delay

Sets a global minimum image age before Watchtower will perform the update.

Image age is determined from the image creation timestamp in the registry config blob.
This helps to establish a time buffer against newly pushed images; however, is not a comprehensive security control.

```text
            Argument: --cooldown-delay
Environment Variable: WATCHTOWER_COOLDOWN_DELAY
                Type: String
             Default: (empty / disabled)
```

- Accepted units: `h` (hours), `m` (minutes), `s` (seconds), `d` (days), `w` (weeks), `M` (months, i.e. 30 days).
- These can be combined (e.g., `24h`, `3d`, `1w`, `1M`, `1w12h`).
- Leaving the setting empty disables cooldown.

!!! Warning
    This setting delays *all* updates, including critical security patches.
    Operators should weigh the trade-off between update immediacy and exposure to unverified images.

!!! Note
    See [Image Cooldown](../../advanced-features/image-cooldown/index.md) for detailed information on how cooldown works, boundary behavior, per-container labels, limitations, and interaction with other features like `--no-pull`.

## Cooldown Platform OS

Overrides the OS used for platform selection when fetching image manifests during cooldown checks.
By default, Watchtower uses the runtime OS (e.g., `linux`).

```text
            Argument: None
Environment Variable: WATCHTOWER_COOLDOWN_PLATFORM_OS
                Type: String
             Default: runtime.GOOS
```

!!! Note
    Useful for cross-platform monitoring (e.g., monitoring Linux containers from a macOS or Windows host).
    Only affects the cooldown image age check; does not impact Docker's local platform detection.

    See [Image Cooldown](../../advanced-features/image-cooldown/index.md) for details on how platform selection works with multi-platform images.

## Cooldown Platform Architecture

Overrides the architecture used for platform selection when fetching image manifests during cooldown checks.
By default, Watchtower uses the runtime architecture (e.g., `amd64`, `arm64`).

```text
            Argument: None
Environment Variable: WATCHTOWER_COOLDOWN_PLATFORM_ARCH
                Type: String
             Default: runtime.GOARCH
```

!!! Note
    Useful for cross-platform monitoring (e.g., monitoring `arm64` containers from an `amd64` host).
    Only affects the cooldown image age check; does not impact Docker's local platform detection.

    See [Image Cooldown](../../advanced-features/image-cooldown/index.md) for details on how platform selection works with multi-platform images.

## Cooldown Platform Variant

Specifies the platform variant for platform selection when fetching image manifests during cooldown checks. This disambiguates when multiple image index entries share the same OS and architecture but differ by variant (e.g., ARM `v7` vs `v8`).

```text
            Argument: None
Environment Variable: WATCHTOWER_COOLDOWN_PLATFORM_VARIANT
                Type: String
             Default: None
```

!!! Note
    Required only for ARM images with multiple variants (e.g., `v7`, `v8`).
    When not specified and multiple variants exist, Watchtower returns an ambiguous platform match error.

    See [Image Cooldown](../../advanced-features/image-cooldown/index.md) for details on how platform selection works with multi-platform images.
