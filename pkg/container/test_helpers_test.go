package container

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/logging"
)

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

// randomSHA256 returns 64 random hex characters, the form of a SHA-256 digest
// without its "sha256:" prefix, so each test gets unique image IDs and digests.
func randomSHA256() string {
	hash := make([]byte, sha256.Size)
	_, _ = rand.Read(hash)

	return hex.EncodeToString(hash)
}

// mustParseMAC returns a 6-byte HardwareAddr as Docker inspect JSON unmarshals it.
//
// Tests must not construct HardwareAddr from the colon-separated string bytes.
// That 17-byte value is not what the Engine API produces.
func mustParseMAC(mac string) dockerNetwork.HardwareAddr {
	parsed, err := net.ParseMAC(mac)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "parse MAC %q", mac)

	return dockerNetwork.HardwareAddr(parsed)
}

// lockedBuffer is a buffer that is safe for concurrent writes, for logs that
// goroutines started by a test write while the test reads them.
type lockedBuffer struct {
	// mu guards buf.
	mu sync.Mutex
	// buf holds the written bytes.
	buf bytes.Buffer
}

// Write appends p to the buffer.
//
// Parameters:
//   - p: Bytes to append.
//
// Returns:
//   - int: Number of bytes written.
//   - error: Always nil.
func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String returns everything written so far.
//
// Returns:
//   - string: The buffer's contents.
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// newCapturedLogger returns a logfmt logger at level that writes to a buffer the
// test reads, in the format production logs use.
//
// Parameters:
//   - level: Lowest level the logger writes.
//
// Returns:
//   - *zerolog.Logger: Logger writing to the buffer.
//   - *lockedBuffer: Buffer holding the logged lines.
func newCapturedLogger(level zerolog.Level) (*zerolog.Logger, *lockedBuffer) {
	buf := &lockedBuffer{}
	w := logging.LogfmtWriter(buf)
	w.TimeFormat = time.RFC3339

	return new(zerolog.New(w).Level(level).With().Timestamp().Logger()), buf
}

// captureLog returns a logfmt *zerolog.Logger writing to a gbytes.Buffer for gomega.Say assertions.
func captureLog(level zerolog.Level) (*zerolog.Logger, *gbytes.Buffer) {
	buf := gbytes.NewBuffer()
	w := logging.LogfmtWriter(buf)
	// Preserve timestamp style used by production logfmt.
	w.TimeFormat = time.RFC3339
	l := zerolog.New(w).Level(level).With().Timestamp().Logger()

	return &l, buf
}

// missingImageHandlers mocks a running container whose image can no longer be
// inspected, which happens when the image is removed out from under it.
//
// The container inspect succeeds and the image inspect returns 404, so callers
// end up with a container whose HasImageInfo reports false.
func missingImageHandlers(containerID string) []http.HandlerFunc {
	return []http.HandlerFunc{
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp(
					fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
				),
			),
			ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
				ID:    containerID,
				Name:  "/test-container",
				Image: "test-image:latest",
				State: &dockerContainer.State{
					Status:  "running",
					Running: true,
				},
				HostConfig: &dockerContainer.HostConfig{},
				Config: &dockerContainer.Config{
					Image: "test-image:latest",
				},
			}),
		),
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
			),
			ghttp.RespondWith(http.StatusNotFound, `{"message":"No such image"}`),
		),
	}
}
