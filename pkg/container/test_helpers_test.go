package container

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/logging"
)

// testLog returns a discarded zerolog logger for tests that do not assert on output.
func testLog() *zerolog.Logger {
	return new(zerolog.Nop())
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
