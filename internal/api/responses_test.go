package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/internal/api"
	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/api/routes"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Tokens used by the response fixtures.
const (
	fixtureAPIToken    = "api-token"
	fixtureEventsToken = "events-token"
)

// timestampField matches a timestamp field and captures its value.
var timestampField = regexp.MustCompile(`"timestamp":"([^"]*)"`)

// Values substituted for response fields that change on every request, other
// than timestamps, which are validated before they are replaced.
var volatileFields = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`"duration":"[^"]*"`), `"duration":"<duration>"`},
	{regexp.MustCompile(`"duration_ms":-?\d+`), `"duration_ms":0`},
}

// Response headers that the snapshots treat specially.
var (
	// droppedHeaders are left out because they restate the body or the clock.
	droppedHeaders = []string{"Content-Length", "Date"}

	// maskedHeaders change on every request, so only their presence is recorded.
	maskedHeaders = []string{"X-Ratelimit-Remaining", "X-Ratelimit-Reset", "X-Request-Id"}

	// perResponseHeaders are always recorded with each response.
	perResponseHeaders = []string{"Content-Type"}
)

// Errors returned by the Docker client fixtures.
var (
	// errFixtureDocker is returned when the Docker daemon is unreachable.
	errFixtureDocker = errors.New("cannot connect to the Docker daemon")

	// errFixtureRegistry is returned when an image's registry cannot be queried.
	errFixtureRegistry = errors.New("registry returned 503 Service Unavailable")
)

// fixtureID returns a 64-character hexadecimal ID made of one repeated digit.
func fixtureID(digit string) string {
	return strings.Repeat(digit, 64)
}

// fixtureContainer builds a container from Docker inspect data. A container
// without an image ID has no image metadata, as when the image was removed.
func fixtureContainer(
	id, name, image, imageID string,
	running bool,
	labels map[string]string,
	repoDigests []string,
) types.Container {
	info := &dockerContainer.InspectResponse{
		ID:    fixtureID(id),
		Name:  name,
		Image: imageID,
		State: &dockerContainer.State{Running: running},
		Config: &dockerContainer.Config{
			Image:  image,
			Labels: labels,
		},
		HostConfig: &dockerContainer.HostConfig{},
	}

	var imageInfo *dockerImage.InspectResponse
	if imageID != "" {
		imageInfo = &dockerImage.InspectResponse{ID: imageID, RepoDigests: repoDigests}
	}

	return container.NewContainer(nil, info, imageInfo)
}

// fixtureContainers returns the containers the fixture Docker client lists.
//
// Two containers share one image so the images endpoint aggregates them. The
// worker image has digests from two repositories, the first of which does not
// match its image name. The cache container has no image metadata.
func fixtureContainers() []types.Container {
	nginxID := "sha256:" + fixtureID("a")
	nginxDigests := []string{"nginx@sha256:" + fixtureID("d")}

	return []types.Container{
		fixtureContainer("1", "/web", "nginx:1.27", nginxID, true, map[string]string{
			"com.centurylinklabs.watchtower.enable": "true",
			"com.centurylinklabs.watchtower.scope":  "prod",
		}, nginxDigests),
		fixtureContainer("2", "/web-2", "nginx:1.27", nginxID, true, map[string]string{
			"com.centurylinklabs.watchtower.scope": "prod",
		}, nginxDigests),
		fixtureContainer("3", "/worker", "ghcr.io/org/worker:latest", "sha256:"+fixtureID("b"), true,
			map[string]string{"com.centurylinklabs.watchtower.monitor-only": "true"},
			[]string{
				"mirror.example.com/org/worker@sha256:" + fixtureID("e"),
				"ghcr.io/org/worker@sha256:" + fixtureID("f"),
			}),
		fixtureContainer("4", "/cache", "redis:7", "", false, map[string]string{
			"com.centurylinklabs.watchtower.no-pull": "true",
		}, nil),
		fixtureContainer("5", "/watchtower", "nickfedor/watchtower:latest", "sha256:"+fixtureID("c"), true,
			map[string]string{"com.centurylinklabs.watchtower": "true"},
			[]string{"nickfedor/watchtower@sha256:" + fixtureID("9")}),
	}
}

// newFixtureClient returns a Docker client that lists the fixture containers and
// reports a newer image for the web containers and a registry error for cache.
func newFixtureClient(t *testing.T) *mockContainer.MockClient {
	t.Helper()

	list := fixtureContainers()

	client := mockContainer.NewMockClient(t)
	client.EXPECT().Ping(mock.Anything).Return(nil).Maybe()
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(list, nil).Maybe()
	client.EXPECT().ListContainers(mock.Anything).Return(list, nil).Maybe()
	client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, c types.Container, _ types.UpdateParams) (bool, types.ImageID, string, error) {
			switch c.Name() {
			case "web", "web-2":
				return true, types.ImageID("sha256:" + fixtureID("7")), "sha256:" + fixtureID("8"), nil
			case "cache":
				return false, "", "", errFixtureRegistry
			default:
				return false, c.ImageID(), "", nil
			}
		}).Maybe()

	return client
}

// newFailingClient returns a Docker client whose daemon is unreachable.
func newFailingClient(t *testing.T) *mockContainer.MockClient {
	t.Helper()

	client := mockContainer.NewMockClient(t)
	client.EXPECT().Ping(mock.Anything).Return(errFixtureDocker).Maybe()
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(nil, errFixtureDocker).Maybe()
	client.EXPECT().ListContainers(mock.Anything).Return(nil, errFixtureDocker).Maybe()

	return client
}

// newFixtureMetrics returns a metrics store on a private registry so scans
// recorded by one test are not visible to another.
func newFixtureMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()

	store, err := metrics.NewWithRegistry(prometheus.NewRegistry())
	require.NoError(t, err)
	t.Cleanup(store.Shutdown)

	return store
}

// fixtureOptions returns API options with every endpoint enabled, backed by the
// given Docker client and metrics store. Each update reports the same scan.
func fixtureOptions(client container.Client, store *metrics.Metrics) config.Options {
	return withTestLogger(config.Options{
		Token:               fixtureAPIToken,
		EventsToken:         fixtureEventsToken,
		RateLimit:           1000,
		EnableUpdateAPI:     true,
		EnableMetricsAPI:    true,
		EnableContainersAPI: true,
		EnableCheckAPI:      true,
		EnableHealthAPI:     true,
		EnableHistoryAPI:    true,
		EnableImagesAPI:     true,
		EnableConfigAPI:     true,
		EnableEventsAPI:     true,
		UnblockHTTPAPI:      true,
		NoStartupMessage:    true,
		Client:              client,
		Filter:              func(types.FilterableContainer) bool { return true },
		FilterDesc:          "Checking all containers (except explicitly disabled with label)",
		IncludeStopped:      true,
		UpdateLock:          newUpdateLock(),
		BaseParams: types.UpdateParams{
			Cleanup:        true,
			LifecycleHooks: true,
		},
		RunUpdatesWithNotifications: func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{Scanned: 5, Updated: 2, Failed: 1, Restarted: 0, Skipped: 1}
		},
		FilterByImage:    func(_ []string, filter types.Filter) types.Filter { return filter },
		DefaultMetrics:   func() *metrics.Metrics { return store },
		EventBroadcaster: events.NewBroadcaster(),
	})
}

// newUpdateLock returns an update lock holding its token, so an update can start.
func newUpdateLock() chan bool {
	lock := make(chan bool, 1)
	lock <- true

	return lock
}

// newFixtureApp builds the HTTP API from opts the same way SetupAndStartAPI
// does, without starting a listener.
func newFixtureApp(t *testing.T, opts config.Options) *fiber.App {
	t.Helper()

	app := api.New(opts.Logger, opts.RateLimit, api.ProxyConfig{}, api.CORSConfig{}, opts.NoStartupMessage)

	err := routes.ValidateAndRegister(t.Context(), app, api.NewAPIAuthMiddleware(opts.Logger, opts.Token), opts)
	require.NoError(t, err)

	return app
}

// normalizeVolatile replaces the values of response fields that change on
// every request with fixed placeholders. Each timestamp must be a valid
// RFC 3339 time, so a missing or malformed one fails the test instead of
// disappearing behind the placeholder.
func normalizeVolatile(t *testing.T, body []byte) []byte {
	t.Helper()

	for _, match := range timestampField.FindAllSubmatch(body, -1) {
		_, err := time.Parse(time.RFC3339Nano, string(match[1]))
		require.NoError(t, err, "invalid timestamp %q", match[1])
	}

	body = timestampField.ReplaceAll(body, []byte(`"timestamp":"<timestamp>"`))

	for _, field := range volatileFields {
		body = field.pattern.ReplaceAll(body, []byte(field.replacement))
	}

	return body
}

// responseRequest describes one request recorded in a response snapshot.
type responseRequest struct {
	method string
	target string
	// token is sent as a bearer token when set.
	token string
}

// snapshotHeaders returns the response headers in snapshot form, without
// dropped headers and with masked values replaced.
func snapshotHeaders(header http.Header) map[string]string {
	out := make(map[string]string, len(header))

	for name, values := range header {
		switch {
		case slices.Contains(droppedHeaders, name):
		case slices.Contains(maskedHeaders, name):
			out[name] = "<varies>"
		default:
			out[name] = strings.Join(values, ", ")
		}
	}

	return out
}

// responseRecorder records responses from one app. The headers of the first
// response, other than per-response ones, become the common headers, and each
// response lists only the headers that differ from them.
type responseRecorder struct {
	app    *fiber.App
	common map[string]string
	out    strings.Builder
}

// newResponseRecorder returns a recorder for app whose snapshot starts with
// the common headers, taken from the response to req.
func newResponseRecorder(t *testing.T, app *fiber.App, req responseRequest) *responseRecorder {
	t.Helper()

	resp := sendRequest(t, app, req)
	require.NoError(t, resp.Body.Close())

	recorder := &responseRecorder{app: app, common: snapshotHeaders(resp.Header)}
	for _, name := range perResponseHeaders {
		delete(recorder.common, name)
	}

	recorder.out.WriteString("=== common headers\n")

	for _, name := range slices.Sorted(maps.Keys(recorder.common)) {
		fmt.Fprintf(&recorder.out, "header: %s: %s\n", name, recorder.common[name])
	}

	return recorder
}

// sendRequest sends req to app and returns the response.
func sendRequest(t *testing.T, app *fiber.App, req responseRequest) *http.Response {
	t.Helper()

	httpReq := httptest.NewRequestWithContext(t.Context(), req.method, req.target, nil)
	if req.token != "" {
		httpReq.Header.Set(fiber.HeaderAuthorization, "Bearer "+req.token)
	}

	resp, err := app.Test(httpReq, fiber.TestConfig{Timeout: probeTimeout, FailOnTimeout: true})
	require.NoError(t, err, "%s %s", req.method, req.target)

	return resp
}

// record sends each request in order and appends its status, headers, and
// body to the snapshot. JSON bodies are indented with their key order preserved.
func (r *responseRecorder) record(t *testing.T, reqs ...responseRequest) {
	t.Helper()

	for _, req := range reqs {
		r.recordOne(t, req)
	}
}

// snapshot returns the recorded snapshot.
func (r *responseRecorder) snapshot() []byte {
	return []byte(r.out.String())
}

// recordOne records the response to a single request.
func (r *responseRecorder) recordOne(t *testing.T, req responseRequest) {
	t.Helper()

	resp := sendRequest(t, r.app, req)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	out := &r.out

	fmt.Fprintf(out, "\n=== %s %s", req.method, req.target)

	switch req.token {
	case fixtureAPIToken:
	case "":
		out.WriteString(" (no token)")
	default:
		fmt.Fprintf(out, " (token: %s)", req.token)
	}

	fmt.Fprintf(out, "\nstatus: %d\n", resp.StatusCode)

	headers := snapshotHeaders(resp.Header)

	for _, name := range slices.Sorted(maps.Keys(headers)) {
		if value, ok := r.common[name]; !ok || value != headers[name] {
			fmt.Fprintf(out, "header: %s: %s\n", name, headers[name])
		}
	}

	for _, name := range slices.Sorted(maps.Keys(r.common)) {
		if _, ok := headers[name]; !ok {
			fmt.Fprintf(out, "missing header: %s\n", name)
		}
	}

	body = normalizeVolatile(t, body)

	if strings.HasPrefix(resp.Header.Get(fiber.HeaderContentType), fiber.MIMEApplicationJSON) {
		var indented bytes.Buffer
		require.NoError(t, json.Indent(&indented, body, "", "  "))

		body = indented.Bytes()
	}

	if len(body) > 0 {
		out.WriteString("body:\n")
		out.Write(body)
		out.WriteString("\n")
	}
}

// TestHTTPAPIResponses snapshots the response of every HTTP API route for a
// fixed set of containers, including the status and history before and after
// an update. Requests run in order because later ones observe earlier updates.
func TestHTTPAPIResponses(t *testing.T) {
	store := newFixtureMetrics(t)
	opts := fixtureOptions(newFixtureClient(t), store)
	recorder := newResponseRecorder(t, newFixtureApp(t, opts), responseRequest{method: http.MethodGet, target: "/livez"})

	recorder.record(t,
		responseRequest{method: http.MethodGet, target: "/livez"},
		responseRequest{method: http.MethodGet, target: "/readyz"},
		responseRequest{method: http.MethodGet, target: "/startupz"},
		responseRequest{method: http.MethodGet, target: "/v1/config", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers?name=worker", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers?name=missing", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers/details", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers/details?image=redis:7", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/images", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/images?name=missing", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/status", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/history", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/update", token: fixtureAPIToken},
	)

	// The update records its scan asynchronously.
	require.Eventually(t, func() bool { return store.GetLastScan() != nil }, probeTimeout, 10*time.Millisecond)

	recorder.record(t,
		responseRequest{method: http.MethodGet, target: "/v1/status", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/history", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/check", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/check?container=web", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/update?async=true", token: fixtureAPIToken},
	)

	// The asynchronous update returns its lock when it finishes.
	require.Eventually(t, func() bool { return len(opts.UpdateLock) == 1 }, probeTimeout, 10*time.Millisecond)

	golden.Assert(t, "http-api-responses", recorder.snapshot())
}

// TestHTTPAPIErrorResponses snapshots the responses for rejected requests,
// invalid parameters, a busy update lock, and an unreachable Docker daemon.
func TestHTTPAPIErrorResponses(t *testing.T) {
	app := newFixtureApp(t, fixtureOptions(newFixtureClient(t), newFixtureMetrics(t)))
	recorder := newResponseRecorder(t, app, responseRequest{method: http.MethodGet, target: "/livez"})

	recorder.record(t,
		responseRequest{method: http.MethodGet, target: "/v1/containers"},
		responseRequest{method: http.MethodGet, target: "/v1/containers", token: "wrong-token"},
		responseRequest{method: http.MethodGet, target: "/v1/events"},
		responseRequest{method: http.MethodGet, target: "/v1/events", token: "wrong-token"},
		responseRequest{method: http.MethodGet, target: "/v1/history?since=yesterday", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/history?until=", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/history?limit=-1", token: fixtureAPIToken},
		responseRequest{
			method: http.MethodGet,
			target: "/v1/history?since=2026-10-08T00:00:00Z&until=2026-10-07T00:00:00Z",
			token:  fixtureAPIToken,
		},
		responseRequest{method: http.MethodGet, target: "/v1/unknown", token: fixtureAPIToken},
	)

	busyOpts := fixtureOptions(newFixtureClient(t), newFixtureMetrics(t))
	busyOpts.UpdateLock = make(chan bool, 1)

	recorder.app = newFixtureApp(t, busyOpts)
	recorder.record(t, responseRequest{method: http.MethodPost, target: "/v1/update", token: fixtureAPIToken})

	failingOpts := fixtureOptions(newFailingClient(t), newFixtureMetrics(t))
	failingOpts.RunUpdatesWithNotifications = func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
		return nil
	}

	recorder.app = newFixtureApp(t, failingOpts)
	recorder.record(t,
		responseRequest{method: http.MethodGet, target: "/readyz"},
		responseRequest{method: http.MethodGet, target: "/v1/containers", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/containers/details", token: fixtureAPIToken},
		responseRequest{method: http.MethodGet, target: "/v1/images", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/check", token: fixtureAPIToken},
		responseRequest{method: http.MethodPost, target: "/v1/update", token: fixtureAPIToken},
	)

	golden.Assert(t, "http-api-error-responses", recorder.snapshot())
}

// TestHTTPAPIMetricsResponse snapshots the Prometheus metric families the
// metrics endpoint exposes for Watchtower. Values depend on earlier scans in
// the process, so only the HELP and TYPE lines are recorded.
func TestHTTPAPIMetricsResponse(t *testing.T) {
	app := newFixtureApp(t, fixtureOptions(newFixtureClient(t), newFixtureMetrics(t)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/metrics", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+fixtureAPIToken)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: probeTimeout, FailOnTimeout: true})
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var lines []string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# HELP watchtower_") || strings.HasPrefix(line, "# TYPE watchtower_") {
			lines = append(lines, line)
		}
	}

	require.NoError(t, scanner.Err())

	snapshot := "header: Content-Type: " + resp.Header.Get(fiber.HeaderContentType) + "\n" +
		strings.Join(lines, "\n") + "\n"

	golden.Assert(t, "http-api-metrics", []byte(snapshot))
}

// readSSEEvents reads the server-sent event stream until count events have
// arrived and returns the raw stream text.
func readSSEEvents(t *testing.T, stream *bufio.Reader, count int) string {
	t.Helper()

	var out strings.Builder

	for received := 0; received < count; {
		line, err := stream.ReadString('\n')
		require.NoError(t, err, "stream ended after %d of %d events", received, count)

		out.WriteString(line)

		if strings.HasPrefix(line, "event:") {
			received++
		}
	}

	// Include the data and blank line that complete the last event.
	for {
		line, err := stream.ReadString('\n')
		require.NoError(t, err)

		out.WriteString(line)

		if line == "\n" {
			return out.String()
		}
	}
}

// TestHTTPAPIEventStream snapshots the server-sent event stream: the events a
// check request publishes, followed by one event of each type an update
// session publishes. The stream is served on a local listener because it
// stays open until the client disconnects.
func TestHTTPAPIEventStream(t *testing.T) {
	opts := fixtureOptions(newFixtureClient(t), newFixtureMetrics(t))
	app := newFixtureApp(t, opts)

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		_ = app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	t.Cleanup(func() { _ = app.Shutdown() })

	baseURL := "http://" + listener.Addr().String()

	ctx, cancel := context.WithTimeout(t.Context(), probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/events", nil)
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+fixtureEventsToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Eventually(t, func() bool { return opts.EventBroadcaster.SubscriberCount() == 1 },
		probeTimeout, 10*time.Millisecond)

	stream := bufio.NewReader(resp.Body)

	checkReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/check", nil)
	require.NoError(t, err)
	checkReq.Header.Set(fiber.HeaderAuthorization, "Bearer "+fixtureAPIToken)

	checkResp, err := http.DefaultClient.Do(checkReq)
	require.NoError(t, err)
	require.NoError(t, checkResp.Body.Close())
	require.Equal(t, http.StatusOK, checkResp.StatusCode)

	checkEvents := readSSEEvents(t, stream, 2)

	session := []events.Event{
		{Type: "scan_started", Data: events.NewScanStartedData(types.UpdateParams{
			Cleanup:             true,
			LifecycleHooks:      true,
			RunOnce:             true,
			UseComposeDependsOn: true,
		})},
		{Type: "scan_failed", Data: events.ScanFailedData{Error: "failed to list containers"}},
		{Type: "image_cleanup", Data: events.ImageCleanupData{Images: []events.ImageCleanupEntry{{
			ImageID:       "sha256:" + fixtureID("a"),
			ImageName:     "nginx:1.26",
			ContainerID:   fixtureID("1"),
			ContainerName: "web",
		}}}},
		{Type: "scan_completed", Data: events.ScanCompletedData{Scanned: 5, Updated: 2, Failed: 1, Skipped: 1}},
	}

	for _, event := range session {
		event.Timestamp = time.Now().UTC()
		opts.EventBroadcaster.Publish(event)
	}

	sessionEvents := readSSEEvents(t, stream, len(session))

	// The server notices the disconnect only when a write fails, so publish
	// until it drops the subscriber and the listener can shut down promptly.
	require.NoError(t, resp.Body.Close())
	require.Eventually(t, func() bool {
		opts.EventBroadcaster.Publish(events.Event{Type: "disconnect_probe"})

		return opts.EventBroadcaster.SubscriberCount() == 0
	}, probeTimeout, 10*time.Millisecond)

	snapshot := "header: Content-Type: " + resp.Header.Get(fiber.HeaderContentType) + "\n" +
		"--- check request\n" + checkEvents + "--- update session\n" + sessionEvents

	golden.Assert(t, "http-api-events", normalizeVolatile(t, []byte(snapshot)))
}
