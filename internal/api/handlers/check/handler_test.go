package check

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		check CheckFunc
	}{
		{
			name:  "with check function",
			check: func(_ context.Context, _, _ []string) ([]ContainerCheck, error) { return nil, nil },
		},
		{
			name:  "with nil check function",
			check: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(testLogger(), tt.check, 5*time.Minute, nil, false, nil, events.ScanStartedData{})
			require.NotNil(t, h)
			assert.Equal(t, "/v1/check", h.Path)
		})
	}
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		checkFunc  CheckFunc
		wantStatus int
	}{
		{
			name: "successful check returns 200",
			checkFunc: func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return []ContainerCheck{
					{Name: "container1", Image: "nginx:latest", ImageID: "sha256:abc", UpdateAvailable: true},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty results returns 200",
			checkFunc: func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return []ContainerCheck{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "check error returns 500",
			checkFunc: func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return nil, errors.New("docker error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(testLogger(), tt.checkFunc, 5*time.Minute, nil, false, nil, events.ScanStartedData{})
			app := fiber.New(fiber.Config{})
			app.Post("/v1/check", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestHandler_Handle_WithFilters(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantImages []string
		wantNames  []string
	}{
		{
			name:       "with image filter",
			query:      "?image=nginx",
			wantStatus: http.StatusOK,
			wantImages: []string{"nginx"},
		},
		{
			name:       "with name filter",
			query:      "?container=my-container",
			wantStatus: http.StatusOK,
			wantNames:  []string{"my-container"},
		},
		{
			name:       "with multiple filters",
			query:      "?image=nginx&container=my-container",
			wantStatus: http.StatusOK,
			wantImages: []string{"nginx"},
			wantNames:  []string{"my-container"},
		},
		{
			name:       "with comma-separated filters",
			query:      "?container=container1,container2",
			wantStatus: http.StatusOK,
			wantNames:  []string{"container1", "container2"},
		},
		{
			name:       "with comma-separated image filter",
			query:      "?image=nginx,redis",
			wantStatus: http.StatusOK,
			wantImages: []string{"nginx", "redis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedImages, capturedNames []string

			h := New(testLogger(), func(ctx context.Context, images, names []string) ([]ContainerCheck, error) {
				capturedImages = images
				capturedNames = names

				return []ContainerCheck{}, nil
			}, 5*time.Minute, nil, false, nil, events.ScanStartedData{})
			app := fiber.New(fiber.Config{})
			app.Post("/v1/check", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check"+tt.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantImages != nil {
				assert.Equal(t, tt.wantImages, capturedImages)
			}

			if tt.wantNames != nil {
				assert.Equal(t, tt.wantNames, capturedNames)
			}
		})
	}
}

func TestHandler_Handle_TimeoutOverride(t *testing.T) {
	t.Run("valid timeout is applied", func(t *testing.T) {
		h := New(testLogger(), func(ctx context.Context, _, _ []string) ([]ContainerCheck, error) {
			return []ContainerCheck{}, nil
		}, 5*time.Minute, nil, false, nil, events.ScanStartedData{})
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check?timeout=2m", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("timeout exceeding max is clamped", func(t *testing.T) {
		h := New(testLogger(), func(ctx context.Context, _, _ []string) ([]ContainerCheck, error) {
			return []ContainerCheck{}, nil
		}, 2*time.Minute, nil, false, nil, events.ScanStartedData{})
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check?timeout=5m", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid timeout is ignored", func(t *testing.T) {
		h := New(testLogger(), func(ctx context.Context, _, _ []string) ([]ContainerCheck, error) {
			return []ContainerCheck{}, nil
		}, 5*time.Minute, nil, false, nil, events.ScanStartedData{})
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check?timeout=bogus", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_Handle_EmitsEvents(t *testing.T) {
	scanStartedData := events.ScanStartedData{
		Cleanup:             true,
		NoRestart:           false,
		MonitorOnly:         false,
		LifecycleHooks:      true,
		RollingRestart:      false,
		LabelPrecedence:     false,
		NoPull:              false,
		RunOnce:             false,
		UseComposeDependsOn: false,
		SkipSelfUpdate:      false,
		EphemeralSelfUpdate: false,
		ReviveStopped:       false,
	}

	t.Run("success emits scan_started and scan_completed", func(t *testing.T) {
		b := events.NewBroadcaster()
		ch, _ := b.SubscribeWithDone()
		require.NotNil(t, ch)

		h := New(testLogger(),
			func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return []ContainerCheck{
					{Name: "c1", Image: "nginx:latest", ImageID: "sha256:abc", UpdateAvailable: true},
					{Name: "c2", Image: "redis:latest", ImageID: "sha256:def", UpdateAvailable: false, Error: "registry error"},
				}, nil
			},
			5*time.Minute, nil, false, b, scanStartedData,
		)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var started, completed bool

		for started == false || completed == false {
			select {
			case evt := <-ch:
				switch evt.Type {
				case "scan_started":
					started = true
					data, ok := evt.Data.(events.ScanStartedData)
					require.True(t, ok)
					assert.True(t, data.Cleanup)
				case "scan_completed":
					completed = true
					data, ok := evt.Data.(events.ScanCompletedData)
					require.True(t, ok)
					assert.Equal(t, 2, data.Scanned)
					assert.Equal(t, 0, data.Updated)
					assert.Equal(t, 1, data.Failed)
				case "scan_failed":
					t.Fatal("unexpected scan_failed event on success")
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timeout waiting for events: started=%v completed=%v", started, completed)
			}
		}
	})

	t.Run("check error emits scan_started and scan_failed", func(t *testing.T) {
		b := events.NewBroadcaster()
		ch, _ := b.SubscribeWithDone()
		require.NotNil(t, ch)

		h := New(testLogger(),
			func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return nil, errors.New("docker api error")
			},
			5*time.Minute, nil, false, b, scanStartedData,
		)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var started, failed bool

		for started == false || failed == false {
			select {
			case evt := <-ch:
				switch evt.Type {
				case "scan_started":
					started = true
				case "scan_failed":
					failed = true
					data, ok := evt.Data.(events.ScanFailedData)
					require.True(t, ok)
					assert.Equal(t, "failed to check for updates", data.Error)
				case "scan_completed":
					t.Fatal("unexpected scan_completed event on error")
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timeout waiting for events: started=%v failed=%v", started, failed)
			}
		}
	})

	t.Run("nil broadcaster emits no events", func(t *testing.T) {
		h := New(testLogger(),
			func(_ context.Context, _, _ []string) ([]ContainerCheck, error) {
				return []ContainerCheck{{Name: "c1"}}, nil
			},
			5*time.Minute, nil, false, nil, events.ScanStartedData{},
		)
		app := fiber.New(fiber.Config{})
		app.Post("/v1/check", h.Handle)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/check", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
