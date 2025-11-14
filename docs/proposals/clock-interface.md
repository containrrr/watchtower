# Proposal: Clock interface for deterministic time

Summary
-------
Introduce a `Clock` interface and replace package-level `now = time.Now` usages in targeted packages (e.g. `pkg/registry/auth`) with dependency injection via a `Clock` to allow deterministic tests and easier control over time in the future.

Motivation
----------
- Currently some packages expose a package-level `now` variable used by tests to override time. A dedicated `Clock` interface reduces global state and improves testability across packages.

Proposal
--------
- Define `type Clock interface { Now() time.Time }` in `pkg/clock/clock.go` and provide `RealClock` implementation that calls `time.Now()`.
- Introduce constructors or package-level `SetClock(clock Clock)` functions where needed (e.g. `pkg/registry/auth`) or prefer injecting `Clock` via function parameters in higher-level constructors.

Compatibility
-------------
- Backwards-compatible: default behavior uses `RealClock` when no override is provided.

Migration
---------
1. Add `pkg/clock/clock.go` containing `Clock` and `RealClock`.
2. Update `pkg/registry/auth` to use `clock.Now()` through injection or a package-level `clock` variable initialized to `RealClock{}`.
3. Replace test overrides of `now` by using a `FakeClock` in tests.

Risks
-----
- Small refactor; tests will need updates but should be straightforward.

References
----------
- Current usage: `pkg/registry/auth/auth.go` uses `var now = time.Now`.
