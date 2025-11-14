package auth

import (
	"sync"
	"testing"
	"time"
)

// Test concurrent stores and gets to ensure the mutex protects the cache
func TestTokenCacheConcurrentStoreAndGet(t *testing.T) {
	// reset cache safely
	tokenCacheMu.Lock()
	tokenCache = map[string]cachedToken{}
	tokenCacheMu.Unlock()

	origNow := now
	defer func() { now = origNow }()
	now = time.Now

	key := "concurrent-key"
	token := "tok-concurrent"

	var wg sync.WaitGroup
	storeers := 50
	getters := 50
	iters := 100

	for i := 0; i < storeers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				storeToken(key, token, 0)
			}
		}()
	}

	for i := 0; i < getters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_ = getCachedToken(key)
			}
		}()
	}

	wg.Wait()

	if got := getCachedToken(key); got != token {
		t.Fatalf("expected token %q, got %q", token, got)
	}
}

// Test concurrent access while token expires: readers run while time is advanced
func TestTokenCacheConcurrentExpiry(t *testing.T) {
	// reset cache safely
	tokenCacheMu.Lock()
	tokenCache = map[string]cachedToken{}
	tokenCacheMu.Unlock()

	// Make now controllable and thread-safe
	origNow := now
	defer func() { now = origNow }()

	base := time.Now()
	var mu sync.Mutex
	current := base
	now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return current
	}

	key := "concurrent-expire"
	storeToken(key, "t", 1)

	var wg sync.WaitGroup
	readers := 100

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = getCachedToken(key)
			}
		}()
	}

	// advance time beyond ttl
	mu.Lock()
	current = current.Add(2 * time.Second)
	mu.Unlock()

	wg.Wait()

	if got := getCachedToken(key); got != "" {
		t.Fatalf("expected token to be expired, got %q", got)
	}
}
