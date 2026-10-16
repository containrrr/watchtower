package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
)

func TestNew(t *testing.T) {
	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}

	tests := []struct {
		name       string
		updateFn   func(ctx context.Context, images, containers []string) *metrics.Metric
		updateLock *scheduling.UpdateLock
		wantPath   string
	}{
		{
			name:       "with provided lock uses it",
			updateFn:   updateFn,
			updateLock: scheduling.NewUpdateLock(),
			wantPath:   "/v1/update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(testLogger(), tt.updateFn, tt.updateLock)
			require.NotNil(t, h)
			assert.Equal(t, tt.wantPath, h.Path)
		})
	}
}

func TestHandler_Handle_Sync(t *testing.T) {
	var called atomic.Int32

	expectedMetric := &metrics.Metric{Scanned: 5, Updated: 2, Failed: 1, Restarted: 1, Skipped: 0}

	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		called.Add(1)

		return expectedMetric
	}

	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), updateFn, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), called.Load())
}

func TestHandler_Handle_Async(t *testing.T) {
	var called atomic.Int32

	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		called.Add(1)

		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?async=true", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), called.Load())
}

func TestHandler_Handle_FullUpdateLocked(t *testing.T) {
	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		t.Error("update function should not be called when lock is held")

		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "30", resp.Header.Get("Retry-After"))

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "another update is already running", result["error"])
	assert.Equal(t, "v1", result["api_version"])
}

func TestHandler_Handle_TargetedUpdateBlocks(t *testing.T) {
	var called atomic.Int32

	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		called.Add(1)

		return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
	}

	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())
	h := New(testLogger(), updateFn, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	done := make(chan int, 1)

	go func() {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?image=myimage:latest", nil)

		resp, err := app.Test(req)
		if err != nil {
			done <- 0

			return
		}
		defer resp.Body.Close()

		done <- resp.StatusCode
	}()

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, int32(0), called.Load())

	lock.Unlock()

	time.Sleep(50 * time.Millisecond)

	code := <-done
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, int32(1), called.Load())
}

func TestHandler_Handle_TargetedContainerUpdateBlocks(t *testing.T) {
	var called atomic.Int32

	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		called.Add(1)

		return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
	}

	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())
	h := New(testLogger(), updateFn, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	done := make(chan int, 1)

	go func() {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?container=mycontainer", nil)

		resp, err := app.Test(req)
		if err != nil {
			done <- 0

			return
		}
		defer resp.Body.Close()

		done <- resp.StatusCode
	}()

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, int32(0), called.Load())

	lock.Unlock()

	time.Sleep(50 * time.Millisecond)

	code := <-done
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, int32(1), called.Load())
}

func TestHandler_Handle_ContextCancellation(t *testing.T) {
	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan int, 1)

	go func() {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/update?image=test:latest", strings.NewReader("test body"))

		resp, err := app.Test(req)
		if err != nil {
			done <- 0

			return
		}
		defer resp.Body.Close()

		done <- resp.StatusCode
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	code := <-done
	assert.True(t, code == http.StatusServiceUnavailable || code == 0,
		"expected 503 or 0, got %d", code)
}

func TestHandler_Handle_PanicRecovery(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		panic("simulated panic")
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?async=true", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	time.Sleep(50 * time.Millisecond)

	assert.True(t, lock.TryLock(), "lock was not released after panic")
}

func TestExtractImages_Unit(t *testing.T) {
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, scheduling.NewUpdateLock())

	tests := []struct {
		name      string
		query     string
		wantLen   int
		wantFirst string
	}{
		{
			name:      "single image",
			query:     "image=nginx:latest",
			wantLen:   1,
			wantFirst: "nginx:latest",
		},
		{
			name:      "comma-separated images",
			query:     "image=nginx:latest,redis:7",
			wantLen:   2,
			wantFirst: "nginx:latest",
		},
		{
			name:      "multiple image params",
			query:     "image=nginx:latest&image=redis:7",
			wantLen:   2,
			wantFirst: "nginx:latest",
		},
		{
			name:      "mixed comma and multiple params",
			query:     "image=nginx:latest,redis:7&image=postgres:16",
			wantLen:   3,
			wantFirst: "nginx:latest",
		},
		{
			name:    "no image param",
			query:   "other=value",
			wantLen: 0,
		},
		{
			name:    "empty query",
			query:   "",
			wantLen: 0,
		},
		{
			name:    "empty image value is filtered out",
			query:   "image=",
			wantLen: 0,
		},
		{
			name:    "only commas are filtered out",
			query:   "image=,,,",
			wantLen: 0,
		},
		{
			name:      "whitespace is trimmed",
			query:     "image=%20nginx:latest%20",
			wantLen:   1,
			wantFirst: "nginx:latest",
		},
		{
			name:      "image with registry path",
			query:     "image=registry.com/org/image:v1.2.3",
			wantLen:   1,
			wantFirst: "registry.com/org/image:v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedImages []string

			app := fiber.New(fiber.Config{})
			app.Get("/test", func(c fiber.Ctx) error {
				capturedImages = h.extractImages(c)

				return nil
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test?"+tt.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Len(t, capturedImages, tt.wantLen, "image count mismatch")

			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantFirst, capturedImages[0], "first image mismatch")
			}
		})
	}
}

func TestExtractContainers_Unit(t *testing.T) {
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, scheduling.NewUpdateLock())

	tests := []struct {
		name      string
		query     string
		wantLen   int
		wantFirst string
	}{
		{
			name:      "single container",
			query:     "container=mycontainer",
			wantLen:   1,
			wantFirst: "mycontainer",
		},
		{
			name:      "comma-separated containers",
			query:     "container=container1,container2",
			wantLen:   2,
			wantFirst: "container1",
		},
		{
			name:      "multiple container params",
			query:     "container=container1&container=container2",
			wantLen:   2,
			wantFirst: "container1",
		},
		{
			name:      "regex pattern",
			query:     "container=^web-.*",
			wantLen:   1,
			wantFirst: "^web-.*",
		},
		{
			name:    "no container param",
			query:   "other=value",
			wantLen: 0,
		},
		{
			name:    "empty query",
			query:   "",
			wantLen: 0,
		},
		{
			name:    "empty container value is filtered out",
			query:   "container=",
			wantLen: 0,
		},
		{
			name:    "only commas are filtered out",
			query:   "container=,,,",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedContainers []string

			app := fiber.New(fiber.Config{})
			app.Get("/test", func(c fiber.Ctx) error {
				capturedContainers = h.extractContainers(c)

				return nil
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test?"+tt.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Len(t, capturedContainers, tt.wantLen, "container count mismatch")

			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantFirst, capturedContainers[0], "first container mismatch")
			}
		})
	}
}

func Test_send429Response(t *testing.T) {
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, scheduling.NewUpdateLock())

	app := fiber.New(fiber.Config{})
	app.Post("/test", func(c fiber.Ctx) error {
		h.send429Response(c)

		return nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "30", resp.Header.Get("Retry-After"))

	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	assert.Contains(t, body, "another update is already running")
	assert.Contains(t, body, "api_version")
	assert.Contains(t, body, "v1")
}

func Test_executeUpdate(t *testing.T) {
	expected := &metrics.Metric{Scanned: 10, Updated: 5, Failed: 2}

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return expected
	}, scheduling.NewUpdateLock())

	metric, duration := h.executeUpdate(t.Context(), []string{"nginx:latest"}, nil)
	assert.Equal(t, expected, metric)
	assert.GreaterOrEqual(t, duration, time.Duration(0))
}

func Test_executeUpdateAsync(t *testing.T) {
	var called atomic.Int32

	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		called.Add(1)

		return &metrics.Metric{}
	}, lock)

	require.True(t, lock.TryLock())
	h.executeUpdateAsync(t.Context(), nil, nil)

	assert.Equal(t, int32(1), called.Load())

	assert.True(t, lock.TryLock(), "lock was not released")
}

func Test_contextForAsync(t *testing.T) {
	t.Run("without deadline uses handler context", func(t *testing.T) {
		type ctxKey struct{}

		handlerCtx := context.WithValue(context.Background(), ctxKey{}, "handler")
		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, scheduling.NewUpdateLock(), handlerCtx)

		ctx, cancel := h.contextForAsync(context.Background())
		defer cancel()

		assert.Equal(t, handlerCtx, ctx)
		_, ok := ctx.Deadline()
		assert.False(t, ok)
	})

	t.Run("projects deadline without inheriting cancellation", func(t *testing.T) {
		handlerCtx := context.Background()
		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, scheduling.NewUpdateLock(), handlerCtx)

		deadline := time.Now().Add(3 * time.Minute).Truncate(time.Millisecond)
		updateCtx, cancelUpdate := context.WithDeadline(context.Background(), deadline)
		cancelUpdate()

		require.ErrorIs(t, updateCtx.Err(), context.Canceled)

		ctx, cancel := h.contextForAsync(updateCtx)
		defer cancel()

		select {
		case <-ctx.Done():
			t.Fatal("async context must not be canceled when updateCtx is canceled")
		default:
		}

		gotDeadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.WithinDuration(t, deadline, gotDeadline, time.Millisecond)
	})

	t.Run("cancels when handler context is canceled", func(t *testing.T) {
		handlerCtx, handlerCancel := context.WithCancel(context.Background())
		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, scheduling.NewUpdateLock(), handlerCtx)

		deadline := time.Now().Add(time.Minute)

		updateCtx, cancelUpdate := context.WithDeadline(context.Background(), deadline)
		defer cancelUpdate()

		ctx, cancel := h.contextForAsync(updateCtx)
		defer cancel()

		handlerCancel()

		require.Eventually(t, func() bool {
			return ctx.Err() != nil
		}, time.Second, time.Millisecond, "async context should cancel with handler context")
	})
}

func Test_executeUpdateAsync_ignoresRequestContextCancellation(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	handlerCtx := context.Background()
	started := make(chan context.Context, 1)
	release := make(chan struct{})

	h := New(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
		started <- ctx

		<-release

		return &metrics.Metric{}
	}, lock, handlerCtx)

	deadline := time.Now().Add(2 * time.Minute)
	updateCtx, cancelUpdate := context.WithDeadline(context.Background(), deadline)
	cancelUpdate()

	require.True(t, lock.TryLock())

	done := make(chan struct{})
	go func() {
		defer close(done)

		h.executeUpdateAsync(updateCtx, nil, nil)
	}()

	var observedCtx context.Context
	select {
	case observedCtx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("update function was not started")
	}

	select {
	case <-observedCtx.Done():
		t.Fatal("async update must not observe a canceled context from the request")
	default:
	}

	gotDeadline, ok := observedCtx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, deadline, gotDeadline, time.Millisecond)

	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("executeUpdateAsync did not finish")
	}

	assert.True(t, lock.TryLock(), "lock was not released")
}

func Test_releaseLock(t *testing.T) {
	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())
	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, lock)

	h.releaseLock()

	assert.True(t, lock.TryLock(), "the lock was still held after releaseLock")
}

func Test_handleSync(t *testing.T) {
	expected := &metrics.Metric{
		Scanned: 8, Updated: 3, Failed: 1, Restarted: 2, Skipped: 1,
	}

	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return expected
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	buf := make([]byte, 2048)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	assert.Contains(t, body, "summary")
	assert.Contains(t, body, "timing")
	assert.Contains(t, body, "scanned")
	assert.Contains(t, body, "api_version")
}

func Test_handleSync_JSONStructure(t *testing.T) {
	expected := &metrics.Metric{
		Scanned: 10, Updated: 3, Failed: 1, Restarted: 2, Skipped: 0,
	}

	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return expected
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	summary := result["summary"].(map[string]any)
	assert.InDelta(t, float64(10), summary["scanned"], 0.001)
	assert.InDelta(t, float64(3), summary["updated"], 0.001)
	assert.InDelta(t, float64(1), summary["failed"], 0.001)
	assert.InDelta(t, float64(2), summary["restarted"], 0.001)
	assert.InDelta(t, float64(0), summary["skipped"], 0.001)

	timing := result["timing"].(map[string]any)
	assert.NotNil(t, timing["duration_ms"])
	assert.NotNil(t, timing["duration"])

	assert.NotEmpty(t, result["timestamp"])
	assert.Equal(t, "v1", result["api_version"])
}

func Test_handleAsync(t *testing.T) {
	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/test", func(c fiber.Ctx) error {
		return h.handleAsync(c, nil, nil, c.Context())
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	require.Eventually(t, lock.TryLock, 2*time.Second, 10*time.Millisecond, "lock should be released after async update")
}

func Test_handleAsync_survivesRequestContextCancellation(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	started := make(chan context.Context, 1)
	release := make(chan struct{})

	h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
		started <- ctx

		<-release

		return &metrics.Metric{}
	}, lock, 5*time.Minute)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	reqCtx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(reqCtx, http.MethodPost, "/v1/update?async=true", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var observedCtx context.Context
	select {
	case observedCtx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("update function was not started")
	}

	// Cancel the request-scoped context after the response was sent while the
	// async update is still in progress.
	cancel()
	time.Sleep(20 * time.Millisecond)

	select {
	case <-observedCtx.Done():
		t.Fatal("async update observed canceled context; it must not use the request context")
	default:
	}

	close(release)

	require.Eventually(t, lock.TryLock, 2*time.Second, 10*time.Millisecond, "lock should be released after async update")
}

func Test_handleAsync_survivesTimeoutMiddlewareCancellation(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	started := make(chan context.Context, 1)
	release := make(chan struct{})

	routeTimeout := 5 * time.Minute
	h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
		started <- ctx

		<-release

		return &metrics.Metric{}
	}, lock, routeTimeout)

	app := fiber.New(fiber.Config{})
	// Match production wiring: timeout middleware cancels c.Context() when the
	// handler returns after sending 202.
	app.Post("/v1/update", timeout.New(h.Handle, timeout.Config{
		Timeout: routeTimeout,
	}))

	before := time.Now()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?async=true", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var observedCtx context.Context
	select {
	case observedCtx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("update function was not started")
	}

	// Middleware cancel runs when the handler returns; allow it to propagate.
	time.Sleep(20 * time.Millisecond)

	select {
	case <-observedCtx.Done():
		t.Fatal("async update must survive Fiber timeout middleware cancellation after 202")
	default:
	}

	deadline, ok := observedCtx.Deadline()
	require.True(t, ok, "async update should retain the route timeout deadline")
	assert.WithinDuration(t, before.Add(routeTimeout), deadline, 5*time.Second)

	close(release)

	require.Eventually(t, lock.TryLock, 2*time.Second, 10*time.Millisecond, "lock should be released after async update")
}

func Test_handleSync_NilMetric(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return nil
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func Test_acquireLock_ContextCancelled(t *testing.T) {
	lock := scheduling.NewUpdateLock()

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})

	var handlerCalled atomic.Bool

	app.Post("/v1/update", func(c fiber.Ctx) error {
		handlerCalled.Store(true)

		return h.Handle(c)
	})

	ctx, cancel := context.WithCancel(t.Context())

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/update?image=nginx", nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 2 * time.Second,
	})
	if err == nil {
		defer resp.Body.Close()
	}

	assert.True(t, handlerCalled.Load(), "handler should have been called")
}

func Test_acquireLock_FullUpdateBusy(t *testing.T) {
	lock := scheduling.NewUpdateLock()
	require.True(t, lock.TryLock())

	h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
		return &metrics.Metric{}
	}, lock)

	app := fiber.New(fiber.Config{})
	app.Post("/v1/update", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	retryAfter := resp.Header.Get("Retry-After")
	assert.Equal(t, "30", retryAfter)
}

func TestHandler_Handle_TimeoutOverride(t *testing.T) {
	t.Run("sync update respects valid timeout", func(t *testing.T) {
		lock := scheduling.NewUpdateLock()

		h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, lock, 5*time.Minute)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/update", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?timeout=2m", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("sync update clamps timeout to max", func(t *testing.T) {
		lock := scheduling.NewUpdateLock()

		h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, lock, 2*time.Minute)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/update", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?timeout=5m", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("async update respects valid timeout", func(t *testing.T) {
		lock := scheduling.NewUpdateLock()

		started := make(chan context.Context, 1)

		h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
			started <- ctx

			return &metrics.Metric{}
		}, lock, 5*time.Minute)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/update", h.Handle)

		before := time.Now()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?async=true&timeout=2m", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var observedCtx context.Context
		select {
		case observedCtx = <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("update function was not started")
		}

		deadline, ok := observedCtx.Deadline()
		require.True(t, ok)
		assert.WithinDuration(t, before.Add(2*time.Minute), deadline, 5*time.Second)

		require.Eventually(t, lock.TryLock, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("invalid timeout is ignored", func(t *testing.T) {
		lock := scheduling.NewUpdateLock()

		h := NewWithTimeout(testLogger(), func(ctx context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, lock, 5*time.Minute)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/update", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update?timeout=bogus", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestHandler_Handle_SyncScanFailed verifies that a synchronous update reports
// a failed run in its summary, and leaves the field out for a successful one.
func TestHandler_Handle_SyncScanFailed(t *testing.T) {
	tests := []struct {
		name   string
		metric *metrics.Metric
		// want is the scan_failed value, or nil when the field is absent.
		want any
	}{
		{name: "failed run", metric: &metrics.Metric{Scanned: 3, ScanFailed: true}, want: true},
		{name: "successful run", metric: &metrics.Metric{Scanned: 3, Updated: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()

			h := New(testLogger(), func(context.Context, []string, []string) *metrics.Metric { return tt.metric }, lock)
			app := fiber.New(fiber.Config{})
			app.Post("/v1/update", h.Handle)

			resp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil))
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var got struct {
				Summary map[string]any `json:"summary"`
			}

			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, tt.want, got.Summary["scan_failed"])
		})
	}
}
