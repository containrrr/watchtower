package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/routes"
)

// Server defines the interface for Watchtower's HTTP API server.
type Server interface {
	Listen(addr string, config ...fiber.ListenConfig) error
	ShutdownWithTimeout(timeout time.Duration) error
}

// Compile-time check that *fiber.App implements Server.
var _ Server = (*fiber.App)(nil)

// GetAPIAddr formats the API address string from host and port, bracketing
// IPv6 addresses as needed.
//
// Parameters:
//   - host: Hostname or IP address.
//   - port: Port number.
//
// Returns:
//   - string: Formatted address string.
func GetAPIAddr(host, port string) string {
	if port == "" {
		return host
	}

	address := host + ":" + port

	bracketed := strings.Contains(host, ":") && !strings.HasPrefix(host, "[")
	if bracketed {
		address = "[" + host + "]:" + port
	}

	return address
}

// SetupAndStartAPI configures and launches the HTTP API.
//
// It creates a Fiber application with the middleware stack, registers the
// configured endpoints, and starts the server. When the update API is enabled
// and UnblockHTTPAPI is false (API-only mode), this call blocks until ctx is
// canceled. Otherwise the server runs in the background and this function
// returns after the listen socket is bound so scheduled updates can run
// concurrently.
//
// Parameters:
//   - ctx: Context for server lifecycle management.
//   - opts: API configuration options. opts.Logger is required when any endpoint
//     is enabled. Nil yields config.ErrMissingLogger.
//
// Returns:
//   - error: Non-nil if route registration or server startup fails.
func SetupAndStartAPI(ctx context.Context, opts config.Options) error {
	if !opts.EnableUpdateAPI &&
		!opts.EnableMetricsAPI &&
		!opts.EnableContainersAPI &&
		!opts.EnableCheckAPI &&
		!opts.EnableSwaggerAPI &&
		!opts.EnableHealthAPI &&
		!opts.EnableHistoryAPI &&
		!opts.EnableImagesAPI &&
		!opts.EnableConfigAPI &&
		!opts.EnableEventsAPI {
		return nil
	}

	address := GetAPIAddr(opts.Host, opts.Port)

	shouldRequireToken := opts.EnableUpdateAPI ||
		opts.EnableMetricsAPI ||
		opts.EnableContainersAPI ||
		opts.EnableCheckAPI ||
		opts.EnableHistoryAPI ||
		opts.EnableImagesAPI ||
		opts.EnableConfigAPI

	shouldRequireEventsToken := opts.EnableEventsAPI

	if shouldRequireToken && opts.Token == "" {
		return config.ErrMissingAPIToken
	}

	if shouldRequireEventsToken && opts.EventsToken == "" {
		return config.ErrMissingEventsAPIToken
	}

	if opts.Logger == nil {
		return config.ErrMissingLogger
	}

	log := opts.Logger

	app := New(
		log,
		opts.RateLimit,
		ProxyConfig{
			TrustedProxies: opts.TrustedProxies,
			ProxyHeader:    opts.ProxyHeader,
		}, CORSConfig{
			AllowedOrigins: opts.CORSAllowedOrigins,
		}, opts.NoStartupMessage,
	)

	authMiddleware := NewAPIAuthMiddleware(log, opts.Token)

	err := routes.ValidateAndRegister(ctx, app, authMiddleware, opts)
	if err != nil {
		return fmt.Errorf("route registration failed: %w", err)
	}

	tlsCertPath, tlsKeyPath := opts.TLSCertPath, opts.TLSKeyPath

	if (tlsCertPath == "") != (tlsKeyPath == "") {
		return config.ErrMissingTLSConfig
	}

	// Block only in API-only update mode.
	block := opts.EnableUpdateAPI && !opts.UnblockHTTPAPI

	return runServer(
		ctx,
		log,
		app,
		address,
		tlsCertPath,
		tlsKeyPath,
		block,
		startupMessageWriter(opts, block),
		opts.OnUnexpectedServerStop,
	)
}

// startupMessageWriter returns the callback that writes the startup message
// once the API socket is bound.
//
// Only the blocking HTTP API mode writes it here, because the API then runs
// instead of the scheduler, which otherwise announces startup.
//
// Parameters:
//   - opts: API configuration options.
//   - block: Whether the API runs in blocking mode.
//
// Returns:
//   - func(): The callback, or nil when no startup message is written here.
func startupMessageWriter(opts config.Options, block bool) func() {
	if !block || opts.WriteStartupMessage == nil {
		return nil
	}

	return func() { writeStartupMessage(opts) }
}

// writeStartupMessage writes the startup message for the blocking HTTP API mode.
//
// Parameters:
//   - opts: API configuration options with a non-nil WriteStartupMessage.
func writeStartupMessage(opts config.Options) {
	startup := opts.Startup
	startup.Sched = time.Time{}
	startup.Filtering = opts.FilterDesc
	startup.Scope = opts.Scope
	startup.Client = opts.Client
	startup.Notifier = opts.Notifier
	startup.Version = opts.Version
	opts.WriteStartupMessage(startup)
}

// isCleanServerStop reports whether err is an expected result of a graceful
// shutdown (or a nil error after Listen returns cleanly).
func isCleanServerStop(err error) bool {
	if err == nil {
		return true
	}

	return errors.Is(err, http.ErrServerClosed) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// handleUnexpectedServerStop logs and forwards unexpected listen errors.
//
// Parameters:
//   - log: Logger for unexpected-stop messages.
//   - err: Error returned by Listen.
//   - addr: Address the server was listening on.
//   - onUnexpectedStop: Optional callback invoked with the listen error.
func handleUnexpectedServerStop(log *zerolog.Logger, err error, addr string, onUnexpectedStop func(error)) {
	if !isCleanServerStop(err) {
		log.Error().Err(err).Str("addr", addr).
			Msg("HTTP server stopped unexpectedly")

		if onUnexpectedStop != nil {
			onUnexpectedStop(err)
		}
	}
}

// runServer binds the listen socket, serves the Fiber app on it, and either
// blocks until shutdown or returns once the socket is bound.
//
// The server stops when ctx is canceled. runServer shuts the app down and then
// closes the listener itself, because a shutdown that arrives before the app
// starts serving is ignored, and only closing the listener stops a server that
// starts serving afterwards.
//
// Parameters:
//   - ctx: Context for server lifecycle management.
//   - log: Logger for bind and listen error messages.
//   - app: Fiber application to start.
//   - address: Address to listen on.
//   - tlsCertPath: Path to TLS certificate file, or empty for HTTP.
//   - tlsKeyPath: Path to TLS key file, or empty for HTTP.
//   - block: When true, wait until the server stops. When false, return after bind.
//   - onBound: Optional callback run once the socket is bound, before serving.
//   - onUnexpectedStop: Optional callback when the server exits unexpectedly in non-blocking mode.
//
// Returns:
//   - error: Non-nil if the server fails to start or exits with an unexpected error while blocking.
func runServer(
	ctx context.Context,
	log *zerolog.Logger,
	app *fiber.App,
	address string,
	tlsCertPath, tlsKeyPath string,
	block bool,
	onBound func(),
	onUnexpectedStop func(error),
) error {
	//nolint:nilerr // Intentionally return nil: skip socket binding when context is already canceled.
	if ctx.Err() != nil {
		return nil
	}

	listener, err := listen(ctx, address, tlsCertPath, tlsKeyPath)
	if err != nil {
		// A cancellation during the bind is a shutdown, not a failure.
		if ctx.Err() != nil {
			return nil //nolint:nilerr // The bind failed because the server is shutting down.
		}

		log.Error().Err(err).Str("addr", address).
			Msg("HTTP server failed to start")

		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	if onBound != nil {
		onBound()
	}

	var serveErr error

	served := make(chan struct{})

	go func() {
		defer close(served)

		serveErr = app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		select {
		case <-ctx.Done():
			stopServer(log, app, listener)
		case <-served:
		}
	}()

	if !block {
		go func() {
			<-served
			handleUnexpectedServerStop(log, serveErr, address, onUnexpectedStop)
		}()

		return nil
	}

	// Blocking mode: wait until the server stops and any shutdown has drained.
	<-served
	<-stopped

	if !isCleanServerStop(serveErr) {
		log.Error().Err(serveErr).Str("addr", address).
			Msg("HTTP server failed")

		return fmt.Errorf("HTTP server failed: %w", serveErr)
	}

	return nil
}

// listen binds the API socket, wrapping it in TLS when a certificate is set.
//
// It binds IPv4 and requires TLS 1.2 or later, as Fiber's Listen does by default.
//
// Parameters:
//   - ctx: Context for the bind.
//   - address: Address to listen on.
//   - tlsCertPath: Path to TLS certificate file, or empty for HTTP.
//   - tlsKeyPath: Path to TLS key file, or empty for HTTP.
//
// Returns:
//   - net.Listener: The bound listener.
//   - error: Non-nil if the certificate cannot be loaded or the socket cannot be bound.
func listen(ctx context.Context, address, tlsCertPath, tlsKeyPath string) (net.Listener, error) {
	var tlsConfig *tls.Config

	if tlsCertPath != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load TLS key pair: %w", err)
		}

		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
	}

	listenConfig := net.ListenConfig{}

	listener, err := listenConfig.Listen(ctx, fiber.NetworkTCP4, address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}

	if tlsConfig != nil {
		return tls.NewListener(listener, tlsConfig), nil
	}

	return listener, nil
}

// stopServer shuts the app down and closes its listener.
//
// The shutdown drains requests in flight. Closing the listener afterwards
// stops a server that had not started serving when the shutdown ran, which
// the shutdown alone ignores.
//
// Parameters:
//   - log: Logger for shutdown errors.
//   - app: Fiber application to stop.
//   - listener: The app's listener.
func stopServer(log *zerolog.Logger, app *fiber.App, listener net.Listener) {
	err := app.ShutdownWithTimeout(ShutdownGracePeriod)
	if err != nil {
		log.Debug().Err(err).Msg("HTTP server shutdown did not finish draining")
	}

	err = listener.Close()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		log.Debug().Err(err).Msg("Failed to close the HTTP server listener")
	}
}
