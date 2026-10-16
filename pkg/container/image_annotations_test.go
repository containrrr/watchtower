package container

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moby/moby/api/types/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
)

// inspectServer is a minimal Docker API stand-in serving one image inspect.
//
// It avoids the gomega-backed ghttp helpers, which are only usable inside the
// ginkgo suite, so these cases can run as plain Go tests.
type inspectServer struct {
	*httptest.Server

	// inspects counts image inspect requests, so a test can assert that a
	// short-circuit never reached the API.
	inspects atomic.Int32
}

// newInspectServer serves imageConfig for any image inspect request. A nil
// config models an image the daemon reports without an OCI config.
func newInspectServer(t *testing.T, imageConfig *dockerspec.DockerOCIImageConfig) *inspectServer {
	t.Helper()

	srv := &inspectServer{}
	srv.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/info"):
			writeJSON(t, w, http.StatusOK, map[string]any{})
		case strings.Contains(r.URL.Path, "/images/") && strings.HasSuffix(r.URL.Path, "/json"):
			srv.inspects.Add(1)
			writeJSON(t, w, http.StatusOK, image.InspectResponse{Config: imageConfig})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// newNotFoundInspectServer serves a failing image inspect.
func newNotFoundInspectServer(t *testing.T) *inspectServer {
	t.Helper()

	srv := &inspectServer{}
	srv.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/images/"):
			srv.inspects.Add(1)
			writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "No such image"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// writeJSON writes a JSON body with the given status.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

// newInspectClient returns a Docker API client pointed at srv.
func newInspectClient(t *testing.T, srv *inspectServer) *dockerClient.Client {
	t.Helper()

	api, err := dockerClient.New(
		dockerClient.WithHost(srv.URL),
	)
	require.NoError(t, err)

	return api
}

func TestImageClientImageAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("reads every OCI annotation from image config labels", func(t *testing.T) {
		t.Parallel()

		srv := newInspectServer(t, &dockerspec.DockerOCIImageConfig{
			Labels: map[string]string{
				oci.SourceLabel:        "https://github.com/org/app.git",
				oci.URLLabel:           "https://example.com/image",
				oci.DocumentationLabel: "https://example.com/docs",
				oci.RevisionLabel:      "abc123",
				oci.VersionLabel:       "1.2.3",
			},
		})

		got := newImageClient(newInspectClient(t, srv), testLog(), registryConfig.Registry{}).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, "https://github.com/org/app.git", got.Source)
		assert.Equal(t, "https://example.com/image", got.URL)
		assert.Equal(t, "https://example.com/docs", got.Documentation)
		assert.Equal(t, "abc123", got.Revision)
		assert.Equal(t, "1.2.3", got.Version)
	})

	t.Run("an image without OCI labels yields empty annotations", func(t *testing.T) {
		t.Parallel()

		srv := newInspectServer(t, &dockerspec.DockerOCIImageConfig{
			Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
		})

		got := newImageClient(newInspectClient(t, srv), testLog(), registryConfig.Registry{}).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, oci.Annotations{}, got)
	})

	t.Run("nil image config yields empty annotations", func(t *testing.T) {
		t.Parallel()

		srv := newInspectServer(t, nil)

		got := newImageClient(newInspectClient(t, srv), testLog(), registryConfig.Registry{}).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, oci.Annotations{}, got)
	})

	t.Run("an empty image reference never reaches the API", func(t *testing.T) {
		t.Parallel()

		srv := newInspectServer(t, &dockerspec.DockerOCIImageConfig{
			Labels: map[string]string{oci.VersionLabel: "1.2.3"},
		})

		got := newImageClient(newInspectClient(t, srv), testLog(), registryConfig.Registry{}).
			GetImageAnnotations(t.Context(), "")

		assert.Equal(t, oci.Annotations{}, got)
		assert.Zero(t, srv.inspects.Load(), "the short-circuit must skip the inspect")
	})

	t.Run("a failed inspect yields empty annotations rather than an error", func(t *testing.T) {
		t.Parallel()

		srv := newNotFoundInspectServer(t)

		got := newImageClient(newInspectClient(t, srv), testLog(), registryConfig.Registry{}).
			GetImageAnnotations(t.Context(), "missing:latest")

		assert.Equal(t, oci.Annotations{}, got)
		assert.Equal(t, int32(1), srv.inspects.Load())
	})
}

// The client-level delegate must reach the same result as the image client.
func TestClientImageAnnotationsDelegates(t *testing.T) {
	t.Parallel()

	srv := newInspectServer(t, &dockerspec.DockerOCIImageConfig{
		Labels: map[string]string{oci.VersionLabel: "1.2.3"},
	})

	got := (&client{api: newInspectClient(t, srv), log: testLog()}).
		GetImageAnnotations(t.Context(), "app:latest")

	assert.Equal(t, "1.2.3", got.Version)
}
