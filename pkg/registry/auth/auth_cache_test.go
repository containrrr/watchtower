package auth

import (
    "testing"
    "time"
)

func TestTokenCacheStoreAndGetHitAndMiss(t *testing.T) {
    // save and restore original now
    origNow := now
    defer func() { now = origNow }()

    // deterministic fake time
    base := time.Date(2025, time.November, 13, 12, 0, 0, 0, time.UTC)
    now = func() time.Time { return base }

    key := "https://auth.example.com/?service=example&scope=repository:repo:pull"
    // ensure empty at start
    if got := getCachedToken(key); got != "" {
        t.Fatalf("expected empty cache initially, got %q", got)
    }

    // store with no expiry (ttl <= 0)
    storeToken(key, "tok-123", 0)
    if got := getCachedToken(key); got != "tok-123" {
        t.Fatalf("expected token tok-123, got %q", got)
    }
}

func TestTokenCacheExpiry(t *testing.T) {
    // save and restore original now
    origNow := now
    defer func() { now = origNow }()

    // deterministic fake time that can be moved forward
    base := time.Date(2025, time.November, 13, 12, 0, 0, 0, time.UTC)
    current := base
    now = func() time.Time { return current }

    key := "https://auth.example.com/?service=example&scope=repository:repo2:pull"
    // store with short ttl (1 second)
    storeToken(key, "short-tok", 1)

    if got := getCachedToken(key); got != "short-tok" {
        t.Fatalf("expected token short-tok immediately after store, got %q", got)
    }

    // advance time beyond ttl
    current = current.Add(2 * time.Second)

    if got := getCachedToken(key); got != "" {
        t.Fatalf("expected token to be expired and removed, got %q", got)
    }
}
