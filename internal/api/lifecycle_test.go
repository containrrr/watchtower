package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func makeListContainersMock(t *testing.T) *mockContainer.MockClient {
	t.Helper()
	mc := mockContainer.NewMockClient(t)
	mc.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil).Maybe()
	mc.EXPECT().ListContainers(mock.Anything).Return([]types.Container{}, nil).Maybe()

	return mc
}

func makeFilter(_ *testing.T) types.Filter {
	return func(_ types.FilterableContainer) bool { return true }
}

// withTestLogger injects a discarded logger when opts.Logger is nil.
func withTestLogger(opts config.Options) config.Options {
	if opts.Logger == nil {
		opts.Logger = testLogger()
	}

	return opts
}

// withTestListenAddr sets Host/Port so tests bind an ephemeral local port.
func withTestListenAddr(opts config.Options) config.Options {
	opts = withTestLogger(opts)

	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}

	if opts.Port == "" {
		opts.Port = "0"
	}

	return opts
}

func runServerAndShutdown(t *testing.T, opts config.Options) {
	t.Helper()

	opts = withTestListenAddr(opts)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- SetupAndStartAPI(ctx, opts)
	}()

	// Wait for SetupAndStartAPI to return (non-blocking modes) or give the
	// server time to bind (blocking modes still run in this goroutine path).
	select {
	case err := <-errCh:
		assert.True(t, err == nil || errors.Is(err, context.Canceled),
			"unexpected error: %v", err)
		// Non-blocking: server is still running under GracefulContext; cancel it.
		cancel()
		time.Sleep(50 * time.Millisecond)

		return
	case <-time.After(500 * time.Millisecond):
		// Blocking mode: still inside SetupAndStartAPI.
	}

	cancel()

	select {
	case err := <-errCh:
		assert.True(t, err == nil || errors.Is(err, context.Canceled),
			"unexpected error: %v", err)
	case <-time.After(shutdownGracePeriod + 2*time.Second):
		t.Fatal("server did not shut down within expected time")
	}
}

func TestSetupAndStartAPI(t *testing.T) {
	testMetrics := metrics.Default()

	tests := []struct {
		name    string
		opts    config.Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "no APIs enabled",
			opts: config.Options{
				Token: "test-token",
			},
			wantErr: false,
		},
		{
			name: "nil RunUpdatesWithNotifications",
			opts: config.Options{
				Token:                       "test-token",
				EnableUpdateAPI:             true,
				UpdateLock:                  scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: nil,
				FilterByImage:               func(_ []string, f types.Filter) types.Filter { return f },
				DefaultMetrics:              func() *metrics.Metrics { return testMetrics },
			},
			wantErr: true,
			errMsg:  "RunUpdatesWithNotifications must be provided",
		},
		{
			name: "nil FilterByImage",
			opts: config.Options{
				Token:           "test-token",
				EnableUpdateAPI: true,
				UpdateLock:      scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
					return &metrics.Metric{}
				},
				FilterByImage:  nil,
				DefaultMetrics: func() *metrics.Metrics { return testMetrics },
			},
			wantErr: true,
			errMsg:  "FilterByImage must be provided",
		},
		{
			name: "nil DefaultMetrics",
			opts: config.Options{
				Token:           "test-token",
				EnableUpdateAPI: true,
				UpdateLock:      scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
					return &metrics.Metric{}
				},
				FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
				DefaultMetrics: nil,
			},
			wantErr: true,
			errMsg:  "DefaultMetrics must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if tt.opts.Token == "" {
				t.Skip("empty token not allowed for token-required APIs")
			}

			err := SetupAndStartAPI(ctx, withTestLogger(tt.opts))

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupAndStartAPI_FullAPILifecycle(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:       "test-token",
		EventsToken: "events-token",
		RateLimit:   60,
		Client:      makeListContainersMock(t),
		Filter:      makeFilter(t),
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:       func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics:      func() *metrics.Metrics { return testMetrics },
		EventBroadcaster:    events.NewBroadcaster(),
		UnblockHTTPAPI:      true,
		EnableUpdateAPI:     true,
		UpdateLock:          scheduling.NewUpdateLock(),
		EnableMetricsAPI:    true,
		EnableContainersAPI: true,
		EnableCheckAPI:      true,
		EnableHealthAPI:     true,
		EnableHistoryAPI:    true,
		EnableImagesAPI:     true,
		EnableConfigAPI:     true,
		EnableEventsAPI:     true,
		EnableSwaggerAPI:    true,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- SetupAndStartAPI(ctx, opts)
	}()

	// UnblockHTTPAPI=true → non-blocking; should return after bind.
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("non-blocking full API setup did not return")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestSetupAndStartAPI_NoAPIs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := SetupAndStartAPI(ctx, config.Options{Token: "test"})
	assert.NoError(t, err)
}

func TestSetupAndStartAPI_MetricsOnly(t *testing.T) {
	testMetrics := metrics.Default()

	opts := config.Options{
		Token:            "test-token",
		EnableMetricsAPI: true,
		RateLimit:        60,
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
	}

	runServerAndShutdown(t, opts)
}

func TestSetupAndStartAPI_ContainersOnly(t *testing.T) {
	opts := config.Options{
		Token:               "test-token",
		EnableContainersAPI: true,
		RateLimit:           60,
		Client:              makeListContainersMock(t),
		Filter:              makeFilter(t),
	}

	runServerAndShutdown(t, opts)
}

func TestSetupAndStartAPI_CheckOnly(t *testing.T) {
	testMetrics := metrics.Default()

	opts := config.Options{
		Token:          "test-token",
		EnableCheckAPI: true,
		RateLimit:      60,
		Client:         makeListContainersMock(t),
		Filter:         makeFilter(t),
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
	}

	runServerAndShutdown(t, opts)
}

func TestSetupAndStartAPI_AllAPIs(t *testing.T) {
	testMetrics := metrics.Default()

	opts := config.Options{
		Token:               "test-token",
		EventsToken:         "events-token",
		EnableUpdateAPI:     true,
		UpdateLock:          scheduling.NewUpdateLock(),
		EnableMetricsAPI:    true,
		EnableContainersAPI: true,
		EnableCheckAPI:      true,
		EnableHistoryAPI:    true,
		EnableImagesAPI:     true,
		EnableConfigAPI:     true,
		EnableEventsAPI:     true,
		RateLimit:           60,
		Client:              makeListContainersMock(t),
		Filter:              makeFilter(t),
		EventBroadcaster:    events.NewBroadcaster(),
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		UnblockHTTPAPI: true,
	}

	runServerAndShutdown(t, opts)
}

func TestIntegration_HealthLiveness_And_Startup(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:            "test-token",
		EnableMetricsAPI: true,
		RateLimit:        60,
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
	})

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)

	go func() {
		errCh <- SetupAndStartAPI(ctx, opts)
	}()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("non-blocking setup did not return")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRegisterUpdateRoute_UnblockHTTPAPI(t *testing.T) {
	var startupCalled atomic.Int32

	opts := withTestListenAddr(config.Options{
		Token:           "test-token",
		EnableUpdateAPI: true,
		UpdateLock:      scheduling.NewUpdateLock(),
		UnblockHTTPAPI:  false,
		RateLimit:       60,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage: func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics {
			return &metrics.Metrics{}
		},
		WriteStartupMessage: func(_ logging.StartupParams) {
			startupCalled.Add(1)
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)

	go func() {
		errCh <- SetupAndStartAPI(ctx, opts)
	}()

	// Blocking mode: wait for bind + startup message without expecting return.
	require.Eventually(t, func() bool {
		return startupCalled.Load() > 0
	}, 2*time.Second, 20*time.Millisecond,
		"WriteStartupMessage should be called when UnblockHTTPAPI is false")

	cancel()

	select {
	case err := <-errCh:
		assert.True(t, err == nil || errors.Is(err, context.Canceled),
			"unexpected error: %v", err)
	case <-time.After(shutdownGracePeriod + 2*time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestSetupAndStartAPI_MissingUpdateDependencies(t *testing.T) {
	testMetrics := metrics.Default()

	tests := []struct {
		name           string
		opts           config.Options
		wantErr        string
		leaveLoggerNil bool
	}{
		{
			name: "missing RunUpdatesWithNotifications",
			opts: config.Options{
				Token:                       "test-token",
				EnableUpdateAPI:             true,
				UpdateLock:                  scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: nil,
				FilterByImage:               func(_ []string, f types.Filter) types.Filter { return f },
				DefaultMetrics:              func() *metrics.Metrics { return testMetrics },
			},
			wantErr: "RunUpdatesWithNotifications must be provided",
		},
		{
			name: "missing FilterByImage",
			opts: config.Options{
				Token:           "test-token",
				EnableUpdateAPI: true,
				UpdateLock:      scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
					return &metrics.Metric{}
				},
				FilterByImage:  nil,
				DefaultMetrics: func() *metrics.Metrics { return testMetrics },
			},
			wantErr: "FilterByImage must be provided",
		},
		{
			name: "missing DefaultMetrics",
			opts: config.Options{
				Token:           "test-token",
				EnableUpdateAPI: true,
				UpdateLock:      scheduling.NewUpdateLock(),
				RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
					return &metrics.Metric{}
				},
				FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
				DefaultMetrics: nil,
			},
			wantErr: "DefaultMetrics must be provided",
		},
		{
			name: "missing TLS key",
			opts: config.Options{
				Token:            "test-token",
				EnableMetricsAPI: true,
				TLSCertPath:      "/path/to/cert.pem",
				TLSKeyPath:       "",
				DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
			},
			wantErr: "TLS requires both",
		},
		{
			name: "missing TLS cert",
			opts: config.Options{
				Token:            "test-token",
				EnableMetricsAPI: true,
				TLSCertPath:      "",
				TLSKeyPath:       "/path/to/key.pem",
				DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
			},
			wantErr: "TLS requires both",
		},
		{
			name: "missing Logger",
			opts: config.Options{
				Token:            "test-token",
				EnableMetricsAPI: true,
				DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
			},
			wantErr:        "API Logger must be provided",
			leaveLoggerNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			opts := tt.opts
			if !tt.leaveLoggerNil {
				opts = withTestLogger(opts)
			}

			err := SetupAndStartAPI(ctx, opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSetupAndStartAPI_AllEndpointsEnableAll(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:            "test-token",
		EventsToken:      "events-token",
		RateLimit:        60,
		Client:           makeListContainersMock(t),
		Filter:           makeFilter(t),
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
		FilterByImage:    func(_ []string, f types.Filter) types.Filter { return f },
		EventBroadcaster: events.NewBroadcaster(),
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		UnblockHTTPAPI:      true,
		EnableUpdateAPI:     true,
		UpdateLock:          scheduling.NewUpdateLock(),
		EnableMetricsAPI:    true,
		EnableContainersAPI: true,
		EnableCheckAPI:      true,
		EnableHealthAPI:     true,
		EnableHistoryAPI:    true,
		EnableImagesAPI:     true,
		EnableConfigAPI:     true,
		EnableEventsAPI:     true,
		EnableSwaggerAPI:    true,
	})

	require.True(t, opts.EnableUpdateAPI)
	require.True(t, opts.EnableMetricsAPI)
	require.True(t, opts.EnableContainersAPI)
	require.True(t, opts.EnableCheckAPI)
	require.True(t, opts.EnableHealthAPI)
	require.True(t, opts.EnableHistoryAPI)
	require.True(t, opts.EnableImagesAPI)
	require.True(t, opts.EnableConfigAPI)
	require.True(t, opts.EnableEventsAPI)
	require.True(t, opts.EnableSwaggerAPI)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- SetupAndStartAPI(ctx, opts)
	}()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("non-blocking full API setup did not return")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRunServer_NonBlockingReturnsAfterBind(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:            "test-token",
		EnableMetricsAPI: true,
		RateLimit:        60,
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
		// Update API off → non-blocking regardless of UnblockHTTPAPI.
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- SetupAndStartAPI(ctx, opts)
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "non-blocking SetupAndStartAPI should return after bind")
	case <-time.After(2 * time.Second):
		t.Fatal("non-blocking SetupAndStartAPI did not return after bind")
	}

	// Server must still be running until ctx is canceled (GracefulContext).
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestRunServer_BlockingUntilCancel(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:           "test-token",
		EnableUpdateAPI: true,
		UpdateLock:      scheduling.NewUpdateLock(),
		UnblockHTTPAPI:  false,
		RateLimit:       60,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		WriteStartupMessage: func(_ logging.StartupParams) {
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- SetupAndStartAPI(ctx, opts)
	}()

	select {
	case err := <-done:
		t.Fatalf("blocking SetupAndStartAPI returned early: %v", err)
	case <-time.After(300 * time.Millisecond):
		// Expected: still blocked while server runs.
	}

	cancel()

	select {
	case err := <-done:
		assert.True(t, err == nil || errors.Is(err, context.Canceled),
			"unexpected error: %v", err)
	case <-time.After(shutdownGracePeriod + 2*time.Second):
		t.Fatal("blocking SetupAndStartAPI did not return after cancel")
	}
}

func TestRunServer_UnblockWithUpdateAPI_NonBlocking(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestListenAddr(config.Options{
		Token:           "test-token",
		EnableUpdateAPI: true,
		UpdateLock:      scheduling.NewUpdateLock(),
		UnblockHTTPAPI:  true,
		RateLimit:       60,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- SetupAndStartAPI(ctx, opts)
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "update API with UnblockHTTPAPI should return after bind")
	case <-time.After(2 * time.Second):
		t.Fatal("SetupAndStartAPI did not return in non-blocking update mode")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestRunServer_BindFailure(t *testing.T) {
	testMetrics := metrics.Default()

	opts := withTestLogger(config.Options{
		Host:             "127.0.0.1",
		Port:             "99999", // invalid port
		Token:            "test-token",
		EnableMetricsAPI: true,
		RateLimit:        60,
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := SetupAndStartAPI(ctx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start HTTP server")
}

func TestRunServer_OnUnexpectedServerStopCallback(t *testing.T) {
	t.Run("non_clean_stop_invokes_callback", func(t *testing.T) {
		called := make(chan error, 1)
		onStop := func(err error) {
			called <- err
		}

		handleUnexpectedServerStop(testLogger(), errors.New("simulated listen failure"), "127.0.0.1:8080", onStop)

		select {
		case err := <-called:
			require.Error(t, err)
			assert.Contains(t, err.Error(), "simulated listen failure")
		case <-time.After(time.Second):
			t.Fatal("OnUnexpectedServerStop was not invoked")
		}
	})

	t.Run("clean_stop_does_not_invoke_callback", func(t *testing.T) {
		called := make(chan error, 1)
		onStop := func(err error) {
			called <- err
		}

		handleUnexpectedServerStop(testLogger(), nil, "127.0.0.1:8080", onStop)

		select {
		case <-called:
			t.Fatal("OnUnexpectedServerStop should not be invoked on clean stop")
		case <-time.After(100 * time.Millisecond):
			// Expected: callback was not called.
		}
	})
}

func TestGetAPIAddr_IPv6(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "IPv4",
			host:     "127.0.0.1",
			port:     "8080",
			expected: "127.0.0.1:8080",
		},
		{
			name:     "hostname",
			host:     "localhost",
			port:     "8080",
			expected: "localhost:8080",
		},
		{
			name:     "IPv6 loopback",
			host:     "::1",
			port:     "8080",
			expected: "[::1]:8080",
		},
		{
			name:     "IPv6 full",
			host:     "2001:db8::1",
			port:     "8080",
			expected: "[2001:db8::1]:8080",
		},
		{
			name:     "IPv6 already bracketed",
			host:     "[::1]",
			port:     "8080",
			expected: "[::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAPIAddr(tt.host, tt.port)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestListen_AddressFamilies verifies the address families listen binds for
// each kind of host. An IPv6 host binds that address, and the unspecified IPv6
// address accepts IPv6 clients, and IPv4 clients where the system supports
// dual-stack listeners. An unset or IPv4 host stays on IPv4, so the default
// bind does not start accepting IPv6 clients.
func TestListen_AddressFamilies(t *testing.T) {
	tests := []struct {
		name    string
		address string
		// needsIPv6 skips the case on hosts without an IPv6 loopback address.
		needsIPv6 bool
		// wantHost is the host the listener reports.
		wantHost string
		// reachable lists the loopback hosts that must accept a connection.
		reachable []string
		// dualStack also requires IPv4 clients to connect where the system
		// supports dual-stack listeners.
		dualStack bool
		// unreachable lists the loopback hosts that must refuse a connection.
		unreachable []string
	}{
		{
			name:        "unset host",
			address:     ":0",
			wantHost:    "0.0.0.0",
			reachable:   []string{"127.0.0.1"},
			unreachable: []string{"::1"},
		},
		{
			name:        "IPv4 unspecified",
			address:     "0.0.0.0:0",
			wantHost:    "0.0.0.0",
			reachable:   []string{"127.0.0.1"},
			unreachable: []string{"::1"},
		},
		{
			name:      "IPv4 loopback",
			address:   "127.0.0.1:0",
			wantHost:  "127.0.0.1",
			reachable: []string{"127.0.0.1"},
		},
		{
			name:      "IPv4-mapped IPv6",
			address:   "[::ffff:127.0.0.1]:0",
			wantHost:  "127.0.0.1",
			reachable: []string{"127.0.0.1"},
		},
		{
			name:      "IPv6 loopback",
			address:   "[::1]:0",
			needsIPv6: true,
			wantHost:  "::1",
			reachable: []string{"::1"},
		},
		{
			name:      "IPv6 unspecified",
			address:   "[::]:0",
			needsIPv6: true,
			wantHost:  "::",
			reachable: []string{"::1"},
			dualStack: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needsIPv6 {
				requireIPv6Loopback(t)
			}

			ln, err := listen(t.Context(), tt.address, "", "")
			require.NoError(t, err)

			defer ln.Close()

			host, port, err := net.SplitHostPort(ln.Addr().String())
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)

			reachable := tt.reachable
			if tt.dualStack && supportsDualStack(t) {
				reachable = append(reachable, "127.0.0.1")
			}

			var dialer net.Dialer

			for _, client := range reachable {
				conn, err := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort(client, port))
				if assert.NoError(t, err, "a client on %s could not connect", client) {
					_ = conn.Close()
				}
			}

			for _, client := range tt.unreachable {
				conn, err := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort(client, port))
				if err == nil {
					_ = conn.Close()
				}

				assert.Error(t, err, "a client on %s connected", client)
			}
		})
	}
}

// TestSetupAndStartAPI_IPv6Host verifies that the API serves requests on an
// IPv6 host and releases the socket when canceled.
func TestSetupAndStartAPI_IPv6Host(t *testing.T) {
	requireIPv6Loopback(t)

	ln, err := listenTCP(t, "[::1]:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	opts := withTestLogger(config.Options{
		Host:            host,
		Port:            port,
		EnableHealthAPI: true,
		RateLimit:       60,
	})

	require.NoError(t, SetupAndStartAPI(ctx, opts))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/livez", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	requireAddrReleased(t, addr)
}

// requireIPv6Loopback skips the test on hosts without an IPv6 loopback
// address, such as containers with IPv6 disabled.
func requireIPv6Loopback(t *testing.T) {
	t.Helper()

	ln, err := listenTCP(t, "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}

	_ = ln.Close()
}

// supportsDualStack reports whether a TCP listener on the unspecified IPv6
// address also accepts IPv4 clients on this system. OpenBSD and DragonFly BSD,
// for example, keep such a listener IPv6 only.
func supportsDualStack(t *testing.T) bool {
	t.Helper()

	ln, err := listenTCP(t, "[::]:0")
	if err != nil {
		return false
	}

	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	var dialer net.Dialer

	conn, err := dialer.DialContext(t.Context(), "tcp4", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}

// listenTCP binds addr for a test, over the address family of its host.
func listenTCP(t *testing.T, addr string) (net.Listener, error) {
	t.Helper()

	var listenConfig net.ListenConfig

	return listenConfig.Listen(t.Context(), "tcp", addr)
}

// freeAddr returns a local IPv4 address with a port that was free when it
// was chosen.
func freeAddr(t *testing.T) string {
	t.Helper()

	ln, err := listenTCP(t, "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return addr
}

// requireAddrReleased waits until addr can be bound again, which shows the
// server stopped listening on it.
func requireAddrReleased(t *testing.T, addr string) {
	t.Helper()

	require.Eventually(t, func() bool {
		ln, err := listenTCP(t, addr)
		if err != nil {
			return false
		}

		_ = ln.Close()

		return true
	}, shutdownGracePeriod+2*time.Second, 20*time.Millisecond, "the server kept listening on %s", addr)
}

// TestRunServer_CancelBetweenBindAndServe verifies that a server canceled
// after its socket is bound but before it starts serving still stops and
// releases the socket, in blocking and non-blocking mode. Fiber runs OnListen
// hooks in that gap, so the hook cancels there and waits until the shutdown
// has run, which a server that is not serving yet ignores.
func TestRunServer_CancelBetweenBindAndServe(t *testing.T) {
	for _, block := range []bool{true, false} {
		t.Run(map[bool]string{true: "blocking", false: "non-blocking"}[block], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			shutdownRan := make(chan struct{})

			app := newApp(testLogger(), 60, proxyConfig{}, corsConfig{}, true)
			app.Hooks().OnPostShutdown(func(error) error {
				close(shutdownRan)

				return nil
			})
			app.Hooks().OnListen(func(fiber.ListenData) error {
				cancel()

				select {
				case <-shutdownRan:
				case <-time.After(shutdownGracePeriod):
				}

				return nil
			})

			addr := freeAddr(t)
			done := make(chan error, 1)

			go func() {
				done <- runServer(ctx, testLogger(), app, addr, "", "", block, nil, nil)
			}()

			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(shutdownGracePeriod + 2*time.Second):
				t.Fatal("the server canceled between bind and serve did not stop")
			}

			requireAddrReleased(t, addr)
		})
	}
}

// TestSetupAndStartAPI_StartupMessageAfterBind verifies that in blocking mode
// the startup message is written once the API socket is bound, and not at all
// when the socket cannot be bound.
func TestSetupAndStartAPI_StartupMessageAfterBind(t *testing.T) {
	blockingOpts := func(addr string, startup func(logging.StartupParams)) config.Options {
		host, port, err := net.SplitHostPort(addr)
		require.NoError(t, err)

		return withTestLogger(config.Options{
			Host:            host,
			Port:            port,
			Token:           "test-token",
			EnableUpdateAPI: true,
			UpdateLock:      scheduling.NewUpdateLock(),
			RateLimit:       60,
			RunUpdatesWithNotifications: func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
				return &metrics.Metric{}
			},
			FilterByImage:       func(_ []string, f types.Filter) types.Filter { return f },
			DefaultMetrics:      func() *metrics.Metrics { return &metrics.Metrics{} },
			WriteStartupMessage: startup,
		})
	}

	t.Run("bound", func(t *testing.T) {
		addr := freeAddr(t)
		bound := make(chan bool, 1)

		opts := blockingOpts(addr, func(logging.StartupParams) {
			// Binding the address fails while the API holds it.
			ln, err := listenTCP(t, addr)
			if err == nil {
				_ = ln.Close()
			}

			bound <- err != nil
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		done := make(chan error, 1)

		go func() {
			done <- SetupAndStartAPI(ctx, opts)
		}()

		select {
		case wasBound := <-bound:
			assert.True(t, wasBound, "the startup message was written before the API socket was bound")
		case <-time.After(2 * time.Second):
			t.Fatal("the startup message was not written")
		}

		cancel()
		require.NoError(t, <-done)
	})

	t.Run("address in use", func(t *testing.T) {
		taken, err := listenTCP(t, "127.0.0.1:0")
		require.NoError(t, err)

		defer taken.Close()

		var called atomic.Bool

		opts := blockingOpts(taken.Addr().String(), func(logging.StartupParams) { called.Store(true) })

		err = SetupAndStartAPI(t.Context(), opts)
		require.ErrorContains(t, err, "failed to start HTTP server")
		assert.False(t, called.Load(), "the startup message was written although the API could not bind")
	})
}

// writeTestCertificate writes a self-signed certificate for 127.0.0.1 and its
// key to dir and returns their paths.
func writeTestCertificate(t *testing.T, dir string) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "watchtower-test"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))

	return certPath, keyPath
}

// TestSetupAndStartAPI_TLS verifies that the API serves HTTPS with the
// configured certificate, refuses clients limited to TLS 1.1, and releases its
// socket when canceled.
func TestSetupAndStartAPI_TLS(t *testing.T) {
	certPath, keyPath := writeTestCertificate(t, t.TempDir())

	addr := freeAddr(t)
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	opts := withTestLogger(config.Options{
		Host:            host,
		Port:            port,
		EnableHealthAPI: true,
		RateLimit:       60,
		TLSCertPath:     certPath,
		TLSKeyPath:      keyPath,
	})

	require.NoError(t, SetupAndStartAPI(ctx, opts))

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+addr+"/livez", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true,
		MaxVersion:         tls.VersionTLS11,
	}}

	conn, err := dialer.DialContext(ctx, "tcp4", addr)
	if err == nil {
		_ = conn.Close()
	}

	require.Error(t, err, "a TLS 1.1 client was accepted")

	cancel()
	requireAddrReleased(t, addr)
}
