# Proposal: Persistent / Distributed Token Cache

Summary
-------
Introduce an optional pluggable token cache interface for registry auth tokens so deployments can opt for a shared cache (Redis, Memcached, or file-backed) across multiple Watchtower instances.

Motivation
----------
- In multi-instance deployments, the in-memory token cache avoids redundant token requests only per instance. A shared cache reduces token endpoint load and synchronizes token usage across instances.

Proposal
--------
- Define a `TokenCache` interface (Get/Set/Delete) in `pkg/registry/auth/cache_interface.go`.
- Keep the existing in-memory cache as the default implementation.
- Provide example Redis-backed implementation in `contrib/redis-token-cache/` (optional).

Migration
---------
1. Add `TokenCache` interface and adapter in `pkg/registry/auth`.
2. Wire `TokenCache` into `GetBearerHeader` to check the cache via the interface.
3. Add configuration options or environment variable to enable persistent cache and connection details.

Risks
-----
- Operational complexity for configuration (credentials for Redis, etc.).
- Need to handle TTL semantics and clock skew.

References
----------
- Current in-memory cache: `pkg/registry/auth/auth.go` (`tokenCache`, `getCachedToken`, `storeToken`).
