package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
)

// FuzzExtractImagesHandler fuzzes the image-name parsing logic through the
// Handler struct with arbitrary query parameter values.
func FuzzExtractImagesHandler(f *testing.F) {
	f.Add("nginx:latest")
	f.Add("nginx:latest,redis:7")
	f.Add("nginx:latest,redis:7,postgres:16")
	f.Add("")
	f.Add(",,,")
	f.Add("a")
	f.Add(strings.Repeat("x", 1000))
	f.Add("image with spaces")
	f.Add("registry.com/org/image:v1.2.3")

	f.Fuzz(func(t *testing.T, raw string) {
		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, nil, testMaxTimeout)

		app := fiber.New(fiber.Config{})
		app.Post("/test", func(c fiber.Ctx) error {
			images := h.extractImages(c)

			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				if len(images) != 0 {
					t.Errorf("whitespace-only input %q produced %d images, want 0", raw, len(images))
				}

				return nil
			}

			if !strings.Contains(trimmed, ",") && len(images) != 1 {
				t.Errorf("non-empty input %q produced %d images, want 1", raw, len(images))
			}

			return nil
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test?image="+url.QueryEscape(raw), nil)

		resp, err := app.Test(req)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("request failed: %v", err)
		}

		if resp != nil {
			defer resp.Body.Close()
		}
	})
}

// FuzzExtractContainersHandler fuzzes the container-name parsing logic through
// the Handler struct with arbitrary query parameter values.
func FuzzExtractContainersHandler(f *testing.F) {
	f.Add("web,api,db")
	f.Add("my-container")
	f.Add("")
	f.Add("web,,api")
	f.Add(",web,")
	f.Add("container-with-dashes")
	f.Add(strings.Repeat("a,", 50))

	f.Fuzz(func(t *testing.T, raw string) {
		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, nil, testMaxTimeout)

		app := fiber.New(fiber.Config{})
		app.Post("/test", func(c fiber.Ctx) error {
			containers := h.extractContainers(c)

			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				if len(containers) != 0 {
					t.Errorf("whitespace-only input %q produced %d containers, want 0", raw, len(containers))
				}

				return nil
			}

			if !strings.Contains(trimmed, ",") && len(containers) != 1 {
				t.Errorf("non-empty input %q produced %d containers, want 1", raw, len(containers))
			}

			return nil
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test?container="+url.QueryEscape(raw), nil)

		resp, err := app.Test(req)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("request failed: %v", err)
		}

		if resp != nil {
			defer resp.Body.Close()
		}
	})
}

// FuzzHandlerNew fuzzes the Handler constructor to ensure it never panics,
// with or without an update lock.
func FuzzHandlerNew(f *testing.F) {
	f.Add(true)
	f.Add(false)

	f.Fuzz(func(t *testing.T, withLock bool) {
		var lock *scheduling.UpdateLock
		if withLock {
			lock = scheduling.NewUpdateLock()
		}

		h := New(testLogger(), func(_ context.Context, _, _ []string) *metrics.Metric {
			return &metrics.Metric{}
		}, lock, testMaxTimeout)

		require.NotNil(t, h)
		assert.Equal(t, "/v1/update", h.Path)
	})
}
