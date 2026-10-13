<!-- markdownlint-disable MD024 -->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Remove the environment file when writing it fails by @nicholas-fedor in [#2790](https://github.com/nicholas-fedor/watchtower/pull/2790)
- Log the ephemeral stop after the orchestrator starts by @nicholas-fedor in [#2788](https://github.com/nicholas-fedor/watchtower/pull/2788)
- Warn once per dependency problem by @nicholas-fedor in [#2786](https://github.com/nicholas-fedor/watchtower/pull/2786)
- Keep environment notification settings with porcelain by @nicholas-fedor in [#2782](https://github.com/nicholas-fedor/watchtower/pull/2782)
- Detect podman container ids from mountinfo by @nicholas-fedor in [#2780](https://github.com/nicholas-fedor/watchtower/pull/2780)
- Mount the docker host socket for ephemeral updates by @nicholas-fedor in [#2776](https://github.com/nicholas-fedor/watchtower/pull/2776)
- Log failed container id detection at debug level by @nicholas-fedor in [#2766](https://github.com/nicholas-fedor/watchtower/pull/2766)
- Detect podman from the version components by @nicholas-fedor in [#2764](https://github.com/nicholas-fedor/watchtower/pull/2764)
- Detect podman wherever the runtime changes behavior by @nicholas-fedor in [#2752](https://github.com/nicholas-fedor/watchtower/pull/2752)
- Restart the old watchtower container after a failed ephemeral rename by @nicholas-fedor in [#2746](https://github.com/nicholas-fedor/watchtower/pull/2746)
- Resolve a service name shared by compose projects to each of them by @nicholas-fedor in [#2744](https://github.com/nicholas-fedor/watchtower/pull/2744)
- Retry token-bucket 429s with backoff from the first throttle by @nicholas-fedor in [#2733](https://github.com/nicholas-fedor/watchtower/pull/2733)
- Apply the legacy gotify tls skip verify option by @nicholas-fedor in [#2731](https://github.com/nicholas-fedor/watchtower/pull/2731)
- Keep legacy email encrypted when skipping tls verification by @nicholas-fedor in [#2728](https://github.com/nicholas-fedor/watchtower/pull/2728)
- Defer updates for images created in the future by @nicholas-fedor in [#2726](https://github.com/nicholas-fedor/watchtower/pull/2726)
- Limit the self-update pull failure delay to run-once mode by @nicholas-fedor in [#2721](https://github.com/nicholas-fedor/watchtower/pull/2721)
- Fail a manifest retry that asks for another host by @nicholas-fedor in [#2719](https://github.com/nicholas-fedor/watchtower/pull/2719)
- Disable self-updates when watchtower cannot identify its container by @nicholas-fedor in [#2717](https://github.com/nicholas-fedor/watchtower/pull/2717)
- Remove only watchtower containers named in the chain label by @nicholas-fedor in [#2715](https://github.com/nicholas-fedor/watchtower/pull/2715)
- Remove old instance images once per scan by @nicholas-fedor in [#2702](https://github.com/nicholas-fedor/watchtower/pull/2702)
- Replace inspect data instead of changing it in place by @nicholas-fedor in [#2691](https://github.com/nicholas-fedor/watchtower/pull/2691)
- Report deliberate skips as skipped instead of failed by @nicholas-fedor in [#2685](https://github.com/nicholas-fedor/watchtower/pull/2685)
- Derive implicit restarts from the dependency graph by @nicholas-fedor in [#2680](https://github.com/nicholas-fedor/watchtower/pull/2680)
- Detect dependency cycles on the graph used for sorting by @nicholas-fedor in [#2674](https://github.com/nicholas-fedor/watchtower/pull/2674)
- Keep in-flight orchestrators during startup cleanup by @nicholas-fedor in [#2669](https://github.com/nicholas-fedor/watchtower/pull/2669)
- Keep the partial report when an update fails by @nicholas-fedor in [#2663](https://github.com/nicholas-fedor/watchtower/pull/2663)
- Run post-check hooks in replacement containers by @nicholas-fedor in [#2659](https://github.com/nicholas-fedor/watchtower/pull/2659)
- Stop an old watchtower instance when an update detects it by @nicholas-fedor in [#2633](https://github.com/nicholas-fedor/watchtower/pull/2633)
- Apply the registry tls flags by @nicholas-fedor in [#2627](https://github.com/nicholas-fedor/watchtower/pull/2627)
- Fall back to the default template on parse errors by @nicholas-fedor in [#2625](https://github.com/nicholas-fedor/watchtower/pull/2625)
- Remove the notify-upgrade file on windows by @nicholas-fedor in [#2603](https://github.com/nicholas-fedor/watchtower/pull/2603)
- Match the report methods and json output of watchtower by @nicholas-fedor in [#2580](https://github.com/nicholas-fedor/watchtower/pull/2580)
- Preserve captured file ownership on inject #2546) by @rooty0 in [#2546](https://github.com/nicholas-fedor/watchtower/pull/2546)

### Documentation

- Report timed-out api requests as service unavailable by @nicholas-fedor in [#2757](https://github.com/nicholas-fedor/watchtower/pull/2757)
- Add rooty0 as a contributor for code, and test by @allcontributors[bot] in [#2554](https://github.com/nicholas-fedor/watchtower/pull/2554)

### Tests

- Set only the lowercase podman variable in the env test by @nicholas-fedor in [#2759](https://github.com/nicholas-fedor/watchtower/pull/2759)
- Add golden tests for notification batching by @nicholas-fedor in [#2618](https://github.com/nicholas-fedor/watchtower/pull/2618)
- Characterize preRun by @nicholas-fedor in [#2605](https://github.com/nicholas-fedor/watchtower/pull/2605)
- Characterize run and runMain by @nicholas-fedor in [#2601](https://github.com/nicholas-fedor/watchtower/pull/2601)
- Characterize the notify-upgrade command by @nicholas-fedor in [#2597](https://github.com/nicholas-fedor/watchtower/pull/2597)
- Characterize the self-update orchestrator by @nicholas-fedor in [#2595](https://github.com/nicholas-fedor/watchtower/pull/2595)
- Characterize image creation time lookups by @nicholas-fedor in [#2591](https://github.com/nicholas-fedor/watchtower/pull/2591)
- Characterize secret file loading by @nicholas-fedor in [#2586](https://github.com/nicholas-fedor/watchtower/pull/2586)
- Make cmd and metrics tests safe to shuffle and repeat by @nicholas-fedor in [#2584](https://github.com/nicholas-fedor/watchtower/pull/2584)
- Fix data races in the test suite by @nicholas-fedor in [#2582](https://github.com/nicholas-fedor/watchtower/pull/2582)
- Add golden tests for http api responses by @nicholas-fedor in [#2572](https://github.com/nicholas-fedor/watchtower/pull/2572)
- Add golden tests for notification template output by @nicholas-fedor in [#2556](https://github.com/nicholas-fedor/watchtower/pull/2556)
- Add golden tests for the http api surface by @nicholas-fedor in [#2545](https://github.com/nicholas-fedor/watchtower/pull/2545)
- Add golden tests for the cli configuration surface by @nicholas-fedor in [#2543](https://github.com/nicholas-fedor/watchtower/pull/2543)

### Build

- Pin mockery as a go tool by @nicholas-fedor in [#2610](https://github.com/nicholas-fedor/watchtower/pull/2610)
- Generate the preview copy of the notification templates by @nicholas-fedor in [#2578](https://github.com/nicholas-fedor/watchtower/pull/2578)

### Continuous Integration

- Make the indirect changes check informational by @nicholas-fedor in [#2713](https://github.com/nicholas-fedor/watchtower/pull/2713)
- Pass the release environment's docker hub secrets by @nicholas-fedor in [#2709](https://github.com/nicholas-fedor/watchtower/pull/2709)
- Run the race detector on every package by @nicholas-fedor in [#2696](https://github.com/nicholas-fedor/watchtower/pull/2696)
- Generate the changelog from the latest default branch by @nicholas-fedor in [#2657](https://github.com/nicholas-fedor/watchtower/pull/2657)
- Use the release environment's docker hub credentials by @nicholas-fedor in [#2647](https://github.com/nicholas-fedor/watchtower/pull/2647)
- Update the changelog automation by @nicholas-fedor in [#2638](https://github.com/nicholas-fedor/watchtower/pull/2638)
- Consolidate ci on github actions by @nicholas-fedor in [#2539](https://github.com/nicholas-fedor/watchtower/pull/2539)

### Chores

- Update github.com/azure/go-ansiterm digest to 51075c6 by @renovate[bot] in [#2784](https://github.com/nicholas-fedor/watchtower/pull/2784)
- Update module github.com/nicholas-fedor/shoutrrr to v0.22.0 by @renovate[bot] in [#2778](https://github.com/nicholas-fedor/watchtower/pull/2778)
- Update module github.com/stretchr/objx to v0.7.0 by @renovate[bot] in [#2774](https://github.com/nicholas-fedor/watchtower/pull/2774)
- Update google.golang.org/genproto/googleapis/rpc digest to 4341edf by @renovate[bot] in [#2771](https://github.com/nicholas-fedor/watchtower/pull/2771)
- Update nicholas-fedor/govulncheck-action action to v1.1.1 by @renovate[bot] in [#2772](https://github.com/nicholas-fedor/watchtower/pull/2772)
- Update google.golang.org/genproto/googleapis/api digest to 4341edf by @renovate[bot] in [#2769](https://github.com/nicholas-fedor/watchtower/pull/2769)
- Update github.com/google/pprof digest to 2af12c3 by @renovate[bot] in [#2768](https://github.com/nicholas-fedor/watchtower/pull/2768)
- Update goreleaser/goreleaser-action action to v7.3.0 by @renovate[bot] in [#2762](https://github.com/nicholas-fedor/watchtower/pull/2762)
- Update github.com/google/pprof digest to 4ba64fc by @renovate[bot] in [#2761](https://github.com/nicholas-fedor/watchtower/pull/2761)
- Update module github.com/klauspost/compress to v1.20.3 by @renovate[bot] in [#2754](https://github.com/nicholas-fedor/watchtower/pull/2754)
- Update module github.com/gofiber/fiber/v3 to v3.6.0 by @renovate[bot] in [#2755](https://github.com/nicholas-fedor/watchtower/pull/2755)
- Update module github.com/vektra/mockery/v3 to v3.8.1 by @renovate[bot] in [#2750](https://github.com/nicholas-fedor/watchtower/pull/2750)
- Update ossf/scorecard-action action to v2.5.0 by @renovate[bot] in [#2748](https://github.com/nicholas-fedor/watchtower/pull/2748)
- Update module github.com/prometheus/common to v0.72.1 by @renovate[bot] in [#2742](https://github.com/nicholas-fedor/watchtower/pull/2742)
- Update golang.org/x/exp digest to faecfb2 by @renovate[bot] in [#2741](https://github.com/nicholas-fedor/watchtower/pull/2741)
- Update module golang.org/x/tools to v0.52.0 by @renovate[bot] in [#2739](https://github.com/nicholas-fedor/watchtower/pull/2739)
- Update module github.com/molecule-man/go-brrr to v1.3.0 by @renovate[bot] in [#2738](https://github.com/nicholas-fedor/watchtower/pull/2738)
- Update dependency python to v3.15.0 by @renovate[bot] in [#2736](https://github.com/nicholas-fedor/watchtower/pull/2736)
- Update module github.com/fvbommel/sortorder to v1.2.1 by @renovate[bot] in [#2735](https://github.com/nicholas-fedor/watchtower/pull/2735)
- Update golang:alpine3.24 docker digest to f92b6ef by @renovate[bot] in [#2724](https://github.com/nicholas-fedor/watchtower/pull/2724)
- Update golang:1.27.2-alpine docker digest to f92b6ef by @renovate[bot] in [#2723](https://github.com/nicholas-fedor/watchtower/pull/2723)
- Skip major updates of indirect go dependencies by @nicholas-fedor in [#2711](https://github.com/nicholas-fedor/watchtower/pull/2711)
- Update module golang.org/x/text to v0.43.0 by @renovate[bot] in [#2705](https://github.com/nicholas-fedor/watchtower/pull/2705)
- Update module golang.org/x/mod to v0.42.0 by @renovate[bot] in [#2704](https://github.com/nicholas-fedor/watchtower/pull/2704)
- Update module github.com/onsi/ginkgo/v2 to v2.33.1 by @renovate[bot] in [#2700](https://github.com/nicholas-fedor/watchtower/pull/2700)
- Update golang.org/x/exp digest to ca0d7ba by @renovate[bot] in [#2699](https://github.com/nicholas-fedor/watchtower/pull/2699)
- Update module golang.org/x/net to v0.61.0 by @renovate[bot] in [#2694](https://github.com/nicholas-fedor/watchtower/pull/2694)
- Update module golang.org/x/crypto to v0.58.0 by @renovate[bot] in [#2693](https://github.com/nicholas-fedor/watchtower/pull/2693)
- Update module golang.org/x/term to v0.47.0 by @renovate[bot] in [#2688](https://github.com/nicholas-fedor/watchtower/pull/2688)
- Update module golang.org/x/sync to v0.24.0 by @renovate[bot] in [#2687](https://github.com/nicholas-fedor/watchtower/pull/2687)
- Update module github.com/docker/docker-credential-helpers to v0.9.10 by @renovate[bot] in [#2683](https://github.com/nicholas-fedor/watchtower/pull/2683)
- Update github.com/tonistiigi/fsutil digest to c1c5bff by @renovate[bot] in [#2682](https://github.com/nicholas-fedor/watchtower/pull/2682)
- Update module golang.org/x/sys to v0.49.0 by @renovate[bot] in [#2677](https://github.com/nicholas-fedor/watchtower/pull/2677)
- Update github.com/tonistiigi/fsutil digest to c60bd37 by @renovate[bot] in [#2676](https://github.com/nicholas-fedor/watchtower/pull/2676)
- Update nicholas-fedor/actionlint-action action to v1.0.19 by @renovate[bot] in [#2672](https://github.com/nicholas-fedor/watchtower/pull/2672)
- Update github.com/tonistiigi/fsutil digest to 0684b79 by @renovate[bot] in [#2671](https://github.com/nicholas-fedor/watchtower/pull/2671)
- Update nicholas-fedor/govulncheck-action action to v1.1.0 by @renovate[bot] in [#2667](https://github.com/nicholas-fedor/watchtower/pull/2667)
- Remove the codacy integration by @nicholas-fedor in [#2665](https://github.com/nicholas-fedor/watchtower/pull/2665)
- Update nicholas-fedor/govulncheck-action action to v1.0.6 by @renovate[bot] in [#2660](https://github.com/nicholas-fedor/watchtower/pull/2660)
- Update module github.com/knadh/koanf/providers/env to v2 by @renovate[bot] in [#2653](https://github.com/nicholas-fedor/watchtower/pull/2653)
- Update module github.com/brunoga/deep to v6 by @renovate[bot] in [#2652](https://github.com/nicholas-fedor/watchtower/pull/2652)
- Update module golang.org/x/net to v0.60.0 by @renovate[bot] in [#2649](https://github.com/nicholas-fedor/watchtower/pull/2649)
- Update module github.com/jedib0t/go-pretty/v6 to v6.8.3 by @renovate[bot] in [#2648](https://github.com/nicholas-fedor/watchtower/pull/2648)
- Update module github.com/microsoft/go-winio to v0.6.3 by @renovate[bot] in [#2644](https://github.com/nicholas-fedor/watchtower/pull/2644)
- Update module github.com/huandu/xstrings to v1.6.2 by @renovate[bot] in [#2645](https://github.com/nicholas-fedor/watchtower/pull/2645)
- Update module github.com/knadh/koanf/providers/structs to v1.0.1 by @renovate[bot] in [#2639](https://github.com/nicholas-fedor/watchtower/pull/2639)
- Update module github.com/knadh/koanf/v2 to v2.3.8 by @renovate[bot] in [#2640](https://github.com/nicholas-fedor/watchtower/pull/2640)
- Update module github.com/knadh/koanf/providers/posflag to v1.0.2 by @renovate[bot] in [#2635](https://github.com/nicholas-fedor/watchtower/pull/2635)
- Update github.com/azure/go-ansiterm digest to 65faa4b by @renovate[bot] in [#2634](https://github.com/nicholas-fedor/watchtower/pull/2634)
- Update module github.com/knadh/koanf/maps to v0.1.3 by @renovate[bot] in [#2629](https://github.com/nicholas-fedor/watchtower/pull/2629)
- Update module github.com/knadh/koanf/parsers/yaml to v1.1.1 by @renovate[bot] in [#2630](https://github.com/nicholas-fedor/watchtower/pull/2630)
- Update github.com/azure/go-ansiterm digest to e8a9660 by @renovate[bot] in [#2620](https://github.com/nicholas-fedor/watchtower/pull/2620)
- Update golang.org/x/exp digest to f45ad48 by @renovate[bot] in [#2621](https://github.com/nicholas-fedor/watchtower/pull/2621)
- Enable gocyclo with a complexity limit of 25 by @nicholas-fedor in [#2616](https://github.com/nicholas-fedor/watchtower/pull/2616)
- Update module github.com/moby/moby/client to v0.6.2 by @renovate[bot] in [#2611](https://github.com/nicholas-fedor/watchtower/pull/2611)
- Update module github.com/docker/cli to v29.9.0+incompatible by @renovate[bot] in [#2612](https://github.com/nicholas-fedor/watchtower/pull/2612)
- Update golang docker tag to v1.27.2 by @renovate[bot] in [#2607](https://github.com/nicholas-fedor/watchtower/pull/2607)
- Update golang:alpine3.24 docker digest to 738d1cf by @renovate[bot] in [#2606](https://github.com/nicholas-fedor/watchtower/pull/2606)
- Update go module directive to v1.27.2 by @renovate[bot] in [#2599](https://github.com/nicholas-fedor/watchtower/pull/2599)
- Update module github.com/docker/go-connections to v0.8.2 by @renovate[bot] in [#2593](https://github.com/nicholas-fedor/watchtower/pull/2593)
- Update module github.com/prometheus/client_golang to v1.25.0 by @renovate[bot] in [#2589](https://github.com/nicholas-fedor/watchtower/pull/2589)
- Update github/codeql-action action to v4.38.3 by @renovate[bot] in [#2588](https://github.com/nicholas-fedor/watchtower/pull/2588)
- Update github.com/xrash/smetrics digest to 55b8f29 by @renovate[bot] in [#2574](https://github.com/nicholas-fedor/watchtower/pull/2574)
- Update module github.com/mattn/go-runewidth to v0.0.31 by @renovate[bot] in [#2575](https://github.com/nicholas-fedor/watchtower/pull/2575)
- Update module github.com/urfave/cli/v2 to v2.27.7 by @renovate[bot] in [#2562](https://github.com/nicholas-fedor/watchtower/pull/2562)
- Generate the swag check copy outside the module by @nicholas-fedor in [#2560](https://github.com/nicholas-fedor/watchtower/pull/2560)
- Update step-security/harden-runner action to v2.22.1 by @renovate[bot] in [#2568](https://github.com/nicholas-fedor/watchtower/pull/2568)
- Update github.com/google/pprof digest to 7bae8d8 by @renovate[bot] in [#2567](https://github.com/nicholas-fedor/watchtower/pull/2567)
- Update module github.com/moby/buildkit to v0.34.0 by @renovate[bot] in [#2565](https://github.com/nicholas-fedor/watchtower/pull/2565)
- Update actions/upload-artifact action to v7.0.2 by @renovate[bot] in [#2561](https://github.com/nicholas-fedor/watchtower/pull/2561)
- Restore urfave/cli v2 for swag by @nicholas-fedor in [#2557](https://github.com/nicholas-fedor/watchtower/pull/2557)
- Update module github.com/urfave/cli/v2 to v3 by @renovate[bot] in [#2549](https://github.com/nicholas-fedor/watchtower/pull/2549)
- Update github.com/xrash/smetrics digest to 55b8f29 by @renovate[bot] in [#2551](https://github.com/nicholas-fedor/watchtower/pull/2551)
- Update module github.com/urfave/cli/v2 to v2.27.7 by @renovate[bot] in [#2548](https://github.com/nicholas-fedor/watchtower/pull/2548)
- Resolve existing lint findings by @nicholas-fedor in [#2541](https://github.com/nicholas-fedor/watchtower/pull/2541)
- Migrate makefile to taskfile by @nicholas-fedor in [#2535](https://github.com/nicholas-fedor/watchtower/pull/2535)
- Update github.com/azure/go-ansiterm digest to 8c912ac by @renovate[bot] in [#2536](https://github.com/nicholas-fedor/watchtower/pull/2536)

### New Contributors

- @rooty0 made their first contribution in [#2546](https://github.com/nicholas-fedor/watchtower/pull/2546)

## [1.23.0] - 2026-10-06

### Added

- Expose OCI image versions and resolve versioned changelog links by @nicholas-fedor in [#2474](https://github.com/nicholas-fedor/watchtower/pull/2474)
- Add Git ref monitoring and container rebuilds by @nicholas-fedor in [#2456](https://github.com/nicholas-fedor/watchtower/pull/2456)

### Fixed

- Copy labeled files under a mount and into missing parent paths by @nicholas-fedor in [#2486](https://github.com/nicholas-fedor/watchtower/pull/2486)

### Continuous Integration

- Fix changelog automation workflows by @nicholas-fedor in [#2451](https://github.com/nicholas-fedor/watchtower/pull/2451)

### Chores

- Update github.com/google/pprof digest to d99a617 by @renovate[bot] in [#2533](https://github.com/nicholas-fedor/watchtower/pull/2533)
- Update module github.com/nicholas-fedor/shoutrrr to v0.21.2 by @renovate[bot] in [#2530](https://github.com/nicholas-fedor/watchtower/pull/2530)
- Update module github.com/mattn/go-colorable to v0.1.16 by @renovate[bot] in [#2529](https://github.com/nicholas-fedor/watchtower/pull/2529)
- Update step-security/harden-runner action to v2.22.0 by @renovate[bot] in [#2527](https://github.com/nicholas-fedor/watchtower/pull/2527)
- Update google.golang.org/genproto/googleapis/rpc digest to fad4113 by @renovate[bot] in [#2523](https://github.com/nicholas-fedor/watchtower/pull/2523)
- Update google.golang.org/genproto/googleapis/api digest to fad4113 by @renovate[bot] in [#2522](https://github.com/nicholas-fedor/watchtower/pull/2522)
- Update github.com/microsoft/go-winio digest to 2430248 by @renovate[bot] in [#2520](https://github.com/nicholas-fedor/watchtower/pull/2520)
- Update github.com/google/pprof digest to 639476b by @renovate[bot] in [#2519](https://github.com/nicholas-fedor/watchtower/pull/2519)
- Update module github.com/valyala/fasthttp to v1.75.0 by @renovate[bot] in [#2517](https://github.com/nicholas-fedor/watchtower/pull/2517)
- Update module github.com/go-git/go-git/v5 to v5.19.3 by @renovate[bot] in [#2516](https://github.com/nicholas-fedor/watchtower/pull/2516)
- Update module github.com/molecule-man/go-brrr to v1.2.0 by @renovate[bot] in [#2513](https://github.com/nicholas-fedor/watchtower/pull/2513)
- Update github.com/google/pprof digest to ebaad5f by @renovate[bot] in [#2512](https://github.com/nicholas-fedor/watchtower/pull/2512)
- Update opentelemetry-go-contrib monorepo to v0.72.0 by @renovate[bot] in [#2511](https://github.com/nicholas-fedor/watchtower/pull/2511)
- Update module golang.org/x/tools to v0.51.0 by @renovate[bot] in [#2509](https://github.com/nicholas-fedor/watchtower/pull/2509)
- Update module github.com/docker/compose/v5 to v5.6.0 by @renovate[bot] in [#2508](https://github.com/nicholas-fedor/watchtower/pull/2508)
- Update module github.com/go-git/go-billy/v5 to v5.9.2 by @renovate[bot] in [#2505](https://github.com/nicholas-fedor/watchtower/pull/2505)
- Update anchore/sbom-action action to v0.24.3 by @renovate[bot] in [#2504](https://github.com/nicholas-fedor/watchtower/pull/2504)
- Update opentelemetry-go monorepo to v1.47.0 by @renovate[bot] in [#2501](https://github.com/nicholas-fedor/watchtower/pull/2501)
- Update module github.com/moby/moby/client to v0.6.1 by @renovate[bot] in [#2500](https://github.com/nicholas-fedor/watchtower/pull/2500)
- Update module github.com/moby/moby/api to v1.56.1 by @renovate[bot] in [#2497](https://github.com/nicholas-fedor/watchtower/pull/2497)
- Update github.com/google/pprof digest to 77d3b59 by @renovate[bot] in [#2496](https://github.com/nicholas-fedor/watchtower/pull/2496)
- Update module github.com/mattn/go-shellwords to v1.0.16 by @renovate[bot] in [#2495](https://github.com/nicholas-fedor/watchtower/pull/2495)
- Update module github.com/gofiber/contrib/v3/zerolog to v1.1.6 by @renovate[bot] in [#2494](https://github.com/nicholas-fedor/watchtower/pull/2494)
- Update module github.com/gofiber/contrib/v3/swaggo to v1.0.12 by @renovate[bot] in [#2491](https://github.com/nicholas-fedor/watchtower/pull/2491)
- Update dependency python to v3.14.8 by @renovate[bot] in [#2490](https://github.com/nicholas-fedor/watchtower/pull/2490)
- Update github.com/google/pprof digest to 60bf690 by @renovate[bot] in [#2487](https://github.com/nicholas-fedor/watchtower/pull/2487)
- Update github.com/microsoft/go-winio digest to f19d971 by @renovate[bot] in [#2488](https://github.com/nicholas-fedor/watchtower/pull/2488)
- Update module github.com/pjbgf/sha1cd to v0.7.0 by @renovate[bot] in [#2482](https://github.com/nicholas-fedor/watchtower/pull/2482)
- Update module go.opentelemetry.io/proto/otlp to v1.11.1 by @renovate[bot] in [#2481](https://github.com/nicholas-fedor/watchtower/pull/2481)
- Update module github.com/tinylib/msgp to v1.6.5 by @renovate[bot] in [#2480](https://github.com/nicholas-fedor/watchtower/pull/2480)
- Update module github.com/cenkalti/backoff/v7 to v7.0.1 by @renovate[bot] in [#2476](https://github.com/nicholas-fedor/watchtower/pull/2476)
- Update module github.com/moby/buildkit to v0.33.1 by @renovate[bot] in [#2477](https://github.com/nicholas-fedor/watchtower/pull/2477)
- Update module github.com/go-git/gcfg to v2 by @renovate[bot] in [#2468](https://github.com/nicholas-fedor/watchtower/pull/2468)
- Update docker/dockerfile:1 docker digest to 4edf897 by @renovate[bot] in [#2470](https://github.com/nicholas-fedor/watchtower/pull/2470)
- Update module github.com/docker/cli to v29.8.2+incompatible by @renovate[bot] in [#2471](https://github.com/nicholas-fedor/watchtower/pull/2471)
- Update module github.com/go-git/gcfg to v2 by @renovate[bot] in [#2464](https://github.com/nicholas-fedor/watchtower/pull/2464)
- Revert compose-go to v2.15.0 by @nicholas-fedor in [#2465](https://github.com/nicholas-fedor/watchtower/pull/2465)
- Update module github.com/compose-spec/compose-go/v2 to v2.16.1 by @renovate[bot] in [#2462](https://github.com/nicholas-fedor/watchtower/pull/2462)
- Update github.com/planetscale/vtprotobuf digest to 8ae5a48 by @renovate[bot] in [#2459](https://github.com/nicholas-fedor/watchtower/pull/2459)
- Update github.com/microsoft/go-winio digest to 7e8af9b by @renovate[bot] in [#2458](https://github.com/nicholas-fedor/watchtower/pull/2458)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.52 by @renovate[bot] in [#2454](https://github.com/nicholas-fedor/watchtower/pull/2454)
- Update module github.com/prometheus/common to v0.72.0 by @renovate[bot] in [#2452](https://github.com/nicholas-fedor/watchtower/pull/2452)
- Update github.com/google/pprof digest to aaccee0 by @renovate[bot] in [#2449](https://github.com/nicholas-fedor/watchtower/pull/2449)
- Update module github.com/go-openapi/jsonreference to v1.0.3 by @renovate[bot] in [#2447](https://github.com/nicholas-fedor/watchtower/pull/2447)
- Update module github.com/klauspost/compress to v1.20.1 by @renovate[bot] in [#2446](https://github.com/nicholas-fedor/watchtower/pull/2446)
- Update module github.com/onsi/gomega to v1.44.0 by @renovate[bot] in [#2443](https://github.com/nicholas-fedor/watchtower/pull/2443)
- Update module github.com/go-openapi/jsonpointer to v1.0.2 by @renovate[bot] in [#2442](https://github.com/nicholas-fedor/watchtower/pull/2442)
- Update module github.com/gofiber/schema to v1.8.8 by @renovate[bot] in [#2441](https://github.com/nicholas-fedor/watchtower/pull/2441)
- Update github/codeql-action action to v4.38.2 by @renovate[bot] in [#2440](https://github.com/nicholas-fedor/watchtower/pull/2440)
- Update module github.com/gofiber/utils/v2 to v2.6.0 by @renovate[bot] in [#2438](https://github.com/nicholas-fedor/watchtower/pull/2438)

## [1.22.3] - 2026-09-22

### Added

- Copy labeled files across container recreate by @nicholas-fedor in [#2387](https://github.com/nicholas-fedor/watchtower/pull/2387)

### Fixed

- Keep container config when image Config is nil by @nicholas-fedor in [#2432](https://github.com/nicholas-fedor/watchtower/pull/2432)
- Share authenticated lscr.io rate limit with ghcr.io by @nicholas-fedor in [#2430](https://github.com/nicholas-fedor/watchtower/pull/2430)
- Apply registry mirrors only to Docker Hub by @nicholas-fedor in [#2428](https://github.com/nicholas-fedor/watchtower/pull/2428)
- Rewrite volumes-from IDs before recreate by @nicholas-fedor in [#2424](https://github.com/nicholas-fedor/watchtower/pull/2424)
- Match engine-generated MACs as inspect HardwareAddr by @nicholas-fedor in [#2418](https://github.com/nicholas-fedor/watchtower/pull/2418)

### Build

- Use the commit date for archive mtime by @nicholas-fedor in [#2435](https://github.com/nicholas-fedor/watchtower/pull/2435)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.21.1 by @renovate[bot] in [#2437](https://github.com/nicholas-fedor/watchtower/pull/2437)
- Update module github.com/gofiber/utils/v2 to v2.5.3 by @renovate[bot] in [#2426](https://github.com/nicholas-fedor/watchtower/pull/2426)
- Update golang:alpine3.24 docker digest to 8a5910f by @renovate[bot] in [#2421](https://github.com/nicholas-fedor/watchtower/pull/2421)
- Update golang:1.27.1-alpine docker digest to 8a5910f by @renovate[bot] in [#2420](https://github.com/nicholas-fedor/watchtower/pull/2420)
- Update orhun/git-cliff-action action to v4.9.1 by @renovate[bot] in [#2414](https://github.com/nicholas-fedor/watchtower/pull/2414)
- Update module github.com/molecule-man/go-brrr to v1.1.1 by @renovate[bot] in [#2413](https://github.com/nicholas-fedor/watchtower/pull/2413)
- Update github/codeql-action action to v4.38.1 by @renovate[bot] in [#2411](https://github.com/nicholas-fedor/watchtower/pull/2411)
- Update alpine docker tag to v3.24.2 by @renovate[bot] in [#2410](https://github.com/nicholas-fedor/watchtower/pull/2410)
- Update golang:alpine3.24 docker digest to 4cb7ac9 by @renovate[bot] in [#2407](https://github.com/nicholas-fedor/watchtower/pull/2407)
- Update golang:1.27.1-alpine docker digest to 4cb7ac9 by @renovate[bot] in [#2406](https://github.com/nicholas-fedor/watchtower/pull/2406)
- Update golang:alpine3.24 docker digest to e9bbdf2 by @renovate[bot] in [#2401](https://github.com/nicholas-fedor/watchtower/pull/2401)
- Update golang:1.27.1-alpine docker digest to e9bbdf2 by @renovate[bot] in [#2400](https://github.com/nicholas-fedor/watchtower/pull/2400)
- Update module github.com/onsi/ginkgo/v2 to v2.33.0 by @renovate[bot] in [#2398](https://github.com/nicholas-fedor/watchtower/pull/2398)
- Update module github.com/onsi/gomega to v1.43.1 by @renovate[bot] in [#2397](https://github.com/nicholas-fedor/watchtower/pull/2397)
- Update codecov/codecov-action action to v7.1.1 by @renovate[bot] in [#2396](https://github.com/nicholas-fedor/watchtower/pull/2396)
- Update docker/setup-buildx-action action to v4.4.1 by @renovate[bot] in [#2394](https://github.com/nicholas-fedor/watchtower/pull/2394)
- Update module github.com/docker/cli to v29.8.1+incompatible by @renovate[bot] in [#2391](https://github.com/nicholas-fedor/watchtower/pull/2391)
- Update module github.com/gofiber/schema to v1.8.7 by @renovate[bot] in [#2388](https://github.com/nicholas-fedor/watchtower/pull/2388)

## [1.22.2] - 2026-09-15

### Fixed

- Reuse anonymous GHCR tokens across public images by @nicholas-fedor in [#2366](https://github.com/nicholas-fedor/watchtower/pull/2366)

### Changed

- Replace SetNoRestartPolicy with SetRestartPolicy by @nicholas-fedor in [#2385](https://github.com/nicholas-fedor/watchtower/pull/2385)
- Replace untyped maps with concrete types by @nicholas-fedor in [#2371](https://github.com/nicholas-fedor/watchtower/pull/2371)

### Documentation

- Add NotaInutilis as a contributor for doc by @allcontributors[bot] in [#2374](https://github.com/nicholas-fedor/watchtower/pull/2374)
- Improve docker connection and socket proxy info by @NotaInutilis in [#2352](https://github.com/nicholas-fedor/watchtower/pull/2352)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.21.0 by @renovate[bot] in [#2384](https://github.com/nicholas-fedor/watchtower/pull/2384)
- Update docker/setup-qemu-action action to v4.4.0 by @renovate[bot] in [#2382](https://github.com/nicholas-fedor/watchtower/pull/2382)
- Update docker/setup-buildx-action action to v4.4.0 by @renovate[bot] in [#2381](https://github.com/nicholas-fedor/watchtower/pull/2381)
- Update codecov/codecov-action action to v7.1.0 by @renovate[bot] in [#2379](https://github.com/nicholas-fedor/watchtower/pull/2379)
- Update module github.com/gofiber/utils/v2 to v2.5.2 by @renovate[bot] in [#2376](https://github.com/nicholas-fedor/watchtower/pull/2376)
- Update module github.com/gofiber/contrib/v3/zerolog to v1.1.5 by @renovate[bot] in [#2369](https://github.com/nicholas-fedor/watchtower/pull/2369)
- Update module github.com/gofiber/contrib/v3/swaggo to v1.0.11 by @renovate[bot] in [#2368](https://github.com/nicholas-fedor/watchtower/pull/2368)
- Update module github.com/molecule-man/go-brrr to v1.1.0 by @renovate[bot] in [#2364](https://github.com/nicholas-fedor/watchtower/pull/2364)
- Update github/codeql-action action to v4.38.0 by @renovate[bot] in [#2362](https://github.com/nicholas-fedor/watchtower/pull/2362)
- Update module golang.org/x/tools to v0.50.0 by @renovate[bot] in [#2360](https://github.com/nicholas-fedor/watchtower/pull/2360)
- Update module github.com/onsi/ginkgo/v2 to v2.32.2 by @renovate[bot] in [#2359](https://github.com/nicholas-fedor/watchtower/pull/2359)

### New Contributors

- @NotaInutilis made their first contribution in [#2352](https://github.com/nicholas-fedor/watchtower/pull/2352)

## [1.22.1] - 2026-09-09

### Fixed

- Match network_mode links across Compose projects by @shawly in [#2319](https://github.com/nicholas-fedor/watchtower/pull/2319)
- Stop skipping GHCR images on sub-millisecond 429s by @nicholas-fedor in [#2327](https://github.com/nicholas-fedor/watchtower/pull/2327)

### Documentation

- Correct Gmail SMTP authentication guidance by @nicholas-fedor in [#2342](https://github.com/nicholas-fedor/watchtower/pull/2342)
- Add star history by @nicholas-fedor in [#2338](https://github.com/nicholas-fedor/watchtower/pull/2338)
- Add shawly as a contributor for code by @allcontributors[bot] in [#2331](https://github.com/nicholas-fedor/watchtower/pull/2331)

### Chores

- Update module golang.org/x/text to v0.42.0 by @renovate[bot] in [#2356](https://github.com/nicholas-fedor/watchtower/pull/2356)
- Update module golang.org/x/net to v0.59.0 by @renovate[bot] in [#2355](https://github.com/nicholas-fedor/watchtower/pull/2355)
- Update module golang.org/x/crypto to v0.57.0 by @renovate[bot] in [#2353](https://github.com/nicholas-fedor/watchtower/pull/2353)
- Update module github.com/nicholas-fedor/shoutrrr to v0.20.0 by @renovate[bot] in [#2349](https://github.com/nicholas-fedor/watchtower/pull/2349)
- Update module github.com/gofiber/schema to v1.8.6 by @renovate[bot] in [#2348](https://github.com/nicholas-fedor/watchtower/pull/2348)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.51 by @renovate[bot] in [#2346](https://github.com/nicholas-fedor/watchtower/pull/2346)
- Update module github.com/valyala/fasthttp to v1.74.0 by @renovate[bot] in [#2345](https://github.com/nicholas-fedor/watchtower/pull/2345)
- Update module github.com/gofiber/utils/v2 to v2.5.1 by @renovate[bot] in [#2344](https://github.com/nicholas-fedor/watchtower/pull/2344)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.50 by @renovate[bot] in [#2329](https://github.com/nicholas-fedor/watchtower/pull/2329)
- Update github.com/google/pprof digest to 6331bc6 by @renovate[bot] in [#2325](https://github.com/nicholas-fedor/watchtower/pull/2325)
- Update module github.com/gofiber/utils/v2 to v2.5.0 by @renovate[bot] in [#2323](https://github.com/nicholas-fedor/watchtower/pull/2323)
- Update go-openapi packages to v0.29.2 by @renovate[bot] in [#2320](https://github.com/nicholas-fedor/watchtower/pull/2320)
- Update go-openapi packages by @renovate[bot] in [#2317](https://github.com/nicholas-fedor/watchtower/pull/2317)
- Update module github.com/moby/moby/client to v0.6.0 by @renovate[bot] in [#2315](https://github.com/nicholas-fedor/watchtower/pull/2315)
- Update module github.com/moby/moby/api to v1.56.0 by @renovate[bot] in [#2312](https://github.com/nicholas-fedor/watchtower/pull/2312)
- Update module github.com/docker/cli to v29.8.0+incompatible by @renovate[bot] in [#2311](https://github.com/nicholas-fedor/watchtower/pull/2311)
- Update github.com/google/pprof digest to d6c3cb2 by @renovate[bot] in [#2309](https://github.com/nicholas-fedor/watchtower/pull/2309)
- Update module github.com/go-openapi/spec to v1 by @renovate[bot] in [#2307](https://github.com/nicholas-fedor/watchtower/pull/2307)
- Update module golang.org/x/crypto to v0.56.0 by @renovate[bot] in [#2306](https://github.com/nicholas-fedor/watchtower/pull/2306)
- Update module github.com/klauspost/compress to v1.20.0 by @renovate[bot] in [#2304](https://github.com/nicholas-fedor/watchtower/pull/2304)
- Update module github.com/gofiber/utils/v2 to v2.4.3 by @renovate[bot] in [#2303](https://github.com/nicholas-fedor/watchtower/pull/2303)
- Update golang:1.27.1-alpine docker digest to cf6fca6 by @renovate[bot] in [#2299](https://github.com/nicholas-fedor/watchtower/pull/2299)
- Update golang:alpine3.24 docker digest to cf6fca6 by @renovate[bot] in [#2300](https://github.com/nicholas-fedor/watchtower/pull/2300)

### New Contributors

- @shawly made their first contribution in [#2319](https://github.com/nicholas-fedor/watchtower/pull/2319)

## [1.22.0] - 2026-09-02

### Added

- Add configurable Docker image disk usage thresholds by @nicholas-fedor in [#2257](https://github.com/nicholas-fedor/watchtower/pull/2257)

### Fixed

- Format disk usage sizes in human-readable units by @nicholas-fedor in [#2297](https://github.com/nicholas-fedor/watchtower/pull/2297)
- Pace throttled registry hosts without bursting advertised quotas by @nicholas-fedor in [#2262](https://github.com/nicholas-fedor/watchtower/pull/2262)
- Complete image removal after self-update SIGTERM by @nicholas-fedor in [#2251](https://github.com/nicholas-fedor/watchtower/pull/2251)

### Changed

- Rename Viper locals from vip to vCfg by @nicholas-fedor in [#2258](https://github.com/nicholas-fedor/watchtower/pull/2258)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.19.0 by @renovate[bot] in [#2294](https://github.com/nicholas-fedor/watchtower/pull/2294)
- Update golang docker tag to v1.27.1 by @renovate[bot] in [#2293](https://github.com/nicholas-fedor/watchtower/pull/2293)
- Update cimg/go docker tag to v1.27.1 by @renovate[bot] in [#2292](https://github.com/nicholas-fedor/watchtower/pull/2292)
- Update golang:alpine3.24 docker digest to 26402d8 by @renovate[bot] in [#2291](https://github.com/nicholas-fedor/watchtower/pull/2291)
- Update github.com/google/pprof digest to ca85771 by @renovate[bot] in [#2290](https://github.com/nicholas-fedor/watchtower/pull/2290)
- Update cimg/go:1.27.0 docker digest to 6026c28 by @renovate[bot] in [#2289](https://github.com/nicholas-fedor/watchtower/pull/2289)
- Update go module directive to v1.27.1 by @renovate[bot] in [#2287](https://github.com/nicholas-fedor/watchtower/pull/2287)
- Update orhun/git-cliff-action action to v4.9.0 by @renovate[bot] in [#2285](https://github.com/nicholas-fedor/watchtower/pull/2285)
- Update module github.com/gofiber/schema to v1.8.5 by @renovate[bot] in [#2284](https://github.com/nicholas-fedor/watchtower/pull/2284)
- Update docker/setup-qemu-action action to v4.3.0 by @renovate[bot] in [#2283](https://github.com/nicholas-fedor/watchtower/pull/2283)
- Update module github.com/prometheus/common to v0.71.0 by @renovate[bot] in [#2280](https://github.com/nicholas-fedor/watchtower/pull/2280)
- Update module github.com/prometheus/client_model to v0.6.3 by @renovate[bot] in [#2279](https://github.com/nicholas-fedor/watchtower/pull/2279)
- Update step-security/harden-runner action to v2.21.1 by @renovate[bot] in [#2277](https://github.com/nicholas-fedor/watchtower/pull/2277)
- Update github.com/google/pprof digest to 4932ad3 by @renovate[bot] in [#2275](https://github.com/nicholas-fedor/watchtower/pull/2275)
- Update github.com/google/pprof digest to 67a7179 by @renovate[bot] in [#2274](https://github.com/nicholas-fedor/watchtower/pull/2274)
- Update anchore/sbom-action action to v0.24.2 by @renovate[bot] in [#2271](https://github.com/nicholas-fedor/watchtower/pull/2271)
- Update module github.com/prometheus/procfs to v0.22.0 by @renovate[bot] in [#2269](https://github.com/nicholas-fedor/watchtower/pull/2269)
- Update module github.com/gofiber/contrib/v3/zerolog to v1.1.4 by @renovate[bot] in [#2268](https://github.com/nicholas-fedor/watchtower/pull/2268)
- Update module github.com/gofiber/contrib/v3/swaggo to v1.0.10 by @renovate[bot] in [#2265](https://github.com/nicholas-fedor/watchtower/pull/2265)
- Update anchore/sbom-action action to v0.24.1 by @renovate[bot] in [#2264](https://github.com/nicholas-fedor/watchtower/pull/2264)
- Update module github.com/andybalholm/brotli to v1.2.3 by @renovate[bot] in [#2263](https://github.com/nicholas-fedor/watchtower/pull/2263)
- Update module github.com/onsi/gomega to v1.43.0 by @renovate[bot] in [#2260](https://github.com/nicholas-fedor/watchtower/pull/2260)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.49 by @renovate[bot] in [#2255](https://github.com/nicholas-fedor/watchtower/pull/2255)
- Update module github.com/docker/docker-credential-helpers to v0.9.9 by @renovate[bot] in [#2253](https://github.com/nicholas-fedor/watchtower/pull/2253)
- Update securego/gosec action to v2.29.0 by @renovate[bot] in [#2249](https://github.com/nicholas-fedor/watchtower/pull/2249)
- Update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.71.0 by @renovate[bot] in [#2247](https://github.com/nicholas-fedor/watchtower/pull/2247)
- Update github/codeql-action action to v4.37.9 by @renovate[bot] in [#2246](https://github.com/nicholas-fedor/watchtower/pull/2246)
- Update module github.com/nicholas-fedor/shoutrrr to v0.18.0 by @renovate[bot] in [#2243](https://github.com/nicholas-fedor/watchtower/pull/2243)

## [1.21.2] - 2026-08-25

### Fixed

- Include skipped containers in scan summaries by @nicholas-fedor in [#2241](https://github.com/nicholas-fedor/watchtower/pull/2241)
- Detect throttles advertised only via retry-after by @jmooo in [#2230](https://github.com/nicholas-fedor/watchtower/pull/2230)

### Documentation

- Add jmooo as a contributor for bug, and code by @allcontributors[bot] in [#2239](https://github.com/nicholas-fedor/watchtower/pull/2239)

### Continuous Integration

- Publish docs on stable tag releases by @nicholas-fedor in [#2235](https://github.com/nicholas-fedor/watchtower/pull/2235)

### Chores

- Apply gofumpt extras and golangci-lint formatter updates by @nicholas-fedor in [#2242](https://github.com/nicholas-fedor/watchtower/pull/2242)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.48 by @renovate[bot] in [#2238](https://github.com/nicholas-fedor/watchtower/pull/2238)
- Update module github.com/sirupsen/logrus to v1.10.2 by @renovate[bot] in [#2237](https://github.com/nicholas-fedor/watchtower/pull/2237)
- Update opentelemetry-go monorepo to v1.46.0 by @renovate[bot] in [#2232](https://github.com/nicholas-fedor/watchtower/pull/2232)
- Update github.com/google/pprof digest to 4d45320 by @renovate[bot] in [#2231](https://github.com/nicholas-fedor/watchtower/pull/2231)

### New Contributors

- @jmooo made their first contribution in [#2230](https://github.com/nicholas-fedor/watchtower/pull/2230)

## [1.21.1] - 2026-08-25

### Documentation

- Add ncrosty58 as a contributor for code by @allcontributors[bot] in [#2194](https://github.com/nicholas-fedor/watchtower/pull/2194)
- Add llc1123 as a contributor for code by @allcontributors[bot] in [#2193](https://github.com/nicholas-fedor/watchtower/pull/2193)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.17.2 by @renovate[bot] in [#2227](https://github.com/nicholas-fedor/watchtower/pull/2227)
- Update module github.com/go-openapi/spec to v0.22.11 by @renovate[bot] in [#2226](https://github.com/nicholas-fedor/watchtower/pull/2226)
- Bump go-openapi jsonreference and spec by @nicholas-fedor in [#2224](https://github.com/nicholas-fedor/watchtower/pull/2224)
- Update github.com/google/pprof digest to 8a17677 by @renovate[bot] in [#2222](https://github.com/nicholas-fedor/watchtower/pull/2222)
- Update cimg/go:1.27.0 docker digest to 4da2d4b by @renovate[bot] in [#2221](https://github.com/nicholas-fedor/watchtower/pull/2221)
- Update module github.com/gofiber/utils/v2 to v2.4.2 by @renovate[bot] in [#2219](https://github.com/nicholas-fedor/watchtower/pull/2219)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.47 by @renovate[bot] in [#2217](https://github.com/nicholas-fedor/watchtower/pull/2217)
- Update go-openapi packages to v0.29.1 by @renovate[bot] in [#2214](https://github.com/nicholas-fedor/watchtower/pull/2214)
- Update github/codeql-action action to v4.37.8 by @renovate[bot] in [#2212](https://github.com/nicholas-fedor/watchtower/pull/2212)
- Update cimg/go:1.27.0 docker digest to 91e576b by @renovate[bot] in [#2211](https://github.com/nicholas-fedor/watchtower/pull/2211)
- Update golang docker tag to v1.27.0 by @renovate[bot] in [#2209](https://github.com/nicholas-fedor/watchtower/pull/2209)
- Update go-openapi packages to v0.29.0 by @renovate[bot] in [#2208](https://github.com/nicholas-fedor/watchtower/pull/2208)
- Update go module directive to v1.27.0 by @renovate[bot] in [#2206](https://github.com/nicholas-fedor/watchtower/pull/2206)
- Update cimg/go docker tag to v1.27.0 by @renovate[bot] in [#2205](https://github.com/nicholas-fedor/watchtower/pull/2205)
- Update module github.com/stretchr/testify to v1.12.1 by @renovate[bot] in [#2202](https://github.com/nicholas-fedor/watchtower/pull/2202)
- Update golang:alpine3.24 docker digest to 4c9fe60 by @renovate[bot] in [#2201](https://github.com/nicholas-fedor/watchtower/pull/2201)
- Update cimg/go docker tag to v1.26.7 by @renovate[bot] in [#2199](https://github.com/nicholas-fedor/watchtower/pull/2199)
- Update golang:alpine3.24 docker digest to 28d89ee by @renovate[bot] in [#2198](https://github.com/nicholas-fedor/watchtower/pull/2198)
- Update docker/setup-buildx-action action to v4.3.0 by @renovate[bot] in [#2196](https://github.com/nicholas-fedor/watchtower/pull/2196)
- Update module github.com/sirupsen/logrus to v1.10.1 by @renovate[bot] in [#2195](https://github.com/nicholas-fedor/watchtower/pull/2195)

## [1.21.0] - 2026-08-18

### Added

- Add JSON porcelain output format by @nicholas-fedor in [#2158](https://github.com/nicholas-fedor/watchtower/pull/2158)

### Fixed

- Support cleanup flag in ephemeral self-update orchestrator by @nicholas-fedor in [#2189](https://github.com/nicholas-fedor/watchtower/pull/2189)
- Handle registry 429 rate limits with backoff and token bucket by @nicholas-fedor in [#2187](https://github.com/nicholas-fedor/watchtower/pull/2187)
- Only warn about missing image info for monitored containers by @ncrosty58 in [#2140](https://github.com/nicholas-fedor/watchtower/pull/2140)
- Remove redundant type field from deprecation warnings by @nicholas-fedor in [#2146](https://github.com/nicholas-fedor/watchtower/pull/2146)
- Remove non-running AutoRemove containers explicitly by @nicholas-fedor in [#2141](https://github.com/nicholas-fedor/watchtower/pull/2141)
- Respect label-enable setting for container details by @nicholas-fedor in [#2137](https://github.com/nicholas-fedor/watchtower/pull/2137)
- Validate required fields and standardize error messages by @nicholas-fedor in [#2124](https://github.com/nicholas-fedor/watchtower/pull/2124)
- Recover orphaned Watchtower containers on startup by @nicholas-fedor in [#2110](https://github.com/nicholas-fedor/watchtower/pull/2110)
- Remove orphaned Watchtower containers stuck in created state by @nicholas-fedor in [#2109](https://github.com/nicholas-fedor/watchtower/pull/2109)
- Increase create/start timeout and use fresh contexts for recovery by @nicholas-fedor in [#2108](https://github.com/nicholas-fedor/watchtower/pull/2108)
- Clear engine-generated MACs on container recreation by @nicholas-fedor in [#2106](https://github.com/nicholas-fedor/watchtower/pull/2106)

### Performance

- Reduce scan-cycle memory allocations by @nicholas-fedor in [#2152](https://github.com/nicholas-fedor/watchtower/pull/2152)

### Changed

- Extract a standalone preview module by @nicholas-fedor in [#2154](https://github.com/nicholas-fedor/watchtower/pull/2154)
- Replace logrus with zerolog by @nicholas-fedor in [#2128](https://github.com/nicholas-fedor/watchtower/pull/2128)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.17.1 by @renovate[bot] in [#2191](https://github.com/nicholas-fedor/watchtower/pull/2191)
- Update module github.com/stretchr/testify to v1.12.0 by @renovate[bot] in [#2185](https://github.com/nicholas-fedor/watchtower/pull/2185)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.46 by @renovate[bot] in [#2184](https://github.com/nicholas-fedor/watchtower/pull/2184)
- Update golang:alpine3.24 docker digest to 3889b42 by @renovate[bot] in [#2181](https://github.com/nicholas-fedor/watchtower/pull/2181)
- Update golang:1.26.6-alpine docker digest to 3889b42 by @renovate[bot] in [#2180](https://github.com/nicholas-fedor/watchtower/pull/2180)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.45 by @renovate[bot] in [#2178](https://github.com/nicholas-fedor/watchtower/pull/2178)
- Update step-security/harden-runner action to v2.21.0 by @renovate[bot] in [#2174](https://github.com/nicholas-fedor/watchtower/pull/2174)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.44 by @renovate[bot] in [#2173](https://github.com/nicholas-fedor/watchtower/pull/2173)
- Update cimg/go docker tag to v1.26.6 by @renovate[bot] in [#2171](https://github.com/nicholas-fedor/watchtower/pull/2171)
- Update module golang.org/x/tools to v0.49.0 by @renovate[bot] in [#2170](https://github.com/nicholas-fedor/watchtower/pull/2170)
- Update module golang.org/x/mod to v0.40.0 by @renovate[bot] in [#2169](https://github.com/nicholas-fedor/watchtower/pull/2169)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.43 by @renovate[bot] in [#2167](https://github.com/nicholas-fedor/watchtower/pull/2167)
- Update golang docker tag to v1.26.6 by @renovate[bot] in [#2166](https://github.com/nicholas-fedor/watchtower/pull/2166)
- Update go module directive to v1.26.6 by @renovate[bot] in [#2163](https://github.com/nicholas-fedor/watchtower/pull/2163)
- Update golang:alpine3.24 docker digest to 70b4654 by @renovate[bot] in [#2162](https://github.com/nicholas-fedor/watchtower/pull/2162)
- Update module github.com/sirupsen/logrus to v1.10.0 by @renovate[bot] in [#2160](https://github.com/nicholas-fedor/watchtower/pull/2160)
- Update github/codeql-action action to v4.37.7 by @renovate[bot] in [#2159](https://github.com/nicholas-fedor/watchtower/pull/2159)
- Update module github.com/gofiber/fiber/v3 to v3.5.0 by @renovate[bot] in [#2156](https://github.com/nicholas-fedor/watchtower/pull/2156)
- Update module golang.org/x/net to v0.58.0 by @renovate[bot] in [#2150](https://github.com/nicholas-fedor/watchtower/pull/2150)
- Update docker/dockerfile:1 docker digest to ecfaec9 by @renovate[bot] in [#2148](https://github.com/nicholas-fedor/watchtower/pull/2148)
- Update module golang.org/x/text to v0.41.0 by @renovate[bot] in [#2143](https://github.com/nicholas-fedor/watchtower/pull/2143)
- Update module golang.org/x/crypto to v0.55.0 by @renovate[bot] in [#2142](https://github.com/nicholas-fedor/watchtower/pull/2142)
- Update module github.com/gofiber/schema to v1.8.4 by @renovate[bot] in [#2138](https://github.com/nicholas-fedor/watchtower/pull/2138)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.42 by @renovate[bot] in [#2134](https://github.com/nicholas-fedor/watchtower/pull/2134)
- Update module github.com/onsi/ginkgo/v2 to v2.32.1 by @renovate[bot] in [#2133](https://github.com/nicholas-fedor/watchtower/pull/2133)
- Update module golang.org/x/mod to v0.39.0 by @renovate[bot] in [#2131](https://github.com/nicholas-fedor/watchtower/pull/2131)
- Update module google.golang.org/protobuf to v1.36.12 by @renovate[bot] in [#2129](https://github.com/nicholas-fedor/watchtower/pull/2129)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.41 by @renovate[bot] in [#2126](https://github.com/nicholas-fedor/watchtower/pull/2126)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.40 by @renovate[bot] in [#2121](https://github.com/nicholas-fedor/watchtower/pull/2121)
- Update actions/attest-build-provenance action to v4.2.2 by @renovate[bot] in [#2120](https://github.com/nicholas-fedor/watchtower/pull/2120)
- Update module github.com/docker/cli to v29.7.2+incompatible by @renovate[bot] in [#2118](https://github.com/nicholas-fedor/watchtower/pull/2118)
- Update module github.com/klauspost/compress to v1.19.2 by @renovate[bot] in [#2116](https://github.com/nicholas-fedor/watchtower/pull/2116)
- Update dependency python to v3.14.7 by @renovate[bot] in [#2112](https://github.com/nicholas-fedor/watchtower/pull/2112)
- Update step-security/harden-runner action to v2.20.1 by @renovate[bot] in [#2102](https://github.com/nicholas-fedor/watchtower/pull/2102)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.39 by @renovate[bot] in [#2101](https://github.com/nicholas-fedor/watchtower/pull/2101)

### New Contributors

- @ncrosty58 made their first contribution in [#2140](https://github.com/nicholas-fedor/watchtower/pull/2140)

## [1.20.3] - 2026-08-05

### Added

- Emit SSE events for the HTTP API check endpoint by @nicholas-fedor in [#2066](https://github.com/nicholas-fedor/watchtower/pull/2066)

### Fixed

- Prevent repeated self-updates when host ports are published by @nicholas-fedor in [#2079](https://github.com/nicholas-fedor/watchtower/pull/2079)

### Documentation

- Remove star history chart by @nicholas-fedor in [#2082](https://github.com/nicholas-fedor/watchtower/pull/2082)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.17.0 by @renovate[bot] in [#2099](https://github.com/nicholas-fedor/watchtower/pull/2099)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.38 by @renovate[bot] in [#2098](https://github.com/nicholas-fedor/watchtower/pull/2098)
- Update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.70.0 by @renovate[bot] in [#2096](https://github.com/nicholas-fedor/watchtower/pull/2096)
- Update github/codeql-action action to v4.37.6 by @renovate[bot] in [#2094](https://github.com/nicholas-fedor/watchtower/pull/2094)
- Update opentelemetry-go monorepo to v1.45.0 by @renovate[bot] in [#2091](https://github.com/nicholas-fedor/watchtower/pull/2091)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.37 by @renovate[bot] in [#2089](https://github.com/nicholas-fedor/watchtower/pull/2089)
- Update github/codeql-action action to v4.37.5 by @renovate[bot] in [#2088](https://github.com/nicholas-fedor/watchtower/pull/2088)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.36 by @renovate[bot] in [#2086](https://github.com/nicholas-fedor/watchtower/pull/2086)
- Update github.com/google/pprof digest to ef3492d by @renovate[bot] in [#2084](https://github.com/nicholas-fedor/watchtower/pull/2084)
- Update github.com/google/pprof digest to 5106ece by @renovate[bot] in [#2080](https://github.com/nicholas-fedor/watchtower/pull/2080)
- Update module github.com/docker/cli to v29.7.1+incompatible by @renovate[bot] in [#2076](https://github.com/nicholas-fedor/watchtower/pull/2076)
- Update module github.com/docker/cli to v29.7.0+incompatible by @renovate[bot] in [#2073](https://github.com/nicholas-fedor/watchtower/pull/2073)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.35 by @renovate[bot] in [#2071](https://github.com/nicholas-fedor/watchtower/pull/2071)
- Update module github.com/gofiber/contrib/v3/swaggo to v1.0.9 by @renovate[bot] in [#2070](https://github.com/nicholas-fedor/watchtower/pull/2070)
- Update github/codeql-action action to v4.37.4 by @renovate[bot] in [#2068](https://github.com/nicholas-fedor/watchtower/pull/2068)
- Update docker/login-action action to v4.6.0 by @renovate[bot] in [#2064](https://github.com/nicholas-fedor/watchtower/pull/2064)
- Update module github.com/gofiber/utils/v2 to v2.4.1 by @renovate[bot] in [#2063](https://github.com/nicholas-fedor/watchtower/pull/2063)

## [1.20.2] - 2026-07-28

### Fixed

- Mark slice flags as changed after docker secret expansion by @nicholas-fedor in [#2060](https://github.com/nicholas-fedor/watchtower/pull/2060)
- Address command execution bugs and update migration docs by @nicholas-fedor in [#2054](https://github.com/nicholas-fedor/watchtower/pull/2054)
- Ensure SSE stream lifecycle properly unsubscribes broadcaster by @nicholas-fedor in [#2050](https://github.com/nicholas-fedor/watchtower/pull/2050)
- Honor UseComposeDependsOn on scheduled update runs by @nicholas-fedor in [#2031](https://github.com/nicholas-fedor/watchtower/pull/2031)
- Harden self-update failure and cancel recovery by @nicholas-fedor in [#2005](https://github.com/nicholas-fedor/watchtower/pull/2005)
- Clean up failed recreates and nil host config by @nicholas-fedor in [#2004](https://github.com/nicholas-fedor/watchtower/pull/2004)
- Prefer config auth and accept identity tokens by @nicholas-fedor in [#2002](https://github.com/nicholas-fedor/watchtower/pull/2002)
- Tighten image filter matching and local-only detection by @nicholas-fedor in [#2003](https://github.com/nicholas-fedor/watchtower/pull/2003)
- Restore correct restart order for named network dependencies by @nicholas-fedor in [#2000](https://github.com/nicholas-fedor/watchtower/pull/2000)

### Changed

- Centralize watchtower configuration management by @nicholas-fedor in [#2035](https://github.com/nicholas-fedor/watchtower/pull/2035)

### Documentation

- Update badges by @nicholas-fedor in [#2037](https://github.com/nicholas-fedor/watchtower/pull/2037)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.16.3 by @renovate[bot] in [#2058](https://github.com/nicholas-fedor/watchtower/pull/2058)
- Update docker/login-action action to v4.5.2 by @renovate[bot] in [#2056](https://github.com/nicholas-fedor/watchtower/pull/2056)
- Update module github.com/moby/moby/client to v0.5.1 by @renovate[bot] in [#2051](https://github.com/nicholas-fedor/watchtower/pull/2051)
- Update go-openapi packages to v0.28.0 by @renovate[bot] in [#2052](https://github.com/nicholas-fedor/watchtower/pull/2052)
- Update module github.com/docker/go-connections to v0.8.1 by @renovate[bot] in [#2049](https://github.com/nicholas-fedor/watchtower/pull/2049)
- Update module github.com/valyala/fasthttp to v1.73.0 by @renovate[bot] in [#2047](https://github.com/nicholas-fedor/watchtower/pull/2047)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.34 by @renovate[bot] in [#2044](https://github.com/nicholas-fedor/watchtower/pull/2044)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.33 by @renovate[bot] in [#2042](https://github.com/nicholas-fedor/watchtower/pull/2042)
- Update module go.yaml.in/yaml/v3 to v3.0.5 by @renovate[bot] in [#2039](https://github.com/nicholas-fedor/watchtower/pull/2039)
- Update module github.com/docker/go-connections to v0.8.0 by @renovate[bot] in [#2033](https://github.com/nicholas-fedor/watchtower/pull/2033)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.32 by @renovate[bot] in [#2027](https://github.com/nicholas-fedor/watchtower/pull/2027)
- Update docker/login-action action to v4.5.1 by @renovate[bot] in [#2026](https://github.com/nicholas-fedor/watchtower/pull/2026)
- Update module github.com/prometheus/client_golang to v1.24.1 by @renovate[bot] in [#2022](https://github.com/nicholas-fedor/watchtower/pull/2022)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.31 by @renovate[bot] in [#2019](https://github.com/nicholas-fedor/watchtower/pull/2019)
- Update module github.com/gofiber/utils/v2 to v2.4.0 by @renovate[bot] in [#2017](https://github.com/nicholas-fedor/watchtower/pull/2017)
- Update ossf/scorecard-action action to v2.4.4 by @renovate[bot] in [#2016](https://github.com/nicholas-fedor/watchtower/pull/2016)
- Update docker/login-action action to v4.5.0 by @renovate[bot] in [#2014](https://github.com/nicholas-fedor/watchtower/pull/2014)
- Update module github.com/mattn/go-isatty to v0.0.24 by @renovate[bot] in [#2013](https://github.com/nicholas-fedor/watchtower/pull/2013)
- Update module github.com/gofiber/utils/v2 to v2.3.0 by @renovate[bot] in [#2010](https://github.com/nicholas-fedor/watchtower/pull/2010)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.30 by @renovate[bot] in [#2009](https://github.com/nicholas-fedor/watchtower/pull/2009)

## [1.20.1] - 2026-07-22

### Fixed

- Use application context for async update execution by @nicholas-fedor in [#1997](https://github.com/nicholas-fedor/watchtower/pull/1997)

### Chores

- Update module github.com/prometheus/common to v0.70.1 by @renovate[bot] in [#1998](https://github.com/nicholas-fedor/watchtower/pull/1998)
- Update github/codeql-action action to v4.37.3 by @renovate[bot] in [#1995](https://github.com/nicholas-fedor/watchtower/pull/1995)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.29 by @renovate[bot] in [#1993](https://github.com/nicholas-fedor/watchtower/pull/1993)

## [1.20.0] - 2026-07-21

### Added

- Add label-based container enable and disable filtering by @nicholas-fedor in [#1969](https://github.com/nicholas-fedor/watchtower/pull/1969)

### Fixed

- Validate notification-url secret file entries by @nicholas-fedor in [#1971](https://github.com/nicholas-fedor/watchtower/pull/1971)
- Ensure revive-stopped configuration reaches scheduled update runs by @nicholas-fedor in [#1961](https://github.com/nicholas-fedor/watchtower/pull/1961)
- Handle local-only images and clarify self-update skip messages by @nicholas-fedor in [#1960](https://github.com/nicholas-fedor/watchtower/pull/1960)
- Derive bearer service from realm by @llc1123 in [#1895](https://github.com/nicholas-fedor/watchtower/pull/1895)

### Changed

- Add and consolidate restart-policy updates by @nicholas-fedor in [#1965](https://github.com/nicholas-fedor/watchtower/pull/1965)
- Parallelize staleness checks with bounded concurrency by @nicholas-fedor in [#1958](https://github.com/nicholas-fedor/watchtower/pull/1958)
- Restructure registry auth and add bearer token caching by @nicholas-fedor in [#1951](https://github.com/nicholas-fedor/watchtower/pull/1951)
- Expand and reorganize http-api and metrics, improve logging, and restructure documentation by @nicholas-fedor in [#1939](https://github.com/nicholas-fedor/watchtower/pull/1939)

### Documentation

- Remove yaml array recommendation and restructure notifications documentation by @nicholas-fedor in [#1967](https://github.com/nicholas-fedor/watchtower/pull/1967)
- Remove unsupported go report card by @nicholas-fedor in [#1963](https://github.com/nicholas-fedor/watchtower/pull/1963)
- Update star history chart by @nicholas-fedor in [#1907](https://github.com/nicholas-fedor/watchtower/pull/1907)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.16.2 by @renovate[bot] in [#1991](https://github.com/nicholas-fedor/watchtower/pull/1991)
- Update github/codeql-action action to v4.37.2 by @renovate[bot] in [#1989](https://github.com/nicholas-fedor/watchtower/pull/1989)
- Update actions/setup-python action to v7 by @renovate[bot] in [#1978](https://github.com/nicholas-fedor/watchtower/pull/1978)
- Update module github.com/prometheus/client_golang to v1.24.0 by @renovate[bot] in [#1986](https://github.com/nicholas-fedor/watchtower/pull/1986)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.28 by @renovate[bot] in [#1985](https://github.com/nicholas-fedor/watchtower/pull/1985)
- Update module github.com/klauspost/compress to v1.19.1 by @renovate[bot] in [#1983](https://github.com/nicholas-fedor/watchtower/pull/1983)
- Update module github.com/go-logr/logr to v1.4.4 by @renovate[bot] in [#1982](https://github.com/nicholas-fedor/watchtower/pull/1982)
- Update go-openapi packages by @renovate[bot] in [#1980](https://github.com/nicholas-fedor/watchtower/pull/1980)
- Update actions/checkout action to v7.0.1 by @renovate[bot] in [#1979](https://github.com/nicholas-fedor/watchtower/pull/1979)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.27 by @renovate[bot] in [#1977](https://github.com/nicholas-fedor/watchtower/pull/1977)
- Update go-openapi packages to v0.27.1 by @renovate[bot] in [#1975](https://github.com/nicholas-fedor/watchtower/pull/1975)
- Update module github.com/gofiber/schema to v1.8.3 by @renovate[bot] in [#1973](https://github.com/nicholas-fedor/watchtower/pull/1973)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.26 by @renovate[bot] in [#1955](https://github.com/nicholas-fedor/watchtower/pull/1955)
- Update module github.com/docker/cli to v29.6.2+incompatible by @renovate[bot] in [#1953](https://github.com/nicholas-fedor/watchtower/pull/1953)
- Update github/codeql-action action to v4.37.1 by @renovate[bot] in [#1952](https://github.com/nicholas-fedor/watchtower/pull/1952)
- Update actions/setup-go action to v7 by @renovate[bot] in [#1946](https://github.com/nicholas-fedor/watchtower/pull/1946)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.25 by @renovate[bot] in [#1948](https://github.com/nicholas-fedor/watchtower/pull/1948)
- Update module github.com/mattn/go-isatty to v0.0.23 by @renovate[bot] in [#1947](https://github.com/nicholas-fedor/watchtower/pull/1947)
- Update module github.com/gofiber/utils/v2 to v2.2.0 by @renovate[bot] in [#1944](https://github.com/nicholas-fedor/watchtower/pull/1944)
- Update module github.com/gofiber/schema to v1.8.2 by @renovate[bot] in [#1941](https://github.com/nicholas-fedor/watchtower/pull/1941)
- Update securego/gosec action to v2.28.0 by @renovate[bot] in [#1937](https://github.com/nicholas-fedor/watchtower/pull/1937)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.23 by @renovate[bot] in [#1935](https://github.com/nicholas-fedor/watchtower/pull/1935)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.21 by @renovate[bot] in [#1934](https://github.com/nicholas-fedor/watchtower/pull/1934)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.20 by @renovate[bot] in [#1932](https://github.com/nicholas-fedor/watchtower/pull/1932)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.19 by @renovate[bot] in [#1931](https://github.com/nicholas-fedor/watchtower/pull/1931)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.18 by @renovate[bot] in [#1929](https://github.com/nicholas-fedor/watchtower/pull/1929)
- Update module github.com/prometheus/common to v0.70.0 by @renovate[bot] in [#1928](https://github.com/nicholas-fedor/watchtower/pull/1928)
- Update github.com/google/pprof digest to b9395ee by @renovate[bot] in [#1926](https://github.com/nicholas-fedor/watchtower/pull/1926)
- Update github.com/google/pprof digest to 301c45c by @renovate[bot] in [#1924](https://github.com/nicholas-fedor/watchtower/pull/1924)
- Update github.com/google/pprof digest to e2ebcbe by @renovate[bot] in [#1922](https://github.com/nicholas-fedor/watchtower/pull/1922)
- Update module golang.org/x/tools to v0.48.0 by @renovate[bot] in [#1920](https://github.com/nicholas-fedor/watchtower/pull/1920)
- Update module golang.org/x/net to v0.57.0 by @renovate[bot] in [#1917](https://github.com/nicholas-fedor/watchtower/pull/1917)
- Update golang:alpine docker digest to 0178a64 by @renovate[bot] in [#1916](https://github.com/nicholas-fedor/watchtower/pull/1916)
- Update module golang.org/x/mod to v0.38.0 by @renovate[bot] in [#1914](https://github.com/nicholas-fedor/watchtower/pull/1914)
- Update github/codeql-action action to v4.37.0 by @renovate[bot] in [#1913](https://github.com/nicholas-fedor/watchtower/pull/1913)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.17 by @renovate[bot] in [#1911](https://github.com/nicholas-fedor/watchtower/pull/1911)
- Update cimg/go docker tag to v1.26.5 by @renovate[bot] in [#1910](https://github.com/nicholas-fedor/watchtower/pull/1910)
- Update golang:alpine docker digest to 9097beb by @renovate[bot] in [#1906](https://github.com/nicholas-fedor/watchtower/pull/1906)
- Update go module directive to v1.26.5 by @renovate[bot] in [#1904](https://github.com/nicholas-fedor/watchtower/pull/1904)
- Update step-security/harden-runner action to v2.20.0 by @renovate[bot] in [#1903](https://github.com/nicholas-fedor/watchtower/pull/1903)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.16 by @renovate[bot] in [#1902](https://github.com/nicholas-fedor/watchtower/pull/1902)
- Update module golang.org/x/text to v0.39.0 by @renovate[bot] in [#1898](https://github.com/nicholas-fedor/watchtower/pull/1898)
- Update cimg/go:1.26.4 docker digest to 66a357f by @renovate[bot] in [#1897](https://github.com/nicholas-fedor/watchtower/pull/1897)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.15 by @renovate[bot] in [#1893](https://github.com/nicholas-fedor/watchtower/pull/1893)
- Update module github.com/pelletier/go-toml/v2 to v2.4.3 by @renovate[bot] in [#1890](https://github.com/nicholas-fedor/watchtower/pull/1890)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.14 by @renovate[bot] in [#1889](https://github.com/nicholas-fedor/watchtower/pull/1889)
- Update nicholas-fedor/actionlint-action action to v1.0.18 by @renovate[bot] in [#1888](https://github.com/nicholas-fedor/watchtower/pull/1888)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.13 by @renovate[bot] in [#1886](https://github.com/nicholas-fedor/watchtower/pull/1886)
- Update nicholas-fedor/actionlint-action action to v1.0.17 by @renovate[bot] in [#1885](https://github.com/nicholas-fedor/watchtower/pull/1885)
- Update docker/login-action action to v4.4.0 by @renovate[bot] in [#1883](https://github.com/nicholas-fedor/watchtower/pull/1883)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.12 by @renovate[bot] in [#1882](https://github.com/nicholas-fedor/watchtower/pull/1882)
- Update docker/setup-buildx-action action to v4.2.0 by @renovate[bot] in [#1881](https://github.com/nicholas-fedor/watchtower/pull/1881)
- Update docker/login-action action to v4.3.0 by @renovate[bot] in [#1878](https://github.com/nicholas-fedor/watchtower/pull/1878)
- Update nicholas-fedor/actionlint-action action to v1.0.16 by @renovate[bot] in [#1877](https://github.com/nicholas-fedor/watchtower/pull/1877)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.11 by @renovate[bot] in [#1875](https://github.com/nicholas-fedor/watchtower/pull/1875)
- Update github/codeql-action action to v4.36.3 by @renovate[bot] in [#1874](https://github.com/nicholas-fedor/watchtower/pull/1874)
- Update docker/setup-qemu-action action to v4.2.0 by @renovate[bot] in [#1872](https://github.com/nicholas-fedor/watchtower/pull/1872)

### New Contributors

- @llc1123 made their first contribution in [#1895](https://github.com/nicholas-fedor/watchtower/pull/1895)

## [1.19.0] - 2026-06-30

### Added

- Add image name filters by @Greite in [#1762](https://github.com/nicholas-fedor/watchtower/pull/1762)

### Fixed

- Correct deprecation notice url by @nicholas-fedor in [#1795](https://github.com/nicholas-fedor/watchtower/pull/1795)

### Documentation

- Add Greite as a contributor for code, test, and doc by @allcontributors[bot] in [#1800](https://github.com/nicholas-fedor/watchtower/pull/1800)

### Chores

- Update module github.com/prometheus/procfs to v0.21.1 by @renovate[bot] in [#1870](https://github.com/nicholas-fedor/watchtower/pull/1870)
- Update golangci/golangci-lint-action action to v9.3.0 by @renovate[bot] in [#1868](https://github.com/nicholas-fedor/watchtower/pull/1868)
- Update goreleaser/goreleaser-action action to v7.2.3 by @renovate[bot] in [#1867](https://github.com/nicholas-fedor/watchtower/pull/1867)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.10 by @renovate[bot] in [#1864](https://github.com/nicholas-fedor/watchtower/pull/1864)
- Update nicholas-fedor/actionlint-action action to v1.0.15 by @renovate[bot] in [#1863](https://github.com/nicholas-fedor/watchtower/pull/1863)
- Update module github.com/prometheus/procfs to v0.21.0 by @renovate[bot] in [#1861](https://github.com/nicholas-fedor/watchtower/pull/1861)
- Update actions/cache action to v6.1.0 by @renovate[bot] in [#1859](https://github.com/nicholas-fedor/watchtower/pull/1859)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.9 by @renovate[bot] in [#1858](https://github.com/nicholas-fedor/watchtower/pull/1858)
- Update module github.com/docker/cli to v29.6.1+incompatible by @renovate[bot] in [#1856](https://github.com/nicholas-fedor/watchtower/pull/1856)
- Update actions/attest-build-provenance action to v4.1.1 by @renovate[bot] in [#1855](https://github.com/nicholas-fedor/watchtower/pull/1855)
- Update module golang.org/x/tools to v0.47.0 by @renovate[bot] in [#1853](https://github.com/nicholas-fedor/watchtower/pull/1853)
- Update nicholas-fedor/actionlint-action action to v1.0.14 by @renovate[bot] in [#1849](https://github.com/nicholas-fedor/watchtower/pull/1849)
- Update actions/setup-python action to v6.3.0 by @renovate[bot] in [#1848](https://github.com/nicholas-fedor/watchtower/pull/1848)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.8 by @renovate[bot] in [#1847](https://github.com/nicholas-fedor/watchtower/pull/1847)
- Update actions/setup-go action to v6.5.0 by @renovate[bot] in [#1845](https://github.com/nicholas-fedor/watchtower/pull/1845)
- Update module github.com/pelletier/go-toml/v2 to v2.4.2 by @renovate[bot] in [#1844](https://github.com/nicholas-fedor/watchtower/pull/1844)
- Update module github.com/onsi/gomega to v1.42.1 by @renovate[bot] in [#1842](https://github.com/nicholas-fedor/watchtower/pull/1842)
- Update actions/cache action to v6 by @renovate[bot] in [#1840](https://github.com/nicholas-fedor/watchtower/pull/1840)
- Update module github.com/onsi/ginkgo/v2 to v2.32.0 by @renovate[bot] in [#1838](https://github.com/nicholas-fedor/watchtower/pull/1838)
- Update nicholas-fedor/actionlint-action action to v1.0.13 by @renovate[bot] in [#1836](https://github.com/nicholas-fedor/watchtower/pull/1836)
- Update module github.com/pelletier/go-toml/v2 to v2.4.1 by @renovate[bot] in [#1835](https://github.com/nicholas-fedor/watchtower/pull/1835)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.7 by @renovate[bot] in [#1833](https://github.com/nicholas-fedor/watchtower/pull/1833)
- Update nicholas-fedor/actionlint-action action to v1.0.12 by @renovate[bot] in [#1832](https://github.com/nicholas-fedor/watchtower/pull/1832)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.5 by @renovate[bot] in [#1830](https://github.com/nicholas-fedor/watchtower/pull/1830)
- Update nicholas-fedor/actionlint-action action to v1.0.10 by @renovate[bot] in [#1828](https://github.com/nicholas-fedor/watchtower/pull/1828)
- Update actions/checkout action to v7 by @renovate[bot] in [#1827](https://github.com/nicholas-fedor/watchtower/pull/1827)
- Update module github.com/moby/moby/client to v0.5.0 by @renovate[bot] in [#1825](https://github.com/nicholas-fedor/watchtower/pull/1825)
- Update module github.com/docker/cli to v29.6.0+incompatible by @renovate[bot] in [#1824](https://github.com/nicholas-fedor/watchtower/pull/1824)
- Update module github.com/moby/moby/api to v1.55.0 by @renovate[bot] in [#1822](https://github.com/nicholas-fedor/watchtower/pull/1822)
- Update golang:alpine docker digest to 3ad5730 by @renovate[bot] in [#1821](https://github.com/nicholas-fedor/watchtower/pull/1821)
- Update module github.com/prometheus/common to v0.69.0 by @renovate[bot] in [#1818](https://github.com/nicholas-fedor/watchtower/pull/1818)
- Update module github.com/pelletier/go-toml/v2 to v2.4.0 by @renovate[bot] in [#1816](https://github.com/nicholas-fedor/watchtower/pull/1816)
- Update alpine:3.24.1 docker digest to 28bd5fe by @renovate[bot] in [#1814](https://github.com/nicholas-fedor/watchtower/pull/1814)
- Update alpine docker tag to v3.24.1 by @renovate[bot] in [#1812](https://github.com/nicholas-fedor/watchtower/pull/1812)
- Update golang:alpine docker digest to f1ddd9f by @renovate[bot] in [#1810](https://github.com/nicholas-fedor/watchtower/pull/1810)
- Update nicholas-fedor/actionlint-action action to v1.0.9 by @renovate[bot] in [#1809](https://github.com/nicholas-fedor/watchtower/pull/1809)
- Update module github.com/onsi/gomega to v1.42.0 by @renovate[bot] in [#1805](https://github.com/nicholas-fedor/watchtower/pull/1805)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.4 by @renovate[bot] in [#1807](https://github.com/nicholas-fedor/watchtower/pull/1807)
- Update module github.com/onsi/ginkgo/v2 to v2.31.0 by @renovate[bot] in [#1804](https://github.com/nicholas-fedor/watchtower/pull/1804)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.3 by @renovate[bot] in [#1802](https://github.com/nicholas-fedor/watchtower/pull/1802)
- Update nicholas-fedor/actionlint-action action to v1.0.8 by @renovate[bot] in [#1801](https://github.com/nicholas-fedor/watchtower/pull/1801)
- Update nicholas-fedor/actionlint-action action to v1.0.7 by @renovate[bot] in [#1797](https://github.com/nicholas-fedor/watchtower/pull/1797)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.2 by @renovate[bot] in [#1794](https://github.com/nicholas-fedor/watchtower/pull/1794)
- Update module golang.org/x/tools to v0.46.0 by @renovate[bot] in [#1792](https://github.com/nicholas-fedor/watchtower/pull/1792)
- Update module github.com/onsi/ginkgo/v2 to v2.30.0 by @renovate[bot] in [#1791](https://github.com/nicholas-fedor/watchtower/pull/1791)
- Update module github.com/felixge/httpsnoop to v1.1.0 by @renovate[bot] in [#1788](https://github.com/nicholas-fedor/watchtower/pull/1788)
- Update golang:alpine docker digest to 7a3e500 by @renovate[bot] in [#1787](https://github.com/nicholas-fedor/watchtower/pull/1787)

### New Contributors

- @Greite made their first contribution in [#1762](https://github.com/nicholas-fedor/watchtower/pull/1762)

## [1.18.1] - 2026-06-11

### Fixed

- Suppress deprecation warning when legacy email subject tag is empty by @nicholas-fedor in [#1785](https://github.com/nicholas-fedor/watchtower/pull/1785)

### Documentation

- Add Thubo as a contributor for docs by @allcontributors[bot] in [#1783](https://github.com/nicholas-fedor/watchtower/pull/1783)
- Fix notify-upgrade exec command by @Thubo in [#1774](https://github.com/nicholas-fedor/watchtower/pull/1774)

### Chores

- Update golang:alpine docker digest to a6a091e by @renovate[bot] in [#1780](https://github.com/nicholas-fedor/watchtower/pull/1780)
- Update golang:alpine docker digest to bd14630 by @renovate[bot] in [#1778](https://github.com/nicholas-fedor/watchtower/pull/1778)
- Update module golang.org/x/net to v0.56.0 by @renovate[bot] in [#1776](https://github.com/nicholas-fedor/watchtower/pull/1776)
- Update dependency python to v3.14.6 by @renovate[bot] in [#1775](https://github.com/nicholas-fedor/watchtower/pull/1775)
- Update alpine:3.24.0 docker digest to a2d49ea by @renovate[bot] in [#1771](https://github.com/nicholas-fedor/watchtower/pull/1771)
- Update alpine:3.24.0 docker digest to 8ddefa9 by @renovate[bot] in [#1769](https://github.com/nicholas-fedor/watchtower/pull/1769)
- Update alpine docker tag to v3.24.0 by @renovate[bot] in [#1767](https://github.com/nicholas-fedor/watchtower/pull/1767)

### New Contributors

- @Thubo made their first contribution in [#1774](https://github.com/nicholas-fedor/watchtower/pull/1774)

## [1.18.0] - 2026-06-09

### Added

- Add read-only /v1/containers endpoint by @barnabasbusa in [#1700](https://github.com/nicholas-fedor/watchtower/pull/1700)
- Add eligible_at timestamp to cooldown deferral logs and notifications by @nicholas-fedor in [#1698](https://github.com/nicholas-fedor/watchtower/pull/1698)
- Add Docker registry mirror support for digest comparison by @nicholas-fedor in [#1693](https://github.com/nicholas-fedor/watchtower/pull/1693)

### Fixed

- Add additional safeguards to address persistent old Watchtower containers by @nicholas-fedor in [#1743](https://github.com/nicholas-fedor/watchtower/pull/1743)
- Update Shoutrrr MS Teams integration by @nicholas-fedor in [#1718](https://github.com/nicholas-fedor/watchtower/pull/1718)
- Resolve stale watchtower container ambiguity in detection and cleanup by @nicholas-fedor in [#1713](https://github.com/nicholas-fedor/watchtower/pull/1713)
- Strip quotes from cron schedule spec before parsing by @nicholas-fedor in [#1686](https://github.com/nicholas-fedor/watchtower/pull/1686)
- Set Detach true in ExecStart to prevent blocking on command execution by @nicholas-fedor in [#1683](https://github.com/nicholas-fedor/watchtower/pull/1683)
- Rewrite implicit restart resolution and dependency matching by @nicholas-fedor in [#1677](https://github.com/nicholas-fedor/watchtower/pull/1677)
- Interpret bare numeric timeout as seconds by @nicholas-fedor in [#1675](https://github.com/nicholas-fedor/watchtower/pull/1675)
- Default empty device CgroupPermissions to 'rwm' for Podman compatibility by @nicholas-fedor in [#1673](https://github.com/nicholas-fedor/watchtower/pull/1673)
- Prevent image pull when within cooldown window by @nicholas-fedor in [#1669](https://github.com/nicholas-fedor/watchtower/pull/1669)

### Documentation

- Add barnabasbusa as a contributor for code and doc by @allcontributors[bot] in [#1706](https://github.com/nicholas-fedor/watchtower/pull/1706)
- Update allcontributors emoji key link to use canonical URL by @nicholas-fedor in [#1696](https://github.com/nicholas-fedor/watchtower/pull/1696)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.16.1 by @renovate[bot] in [#1765](https://github.com/nicholas-fedor/watchtower/pull/1765)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.1 by @renovate[bot] in [#1763](https://github.com/nicholas-fedor/watchtower/pull/1763)
- Update module golang.org/x/text to v0.38.0 by @renovate[bot] in [#1758](https://github.com/nicholas-fedor/watchtower/pull/1758)
- Update module golang.org/x/term to v0.44.0 by @renovate[bot] in [#1756](https://github.com/nicholas-fedor/watchtower/pull/1756)
- Update nicholas-fedor/actionlint-action action to v1.0.6 by @renovate[bot] in [#1755](https://github.com/nicholas-fedor/watchtower/pull/1755)
- Update module golang.org/x/sys to v0.46.0 by @renovate[bot] in [#1753](https://github.com/nicholas-fedor/watchtower/pull/1753)
- Update module github.com/docker/docker-credential-helpers to v0.9.8 by @renovate[bot] in [#1752](https://github.com/nicholas-fedor/watchtower/pull/1752)
- Update module golang.org/x/sync to v0.21.0 by @renovate[bot] in [#1750](https://github.com/nicholas-fedor/watchtower/pull/1750)
- Update module golang.org/x/mod to v0.37.0 by @renovate[bot] in [#1749](https://github.com/nicholas-fedor/watchtower/pull/1749)
- Update nicholas-fedor/govulncheck-action action to v1.0.5 by @renovate[bot] in [#1746](https://github.com/nicholas-fedor/watchtower/pull/1746)
- Update nicholas-fedor/actionlint-action action to v1.0.5 by @renovate[bot] in [#1744](https://github.com/nicholas-fedor/watchtower/pull/1744)
- Update nicholas-fedor/actionlint-action action to v1.0.4 by @renovate[bot] in [#1741](https://github.com/nicholas-fedor/watchtower/pull/1741)
- Update codecov/codecov-action action to v7 by @renovate[bot] in [#1739](https://github.com/nicholas-fedor/watchtower/pull/1739)
- Deprecate legacy notification types for v2 removal by @nicholas-fedor in [#1737](https://github.com/nicholas-fedor/watchtower/pull/1737)
- Update github/codeql-action action to v4.36.2 by @renovate[bot] in [#1735](https://github.com/nicholas-fedor/watchtower/pull/1735)
- Update github.com/google/pprof digest to 7023385 by @renovate[bot] in [#1733](https://github.com/nicholas-fedor/watchtower/pull/1733)
- Update module github.com/docker/cli to v29.5.3+incompatible by @renovate[bot] in [#1731](https://github.com/nicholas-fedor/watchtower/pull/1731)
- Update cimg/go docker tag to v1.26.4 by @renovate[bot] in [#1728](https://github.com/nicholas-fedor/watchtower/pull/1728)
- Update module github.com/prometheus/common to v0.68.1 by @renovate[bot] in [#1726](https://github.com/nicholas-fedor/watchtower/pull/1726)
- Update golang:alpine docker digest to f23e8b2 by @renovate[bot] in [#1724](https://github.com/nicholas-fedor/watchtower/pull/1724)
- Update golang:1.26.4-alpine3.22 docker digest to 727cfc3 by @renovate[bot] in [#1723](https://github.com/nicholas-fedor/watchtower/pull/1723)
- Update golang docker tag to v1.26.4 by @renovate[bot] in [#1721](https://github.com/nicholas-fedor/watchtower/pull/1721)
- Update golang:alpine docker digest to 376f4a3 by @renovate[bot] in [#1719](https://github.com/nicholas-fedor/watchtower/pull/1719)
- Update module github.com/nicholas-fedor/shoutrrr to v0.16.0 by @renovate[bot] in [#1716](https://github.com/nicholas-fedor/watchtower/pull/1716)
- Update go module directive to v1.26.4 by @renovate[bot] in [#1715](https://github.com/nicholas-fedor/watchtower/pull/1715)
- Update github/codeql-action action to v4.36.1 by @renovate[bot] in [#1711](https://github.com/nicholas-fedor/watchtower/pull/1711)
- Update actions/checkout action to v6.0.3 by @renovate[bot] in [#1710](https://github.com/nicholas-fedor/watchtower/pull/1710)
- Update securego/gosec action to v2.27.1 by @renovate[bot] in [#1707](https://github.com/nicholas-fedor/watchtower/pull/1707)
- Update module github.com/prometheus/common to v0.68.0 by @renovate[bot] in [#1703](https://github.com/nicholas-fedor/watchtower/pull/1703)
- Update module github.com/mattn/go-colorable to v0.1.15 by @renovate[bot] in [#1701](https://github.com/nicholas-fedor/watchtower/pull/1701)
- Update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.69.0 by @renovate[bot] in [#1688](https://github.com/nicholas-fedor/watchtower/pull/1688)
- Update opentelemetry-go monorepo to v1.44.0 by @renovate[bot] in [#1681](https://github.com/nicholas-fedor/watchtower/pull/1681)
- Update docker/setup-qemu-action action to v4.1.0 by @renovate[bot] in [#1679](https://github.com/nicholas-fedor/watchtower/pull/1679)
- Update module github.com/nicholas-fedor/shoutrrr to v0.15.1 by @renovate[bot] in [#1671](https://github.com/nicholas-fedor/watchtower/pull/1671)

### New Contributors

- @barnabasbusa made their first contribution in [#1700](https://github.com/nicholas-fedor/watchtower/pull/1700)

## [1.17.2] - 2026-05-23

### Fixed

- Add timeout input validation and improve docs by @nicholas-fedor in [#1665](https://github.com/nicholas-fedor/watchtower/pull/1665)
- Restore revive-stopped configuration handling by @nicholas-fedor in [#1657](https://github.com/nicholas-fedor/watchtower/pull/1657)

### Changed

- Replace curly apostrophes with straight apostrophes by @nicholas-fedor in [#1667](https://github.com/nicholas-fedor/watchtower/pull/1667)

### Chores

- Update docker/setup-buildx-action action to v4.1.0 by @renovate[bot] in [#1662](https://github.com/nicholas-fedor/watchtower/pull/1662)
- Update github/codeql-action action to v4.36.0 by @renovate[bot] in [#1660](https://github.com/nicholas-fedor/watchtower/pull/1660)
- Update docker/login-action action to v4.2.0 by @renovate[bot] in [#1659](https://github.com/nicholas-fedor/watchtower/pull/1659)
- Update module golang.org/x/net to v0.55.0 by @renovate[bot] in [#1655](https://github.com/nicholas-fedor/watchtower/pull/1655)
- Update golangci/golangci-lint-action action to v9.2.1 by @renovate[bot] in [#1654](https://github.com/nicholas-fedor/watchtower/pull/1654)
- Update module golang.org/x/sys to v0.45.0 by @renovate[bot] in [#1651](https://github.com/nicholas-fedor/watchtower/pull/1651)
- Update cimg/go:1.26.3 docker digest to 9a5aff9 by @renovate[bot] in [#1650](https://github.com/nicholas-fedor/watchtower/pull/1650)
- Update step-security/harden-runner action to v2.19.4 by @renovate[bot] in [#1647](https://github.com/nicholas-fedor/watchtower/pull/1647)
- Update nicholas-fedor/go-proxy-pull-action action to v1.1.0 by @renovate[bot] in [#1645](https://github.com/nicholas-fedor/watchtower/pull/1645)
- Update module github.com/docker/cli to v29.5.2+incompatible by @renovate[bot] in [#1643](https://github.com/nicholas-fedor/watchtower/pull/1643)
- Update docker/dockerfile:1 docker digest to 87999aa by @renovate[bot] in [#1641](https://github.com/nicholas-fedor/watchtower/pull/1641)

## [1.17.1] - 2026-05-19

### Fixed

- Restore no-restart flag to only skip container start by @nicholas-fedor in [#1626](https://github.com/nicholas-fedor/watchtower/pull/1626)
- Add missing watchtower label to Docker images by @nicholas-fedor in [#1620](https://github.com/nicholas-fedor/watchtower/pull/1620)

### Changed

- Migrate from docker/docker to moby/moby split packages for v29 compatibility by @nicholas-fedor in [#1613](https://github.com/nicholas-fedor/watchtower/pull/1613)

### Tests

- Increase sleep duration to fix timer resolution race by @nicholas-fedor in [#1614](https://github.com/nicholas-fedor/watchtower/pull/1614)

### Chores

- Update nicholas-fedor/go-proxy-pull-action action to v1.0.9 by @renovate[bot] in [#1639](https://github.com/nicholas-fedor/watchtower/pull/1639)
- Update goreleaser/goreleaser-action action to v7.2.2 by @renovate[bot] in [#1637](https://github.com/nicholas-fedor/watchtower/pull/1637)
- Update module github.com/docker/cli to v29.5.1+incompatible by @renovate[bot] in [#1635](https://github.com/nicholas-fedor/watchtower/pull/1635)
- Update codecov/codecov-action action to v6.0.1 by @renovate[bot] in [#1634](https://github.com/nicholas-fedor/watchtower/pull/1634)
- Update module github.com/onsi/gomega to v1.41.0 by @renovate[bot] in [#1631](https://github.com/nicholas-fedor/watchtower/pull/1631)
- Update module github.com/onsi/ginkgo/v2 to v2.29.0 by @renovate[bot] in [#1630](https://github.com/nicholas-fedor/watchtower/pull/1630)
- Update github/codeql-action action to v4.35.5 by @renovate[bot] in [#1627](https://github.com/nicholas-fedor/watchtower/pull/1627)
- Update step-security/harden-runner action to v2.19.3 by @renovate[bot] in [#1624](https://github.com/nicholas-fedor/watchtower/pull/1624)
- Update module github.com/docker/cli to v29.5.0+incompatible by @renovate[bot] in [#1622](https://github.com/nicholas-fedor/watchtower/pull/1622)
- Update step-security/harden-runner action to v2.19.2 by @renovate[bot] in [#1617](https://github.com/nicholas-fedor/watchtower/pull/1617)

## [1.17.0] - 2026-05-11

### Added

- Add async query parameter support by @nicholas-fedor in [#1609](https://github.com/nicholas-fedor/watchtower/pull/1609)

### Documentation

- Add link to reference linked containers advanced features documentation by @nicholas-fedor in [#1532](https://github.com/nicholas-fedor/watchtower/pull/1532)
- Update changelog for main by @nicholas-fedor in [#1512](https://github.com/nicholas-fedor/watchtower/pull/1512)

### Continuous Integration

- Modernize GoReleaser config and GitHub Actions workflows by @nicholas-fedor in [#1607](https://github.com/nicholas-fedor/watchtower/pull/1607)
- Refactor GitHub Actions lint workflow by @nicholas-fedor in [#1606](https://github.com/nicholas-fedor/watchtower/pull/1606)
- Enable host network for buildkit driver by @nicholas-fedor in [#1569](https://github.com/nicholas-fedor/watchtower/pull/1569)
- Reorder containerd snapshotter enablement by @nicholas-fedor in [#1568](https://github.com/nicholas-fedor/watchtower/pull/1568)

### Chores

- Update dependency python to v3.14.5 by @renovate[bot] in [#1611](https://github.com/nicholas-fedor/watchtower/pull/1611)
- Update module golang.org/x/tools to v0.45.0 by @renovate[bot] in [#1605](https://github.com/nicholas-fedor/watchtower/pull/1605)
- Update module golang.org/x/net to v0.54.0 by @renovate[bot] in [#1603](https://github.com/nicholas-fedor/watchtower/pull/1603)
- Update module golang.org/x/mod to v0.36.0 by @renovate[bot] in [#1602](https://github.com/nicholas-fedor/watchtower/pull/1602)
- Update cimg/go docker tag to v1.26.3 by @renovate[bot] in [#1601](https://github.com/nicholas-fedor/watchtower/pull/1601)
- Update module golang.org/x/sys to v0.44.0 by @renovate[bot] in [#1600](https://github.com/nicholas-fedor/watchtower/pull/1600)
- Update module github.com/docker/docker-credential-helpers to v0.9.7 by @renovate[bot] in [#1599](https://github.com/nicholas-fedor/watchtower/pull/1599)
- Update golang docker tag to v1.26.3 by @renovate[bot] in [#1598](https://github.com/nicholas-fedor/watchtower/pull/1598)
- Update go module directive to v1.26.3 by @renovate[bot] in [#1597](https://github.com/nicholas-fedor/watchtower/pull/1597)
- Update cimg/go:1.26.2 docker digest to 0594489 by @renovate[bot] in [#1596](https://github.com/nicholas-fedor/watchtower/pull/1596)
- Update golang:alpine docker digest to 91eda97 by @renovate[bot] in [#1595](https://github.com/nicholas-fedor/watchtower/pull/1595)
- Update github/codeql-action digest to 68bde55 by @renovate[bot] in [#1594](https://github.com/nicholas-fedor/watchtower/pull/1594)
- Update github.com/google/pprof digest to 92041b7 by @renovate[bot] in [#1593](https://github.com/nicholas-fedor/watchtower/pull/1593)
- Update module github.com/nicholas-fedor/shoutrrr to v0.15.0 by @renovate[bot] in [#1592](https://github.com/nicholas-fedor/watchtower/pull/1592)
- Update module github.com/docker/cli to v29.4.3+incompatible by @renovate[bot] in [#1591](https://github.com/nicholas-fedor/watchtower/pull/1591)
- Update module github.com/fsnotify/fsnotify to v1.10.1 by @renovate[bot] in [#1590](https://github.com/nicholas-fedor/watchtower/pull/1590)
- Update step-security/harden-runner action to v2.19.1 by @renovate[bot] in [#1588](https://github.com/nicholas-fedor/watchtower/pull/1588)
- Update module github.com/pelletier/go-toml/v2 to v2.3.1 by @renovate[bot] in [#1587](https://github.com/nicholas-fedor/watchtower/pull/1587)
- Update github/codeql-action digest to e46ed2c by @renovate[bot] in [#1586](https://github.com/nicholas-fedor/watchtower/pull/1586)
- Update module github.com/docker/cli to v29.4.2+incompatible by @renovate[bot] in [#1585](https://github.com/nicholas-fedor/watchtower/pull/1585)
- Update module github.com/masterminds/semver/v3 to v3.5.0 by @renovate[bot] in [#1584](https://github.com/nicholas-fedor/watchtower/pull/1584)
- Update module github.com/fsnotify/fsnotify to v1.10.0 by @renovate[bot] in [#1583](https://github.com/nicholas-fedor/watchtower/pull/1583)
- Update module github.com/onsi/ginkgo/v2 to v2.28.3 by @renovate[bot] in [#1581](https://github.com/nicholas-fedor/watchtower/pull/1581)
- Update securego/gosec action to v2.26.1 by @renovate[bot] in [#1580](https://github.com/nicholas-fedor/watchtower/pull/1580)
- Update module github.com/onsi/ginkgo/v2 to v2.28.2 by @renovate[bot] in [#1579](https://github.com/nicholas-fedor/watchtower/pull/1579)
- Update module github.com/mattn/go-isatty to v0.0.22 by @renovate[bot] in [#1578](https://github.com/nicholas-fedor/watchtower/pull/1578)
- Update orhun/git-cliff-action digest to f50e115 by @renovate[bot] in [#1577](https://github.com/nicholas-fedor/watchtower/pull/1577)
- Update module github.com/docker/cli to v29.4.1+incompatible by @renovate[bot] in [#1574](https://github.com/nicholas-fedor/watchtower/pull/1574)
- Update module github.com/docker/docker-credential-helpers to v0.9.6 by @renovate[bot] in [#1573](https://github.com/nicholas-fedor/watchtower/pull/1573)
- Update step-security/harden-runner action to v2.19.0 by @renovate[bot] in [#1571](https://github.com/nicholas-fedor/watchtower/pull/1571)
- Update golang:1.26.2-alpine3.22 docker digest to 7ef9411 by @renovate[bot] in [#1567](https://github.com/nicholas-fedor/watchtower/pull/1567)
- Update golang:1.26.2-alpine3.22 docker digest to 6ebcc4e by @renovate[bot] in [#1565](https://github.com/nicholas-fedor/watchtower/pull/1565)
- Update golang:1.26.2-alpine3.22 docker digest to 18e6f5a by @renovate[bot] in [#1564](https://github.com/nicholas-fedor/watchtower/pull/1564)
- Update golang:alpine docker digest to f853308 by @renovate[bot] in [#1562](https://github.com/nicholas-fedor/watchtower/pull/1562)
- Update module github.com/docker/go-connections to v0.7.0 by @renovate[bot] in [#1561](https://github.com/nicholas-fedor/watchtower/pull/1561)
- Update alpine docker tag to v3.23.4 by @renovate[bot] in [#1560](https://github.com/nicholas-fedor/watchtower/pull/1560)
- Update golang:alpine docker digest to 27f8293 by @renovate[bot] in [#1559](https://github.com/nicholas-fedor/watchtower/pull/1559)
- Update github/codeql-action digest to 95e58e9 by @renovate[bot] in [#1558](https://github.com/nicholas-fedor/watchtower/pull/1558)
- Update step-security/harden-runner action to v2.18.0 by @renovate[bot] in [#1557](https://github.com/nicholas-fedor/watchtower/pull/1557)
- Update actions/cache digest to 27d5ce7 by @renovate[bot] in [#1556](https://github.com/nicholas-fedor/watchtower/pull/1556)
- Update module golang.org/x/tools to v0.44.0 by @renovate[bot] in [#1553](https://github.com/nicholas-fedor/watchtower/pull/1553)
- Update module golang.org/x/net to v0.53.0 by @renovate[bot] in [#1552](https://github.com/nicholas-fedor/watchtower/pull/1552)
- Update step-security/harden-runner action to v2.17.0 by @renovate[bot] in [#1551](https://github.com/nicholas-fedor/watchtower/pull/1551)
- Update module golang.org/x/text to v0.36.0 by @renovate[bot] in [#1550](https://github.com/nicholas-fedor/watchtower/pull/1550)
- Update module golang.org/x/term to v0.42.0 by @renovate[bot] in [#1549](https://github.com/nicholas-fedor/watchtower/pull/1549)
- Update module golang.org/x/mod to v0.35.0 by @renovate[bot] in [#1548](https://github.com/nicholas-fedor/watchtower/pull/1548)
- Update module github.com/mattn/go-isatty to v0.0.21 by @renovate[bot] in [#1547](https://github.com/nicholas-fedor/watchtower/pull/1547)
- Update cimg/go docker tag to v1.26.2 by @renovate[bot] in [#1546](https://github.com/nicholas-fedor/watchtower/pull/1546)
- Update cimg/go:1.26.1 docker digest to f813931 by @renovate[bot] in [#1545](https://github.com/nicholas-fedor/watchtower/pull/1545)
- Update module golang.org/x/sys to v0.43.0 by @renovate[bot] in [#1544](https://github.com/nicholas-fedor/watchtower/pull/1544)
- Update golangci/golangci-lint-action digest to 46c9287 by @renovate[bot] in [#1543](https://github.com/nicholas-fedor/watchtower/pull/1543)
- Update golang docker tag to v1.26.2 by @renovate[bot] in [#1542](https://github.com/nicholas-fedor/watchtower/pull/1542)
- Update nicholas-fedor/go-proxy-pull-action digest to 6568a55 by @renovate[bot] in [#1541](https://github.com/nicholas-fedor/watchtower/pull/1541)
- Update golang:alpine docker digest to c2a1f7b by @renovate[bot] in [#1540](https://github.com/nicholas-fedor/watchtower/pull/1540)
- Update dependency go to v1.26.2 by @renovate[bot] in [#1539](https://github.com/nicholas-fedor/watchtower/pull/1539)
- Update docker/dockerfile:1 docker digest to 2780b5c by @renovate[bot] in [#1538](https://github.com/nicholas-fedor/watchtower/pull/1538)
- Update module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp to v0.68.0 by @renovate[bot] in [#1537](https://github.com/nicholas-fedor/watchtower/pull/1537)
- Update module github.com/docker/cli to v29.4.0+incompatible by @renovate[bot] in [#1536](https://github.com/nicholas-fedor/watchtower/pull/1536)
- Update docker/login-action digest to ba75415 by @renovate[bot] in [#1535](https://github.com/nicholas-fedor/watchtower/pull/1535)
- Update goreleaser/goreleaser-action digest to 01cbe07 by @renovate[bot] in [#1534](https://github.com/nicholas-fedor/watchtower/pull/1534)
- Update goreleaser/goreleaser-action digest to 2a473d7 by @renovate[bot] in [#1531](https://github.com/nicholas-fedor/watchtower/pull/1531)
- Update module go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp to v1.43.0 by @renovate[bot] in [#1530](https://github.com/nicholas-fedor/watchtower/pull/1530)
- Update module go.opentelemetry.io/otel/metric to v1.43.0 by @renovate[bot] in [#1529](https://github.com/nicholas-fedor/watchtower/pull/1529)
- Update docker/login-action digest to 4907a6d by @renovate[bot] in [#1526](https://github.com/nicholas-fedor/watchtower/pull/1526)
- Update github.com/google/pprof digest to 545e8a4 by @renovate[bot] in [#1525](https://github.com/nicholas-fedor/watchtower/pull/1525)
- Update peter-evans/create-pull-request digest to d32e88d by @renovate[bot] in [#1524](https://github.com/nicholas-fedor/watchtower/pull/1524)
- Update golangci/golangci-lint-action digest to 36fe29c by @renovate[bot] in [#1523](https://github.com/nicholas-fedor/watchtower/pull/1523)
- Update docker/setup-qemu-action digest to f4e8dee by @renovate[bot] in [#1522](https://github.com/nicholas-fedor/watchtower/pull/1522)
- Update docker/setup-buildx-action digest to 2116288 by @renovate[bot] in [#1521](https://github.com/nicholas-fedor/watchtower/pull/1521)
- Update docker/login-action digest to bb555fc by @renovate[bot] in [#1520](https://github.com/nicholas-fedor/watchtower/pull/1520)
- Update module google.golang.org/grpc to v1.80.0 by @renovate[bot] in [#1518](https://github.com/nicholas-fedor/watchtower/pull/1518)
- Update google.golang.org/genproto/googleapis/rpc digest to 9d38bb4 by @renovate[bot] in [#1517](https://github.com/nicholas-fedor/watchtower/pull/1517)
- Update google.golang.org/genproto/googleapis/api digest to 9d38bb4 by @renovate[bot] in [#1516](https://github.com/nicholas-fedor/watchtower/pull/1516)

## [1.16.1] - 2026-04-01

### Fixed

- Skip registry requests for locally built images by @nicholas-fedor in [#1514](https://github.com/nicholas-fedor/watchtower/pull/1514)

### Chores

- Update google.golang.org/genproto/googleapis/rpc digest to 3a24fdc by @renovate[bot] in [#1515](https://github.com/nicholas-fedor/watchtower/pull/1515)
- Update module github.com/nicholas-fedor/shoutrrr to v0.14.3 by @renovate[bot] in [#1513](https://github.com/nicholas-fedor/watchtower/pull/1513)
- Update google.golang.org/genproto/googleapis/rpc digest to f93e5f3 by @renovate[bot] in [#1511](https://github.com/nicholas-fedor/watchtower/pull/1511)
- Update google.golang.org/genproto/googleapis/api digest to f93e5f3 by @renovate[bot] in [#1510](https://github.com/nicholas-fedor/watchtower/pull/1510)

## [1.16.0] - 2026-03-31

### Added

- Overhaul HTTP API with security hardening and rate limiting by @nicholas-fedor in [#1505](https://github.com/nicholas-fedor/watchtower/pull/1505)
- Add image cooldown supply-chain defense mechanism by @nicholas-fedor in [#1495](https://github.com/nicholas-fedor/watchtower/pull/1495)
- Add ephemeral self-update capability by @nicholas-fedor in [#1491](https://github.com/nicholas-fedor/watchtower/pull/1491)

### Fixed

- Correct self-update and add skipped container tracking in HTTP API by @nicholas-fedor in [#1506](https://github.com/nicholas-fedor/watchtower/pull/1506)
- Retry transient Docker daemon connection failures during container listing by @nicholas-fedor in [#1492](https://github.com/nicholas-fedor/watchtower/pull/1492)
- Deduplicate image removal and grouped notification entries by @nicholas-fedor in [#1483](https://github.com/nicholas-fedor/watchtower/pull/1483)
- Make Watchtower cleanup non-fatal after self-update by @nicholas-fedor in [#1482](https://github.com/nicholas-fedor/watchtower/pull/1482)
- Skip self-update when Watchtower has host-bound ports by @nicholas-fedor in [#1481](https://github.com/nicholas-fedor/watchtower/pull/1481)
- Prefer Watchtower-labeled container when multiple share hostname by @nicholas-fedor in [#1480](https://github.com/nicholas-fedor/watchtower/pull/1480)
- Replace global mutex with per-image keyed locks by @nicholas-fedor in [#1479](https://github.com/nicholas-fedor/watchtower/pull/1479)
- Validate port bindings before ContainerCreate by @nicholas-fedor in [#1478](https://github.com/nicholas-fedor/watchtower/pull/1478)
- Differentiate auth, not-found, and transient errors during image pull by @nicholas-fedor in [#1477](https://github.com/nicholas-fedor/watchtower/pull/1477)
- Replace fragile string-based error checking with typed errors by @nicholas-fedor in [#1476](https://github.com/nicholas-fedor/watchtower/pull/1476)
- Handle container removed between check and stop by @nicholas-fedor in [#1475](https://github.com/nicholas-fedor/watchtower/pull/1475)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1471](https://github.com/nicholas-fedor/watchtower/pull/1471)
- Update changelog for main by @nicholas-fedor in [#1468](https://github.com/nicholas-fedor/watchtower/pull/1468)

### Tests

- Reorganize container tests into dedicated files by @nicholas-fedor in [#1484](https://github.com/nicholas-fedor/watchtower/pull/1484)

### Chores

- Update step-security/harden-runner action to v2.16.1 by @renovate[bot] in [#1507](https://github.com/nicholas-fedor/watchtower/pull/1507)
- Update module github.com/nicholas-fedor/shoutrrr to v0.14.2 by @renovate[bot] in [#1504](https://github.com/nicholas-fedor/watchtower/pull/1504)
- Update docker/setup-qemu-action digest to 6412e4f by @renovate[bot] in [#1503](https://github.com/nicholas-fedor/watchtower/pull/1503)
- Update docker/setup-buildx-action digest to e35beed by @renovate[bot] in [#1502](https://github.com/nicholas-fedor/watchtower/pull/1502)
- Update docker/login-action digest to de05a6d by @renovate[bot] in [#1501](https://github.com/nicholas-fedor/watchtower/pull/1501)
- Update docker/setup-buildx-action digest to dae0651 by @renovate[bot] in [#1500](https://github.com/nicholas-fedor/watchtower/pull/1500)
- Update docker/login-action digest to bb9683d by @renovate[bot] in [#1499](https://github.com/nicholas-fedor/watchtower/pull/1499)
- Update google.golang.org/genproto/googleapis/rpc digest to d5a96ad by @renovate[bot] in [#1498](https://github.com/nicholas-fedor/watchtower/pull/1498)
- Update google.golang.org/genproto/googleapis/api digest to d5a96ad by @renovate[bot] in [#1497](https://github.com/nicholas-fedor/watchtower/pull/1497)
- Update google.golang.org/genproto/googleapis/rpc digest to b2ae96c by @renovate[bot] in [#1496](https://github.com/nicholas-fedor/watchtower/pull/1496)
- Update docker/login-action digest to 5c42dd2 by @renovate[bot] in [#1494](https://github.com/nicholas-fedor/watchtower/pull/1494)
- Update crazy-max/ghaction-import-gpg digest to 1c06494 by @renovate[bot] in [#1493](https://github.com/nicholas-fedor/watchtower/pull/1493)
- Update crazy-max/ghaction-import-gpg digest to da46d52 by @renovate[bot] in [#1489](https://github.com/nicholas-fedor/watchtower/pull/1489)
- Update peter-evans/create-pull-request digest to 8170bcc by @renovate[bot] in [#1488](https://github.com/nicholas-fedor/watchtower/pull/1488)
- Update github/codeql-action digest to c10b806 by @renovate[bot] in [#1487](https://github.com/nicholas-fedor/watchtower/pull/1487)
- Update github/codeql-action digest to b8bb9f2 by @renovate[bot] in [#1486](https://github.com/nicholas-fedor/watchtower/pull/1486)
- Update golangci/golangci-lint-action digest to 2d7e7b6 by @renovate[bot] in [#1474](https://github.com/nicholas-fedor/watchtower/pull/1474)
- Update codecov/codecov-action digest to 57e3a13 by @renovate[bot] in [#1473](https://github.com/nicholas-fedor/watchtower/pull/1473)
- Update module github.com/docker/cli to v29.3.1+incompatible by @renovate[bot] in [#1472](https://github.com/nicholas-fedor/watchtower/pull/1472)
- Update peter-evans/create-pull-request digest to 0041819 by @renovate[bot] in [#1470](https://github.com/nicholas-fedor/watchtower/pull/1470)
- Update docker/setup-qemu-action digest to 6804d31 by @renovate[bot] in [#1469](https://github.com/nicholas-fedor/watchtower/pull/1469)
- Update docker/setup-buildx-action digest to 172dff0 by @renovate[bot] in [#1467](https://github.com/nicholas-fedor/watchtower/pull/1467)
- Update docker/login-action digest to a0d57b8 by @renovate[bot] in [#1466](https://github.com/nicholas-fedor/watchtower/pull/1466)

## [1.15.0] - 2026-03-24

### Added

- Add option to disable Docker Compose depends_on dependency detection by @nicholas-fedor in [#1457](https://github.com/nicholas-fedor/watchtower/pull/1457)

### Fixed

- Resolve type mismatch with shoutrrr API change by @nicholas-fedor in [#1463](https://github.com/nicholas-fedor/watchtower/pull/1463)
- Prevent cleanup of images actively used by active containers by @LJspice in [#1427](https://github.com/nicholas-fedor/watchtower/pull/1427)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1426](https://github.com/nicholas-fedor/watchtower/pull/1426)
- Add LJspice as a contributor for code by @allcontributors[bot] in [#1449](https://github.com/nicholas-fedor/watchtower/pull/1449)

### Continuous Integration

- Onboard StepSecurity by @stepsecurity-app[bot] in [#1456](https://github.com/nicholas-fedor/watchtower/pull/1456)
- Enable updates for transitive go dependencies by @nicholas-fedor in [#1428](https://github.com/nicholas-fedor/watchtower/pull/1428)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.14.1 by @renovate[bot] in [#1462](https://github.com/nicholas-fedor/watchtower/pull/1462)
- Update module github.com/pelletier/go-toml/v2 to v2.3.0 by @renovate[bot] in [#1461](https://github.com/nicholas-fedor/watchtower/pull/1461)
- Update docker/login-action digest to 292fe2d by @renovate[bot] in [#1460](https://github.com/nicholas-fedor/watchtower/pull/1460)
- Update goreleaser/goreleaser-action digest to fdcf0b9 by @renovate[bot] in [#1459](https://github.com/nicholas-fedor/watchtower/pull/1459)
- Update module go.yaml.in/yaml/v2 to v2.4.4 by @renovate[bot] in [#1440](https://github.com/nicholas-fedor/watchtower/pull/1440)
- Update golangci/golangci-lint-action digest to e94e72c by @renovate[bot] in [#1458](https://github.com/nicholas-fedor/watchtower/pull/1458)
- Update github/codeql-action digest to 3869755 by @renovate[bot] in [#1455](https://github.com/nicholas-fedor/watchtower/pull/1455)
- Update docker/setup-qemu-action digest to 6632d37 by @renovate[bot] in [#1454](https://github.com/nicholas-fedor/watchtower/pull/1454)
- Update docker/setup-buildx-action digest to d91f340 by @renovate[bot] in [#1453](https://github.com/nicholas-fedor/watchtower/pull/1453)
- Update docker/login-action digest to da5b89b by @renovate[bot] in [#1452](https://github.com/nicholas-fedor/watchtower/pull/1452)
- Update github/codeql-action digest to c6f9311 by @renovate[bot] in [#1450](https://github.com/nicholas-fedor/watchtower/pull/1450)
- Update golangci/golangci-lint-action digest to b269f19 by @renovate[bot] in [#1448](https://github.com/nicholas-fedor/watchtower/pull/1448)
- Update peter-evans/create-pull-request digest to b993918 by @renovate[bot] in [#1447](https://github.com/nicholas-fedor/watchtower/pull/1447)
- Update google.golang.org/genproto/googleapis/rpc digest to d00831a by @renovate[bot] in [#1444](https://github.com/nicholas-fedor/watchtower/pull/1444)
- Update golangci/golangci-lint-action digest to fa2a845 by @renovate[bot] in [#1446](https://github.com/nicholas-fedor/watchtower/pull/1446)
- Update google.golang.org/genproto/googleapis/api digest to d00831a by @renovate[bot] in [#1445](https://github.com/nicholas-fedor/watchtower/pull/1445)
- Update google.golang.org/genproto/googleapis/api digest to cd36c79 by @renovate[bot] in [#1443](https://github.com/nicholas-fedor/watchtower/pull/1443)
- Update securego/gosec action to v2.25.0 by @renovate[bot] in [#1442](https://github.com/nicholas-fedor/watchtower/pull/1442)
- Update module go.yaml.in/yaml/v2 to v3 by @renovate[bot] in [#1439](https://github.com/nicholas-fedor/watchtower/pull/1439)
- Update codecov/codecov-action digest to 1af5884 by @renovate[bot] in [#1438](https://github.com/nicholas-fedor/watchtower/pull/1438)
- Update module go.yaml.in/yaml/v2 to v2.4.4 by @renovate[bot] in [#1436](https://github.com/nicholas-fedor/watchtower/pull/1436)
- Update golangci/golangci-lint-action digest to 2bcbc9e by @renovate[bot] in [#1435](https://github.com/nicholas-fedor/watchtower/pull/1435)
- Update module go.yaml.in/yaml/v2 to v3 by @renovate[bot] in [#1434](https://github.com/nicholas-fedor/watchtower/pull/1434)
- Update module golang.org/x/tools to v0.43.0 by @renovate[bot] in [#1433](https://github.com/nicholas-fedor/watchtower/pull/1433)
- Update module golang.org/x/net to v0.52.0 by @renovate[bot] in [#1432](https://github.com/nicholas-fedor/watchtower/pull/1432)
- Update actions/cache digest to 6682284 by @renovate[bot] in [#1431](https://github.com/nicholas-fedor/watchtower/pull/1431)
- Update google.golang.org/genproto/googleapis/rpc digest to 0b37fe3 by @renovate[bot] in [#1430](https://github.com/nicholas-fedor/watchtower/pull/1430)
- Update google.golang.org/genproto/googleapis/api digest to 0b37fe3 by @renovate[bot] in [#1429](https://github.com/nicholas-fedor/watchtower/pull/1429)
- Update nicholas-fedor/govulncheck-action digest to b438bbb by @renovate[bot] in [#1425](https://github.com/nicholas-fedor/watchtower/pull/1425)

### New Contributors

- @stepsecurity-app[bot] made their first contribution in [#1456](https://github.com/nicholas-fedor/watchtower/pull/1456)
- @LJspice made their first contribution in [#1427](https://github.com/nicholas-fedor/watchtower/pull/1427)

## [1.14.4] - 2026-03-17

### Fixed

- Improve graceful shutdown with context cancellation by @nicholas-fedor in [#1408](https://github.com/nicholas-fedor/watchtower/pull/1408)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1424](https://github.com/nicholas-fedor/watchtower/pull/1424)
- Update changelog for main by @nicholas-fedor in [#1382](https://github.com/nicholas-fedor/watchtower/pull/1382)
- Document regex pattern matching support by @nicholas-fedor in [#1404](https://github.com/nicholas-fedor/watchtower/pull/1404)
- Update changelog for main by @nicholas-fedor in [#1380](https://github.com/nicholas-fedor/watchtower/pull/1380)

### Continuous Integration

- Refactor and cleanup security workflow by @nicholas-fedor in [#1397](https://github.com/nicholas-fedor/watchtower/pull/1397)

### Chores

- Update actions/setup-go digest to 4a36011 by @renovate[bot] in [#1423](https://github.com/nicholas-fedor/watchtower/pull/1423)
- Update docker/dockerfile:1 docker digest to 4a43a54 by @renovate[bot] in [#1422](https://github.com/nicholas-fedor/watchtower/pull/1422)
- Apply go modernize to use modern Go atomic types by @nicholas-fedor in [#1421](https://github.com/nicholas-fedor/watchtower/pull/1421)
- Update nicholas-fedor/govulncheck-action digest to 4878bd2 by @renovate[bot] in [#1420](https://github.com/nicholas-fedor/watchtower/pull/1420)
- Update github/codeql-action digest to b1bff81 by @renovate[bot] in [#1419](https://github.com/nicholas-fedor/watchtower/pull/1419)
- Update docker/setup-qemu-action digest to b99055d by @renovate[bot] in [#1418](https://github.com/nicholas-fedor/watchtower/pull/1418)
- Update actions/setup-go digest to 8f19afc by @renovate[bot] in [#1417](https://github.com/nicholas-fedor/watchtower/pull/1417)
- Update docker/setup-buildx-action digest to 8016837 by @renovate[bot] in [#1415](https://github.com/nicholas-fedor/watchtower/pull/1415)
- Update docker/login-action digest to c144859 by @renovate[bot] in [#1414](https://github.com/nicholas-fedor/watchtower/pull/1414)
- Update peter-evans/create-pull-request digest to 36d7c84 by @renovate[bot] in [#1412](https://github.com/nicholas-fedor/watchtower/pull/1412)
- Update OpenTelemetry and golang.org/x dependencies by @nicholas-fedor in [#1411](https://github.com/nicholas-fedor/watchtower/pull/1411)
- Update module golang.org/x/text to v0.35.0 by @renovate[bot] in [#1410](https://github.com/nicholas-fedor/watchtower/pull/1410)
- Update cimg/go:1.26.1 docker digest to ff658f9 by @renovate[bot] in [#1409](https://github.com/nicholas-fedor/watchtower/pull/1409)
- Update module github.com/nicholas-fedor/shoutrrr to v0.14.0 by @renovate[bot] in [#1407](https://github.com/nicholas-fedor/watchtower/pull/1407)
- Update cimg/go:1.26.1 docker digest to d6efbd2 by @renovate[bot] in [#1406](https://github.com/nicholas-fedor/watchtower/pull/1406)
- Update cimg/go docker tag to v1.26.1 by @renovate[bot] in [#1405](https://github.com/nicholas-fedor/watchtower/pull/1405)
- Update docker/setup-qemu-action digest to a4bc6cd by @renovate[bot] in [#1401](https://github.com/nicholas-fedor/watchtower/pull/1401)
- Update docker/setup-buildx-action digest to c8ad1c5 by @renovate[bot] in [#1400](https://github.com/nicholas-fedor/watchtower/pull/1400)
- Update docker/login-action digest to 9fe7774 by @renovate[bot] in [#1399](https://github.com/nicholas-fedor/watchtower/pull/1399)
- Update actions/setup-python digest to 28f2168 by @renovate[bot] in [#1398](https://github.com/nicholas-fedor/watchtower/pull/1398)
- Update nicholas-fedor/govulncheck-action digest to ac1aadb by @renovate[bot] in [#1396](https://github.com/nicholas-fedor/watchtower/pull/1396)
- Update docker/setup-buildx-action digest to 8f54c6f by @renovate[bot] in [#1395](https://github.com/nicholas-fedor/watchtower/pull/1395)
- Update golang docker tag to v1.26.1 by @renovate[bot] in [#1394](https://github.com/nicholas-fedor/watchtower/pull/1394)
- Update dependency go to v1.26.1 by @renovate[bot] in [#1393](https://github.com/nicholas-fedor/watchtower/pull/1393)
- Update nicholas-fedor/go-proxy-pull-action digest to 66b03fb by @renovate[bot] in [#1392](https://github.com/nicholas-fedor/watchtower/pull/1392)
- Update golang:alpine docker digest to 2389ebf by @renovate[bot] in [#1391](https://github.com/nicholas-fedor/watchtower/pull/1391)
- Update module github.com/docker/cli to v29.3.0+incompatible by @renovate[bot] in [#1390](https://github.com/nicholas-fedor/watchtower/pull/1390)
- Update github/codeql-action digest to 0d579ff by @renovate[bot] in [#1389](https://github.com/nicholas-fedor/watchtower/pull/1389)
- Update docker/login-action digest to db14339 by @renovate[bot] in [#1388](https://github.com/nicholas-fedor/watchtower/pull/1388)
- Update peter-evans/create-pull-request digest to a45d1fb by @renovate[bot] in [#1387](https://github.com/nicholas-fedor/watchtower/pull/1387)
- Update docker/setup-qemu-action digest to 72cd565 by @renovate[bot] in [#1386](https://github.com/nicholas-fedor/watchtower/pull/1386)
- Update docker/setup-buildx-action digest to 2ae358d by @renovate[bot] in [#1385](https://github.com/nicholas-fedor/watchtower/pull/1385)
- Update docker/login-action digest to e46b7e3 by @renovate[bot] in [#1384](https://github.com/nicholas-fedor/watchtower/pull/1384)
- Update docker/setup-buildx-action digest to 28a438e by @renovate[bot] in [#1383](https://github.com/nicholas-fedor/watchtower/pull/1383)
- Update actions/attest-build-provenance digest to 10334b5 by @renovate[bot] in [#1381](https://github.com/nicholas-fedor/watchtower/pull/1381)
- Update docker/setup-qemu-action digest to ce36039 by @renovate[bot] in [#1379](https://github.com/nicholas-fedor/watchtower/pull/1379)

## [1.14.3] - 2026-03-04

### Fixed

- Resolve gosec SARIF validation error by @nicholas-fedor in [#1374](https://github.com/nicholas-fedor/watchtower/pull/1374)
- Return HTTP 429 for full updates when another update is running by @veeceey in [#1304](https://github.com/nicholas-fedor/watchtower/pull/1304)
- Propagate context throughout watchtower for improved cancellation and timeout handling by @nicholas-fedor in [#1335](https://github.com/nicholas-fedor/watchtower/pull/1335)
- Log container startup failures at warning level by @nicholas-fedor in [#1332](https://github.com/nicholas-fedor/watchtower/pull/1332)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1375](https://github.com/nicholas-fedor/watchtower/pull/1375)
- Update changelog for main by @nicholas-fedor in [#1345](https://github.com/nicholas-fedor/watchtower/pull/1345)
- Update changelog for main by @nicholas-fedor in [#1282](https://github.com/nicholas-fedor/watchtower/pull/1282)
- Add veeceey as a contributor for code by @allcontributors[bot] in [#1341](https://github.com/nicholas-fedor/watchtower/pull/1341)
- Update api documentation with additional details by @nicholas-fedor in [#1342](https://github.com/nicholas-fedor/watchtower/pull/1342)

### Continuous Integration

- Resolve gosec SARIF validation error by @nicholas-fedor in [#1376](https://github.com/nicholas-fedor/watchtower/pull/1376)

### Chores

- Update transitive dependencies by @nicholas-fedor in [#1373](https://github.com/nicholas-fedor/watchtower/pull/1373)
- Update docker/setup-buildx-action digest to 9cd4410 by @renovate[bot] in [#1372](https://github.com/nicholas-fedor/watchtower/pull/1372)
- Update docker/login-action digest to b45d80f by @renovate[bot] in [#1371](https://github.com/nicholas-fedor/watchtower/pull/1371)
- Update docker/login-action digest to cad8984 by @renovate[bot] in [#1370](https://github.com/nicholas-fedor/watchtower/pull/1370)
- Update docker/setup-buildx-action digest to 1282d41 by @renovate[bot] in [#1368](https://github.com/nicholas-fedor/watchtower/pull/1368)
- Update golangci/golangci-lint-action digest to b7bcab6 by @renovate[bot] in [#1367](https://github.com/nicholas-fedor/watchtower/pull/1367)
- Update docker/setup-qemu-action digest to 1ea3db7 by @renovate[bot] in [#1366](https://github.com/nicholas-fedor/watchtower/pull/1366)
- Update nicholas-fedor/govulncheck-action digest to 1ffd170 by @renovate[bot] in [#1365](https://github.com/nicholas-fedor/watchtower/pull/1365)
- Update actions/setup-go digest to 27fdb26 by @renovate[bot] in [#1364](https://github.com/nicholas-fedor/watchtower/pull/1364)
- Update github/codeql-action digest to c793b71 by @renovate[bot] in [#1363](https://github.com/nicholas-fedor/watchtower/pull/1363)
- Update crazy-max/ghaction-import-gpg digest to 92a10f9 by @renovate[bot] in [#1362](https://github.com/nicholas-fedor/watchtower/pull/1362)
- Update securego/gosec action to v2.24.7 by @renovate[bot] in [#1361](https://github.com/nicholas-fedor/watchtower/pull/1361)
- Update peter-evans/create-pull-request digest to 3499eb6 by @renovate[bot] in [#1360](https://github.com/nicholas-fedor/watchtower/pull/1360)
- Update goreleaser/goreleaser-action digest to 4be059c by @renovate[bot] in [#1359](https://github.com/nicholas-fedor/watchtower/pull/1359)
- Update securego/gosec action to v2.24.6 by @renovate[bot] in [#1358](https://github.com/nicholas-fedor/watchtower/pull/1358)
- Update securego/gosec action to v2.24.4 by @renovate[bot] in [#1357](https://github.com/nicholas-fedor/watchtower/pull/1357)
- Update peter-evans/create-pull-request digest to 3f3b473 by @renovate[bot] in [#1356](https://github.com/nicholas-fedor/watchtower/pull/1356)
- Update golangci/golangci-lint-action digest to b207e52 by @renovate[bot] in [#1355](https://github.com/nicholas-fedor/watchtower/pull/1355)
- Update actions/attest-build-provenance digest to c5efebd by @renovate[bot] in [#1353](https://github.com/nicholas-fedor/watchtower/pull/1353)
- Update securego/gosec action to v2.24.0 by @renovate[bot] in [#1352](https://github.com/nicholas-fedor/watchtower/pull/1352)
- Update actions/attest-build-provenance digest to a2bbfa2 by @renovate[bot] in [#1351](https://github.com/nicholas-fedor/watchtower/pull/1351)
- Update nicholas-fedor/govulncheck-action digest to c6b69a0 by @renovate[bot] in [#1350](https://github.com/nicholas-fedor/watchtower/pull/1350)
- Update actions/setup-go digest to def8c39 by @renovate[bot] in [#1349](https://github.com/nicholas-fedor/watchtower/pull/1349)
- Update actions/attest-build-provenance digest to 0856891 by @renovate[bot] in [#1348](https://github.com/nicholas-fedor/watchtower/pull/1348)
- Update nicholas-fedor/govulncheck-action digest to 15fce97 by @renovate[bot] in [#1347](https://github.com/nicholas-fedor/watchtower/pull/1347)
- Update actions/setup-go digest to 4b73464 by @renovate[bot] in [#1346](https://github.com/nicholas-fedor/watchtower/pull/1346)
- Update indirect Go dependencies by @nicholas-fedor in [#1343](https://github.com/nicholas-fedor/watchtower/pull/1343)
- Update golangci/golangci-lint-action digest to 02d66c3 by @renovate[bot] in [#1344](https://github.com/nicholas-fedor/watchtower/pull/1344)
- Update cimg/go:1.26.0 docker digest to e82c772 by @renovate[bot] in [#1340](https://github.com/nicholas-fedor/watchtower/pull/1340)
- Update goreleaser/goreleaser-action digest to 6c92f1d by @renovate[bot] in [#1339](https://github.com/nicholas-fedor/watchtower/pull/1339)
- Update goreleaser/goreleaser-action digest to ff4cb9c by @renovate[bot] in [#1338](https://github.com/nicholas-fedor/watchtower/pull/1338)
- Update orhun/git-cliff-action digest to c93ef52 by @renovate[bot] in [#1336](https://github.com/nicholas-fedor/watchtower/pull/1336)
- Update github/codeql-action digest to 89a39a4 by @renovate[bot] in [#1334](https://github.com/nicholas-fedor/watchtower/pull/1334)
- Resolve golangci-lint false positives by @nicholas-fedor in [#1331](https://github.com/nicholas-fedor/watchtower/pull/1331)
- Update golangci/golangci-lint-action digest to 17a5bf4 by @renovate[bot] in [#1328](https://github.com/nicholas-fedor/watchtower/pull/1328)

### New Contributors

- @veeceey made their first contribution in [#1304](https://github.com/nicholas-fedor/watchtower/pull/1304)

## [1.14.2] - 2026-02-17

### Fixed

- Add service-only matching for cross-project dependencies by @nicholas-fedor in [#1307](https://github.com/nicholas-fedor/watchtower/pull/1307)

### Performance

- Cache HTTP client for registry authentication requests by @nicholas-fedor in [#1287](https://github.com/nicholas-fedor/watchtower/pull/1287)

### Changed

- Improve digest matching efficiency and reliability by @nicholas-fedor in [#1289](https://github.com/nicholas-fedor/watchtower/pull/1289)

### Tests

- Update DNS error pattern in digest test by @nicholas-fedor in [#1324](https://github.com/nicholas-fedor/watchtower/pull/1324)

### Build

- Exclude unsupported architectures from goreleaser configurations by @nicholas-fedor in [#1326](https://github.com/nicholas-fedor/watchtower/pull/1326)

### Continuous Integration

- Update bug report template by @nicholas-fedor in [#1305](https://github.com/nicholas-fedor/watchtower/pull/1305)

### Chores

- Update golangci-lint config and format test files by @nicholas-fedor in [#1323](https://github.com/nicholas-fedor/watchtower/pull/1323)
- Update golangci/golangci-lint-action digest to fce8c98 by @renovate[bot] in [#1322](https://github.com/nicholas-fedor/watchtower/pull/1322)
- Update crazy-max/ghaction-import-gpg digest to 5a30dd9 by @renovate[bot] in [#1320](https://github.com/nicholas-fedor/watchtower/pull/1320)
- Update crazy-max/ghaction-import-gpg digest to ad49e30 by @renovate[bot] in [#1319](https://github.com/nicholas-fedor/watchtower/pull/1319)
- Update actions/attest-build-provenance digest to 02a49bd by @renovate[bot] in [#1316](https://github.com/nicholas-fedor/watchtower/pull/1316)
- Update github/codeql-action digest to 9e907b5 by @renovate[bot] in [#1315](https://github.com/nicholas-fedor/watchtower/pull/1315)
- Update cimg/go docker tag to v1.26.0 by @renovate[bot] in [#1313](https://github.com/nicholas-fedor/watchtower/pull/1313)
- Update securego/gosec action to v2.23.0 by @renovate[bot] in [#1312](https://github.com/nicholas-fedor/watchtower/pull/1312)
- Update nicholas-fedor/go-proxy-pull-action digest to 95b3e6c by @renovate[bot] in [#1311](https://github.com/nicholas-fedor/watchtower/pull/1311)
- Update golang docker tag to v1.26.0 by @renovate[bot] in [#1310](https://github.com/nicholas-fedor/watchtower/pull/1310)
- Update dependency go to v1.26.0 by @renovate[bot] in [#1309](https://github.com/nicholas-fedor/watchtower/pull/1309)
- Update golang:alpine docker digest to d4c4845 by @renovate[bot] in [#1308](https://github.com/nicholas-fedor/watchtower/pull/1308)
- Update module golang.org/x/text to v0.34.0 by @renovate[bot] in [#1303](https://github.com/nicholas-fedor/watchtower/pull/1303)
- Update goreleaser/goreleaser-action digest to ec59f47 by @renovate[bot] in [#1302](https://github.com/nicholas-fedor/watchtower/pull/1302)
- Delete FUNDING.yml by @nicholas-fedor in [#1298](https://github.com/nicholas-fedor/watchtower/pull/1298)
- Add FUNDING.yml by @nicholas-fedor in [#1297](https://github.com/nicholas-fedor/watchtower/pull/1297)
- Update nicholas-fedor/go-proxy-pull-action digest to f0551db by @renovate[bot] in [#1296](https://github.com/nicholas-fedor/watchtower/pull/1296)
- Update golang:1.25.7-alpine3.22 docker digest to 20c8a94 by @renovate[bot] in [#1295](https://github.com/nicholas-fedor/watchtower/pull/1295)
- Update golang:alpine docker digest to f6751d8 by @renovate[bot] in [#1294](https://github.com/nicholas-fedor/watchtower/pull/1294)
- Update github/codeql-action digest to 45cbd0c by @renovate[bot] in [#1293](https://github.com/nicholas-fedor/watchtower/pull/1293)
- Update cimg/go docker tag to v1.25.7 by @renovate[bot] in [#1291](https://github.com/nicholas-fedor/watchtower/pull/1291)
- Update nicholas-fedor/go-proxy-pull-action digest to 9c51cce by @renovate[bot] in [#1288](https://github.com/nicholas-fedor/watchtower/pull/1288)
- Update golang docker tag to v1.25.7 by @renovate[bot] in [#1286](https://github.com/nicholas-fedor/watchtower/pull/1286)
- Update golang:alpine docker digest to f4622e3 by @renovate[bot] in [#1285](https://github.com/nicholas-fedor/watchtower/pull/1285)
- Update dependency go to v1.25.7 by @renovate[bot] in [#1284](https://github.com/nicholas-fedor/watchtower/pull/1284)
- Update nicholas-fedor/go-proxy-pull-action digest to 998d963 by @renovate[bot] in [#1283](https://github.com/nicholas-fedor/watchtower/pull/1283)
- Update cimg/go:1.25.6 docker digest to 81789fa by @renovate[bot] in [#1281](https://github.com/nicholas-fedor/watchtower/pull/1281)
- Update actions/checkout digest to de0fac2 by @renovate[bot] in [#1280](https://github.com/nicholas-fedor/watchtower/pull/1280)

## [1.14.1] - 2026-02-03

### Fixed

- Add space between notification fields by removing whitespace-trimming in template by @nicholas-fedor in [#1279](https://github.com/nicholas-fedor/watchtower/pull/1279)
- Prevent spurious safeguard delay when SkipSelfUpdate is enabled by @nicholas-fedor in [#1276](https://github.com/nicholas-fedor/watchtower/pull/1276)
- Improve dependency resolution and add error notifications by @nicholas-fedor in [#1265](https://github.com/nicholas-fedor/watchtower/pull/1265)

### Changed

- Extract getRawLabelValue to eliminate code duplication by @nicholas-fedor in [#1277](https://github.com/nicholas-fedor/watchtower/pull/1277)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1227](https://github.com/nicholas-fedor/watchtower/pull/1227)
- Update documentation for multiple notification URLs and CLI flags by @nicholas-fedor in [#1251](https://github.com/nicholas-fedor/watchtower/pull/1251)
- Update changelog for main by @nicholas-fedor in [#1224](https://github.com/nicholas-fedor/watchtower/pull/1224)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.13.2 by @renovate[bot] in [#1275](https://github.com/nicholas-fedor/watchtower/pull/1275)
- Update module github.com/docker/cli to v29.2.1+incompatible by @renovate[bot] in [#1274](https://github.com/nicholas-fedor/watchtower/pull/1274)
- Update github/codeql-action digest to 6bc82e0 by @renovate[bot] in [#1273](https://github.com/nicholas-fedor/watchtower/pull/1273)
- Update actions/attest-build-provenance digest to c44148e by @renovate[bot] in [#1272](https://github.com/nicholas-fedor/watchtower/pull/1272)
- Update goreleaser/goreleaser-action digest to 4247c53 by @renovate[bot] in [#1270](https://github.com/nicholas-fedor/watchtower/pull/1270)
- Update golangci/golangci-lint-action digest to b62bd5d by @renovate[bot] in [#1269](https://github.com/nicholas-fedor/watchtower/pull/1269)
- Update peter-evans/create-pull-request digest to 6699836 by @renovate[bot] in [#1268](https://github.com/nicholas-fedor/watchtower/pull/1268)
- Update golangci/golangci-lint-action digest to d6deb2e by @renovate[bot] in [#1264](https://github.com/nicholas-fedor/watchtower/pull/1264)
- Update module github.com/onsi/ginkgo/v2 to v2.28.1 by @renovate[bot] in [#1262](https://github.com/nicholas-fedor/watchtower/pull/1262)
- Update docker/login-action digest to 3227f53 by @renovate[bot] in [#1263](https://github.com/nicholas-fedor/watchtower/pull/1263)
- Update module github.com/onsi/gomega to v1.39.1 by @renovate[bot] in [#1261](https://github.com/nicholas-fedor/watchtower/pull/1261)
- Update nicholas-fedor/go-proxy-pull-action digest to a35ee0c by @renovate[bot] in [#1260](https://github.com/nicholas-fedor/watchtower/pull/1260)
- Update goreleaser/goreleaser-action digest to 902ab4a by @renovate[bot] in [#1259](https://github.com/nicholas-fedor/watchtower/pull/1259)
- Update golang:alpine docker digest to 98e6cff by @renovate[bot] in [#1258](https://github.com/nicholas-fedor/watchtower/pull/1258)
- Update golang:1.25.6-alpine3.22 docker digest to fa3380a by @renovate[bot] in [#1257](https://github.com/nicholas-fedor/watchtower/pull/1257)
- Update docker/setup-qemu-action digest to 7e10951 by @renovate[bot] in [#1256](https://github.com/nicholas-fedor/watchtower/pull/1256)
- Update actions/attest-build-provenance digest to 18db129 by @renovate[bot] in [#1255](https://github.com/nicholas-fedor/watchtower/pull/1255)
- Update docker/setup-buildx-action digest to 7c525be by @renovate[bot] in [#1254](https://github.com/nicholas-fedor/watchtower/pull/1254)
- Update docker/login-action digest to 2e1345c by @renovate[bot] in [#1253](https://github.com/nicholas-fedor/watchtower/pull/1253)
- Update actions/cache digest to cdf6c1f by @renovate[bot] in [#1252](https://github.com/nicholas-fedor/watchtower/pull/1252)
- Update alpine docker tag to v3.23.3 by @renovate[bot] in [#1249](https://github.com/nicholas-fedor/watchtower/pull/1249)
- Update nicholas-fedor/go-proxy-pull-action digest to 8689366 by @renovate[bot] in [#1248](https://github.com/nicholas-fedor/watchtower/pull/1248)
- Update golang:alpine docker digest to 660f0b8 by @renovate[bot] in [#1247](https://github.com/nicholas-fedor/watchtower/pull/1247)
- Update golang:1.25.6-alpine3.22 docker digest to 2dcdada by @renovate[bot] in [#1246](https://github.com/nicholas-fedor/watchtower/pull/1246)
- Update golang:1.25.6-alpine3.22 docker digest to ad295fc by @renovate[bot] in [#1245](https://github.com/nicholas-fedor/watchtower/pull/1245)
- Update actions/attest-build-provenance digest to 57db8ba by @renovate[bot] in [#1244](https://github.com/nicholas-fedor/watchtower/pull/1244)
- Update actions/attest-build-provenance digest to c4c3d11 by @renovate[bot] in [#1243](https://github.com/nicholas-fedor/watchtower/pull/1243)
- Update docker/login-action digest to c94ce9f by @renovate[bot] in [#1242](https://github.com/nicholas-fedor/watchtower/pull/1242)
- Update module github.com/docker/cli to v29.2.0+incompatible by @renovate[bot] in [#1241](https://github.com/nicholas-fedor/watchtower/pull/1241)
- Update nicholas-fedor/govulncheck-action digest to 5b70be9 by @renovate[bot] in [#1240](https://github.com/nicholas-fedor/watchtower/pull/1240)
- Update github/codeql-action digest to b20883b by @renovate[bot] in [#1239](https://github.com/nicholas-fedor/watchtower/pull/1239)
- Update actions/setup-go digest to a5f9b05 by @renovate[bot] in [#1238](https://github.com/nicholas-fedor/watchtower/pull/1238)
- Update actions/attest-build-provenance digest to 96278af by @renovate[bot] in [#1237](https://github.com/nicholas-fedor/watchtower/pull/1237)
- Update goreleaser/goreleaser-action digest to 4c34bd9 by @renovate[bot] in [#1236](https://github.com/nicholas-fedor/watchtower/pull/1236)
- Update golangci/golangci-lint-action digest to 2c963d3 by @renovate[bot] in [#1235](https://github.com/nicholas-fedor/watchtower/pull/1235)
- Update github/codeql-action digest to 19b2f06 by @renovate[bot] in [#1233](https://github.com/nicholas-fedor/watchtower/pull/1233)
- Update actions/setup-python digest to a309ff8 by @renovate[bot] in [#1231](https://github.com/nicholas-fedor/watchtower/pull/1231)
- Update grpc-gateway and genproto dependencies by @nicholas-fedor in [#1230](https://github.com/nicholas-fedor/watchtower/pull/1230)
- Gix formatting in bug issue template by @nicholas-fedor in [#1229](https://github.com/nicholas-fedor/watchtower/pull/1229)
- Enhance GitHub issue templates and configuration by @nicholas-fedor in [#1228](https://github.com/nicholas-fedor/watchtower/pull/1228)
- Update peter-evans/create-pull-request digest to c0f553f by @renovate[bot] in [#1226](https://github.com/nicholas-fedor/watchtower/pull/1226)
- Update nicholas-fedor/go-proxy-pull-action digest to c7a2ab4 by @renovate[bot] in [#1223](https://github.com/nicholas-fedor/watchtower/pull/1223)

## [1.14.0] - 2026-01-20

### Added

- Add support for file-based notification templates by @nicholas-fedor in [#1209](https://github.com/nicholas-fedor/watchtower/pull/1209)
- Auto-detect container stop timeout from Docker config by @skylenet in [#1182](https://github.com/nicholas-fedor/watchtower/pull/1182)

### Fixed

- Enhance transitive dependency restart for Docker Compose services by @nicholas-fedor in [#1211](https://github.com/nicholas-fedor/watchtower/pull/1211)
- Fix Watchtower self-update logging during safeguard period by @nicholas-fedor in [#1204](https://github.com/nicholas-fedor/watchtower/pull/1204)
- Resolve data race in concurrent container operations by @nicholas-fedor in [#1201](https://github.com/nicholas-fedor/watchtower/pull/1201)
- Correct self-update, container management, and dependency issues by @nicholas-fedor in [#1199](https://github.com/nicholas-fedor/watchtower/pull/1199)
- Respect cross-project dependencies in watchtower depends-on labels by @nicholas-fedor in [#1162](https://github.com/nicholas-fedor/watchtower/pull/1162)
- Lower log level for pinned image skip to debug by @nicholas-fedor in [#1160](https://github.com/nicholas-fedor/watchtower/pull/1160)
- Resolve self-update container cleanup issues by @nicholas-fedor in [#1159](https://github.com/nicholas-fedor/watchtower/pull/1159)
- Prevent panic in GetCreateConfig when containerInfo is nil by @nicholas-fedor in [#1147](https://github.com/nicholas-fedor/watchtower/pull/1147)

### Changed

- Update cron package to v3 with enhanced concurrency control by @nicholas-fedor in [#1208](https://github.com/nicholas-fedor/watchtower/pull/1208)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1186](https://github.com/nicholas-fedor/watchtower/pull/1186)
- Update changelog for main by @nicholas-fedor in [#1157](https://github.com/nicholas-fedor/watchtower/pull/1157)
- Add skylenet to contributor section by @nicholas-fedor in [#1183](https://github.com/nicholas-fedor/watchtower/pull/1183)
- Update changelog for main by @nicholas-fedor in [#1148](https://github.com/nicholas-fedor/watchtower/pull/1148)

### Chores

- Update actions/setup-python digest to bfe8cc5 by @renovate[bot] in [#1221](https://github.com/nicholas-fedor/watchtower/pull/1221)
- Update goreleaser/goreleaser-action digest to aacbb7f by @renovate[bot] in [#1220](https://github.com/nicholas-fedor/watchtower/pull/1220)
- Update golangci/golangci-lint-action digest to a3a03ee by @renovate[bot] in [#1219](https://github.com/nicholas-fedor/watchtower/pull/1219)
- Update golang:alpine docker digest to d9b2e14 by @renovate[bot] in [#1218](https://github.com/nicholas-fedor/watchtower/pull/1218)
- Update golang:1.25.6-alpine3.22 docker digest to d9c983d by @renovate[bot] in [#1217](https://github.com/nicholas-fedor/watchtower/pull/1217)
- Update cimg/go docker tag to v1.25.6 by @renovate[bot] in [#1215](https://github.com/nicholas-fedor/watchtower/pull/1215)
- Update actions/cache digest to 8b402f5 by @renovate[bot] in [#1214](https://github.com/nicholas-fedor/watchtower/pull/1214)
- Update actions/attest-build-provenance digest to 6865550 by @renovate[bot] in [#1213](https://github.com/nicholas-fedor/watchtower/pull/1213)
- Update module github.com/docker/cli to v29.1.5+incompatible by @renovate[bot] in [#1212](https://github.com/nicholas-fedor/watchtower/pull/1212)
- Update golang docker tag to v1.25.6 by @renovate[bot] in [#1207](https://github.com/nicholas-fedor/watchtower/pull/1207)
- Update golang:alpine docker digest to e689855 by @renovate[bot] in [#1206](https://github.com/nicholas-fedor/watchtower/pull/1206)
- Update dependency go to v1.25.6 by @renovate[bot] in [#1205](https://github.com/nicholas-fedor/watchtower/pull/1205)
- Update module github.com/sirupsen/logrus to v1.9.4 by @renovate[bot] in [#1203](https://github.com/nicholas-fedor/watchtower/pull/1203)
- Update peter-evans/create-pull-request digest to 34aa40e by @renovate[bot] in [#1202](https://github.com/nicholas-fedor/watchtower/pull/1202)
- Update peter-evans/create-pull-request digest to 641099d by @renovate[bot] in [#1200](https://github.com/nicholas-fedor/watchtower/pull/1200)
- Update cimg/go:1.25.5 docker digest to e88af54 by @renovate[bot] in [#1198](https://github.com/nicholas-fedor/watchtower/pull/1198)
- Update docker/login-action digest to 0567fa5 by @renovate[bot] in [#1197](https://github.com/nicholas-fedor/watchtower/pull/1197)
- Update nicholas-fedor/govulncheck-action digest to 72a75e9 by @renovate[bot] in [#1196](https://github.com/nicholas-fedor/watchtower/pull/1196)
- Update actions/setup-go digest to 7a3fe6c by @renovate[bot] in [#1195](https://github.com/nicholas-fedor/watchtower/pull/1195)
- Update module github.com/onsi/ginkgo/v2 to v2.27.5 by @renovate[bot] in [#1194](https://github.com/nicholas-fedor/watchtower/pull/1194)
- Update nicholas-fedor/govulncheck-action digest to 301b2fd by @renovate[bot] in [#1193](https://github.com/nicholas-fedor/watchtower/pull/1193)
- Update actions/setup-go digest to d73f6bc by @renovate[bot] in [#1192](https://github.com/nicholas-fedor/watchtower/pull/1192)
- Update golangci/golangci-lint-action digest to de73c35 by @renovate[bot] in [#1191](https://github.com/nicholas-fedor/watchtower/pull/1191)
- Update github/codeql-action digest to cdefb33 by @renovate[bot] in [#1190](https://github.com/nicholas-fedor/watchtower/pull/1190)
- Update module golang.org/x/text to v0.33.0 by @renovate[bot] in [#1188](https://github.com/nicholas-fedor/watchtower/pull/1188)
- Update nicholas-fedor/govulncheck-action digest to 5e52ebd by @renovate[bot] in [#1187](https://github.com/nicholas-fedor/watchtower/pull/1187)
- Update actions/checkout digest to 0c366fd by @renovate[bot] in [#1185](https://github.com/nicholas-fedor/watchtower/pull/1185)
- Update actions/attest-build-provenance digest to 98f3aa9 by @renovate[bot] in [#1184](https://github.com/nicholas-fedor/watchtower/pull/1184)
- Update module github.com/docker/cli to v29.1.4+incompatible by @renovate[bot] in [#1181](https://github.com/nicholas-fedor/watchtower/pull/1181)
- Update nicholas-fedor/govulncheck-action digest to 9813de4 by @renovate[bot] in [#1180](https://github.com/nicholas-fedor/watchtower/pull/1180)
- Update module github.com/onsi/gomega to v1.39.0 by @renovate[bot] in [#1176](https://github.com/nicholas-fedor/watchtower/pull/1176)
- Update cimg/go:1.25.5 docker digest to 955eb92 by @renovate[bot] in [#1179](https://github.com/nicholas-fedor/watchtower/pull/1179)
- Update actions/checkout digest to 064fe7f by @renovate[bot] in [#1178](https://github.com/nicholas-fedor/watchtower/pull/1178)
- Update module github.com/onsi/ginkgo/v2 to v2.27.4 by @renovate[bot] in [#1175](https://github.com/nicholas-fedor/watchtower/pull/1175)
- Update docker/setup-qemu-action digest to 45136fd by @renovate[bot] in [#1174](https://github.com/nicholas-fedor/watchtower/pull/1174)
- Update docker/setup-buildx-action digest to c7c4c00 by @renovate[bot] in [#1173](https://github.com/nicholas-fedor/watchtower/pull/1173)
- Update nicholas-fedor/govulncheck-action digest to 0076f09 by @renovate[bot] in [#1172](https://github.com/nicholas-fedor/watchtower/pull/1172)
- Update docker/setup-qemu-action digest to 6b85f87 by @renovate[bot] in [#1171](https://github.com/nicholas-fedor/watchtower/pull/1171)
- Update actions/setup-go digest to ae252ee by @renovate[bot] in [#1170](https://github.com/nicholas-fedor/watchtower/pull/1170)
- Update docker/login-action digest to 916386b by @renovate[bot] in [#1169](https://github.com/nicholas-fedor/watchtower/pull/1169)
- Update actions/attest-build-provenance digest to 63e6444 by @renovate[bot] in [#1167](https://github.com/nicholas-fedor/watchtower/pull/1167)
- Update golangci/golangci-lint-action digest to f75c1c4 by @renovate[bot] in [#1166](https://github.com/nicholas-fedor/watchtower/pull/1166)
- Update actions/attest-build-provenance digest to 405d0ea by @renovate[bot] in [#1164](https://github.com/nicholas-fedor/watchtower/pull/1164)
- Update peter-evans/create-pull-request digest to 2271f1d by @renovate[bot] in [#1163](https://github.com/nicholas-fedor/watchtower/pull/1163)
- Update actions/setup-python digest to 4f41a90 by @renovate[bot] in [#1158](https://github.com/nicholas-fedor/watchtower/pull/1158)
- Update golangci/golangci-lint-action digest to e9dc929 by @renovate[bot] in [#1156](https://github.com/nicholas-fedor/watchtower/pull/1156)

### New Contributors

- @skylenet made their first contribution in [#1182](https://github.com/nicholas-fedor/watchtower/pull/1182)

## [1.13.1] - 2025-12-22

### Fixed

- Resolve false positive circular reference detection by @nicholas-fedor in [#1139](https://github.com/nicholas-fedor/watchtower/pull/1139)
- Add SkipSelfUpdate parameter to refine self-update behavior by @nicholas-fedor in [#1137](https://github.com/nicholas-fedor/watchtower/pull/1137)
- Resolve false positive circular reference detection in dependency sorting by @nicholas-fedor in [#1120](https://github.com/nicholas-fedor/watchtower/pull/1120)
- Implement notification level filtering for report mode by @nicholas-fedor in [#1115](https://github.com/nicholas-fedor/watchtower/pull/1115)
- Ensure local image staleness checking for --no-pull flag by @nicholas-fedor in [#1099](https://github.com/nicholas-fedor/watchtower/pull/1099)

### Changed

- Add notification template entry and use em dash for self-update cleanup message by @nicholas-fedor in [#1136](https://github.com/nicholas-fedor/watchtower/pull/1136)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1145](https://github.com/nicholas-fedor/watchtower/pull/1145)
- Update changelog for main by @nicholas-fedor in [#1140](https://github.com/nicholas-fedor/watchtower/pull/1140)
- Update changelog for main by @nicholas-fedor in [#1133](https://github.com/nicholas-fedor/watchtower/pull/1133)
- Update changelog for main by @nicholas-fedor in [#1131](https://github.com/nicholas-fedor/watchtower/pull/1131)
- Update changelog for main by @nicholas-fedor in [#1129](https://github.com/nicholas-fedor/watchtower/pull/1129)
- Update changelog for main by @nicholas-fedor in [#1126](https://github.com/nicholas-fedor/watchtower/pull/1126)
- Update star history chart by @nicholas-fedor in [#1125](https://github.com/nicholas-fedor/watchtower/pull/1125)
- Update changelog for main by @nicholas-fedor in [#1121](https://github.com/nicholas-fedor/watchtower/pull/1121)
- Update changelog for main by @nicholas-fedor in [#1114](https://github.com/nicholas-fedor/watchtower/pull/1114)
- Update changelog for main by @nicholas-fedor in [#1101](https://github.com/nicholas-fedor/watchtower/pull/1101)

### Tests

- Refactor and realign test scope of actions_test.go file by @nicholas-fedor in [#1100](https://github.com/nicholas-fedor/watchtower/pull/1100)

### Chores

- Update golangci/golangci-lint-action digest to 2e568c9 by @renovate[bot] in [#1144](https://github.com/nicholas-fedor/watchtower/pull/1144)
- Update docker/login-action digest to 6862ffc by @renovate[bot] in [#1132](https://github.com/nicholas-fedor/watchtower/pull/1132)
- Update golang:alpine docker digest to ac09a5f by @renovate[bot] in [#1130](https://github.com/nicholas-fedor/watchtower/pull/1130)
- Update actions/attest-build-provenance digest to 00014ed by @renovate[bot] in [#1128](https://github.com/nicholas-fedor/watchtower/pull/1128)
- Update cimg/go:1.25.5 docker digest to b644c11 by @renovate[bot] in [#1127](https://github.com/nicholas-fedor/watchtower/pull/1127)
- Update nicholas-fedor/govulncheck-action digest to ec02307 by @renovate[bot] in [#1123](https://github.com/nicholas-fedor/watchtower/pull/1123)
- Update alpine:3.23.2 docker digest to 865b95f by @renovate[bot] in [#1122](https://github.com/nicholas-fedor/watchtower/pull/1122)
- Update golang:alpine docker digest to 7256733 by @renovate[bot] in [#1116](https://github.com/nicholas-fedor/watchtower/pull/1116)
- Update alpine docker tag to v3.23.2 by @renovate[bot] in [#1113](https://github.com/nicholas-fedor/watchtower/pull/1113)
- Update actions/setup-go digest to 4aaadf4 by @renovate[bot] in [#1112](https://github.com/nicholas-fedor/watchtower/pull/1112)

## [1.13.0] - 2025-12-16

### Fixed

- Enable graceful shutdown with context cancellation by @nicholas-fedor in [#1094](https://github.com/nicholas-fedor/watchtower/pull/1094)
- Suppress HTTP API server startup message when no-startup-message flag is set by @nicholas-fedor in [#1090](https://github.com/nicholas-fedor/watchtower/pull/1090)
- Resolve chained dependency handling and add Compose depends_on support by @nicholas-fedor in [#1086](https://github.com/nicholas-fedor/watchtower/pull/1086)
- Propagate notifications in error logging by @nicholas-fedor in [#1085](https://github.com/nicholas-fedor/watchtower/pull/1085)
- Prevent multiple orphaned containers on self-update by @nicholas-fedor in [#1075](https://github.com/nicholas-fedor/watchtower/pull/1075)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1096](https://github.com/nicholas-fedor/watchtower/pull/1096)
- Update changelog for main by @nicholas-fedor in [#1078](https://github.com/nicholas-fedor/watchtower/pull/1078)
- Update changelog for main by @nicholas-fedor in [#1074](https://github.com/nicholas-fedor/watchtower/pull/1074)

### Chores

- Update github/codeql-action digest to 5d4e8d1 by @renovate[bot] in [#1097](https://github.com/nicholas-fedor/watchtower/pull/1097)
- Update docker/setup-buildx-action digest to 8d2750c by @renovate[bot] in [#1095](https://github.com/nicholas-fedor/watchtower/pull/1095)
- Update actions/attest-build-provenance digest to 8835c60 by @renovate[bot] in [#1093](https://github.com/nicholas-fedor/watchtower/pull/1093)
- Update actions/attest-build-provenance digest to 331a7ac by @renovate[bot] in [#1092](https://github.com/nicholas-fedor/watchtower/pull/1092)
- Update golangci/golangci-lint-action digest to ef75033 by @renovate[bot] in [#1091](https://github.com/nicholas-fedor/watchtower/pull/1091)
- Update Go dependencies by @nicholas-fedor in [#1088](https://github.com/nicholas-fedor/watchtower/pull/1088)
- Update orhun/git-cliff-action digest to e16f179 by @renovate[bot] in [#1087](https://github.com/nicholas-fedor/watchtower/pull/1087)
- Update actions/cache digest to 9255dc7 by @renovate[bot] in [#1084](https://github.com/nicholas-fedor/watchtower/pull/1084)
- Update module github.com/docker/cli to v29.1.3+incompatible by @renovate[bot] in [#1083](https://github.com/nicholas-fedor/watchtower/pull/1083)
- Update github/codeql-action digest to 1b168cd by @renovate[bot] in [#1082](https://github.com/nicholas-fedor/watchtower/pull/1082)
- Update actions/cache action to v5 by @renovate[bot] in [#1081](https://github.com/nicholas-fedor/watchtower/pull/1081)
- Update securego/gosec action to v2.22.11 by @renovate[bot] in [#1077](https://github.com/nicholas-fedor/watchtower/pull/1077)
- Update cimg/go:1.25.5 docker digest to 9a8ad8c by @renovate[bot] in [#1073](https://github.com/nicholas-fedor/watchtower/pull/1073)

## [1.12.5] - 2025-12-10

### Fixed

- Resolve notification URL parsing regression with comma handling and whitespace trimming by @nicholas-fedor in [#1069](https://github.com/nicholas-fedor/watchtower/pull/1069)
- Resolve container listing failure due to unsupported "restarting" status by @ApprenticeofEnder in [#1061](https://github.com/nicholas-fedor/watchtower/pull/1061)
- Refine MAC address validation logging messages by @nicholas-fedor in [#1066](https://github.com/nicholas-fedor/watchtower/pull/1066)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1072](https://github.com/nicholas-fedor/watchtower/pull/1072)
- Update changelog for main by @nicholas-fedor in [#1063](https://github.com/nicholas-fedor/watchtower/pull/1063)
- Add ApprenticeofEnder to contributors list by @nicholas-fedor in [#1070](https://github.com/nicholas-fedor/watchtower/pull/1070)
- Add image update explanation and tagging examples by @nicholas-fedor in [#1062](https://github.com/nicholas-fedor/watchtower/pull/1062)

### Continuous Integration

- Correct actionlint issues in create-manifests.yaml by @nicholas-fedor in [#1071](https://github.com/nicholas-fedor/watchtower/pull/1071)

### Chores

- Update peter-evans/create-pull-request digest to 0979079 by @renovate[bot] in [#1068](https://github.com/nicholas-fedor/watchtower/pull/1068)
- Update peter-evans/create-pull-request digest to 98357b1 by @renovate[bot] in [#1065](https://github.com/nicholas-fedor/watchtower/pull/1065)
- Update codecov/codecov-action digest to 671740a by @renovate[bot] in [#1064](https://github.com/nicholas-fedor/watchtower/pull/1064)

### New Contributors

- @ApprenticeofEnder made their first contribution in [#1061](https://github.com/nicholas-fedor/watchtower/pull/1061)

## [1.12.4] - 2025-12-09

### Fixed

- Clear hostname when UTS mode is set to prevent recreation conflict by @nicholas-fedor in [#1056](https://github.com/nicholas-fedor/watchtower/pull/1056)
- Resolve TLS connection issues by @nicholas-fedor in [#1044](https://github.com/nicholas-fedor/watchtower/pull/1044)
- Normalize container name handling to resolve chained dependencies by @nicholas-fedor in [#1015](https://github.com/nicholas-fedor/watchtower/pull/1015)
- Address gaps in self-update functionality by @nicholas-fedor in [#1008](https://github.com/nicholas-fedor/watchtower/pull/1008)

### Changed

- Improve notification flag parsing by @nicholas-fedor in [#1039](https://github.com/nicholas-fedor/watchtower/pull/1039)

### Documentation

- Update changelog for main by @nicholas-fedor in [#1060](https://github.com/nicholas-fedor/watchtower/pull/1060)
- Update changelog for main by @nicholas-fedor in [#1057](https://github.com/nicholas-fedor/watchtower/pull/1057)
- Update changelog for main by @nicholas-fedor in [#1055](https://github.com/nicholas-fedor/watchtower/pull/1055)
- Update changelog for main by @nicholas-fedor in [#1051](https://github.com/nicholas-fedor/watchtower/pull/1051)
- Add AzariasB to contributors list by @nicholas-fedor in [#1053](https://github.com/nicholas-fedor/watchtower/pull/1053)
- Add contributors to README.md by @nicholas-fedor in [#1052](https://github.com/nicholas-fedor/watchtower/pull/1052)
- Update changelog for main by @nicholas-fedor in [#1046](https://github.com/nicholas-fedor/watchtower/pull/1046)
- Update changelog for main by @nicholas-fedor in [#1043](https://github.com/nicholas-fedor/watchtower/pull/1043)
- Update changelog for main by @nicholas-fedor in [#1040](https://github.com/nicholas-fedor/watchtower/pull/1040)
- Update changelog for main by @nicholas-fedor in [#1032](https://github.com/nicholas-fedor/watchtower/pull/1032)
- Update changelog for main by @nicholas-fedor in [#1027](https://github.com/nicholas-fedor/watchtower/pull/1027)
- Update changelog for main by @nicholas-fedor in [#1023](https://github.com/nicholas-fedor/watchtower/pull/1023)
- Update changelog for main by @nicholas-fedor in [#1021](https://github.com/nicholas-fedor/watchtower/pull/1021)
- Update changelog for main by @nicholas-fedor in [#1010](https://github.com/nicholas-fedor/watchtower/pull/1010)
- Update changelog for main by @nicholas-fedor in [#1004](https://github.com/nicholas-fedor/watchtower/pull/1004)
- Update changelog for main by @nicholas-fedor in [#1001](https://github.com/nicholas-fedor/watchtower/pull/1001)
- Update changelog for main by @nicholas-fedor in [#980](https://github.com/nicholas-fedor/watchtower/pull/980)
- Enhance lifecycle hooks documentation and examples by @nicholas-fedor in [#998](https://github.com/nicholas-fedor/watchtower/pull/998)
- Update changelog for main by @nicholas-fedor in [#974](https://github.com/nicholas-fedor/watchtower/pull/974)
- Update changelog for main by @nicholas-fedor in [#967](https://github.com/nicholas-fedor/watchtower/pull/967)
- Update changelog for main by @nicholas-fedor in [#964](https://github.com/nicholas-fedor/watchtower/pull/964)
- Update changelog for main by @nicholas-fedor in [#956](https://github.com/nicholas-fedor/watchtower/pull/956)
- Update changelog for main by @nicholas-fedor in [#953](https://github.com/nicholas-fedor/watchtower/pull/953)
- Update changelog for main by @nicholas-fedor in [#950](https://github.com/nicholas-fedor/watchtower/pull/950)
- Fix example notification template by @AzariasB in [#944](https://github.com/nicholas-fedor/watchtower/pull/944)
- Update changelog for main by @nicholas-fedor in [#949](https://github.com/nicholas-fedor/watchtower/pull/949)
- Update changelog for main by @nicholas-fedor in [#942](https://github.com/nicholas-fedor/watchtower/pull/942)
- Update changelog for main by @nicholas-fedor in [#938](https://github.com/nicholas-fedor/watchtower/pull/938)
- Update changelog for main by @nicholas-fedor in [#933](https://github.com/nicholas-fedor/watchtower/pull/933)
- Update changelog for main by @nicholas-fedor in [#926](https://github.com/nicholas-fedor/watchtower/pull/926)
- Update changelog for main by @nicholas-fedor in [#924](https://github.com/nicholas-fedor/watchtower/pull/924)
- Update changelog and configuration by @nicholas-fedor in [#921](https://github.com/nicholas-fedor/watchtower/pull/921)

### Continuous Integration

- Enable major and minor Docker production image release tags by @RoboMagus in [#1036](https://github.com/nicholas-fedor/watchtower/pull/1036)
- Exclude revive linter for api packages by @nicholas-fedor in [#1042](https://github.com/nicholas-fedor/watchtower/pull/1042)
- Add push option to delete command by @nicholas-fedor in [#920](https://github.com/nicholas-fedor/watchtower/pull/920)
- Resolve mike deploy alias conflict by @nicholas-fedor in [#919](https://github.com/nicholas-fedor/watchtower/pull/919)

### Chores

- Update peter-evans/create-pull-request digest to 41c0e4b by @renovate[bot] in [#1059](https://github.com/nicholas-fedor/watchtower/pull/1059)
- Update module github.com/nicholas-fedor/shoutrrr to v0.13.1 by @renovate[bot] in [#1058](https://github.com/nicholas-fedor/watchtower/pull/1058)
- Clean up shoutrrr notification warning messages by @nicholas-fedor in [#1054](https://github.com/nicholas-fedor/watchtower/pull/1054)
- Update module golang.org/x/text to v0.32.0 by @renovate[bot] in [#1050](https://github.com/nicholas-fedor/watchtower/pull/1050)
- Update module github.com/onsi/gomega to v1.38.3 by @renovate[bot] in [#1049](https://github.com/nicholas-fedor/watchtower/pull/1049)
- Update module github.com/onsi/ginkgo/v2 to v2.27.3 by @renovate[bot] in [#1048](https://github.com/nicholas-fedor/watchtower/pull/1048)
- Update actions/attest-build-provenance digest to c6f9859 by @renovate[bot] in [#1047](https://github.com/nicholas-fedor/watchtower/pull/1047)
- Update golangci/golangci-lint-action digest to ca80bee by @renovate[bot] in [#1045](https://github.com/nicholas-fedor/watchtower/pull/1045)
- Standardize formatting rules by @nicholas-fedor in [#1037](https://github.com/nicholas-fedor/watchtower/pull/1037)
- Update module github.com/nicholas-fedor/shoutrrr to v0.13.0 by @renovate[bot] in [#1035](https://github.com/nicholas-fedor/watchtower/pull/1035)
- Update peter-evans/create-pull-request digest to 22a9089 by @renovate[bot] in [#1034](https://github.com/nicholas-fedor/watchtower/pull/1034)
- Update github/codeql-action digest to cf1bb45 by @renovate[bot] in [#1033](https://github.com/nicholas-fedor/watchtower/pull/1033)
- Update peter-evans/create-pull-request digest to d4f3be6 by @renovate[bot] in [#1031](https://github.com/nicholas-fedor/watchtower/pull/1031)
- Update module github.com/spf13/cobra to v1.10.2 by @renovate[bot] in [#1026](https://github.com/nicholas-fedor/watchtower/pull/1026)
- Update golang:alpine docker digest to 2611181 by @renovate[bot] in [#1025](https://github.com/nicholas-fedor/watchtower/pull/1025)
- Update alpine docker tag to v3.23.0 by @renovate[bot] in [#1022](https://github.com/nicholas-fedor/watchtower/pull/1022)
- Update docker/setup-buildx-action digest to 65d18f8 by @renovate[bot] in [#1020](https://github.com/nicholas-fedor/watchtower/pull/1020)
- Update module github.com/docker/cli to v29.1.2+incompatible by @renovate[bot] in [#1017](https://github.com/nicholas-fedor/watchtower/pull/1017)
- Update cimg/go docker tag to v1.25.5 by @renovate[bot] in [#1016](https://github.com/nicholas-fedor/watchtower/pull/1016)
- Update golang docker tag to v1.25.5 by @renovate[bot] in [#1014](https://github.com/nicholas-fedor/watchtower/pull/1014)
- Update nicholas-fedor/go-proxy-pull-action digest to 501ad32 by @renovate[bot] in [#1013](https://github.com/nicholas-fedor/watchtower/pull/1013)
- Update actions/checkout digest to 8e8c483 by @renovate[bot] in [#1011](https://github.com/nicholas-fedor/watchtower/pull/1011)
- Update golang:alpine docker digest to 3587db7 by @renovate[bot] in [#1012](https://github.com/nicholas-fedor/watchtower/pull/1012)
- Update dependency go to v1.25.5 by @renovate[bot] in [#1009](https://github.com/nicholas-fedor/watchtower/pull/1009)
- Update nicholas-fedor/govulncheck-action digest to 5d80989 by @renovate[bot] in [#1007](https://github.com/nicholas-fedor/watchtower/pull/1007)
- Update actions/checkout digest to 8e8c483 by @renovate[bot] in [#1006](https://github.com/nicholas-fedor/watchtower/pull/1006)
- Update actions/attest-build-provenance digest to ca0aaa1 by @renovate[bot] in [#1005](https://github.com/nicholas-fedor/watchtower/pull/1005)
- Update peter-evans/create-pull-request digest to bc8a47f by @renovate[bot] in [#1003](https://github.com/nicholas-fedor/watchtower/pull/1003)
- Update goreleaser/goreleaser-action digest to d31d51a by @renovate[bot] in [#1002](https://github.com/nicholas-fedor/watchtower/pull/1002)
- Update golangci/golangci-lint-action digest to 1e7e51e by @renovate[bot] in [#1000](https://github.com/nicholas-fedor/watchtower/pull/1000)
- Update github/codeql-action digest to fe4161a by @renovate[bot] in [#999](https://github.com/nicholas-fedor/watchtower/pull/999)
- Update golangci/golangci-lint-action digest to 13fed6f by @renovate[bot] in [#996](https://github.com/nicholas-fedor/watchtower/pull/996)
- Update module github.com/docker/cli to v29.1.1+incompatible by @renovate[bot] in [#995](https://github.com/nicholas-fedor/watchtower/pull/995)
- Update module github.com/docker/cli to v29.1.0+incompatible by @renovate[bot] in [#994](https://github.com/nicholas-fedor/watchtower/pull/994)
- Update goreleaser/goreleaser-action digest to f3511a2 by @renovate[bot] in [#993](https://github.com/nicholas-fedor/watchtower/pull/993)
- Update module github.com/nicholas-fedor/shoutrrr to v0.12.1 by @renovate[bot] in [#990](https://github.com/nicholas-fedor/watchtower/pull/990)
- Update module github.com/docker/cli to v29.0.4+incompatible by @renovate[bot] in [#987](https://github.com/nicholas-fedor/watchtower/pull/987)
- Update actions/setup-python digest to 83679a8 by @renovate[bot] in [#985](https://github.com/nicholas-fedor/watchtower/pull/985)
- Update actions/attest-build-provenance digest to 08a89fb by @renovate[bot] in [#984](https://github.com/nicholas-fedor/watchtower/pull/984)
- Update module github.com/docker/cli to v29.0.3+incompatible by @renovate[bot] in [#983](https://github.com/nicholas-fedor/watchtower/pull/983)
- Update golangci/golangci-lint-action digest to a6071aa by @renovate[bot] in [#982](https://github.com/nicholas-fedor/watchtower/pull/982)
- Update github/codeql-action digest to fdbfb4d by @renovate[bot] in [#981](https://github.com/nicholas-fedor/watchtower/pull/981)
- Update nicholas-fedor/govulncheck-action digest to 22f7e2d by @renovate[bot] in [#979](https://github.com/nicholas-fedor/watchtower/pull/979)
- Update actions/checkout digest to c2d88d3 by @renovate[bot] in [#978](https://github.com/nicholas-fedor/watchtower/pull/978)
- Update peter-evans/create-pull-request digest to 84ae59a by @renovate[bot] in [#973](https://github.com/nicholas-fedor/watchtower/pull/973)
- Update golangci/golangci-lint-action digest to e7fa5ac by @renovate[bot] in [#972](https://github.com/nicholas-fedor/watchtower/pull/972)
- Update actions/checkout action to v6 by @renovate[bot] in [#966](https://github.com/nicholas-fedor/watchtower/pull/966)
- Update nicholas-fedor/govulncheck-action digest to d800c37 by @renovate[bot] in [#969](https://github.com/nicholas-fedor/watchtower/pull/969)
- Update actions/attest-build-provenance digest to f8ed128 by @renovate[bot] in [#968](https://github.com/nicholas-fedor/watchtower/pull/968)
- Update actions/checkout digest to 1af3b93 by @renovate[bot] in [#965](https://github.com/nicholas-fedor/watchtower/pull/965)
- Update nicholas-fedor/govulncheck-action digest to 55deb21 by @renovate[bot] in [#963](https://github.com/nicholas-fedor/watchtower/pull/963)
- Update actions/setup-go digest to 4dc6199 by @renovate[bot] in [#962](https://github.com/nicholas-fedor/watchtower/pull/962)
- Update nicholas-fedor/govulncheck-action digest to 0ee4877 by @renovate[bot] in [#960](https://github.com/nicholas-fedor/watchtower/pull/960)
- Update actions/setup-go digest to f3787be by @renovate[bot] in [#959](https://github.com/nicholas-fedor/watchtower/pull/959)
- Update codecov/codecov-action digest to 96b38e9 by @renovate[bot] in [#958](https://github.com/nicholas-fedor/watchtower/pull/958)
- Update golangci/golangci-lint-action digest to 1dfda28 by @renovate[bot] in [#955](https://github.com/nicholas-fedor/watchtower/pull/955)
- Update github/codeql-action digest to e12f017 by @renovate[bot] in [#954](https://github.com/nicholas-fedor/watchtower/pull/954)
- Update actions/setup-python digest to bfc4944 by @renovate[bot] in [#952](https://github.com/nicholas-fedor/watchtower/pull/952)
- Update actions/attest-build-provenance digest to 268464d by @renovate[bot] in [#951](https://github.com/nicholas-fedor/watchtower/pull/951)
- Update module github.com/docker/cli to v29.0.2+incompatible by @renovate[bot] in [#948](https://github.com/nicholas-fedor/watchtower/pull/948)
- Update cimg/go:1.25.4 docker digest to cf75b46 by @renovate[bot] in [#947](https://github.com/nicholas-fedor/watchtower/pull/947)
- Update actions/checkout digest to 93cb6ef by @renovate[bot] in [#946](https://github.com/nicholas-fedor/watchtower/pull/946)
- Update golangci/golangci-lint-action digest to 8b0f942 by @renovate[bot] in [#941](https://github.com/nicholas-fedor/watchtower/pull/941)
- Update golangci/golangci-lint-action digest to 37a9faf by @renovate[bot] in [#937](https://github.com/nicholas-fedor/watchtower/pull/937)
- Update module github.com/docker/cli to v29.0.1+incompatible by @renovate[bot] in [#935](https://github.com/nicholas-fedor/watchtower/pull/935)
- Update cimg/go docker tag to v1.25.4 by @renovate[bot] in [#934](https://github.com/nicholas-fedor/watchtower/pull/934)
- Update peter-evans/create-pull-request digest to b4733b9 by @renovate[bot] in [#932](https://github.com/nicholas-fedor/watchtower/pull/932)
- Update cimg/go:1.25.3 docker digest to 0184935 by @renovate[bot] in [#931](https://github.com/nicholas-fedor/watchtower/pull/931)
- Update github/codeql-action digest to 014f16e by @renovate[bot] in [#925](https://github.com/nicholas-fedor/watchtower/pull/925)
- Update actions/setup-python digest to 97aeb3e by @renovate[bot] in [#923](https://github.com/nicholas-fedor/watchtower/pull/923)

### New Contributors

- @RoboMagus made their first contribution in [#1036](https://github.com/nicholas-fedor/watchtower/pull/1036)
- @AzariasB made their first contribution in [#944](https://github.com/nicholas-fedor/watchtower/pull/944)

## [1.12.3] - 2025-11-13

### Fixed

- Fix logic for legacy template usage by @nicholas-fedor in [#910](https://github.com/nicholas-fedor/watchtower/pull/910)

### Documentation

- Update changelog for main by @nicholas-fedor in [#912](https://github.com/nicholas-fedor/watchtower/pull/912)
- Update changelog for v1.12.2 by @nicholas-fedor in [#905](https://github.com/nicholas-fedor/watchtower/pull/905)

### Continuous Integration

- Extract changelog generation to dedicated workflow by @nicholas-fedor in [#911](https://github.com/nicholas-fedor/watchtower/pull/911)

### Chores

- Update nicholas-fedor/govulncheck-action digest to 077e0b4 by @renovate[bot] in [#917](https://github.com/nicholas-fedor/watchtower/pull/917)
- Update actions/setup-python digest to 443da59 by @renovate[bot] in [#915](https://github.com/nicholas-fedor/watchtower/pull/915)
- Update actions/setup-go digest to 3a0c2c8 by @renovate[bot] in [#914](https://github.com/nicholas-fedor/watchtower/pull/914)
- Update codecov/codecov-action digest to 9b6d1f8 by @renovate[bot] in [#908](https://github.com/nicholas-fedor/watchtower/pull/908)

## [1.12.2] - 2025-11-11

### Added

- Add dev container by @yubiuser in [#825](https://github.com/nicholas-fedor/watchtower/pull/825)

### Fixed

- Correct container ID handling in notifications and logging by @nicholas-fedor in [#893](https://github.com/nicholas-fedor/watchtower/pull/893)
- fix: resolve notification buffer, formatting, and message issues by @nicholas-fedor in [#864](https://github.com/nicholas-fedor/watchtower/pull/864)
- Handle ghost containers in ListAllContainers by @nicholas-fedor in [#859](https://github.com/nicholas-fedor/watchtower/pull/859)
- Restore notifications for monitor-only containers by @nicholas-fedor in [#850](https://github.com/nicholas-fedor/watchtower/pull/850)
- Make shoutrrr notifier thread-safe by @nicholas-fedor in [#844](https://github.com/nicholas-fedor/watchtower/pull/844)
- Prevent double notification entries for unchanged containers by @nicholas-fedor in [#836](https://github.com/nicholas-fedor/watchtower/pull/836)
- Implement cleanup detection for update-on-start control by @nicholas-fedor in [#831](https://github.com/nicholas-fedor/watchtower/pull/831)
- Clear shoutrrr notification queue after sending by @nicholas-fedor in [#824](https://github.com/nicholas-fedor/watchtower/pull/824)
- Improve Podman detection reliability by @nicholas-fedor in [#799](https://github.com/nicholas-fedor/watchtower/pull/799)
- Resolve issues with notification split by container feature by @nicholas-fedor in [#775](https://github.com/nicholas-fedor/watchtower/pull/775)
- Improve WATCHTOWER_UPDATE_ON_START logging messages by @nicholas-fedor in [#768](https://github.com/nicholas-fedor/watchtower/pull/768)

### Changed

- Update LogScheduleInfo function and related components by @yubiuser in [#821](https://github.com/nicholas-fedor/watchtower/pull/821)

### Documentation

- Update changelog for main by @nicholas-fedor in [#903](https://github.com/nicholas-fedor/watchtower/pull/903)
- Update changelog for main by @nicholas-fedor in [#899](https://github.com/nicholas-fedor/watchtower/pull/899)
- Update changelog for main by @nicholas-fedor in [#894](https://github.com/nicholas-fedor/watchtower/pull/894)
- Update changelog for main by @nicholas-fedor in [#885](https://github.com/nicholas-fedor/watchtower/pull/885)
- Update changelog for main by @nicholas-fedor in [#874](https://github.com/nicholas-fedor/watchtower/pull/874)
- Update changelog for main by @nicholas-fedor in [#865](https://github.com/nicholas-fedor/watchtower/pull/865)
- Update changelog for main by @nicholas-fedor in [#860](https://github.com/nicholas-fedor/watchtower/pull/860)
- Update changelog for main by @nicholas-fedor in [#851](https://github.com/nicholas-fedor/watchtower/pull/851)
- Update changelog for main by @nicholas-fedor in [#849](https://github.com/nicholas-fedor/watchtower/pull/849)
- Update changelog for main by @nicholas-fedor in [#846](https://github.com/nicholas-fedor/watchtower/pull/846)
- Update changelog for main by @nicholas-fedor in [#842](https://github.com/nicholas-fedor/watchtower/pull/842)
- Update changelog for main by @nicholas-fedor in [#837](https://github.com/nicholas-fedor/watchtower/pull/837)
- Update changelog for main by @nicholas-fedor in [#815](https://github.com/nicholas-fedor/watchtower/pull/815)
- Correct API authentication header documentation by @nicholas-fedor in [#802](https://github.com/nicholas-fedor/watchtower/pull/802)

### Tests

- Resolve data race in concurrent test logging by @nicholas-fedor in [#800](https://github.com/nicholas-fedor/watchtower/pull/800)

### Continuous Integration

- Disable persist-credentials in release workflows by @nicholas-fedor in [#873](https://github.com/nicholas-fedor/watchtower/pull/873)
- Disable GITHUB_TOKEN in release-dev.yaml by @nicholas-fedor in [#872](https://github.com/nicholas-fedor/watchtower/pull/872)
- Remove go lint step in favor of golangci-lint --fix by @nicholas-fedor in [#847](https://github.com/nicholas-fedor/watchtower/pull/847)
- Add check-latest to Go setup in workflows by @nicholas-fedor in [#787](https://github.com/nicholas-fedor/watchtower/pull/787)
- Ci(test); add latest version checking by @nicholas-fedor in [#786](https://github.com/nicholas-fedor/watchtower/pull/786)

### Chores

- Update dependencies and remove outdated comment by @nicholas-fedor in [#901](https://github.com/nicholas-fedor/watchtower/pull/901)
- Update module golang.org/x/text to v0.31.0 by @renovate[bot] in [#902](https://github.com/nicholas-fedor/watchtower/pull/902)
- Update module github.com/docker/cli to v29 by @renovate[bot] in [#898](https://github.com/nicholas-fedor/watchtower/pull/898)
- Update golangci/golangci-lint-action digest to 199a9c2 by @renovate[bot] in [#897](https://github.com/nicholas-fedor/watchtower/pull/897)
- Update golangci/golangci-lint-action digest to c7c1219 by @renovate[bot] in [#896](https://github.com/nicholas-fedor/watchtower/pull/896)
- Update golangci/golangci-lint-action digest to 0a35821 by @renovate[bot] in [#892](https://github.com/nicholas-fedor/watchtower/pull/892)
- Update golangci/golangci-lint-action digest to a66d26a by @renovate[bot] in [#891](https://github.com/nicholas-fedor/watchtower/pull/891)
- Update goreleaser/goreleaser-action digest to 9cf3611 by @renovate[bot] in [#890](https://github.com/nicholas-fedor/watchtower/pull/890)
- Update nicholas-fedor/go-proxy-pull-action digest to a32dd3b by @renovate[bot] in [#889](https://github.com/nicholas-fedor/watchtower/pull/889)
- Update golang:alpine docker digest to d3f0cf7 by @renovate[bot] in [#888](https://github.com/nicholas-fedor/watchtower/pull/888)
- Update golang:1.25.4-alpine3.22 docker digest to d3f0cf7 by @renovate[bot] in [#887](https://github.com/nicholas-fedor/watchtower/pull/887)
- Update module github.com/docker/docker to v28.5.2+incompatible by @renovate[bot] in [#884](https://github.com/nicholas-fedor/watchtower/pull/884)
- Update golang docker tag to v1.25.4 by @renovate[bot] in [#883](https://github.com/nicholas-fedor/watchtower/pull/883)
- Update nicholas-fedor/go-proxy-pull-action digest to 41fdd3e by @renovate[bot] in [#882](https://github.com/nicholas-fedor/watchtower/pull/882)
- Update golang:alpine docker digest to 8b6b77a by @renovate[bot] in [#881](https://github.com/nicholas-fedor/watchtower/pull/881)
- Update module github.com/docker/cli to v28.5.2+incompatible by @renovate[bot] in [#880](https://github.com/nicholas-fedor/watchtower/pull/880)
- Update dependency go to v1.25.4 by @renovate[bot] in [#879](https://github.com/nicholas-fedor/watchtower/pull/879)
- Update goreleaser/goreleaser-action digest to aab4704 by @renovate[bot] in [#878](https://github.com/nicholas-fedor/watchtower/pull/878)
- Update docker/setup-qemu-action digest to c7c5346 by @renovate[bot] in [#877](https://github.com/nicholas-fedor/watchtower/pull/877)
- Update cimg/go:1.25.3 docker digest to af601f9 by @renovate[bot] in [#875](https://github.com/nicholas-fedor/watchtower/pull/875)
- Update module github.com/nicholas-fedor/shoutrrr to v0.12.0 by @renovate[bot] in [#871](https://github.com/nicholas-fedor/watchtower/pull/871)
- Update nicholas-fedor/govulncheck-action digest to fa0b698 by @renovate[bot] in [#870](https://github.com/nicholas-fedor/watchtower/pull/870)
- Update actions/checkout digest to 71cf226 by @renovate[bot] in [#869](https://github.com/nicholas-fedor/watchtower/pull/869)
- Update dependency path-filtering to v3 by @renovate[bot] in [#868](https://github.com/nicholas-fedor/watchtower/pull/868)
- Update golangci/golangci-lint-action digest to 7fe1b22 by @renovate[bot] in [#867](https://github.com/nicholas-fedor/watchtower/pull/867)
- Pin dependencies by @renovate[bot] in [#866](https://github.com/nicholas-fedor/watchtower/pull/866)
- Update peter-evans/create-pull-request digest to 0edc001 by @renovate[bot] in [#863](https://github.com/nicholas-fedor/watchtower/pull/863)
- Update github/codeql-action digest to 0499de3 by @renovate[bot] in [#862](https://github.com/nicholas-fedor/watchtower/pull/862)
- Update github/codeql-action digest to 5fe9434 by @renovate[bot] in [#861](https://github.com/nicholas-fedor/watchtower/pull/861)
- Update cimg/go:1.25.3 docker digest to e31a463 by @renovate[bot] in [#858](https://github.com/nicholas-fedor/watchtower/pull/858)
- Update cimg/go:1.25.3 docker digest to 68b245d by @renovate[bot] in [#856](https://github.com/nicholas-fedor/watchtower/pull/856)
- Update nicholas-fedor/go-proxy-pull-action digest to 0591509 by @renovate[bot] in [#855](https://github.com/nicholas-fedor/watchtower/pull/855)
- Update nicholas-fedor/govulncheck-action digest to 803f85c by @renovate[bot] in [#853](https://github.com/nicholas-fedor/watchtower/pull/853)
- Update actions/setup-go digest to faf5242 by @renovate[bot] in [#852](https://github.com/nicholas-fedor/watchtower/pull/852)
- Update module github.com/nicholas-fedor/shoutrrr to v0.11.1 by @renovate[bot] in [#848](https://github.com/nicholas-fedor/watchtower/pull/848)
- Update nicholas-fedor/govulncheck-action digest to 76fb91b by @renovate[bot] in [#845](https://github.com/nicholas-fedor/watchtower/pull/845)
- Update module github.com/onsi/ginkgo/v2 to v2.27.2 by @renovate[bot] in [#841](https://github.com/nicholas-fedor/watchtower/pull/841)
- Update actions/setup-go digest to 7bc60db by @renovate[bot] in [#840](https://github.com/nicholas-fedor/watchtower/pull/840)
- Update golangci/golangci-lint-action digest to 14973f1 by @renovate[bot] in [#839](https://github.com/nicholas-fedor/watchtower/pull/839)
- Add makefile by @nicholas-fedor in [#835](https://github.com/nicholas-fedor/watchtower/pull/835)
- Update github/codeql-action digest to 4e94bd1 by @renovate[bot] in [#833](https://github.com/nicholas-fedor/watchtower/pull/833)
- Update module github.com/nicholas-fedor/shoutrrr to v0.11.0 by @renovate[bot] in [#832](https://github.com/nicholas-fedor/watchtower/pull/832)
- Update module github.com/onsi/ginkgo/v2 to v2.27.1 by @renovate[bot] in [#830](https://github.com/nicholas-fedor/watchtower/pull/830)
- Update actions/setup-python digest to cfd55ca by @renovate[bot] in [#829](https://github.com/nicholas-fedor/watchtower/pull/829)
- Update actions/setup-python digest to bba65e5 by @renovate[bot] in [#823](https://github.com/nicholas-fedor/watchtower/pull/823)
- Update golangci/golangci-lint-action digest to b002b6e by @renovate[bot] in [#822](https://github.com/nicholas-fedor/watchtower/pull/822)
- Update github/codeql-action digest to 16140ae by @renovate[bot] in [#820](https://github.com/nicholas-fedor/watchtower/pull/820)
- Update actions/attest-build-provenance digest to ba965ac by @renovate[bot] in [#819](https://github.com/nicholas-fedor/watchtower/pull/819)
- Update docker/login-action digest to 28fdb31 by @renovate[bot] in [#816](https://github.com/nicholas-fedor/watchtower/pull/816)
- Update module github.com/nicholas-fedor/shoutrrr to v0.10.3 by @renovate[bot] in [#814](https://github.com/nicholas-fedor/watchtower/pull/814)
- Update dependency path-filtering to v2.1.0 by @renovate[bot] in [#813](https://github.com/nicholas-fedor/watchtower/pull/813)
- Update golangci/golangci-lint-action digest to b68d21b by @renovate[bot] in [#812](https://github.com/nicholas-fedor/watchtower/pull/812)
- Update nicholas-fedor/go-proxy-pull-action digest to 3349087 by @renovate[bot] in [#811](https://github.com/nicholas-fedor/watchtower/pull/811)
- Update securego/gosec action to v2.22.10 by @renovate[bot] in [#810](https://github.com/nicholas-fedor/watchtower/pull/810)
- Update golang:alpine docker digest to aee43c3 by @renovate[bot] in [#809](https://github.com/nicholas-fedor/watchtower/pull/809)
- Update actions/setup-python digest to 18566f8 by @renovate[bot] in [#808](https://github.com/nicholas-fedor/watchtower/pull/808)
- Update cimg/go docker tag to v1.25.3 by @renovate[bot] in [#807](https://github.com/nicholas-fedor/watchtower/pull/807)
- Update nicholas-fedor/go-proxy-pull-action digest to 6b27ce6 by @renovate[bot] in [#806](https://github.com/nicholas-fedor/watchtower/pull/806)
- Update golang:alpine docker digest to ecb8038 by @renovate[bot] in [#805](https://github.com/nicholas-fedor/watchtower/pull/805)
- Update dependency go to v1.25.3 by @renovate[bot] in [#804](https://github.com/nicholas-fedor/watchtower/pull/804)
- Update golangci/golangci-lint-action digest to 06188a2 by @renovate[bot] in [#803](https://github.com/nicholas-fedor/watchtower/pull/803)
- Update nicholas-fedor/go-proxy-pull-action digest to 28967b1 by @renovate[bot] in [#798](https://github.com/nicholas-fedor/watchtower/pull/798)
- Update golang:alpine docker digest to 06cdd34 by @renovate[bot] in [#797](https://github.com/nicholas-fedor/watchtower/pull/797)
- Update github/codeql-action digest to f443b60 by @renovate[bot] in [#795](https://github.com/nicholas-fedor/watchtower/pull/795)
- Update nicholas-fedor/go-proxy-pull-action digest to f36283c by @renovate[bot] in [#793](https://github.com/nicholas-fedor/watchtower/pull/793)
- Update nicholas-fedor/go-proxy-pull-action digest to df60457 by @renovate[bot] in [#792](https://github.com/nicholas-fedor/watchtower/pull/792)
- Update nicholas-fedor/go-proxy-pull-action digest to 9d5bb93 by @renovate[bot] in [#791](https://github.com/nicholas-fedor/watchtower/pull/791)
- Update golang:alpine docker digest to 182059d by @renovate[bot] in [#790](https://github.com/nicholas-fedor/watchtower/pull/790)
- Update alpine:3.22.2 docker digest to 4b7ce07 by @renovate[bot] in [#788](https://github.com/nicholas-fedor/watchtower/pull/788)
- Update golang:alpine docker digest to a86c313 by @renovate[bot] in [#789](https://github.com/nicholas-fedor/watchtower/pull/789)
- Update alpine docker tag to v3.22.2 by @renovate[bot] in [#785](https://github.com/nicholas-fedor/watchtower/pull/785)
- Update github/codeql-action action to v4 by @renovate[bot] in [#777](https://github.com/nicholas-fedor/watchtower/pull/777)
- Update module golang.org/x/text to v0.30.0 by @renovate[bot] in [#784](https://github.com/nicholas-fedor/watchtower/pull/784)
- Update module github.com/docker/docker to v28.5.1+incompatible by @renovate[bot] in [#783](https://github.com/nicholas-fedor/watchtower/pull/783)
- Update module github.com/docker/cli to v28.5.1+incompatible by @renovate[bot] in [#782](https://github.com/nicholas-fedor/watchtower/pull/782)
- Update cimg/go docker tag to v1.25.2 by @renovate[bot] in [#781](https://github.com/nicholas-fedor/watchtower/pull/781)
- Update nicholas-fedor/go-proxy-pull-action digest to 22f4f2d by @renovate[bot] in [#780](https://github.com/nicholas-fedor/watchtower/pull/780)
- Update dependency go to v1.25.2 by @renovate[bot] in [#779](https://github.com/nicholas-fedor/watchtower/pull/779)
- Update golang:alpine docker digest to 6104e2b by @renovate[bot] in [#778](https://github.com/nicholas-fedor/watchtower/pull/778)
- Update github/codeql-action digest to a8d1ac4 by @renovate[bot] in [#776](https://github.com/nicholas-fedor/watchtower/pull/776)
- Update actions/attest-build-provenance digest to bed76f6 by @renovate[bot] in [#773](https://github.com/nicholas-fedor/watchtower/pull/773)
- Update golangci/golangci-lint-action digest to 1d64cc1 by @renovate[bot] in [#774](https://github.com/nicholas-fedor/watchtower/pull/774)
- Update peter-evans/create-pull-request digest to 46cdba7 by @renovate[bot] in [#772](https://github.com/nicholas-fedor/watchtower/pull/772)
- Update module github.com/nicholas-fedor/shoutrrr to v0.10.1 by @renovate[bot] in [#770](https://github.com/nicholas-fedor/watchtower/pull/770)

### New Contributors

- @yubiuser made their first contribution in [#825](https://github.com/nicholas-fedor/watchtower/pull/825)

## [1.12.1] - 2025-10-04

### Fixed

- Prevent I/O blocking in API update handler by @nicholas-fedor in [#765](https://github.com/nicholas-fedor/watchtower/pull/765)
- Digest retrieval failed, falling back to full pull by @nicholas-fedor in [#763](https://github.com/nicholas-fedor/watchtower/pull/763)

### Documentation

- Update changelog for v1.12.0 by @nicholas-fedor in [#759](https://github.com/nicholas-fedor/watchtower/pull/759)

### Chores

- Update orhun/git-cliff-action digest to d77b37d by @renovate[bot] in [#762](https://github.com/nicholas-fedor/watchtower/pull/762)

## [1.12.0] - 2025-10-03

### Added

- Add --notification-split-by-container flag by @nicholas-fedor in [#721](https://github.com/nicholas-fedor/watchtower/pull/721)
- Add --cpu-copy-mode flag for Podman CPU compatibility by @nicholas-fedor in [#712](https://github.com/nicholas-fedor/watchtower/pull/712)
- Add HTTP API host configuration support by @nicholas-fedor in [#697](https://github.com/nicholas-fedor/watchtower/pull/697)
- Add UID and GID support for lifecycle hooks scripts by @nicholas-fedor in [#690](https://github.com/nicholas-fedor/watchtower/pull/690)
- Enhance HTTP API update endpoint with structured JSON response by @nicholas-fedor in [#673](https://github.com/nicholas-fedor/watchtower/pull/673)
- Add --update-on-start flag for immediate update check by @nicholas-fedor in [#672](https://github.com/nicholas-fedor/watchtower/pull/672)
- Add health check waiting for rolling restarts by @nicholas-fedor in [#671](https://github.com/nicholas-fedor/watchtower/pull/671)
- Add container metadata for lifecycle hooks by @nicholas-fedor in [#670](https://github.com/nicholas-fedor/watchtower/pull/670)

### Fixed

- Ensure Watchtower updates itself last to fix notification split issue by @nicholas-fedor in [#756](https://github.com/nicholas-fedor/watchtower/pull/756)
- Correct shutdown lock waiting logic and prevent test timeouts by @nicholas-fedor in [#753](https://github.com/nicholas-fedor/watchtower/pull/753)
- Resolve data race in shoutrrr notifications by @nicholas-fedor in [#746](https://github.com/nicholas-fedor/watchtower/pull/746)
- Prevent nil pointer dereference in container cleanup by @nicholas-fedor in [#745](https://github.com/nicholas-fedor/watchtower/pull/745)
- Integrate --update-on-start with normal update cycle by @nicholas-fedor in [#740](https://github.com/nicholas-fedor/watchtower/pull/740)
- Improve CI test reliability across platforms by @nicholas-fedor in [#732](https://github.com/nicholas-fedor/watchtower/pull/732)
- Prevent concurrent Docker client access causing crashes by @nicholas-fedor in [#731](https://github.com/nicholas-fedor/watchtower/pull/731)
- Resolve Docker Distribution API manifest HEAD request issues by @nicholas-fedor in [#728](https://github.com/nicholas-fedor/watchtower/pull/728)
- Improve self-update handling with robust digest parsing by @nicholas-fedor in [#724](https://github.com/nicholas-fedor/watchtower/pull/724)
- Address container identification issue by @nicholas-fedor in [#718](https://github.com/nicholas-fedor/watchtower/pull/718)
- Improve logging clarity and accuracy for scheduling modes by @nicholas-fedor in [#716](https://github.com/nicholas-fedor/watchtower/pull/716)
- Improve Container ID Retrieval for Self-Update by @nicholas-fedor in [#714](https://github.com/nicholas-fedor/watchtower/pull/714)
- Send report notifications in monitor-only mode by @nicholas-fedor in [#709](https://github.com/nicholas-fedor/watchtower/pull/709)
- Resolve scope issues in self-updates and improve digest request handling by @nicholas-fedor in [#683](https://github.com/nicholas-fedor/watchtower/pull/683)
- Improve digest fetching by falling back to GET when HEAD returns 404 by @nicholas-fedor in [#669](https://github.com/nicholas-fedor/watchtower/pull/669)
- Resolve HTTP API failures on multiple simultaneous requests by @nicholas-fedor in [#668](https://github.com/nicholas-fedor/watchtower/pull/668)
- Add HEAD to GET fallback for digest fetching by @nicholas-fedor in [#667](https://github.com/nicholas-fedor/watchtower/pull/667)
- Enhance scope isolation and self-update safeguards by @nicholas-fedor in [#666](https://github.com/nicholas-fedor/watchtower/pull/666)
- Prevent dereferencing an uninitialized notifier instance by @nicholas-fedor in [#644](https://github.com/nicholas-fedor/watchtower/pull/644)

### Documentation

- Update changelog for main by @nicholas-fedor in [#757](https://github.com/nicholas-fedor/watchtower/pull/757)
- Update changelog for main by @nicholas-fedor in [#754](https://github.com/nicholas-fedor/watchtower/pull/754)
- Update changelog for main by @nicholas-fedor in [#752](https://github.com/nicholas-fedor/watchtower/pull/752)
- Update changelog for main by @nicholas-fedor in [#747](https://github.com/nicholas-fedor/watchtower/pull/747)
- Update changelog for main by @nicholas-fedor in [#743](https://github.com/nicholas-fedor/watchtower/pull/743)
- Update changelog for main by @nicholas-fedor in [#741](https://github.com/nicholas-fedor/watchtower/pull/741)
- Update changelog for main by @nicholas-fedor in [#736](https://github.com/nicholas-fedor/watchtower/pull/736)
- Update changelog for main by @nicholas-fedor in [#729](https://github.com/nicholas-fedor/watchtower/pull/729)
- Update changelog for main by @nicholas-fedor in [#725](https://github.com/nicholas-fedor/watchtower/pull/725)
- Update changelog for main by @nicholas-fedor in [#722](https://github.com/nicholas-fedor/watchtower/pull/722)
- Update changelog for main by @nicholas-fedor in [#719](https://github.com/nicholas-fedor/watchtower/pull/719)
- Update changelog for main by @nicholas-fedor in [#717](https://github.com/nicholas-fedor/watchtower/pull/717)
- Update changelog for main by @nicholas-fedor in [#715](https://github.com/nicholas-fedor/watchtower/pull/715)
- Update changelog for main by @nicholas-fedor in [#713](https://github.com/nicholas-fedor/watchtower/pull/713)
- Update changelog for main by @nicholas-fedor in [#710](https://github.com/nicholas-fedor/watchtower/pull/710)
- Update changelog for main by @nicholas-fedor in [#706](https://github.com/nicholas-fedor/watchtower/pull/706)
- Update changelog by @github-actions[bot] in [#693](https://github.com/nicholas-fedor/watchtower/pull/693)
- Correct spacing between author and PR reference by @nicholas-fedor in [#680](https://github.com/nicholas-fedor/watchtower/pull/680)
- Update Signal service documentation by @nicholas-fedor in [#679](https://github.com/nicholas-fedor/watchtower/pull/679)
- Add Signal notification support documentation by @nicholas-fedor in [#675](https://github.com/nicholas-fedor/watchtower/pull/675)
- Add dev as default version to suppress warning by @nicholas-fedor in [#633](https://github.com/nicholas-fedor/watchtower/pull/633)
- Update outdated warning condition to use site_url for dev by @nicholas-fedor in [#632](https://github.com/nicholas-fedor/watchtower/pull/632)
- Update outdated warning condition to use mike version by @nicholas-fedor in [#631](https://github.com/nicholas-fedor/watchtower/pull/631)
- Add null check for page in outdated warning condition by @nicholas-fedor in [#630](https://github.com/nicholas-fedor/watchtower/pull/630)
- Refine outdated warning condition for dev version by @nicholas-fedor in [#629](https://github.com/nicholas-fedor/watchtower/pull/629)
- Refine outdated warning condition for dev version by @nicholas-fedor in [#628](https://github.com/nicholas-fedor/watchtower/pull/628)
- Use root-relative URL for outdated warning link by @nicholas-fedor in [#627](https://github.com/nicholas-fedor/watchtower/pull/627)
- Hide version warning for dev by @nicholas-fedor in [#626](https://github.com/nicholas-fedor/watchtower/pull/626)
- Set default version to latest release by @nicholas-fedor in [#625](https://github.com/nicholas-fedor/watchtower/pull/625)

### Continuous Integration

- Resolve detached HEAD issue in changelog job by @nicholas-fedor in [#758](https://github.com/nicholas-fedor/watchtower/pull/758)
- Double-quote coverage output file by @nicholas-fedor in [#735](https://github.com/nicholas-fedor/watchtower/pull/735)
- Resolve CI test failures and WASM bug by @nicholas-fedor in [#734](https://github.com/nicholas-fedor/watchtower/pull/734)
- Pin go version by @nicholas-fedor in [#733](https://github.com/nicholas-fedor/watchtower/pull/733)
- Remove default commit signing by @nicholas-fedor in [#705](https://github.com/nicholas-fedor/watchtower/pull/705)
- Add gpg signing to changelog commit by @nicholas-fedor in [#703](https://github.com/nicholas-fedor/watchtower/pull/703)
- Update token to CHANGELOG_PAT by @nicholas-fedor in [#701](https://github.com/nicholas-fedor/watchtower/pull/701)
- Enable commit signing by @nicholas-fedor in [#692](https://github.com/nicholas-fedor/watchtower/pull/692)
- Add pr write permissions for changelog by @nicholas-fedor in [#688](https://github.com/nicholas-fedor/watchtower/pull/688)
- Add changelog update step by @nicholas-fedor in [#684](https://github.com/nicholas-fedor/watchtower/pull/684)
- Add automatic changelog generation using git-cliff by @nicholas-fedor in [#677](https://github.com/nicholas-fedor/watchtower/pull/677)
- Fix ShellCheck SC2086 Warnings in Publish Docs Workflow by @nicholas-fedor in [#624](https://github.com/nicholas-fedor/watchtower/pull/624)
- Fix ShellCheck SC2086 warnings in workflows by @nicholas-fedor in [#623](https://github.com/nicholas-fedor/watchtower/pull/623)
- Remove unnecessary SBOM upload step by @nicholas-fedor in [#622](https://github.com/nicholas-fedor/watchtower/pull/622)

### Chores

- Update module github.com/docker/cli to v28.5.0+incompatible by @renovate[bot] in [#750](https://github.com/nicholas-fedor/watchtower/pull/750)
- Update module github.com/docker/docker to v28.5.0+incompatible by @renovate[bot] in [#751](https://github.com/nicholas-fedor/watchtower/pull/751)
- Update module github.com/onsi/ginkgo/v2 to v2.26.0 by @renovate[bot] in [#749](https://github.com/nicholas-fedor/watchtower/pull/749)
- Update github/codeql-action digest to 64d10c1 by @renovate[bot] in [#748](https://github.com/nicholas-fedor/watchtower/pull/748)
- Update genproto dependencies by @nicholas-fedor in [#742](https://github.com/nicholas-fedor/watchtower/pull/742)
- Update peter-evans/create-pull-request digest to d3e081a by @renovate[bot] in [#739](https://github.com/nicholas-fedor/watchtower/pull/739)
- Update golangci/golangci-lint-action digest to 7409966 by @renovate[bot] in [#738](https://github.com/nicholas-fedor/watchtower/pull/738)
- Update docker/login-action digest to 5e57cd1 by @renovate[bot] in [#737](https://github.com/nicholas-fedor/watchtower/pull/737)
- Update module github.com/nicholas-fedor/shoutrrr to v0.10.0 by @renovate[bot] in [#727](https://github.com/nicholas-fedor/watchtower/pull/727)
- Update github/codeql-action digest to 3599b3b by @renovate[bot] in [#723](https://github.com/nicholas-fedor/watchtower/pull/723)
- Update actions/setup-python digest to 2e3e4b1 by @renovate[bot] in [#711](https://github.com/nicholas-fedor/watchtower/pull/711)
- Update securego/gosec action to v2.22.9 by @renovate[bot] in [#700](https://github.com/nicholas-fedor/watchtower/pull/700)
- Update github/codeql-action digest to 303c0ae by @renovate[bot] in [#698](https://github.com/nicholas-fedor/watchtower/pull/698)
- Update actions/cache digest to 0057852 by @renovate[bot] in [#695](https://github.com/nicholas-fedor/watchtower/pull/695)
- Update peter-evans/create-pull-request digest to 9ec683e by @renovate[bot] in [#694](https://github.com/nicholas-fedor/watchtower/pull/694)
- Update golangci/golangci-lint-action digest to f33eece by @renovate[bot] in [#682](https://github.com/nicholas-fedor/watchtower/pull/682)
- Update securego/gosec digest to f9c52aa by @renovate[bot] in [#681](https://github.com/nicholas-fedor/watchtower/pull/681)
- Pin orhun/git-cliff-action action to 98c9344 by @renovate[bot] in [#678](https://github.com/nicholas-fedor/watchtower/pull/678)
- Update module github.com/nicholas-fedor/shoutrrr to v0.9.1 by @renovate[bot] in [#676](https://github.com/nicholas-fedor/watchtower/pull/676)
- Update module github.com/nicholas-fedor/shoutrrr to v0.9.0 by @renovate[bot] in [#674](https://github.com/nicholas-fedor/watchtower/pull/674)
- Update actions/setup-python digest to 4267e28 by @renovate[bot] in [#665](https://github.com/nicholas-fedor/watchtower/pull/665)
- Update nicholas-fedor/govulncheck-action digest to 1e9ef2c by @renovate[bot] in [#663](https://github.com/nicholas-fedor/watchtower/pull/663)
- Update securego/gosec digest to 506407e by @renovate[bot] in [#664](https://github.com/nicholas-fedor/watchtower/pull/664)
- Update golangci/golangci-lint-action digest to f08454a by @renovate[bot] in [#662](https://github.com/nicholas-fedor/watchtower/pull/662)
- Update actions/setup-go digest to c0137ca by @renovate[bot] in [#661](https://github.com/nicholas-fedor/watchtower/pull/661)
- Update securego/gosec digest to 3ead143 by @renovate[bot] in [#660](https://github.com/nicholas-fedor/watchtower/pull/660)
- Update securego/gosec digest to e81fba3 by @renovate[bot] in [#659](https://github.com/nicholas-fedor/watchtower/pull/659)
- Update docker/login-action digest to 5b7b28b by @renovate[bot] in [#657](https://github.com/nicholas-fedor/watchtower/pull/657)
- Update docker/setup-qemu-action digest to e77e806 by @renovate[bot] in [#658](https://github.com/nicholas-fedor/watchtower/pull/658)
- Update actions/attest-build-provenance digest to 3752c92 by @renovate[bot] in [#655](https://github.com/nicholas-fedor/watchtower/pull/655)
- Update docker/login-action digest to 65c0768 by @renovate[bot] in [#656](https://github.com/nicholas-fedor/watchtower/pull/656)
- Update github/codeql-action digest to 192325c by @renovate[bot] in [#652](https://github.com/nicholas-fedor/watchtower/pull/652)
- Update github/codeql-action digest to d3678e2 by @renovate[bot] in [#651](https://github.com/nicholas-fedor/watchtower/pull/651)
- Update module github.com/spf13/viper to v1.21.0 by @renovate[bot] in [#650](https://github.com/nicholas-fedor/watchtower/pull/650)
- Update golangci/golangci-lint-action digest to 7574dab by @renovate[bot] in [#649](https://github.com/nicholas-fedor/watchtower/pull/649)
- Update docker/login-action digest to bdf14dc by @renovate[bot] in [#648](https://github.com/nicholas-fedor/watchtower/pull/648)
- Update securego/gosec digest to 4be6b11 by @renovate[bot] in [#647](https://github.com/nicholas-fedor/watchtower/pull/647)
- Update golangci/golangci-lint-action digest to dc56f00 by @renovate[bot] in [#646](https://github.com/nicholas-fedor/watchtower/pull/646)
- Update module golang.org/x/text to v0.29.0 by @renovate[bot] in [#645](https://github.com/nicholas-fedor/watchtower/pull/645)
- Update nicholas-fedor/go-proxy-pull-action digest to 5bb09e7 by @renovate[bot] in [#643](https://github.com/nicholas-fedor/watchtower/pull/643)
- Update module github.com/prometheus/client_golang to v1.23.2 by @renovate[bot] in [#641](https://github.com/nicholas-fedor/watchtower/pull/641)
- Update golang:alpine docker digest to b6ed3fd by @renovate[bot] in [#640](https://github.com/nicholas-fedor/watchtower/pull/640)
- Update github/codeql-action digest to f1f6e5f by @renovate[bot] in [#639](https://github.com/nicholas-fedor/watchtower/pull/639)
- Update module github.com/onsi/ginkgo/v2 to v2.25.3 by @renovate[bot] in [#637](https://github.com/nicholas-fedor/watchtower/pull/637)
- Update module github.com/prometheus/client_golang to v1.23.1 by @renovate[bot] in [#638](https://github.com/nicholas-fedor/watchtower/pull/638)
- Update cimg/go docker tag to v1.25.1 by @renovate[bot] in [#636](https://github.com/nicholas-fedor/watchtower/pull/636)
- Update codecov/codecov-action digest to 5a10915 by @renovate[bot] in [#635](https://github.com/nicholas-fedor/watchtower/pull/635)
- Update codecov/codecov-action digest to 206148c by @renovate[bot] in [#634](https://github.com/nicholas-fedor/watchtower/pull/634)

### New Contributors

- @github-actions[bot] made their first contribution in [#693](https://github.com/nicholas-fedor/watchtower/pull/693)

## [1.11.8] - 2025-09-04

### Documentation

- Improve content width and background consistency by @nicholas-fedor in [#586](https://github.com/nicholas-fedor/watchtower/pull/586)
- Update references to documentation site by @nicholas-fedor in [#585](https://github.com/nicholas-fedor/watchtower/pull/585)
- Implement mkdocs-mike versioning by @nicholas-fedor in [#581](https://github.com/nicholas-fedor/watchtower/pull/581)
- Re-add missing assets by @nicholas-fedor in [#580](https://github.com/nicholas-fedor/watchtower/pull/580)
- Correct docs pathing by @nicholas-fedor in [#579](https://github.com/nicholas-fedor/watchtower/pull/579)
- Resolve mkdocs pathing by @nicholas-fedor in [#577](https://github.com/nicholas-fedor/watchtower/pull/577)
- Overhaul documentation website by @nicholas-fedor in [#574](https://github.com/nicholas-fedor/watchtower/pull/574)

### Build

- Remove prefix from image tags by @nicholas-fedor in [#558](https://github.com/nicholas-fedor/watchtower/pull/558)

### Continuous Integration

- Correct pip requirements path by @nicholas-fedor in [#578](https://github.com/nicholas-fedor/watchtower/pull/578)
- Correct build directory by @nicholas-fedor in [#576](https://github.com/nicholas-fedor/watchtower/pull/576)

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.8.18 by @renovate[bot] in [#621](https://github.com/nicholas-fedor/watchtower/pull/621)
- Update nicholas-fedor/govulncheck-action digest to 6bacd52 by @renovate[bot] in [#620](https://github.com/nicholas-fedor/watchtower/pull/620)
- Update module github.com/docker/docker to v28.4.0+incompatible by @renovate[bot] in [#619](https://github.com/nicholas-fedor/watchtower/pull/619)
- Update module github.com/docker/cli to v28.4.0+incompatible by @renovate[bot] in [#618](https://github.com/nicholas-fedor/watchtower/pull/618)
- Update dependency path-filtering to v2.0.4 by @renovate[bot] in [#617](https://github.com/nicholas-fedor/watchtower/pull/617)
- Update dependency go to v1.25.1 by @renovate[bot] in [#616](https://github.com/nicholas-fedor/watchtower/pull/616)
- Update github/codeql-action digest to 2d92b76 by @renovate[bot] in [#613](https://github.com/nicholas-fedor/watchtower/pull/613)
- Update nicholas-fedor/go-proxy-pull-action digest to ca64499 by @renovate[bot] in [#615](https://github.com/nicholas-fedor/watchtower/pull/615)
- Update golang:alpine docker digest to 2ad042d by @renovate[bot] in [#614](https://github.com/nicholas-fedor/watchtower/pull/614)
- Update Go dependencies by @nicholas-fedor in [#612](https://github.com/nicholas-fedor/watchtower/pull/612)
- Update actions/setup-python digest to e797f83 by @renovate[bot] in [#611](https://github.com/nicholas-fedor/watchtower/pull/611)
- Update actions/setup-go digest to 4469467 by @renovate[bot] in [#610](https://github.com/nicholas-fedor/watchtower/pull/610)
- Update module github.com/spf13/pflag to v1.0.8 by @renovate[bot] in [#606](https://github.com/nicholas-fedor/watchtower/pull/606)
- Update actions/setup-python digest to 3d1e2d2 by @renovate[bot] in [#604](https://github.com/nicholas-fedor/watchtower/pull/604)
- Update nicholas-fedor/govulncheck-action digest to d4283df by @renovate[bot] in [#605](https://github.com/nicholas-fedor/watchtower/pull/605)
- Update actions/setup-go digest to 1d76b95 by @renovate[bot] in [#603](https://github.com/nicholas-fedor/watchtower/pull/603)
- Update actions/attest-build-provenance digest to 0b6e980 by @renovate[bot] in [#602](https://github.com/nicholas-fedor/watchtower/pull/602)
- Update module github.com/onsi/ginkgo/v2 to v2.25.2 by @renovate[bot] in [#599](https://github.com/nicholas-fedor/watchtower/pull/599)
- Update dependency path-filtering to v2.0.3 by @renovate[bot] in [#598](https://github.com/nicholas-fedor/watchtower/pull/598)
- Update module github.com/stretchr/testify to v1.11.1 by @renovate[bot] in [#597](https://github.com/nicholas-fedor/watchtower/pull/597)
- Update actions/attest-build-provenance digest to 864457a by @renovate[bot] in [#596](https://github.com/nicholas-fedor/watchtower/pull/596)
- Update actions/attest-build-provenance digest to 57aa2b0 by @renovate[bot] in [#595](https://github.com/nicholas-fedor/watchtower/pull/595)
- Update module github.com/onsi/gomega to v1.38.2 by @renovate[bot] in [#594](https://github.com/nicholas-fedor/watchtower/pull/594)
- Update golangci/golangci-lint-action digest to 3c28b2c by @renovate[bot] in [#593](https://github.com/nicholas-fedor/watchtower/pull/593)
- Update goreleaser/goreleaser-action digest to a08664b by @renovate[bot] in [#592](https://github.com/nicholas-fedor/watchtower/pull/592)
- Update actions/setup-python digest to 65b0712 by @renovate[bot] in [#591](https://github.com/nicholas-fedor/watchtower/pull/591)
- Update golangci/golangci-lint-action digest to d65369c by @renovate[bot] in [#590](https://github.com/nicholas-fedor/watchtower/pull/590)
- Update actions/attest-build-provenance digest to ff19f40 by @renovate[bot] in [#589](https://github.com/nicholas-fedor/watchtower/pull/589)
- Update securego/gosec digest to 5af1117 by @renovate[bot] in [#588](https://github.com/nicholas-fedor/watchtower/pull/588)
- Update module github.com/stretchr/testify to v1.11.0 by @renovate[bot] in [#587](https://github.com/nicholas-fedor/watchtower/pull/587)
- Update module github.com/onsi/gomega to v1.38.1 by @renovate[bot] in [#584](https://github.com/nicholas-fedor/watchtower/pull/584)
- Update golangci/golangci-lint-action digest to 030ca6c by @renovate[bot] in [#583](https://github.com/nicholas-fedor/watchtower/pull/583)
- Update module github.com/onsi/ginkgo/v2 to v2.25.1 by @renovate[bot] in [#582](https://github.com/nicholas-fedor/watchtower/pull/582)
- Update golangci/golangci-lint-action digest to c21e01f by @renovate[bot] in [#575](https://github.com/nicholas-fedor/watchtower/pull/575)
- Update github/codeql-action digest to 3c3833e by @renovate[bot] in [#573](https://github.com/nicholas-fedor/watchtower/pull/573)
- Update docker/setup-buildx-action digest to 1583c0f by @renovate[bot] in [#572](https://github.com/nicholas-fedor/watchtower/pull/572)
- Update module github.com/onsi/ginkgo/v2 to v2.25.0 by @renovate[bot] in [#571](https://github.com/nicholas-fedor/watchtower/pull/571)
- Update codecov/codecov-action digest to 3cb13a1 by @renovate[bot] in [#570](https://github.com/nicholas-fedor/watchtower/pull/570)
- Update codecov/codecov-action digest to fdcc847 by @renovate[bot] in [#569](https://github.com/nicholas-fedor/watchtower/pull/569)
- Update module github.com/onsi/ginkgo/v2 to v2.24.0 by @renovate[bot] in [#568](https://github.com/nicholas-fedor/watchtower/pull/568)
- Update github/codeql-action digest to 96f518a by @renovate[bot] in [#566](https://github.com/nicholas-fedor/watchtower/pull/566)
- Update docker/setup-buildx-action digest to 4cc794f by @renovate[bot] in [#565](https://github.com/nicholas-fedor/watchtower/pull/565)
- Update securego/gosec digest to 287b46c by @renovate[bot] in [#564](https://github.com/nicholas-fedor/watchtower/pull/564)
- Update codecov/codecov-action digest to 39a2af1 by @renovate[bot] in [#563](https://github.com/nicholas-fedor/watchtower/pull/563)
- Update nicholas-fedor/go-proxy-pull-action digest to 40d406b by @renovate[bot] in [#561](https://github.com/nicholas-fedor/watchtower/pull/561)
- Update golang:alpine docker digest to f18a072 by @renovate[bot] in [#560](https://github.com/nicholas-fedor/watchtower/pull/560)
- Bump Go version to v1.25.x by @nicholas-fedor in [#559](https://github.com/nicholas-fedor/watchtower/pull/559)
- Update dependency python to v3.13.7 by @renovate[bot] in [#557](https://github.com/nicholas-fedor/watchtower/pull/557)
- Update actions/attest-build-provenance digest to 8bd83f1 by @renovate[bot] in [#556](https://github.com/nicholas-fedor/watchtower/pull/556)
- Update cimg/go docker tag to v1.25.0 by @renovate[bot] in [#555](https://github.com/nicholas-fedor/watchtower/pull/555)
- Update securego/gosec digest to cee0aea by @renovate[bot] in [#554](https://github.com/nicholas-fedor/watchtower/pull/554)
- Update goreleaser/goreleaser-action digest to 35b9a27 by @renovate[bot] in [#553](https://github.com/nicholas-fedor/watchtower/pull/553)
- Update golang:alpine docker digest to 244baa3 by @renovate[bot] in [#551](https://github.com/nicholas-fedor/watchtower/pull/551)
- Update nicholas-fedor/go-proxy-pull-action digest to 46417d8 by @renovate[bot] in [#552](https://github.com/nicholas-fedor/watchtower/pull/552)
- Update nicholas-fedor/govulncheck-action digest to 1862128 by @renovate[bot] in [#550](https://github.com/nicholas-fedor/watchtower/pull/550)
- Update golang:alpine docker digest to fa0c7cf by @renovate[bot] in [#549](https://github.com/nicholas-fedor/watchtower/pull/549)
- Update golang:alpine docker digest to b917bea by @renovate[bot] in [#547](https://github.com/nicholas-fedor/watchtower/pull/547)
- Update nicholas-fedor/go-proxy-pull-action digest to 7740eae by @renovate[bot] in [#548](https://github.com/nicholas-fedor/watchtower/pull/548)
- Update actions/setup-go digest to e75c3e8 by @renovate[bot] in [#546](https://github.com/nicholas-fedor/watchtower/pull/546)
- Update actions/checkout digest to ff7abcd by @renovate[bot] in [#545](https://github.com/nicholas-fedor/watchtower/pull/545)
- Update dependency go to v1.25.0 by @renovate[bot] in [#544](https://github.com/nicholas-fedor/watchtower/pull/544)
- Update github/codeql-action digest to df55935 by @renovate[bot] in [#542](https://github.com/nicholas-fedor/watchtower/pull/542)
- Update cimg/go docker tag to v1.24.6 by @renovate[bot] in [#541](https://github.com/nicholas-fedor/watchtower/pull/541)

## [1.11.7] - 2025-08-11

### Added

- Enable Go version updates with gomodTidy by @nicholas-fedor in [#469](https://github.com/nicholas-fedor/watchtower/pull/469)
- Enable build provenance attestations and workflow optimizations by @nicholas-fedor in [#410](https://github.com/nicholas-fedor/watchtower/pull/410)

### Fixed

- Enhance SMTP configuration with timeout constant and URL parameters by @nicholas-fedor in [#527](https://github.com/nicholas-fedor/watchtower/pull/527)
- Streamline StopContainer implementation by @nicholas-fedor in [#504](https://github.com/nicholas-fedor/watchtower/pull/504)
- Remove newlines in goreleaser arguments by @nicholas-fedor in [#491](https://github.com/nicholas-fedor/watchtower/pull/491)
- Use chore for go dependency updates by @nicholas-fedor in [#481](https://github.com/nicholas-fedor/watchtower/pull/481)
- Handle undefined dry-run input in build workflow by @nicholas-fedor in [#480](https://github.com/nicholas-fedor/watchtower/pull/480)
- Resolve unexpected value errors in release workflows by @nicholas-fedor in [#475](https://github.com/nicholas-fedor/watchtower/pull/475)
- Enable dynamic dev build versioning by @nicholas-fedor in [#474](https://github.com/nicholas-fedor/watchtower/pull/474)
- Disable CGO for static binaries by @nicholas-fedor in [#466](https://github.com/nicholas-fedor/watchtower/pull/466)
- Correct binary copy path in Dockerfile by @nicholas-fedor in [#465](https://github.com/nicholas-fedor/watchtower/pull/465)
- Add Syft installation step for prod builds by @nicholas-fedor in [#461](https://github.com/nicholas-fedor/watchtower/pull/461)
- Add package write permissions and fix Dockerfile base image label by @nicholas-fedor in [#460](https://github.com/nicholas-fedor/watchtower/pull/460)
- Resolve docker build error and enable attestations with imagetools by @nicholas-fedor in [#458](https://github.com/nicholas-fedor/watchtower/pull/458)
- Add architecture variants to builds and dockers by @nicholas-fedor in [#457](https://github.com/nicholas-fedor/watchtower/pull/457)
- Align dockers with builds and update Dockerfile for binary paths by @nicholas-fedor in [#456](https://github.com/nicholas-fedor/watchtower/pull/456)
- Revert to output type and fix conditional syntax by @nicholas-fedor in [#455](https://github.com/nicholas-fedor/watchtower/pull/455)
- Use --load for dry-run and --push for releases by @nicholas-fedor in [#454](https://github.com/nicholas-fedor/watchtower/pull/454)
- Skip attestations in dry-run mode by @nicholas-fedor in [#453](https://github.com/nicholas-fedor/watchtower/pull/453)
- Enable dry-run builds with conditional output type by @nicholas-fedor in [#452](https://github.com/nicholas-fedor/watchtower/pull/452)
- Update GoReleaser version to ~> v2 to support goriscv64 by @nicholas-fedor in [#451](https://github.com/nicholas-fedor/watchtower/pull/451)
- Add attestations and id-token permissions to release-dev by @nicholas-fedor in [#450](https://github.com/nicholas-fedor/watchtower/pull/450)
- Refactor build workflows and correct output types by @nicholas-fedor in [#449](https://github.com/nicholas-fedor/watchtower/pull/449)
- Use docker buildx imagetools for manifest creation with attestations by @nicholas-fedor in [#439](https://github.com/nicholas-fedor/watchtower/pull/439)
- Add password-stdin to Docker Hub auth verify in dev workflow by @nicholas-fedor in [#435](https://github.com/nicholas-fedor/watchtower/pull/435)
- Add Docker Hub token to attestation steps in dev workflow by @nicholas-fedor in [#434](https://github.com/nicholas-fedor/watchtower/pull/434)
- Remove docker:// prefix from SBOM image references in dev workflow by @nicholas-fedor in [#433](https://github.com/nicholas-fedor/watchtower/pull/433)
- Pull images locally for SBOM scan in dev workflow by @nicholas-fedor in [#432](https://github.com/nicholas-fedor/watchtower/pull/432)
- Add registry creds to SBOM action for direct scan by @nicholas-fedor in [#431](https://github.com/nicholas-fedor/watchtower/pull/431)
- Install Syft for SBOM generation in prod workflow by @nicholas-fedor in [#427](https://github.com/nicholas-fedor/watchtower/pull/427)
- Update GoReleaser dry run flags in prod workflow by @nicholas-fedor in [#426](https://github.com/nicholas-fedor/watchtower/pull/426)
- Skip docs update and artifact uploads in prod dry run by @nicholas-fedor in [#425](https://github.com/nicholas-fedor/watchtower/pull/425)
- Add default GO variants to norm_arch in dev workflow by @nicholas-fedor in [#424](https://github.com/nicholas-fedor/watchtower/pull/424)
- Correct norm_arch for ARMv6 in dev workflow by @nicholas-fedor in [#423](https://github.com/nicholas-fedor/watchtower/pull/423)
- Remove dist/ from binary COPY in dev Dockerfile by @nicholas-fedor in [#422](https://github.com/nicholas-fedor/watchtower/pull/422)
- Add build prefix handling in dev workflow and Dockerfile by @nicholas-fedor in [#421](https://github.com/nicholas-fedor/watchtower/pull/421)
- Correct artifact download path and ARMv6 arch in dev workflow by @nicholas-fedor in [#420](https://github.com/nicholas-fedor/watchtower/pull/420)
- Add unique IDs to GoReleaser build configs by @nicholas-fedor in [#419](https://github.com/nicholas-fedor/watchtower/pull/419)
- Remove arm/v6 by @nicholas-fedor in [#415](https://github.com/nicholas-fedor/watchtower/pull/415)
- Exclude test files from gosec scanning by removing -tests flag by @nicholas-fedor in [#411](https://github.com/nicholas-fedor/watchtower/pull/411)

### Changed

- Inline registry logins in workflows by @nicholas-fedor in [#430](https://github.com/nicholas-fedor/watchtower/pull/430)
- Reorganize build system, workflows, and related files by @nicholas-fedor in [#416](https://github.com/nicholas-fedor/watchtower/pull/416)
- * feat(circleci): add path-filtering orb to conditionally run tests only on Go file changes by @nicholas-fedor in [#412](https://github.com/nicholas-fedor/watchtower/pull/412)

### Continuous Integration

- Use explicit Buildx outputs for dev and prod builds by @nicholas-fedor in [#448](https://github.com/nicholas-fedor/watchtower/pull/448)
- Use oci output for goreleaser attestations by @nicholas-fedor in [#447](https://github.com/nicholas-fedor/watchtower/pull/447)
- Use oci exporter for goreleaser attestations by @nicholas-fedor in [#445](https://github.com/nicholas-fedor/watchtower/pull/445)
- Add buildx setup to enable attestations in release workflows by @nicholas-fedor in [#444](https://github.com/nicholas-fedor/watchtower/pull/444)
- Align release workflows with GoReleaser and simplify Dockerfiles by @nicholas-fedor in [#441](https://github.com/nicholas-fedor/watchtower/pull/441)
- Add push trigger and test job to dev release workflow by @nicholas-fedor in [#440](https://github.com/nicholas-fedor/watchtower/pull/440)
- Simplify dev image release workflow by removing GoReleaser by @nicholas-fedor in [#436](https://github.com/nicholas-fedor/watchtower/pull/436)

### Chores

- Update actions/checkout action to v5 by @renovate[bot] in [#540](https://github.com/nicholas-fedor/watchtower/pull/540)
- Update securego/gosec digest to ef7adab by @renovate[bot] in [#539](https://github.com/nicholas-fedor/watchtower/pull/539)
- Update nicholas-fedor/govulncheck-action digest to ae17d3c by @renovate[bot] in [#538](https://github.com/nicholas-fedor/watchtower/pull/538)
- Update golangci/golangci-lint-action digest to 9511564 by @renovate[bot] in [#537](https://github.com/nicholas-fedor/watchtower/pull/537)
- Update actions/attest-build-provenance digest to f0878de by @renovate[bot] in [#536](https://github.com/nicholas-fedor/watchtower/pull/536)
- Update actions/checkout digest by @renovate[bot] in [#533](https://github.com/nicholas-fedor/watchtower/pull/533)
- Update docker/setup-buildx-action digest to af1b253 by @renovate[bot] in [#534](https://github.com/nicholas-fedor/watchtower/pull/534)
- Update securego/gosec digest to e201bb8 by @renovate[bot] in [#532](https://github.com/nicholas-fedor/watchtower/pull/532)
- Update actions/checkout digest to 08eba0b by @renovate[bot] in [#531](https://github.com/nicholas-fedor/watchtower/pull/531)
- Upgrade build step to contents;write permission by @nicholas-fedor in [#530](https://github.com/nicholas-fedor/watchtower/pull/530)
- Update google.golang.org/genproto modules by @nicholas-fedor in [#529](https://github.com/nicholas-fedor/watchtower/pull/529)
- Correct permissions and sbom generation by @nicholas-fedor in [#528](https://github.com/nicholas-fedor/watchtower/pull/528)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.17 by @renovate[bot] in [#526](https://github.com/nicholas-fedor/watchtower/pull/526)
- Add ARG BASE_IMAGE in scratch stage by @nicholas-fedor in [#525](https://github.com/nicholas-fedor/watchtower/pull/525)
- Update github/codeql-action digest to 76621b6 by @renovate[bot] in [#522](https://github.com/nicholas-fedor/watchtower/pull/522)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.16 by @renovate[bot] in [#524](https://github.com/nicholas-fedor/watchtower/pull/524)
- Update module golang.org/x/text to v0.28.0 by @renovate[bot] in [#523](https://github.com/nicholas-fedor/watchtower/pull/523)
- Update module github.com/docker/go-connections to v0.6.0 by @renovate[bot] in [#521](https://github.com/nicholas-fedor/watchtower/pull/521)
- Update github/codeql-action digest to a4e1a01 by @renovate[bot] in [#520](https://github.com/nicholas-fedor/watchtower/pull/520)
- Update docker/setup-buildx-action digest to 2c8bcda by @renovate[bot] in [#519](https://github.com/nicholas-fedor/watchtower/pull/519)
- Update dependency python to v3.13.6 by @renovate[bot] in [#518](https://github.com/nicholas-fedor/watchtower/pull/518)
- Update goreleaser/goreleaser-action digest to e435ccd by @renovate[bot] in [#516](https://github.com/nicholas-fedor/watchtower/pull/516)
- Update nicholas-fedor/go-proxy-pull-action digest to ba81e12 by @renovate[bot] in [#517](https://github.com/nicholas-fedor/watchtower/pull/517)
- Update actions/attest-build-provenance digest to 463e6df by @renovate[bot] in [#514](https://github.com/nicholas-fedor/watchtower/pull/514)
- Update golang:alpine docker digest to c8c5f95 by @renovate[bot] in [#515](https://github.com/nicholas-fedor/watchtower/pull/515)
- Update dependency go to v1.24.6 by @renovate[bot] in [#513](https://github.com/nicholas-fedor/watchtower/pull/513)
- Update golangci/golangci-lint-action digest to f9e969a by @renovate[bot] in [#512](https://github.com/nicholas-fedor/watchtower/pull/512)
- Update docker/setup-buildx-action digest to c65d441 by @renovate[bot] in [#511](https://github.com/nicholas-fedor/watchtower/pull/511)
- Update docker/setup-buildx-action digest to 774224a by @renovate[bot] in [#510](https://github.com/nicholas-fedor/watchtower/pull/510)
- Update docker/setup-buildx-action digest to ae7d689 by @renovate[bot] in [#509](https://github.com/nicholas-fedor/watchtower/pull/509)
- Update actions/setup-python digest to 9322b3c by @renovate[bot] in [#508](https://github.com/nicholas-fedor/watchtower/pull/508)
- Update docker/login-action digest to 184bdaa by @renovate[bot] in [#507](https://github.com/nicholas-fedor/watchtower/pull/507)
- Update goreleaser/goreleaser-action digest to 2ff5850 by @renovate[bot] in [#506](https://github.com/nicholas-fedor/watchtower/pull/506)
- Update goreleaser/goreleaser-action digest to 9a6cd01 by @renovate[bot] in [#505](https://github.com/nicholas-fedor/watchtower/pull/505)
- Update goreleaser/goreleaser-action digest to ca48102 by @renovate[bot] in [#503](https://github.com/nicholas-fedor/watchtower/pull/503)
- Update docker/login-action digest to ef38ec3 by @renovate[bot] in [#502](https://github.com/nicholas-fedor/watchtower/pull/502)
- Update module github.com/prometheus/client_golang to v1.23.0 by @renovate[bot] in [#500](https://github.com/nicholas-fedor/watchtower/pull/500)
- Update actions/setup-python digest to fbeb884 by @renovate[bot] in [#499](https://github.com/nicholas-fedor/watchtower/pull/499)
- Update github/codeql-action digest to 51f7732 by @renovate[bot] in [#498](https://github.com/nicholas-fedor/watchtower/pull/498)
- Update actions/attest-build-provenance digest to fef91c1 by @renovate[bot] in [#496](https://github.com/nicholas-fedor/watchtower/pull/496)
- Update actions/setup-python digest to 03bb615 by @renovate[bot] in [#497](https://github.com/nicholas-fedor/watchtower/pull/497)
- Update module github.com/docker/docker to v28.3.3+incompatible by @renovate[bot] in [#495](https://github.com/nicholas-fedor/watchtower/pull/495)
- Update module github.com/docker/cli to v28.3.3+incompatible by @renovate[bot] in [#494](https://github.com/nicholas-fedor/watchtower/pull/494)
- Minor prod.yaml corrections by @nicholas-fedor in [#493](https://github.com/nicholas-fedor/watchtower/pull/493)
- Remove goreleaser binary build id and capitalization from project name by @nicholas-fedor in [#492](https://github.com/nicholas-fedor/watchtower/pull/492)
- Update actions/attest-build-provenance digest to 961f313 by @renovate[bot] in [#489](https://github.com/nicholas-fedor/watchtower/pull/489)
- Enhance GoReleaser configurations and workflow documentation by @nicholas-fedor in [#490](https://github.com/nicholas-fedor/watchtower/pull/490)
- Update securego/gosec digest to ba592af by @renovate[bot] in [#488](https://github.com/nicholas-fedor/watchtower/pull/488)
- Minor workflow corrections by @nicholas-fedor in [#486](https://github.com/nicholas-fedor/watchtower/pull/486)
- Update actions/setup-python digest to 36da51d by @renovate[bot] in [#485](https://github.com/nicholas-fedor/watchtower/pull/485)
- Update dependency path-filtering to v2.0.2 by @renovate[bot] in [#484](https://github.com/nicholas-fedor/watchtower/pull/484)
- Update nicholas-fedor/govulncheck-action digest to affabe3 by @renovate[bot] in [#483](https://github.com/nicholas-fedor/watchtower/pull/483)
- Update actions/setup-python digest to 3c6f142 by @renovate[bot] in [#482](https://github.com/nicholas-fedor/watchtower/pull/482)
- Add snapshot name template to dev configuration by @nicholas-fedor in [#479](https://github.com/nicholas-fedor/watchtower/pull/479)
- Update github/codeql-action digest to 4e828ff by @renovate[bot] in [#478](https://github.com/nicholas-fedor/watchtower/pull/478)
- Update actions/checkout digest to 8edcb1b by @renovate[bot] in [#477](https://github.com/nicholas-fedor/watchtower/pull/477)
- Add .dockerignore to optimize Docker build context by @nicholas-fedor in [#476](https://github.com/nicholas-fedor/watchtower/pull/476)
- Update module github.com/onsi/gomega to v1.38.0 by @renovate[bot] in [#473](https://github.com/nicholas-fedor/watchtower/pull/473)
- Update actions/attest-build-provenance digest to 7a3eb4a by @renovate[bot] in [#472](https://github.com/nicholas-fedor/watchtower/pull/472)
- Update actions/attest-build-provenance digest to fe74bb2 by @renovate[bot] in [#471](https://github.com/nicholas-fedor/watchtower/pull/471)
- Update dependency go to v1.24.5 by @renovate[bot] in [#470](https://github.com/nicholas-fedor/watchtower/pull/470)
- Update securego/gosec digest to 2ef6017 by @renovate[bot] in [#468](https://github.com/nicholas-fedor/watchtower/pull/468)
- Update actions/setup-python digest to 88ffd4d by @renovate[bot] in [#467](https://github.com/nicholas-fedor/watchtower/pull/467)
- Update securego/gosec digest to 6ea6b35 by @renovate[bot] in [#464](https://github.com/nicholas-fedor/watchtower/pull/464)
- Update github/codeql-action digest to d6bbdef by @renovate[bot] in [#463](https://github.com/nicholas-fedor/watchtower/pull/463)
- Update .gitignore with comprehensive Go, VSCode, and mkdocs patterns by @nicholas-fedor in [#462](https://github.com/nicholas-fedor/watchtower/pull/462)
- Update securego/gosec digest to 925741b by @renovate[bot] in [#459](https://github.com/nicholas-fedor/watchtower/pull/459)
- Update docker/setup-qemu-action digest to 05340d1 by @renovate[bot] in [#446](https://github.com/nicholas-fedor/watchtower/pull/446)
- Update actions/upload-artifact digest to de65e23 by @renovate[bot] in [#443](https://github.com/nicholas-fedor/watchtower/pull/443)
- Update actions/attest-build-provenance digest to f923cf6 by @renovate[bot] in [#442](https://github.com/nicholas-fedor/watchtower/pull/442)
- Update docker/login-action digest to 3d10084 by @renovate[bot] in [#437](https://github.com/nicholas-fedor/watchtower/pull/437)
- Update docker/setup-qemu-action digest to 05340d1 by @renovate[bot] in [#438](https://github.com/nicholas-fedor/watchtower/pull/438)
- Update go dependencies by @nicholas-fedor in [#429](https://github.com/nicholas-fedor/watchtower/pull/429)
- Pin dependencies by @renovate[bot] in [#417](https://github.com/nicholas-fedor/watchtower/pull/417)
- Update anchore/sbom-action digest to 9e07fd7 by @renovate[bot] in [#418](https://github.com/nicholas-fedor/watchtower/pull/418)
- Update nginx:alpine docker digest to d67ea0d by @renovate[bot] in [#413](https://github.com/nicholas-fedor/watchtower/pull/413)
- Update nicholas-fedor/go-proxy-pull-action digest to c1e755b by @renovate[bot] in [#409](https://github.com/nicholas-fedor/watchtower/pull/409)
- Update golang:alpine docker digest to daae04e by @renovate[bot] in [#408](https://github.com/nicholas-fedor/watchtower/pull/408)

## [1.11.6] - 2025-07-16

### Fixed

- Restore proxy, DialContext, and redirect handling in NewAuthClient by @nicholas-fedor in [#403](https://github.com/nicholas-fedor/watchtower/pull/403)

### Documentation

- Add REPO_USER, REPO_PASS, DOCKER_CONFIG and update REPO_PASS note by @nicholas-fedor in [#393](https://github.com/nicholas-fedor/watchtower/pull/393)

### Chores

- Update module github.com/spf13/pflag to v1.0.7 by @renovate[bot] in [#407](https://github.com/nicholas-fedor/watchtower/pull/407)
- Update actions/setup-go digest to 8e57b58 by @renovate[bot] in [#406](https://github.com/nicholas-fedor/watchtower/pull/406)
- Update alpine docker tag to v3.22.1 by @renovate[bot] in [#405](https://github.com/nicholas-fedor/watchtower/pull/405)
- Update nginx:alpine docker digest to f741b7f by @renovate[bot] in [#404](https://github.com/nicholas-fedor/watchtower/pull/404)
- Update nicholas-fedor/go-proxy-pull-action digest to d2df5a3 by @renovate[bot] in [#401](https://github.com/nicholas-fedor/watchtower/pull/401)
- Update nginx:alpine docker digest to 2ce90e4 by @renovate[bot] in [#400](https://github.com/nicholas-fedor/watchtower/pull/400)
- Update golang:alpine docker digest to 48ee313 by @renovate[bot] in [#398](https://github.com/nicholas-fedor/watchtower/pull/398)
- Update nginx:alpine docker digest to 186168f by @renovate[bot] in [#399](https://github.com/nicholas-fedor/watchtower/pull/399)
- Update golang:alpine docker digest to d3150d8 by @renovate[bot] in [#397](https://github.com/nicholas-fedor/watchtower/pull/397)
- Update nginx:1.29.0 docker digest to f5c017f by @renovate[bot] in [#395](https://github.com/nicholas-fedor/watchtower/pull/395)
- Update nginx docker digest to f5c017f by @renovate[bot] in [#394](https://github.com/nicholas-fedor/watchtower/pull/394)
- Update nginx:1.29.0 docker digest to 63f92a6 by @renovate[bot] in [#392](https://github.com/nicholas-fedor/watchtower/pull/392)
- Update nginx docker digest to 63f92a6 by @renovate[bot] in [#391](https://github.com/nicholas-fedor/watchtower/pull/391)
- Update prom/prometheus docker digest to 63805eb by @renovate[bot] in [#390](https://github.com/nicholas-fedor/watchtower/pull/390)
- Update golangci/golangci-lint-action digest to 3d16f46 by @renovate[bot] in [#388](https://github.com/nicholas-fedor/watchtower/pull/388)
- Update actions/setup-go digest to 7c0b336 by @renovate[bot] in [#385](https://github.com/nicholas-fedor/watchtower/pull/385)
- Update module golang.org/x/text to v0.27.0 by @renovate[bot] in [#384](https://github.com/nicholas-fedor/watchtower/pull/384)
- Update module github.com/docker/docker to v28.3.2+incompatible by @renovate[bot] in [#383](https://github.com/nicholas-fedor/watchtower/pull/383)
- Update cimg/go docker tag to v1.24.5 by @renovate[bot] in [#382](https://github.com/nicholas-fedor/watchtower/pull/382)
- Update nicholas-fedor/go-proxy-pull-action digest to 882cfc4 by @renovate[bot] in [#381](https://github.com/nicholas-fedor/watchtower/pull/381)
- Update cimg/go:1.24.4 docker digest to fbe3a29 by @renovate[bot] in [#380](https://github.com/nicholas-fedor/watchtower/pull/380)
- Update module github.com/docker/cli to v28.3.2+incompatible by @renovate[bot] in [#379](https://github.com/nicholas-fedor/watchtower/pull/379)
- Update cimg/go:1.24.4 docker digest to 8ba7a25 by @renovate[bot] in [#378](https://github.com/nicholas-fedor/watchtower/pull/378)
- Update golang:alpine docker digest to ddf5200 by @renovate[bot] in [#377](https://github.com/nicholas-fedor/watchtower/pull/377)
- Update actions/setup-go digest to 6f26dcc by @renovate[bot] in [#376](https://github.com/nicholas-fedor/watchtower/pull/376)
- Update actions/setup-go digest to 8d4083a by @renovate[bot] in [#375](https://github.com/nicholas-fedor/watchtower/pull/375)
- Update golangci/golangci-lint-action digest to cbc80ac by @renovate[bot] in [#374](https://github.com/nicholas-fedor/watchtower/pull/374)
- Update goreleaser/goreleaser-action digest to 0931acf by @renovate[bot] in [#373](https://github.com/nicholas-fedor/watchtower/pull/373)
- Update actions/setup-python digest to 532b046 by @renovate[bot] in [#372](https://github.com/nicholas-fedor/watchtower/pull/372)
- Update module github.com/docker/cli to v28.3.1+incompatible by @renovate[bot] in [#370](https://github.com/nicholas-fedor/watchtower/pull/370)
- Update module github.com/docker/docker to v28.3.1+incompatible by @renovate[bot] in [#371](https://github.com/nicholas-fedor/watchtower/pull/371)

## [1.11.5] - 2025-07-01

### Fixed

- Handle unauthenticated registries and update linting by @nicholas-fedor in [#369](https://github.com/nicholas-fedor/watchtower/pull/369)

### Chores

- Update nginx:1.29.0 docker digest to 93230cd by @renovate[bot] in [#368](https://github.com/nicholas-fedor/watchtower/pull/368)
- Update nginx docker digest to 93230cd by @renovate[bot] in [#367](https://github.com/nicholas-fedor/watchtower/pull/367)

## [1.11.4] - 2025-07-01

### Fixed

- Fix registry redirect handling for image updates by @nicholas-fedor in [#359](https://github.com/nicholas-fedor/watchtower/pull/359)
- Resolve update failures of containers with multiple networks by @nicholas-fedor in [#351](https://github.com/nicholas-fedor/watchtower/pull/351)
- Ensure pinned container images are skipped during updates by @nicholas-fedor in [#347](https://github.com/nicholas-fedor/watchtower/pull/347)
- Increase default timeout to 30s and demote timeout log to debug by @nicholas-fedor in [#325](https://github.com/nicholas-fedor/watchtower/pull/325)

### Documentation

- Enhance usage examples in doc.go by @nicholas-fedor in [#344](https://github.com/nicholas-fedor/watchtower/pull/344)

### Chores

- Update nginx:1.29.0 docker digest to c8a4413 by @renovate[bot] in [#365](https://github.com/nicholas-fedor/watchtower/pull/365)
- Update nginx docker digest to c8a4413 by @renovate[bot] in [#364](https://github.com/nicholas-fedor/watchtower/pull/364)
- Update golangci/golangci-lint-action digest to 4f58623 by @renovate[bot] in [#363](https://github.com/nicholas-fedor/watchtower/pull/363)
- Update prom/prometheus docker digest to 7a34573 by @renovate[bot] in [#362](https://github.com/nicholas-fedor/watchtower/pull/362)
- Update golangci/golangci-lint-action digest to f509bac by @renovate[bot] in [#361](https://github.com/nicholas-fedor/watchtower/pull/361)
- Update prom/prometheus docker digest to 3b1d5be by @renovate[bot] in [#360](https://github.com/nicholas-fedor/watchtower/pull/360)
- Update codecov/codecov-action digest to 2db07e3 by @renovate[bot] in [#358](https://github.com/nicholas-fedor/watchtower/pull/358)
- Update module github.com/docker/cli to v28.3.0+incompatible by @renovate[bot] in [#356](https://github.com/nicholas-fedor/watchtower/pull/356)
- Update module github.com/docker/docker to v28.3.0+incompatible by @renovate[bot] in [#357](https://github.com/nicholas-fedor/watchtower/pull/357)
- Update nginx docker tag to v1.29.0 by @renovate[bot] in [#355](https://github.com/nicholas-fedor/watchtower/pull/355)
- Update actions/setup-python digest to 1264885 by @renovate[bot] in [#354](https://github.com/nicholas-fedor/watchtower/pull/354)
- Update nginx:alpine docker digest to b2e814d by @renovate[bot] in [#353](https://github.com/nicholas-fedor/watchtower/pull/353)
- Update nginx docker digest to dc53c8f by @renovate[bot] in [#352](https://github.com/nicholas-fedor/watchtower/pull/352)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.15 by @renovate[bot] in [#349](https://github.com/nicholas-fedor/watchtower/pull/349)
- Update golangci/golangci-lint-action digest to 8861dcf by @renovate[bot] in [#348](https://github.com/nicholas-fedor/watchtower/pull/348)
- Refactor `SliceSubtract` and update comment by @nicholas-fedor in [#343](https://github.com/nicholas-fedor/watchtower/pull/343)
- Add .gitattributes file by @nicholas-fedor in [#342](https://github.com/nicholas-fedor/watchtower/pull/342)
- Update actions/setup-python digest to e9c40fb by @renovate[bot] in [#341](https://github.com/nicholas-fedor/watchtower/pull/341)
- Update actions/setup-python digest to 5fa0ee6 by @renovate[bot] in [#340](https://github.com/nicholas-fedor/watchtower/pull/340)
- Update actions/setup-go digest to fa96338 by @renovate[bot] in [#339](https://github.com/nicholas-fedor/watchtower/pull/339)
- Update docker/setup-buildx-action digest to e468171 by @renovate[bot] in [#338](https://github.com/nicholas-fedor/watchtower/pull/338)
- Update grafana/grafana docker digest to b5b59bf by @renovate[bot] in [#337](https://github.com/nicholas-fedor/watchtower/pull/337)
- Update golangci/golangci-lint-action digest to dee96ac by @renovate[bot] in [#335](https://github.com/nicholas-fedor/watchtower/pull/335)
- Update nicholas-fedor/go-proxy-pull-action digest to 0aec514 by @renovate[bot] in [#336](https://github.com/nicholas-fedor/watchtower/pull/336)
- Update config by @nicholas-fedor in [#334](https://github.com/nicholas-fedor/watchtower/pull/334)
- Update docker/setup-qemu-action digest to 05340d1 by @renovate[bot] in [#332](https://github.com/nicholas-fedor/watchtower/pull/332)
- Update docker/setup-buildx-action digest to 18ce135 by @renovate[bot] in [#331](https://github.com/nicholas-fedor/watchtower/pull/331)
- Update docker/setup-buildx-action digest to 6229134 by @renovate[bot] in [#330](https://github.com/nicholas-fedor/watchtower/pull/330)
- Update docker/login-action digest to 3d10084 by @renovate[bot] in [#329](https://github.com/nicholas-fedor/watchtower/pull/329)
- Update grafana/grafana docker digest to 63ef313 by @renovate[bot] in [#328](https://github.com/nicholas-fedor/watchtower/pull/328)
- Update github.com/google/pprof by @nicholas-fedor in [#327](https://github.com/nicholas-fedor/watchtower/pull/327)
- Update dependency python to v3.13.5 by @renovate[bot] in [#326](https://github.com/nicholas-fedor/watchtower/pull/326)

## [1.11.3] - 2025-06-11

### Added

- Add riscv64 architecture support by @nicholas-fedor in [#318](https://github.com/nicholas-fedor/watchtower/pull/318)

### Fixed

- Resolve premature image cleanup by @nicholas-fedor in [#321](https://github.com/nicholas-fedor/watchtower/pull/321)
- Demote log messages to debug by @nicholas-fedor in [#315](https://github.com/nicholas-fedor/watchtower/pull/315)

### Chores

- Update nginx:1.28.0 docker digest to 20555a0 by @renovate[bot] in [#324](https://github.com/nicholas-fedor/watchtower/pull/324)
- Update actions/setup-go digest to 4de67c0 by @renovate[bot] in [#323](https://github.com/nicholas-fedor/watchtower/pull/323)
- Update actions/attest-build-provenance digest to e8998f9 by @renovate[bot] in [#322](https://github.com/nicholas-fedor/watchtower/pull/322)
- Update nginx:1.28.0 docker digest to 1fd589a by @renovate[bot] in [#320](https://github.com/nicholas-fedor/watchtower/pull/320)
- Update nginx docker digest to 6784fb0 by @renovate[bot] in [#319](https://github.com/nicholas-fedor/watchtower/pull/319)
- Update golangci/golangci-lint-action digest to cf2fd4c by @renovate[bot] in [#316](https://github.com/nicholas-fedor/watchtower/pull/316)

## [1.11.2] - 2025-06-10

### Fixed

- Reduce MAC address warning to debug for non-running containers by @nicholas-fedor in [#314](https://github.com/nicholas-fedor/watchtower/pull/314)

## [1.11.1] - 2025-06-09

### Fixed

- Reintroduce option to skip tls verification by @nicholas-fedor in [#312](https://github.com/nicholas-fedor/watchtower/pull/312)

### Chores

- Update cimg/go docker tag to v1.24.4 by @renovate[bot] in [#311](https://github.com/nicholas-fedor/watchtower/pull/311)
- Update golangci/golangci-lint-action digest to 09dada9 by @renovate[bot] in [#310](https://github.com/nicholas-fedor/watchtower/pull/310)
- Update nicholas-fedor/go-proxy-pull-action digest to 295b256 by @renovate[bot] in [#306](https://github.com/nicholas-fedor/watchtower/pull/306)

## [1.11.0] - 2025-06-07

### Fixed

- Resolve DOCKER_API_VERSION 404 errors and enhance API handling by @nicholas-fedor in [#305](https://github.com/nicholas-fedor/watchtower/pull/305)

### Chores

- Update module golang.org/x/text to v0.26.0 by @renovate[bot] in [#304](https://github.com/nicholas-fedor/watchtower/pull/304)
- Update golang:alpine docker digest to 68932fa by @renovate[bot] in [#302](https://github.com/nicholas-fedor/watchtower/pull/302)
- Update actions/checkout digest to 09d2aca by @renovate[bot] in [#301](https://github.com/nicholas-fedor/watchtower/pull/301)
- Update dependency python to v3.13.4 by @renovate[bot] in [#300](https://github.com/nicholas-fedor/watchtower/pull/300)
- Update codecov/codecov-action digest to 78f372e by @renovate[bot] in [#299](https://github.com/nicholas-fedor/watchtower/pull/299)
- Update golangci/golangci-lint-action digest to 5286ed6 by @renovate[bot] in [#298](https://github.com/nicholas-fedor/watchtower/pull/298)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.14 by @renovate[bot] in [#297](https://github.com/nicholas-fedor/watchtower/pull/297)
- Update prom/prometheus docker digest to 9abc6cf by @renovate[bot] in [#296](https://github.com/nicholas-fedor/watchtower/pull/296)
- Update golang:alpine docker digest to b4f875e by @renovate[bot] in [#295](https://github.com/nicholas-fedor/watchtower/pull/295)
- Update golang:alpine docker digest to 2853d62 by @renovate[bot] in [#294](https://github.com/nicholas-fedor/watchtower/pull/294)
- Update module github.com/docker/docker to v28.2.2+incompatible by @renovate[bot] in [#291](https://github.com/nicholas-fedor/watchtower/pull/291)
- Update alpine docker tag to v3.22.0 by @renovate[bot] in [#293](https://github.com/nicholas-fedor/watchtower/pull/293)
- Update module github.com/docker/cli to v28.2.2+incompatible by @renovate[bot] in [#290](https://github.com/nicholas-fedor/watchtower/pull/290)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.13 by @renovate[bot] in [#283](https://github.com/nicholas-fedor/watchtower/pull/283)
- Update module github.com/docker/docker to v28.2.1+incompatible by @renovate[bot] in [#287](https://github.com/nicholas-fedor/watchtower/pull/287)
- Update module github.com/docker/cli to v28.2.1+incompatible by @renovate[bot] in [#288](https://github.com/nicholas-fedor/watchtower/pull/288)
- Update module github.com/docker/cli to v28.2.0+incompatible by @renovate[bot] in [#286](https://github.com/nicholas-fedor/watchtower/pull/286)
- Update golangci/golangci-lint-action digest to 481777f by @renovate[bot] in [#285](https://github.com/nicholas-fedor/watchtower/pull/285)

## [1.10.0] - 2025-05-27

### Added

- Add a warning about using Watchtower in production by @simonaronsson
- Add linking and output messages by @piksel
- Add support for "none" scope by @piksel
- Add DisableMemorySwappiness flag for Podman compatibility by @nicholas-fedor in [#264](https://github.com/nicholas-fedor/watchtower/pull/264)

### Fixed

- Correct logging level of watchtower handling for shutdown signals and context cancellation by @nicholas-fedor in [#282](https://github.com/nicholas-fedor/watchtower/pull/282)
- Fix incorrect actions config by @piksel
- Instance cleanup without scope by @piksel
- Set nopull param from args by @piksel
- Handle missing healthcheck keys in config by @piksel
- Use new healthcheck config if not overridden by @piksel
- Correct missing commit history by @nicholas-fedor in [#276](https://github.com/nicholas-fedor/watchtower/pull/276)

### Changed

- Merge containrrr/main to resolve 37 commits behind, keeping local file state by @nicholas-fedor
- Docs for "image" query parameter by @nothub
- Update notifications.md by @simonaronsson
- Document how to skip empty notifications using templates by @simonaronsson

### Documentation

- Add andriibratanin as a contributor for doc by @allcontributors[bot]
- Add nothub as a contributor for code by @allcontributors[bot]
- Add nothub as a contributor for doc by @allcontributors[bot]
- Fix list formatting in container-selection by @andriibratanin

### Tests

- Add unit test for volume subpath preservation by @nicholas-fedor in [#265](https://github.com/nicholas-fedor/watchtower/pull/265)

### Continuous Integration

- Fix manual release by @piksel

### Chores

- Update docker/build-push-action digest to 2634353 by @renovate[bot] in [#280](https://github.com/nicholas-fedor/watchtower/pull/280)
- Bump github.com/prometheus/client_golang from 1.17.0 to 1.18.0 by @dependabot[bot]
- Bump github/codeql-action from 2 to 3 by @dependabot[bot]
- Bump github.com/spf13/viper from 1.18.1 to 1.18.2 by @dependabot[bot]
- Bump github.com/spf13/viper from 1.17.0 to 1.18.1 by @dependabot[bot]
- Bump alpine from 3.18.5 to 3.19.0 in /dockerfiles by @dependabot[bot]
- Bump actions/setup-python from 4 to 5 by @dependabot[bot]
- Bump alpine from 3.18.4 to 3.18.5 in /dockerfiles by @dependabot[bot]
- Fix json syntax error in .all-contributorsrc by @PeterDaveHello
- Bump go/stdlib to v1.20.x by @piksel
- Bump github.com/spf13/cobra from 1.7.0 to 1.8.0 by @dependabot[bot]
- Bump golang.org/x/text from 0.13.0 to 0.14.0 by @dependabot[bot]
- Bump github.com/docker/cli from 24.0.6+incompatible to 24.0.7+incompatible by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.28.1 to 1.29.0 by @dependabot[bot]
- Bump github.com/docker/docker from 24.0.6+incompatible to 24.0.7+incompatible by @dependabot[bot]
- Bump github.com/prometheus/client_golang by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.28.0 to 1.28.1 by @dependabot[bot]
- Replace usages of ioutil by @donuts-are-good
- Bump golang.org/x/net from 0.16.0 to 0.17.0 by @dependabot[bot]
- Bump golang.org/x/net from 0.15.0 to 0.16.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.27.10 to 1.28.0 by @dependabot[bot]
- Bump github.com/docker/distribution from 2.8.2+incompatible to 2.8.3+incompatible by @dependabot[bot]
- Update golangci/golangci-lint-action digest to 3f6d2b9 by @renovate[bot] in [#277](https://github.com/nicholas-fedor/watchtower/pull/277)
- Update grafana/grafana docker digest to 06dc8d6 by @renovate[bot] in [#274](https://github.com/nicholas-fedor/watchtower/pull/274)
- Update nginx:1.28.0 docker digest to eaa7e36 by @renovate[bot] in [#273](https://github.com/nicholas-fedor/watchtower/pull/273)
- Update nginx docker digest to fb39280 by @renovate[bot] in [#272](https://github.com/nicholas-fedor/watchtower/pull/272)
- Update actions/setup-python digest to 5db1cf9 by @renovate[bot] in [#271](https://github.com/nicholas-fedor/watchtower/pull/271)
- Update grafana/grafana docker digest to 6fc2fee by @renovate[bot] in [#270](https://github.com/nicholas-fedor/watchtower/pull/270)
- Update docker/setup-buildx-action digest to 3f1544e by @renovate[bot] in [#269](https://github.com/nicholas-fedor/watchtower/pull/269)
- Update codecov/codecov-action digest to 15559ed by @renovate[bot] in [#268](https://github.com/nicholas-fedor/watchtower/pull/268)
- Update golangci/golangci-lint-action digest to 58da348 by @renovate[bot] in [#266](https://github.com/nicholas-fedor/watchtower/pull/266)
- Update prom/prometheus docker digest to 78ed1f9 by @renovate[bot] in [#263](https://github.com/nicholas-fedor/watchtower/pull/263)
- Update golangci/golangci-lint-action digest to 2086983 by @renovate[bot] in [#261](https://github.com/nicholas-fedor/watchtower/pull/261)
- Update codecov/codecov-action digest to 18283e0 by @renovate[bot] in [#260](https://github.com/nicholas-fedor/watchtower/pull/260)
- Update docker/build-push-action digest to 0788c44 by @renovate[bot] in [#259](https://github.com/nicholas-fedor/watchtower/pull/259)
- Update codecov/codecov-action digest to b203f00 by @renovate[bot] in [#258](https://github.com/nicholas-fedor/watchtower/pull/258)
- Update golangci/golangci-lint-action digest to 0b0f1dd by @renovate[bot] in [#255](https://github.com/nicholas-fedor/watchtower/pull/255)

### New Contributors

- @PeterDaveHello made their first contribution
- @andriibratanin made their first contribution
- @donuts-are-good made their first contribution

## [1.9.2] - 2025-05-08

### Chores

- Update Go version and dependencies by @nicholas-fedor in [#251](https://github.com/nicholas-fedor/watchtower/pull/251)
- Update actions/setup-go digest to d35c59a by @renovate[bot] in [#250](https://github.com/nicholas-fedor/watchtower/pull/250)
- Update cimg/go docker tag to v1.24.3 by @renovate[bot] in [#249](https://github.com/nicholas-fedor/watchtower/pull/249)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.9 by @renovate[bot] in [#248](https://github.com/nicholas-fedor/watchtower/pull/248)
- Update nicholas-fedor/go-proxy-pull-action digest to ad5d0f8 by @renovate[bot] in [#247](https://github.com/nicholas-fedor/watchtower/pull/247)
- Update golang:alpine docker digest to ef18ee7 by @renovate[bot] in [#246](https://github.com/nicholas-fedor/watchtower/pull/246)
- Update actions/setup-go digest to 29694d7 by @renovate[bot] in [#245](https://github.com/nicholas-fedor/watchtower/pull/245)
- Update module golang.org/x/text to v0.25.0 by @renovate[bot] in [#244](https://github.com/nicholas-fedor/watchtower/pull/244)
- Update actions/setup-go digest to 78535dd by @renovate[bot] in [#243](https://github.com/nicholas-fedor/watchtower/pull/243)
- Update grafana/grafana docker digest to 263cbef by @renovate[bot] in [#242](https://github.com/nicholas-fedor/watchtower/pull/242)
- Update golangci/golangci-lint-action digest to 4d56fa9 by @renovate[bot] in [#241](https://github.com/nicholas-fedor/watchtower/pull/241)
- Update prom/prometheus docker digest to e2b8aa6 by @renovate[bot] in [#240](https://github.com/nicholas-fedor/watchtower/pull/240)
- Update actions/setup-go digest to bb65d88 by @renovate[bot] in [#239](https://github.com/nicholas-fedor/watchtower/pull/239)

## [1.9.1] - 2025-04-29

### Fixed

- Enhance host networking and alias handling for container recreation by @nicholas-fedor in [#237](https://github.com/nicholas-fedor/watchtower/pull/237)
- Correct excess contributors per row by @nicholas-fedor in [#206](https://github.com/nicholas-fedor/watchtower/pull/206)

### Chores

- Update actions/setup-go digest to 7f17e83 by @renovate[bot] in [#238](https://github.com/nicholas-fedor/watchtower/pull/238)
- Update nginx:1.28.0 docker digest to 0ad9e58 by @renovate[bot] in [#236](https://github.com/nicholas-fedor/watchtower/pull/236)
- Update nginx docker digest to c15da6c by @renovate[bot] in [#235](https://github.com/nicholas-fedor/watchtower/pull/235)
- Update actions/attest-build-provenance digest to db473fd by @renovate[bot] in [#234](https://github.com/nicholas-fedor/watchtower/pull/234)
- Update nginx docker tag to v1.28.0 by @renovate[bot] in [#233](https://github.com/nicholas-fedor/watchtower/pull/233)
- Update grafana/grafana docker digest to 52c3e20 by @renovate[bot] in [#231](https://github.com/nicholas-fedor/watchtower/pull/231)
- Update docker/setup-qemu-action digest to 737ba1e by @renovate[bot] in [#230](https://github.com/nicholas-fedor/watchtower/pull/230)
- Update docker/setup-buildx-action digest to e600775 by @renovate[bot] in [#229](https://github.com/nicholas-fedor/watchtower/pull/229)
- Update actions/setup-python digest to a26af69 by @renovate[bot] in [#228](https://github.com/nicholas-fedor/watchtower/pull/228)
- Update docker/login-action digest to 6d4b68b by @renovate[bot] in [#227](https://github.com/nicholas-fedor/watchtower/pull/227)
- Update docker/build-push-action digest to 14487ce by @renovate[bot] in [#226](https://github.com/nicholas-fedor/watchtower/pull/226)
- Update actions/setup-python digest to 30eafe9 by @renovate[bot] in [#225](https://github.com/nicholas-fedor/watchtower/pull/225)
- Update docker/setup-qemu-action digest to 5a7ea16 by @renovate[bot] in [#224](https://github.com/nicholas-fedor/watchtower/pull/224)
- Update docker/login-action digest to abd3abc by @renovate[bot] in [#223](https://github.com/nicholas-fedor/watchtower/pull/223)
- Update docker/build-push-action digest to c566248 by @renovate[bot] in [#222](https://github.com/nicholas-fedor/watchtower/pull/222)
- Update module github.com/docker/docker to v28.1.1+incompatible by @renovate[bot] in [#217](https://github.com/nicholas-fedor/watchtower/pull/217)
- Update module github.com/docker/cli to v28.1.1+incompatible by @renovate[bot] in [#215](https://github.com/nicholas-fedor/watchtower/pull/215)
- Update golangci/golangci-lint-action digest to a3942e2 by @renovate[bot] in [#218](https://github.com/nicholas-fedor/watchtower/pull/218)
- Update docker/build-push-action digest to 67dc78b by @renovate[bot] in [#216](https://github.com/nicholas-fedor/watchtower/pull/216)
- Update actions/setup-python digest to 6ed2c67 by @renovate[bot] in [#214](https://github.com/nicholas-fedor/watchtower/pull/214)
- Update nginx:alpine docker digest to 65645c7 by @renovate[bot] in [#213](https://github.com/nicholas-fedor/watchtower/pull/213)
- Update nginx docker tag to v1.27.5 by @renovate[bot] in [#212](https://github.com/nicholas-fedor/watchtower/pull/212)
- Update nginx:alpine docker digest to 7c88831 by @renovate[bot] in [#211](https://github.com/nicholas-fedor/watchtower/pull/211)
- Update nginx docker digest to 5ed8fcc by @renovate[bot] in [#210](https://github.com/nicholas-fedor/watchtower/pull/210)
- Update prom/prometheus docker digest to 339ce86 by @renovate[bot] in [#209](https://github.com/nicholas-fedor/watchtower/pull/209)
- Update codecov/codecov-action digest to ad3126e by @renovate[bot] in [#208](https://github.com/nicholas-fedor/watchtower/pull/208)
- Update golangci/golangci-lint-action digest to 7ecb048 by @renovate[bot] in [#207](https://github.com/nicholas-fedor/watchtower/pull/207)

## [1.9.0] - 2025-04-14

### Fixed

- Add path globbing by @nicholas-fedor in [#201](https://github.com/nicholas-fedor/watchtower/pull/201)

### Changed

- Optional tag filter by @Foxite in [#205](https://github.com/nicholas-fedor/watchtower/pull/205)

### Chores

- Update actions/setup-python digest to e348410 by @renovate[bot] in [#204](https://github.com/nicholas-fedor/watchtower/pull/204)
- Update module github.com/nicholas-fedor/shoutrrr to v0.8.8 by @renovate[bot] in [#203](https://github.com/nicholas-fedor/watchtower/pull/203)
- Address miscellaneous updates and fixes by @nicholas-fedor in [#202](https://github.com/nicholas-fedor/watchtower/pull/202)

## [1.8.9] - 2025-04-10

### Added

- Enhance default-legacy template with fields and debug logging by @nicholas-fedor in [#200](https://github.com/nicholas-fedor/watchtower/pull/200)

### Chores

- Update docker/build-push-action digest to 88844b9 by @renovate[bot] in [#199](https://github.com/nicholas-fedor/watchtower/pull/199)
- Update dependency python to v3.13.3 by @renovate[bot] in [#197](https://github.com/nicholas-fedor/watchtower/pull/197)
- Update docker/build-push-action digest to 548776e by @renovate[bot] in [#195](https://github.com/nicholas-fedor/watchtower/pull/195)

## [1.8.8] - 2025-04-08

### Added

- Update shoutrrr to v0.8.7 by @nicholas-fedor in [#194](https://github.com/nicholas-fedor/watchtower/pull/194)
- Change staleness check logging to debug by @nicholas-fedor in [#193](https://github.com/nicholas-fedor/watchtower/pull/193)
- Enhance HEAD request compatibility with OCI indexes by @nicholas-fedor in [#185](https://github.com/nicholas-fedor/watchtower/pull/185)
- Standardize comments by @nicholas-fedor in [#175](https://github.com/nicholas-fedor/watchtower/pull/175)
- Standardize logrus logging by @nicholas-fedor in [#171](https://github.com/nicholas-fedor/watchtower/pull/171)

### Fixed

- Remove amd64 from tag by @nicholas-fedor in [#192](https://github.com/nicholas-fedor/watchtower/pull/192)
- Remove additional platforms by @nicholas-fedor in [#191](https://github.com/nicholas-fedor/watchtower/pull/191)
- Remove goos variable by @nicholas-fedor in [#190](https://github.com/nicholas-fedor/watchtower/pull/190)
- Remove redundant test step by @nicholas-fedor in [#189](https://github.com/nicholas-fedor/watchtower/pull/189)
- Fix/release dev by @nicholas-fedor in [#188](https://github.com/nicholas-fedor/watchtower/pull/188)
- Correct misconfigured dev build workflow by @nicholas-fedor in [#187](https://github.com/nicholas-fedor/watchtower/pull/187)
- Add path globbing by @nicholas-fedor in [#172](https://github.com/nicholas-fedor/watchtower/pull/172)
- Fix gh attestations by @nicholas-fedor in [#169](https://github.com/nicholas-fedor/watchtower/pull/169)
- Correct checksum file for build attestation by @nicholas-fedor in [#166](https://github.com/nicholas-fedor/watchtower/pull/166)

### Chores

- Update Go versioning by @nicholas-fedor in [#186](https://github.com/nicholas-fedor/watchtower/pull/186)
- Update module github.com/prometheus/client_golang to v1.22.0 by @renovate[bot] in [#184](https://github.com/nicholas-fedor/watchtower/pull/184)
- Update nginx:1.27.4 docker digest to 09369da by @renovate[bot] in [#183](https://github.com/nicholas-fedor/watchtower/pull/183)
- Update nginx docker digest to 09369da by @renovate[bot] in [#182](https://github.com/nicholas-fedor/watchtower/pull/182)
- Update nginx:1.27.4 docker digest to f921d7b by @renovate[bot] in [#181](https://github.com/nicholas-fedor/watchtower/pull/181)
- Update nginx docker digest to 2df6f48 by @renovate[bot] in [#180](https://github.com/nicholas-fedor/watchtower/pull/180)
- Update golangci/golangci-lint-action digest to c2427fe by @renovate[bot] in [#177](https://github.com/nicholas-fedor/watchtower/pull/177)
- Update module github.com/onsi/ginkgo/v2 to v2.23.4 by @renovate[bot] in [#176](https://github.com/nicholas-fedor/watchtower/pull/176)
- Update module golang.org/x/text to v0.24.0 by @renovate[bot] in [#174](https://github.com/nicholas-fedor/watchtower/pull/174)
- Update codecov/codecov-action digest to f95a404 by @renovate[bot] in [#170](https://github.com/nicholas-fedor/watchtower/pull/170)
- Update golangci/golangci-lint-action digest to 9551b25 by @renovate[bot] in [#168](https://github.com/nicholas-fedor/watchtower/pull/168)

## [1.8.7] - 2025-04-03

### Fixed

- Exclude Aliases and DNSNames on default bridge network by @nicholas-fedor in [#163](https://github.com/nicholas-fedor/watchtower/pull/163)

### Chores

- Update golang:alpine docker digest to 7772cb5 by @renovate[bot] in [#162](https://github.com/nicholas-fedor/watchtower/pull/162)
- Update nicholas-fedor/go-proxy-pull-action digest to a4ce118 by @renovate[bot] in [#164](https://github.com/nicholas-fedor/watchtower/pull/164)
- Update cimg/go docker tag to v1.24.2 by @renovate[bot] in [#165](https://github.com/nicholas-fedor/watchtower/pull/165)
- Update actions/setup-go digest to dca8468 by @renovate[bot] in [#161](https://github.com/nicholas-fedor/watchtower/pull/161)
- Update golangci/golangci-lint-action digest to a5307c8 by @renovate[bot] in [#158](https://github.com/nicholas-fedor/watchtower/pull/158)
- Update release-prod to properly chain workflow by @nicholas-fedor in [#156](https://github.com/nicholas-fedor/watchtower/pull/156)

## [1.8.6] - 2025-03-31

### Added

- Improve pre-1.44 api support by @nicholas-fedor in [#152](https://github.com/nicholas-fedor/watchtower/pull/152)
- Cleanup github actions by @nicholas-fedor in [#148](https://github.com/nicholas-fedor/watchtower/pull/148)
- Add debug logging by @nicholas-fedor in [#141](https://github.com/nicholas-fedor/watchtower/pull/141)
- Enhance network preservation and lifecycle management by @nicholas-fedor in [#128](https://github.com/nicholas-fedor/watchtower/pull/128)

### Fixed

- Update test to reflect updated shoutrrr teams handling by @nicholas-fedor in [#155](https://github.com/nicholas-fedor/watchtower/pull/155)
- Update permissions by @nicholas-fedor in [#149](https://github.com/nicholas-fedor/watchtower/pull/149)
- Enhance RunHTTPServer shutdown handling by @nicholas-fedor in [#137](https://github.com/nicholas-fedor/watchtower/pull/137)
- Preserve static MAC address in StartContainer with test coverage by @nicholas-fedor in [#123](https://github.com/nicholas-fedor/watchtower/pull/123)

### Changed

- Improve documentation regarding Docker API version by @nicholas-fedor in [#143](https://github.com/nicholas-fedor/watchtower/pull/143)
- Revert "fix(container): preserve static MAC address in StartContainer with te…" by @nicholas-fedor in [#124](https://github.com/nicholas-fedor/watchtower/pull/124)

### Documentation

- Add api version warning and k8s note by @nicholas-fedor in [#153](https://github.com/nicholas-fedor/watchtower/pull/153)

### Chores

- Update go deps by @nicholas-fedor in [#154](https://github.com/nicholas-fedor/watchtower/pull/154)
- Update goreleaser/goreleaser-action digest to 90c43f2 by @renovate[bot] in [#151](https://github.com/nicholas-fedor/watchtower/pull/151)
- Update config to v2 by @nicholas-fedor in [#150](https://github.com/nicholas-fedor/watchtower/pull/150)
- Update goreleaser/goreleaser-action digest to 9c156ee by @renovate[bot] in [#147](https://github.com/nicholas-fedor/watchtower/pull/147)
- Update golangci/golangci-lint-action digest to 2968cc1 by @renovate[bot] in [#134](https://github.com/nicholas-fedor/watchtower/pull/134)
- Update docker/setup-buildx-action digest to 941183f by @renovate[bot] in [#146](https://github.com/nicholas-fedor/watchtower/pull/146)
- Update module github.com/spf13/viper to v1.20.1 by @renovate[bot] in [#144](https://github.com/nicholas-fedor/watchtower/pull/144)
- Update module github.com/docker/docker to v28.0.4+incompatible by @renovate[bot] in [#140](https://github.com/nicholas-fedor/watchtower/pull/140)
- Update grafana/grafana docker digest to 62d2b9d by @renovate[bot] in [#142](https://github.com/nicholas-fedor/watchtower/pull/142)
- Update module github.com/docker/cli to v28.0.4+incompatible by @renovate[bot] in [#138](https://github.com/nicholas-fedor/watchtower/pull/138)
- Update module github.com/docker/docker to v28.0.3+incompatible by @renovate[bot] in [#139](https://github.com/nicholas-fedor/watchtower/pull/139)
- Update codecov/codecov-action digest to ea99328 by @renovate[bot] in [#136](https://github.com/nicholas-fedor/watchtower/pull/136)
- Update actions/setup-python digest to 8d9ed9a by @renovate[bot] in [#133](https://github.com/nicholas-fedor/watchtower/pull/133)
- Update golangci/golangci-lint-action digest to 1f07148 by @renovate[bot] in [#132](https://github.com/nicholas-fedor/watchtower/pull/132)
- Update module github.com/onsi/gomega to v1.36.3 by @renovate[bot] in [#130](https://github.com/nicholas-fedor/watchtower/pull/130)
- Update module github.com/onsi/ginkgo/v2 to v2.23.3 by @renovate[bot] in [#129](https://github.com/nicholas-fedor/watchtower/pull/129)
- Update module github.com/onsi/ginkgo/v2 to v2.23.2 by @renovate[bot] in [#127](https://github.com/nicholas-fedor/watchtower/pull/127)
- Update module github.com/docker/docker to v28.0.2+incompatible by @renovate[bot] in [#121](https://github.com/nicholas-fedor/watchtower/pull/121)
- Update module github.com/onsi/ginkgo/v2 to v2.23.1 by @renovate[bot] in [#122](https://github.com/nicholas-fedor/watchtower/pull/122)
- Update module github.com/docker/cli to v28.0.2+incompatible by @renovate[bot] in [#120](https://github.com/nicholas-fedor/watchtower/pull/120)
- Update golangci/golangci-lint-action digest to 9938e10 by @renovate[bot] in [#119](https://github.com/nicholas-fedor/watchtower/pull/119)

## [1.8.5] - 2025-03-19

### Chores

- Update module github.com/nicholas-fedor/shoutrrr to v0.8.5 by @renovate[bot] in [#118](https://github.com/nicholas-fedor/watchtower/pull/118)
- Update golangci/golangci-lint-action digest to b91d580 by @renovate[bot] in [#117](https://github.com/nicholas-fedor/watchtower/pull/117)
- Update nginx:1.27.4 docker digest to 124b44b by @renovate[bot] in [#116](https://github.com/nicholas-fedor/watchtower/pull/116)
- Update nginx docker digest to 124b44b by @renovate[bot] in [#115](https://github.com/nicholas-fedor/watchtower/pull/115)
- Update prom/prometheus docker digest to 502ad90 by @renovate[bot] in [#114](https://github.com/nicholas-fedor/watchtower/pull/114)
- Update actions/setup-go digest to 0aaccfd by @renovate[bot] in [#113](https://github.com/nicholas-fedor/watchtower/pull/113)
- Update nginx:1.27.4 docker digest to 57a5631 by @renovate[bot] in [#112](https://github.com/nicholas-fedor/watchtower/pull/112)
- Update nginx docker digest to 57a5631 by @renovate[bot] in [#111](https://github.com/nicholas-fedor/watchtower/pull/111)
- Update nginx:1.27.4 docker digest to 706959c by @renovate[bot] in [#110](https://github.com/nicholas-fedor/watchtower/pull/110)
- Update golangci/golangci-lint-action digest to 55c2c14 by @renovate[bot] in [#108](https://github.com/nicholas-fedor/watchtower/pull/108)
- Update nginx docker digest to 706959c by @renovate[bot] in [#109](https://github.com/nicholas-fedor/watchtower/pull/109)
- Update golangci/golangci-lint-action digest to eb5c0cc by @renovate[bot]
- Update module github.com/spf13/viper to v1.20.0 by @renovate[bot]
- Update docker/login-action digest to 74a5d14 by @renovate[bot]

## [1.8.4] - 2025-03-14

### Fixed

- Correct helper name by @nicholas-fedor
- Add support for updating github action digests by @nicholas-fedor
- Update github actions custom manager and automerge by @nicholas-fedor

### Chores

- Update package dependencies by @nicholas-fedor
- Update nicholas-fedor/go-proxy-pull-action digest to 96d97dd by @renovate[bot]
- Update actions/setup-python digest to 19e4675 by @renovate[bot]
- Update docker/setup-buildx-action digest to afeb29a by @renovate[bot]
- Update actions/setup-python digest to 6fd11e1 by @renovate[bot]
- Update golangci/golangci-lint-action digest to 4696ba8 by @renovate[bot]
- Pin dependencies by @renovate[bot]
- Update config to use config:best-practices by @nicholas-fedor
- Revert to default renovate configuration by @nicholas-fedor
- Update digest codecov/codecov-action digest to 3440e5e by @renovate[bot]
- Update digest actions/setup-go digest to c4c1141 by @renovate[bot]
- Remove schedule by @nicholas-fedor
- Update digest codecov/codecov-action digest to cd4e7cf by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to b871b4f by @renovate[bot]
- Update digest docker/build-push-action digest to 84ad562 by @renovate[bot]
- Delete .github/workflows/greetings.yml by @nicholas-fedor
- Update digest image cimg/go to v1.24.1 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to d8648ac by @renovate[bot]
- Update digest module golang.org/x/net to v0.37.0 by @renovate[bot]
- Update digest module github.com/onsi/ginkgo/v2 to v2.23.0 by @renovate[bot]
- Update digest module golang.org/x/text to v0.23.0 by @renovate[bot]
- Update digest go to 1.24.1 by @renovate[bot]
- Update digest module golang.org/x/net to v0.36.0 by @renovate[bot]
- Update digest module github.com/prometheus/client_golang to v1.21.1 by @renovate[bot]
- Update digest actions/setup-python digest to 9e62be8 by @renovate[bot]
- Bump golangci/golangci-lint-action by @dependabot[bot]
- Update digest module github.com/nicholas-fedor/shoutrrr to v0.8.3 by @renovate[bot]
- Update digest codecov/codecov-action digest to 0565863 by @renovate[bot]

## [1.8.3] - 2025-02-26

### Added

- Add watchtower-sq180.png by @nicholas-fedor

### Removed

- Remove references to containrrr for nicholas-fedor by @nicholas-fedor

### Chores

- Update digest module github.com/docker/cli to v28.0.1+incompatible by @renovate[bot]
- Update digest module github.com/docker/docker to v28.0.1+incompatible by @renovate[bot]
- Update digest docker/setup-buildx-action digest to b5ca514 by @renovate[bot]
- Update digest docker/build-push-action digest to 471d1dc by @renovate[bot]
- Update digest codecov/codecov-action digest to 2488e99 by @renovate[bot]
- Bump golangci/golangci-lint-action by @dependabot[bot]
- Update digest golangci/golangci-lint-action digest to 456fc0f by @renovate[bot]
- Update digest codecov/codecov-action digest to 1fecca8 by @renovate[bot]

## [1.8.2] - 2025-02-20

### Changed

- Update deprecated method call by @nicholas-fedor
- Refactor by @nicholas-fedor
- Match FROM case by @nicholas-fedor
- Revert to built-in cache defaults by @nicholas-fedor
- Centralize Go version and clean up GitHub Actions workflows by @nicholas-fedor
- Update GoReleaser config by @nicholas-fedor
- Version updates by @nicholas-fedor

### Chores

- Update digest module github.com/docker/docker to v28.0.0+incompatible by @renovate[bot]
- Update digest module github.com/docker/cli to v28.0.0+incompatible by @renovate[bot]
- Update digest module github.com/prometheus/client_golang to v1.21.0 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 378320c by @renovate[bot]
- Update digest docker/build-push-action digest to b16f42f by @renovate[bot]
- Update digest codecov/codecov-action digest to 2e6e9c5 by @renovate[bot]
- Update digest actions/setup-python digest to 6ca8e85 by @renovate[bot]
- Bump golangci/golangci-lint-action by @dependabot[bot]
- Update digest module github.com/spf13/cobra to v1.9.1 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 3b4f037 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to fe19838 by @renovate[bot]
- Update digest nicholas-fedor/go-proxy-pull-action digest to 4678aed by @renovate[bot]

## [1.8.1] - 2025-02-15

### Added

- Add end of file newline by @nicholas-fedor
- Add Code of Conduct by @nicholas-fedor
- Add Nicholas Fedor to contributors by @nicholas-fedor

### Fixed

- Fix spelling by @nicholas-fedor
- Fix indentation by @nicholas-fedor
- Fix indentation by @nicholas-fedor
- Fix bare URL by @nicholas-fedor

### Changed

- Replace dot imports with explicit package references by @nicholas-fedor
- Replace dot imports with explicit package references by @nicholas-fedor
- Replace dot imports with explicit package references by @nicholas-fedor
- Update Go Lint version by @nicholas-fedor
- Correct indentation by @nicholas-fedor
- Reorganize imports by @nicholas-fedor
- Convert single quotations to double quotations by @nicholas-fedor
- Correct indentation by @nicholas-fedor
- Convert single quotations to double quotations by @nicholas-fedor
- Replace dot imports with explicit package references by @nicholas-fedor
- Update README.md by @nicholas-fedor
- Update Go version reference by @nicholas-fedor
- Update references from containrrr to nicholas-fedor by @nicholas-fedor
- Update workflow to use Docker's official actions and digests by @nicholas-fedor

### Removed

- Remove version specification by @nicholas-fedor

### Chores

- Update digest module github.com/spf13/cobra to v1.9.0 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 2226d7c by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 0e58f8e by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 6a3fb76 by @renovate[bot]
- Update digest image alpine to v3.21.3 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 818ec4d by @renovate[bot]
- Update digest nicholas-fedor/go-proxy-pull-action digest to 2c0cd90 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to e0ebdd2 by @renovate[bot]
- Update digest image cimg/go to v1.24.0 by @renovate[bot]
- Update digest goreleaser/goreleaser-action digest to 90a3faa by @renovate[bot]
- Update digest codecov/codecov-action digest to 4898080 by @renovate[bot]
- Bump golangci/golangci-lint-action by @dependabot[bot]
- Bump golang.org/x/net from 0.34.0 to 0.35.0 by @dependabot[bot]
- Update digest golangci/golangci-lint-action digest to db1c463 by @renovate[bot]

## [1.8.0] - 2025-02-08

### Changed

- Re-enable test by @nicholas-fedor
- Update minimum supported Docker API version by @nicholas-fedor
- Correct referenced Docker Hub by @nicholas-fedor

## [1.7.13] - 2025-02-08

### Added

- Add CircleCI badge by @nicholas-fedor
- Add .circleci/config.yml by @nicholas-fedor
- Add OCI image labels by @nicholas-fedor
- Add permissions by @nicholas-fedor

### Changed

- Update workflow name by @nicholas-fedor
- Update referenced link by @nicholas-fedor
- Update referenced links by @nicholas-fedor
- Update referenced links by @nicholas-fedor
- Update referenced links by @nicholas-fedor
- Update SECURITY.md by @nicholas-fedor
- Update SHA's to latest by @nicholas-fedor
- Update docker/login-action SHA to latest by @nicholas-fedor
- Update release-dev.yaml by @nicholas-fedor
- Update publish-docs.yml by @nicholas-fedor
- Update greetings.yml by @nicholas-fedor
- Pin tag for codecov/codecov-action@v5 by @nicholas-fedor
- Pin tag for golangci/golangci-lint-action@v6 by @nicholas-fedor
- Pin tag for codecov/codecov-action@v5 by @nicholas-fedor
- Pin tag for golangci/golangci-lint-action@v6 by @nicholas-fedor
- Pin tag for hmarr/auto-approve-action@v4 by @nicholas-fedor

### Removed

- Delete codeql-analysis.yml by @nicholas-fedor

### Chores

- Update digest image nginx to v1.27.4 by @renovate[bot]
- Update digest module golang.org/x/text to v0.22.0 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 1cc4e00 by @renovate[bot]
- Update digest codecov/codecov-action digest to 5efa07b by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 5421a11 by @renovate[bot]
- Update digest codecov/codecov-action digest to 04351de by @renovate[bot]
- Bump codecov/codecov-action by @dependabot[bot]
- Update digest hmarr/auto-approve-action digest to 8f92909 by @renovate[bot]
- Update digest golangci/golangci-lint-action digest to 9665fb5 by @renovate[bot]
- Update digest codecov/codecov-action digest to 2d2cd3c by @renovate[bot]

## [1.7.12] - 2025-02-01

### Added

- Add digest handling by @nicholas-fedor

### Changed

- Correct package dependencies by @nicholas-fedor
- Migrate to Ginkgo v2 by @nicholas-fedor
- Update release.yml by @nicholas-fedor

### Chores

- Update module github.com/onsi/ginkgo to v2.22.2 by @renovate[bot]

## [1.7.11] - 2025-02-01

### Chores

- Update module github.com/spf13/pflag to v1.0.6 by @renovate[bot]
- Update module github.com/docker/cli to v27.5.1+incompatible by @renovate[bot]
- Update module github.com/docker/docker to v27.5.1+incompatible by @renovate[bot]

## [1.7.10] - 2025-01-19

### Added

- Add GH action by @nicholas-fedor
- Add version retractions by @nicholas-fedor

### Changed

- Modify to indirect by @nicholas-fedor
- Dependency updates by @nicholas-fedor
- Update clean-cache.yml by @nicholas-fedor
- Create clean-cache.yml by @nicholas-fedor
- Update linter by @nicholas-fedor
- Update deprecated flag by @nicholas-fedor
- Enable CodeCov by @nicholas-fedor
- Modify version by @nicholas-fedor
- Update deprecated option by @nicholas-fedor
- Update broken linter by @nicholas-fedor

### Removed

- Remove deprecated option by @nicholas-fedor

### Chores

- Update module github.com/onsi/ginkgo to v2.22.2 by @renovate[bot]
- Update module github.com/onsi/ginkgo to v2.22.2 by @renovate[bot]
- Update module github.com/onsi/ginkgo to v2.22.2 by @renovate[bot]
- Update module github.com/onsi/ginkgo to v2.22.2 by @renovate[bot]

## [1.7.2] - 2025-01-18

### Added

- Add a flag/env to explicitly exclude containers by name by @rdamazio
- Add template preview by @piksel
- Add --health-check command line switch by @bugficks
- Allow logging output to use JSON formatter by @GridexX
- Add a label take precedence argument by @jebabin
- Update shoutrrr to v0.8 by @piksel
- Enabled loading http-api-token from file by @piksel
- Support container network mode by @schizo99
- Log removed/untagged images by @piksel
- Add no-pull label for containers by @gilbsgilbs
- Update shoutrrr to v0.7 by @piksel
- Add json template by @piksel
- Add oci image index support by @piksel
- Ignore removal error due to non-existing containers by @nothub
- Allow log level to be set to any level by @matthewmcneely
- Add porcelain output by @piksel
- Regex container name filtering by @mateuszdrab
- Support secrets for notification_url by @jlaska
- Update shoutrrr to v0.6.1 by @piksel
- Update shoutrrr to v0.6.1 by @piksel
- Optional query parameter to update only containers of a specified image by @Foxite
- Add general notification delay by @lazou
- Add additional information for metrics.md by @jamesmacwhite
- Add title field to template data by @piksel
- Support delayed sending by @piksel
- Improve HTTP API logging, honor no-startup-message by @jinnatar
- Improve session result logging by @piksel
- Use a more specific error type for no container info by @MorrisLaw
- Add context fields to lifecycle events by @piksel
- Update to v0.5 by @piksel
- Allow hostname override for notifiers by @nightah
- Check container config before update by @piksel
- Update shoutrrr to v0.4.4 by @piksel
- Make head pull failure warning toggleable by @piksel
- Add ghcr by @ksurl
- Use short image/container IDs in logs by @piksel
- Add version info to goreleasers ldflags by @simonaronsson
- Add dockerhub login step by @simonaronsson
- Add gh token to goreleaser by @simonaronsson
- Include additional info in startup by @piksel
- Add macos to the testing matrix by @simonaronsson
- Add names to steps by @simonaronsson
- Add WATCHTOWER_INCLUDE_RESTARTING env for include-restarting flag by @ilike2burnthing
- Add details/summary to issue template by @piksel
- Added a link to HTTP API documentation by @jeroenrnl
- Swap viper and cobra for config by @piksel
- Add defered closer calls for the http clients by @simonaronsson
- Add http head based digest comparison to avoid dockerhub rate limits by @simonaronsson
- Adds scopeUID config to enable multiple instances of Watchtower by @victorcmoura
- Add string functions for lowercase, uppercase and capitalize to shoutrrr templates by @PssbleTrngle
- Add files via upload by @simonaronsson
- Add files via upload by @simonaronsson
- Add files via upload by @simonaronsson
- Adds the option to skip TLS verification for a Gotify instance by @tammert
- Add template support for shoutrrr notifications by @arnested
- Added --trace flag and new log.Trace() lines for sensitive information by @tammert
- Add blacklist behavior description by @Miosame
- Added documentation using an SMTP relay to #508 by @ogmueller
- Add logo to repo by @simonaronsson
- Add automatic issue locking by @simonaronsson
- Add ability to overrider depending containers with special label by @Saicheg
- Add shoutrrr.go by @mbrandau
- Add shoutrrr by @mbrandau
- Add additional note on gcloud credentials
- Add timeout override for pre-update lifecycle hook by @simonaronsson
- Add --no-startup-message flag
- Add "help needed" section to readme by @simonaronsson
- Add github action to build and publish dev image to dockerhub by @zoispag
- Add support for multiple email recipients by @simonaronsson
- Adds development self-contained builds instructions to CONTRIBUTING.md by @victorcmoura
- Adds self-contained dev Dockerfile by @victorcmoura
- Added full path to config file by @jnidzwetzki
- Added Mail Subject Tag to email.go by @simonaronsson
- Add --revive-stopped flag to start stopped containers after an update by @zoispag
- Add lifecycle hooks to documentation nav by @zoispag
- Add pre/post update check lifecycle hooks
- Add docker api version parameter by @kaloyan-raev
- Add new documentation to menu by @simonaronsson
- Add information on how to use credential helpers by @simonaronsson
- Add docker pull count badge by @simonaronsson
- Add support for Gotify notifications by @lukapeschke
- Add githubs fingerprint to known host for mkdocs by @simonaronsson
- Add sponsorship option by @simonaronsson
- Add probot: welcome by @simonaronsson
- Add probot: stale by @simonaronsson
- Add all contributors by @simonaronsson
- Add a Gitter chat badge to README.md by @gitter-badger
- Add tests for check action, resolve wt cleanup bug by @simonaronsson
- Add Dockerfile.self-contained by @svengo
- Add placeholder client test by @simonaronsson
- Add test coverage badge by @simonaronsson
- Add additional login due to manifest shenanigans by @simonaronsson
- Add licensing badge and latest semver badge by @simonaronsson
- Add new flag to readme by @simonaronsson
- Add Slack Channel, IconEmoji, and IconURL options by @mariotacke
- Add a monitor only flag by @simonaronsson
- Add a logo and spice up the top part of the readme by @simonaronsson
- Add reminder to pull proper image for ARM users by @connormcmk
- Add documentation for how to pass env variables by @connormcmk
- Add a "Reviewed by Hound" badge by @salbertson
- Add arm64v8 build by @schnapster
- Add msteams to notifications overview by @stffabi
- Add tests for filters by @stffabi
- Add --stop-timeout parameter by @Robotex
- Add slackrus slack notifications by @ubergesundheit
- Add date header to mail by @stffabi
- Added example for Cron expression by @cron410
- Add a method of enabling or disabling containers using labels by @belak
- Adding basic (but flexible) notification system which hooks into logrus. by @rdamazio
- Added host network check by @Robotex
- Added arm release builds. by @stffabi
- Added glide for vendoring dependencies. by @stffabi
- Support loading authentication credentials from Docker config file
- Add auth config, registry auth fails silently without
- Support Zodiac-based deployments by @bdehamer
- Add support for whitelist of monitored containers by @bdehamer
- Add godoc comments by @bdehamer
- Support for --cleanup flag by @bdehamer
- Add README content by @bdehamer
- Support TLS connections to remote daemons by @bdehamer
- Add more accessors to Container struct by @bdehamer
- Support --debug flag by @bdehamer
- Add --no-pull support by @bdehamer

### Fixed

- Only remove container id network aliases by @piksel
- Received typo by @testwill
- Correctly set the delay from options by @Tentoe
- Empty out the aliases on recreation by @simonaronsson
- Always use container interface by @piksel
- Image name parsing behavior by @Pwuts
- Fix anchor links in the metrics part by @louis-genestier
- Remove logging of credentials by @piksel
- Ignore empty challenge fields by @piksel
- Always add missing slashes to link names by @piksel
- Update metrics from sessions started via API by @SamKirsch10
- Use correct modern css color syntax by @piksel
- Explicitly accept non-commands as root args by @piksel
- Detect schedule set from env by @piksel
- Include icon in slack legacy url by @Choromanski
- Fix typo in grafana dashboard.json by @lyh16
- Gracefully skip pinned images by @piksel
- Testing for flag files on windows by @piksel
- Title customization by @piksel
- Correctly handle non-stale restarts by @piksel
- Move invalid token to log field by @piksel
- Add missing portmap when needed by @piksel
- Fix typo on --http-api-update environment variable and add warning note for --http-api-periodic-polls by @jamesmacwhite
- Linked/depends-on container restarting by @piksel
- Fix docker-compose syntax in quick-start. GH #1105 by @atombrella
- Return appropriate status for unauthorized requests by @hypnoglow
- Fixing flags usage text to first capital letter. by @dhiemaz
- Default templates and logic by @piksel
- Check container image info for nil by @piksel
- Fix more spelling mistakes by @piksel
- Fix goreleaser GHCR login by @piksel
- Fix goreleaser tags for ghcr.io by @piksel
- Fix manifest creation in release job by @piksel
- Use default http transport for head by @piksel
- Merge artifacts and broken shoutrrr tests by @piksel
- Fix depends on behavior and simplify some of its logic by @simonaronsson
- Move notify URL to trace log by @piksel
- Don't panic on unconfigured notifier by @piksel
- Fix docker secrets by @simonaronsson
- Fix tag name parsing, hopefully by @simonaronsson
- Fix arguments doc formatting by @nymous
- Disallow log level 'trace' by @zoispag
- Set log level to debug for message about API token by @zoispag
- Set correct get url for linter by @simonaronsson
- Fix notifications and old instance cleanup by @piksel
- Fix doc typo by @simonaronsson
- Fix manifest tag index in manifest.go by @piksel
- Fix linting issues by @simonaronsson
- Fix syntax highlight and typo in docs by @piksel
- Fix cleanup for rolling updates by @piksel
- Fix typo by @rg9400
- Fix default interval to be the intended value by @piksel
- Fix erroneous poll interval change by @simonaronsson
- Fix host flag by @ksurl
- Return nil imageinfo when retrieve fails by @piksel
- Fix fmt and vetting issues by @simonaronsson
- Make shoutrrr init failure a fatal error by @piksel
- Display errors on init failure by @piksel
- Fix typos by @pgrimaud
- Always use configured delay for notifications by @piksel
- Fix linting and formatting by @simonaronsson
- Fix some errors and clean up
- Improve logging by @simonaronsson
- Update mock client for tests by @simonaronsson
- Fix #472 by @mbrandau
- Fix according to remarks
- 🐛 bump alpine version by @codingCoffee
- Fixed small typo by @jnidzwetzki
- Fixed wrong filename in documentation by @jnidzwetzki
- Fix some var ref errors by @simonaronsson
- Fix a small typo by @foosel
- Switch exit code for run once to 0 by @simonaronsson
- Fix exempt labels by @simonaronsson
- Fix typo in arguments docs by @chugunov
- Resolve merge issues by @simonaronsson
- Remove linting issues by @simonaronsson
- Remove unnecessary cronSet check by @simonaronsson
- Fix port typing issue introduced in 998e805 by @simonaronsson
- Fix linter errors by @simonaronsson
- Stop marking milestone issues as stale by @simonaronsson
- Fix format by @simonaronsson
- Fix layout by @simonaronsson
- Fix linting errors by @simonaronsson
- Fix tag splitting by @simonaronsson
- Fix readme by @simonaronsson
- Fix merge conflicts and do some refactoring by @simonaronsson
- Fix linting errors by @simonaronsson
- Fix some minor issues by @simonaronsson
- Fixing name of Docker Hub account by @waja
- Fix confusing word by @connormcmk
- Fixed badges layouts for consistency by @mxschmitt
- Fix the version information output. Additionally output the commit hash and the build date. by @stffabi
- Fix code style.
- Fixing function documentation by @rdamazio
- Fixed package path. by @stffabi
- Fixed typo in LICENSE and renamed to md. by @stffabi
- Fix env name by @haswalt
- Fix comment from HoundCI by @dolanor
- Fix issue where updated containers aren't stopped by @bdehamer
- Fix aggressive image pulling by @bdehamer

### Changed

- Dependency update by @nicholas-fedor
- Update notifications.md by @DMJoh
- Set default email client host by @piksel
- Preparations for soft deprecation of legacy notification args by @piksel
- [StepSecurity] ci: Harden GitHub Actions by @step-security-bot
- Feat : added new issue templates by @tamalCodes
- Create dependabot.yml to update versions for GitHub Actions, Go modules and Docker images by @jauderho
- Expand docker.config section by @EDIflyer
- Bump shoutrrr to v0.5.3 by @piksel
- Update greetings.yml by @simonaronsson
- Bump vulnerable packages by @simonaronsson
- Bump version of vulnerable dependencies by @piksel
- Post update time out by @patricegautier
- Prefer long flags in quick start example by @rootulp
- Create pull_request_template.md by @simonaronsson
- Update README.md by @modem7
- Update dependencies (sane go.mod) by @piksel
- Use golang:1.15 in ECR credential helper example by @jerbob
- Session report collection and report templates by @piksel
- Pre-update lifecycle hook
- * feat: custom user agent by @piksel
- Update index.md by @zenjabba
- Allow running periodic updates with enabled HTTP API by @DasSkelett
- Documentation updates by @piksel
- Feat/head failure toggle by @simonaronsson
- Update bug_report.md by @simonaronsson
- Move token logs to trace by @simonaronsson
- Create SECURITY.md by @simonaronsson
- Rem vals we dont need or use from the gr config by @simonaronsson
- Permanently disable cgo for production releases by @simonaronsson
- Update release.yml by @simonaronsson
- Update release-dev.yaml by @simonaronsson
- Doc fix: default interval is 24h instead of 5m by @reinout
- Update Shoutrrr to v0.4 by @piksel
- Set different default branch for mkdocs edit by @zoispag
- Typo in --http-api by @kopiro
- Update HTTP API docs by @dapplion
- Make test command windows compatible by @simonaronsson
- Update README.md by @simonaronsson
- Update README.md by @simonaronsson
- Cleanup readme by @simonaronsson
- Create post-release.yml by @simonaronsson
- Prometheus support by @simonaronsson
- Cherrypick notification changes from #450 by @simonaronsson
- Log based on registry known-support - reduce noise on notifications by @tkalus
- Revert "feat(config): swap viper and cobra for config" by @simonaronsson
- Clean up scope builder and remove fmt print by @simonaronsson
- Make sure all different ref formats are supported by @simonaronsson
- Move secret value "credentials" to trace log by @piksel
- Documentation theme updates by @piksel
- Actually fix it by @simonaronsson
- Allow watchtower to update rebooting containers
- Update README to reflect migration to GitHub discussions by @TheCoolBlackCat
- Update to improve the private registry docs by @chander
- Monitor-only for individual containers by @dhet
- Disabling color through environment variables by @bugficks
- Rolling restart by @osheroff
- Skip updating containers where no local image info can be retrieved by @piksel
- Make sure all shoutrrr notifications are sent by @CedricFinance
- Warning if `WATCHTOWER_NO_PULL` and` WATCHTOWER_MONITOR_ONLY` are used simultaneously. by @m-sedl
- Lifecycle logs as Debug instead of Info by @MichaelSp
- Update private-registries.md by @bugficks
- Create code_of_conduct.md by @simonaronsson
- Update README.md by @simonaronsson
- Make background transparent by @simonaronsson
- Update README.md by @simonaronsson
- Update README.md by @simonaronsson
- Create CODEOWNERS by @simonaronsson
- Allows flags containing sensitive stuff to be passed as files by @tammert
- Image of running container no longer needed locally by @tammert
- Notification docs: Add SMTP port to gmail configuration by @mattdoran
- `config.json` symlink workaround described by @tammert
- Create config.yml by @simonaronsson
- Update shoutrrr to get latest and updated services by @arnested
- Update README.md by @simonaronsson
- Update README.md by @simonaronsson
- Update README.md by @simonaronsson
- Update .all-contributorsrc by @simonaronsson
- Only run greeting on issues for the time being by @simonaronsson
- Disable godacov by @simonaronsson
- Comment out test that is incompatible with CircleCI by @simonaronsson
- Bump minimum API version to 1.25 by @simonaronsson
- Increases stopContainer timeout to 10min by @bopoh24
- Update README.md by @simonaronsson
- Increases stopContainer timeout from 60 seconds to 10min by @victorcmoura
- Watchtower HTTP API based updates by @victorcmoura
- Typo Correction by @patski123
- Update shoutrrr by @mbrandau
- Update documentation by @mbrandau
- Reuse router by @mbrandau
- Adjust documentation by @mbrandau
- Use CreateSender instead of calling Send multiple times by @mbrandau
- Adjust flags by @mbrandau
- Copy note about setting both interval and schedule by @sixcorners
- Feature/367 fix: skip container if pre-update command fails by @simonaronsson
- Update arguments.md by @simonaronsson
- Update arguments.md by @aneisch
- Update container-selection.md by @simonaronsson
- Clarify container selection by @simonaronsson
- Time Zone improvements
- Start up notification
- Return on error after http.Post to gotify instance by @pagdot
- Update README.md by @simonaronsson
- Document the TZ environment variable by @Germs2004
- Unset Hostname when NetworkMode is container by @raymondelooff
- Update .all-contributorsrc by @simonaronsson
- Tidy up mod and sum files by @simonaronsson
- Extract code from the container package by @simonaronsson
- Update publish-dev-dockerimage.yaml by @simonaronsson
- #387 fix: add comments to pass linting by @simonaronsson
- #387 fix: switch to image id map and add additional tests by @simonaronsson
- Edits goreleaser to match exclusive dockerfiles folder by @victorcmoura
- Moved dockerfiles to an exclusive folder by @victorcmoura
- Proper set implementation by @mindrunner
- Do not delete same image twice when cleaning up by @mindrunner
- #413 Change initial logging message from debug to info by @zoispag
- Sync by @zoispag
- Update private-registries.md by @simonaronsson
- Update private-registries.md by @jnidzwetzki
- Documented private registries by @jnidzwetzki
- Renamed documentation file by @jnidzwetzki
- Update credential-helpers.md by @jnidzwetzki
- Changed codeblock language by @jnidzwetzki
- Don’t delete cmd when runtime entrypoint is different by @jsclayton
- Update instruction on multiple notifications by @simonaronsson
- Update notifications.md by @simonaronsson
- Update greetings.yml by @simonaronsson
- Greet new reporters and contributors using actions by @simonaronsson
- Update notifications.md by @simonaronsson
- Update flags.go by @8ear
- Update flags.go by @8ear
- Update email.go by @8ear
- Update email.go by @8ear
- Update email.go by @8ear
- Update email.go by @8ear
- Insert nota bene about docker-compose into notification doc page by @foxbit19
- Update FUNDING.yml by @simonaronsson
- Add optional email delay by @simonaronsson
- Update check.go by @sixth
- Wording clarification on "Filter by enable label" by @rjbudke
- Correcting a few typos and text styling. by @noplanman
- Feat/lifecycle hooks by @simonaronsson
- Split out more code into separate files by @simonaronsson
- Move actions into internal by @simonaronsson
- Move actions into pkg by @simonaronsson
- Move container into pkg by @simonaronsson
- Extract types and pkgs to new files by @simonaronsson
- Re-apply based on new go flags package by @zoispag
- Turn on publish filter again by @simonaronsson
- Change fingerprint by @simonaronsson
- Inject ssh key during publish by @simonaronsson
- Resolve pip issue by @simonaronsson
- Switch urfave to cobra by @simonaronsson
- Also keep the original markdown docs :P~ by @simonaronsson
- Move documentation from readme to gh pages by @simonaronsson
- Update stale.yml by @simonaronsson
- Ansem93 patch 1 by @simonaronsson
- Update emoji-key by @simonaronsson
- Update .all-contributorsrc by @simonaronsson
- Update README.md by @simonaronsson
- Make it possible to use watchtower to update exited or created containers as well by @simonaronsson
- Set theme jekyll-theme-minimal by @simonaronsson
- Set theme jekyll-theme-tactile by @simonaronsson
- Update tls example by @simonaronsson
- Exclude markdown files from coverage analysis by @simonaronsson
- Improve test coverage and add an api-server mock by @simonaronsson
- Improve test coverage by @simonaronsson
- Pin down dockerfile alpine version by @simonaronsson
- Fix mail notification regression by @simonaronsson
- Send coverage to codacy by @simonaronsson
- Update issue templates by @simonaronsson
- Check if schedule len > 0 to prevent collisions by @techknowlogick
- Switch from hound to codacy by @simonaronsson
- Strip v. from tag when creating manifests by @simonaronsson
- Additional release logic to try to push manifested releases on publish by @thelamer
- Utilize goreleaser builtins and reformat code by @simonaronsson
- Additional release logic to try to push manifested releases on publish by @thelamer
- Update CONTRIBUTING.md by @simonaronsson
- Update CONTRIBUTING.md by @simonaronsson
- Reduce to one dockerfile as they have the exact same content by @simonaronsson
- Updated README.md: v2tech → containrrr by @KopfKrieg
- Github/v2tec/watchtower#114 adding if statement to accept the oneshot flag to run once and exit by @thelamer
- Update question.md by @simonaronsson
- Update issue templates by @simonaronsson
- Create CODE_OF_CONDUCT.md by @simonaronsson
- Update README.md by @simonaronsson
- Minor changes to make it work with new goreleaser version by @simonaronsson
- 32bit support
- Update README.md by @simonaronsson
- Print to log if multiple watchtower instances are detected by @simonaronsson
- Setup a working pipeline by @simonaronsson
- Change repo paths by @simonaronsson
- Change push dir for image by @simonaronsson
- Switch refs in readme by @simonaronsson
- Spelling updates for README.md by @huddlesj
- Notifications via MSTeams by @darknode
- Some linting by @stffabi
- Always exclude containers that have the com.centurylinklabs.watchtower.enable set to false. by @stffabi
- Implemented enableLabel by a Filter by @stffabi
- Update to build image with docker 17.05.0-ce and latest goreleaser by @stffabi
- Build containers from scratch and use ca-certificates and zoneinfo from latest alpine. by @stffabi
- Made the notification level flag global for all notification types. by @stffabi
- Glide alias Sirupsen to sirupsen by @stffabi
- Change upper case S in sirupsen to lower case to avoid build error by @ubergesundheit
- Send mails that correspond to RFC2045 with a base64 line limit of 76 characters. by @stffabi
- Only authenticate if user has been set. by @stffabi
- Possibility to disable the TLS verify for sending mails. by @stffabi
- Output error message when a pull failed. by @stffabi
- Update email.go by @fomk
- Update email.go by @fomk
- SMTP port configurable through `notification-email-server-port`. Defaults to 25. by @stffabi
- Do not send an email notification when no messages have been logged. by @stffabi
- Push semver containers during a release build. by @stffabi
- Login to docker to publish docker images. by @stffabi
- Crosscompile for windows. by @stffabi
- - Update to circleci 2.0 by @stffabi
- Updated README.md to document notifications by @rdamazio
- Correct repository owner by @mrw34
- Output "Checking containers for updated images" as debug entry. fixes GH-66 by @stffabi
- Do not initiate a RemoveContainer for containers which have AutoRemove (--rm) active. by @stffabi
- Update README.md by @wmbutler
- Name release tarballs for arm architecture armhf.
- - Use GoBuilder container for building and release tags with goreleaser. by @stffabi
- Copy watchtower binary for ci builds to artifacts. by @stffabi
- Make the algorithm follow docker-compose more precisely by @dolanor
- Set minimum required API Version of docker to 1.24, this basically means we require at least docker 1.12.x or newer, therefore we also support docker 1.13.x. by @stffabi
- Go fmt... by @stffabi
- Possibility to define a cron expression which specifies when to check for updated images. This allows to have a schedule in which updates should be made and therefore one could define a maintenance window. by @stffabi
- RenameContainer implemented, this fixes the problem that watchtower can't update itself. by @stffabi
- Do not publish docker images for the time being. This will be setup differently. by @stffabi
- Renamed centurylink to v2tec. by @stffabi
- Update LICENSE
- CliConfig moved. by @stffabi
- Update README.md by @ATCUSA
- NewNativeStore has to be called with the CredentialsStore from the configfile. See also https://github.com/docker/docker/commit/07c4b4124b46be30ea3ac7d114c44c4f911ca182#diff-b082736d194e2fdfc6aca9d0c86a781bL26
- Skip restarting by @haswalt
- Setup using env vars as well. Add no retsart option by @haswalt
- Go fmt done! by @dolanor
- Make an updated container connects to all the previously connected net by @dolanor
- Reuse the network config for the relaunch by @dolanor
- Deploy to official and unofficial hubs
- Automatically deploy from hub branch to rosscado/watchtower docker hub repo
- Change image name to push to rosscado/watchtower by @rosscado
- Automatically push rosscado/watchtower:auth branch to rosscado/watchtower hub
- Refactor port mapping functions for build simplicity
- When authentication credentials are supplied as env vars they are always used.
- Cannot load host Docker config from container. Remove option and rely on environment variables
- Registry authentication was failing silently when pulling images.
- Build instructions for contributors (because it's not obvious)
- Build instructions for contributors (because it's not obvious)
- Bdehamer/golang-builder doesn't work, use centurylink/golang-builder instead
- Rework TLS support, remove unsupported options
- Godeps doesn't work, go without
- Godep doesn't work, distro required
- Consistent context
- Updating dependencies with
- Migrate from codegangsta lib to urfave
- Ignore build output (watchtower binary)
- Discard obsolete samalba/dockerclient library and dependent tests
- Go fmt
- Port client lib from samalba/dockerclient to docker/docker/client
- Private registry authentication distinct from host
- Improve error reporting
- Improve error handling
- Reinstate MAINTAINER and LABEL, Ubuntu base image required by dockerclient upgrade
- Migrate Godeps/_workspace/ to vendor/
- Readme update
- Parameterize repo auth
- Merge resolution
- Will not compile without these updates due to change in docker lib
- Updates
- Minor README edits by @bdehamer
- Configure hound by @bdehamer
- Turn DockerClient into dockerClient by @bdehamer
- Update MAINTAINER email in Dockerfile by @bdehamer
- Create LICENSE by @bdehamer
- Handle errors without halting by @bdehamer
- Account for latency in container removal by @bdehamer
- Refactor Client interface by @bdehamer
- Go-lint clean-up by @bdehamer
- Allow user-configurable DOCKER_HOST by @bdehamer
- Refactoring & renaming by @bdehamer
- Enable watchtower to update itself by @bdehamer
- Wait for container stop after kill by @bdehamer
- Set-up CircleCI builds by @bdehamer
- Handle container links by @bdehamer
- Initial commit by @bdehamer

### Removed

- Delete FUNDING.yml by @simonaronsson
- Delete unused circleci config by @simonaronsson
- Remove cgo_enable flag for the test step by @simonaronsson
- Remove gitter badge by @simonaronsson
- Delete code of conduct in favor of org-wide one by @simonaronsson
- Delete logo.png by @simonaronsson
- Removed accidental dot by @tammert
- Remove issue template for questions by @simonaronsson
- Remove dockerhub readme sync by @simonaronsson
- Removed all potential debug password prints, both plaintext and encoded by @tammert
- Remove old shoutrrr version by @mbrandau
- Remove welcome bot config by @simonaronsson
- Remove dead code and goverage dependency by @simonaronsson
- Delete _config.yml by @simonaronsson
- Delete .hound.yml by @simonaronsson
- Remove (another) erroneuos i in goreleaser by @simonaronsson
- Remove erroneuos i in goreleaser by @simonaronsson
- Removed unused mock code. by @stffabi
- Removed hub_mirror deployment, came in with PR #26.

### Documentation

- Clarify what volumes are removed when requested by @szaimen
- Update interval text in introduction by @valankar
- Add testwill as a contributor for doc by @allcontributors[bot]
- Bump go version in credential helper example by @aioue
- Fix env variable name `notification(s)-delay` by @kirbylink
- Add "HTTP API Mode" link to nav menu by @SergeAx
- Add containrrr-dark color scheme by @piksel
- Add dark mode by @carueda
- Add IAmTamal as a contributor for doc by @allcontributors[bot]
- Add jauderho as a contributor for code by @allcontributors[bot]
- Add EDIflyer as a contributor for doc by @allcontributors[bot]
- Change example region to a replace-me token by @frinzekt
- Clarify container label usage by @EDIflyer
- Add Foxite as a contributor for code by @allcontributors[bot]
- Add ksurl as a contributor for infra by @allcontributors[bot]
- Add jamesmacwhite as a contributor for doc by @allcontributors[bot]
- Add note about docker hub private images by @piksel
- Fix redirect link by @jiangtj
- Add patricegautier as a contributor for code by @allcontributors[bot]
- Add hypnoglow as a contributor for code by @allcontributors[bot]
- Add modem7 as a contributor for doc by @allcontributors[bot]
- Add djquan as a contributor for doc by @allcontributors[bot]
- Remove broken badge and fix docker-compose snippet by @djquan
- Link to versioned shoutrrr docs by @piksel
- Fix note paragraph on Arguments page by @noplanman
- Fix markdown lint issues by @piksel
- Add zenjabba as a contributor by @allcontributors[bot]
- Remove stray paragraph in notifications by @piksel
- Add DasSkelett as a contributor by @allcontributors[bot]
- Fix more auto-format casualties by @piksel
- Add ksurl as a contributor by @allcontributors[bot]
- Suggest mounting localtime, not of timezone by @piksel
- Remove the explicit file name from edit url by @piksel
- Add reinout as a contributor by @allcontributors[bot]
- Add hydrargyrum as a contributor by @allcontributors[bot]
- Fix broken markup of "HTTP API Token" by @hydrargyrum
- Update changed contributor username by @jokay
- Add nymous as a contributor by @allcontributors[bot]
- Add SrihariThalla as a contributor by @allcontributors[bot]
- Add zoispag as a contributor by @allcontributors[bot]
- Add SrihariThalla as a contributor by @allcontributors[bot]
- Add tkalus as a contributor by @allcontributors[bot]
- Add rg9400 as a contributor by @allcontributors[bot]
- Add ksurl as a contributor by @allcontributors[bot]
- Add yrien30 as a contributor by @allcontributors[bot]
- Add chander as a contributor by @allcontributors[bot]
- Add dhet as a contributor by @allcontributors[bot]
- Add bugficks as a contributor by @allcontributors[bot]
- Add osheroff as a contributor by @allcontributors[bot]
- Add x-jokay as a contributor by @allcontributors[bot]
- Add MihailITPlace as a contributor by @allcontributors[bot]
- Add MichaelSp as a contributor by @allcontributors[bot]
- Fix environment variable name typo
- Document DOCKER_CONFIG environment variable by @piksel
- Add bugficks as a contributor by @allcontributors[bot]
- Add MihailITPlace as a contributor by @allcontributors[bot]
- Add mattdoran as a contributor by @allcontributors[bot]
- Add pgrimaud as a contributor by @allcontributors[bot]
- Add andrewjmetzger as a contributor by @allcontributors[bot]
- Add miosame as a contributor by @allcontributors[bot]
- Add tammert as a contributor by @allcontributors[bot]
- Add ogmueller as a contributor by @allcontributors[bot]
- Add bopoh24 as a contributor by @allcontributors[bot]
- Add Saicheg as a contributor by @allcontributors[bot]
- Add victorcmoura as a contributor by @allcontributors[bot]
- Add patski123 as a contributor by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update cron docs link by @piksel
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Add auanasgheps as a contributor by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc [skip ci] by @allcontributors[bot]
- Update README.md [skip ci] by @allcontributors[bot]
- Update .all-contributorsrc by @allcontributors[bot]
- Update README.md by @allcontributors[bot]
- Add missing arguments by @pjknkda
- Update .all-contributorsrc by @allcontributors[bot]
- Update README.md by @allcontributors[bot]
- Add foosel as a contributor by @allcontributors[bot]
- Add sixth as a contributor by @allcontributors[bot]
- Add kaloyan-raev as a contributor by @allcontributors[bot]
- Add rjbudke as a contributor by @allcontributors[bot]
- Add noplanman as a contributor by @allcontributors[bot]
- Add chugunov as a contributor by @allcontributors[bot]
- Add alexandremenif as a contributor by @allcontributors[bot]
- Add lukapeschke as a contributor by @allcontributors[bot]
- Update .all-contributorsrc by @allcontributors[bot]
- Update README.md by @allcontributors[bot]
- Add Ansem93 as a contributor by @allcontributors[bot]
- Add bdehamer as a contributor by @allcontributors[bot]
- Add rosscado as a contributor by @allcontributors[bot]
- Add stffabi as a contributor by @allcontributors[bot]
- Add Codelica as a contributor by @allcontributors[bot]
- Add kopfkrieg as a contributor by @allcontributors[bot]

### Tests

- Check flag/docs consistency by @piksel
- Ensure temp files are cleaned up by @piksel
- Refactor/simplify container mock builders by @piksel
- Fully reset ghttp server by @piksel
- Container client tests refactor by @piksel
- Reduce test output noise by @piksel
- Remove unused cross package dependency on mock api server by @piksel
- Refactor client tests by @piksel
- Fix metrics api test stability by @piksel

### Continuous Integration

- Add dependabot auto approve by @piksel
- Replace golint with staticcheck by @piksel
- Use pull_request_target for greeting by @piksel
- Fix docs generation by @piksel
- Add pip caching for docs workflow by @ksurl
- Fix version in non-dev dockerfile by @piksel
- Fix version in dev dockerfile by @piksel
- Add executable bit to build by @piksel
- Build latest-dev with script by @piksel
- Move docs to separate action by @piksel
- Update code of conduct URL in github action by @zoispag
- Add codeQL analysis checks by @zoispag

### Chores

- Consolidated all post-fork updates including dependency bumps and workflow changes by @dependabot[bot]
- Bump alpine from 3.18.3 to 3.18.4 in /dockerfiles by @dependabot[bot]
- Update Dockerfile.dev-self-contained to allow better build cache by @jebabin
- Bump github.com/docker/docker from 24.0.5+incompatible to 24.0.6+incompatible by @dependabot[bot]
- Bump github.com/docker/cli from 24.0.5+incompatible to 24.0.6+incompatible by @dependabot[bot]
- Bump golang.org/x/net from 0.14.0 to 0.15.0 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 4.4.0 to 5.0.0 by @dependabot[bot]
- Bump actions/checkout from 3 to 4 by @dependabot[bot]
- Bump golang.org/x/text from 0.12.0 to 0.13.0 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 4.3.0 to 4.4.0 by @dependabot[bot]
- Bump alpine from 3.18.2 to 3.18.3 in /dockerfiles by @dependabot[bot]
- Bump golang.org/x/net from 0.12.0 to 0.14.0 by @dependabot[bot]
- Bump golang.org/x/text from 0.11.0 to 0.12.0 by @dependabot[bot]
- Bump github.com/docker/cli by @dependabot[bot]
- Bump github.com/docker/docker by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.27.8 to 1.27.10 by @dependabot[bot]
- Bump github.com/docker/cli from 24.0.2+incompatible to 24.0.4+incompatible by @dependabot[bot]
- Bump golang.org/x/net from 0.11.0 to 0.12.0 by @dependabot[bot]
- Bump github.com/docker/docker from 24.0.2+incompatible to 24.0.4+incompatible by @dependabot[bot]
- Bump golang.org/x/net from 0.10.0 to 0.11.0 by @dependabot[bot]
- Bump github.com/prometheus/client_golang from 1.15.1 to 1.16.0 by @dependabot[bot]
- Bump alpine from 3.18.0 to 3.18.2 in /dockerfiles by @dependabot[bot]
- Bump github.com/spf13/viper from 1.15.0 to 1.16.0 by @dependabot[bot]
- Bump github.com/sirupsen/logrus from 1.9.2 to 1.9.3 by @dependabot[bot]
- Bump docker/login-action from 2.1.0 to 2.2.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.27.7 to 1.27.8 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 4.2.0 to 4.3.0 by @dependabot[bot]
- Bump golang.org/x/text from 0.9.0 to 0.10.0 by @dependabot[bot]
- Bump github.com/stretchr/testify from 1.8.3 to 1.8.4 by @dependabot[bot]
- Bump github.com/docker/docker from 23.0.6+incompatible to 24.0.2+incompatible by @dependabot[bot]
- Bump github.com/docker/cli from 24.0.1+incompatible to 24.0.2+incompatible by @dependabot[bot]
- Bump github.com/docker/cli by @dependabot[bot]
- Bump github.com/stretchr/testify from 1.8.2 to 1.8.3 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.27.6 to 1.27.7 by @dependabot[bot]
- Bump github.com/sirupsen/logrus from 1.9.0 to 1.9.2 by @dependabot[bot]
- Bump alpine from 3.17.3 to 3.18.0 in /dockerfiles by @dependabot[bot]
- Bump github.com/docker/distribution from 2.8.1+incompatible to 2.8.2+incompatible by @dependabot[bot]
- Bump github.com/prometheus/client_golang by @dependabot[bot]
- Bump github.com/docker/cli by @dependabot[bot]
- Bump golang.org/x/net from 0.9.0 to 0.10.0 by @dependabot[bot]
- Bump github.com/docker/docker by @dependabot[bot]
- Bump github.com/docker/cli by @dependabot[bot]
- Bump github.com/docker/docker from 23.0.4+incompatible to 23.0.5+incompatible by @dependabot[bot]
- Bump github.com/docker/docker from 23.0.3+incompatible to 23.0.4+incompatible by @dependabot[bot]
- Bump github.com/docker/cli from 23.0.3+incompatible to 23.0.4+incompatible by @dependabot[bot]
- Bump github.com/prometheus/client_golang by @dependabot[bot]
- Bump github.com/stretchr/testify from 1.8.1 to 1.8.2 by @dependabot[bot]
- Bump github.com/robfig/cron by @dependabot[bot]
- Bump github.com/spf13/cobra from 1.6.1 to 1.7.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.26.0 to 1.27.6 by @dependabot[bot]
- Bump golang.org/x/net from 0.5.0 to 0.9.0 by @dependabot[bot]
- Bump golang.org/x/text from 0.6.0 to 0.8.0 by @dependabot[bot]
- Bump github.com/docker/cli from 20.10.23+incompatible to 23.0.3+incompatible by @dependabot[bot]
- Bump github.com/docker/docker from 23.0.2+incompatible to 23.0.3+incompatible by @dependabot[bot]
- Bump actions/setup-go from 3 to 4 by @dependabot[bot]
- Bump alpine from 3.17.1 to 3.17.3 in /dockerfiles by @dependabot[bot]
- Bump docker/docker from 20.10.23+inc to 23.0.2+inc by @dependabot[bot]
- Bump andrewslotin/go-proxy-pull-action from 1.0.3 to 1.1.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.25.0 to 1.26.0 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 4.1.0 to 4.2.0 by @dependabot[bot]
- Bump github.com/docker/cli from 20.10.22+incompatible to 20.10.23+incompatible by @dependabot[bot]
- Bump github.com/spf13/viper from 1.14.0 to 1.15.0 by @dependabot[bot]
- Bump github.com/docker/docker from 20.10.22+incompatible to 20.10.23+incompatible by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.24.2 to 1.25.0 by @dependabot[bot]
- Bump golang.org/x/net from 0.4.0 to 0.5.0 by @dependabot[bot]
- Bump dominikh/staticcheck-action from 1.2.0 to 1.3.0 by @dependabot[bot]
- Bump alpine from 3.17.0 to 3.17.1 in /dockerfiles by @dependabot[bot]
- Bump golang.org/x/text from 0.5.0 to 0.6.0 by @dependabot[bot]
- Bump github.com/docker/docker by @dependabot[bot]
- Bump github.com/docker/cli by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.24.1 to 1.24.2 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 3.2.0 to 4.1.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.24.0 to 1.24.1 by @dependabot[bot]
- Bump golang.org/x/net from 0.3.0 to 0.4.0 by @dependabot[bot]
- Bump golang.org/x/net from 0.1.0 to 0.3.0 by @dependabot[bot]
- Bump golang.org/x/text from 0.4.0 to 0.5.0 by @dependabot[bot]
- Bump alpine from 3.16.2 to 3.17.0 in /dockerfiles by @dependabot[bot]
- Bump github.com/spf13/viper from 1.13.0 to 1.14.0 by @dependabot[bot]
- Bump github.com/spf13/cobra from 1.6.0 to 1.6.1 by @dependabot[bot]
- Bump github.com/prometheus/client_golang from 1.13.0 to 1.14.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.23.0 to 1.24.0 by @dependabot[bot]
- Bump github.com/docker/cli from 20.10.19+incompatible to 20.10.21+incompatible by @dependabot[bot]
- Bump github.com/docker/docker from 20.10.19+incompatible to 20.10.21+incompatible by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.22.1 to 1.23.0 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 3.1.0 to 3.2.0 by @dependabot[bot]
- Bulk update dependencies by @piksel
- Bump github.com/docker/cli from 20.10.18+incompatible to 20.10.19+incompatible by @dependabot[bot]
- Bump github.com/docker/docker from 20.10.18+incompatible to 20.10.19+incompatible by @dependabot[bot]
- Bump github.com/spf13/cobra from 1.5.0 to 1.6.0 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.21.1 to 1.22.1 by @dependabot[bot]
- Bump golang.org/x/text from 0.3.7 to 0.3.8 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.20.2 to 1.21.1 by @dependabot[bot]
- Add text render and remove default value by @Choromanski
- Bump github.com/spf13/viper from 1.12.0 to 1.13.0 by @dependabot[bot]
- Bump github.com/docker/cli from 20.10.17+incompatible to 20.10.18+incompatible by @dependabot[bot]
- Bump github.com/docker/docker from 20.10.17+incompatible to 20.10.18+incompatible by @dependabot[bot]
- Bump github/codeql-action from 1 to 2 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.20.1 to 1.20.2 by @dependabot[bot]
- Bump github.com/prometheus/client_golang from 1.7.1 to 1.13.0 by @dependabot[bot]
- Update go version to 1.18 by @jauderho
- Bump actions/setup-go from 2 to 3 by @dependabot[bot]
- Bump actions/checkout from 2 to 3 by @dependabot[bot]
- Bump github.com/spf13/viper from 1.6.3 to 1.12.0 by @dependabot[bot]
- Bump github.com/docker/distribution from 2.8.0+incompatible to 2.8.1+incompatible by @dependabot[bot]
- Bump codecov/codecov-action from 1 to 3 by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.20.0 to 1.20.1 by @dependabot[bot]
- Bump github.com/stretchr/testify from 1.6.1 to 1.8.0 by @dependabot[bot]
- Bump goreleaser/goreleaser-action from 2 to 3 by @dependabot[bot]
- Bump alpine from 3.15 to 3.16.2 in /dockerfiles by @dependabot[bot]
- Bump github.com/onsi/gomega from 1.10.3 to 1.20.0 by @dependabot[bot]
- Bump github.com/sirupsen/logrus from 1.8.1 to 1.9.0 by @dependabot[bot]
- Bump actions/setup-python from 3 to 4 by @dependabot[bot]
- Bump docker/login-action from 1 to 2 by @dependabot[bot]
- Bump github.com/spf13/cobra from 1.4.0 to 1.5.0 by @dependabot[bot]
- Bump github.com/onsi/ginkgo from 1.14.2 to 1.16.5 by @dependabot[bot]
- Bump github.com/docker/distribution from 2.7.1+incompatible to 2.8.0+incompatible by @dependabot[bot]
- Bump alpine version in dockerfile by @naioja
- Bump shoutrrr and containrd by @piksel
- Run code coverage on main push by @piksel
- Fix default branch in Dockerfiles by @piksel
- Set image platform on image build by @piksel
- Fix badge url for contributors and docker pulls by @DaxServer
- Update Badges on Docs by @DaxServer
- Disable fail-fast for pr tests by @piksel
- Add coverage upload by @simonaronsson
- Set up release workflow by @simonaronsson
- Add manual dispatch by @simonaronsson
- Rename workflow by @simonaronsson
- Move to github actions by @simonaronsson

### New Contributors

- @nicholas-fedor made their first contribution
- @rdamazio made their first contribution
- @jebabin made their first contribution
- @szaimen made their first contribution
- @bugficks made their first contribution
- @GridexX made their first contribution
- @valankar made their first contribution
- @testwill made their first contribution
- @aioue made their first contribution
- @kirbylink made their first contribution
- @Tentoe made their first contribution
- @schizo99 made their first contribution
- @SergeAx made their first contribution
- @Pwuts made their first contribution
- @louis-genestier made their first contribution
- @gilbsgilbs made their first contribution
- @DMJoh made their first contribution
- @SamKirsch10 made their first contribution
- @step-security-bot made their first contribution
- @carueda made their first contribution
- @Choromanski made their first contribution
- @tamalCodes made their first contribution
- @matthewmcneely made their first contribution
- @jauderho made their first contribution
- @mateuszdrab made their first contribution
- @EDIflyer made their first contribution
- @jlaska made their first contribution
- @frinzekt made their first contribution
- @Foxite made their first contribution
- @lyh16 made their first contribution
- @ksurl made their first contribution
- @lazou made their first contribution
- @jamesmacwhite made their first contribution
- @atombrella made their first contribution
- @naioja made their first contribution
- @jiangtj made their first contribution
- @jinnatar made their first contribution
- @patricegautier made their first contribution
- @MorrisLaw made their first contribution
- @hypnoglow made their first contribution
- @dhiemaz made their first contribution
- @rootulp made their first contribution
- @modem7 made their first contribution
- @jerbob made their first contribution
- @djquan made their first contribution
- @noplanman made their first contribution
- @ made their first contribution
- @nightah made their first contribution
- @zenjabba made their first contribution
- @DasSkelett made their first contribution
- @reinout made their first contribution
- @zoispag made their first contribution
- @kopiro made their first contribution
- @hydrargyrum made their first contribution
- @dapplion made their first contribution
- @jokay made their first contribution
- @nymous made their first contribution
- @DaxServer made their first contribution
- @ilike2burnthing made their first contribution
- @jeroenrnl made their first contribution
- @tkalus made their first contribution
- @rg9400 made their first contribution
- @TheCoolBlackCat made their first contribution
- @chander made their first contribution
- @dhet made their first contribution
- @osheroff made their first contribution
- @victorcmoura made their first contribution
- @CedricFinance made their first contribution
- @m-sedl made their first contribution
- @MichaelSp made their first contribution
- @PssbleTrngle made their first contribution
- @tammert made their first contribution
- @mattdoran made their first contribution
- @arnested made their first contribution
- @pgrimaud made their first contribution
- @Miosame made their first contribution
- @ogmueller made their first contribution
- @bopoh24 made their first contribution
- @Saicheg made their first contribution
- @patski123 made their first contribution
- @mbrandau made their first contribution
- @aneisch made their first contribution
- @sixcorners made their first contribution
- @Germs2004 made their first contribution
- @pagdot made their first contribution
- @raymondelooff made their first contribution
- @codingCoffee made their first contribution
- @jnidzwetzki made their first contribution
- @mindrunner made their first contribution
- @jsclayton made their first contribution
- @pjknkda made their first contribution
- @foxbit19 made their first contribution
- @8ear made their first contribution
- @foosel made their first contribution
- @sixth made their first contribution
- @kaloyan-raev made their first contribution
- @rjbudke made their first contribution
- @chugunov made their first contribution
- @lukapeschke made their first contribution
- @gitter-badger made their first contribution
- @svengo made their first contribution
- @techknowlogick made their first contribution
- @thelamer made their first contribution
- @KopfKrieg made their first contribution
- @mariotacke made their first contribution
- @waja made their first contribution
- @salbertson made their first contribution
- @connormcmk made their first contribution
- @stffabi made their first contribution
- @huddlesj made their first contribution
- @schnapster made their first contribution
- @darknode made their first contribution
- @Robotex made their first contribution
- @ubergesundheit made their first contribution
- @fomk made their first contribution
- @mxschmitt made their first contribution
- @cron410 made their first contribution
- @belak made their first contribution
- @mrw34 made their first contribution
- @wmbutler made their first contribution
- @dolanor made their first contribution
- @ATCUSA made their first contribution
- @rosscado made their first contribution
- @haswalt made their first contribution
- @bdehamer made their first contribution

## Compare Releases

- [unreleased](https://github.com/nicholas-fedor/watchtower/compare/v1.23.0...HEAD)
- [1.23.0](https://github.com/nicholas-fedor/watchtower/compare/v1.22.3...v1.23.0)
- [1.22.3](https://github.com/nicholas-fedor/watchtower/compare/v1.22.2...v1.22.3)
- [1.22.2](https://github.com/nicholas-fedor/watchtower/compare/v1.22.1...v1.22.2)
- [1.22.1](https://github.com/nicholas-fedor/watchtower/compare/v1.22.0...v1.22.1)
- [1.22.0](https://github.com/nicholas-fedor/watchtower/compare/v1.21.2...v1.22.0)
- [1.21.2](https://github.com/nicholas-fedor/watchtower/compare/v1.21.1...v1.21.2)
- [1.21.1](https://github.com/nicholas-fedor/watchtower/compare/v1.21.0...v1.21.1)
- [1.21.0](https://github.com/nicholas-fedor/watchtower/compare/v1.20.3...v1.21.0)
- [1.20.3](https://github.com/nicholas-fedor/watchtower/compare/v1.20.2...v1.20.3)
- [1.20.2](https://github.com/nicholas-fedor/watchtower/compare/v1.20.1...v1.20.2)
- [1.20.1](https://github.com/nicholas-fedor/watchtower/compare/v1.20.0...v1.20.1)
- [1.20.0](https://github.com/nicholas-fedor/watchtower/compare/v1.19.0...v1.20.0)
- [1.19.0](https://github.com/nicholas-fedor/watchtower/compare/v1.18.1...v1.19.0)
- [1.18.1](https://github.com/nicholas-fedor/watchtower/compare/v1.18.0...v1.18.1)
- [1.18.0](https://github.com/nicholas-fedor/watchtower/compare/v1.17.2...v1.18.0)
- [1.17.2](https://github.com/nicholas-fedor/watchtower/compare/v1.17.1...v1.17.2)
- [1.17.1](https://github.com/nicholas-fedor/watchtower/compare/v1.17.0...v1.17.1)
- [1.17.0](https://github.com/nicholas-fedor/watchtower/compare/v1.16.1...v1.17.0)
- [1.16.1](https://github.com/nicholas-fedor/watchtower/compare/v1.16.0...v1.16.1)
- [1.16.0](https://github.com/nicholas-fedor/watchtower/compare/v1.15.0...v1.16.0)
- [1.15.0](https://github.com/nicholas-fedor/watchtower/compare/v1.14.4...v1.15.0)
- [1.14.4](https://github.com/nicholas-fedor/watchtower/compare/v1.14.3...v1.14.4)
- [1.14.3](https://github.com/nicholas-fedor/watchtower/compare/v1.14.2...v1.14.3)
- [1.14.2](https://github.com/nicholas-fedor/watchtower/compare/v1.14.1...v1.14.2)
- [1.14.1](https://github.com/nicholas-fedor/watchtower/compare/v1.14.0...v1.14.1)
- [1.14.0](https://github.com/nicholas-fedor/watchtower/compare/v1.13.1...v1.14.0)
- [1.13.1](https://github.com/nicholas-fedor/watchtower/compare/v1.13.0...v1.13.1)
- [1.13.0](https://github.com/nicholas-fedor/watchtower/compare/v1.12.5...v1.13.0)
- [1.12.5](https://github.com/nicholas-fedor/watchtower/compare/v1.12.4...v1.12.5)
- [1.12.4](https://github.com/nicholas-fedor/watchtower/compare/v1.12.3...v1.12.4)
- [1.12.3](https://github.com/nicholas-fedor/watchtower/compare/v1.12.2...v1.12.3)
- [1.12.2](https://github.com/nicholas-fedor/watchtower/compare/v1.12.1...v1.12.2)
- [1.12.1](https://github.com/nicholas-fedor/watchtower/compare/v1.12.0...v1.12.1)
- [1.12.0](https://github.com/nicholas-fedor/watchtower/compare/v1.11.8...v1.12.0)
- [1.11.8](https://github.com/nicholas-fedor/watchtower/compare/v1.11.7...v1.11.8)
- [1.11.7](https://github.com/nicholas-fedor/watchtower/compare/v1.11.6...v1.11.7)
- [1.11.6](https://github.com/nicholas-fedor/watchtower/compare/v1.11.5...v1.11.6)
- [1.11.5](https://github.com/nicholas-fedor/watchtower/compare/v1.11.4...v1.11.5)
- [1.11.4](https://github.com/nicholas-fedor/watchtower/compare/v1.11.3...v1.11.4)
- [1.11.3](https://github.com/nicholas-fedor/watchtower/compare/v1.11.2...v1.11.3)
- [1.11.2](https://github.com/nicholas-fedor/watchtower/compare/v1.11.1...v1.11.2)
- [1.11.1](https://github.com/nicholas-fedor/watchtower/compare/v1.11.0...v1.11.1)
- [1.11.0](https://github.com/nicholas-fedor/watchtower/compare/v1.10.0...v1.11.0)
- [1.10.0](https://github.com/nicholas-fedor/watchtower/compare/v1.9.2...v1.10.0)
- [1.9.2](https://github.com/nicholas-fedor/watchtower/compare/v1.9.1...v1.9.2)
- [1.9.1](https://github.com/nicholas-fedor/watchtower/compare/v1.9.0...v1.9.1)
- [1.9.0](https://github.com/nicholas-fedor/watchtower/compare/v1.8.9...v1.9.0)
- [1.8.9](https://github.com/nicholas-fedor/watchtower/compare/v1.8.8...v1.8.9)
- [1.8.8](https://github.com/nicholas-fedor/watchtower/compare/v1.8.7...v1.8.8)
- [1.8.7](https://github.com/nicholas-fedor/watchtower/compare/v1.8.6...v1.8.7)
- [1.8.6](https://github.com/nicholas-fedor/watchtower/compare/v1.8.5...v1.8.6)
- [1.8.5](https://github.com/nicholas-fedor/watchtower/compare/v1.8.4...v1.8.5)
- [1.8.4](https://github.com/nicholas-fedor/watchtower/compare/v1.8.3...v1.8.4)
- [1.8.3](https://github.com/nicholas-fedor/watchtower/compare/v1.8.2...v1.8.3)
- [1.8.2](https://github.com/nicholas-fedor/watchtower/compare/v1.8.1...v1.8.2)
- [1.8.1](https://github.com/nicholas-fedor/watchtower/compare/v1.8.0...v1.8.1)
- [1.8.0](https://github.com/nicholas-fedor/watchtower/compare/v1.7.13...v1.8.0)
- [1.7.13](https://github.com/nicholas-fedor/watchtower/compare/v1.7.12...v1.7.13)
- [1.7.12](https://github.com/nicholas-fedor/watchtower/compare/v1.7.11...v1.7.12)
- [1.7.11](https://github.com/nicholas-fedor/watchtower/compare/v1.7.10...v1.7.11)
- [1.7.10](https://github.com/nicholas-fedor/watchtower/compare/v1.7.2...v1.7.10)

<!-- generated by git-cliff -->
