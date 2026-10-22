package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/routes"
)

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
// The server shuts down gracefully when ctx is canceled, waiting up to five
// seconds for the requests it is serving. The returned channel closes once that
// shutdown attempt has finished, so a caller can wait for it before the process
// exits. Requests still running when the five seconds expire may remain active
// after the channel closes.
//
// Parameters:
//   - ctx: Context for server lifecycle management.
//   - opts: API configuration options. opts.Logger is required when any endpoint
//     is enabled. Nil yields config.ErrMissingLogger.
//
// Returns:
//   - <-chan struct{}: Closed once the server has stopped and its shutdown
//     attempt has finished. It is already closed when no endpoint is enabled,
//     and nil when err is non-nil.
//   - error: Non-nil if route registration or server startup fails.
func SetupAndStartAPI(ctx context.Context, opts config.Options) (<-chan struct{}, error) {
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
		return stoppedNow(), nil
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
		return nil, config.ErrMissingAPIToken
	}

	if shouldRequireEventsToken && opts.EventsToken == "" {
		return nil, config.ErrMissingEventsAPIToken
	}

	if opts.Logger == nil {
		return nil, config.ErrMissingLogger
	}

	log := opts.Logger

	app := newApp(
		log,
		opts.RateLimit,
		proxyConfig{
			TrustedProxies: opts.TrustedProxies,
			ProxyHeader:    opts.ProxyHeader,
		}, corsConfig{
			AllowedOrigins: opts.CORSAllowedOrigins,
		}, opts.NoStartupMessage,
	)

	authMiddleware := newAPIAuthMiddleware(log, opts.Token)

	err := routes.ValidateAndRegister(ctx, app, authMiddleware, opts)
	if err != nil {
		return nil, fmt.Errorf("route registration failed: %w", err)
	}

	tlsCertPath, tlsKeyPath := opts.TLSCertPath, opts.TLSKeyPath

	if (tlsCertPath == "") != (tlsKeyPath == "") {
		return nil, config.ErrMissingTLSConfig
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
//   - <-chan struct{}: Closed once the server has stopped, its shutdown attempt
//     has finished, and any unexpected stop has been handled. Nil when err is
//     non-nil.
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
) (<-chan struct{}, error) {
	//nolint:nilerr // Intentionally return nil: skip socket binding when context is already canceled.
	if ctx.Err() != nil {
		return stoppedNow(), nil
	}

	listener, err := listen(ctx, address, tlsCertPath, tlsKeyPath)
	if err != nil {
		// A cancellation during the bind is a shutdown, not a failure.
		if ctx.Err() != nil {
			return stoppedNow(), nil //nolint:nilerr // The bind failed because the server is shutting down.
		}

		log.Error().Err(err).Str("addr", address).
			Msg("HTTP server failed to start")

		return nil, fmt.Errorf("failed to start HTTP server: %w", err)
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
		done := make(chan struct{})

		go func() {
			defer close(done)

			<-served
			handleUnexpectedServerStop(log, serveErr, address, onUnexpectedStop)
			<-stopped
		}()

		return done, nil
	}

	// Blocking mode: wait until the server stops and any shutdown has drained.
	<-served
	<-stopped

	if !isCleanServerStop(serveErr) {
		log.Error().Err(serveErr).Str("addr", address).
			Msg("HTTP server failed")

		return nil, fmt.Errorf("HTTP server failed: %w", serveErr)
	}

	return stoppedNow(), nil
}

// stoppedNow returns a closed channel, for a server that has already stopped
// or never started.
//
// Returns:
//   - <-chan struct{}: A closed channel.
func stoppedNow() <-chan struct{} {
	stopped := make(chan struct{})
	close(stopped)

	return stopped
}

// listen binds the API socket, wrapping it in TLS when a certificate is set.
//
// It binds the network that listenNetwork chooses for the address and
// requires TLS 1.2 or later, as Fiber's Listen does by default.
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

	listener, err := listenConfig.Listen(ctx, listenNetwork(address), address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}

	if tlsConfig != nil {
		return tls.NewListener(listener, tlsConfig), nil
	}

	return listener, nil
}

// listenNetwork returns the network to bind address on.
//
// An IPv6 host binds on "tcp", so the unspecified address "::" accepts IPv6
// clients, and also IPv4 clients on systems that support IPv4-mapped IPv6
// addresses, such as Linux, macOS and Windows. Any other host, including an
// unset one or an IPv4-mapped address, binds on "tcp4" and accepts IPv4
// clients only, as Fiber does by default.
//
// Parameters:
//   - address: Address to listen on.
//
// Returns:
//   - string: The Fiber network name.
func listenNetwork(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fiber.NetworkTCP4
	}

	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.Is6() || ip.Is4In6() {
		return fiber.NetworkTCP4
	}

	return fiber.NetworkTCP
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
	err := app.ShutdownWithTimeout(shutdownGracePeriod)
	if err != nil {
		log.Debug().Err(err).Msg("HTTP server shutdown did not finish draining")
	}

	err = listener.Close()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		log.Debug().Err(err).Msg("Failed to close the HTTP server listener")
	}
}
