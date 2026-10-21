package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"text/template"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	shoutrrrTypes "github.com/nicholas-fedor/shoutrrr/pkg/types"

	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var allButTrace = zerolog.DebugLevel

const shoutrrrFatalHelperEnv = "WATCHTOWER_SHOUTRRR_FATAL_HELPER"

// lockedBuffer is a buffer that is safe for concurrent writes, for logs that
// the notifier's goroutines write while the test reads them.
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

// newBufferLogger returns a JSON logger at trace level and the buffer it writes
// to, for tests that check what the notifier logs.
//
// Returns:
//   - *zerolog.Logger: Logger writing to the buffer.
//   - *lockedBuffer: Buffer holding the log output.
func newBufferLogger() (*zerolog.Logger, *lockedBuffer) {
	buf := &lockedBuffer{}
	// JSON encoding keeps field assertions simple (failure_type, failed_count, etc.).
	l := zerolog.New(buf).Level(zerolog.TraceLevel).With().Timestamp().Logger()

	return &l, buf
}

// createTestNotifier wraps createNotifier with the default template. Pass a
// custom template to createNotifier directly when a test needs one.
func createTestNotifier(
	log *zerolog.Logger,
	urls []string,
	level zerolog.Level,
	legacy bool,
	data StaticData,
	stdout bool,
	delay time.Duration,
) *shoutrrrTypeNotifier {
	return createNotifier(log, urls, level, "", legacy, data, stdout, delay)
}

// TODO: Remove legacyMockData when legacy notification types are removed.
//
//nolint:godox
var legacyMockData = notificationData{
	Entries: []*notificationEntry{
		{
			Level:   "info",
			Message: "foo Bar",
		},
	},
}

var mockDataMultipleEntries = notificationData{
	Entries: []*notificationEntry{
		{
			Level:   "info",
			Message: "The situation is under control",
		},
		{
			Level:   "warning",
			Message: "All the smoke might be covering up some problems",
		},
		{
			Level:   "error",
			Message: "Turns out everything is on fire",
		},
	},
}

var mockDataAllFresh = notificationData{
	Entries: []*notificationEntry{},
	Report:  newProgressReport(session.FreshState),
}

// mockDataFromStates generates mock notification data with specified container states.
// It includes legacy log entries and static data for testing purposes.
//
// TODO: Remove legacyMockData reference when legacy notification types are removed.
//
//nolint:godox
func mockDataFromStates(t *testing.T, states ...session.State) notificationData {
	t.Helper()

	hostname := "Mock"
	prefix := ""

	return notificationData{
		Entries: legacyMockData.Entries,
		Report:  newProgressReport(states...),
		Title:   getTitle(newTestLogger(t), hostname, prefix),
		Host:    hostname,
	}
}

// Error values recorded on the skipped and failed containers of a progress report.
var (
	// errReportSkipped is the reason recorded for a skipped container.
	errReportSkipped = errors.New("unpossible")
	// errReportFailed is the error recorded for a failed container.
	errReportFailed = errors.New("accidentally the whole container")
)

// Container ID prefixes that keep the containers of each state in a progress
// report distinct, such as "c7941..." for the first skipped container.
const (
	// updatedIDPrefix prefixes updated containers.
	updatedIDPrefix = 11
	// failedIDPrefix prefixes failed containers.
	failedIDPrefix = 21
	// freshIDPrefix prefixes fresh containers.
	freshIDPrefix = 31
	// skippedIDPrefix prefixes skipped containers.
	skippedIDPrefix = 41
	// restartedIDPrefix prefixes restarted containers.
	restartedIDPrefix = 51
)

// progressIDLength is the length of a full container or image ID.
const progressIDLength = 64

// maxContainersPerState is the number of containers of one state that keep
// unique IDs. The ID prefixes of the states are ten apart, so the eleventh
// updated container would reuse the first failed container's ID.
const maxContainersPerState = 10

// newProgressReport builds a session report with one container for each given
// state, numbering repeated states so every container has a unique ID and name.
//
// Parameters:
//   - states: Outcome of each container, in order.
//
// Returns:
//   - types.Report: Report built through session.Progress.
func newProgressReport(states ...session.State) types.Report {
	log := new(zerolog.Nop())
	// Count each state's containers so their IDs and names stay unique.
	stateNums := make(map[session.State]int)
	progress := session.Progress{}
	failed := make(map[types.ContainerID]error)

	for _, state := range states {
		index := stateNums[state]

		switch state {
		case session.SkippedState:
			c, _ := newProgressContainer(index, skippedIDPrefix, "skip%d")
			progress.AddSkipped(log, c, errReportSkipped, types.UpdateParams{})
		case session.FreshState:
			c, _ := newProgressContainer(index, freshIDPrefix, "frsh%d")
			progress.AddScanned(log, c, c.ImageID(), types.UpdateParams{})
		case session.UpdatedState:
			c, newImage := newProgressContainer(index, updatedIDPrefix, "updt%d")
			progress.AddScanned(log, c, newImage, types.UpdateParams{})
			progress.MarkForUpdate(log, c.ID())
		case session.FailedState:
			c, newImage := newProgressContainer(index, failedIDPrefix, "fail%d")
			progress.AddScanned(log, c, newImage, types.UpdateParams{})

			failed[c.ID()] = errReportFailed
		case session.RestartedState:
			c, _ := newProgressContainer(index, restartedIDPrefix, "rstr%d")
			progress.AddScanned(log, c, c.ImageID(), types.UpdateParams{})
			progress.MarkRestarted(log, c.ID())
		case session.UnknownState, session.ScannedState, session.StaleState:
			// Reports never end in these intermediate states.
			continue
		}

		stateNums[state] = index + 1
	}

	progress.UpdateFailed(log, failed)

	return progress.Report(log)
}

// newProgressContainer builds a running container for a progress report, with
// an ID and image IDs derived from idPrefix and index, and a name from
// nameFormat and index. It panics when index is maxContainersPerState or more.
//
// Parameters:
//   - index: Zero-based position of the container among its state's containers.
//   - idPrefix: Digits that start the container's ID after "c79".
//   - nameFormat: Name format with one %d verb for the one-based position.
//
// Returns:
//   - types.Container: Container running image mock/<name>:latest.
//   - types.ImageID: ID of the newer image the container can move to.
func newProgressContainer(index, idPrefix int, nameFormat string) (types.Container, types.ImageID) {
	if index >= maxContainersPerState {
		panic(fmt.Sprintf("newProgressContainer: index %d would reuse another state's container ID", index))
	}

	indexStr := strconv.Itoa(idPrefix + index)
	// Pad the ID with zeros to full length, after the "c79" prefix.
	idSuffix := indexStr + strings.Repeat("0", progressIDLength-3-len(indexStr))
	name := fmt.Sprintf(nameFormat, index+1)
	oldImageID := "01d" + idSuffix
	imageName := fmt.Sprintf("mock/%s:latest", name)

	inspect := dockerContainer.InspectResponse{
		ID:      "c79" + idSuffix,
		Image:   oldImageID,
		Name:    name,
		State:   &dockerContainer.State{Running: true},
		Created: time.Now().Format(time.RFC3339Nano),
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{Image: imageName},
	}
	imageInfo := dockerImage.InspectResponse{
		ID: oldImageID,
		RepoDigests: []string{
			fmt.Sprintf("%s@sha256:%s", oldImageID, strings.ReplaceAll(oldImageID, ":", "_")),
		},
	}

	return container.NewContainer(nil, &inspect, &imageInfo), types.ImageID("d0a" + idSuffix)
}

// TestShoutrrr verifies the shoutrrr notifier: message templates, the log
// hook, batching, the title, send error logging, closing, which reports it
// sends, and deduplication of grouped entries.
func TestShoutrrr(t *testing.T) {
	t.Parallel()

	t.Run("passing a common template name", func(t *testing.T) {
		t.Parallel()

		t.Run("should format using that template", func(t *testing.T) {
			t.Parallel()

			expected := `
updt1 (mock/updt1:latest): Updated
`[1:]
			data := mockDataFromStates(t, session.UpdatedState)
			assert.Equal(t, expected, getTemplatedResult(t, `porcelain.v1.summary-no-log`, false, data))
		})
	})

	t.Run("rendering porcelain JSON", func(t *testing.T) {
		t.Parallel()

		t.Run("should produce valid JSON with container details", func(t *testing.T) {
			t.Parallel()

			data := mockDataFromStates(t, session.UpdatedState, session.FreshState, session.FailedState)
			result := getTemplatedResult(t, `porcelain.json`, false, data)

			assert.JSONEq(t, `{
				"containers": [
					{
						"name": "updt1",
						"image": "mock/updt1:latest",
						"image_id": "01d110000000",
						"latest_image_id": "d0a110000000",
						"state": "Updated",
						"update_available": true
					},
					{
						"name": "fail1",
						"image": "mock/fail1:latest",
						"image_id": "01d210000000",
						"latest_image_id": "d0a210000000",
						"state": "Failed",
						"update_available": true,
						"error": "accidentally the whole container"
					},
					{
						"name": "frsh1",
						"image": "mock/frsh1:latest",
						"image_id": "01d310000000",
						"latest_image_id": "01d310000000",
						"state": "Fresh",
						"update_available": false
					}
				]
			}`, result)
		})
	})

	t.Run("adding a log hook", func(t *testing.T) {
		t.Parallel()

		t.Run("it has not been added before", func(t *testing.T) {
			t.Parallel()

			t.Run("should start receiving via RegisterHook", func(t *testing.T) {
				t.Parallel()

				level := zerolog.TraceLevel
				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{},
					level,
					true,
					StaticData{},
					false,
					time.Second,
				)
				assert.False(t, shoutrrr.receiving.Load())

				log := newTestLogger(t)

				shoutrrr.RegisterHook(log)
				defer shoutrrr.Close()

				assert.True(t, shoutrrr.receiving.Load())
			})
		})
		t.Run("it is being added a second time", func(t *testing.T) {
			t.Parallel()

			t.Run("should be idempotent", func(t *testing.T) {
				t.Parallel()

				level := zerolog.TraceLevel
				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{},
					level,
					true,
					StaticData{},
					false,
					time.Second,
				)
				log := newTestLogger(t)

				shoutrrr.RegisterHook(log)
				defer shoutrrr.Close()

				shoutrrr.RegisterHook(log)
				assert.True(t, shoutrrr.receiving.Load())
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy template tests when legacy notification types are removed.
	t.Run("using legacy templates", func(t *testing.T) {
		t.Parallel()

		t.Run("no custom template is provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should format the messages using the default template", func(t *testing.T) {
				t.Parallel()

				cmd := new(cobra.Command)
				flags.RegisterAll(cmd)

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{},
					zerolog.TraceLevel,
					true,
					StaticData{},
					false,
					time.Second,
				)
				entries := []*notificationEntry{
					{Message: "foo bar"},
				}

				s, err := shoutrrr.buildMessage(notificationData{Entries: entries})
				require.NoError(t, err)
				assert.Equal(t, "foo bar", s)
			})

			t.Run("should format image usage-check skips without dumping raw fields", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{},
					zerolog.TraceLevel,
					true,
					StaticData{},
					false,
					time.Second,
				)
				entries := []*notificationEntry{
					{
						Message: "Failed to list containers for image usage check, skipping removal",
						Data: map[string]any{
							"error":      `Get "http://socket-proxy-write:2375/v1.55/containers/json?all=1": terminated signal received`,
							"image_id":   "769846626f2f",
							"image_name": "ghcr.io/amir20/dtop:latest",
						},
					},
				}

				s, err := shoutrrr.buildMessage(notificationData{Entries: entries})
				require.NoError(t, err)
				assert.Equal(t, "Skipped image cleanup: ghcr.io/amir20/dtop:latest (769846626f2f): "+
					`Get "http://socket-proxy-write:2375/v1.55/containers/json?all=1": terminated signal received`, s)
				assert.NotContains(t, s, " | ")
			})

			t.Run("should format Docker image usage budget messages without dumping raw fields", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{},
					zerolog.TraceLevel,
					true,
					StaticData{},
					false,
					time.Second,
				)
				entries := []*notificationEntry{
					{
						Message: "Docker image usage exceeds configured maximum",
						Data: map[string]any{
							"usage":       int64(10_000),
							"max":         int64(10_000),
							"warn":        int64(8_000),
							"reclaimable": int64(2_000),
							"image_count": int64(4),
						},
					},
					{
						Message: "Docker image usage exceeds configured warning threshold",
						Data: map[string]any{
							"usage":       int64(8_000),
							"max":         int64(10_000),
							"warn":        int64(8_000),
							"reclaimable": int64(2_000),
							"image_count": int64(4),
						},
					},
					{
						Message: "Failed to query Docker image disk usage",
						Data:    map[string]any{"error": "df unavailable"},
					},
					{
						Message: "Docker image usage budget enabled",
						Data: map[string]any{
							"disk_space_max":  int64(40_000_000_000),
							"disk_space_warn": int64(32_000_000_000),
						},
					},
					{
						Message: "Docker image usage exceeds configured maximum",
						Data: map[string]any{
							"usage":       int64(0),
							"max":         int64(0),
							"reclaimable": int64(0),
							"image_count": int64(0),
						},
					},
				}

				s, err := shoutrrr.buildMessage(notificationData{Entries: entries})
				require.NoError(t, err)
				assert.Contains(t, s, "Docker image usage exceeds configured maximum: 10 KB of 10 KB used (2 KB reclaimable, 4 images)")
				assert.Contains(t, s, "Docker image usage exceeds configured warning threshold: 8 KB of 8 KB used (2 KB reclaimable, 4 images)")
				assert.Contains(t, s, "Failed to query Docker image disk usage: df unavailable")
				assert.Contains(t, s, "Docker image usage budget enabled: maximum 40 GB, warning at 32 GB")
				assert.Contains(t, s, "Docker image usage exceeds configured maximum: 0 B of 0 B used (0 B reclaimable, 0 images)")
				assert.NotContains(t, s, "unknown")
				assert.NotContains(t, s, " | ")
			})
		})
		t.Run("given a valid custom template", func(t *testing.T) {
			t.Parallel()

			t.Run("should format the messages using the custom template", func(t *testing.T) {
				t.Parallel()

				tplString := `{{range .}}{{.Level}}: {{.Message}}{{println}}{{end}}`
				tpl, err := getShoutrrrTemplate(newTestLogger(t), tplString, true)
				require.NoError(t, err)

				shoutrrr := &shoutrrrTypeNotifier{
					template:       tpl,
					legacyTemplate: true,
				}

				entries := []*notificationEntry{
					{
						Level:   "info",
						Message: "foo bar",
					},
				}

				s, err := shoutrrr.buildMessage(notificationData{Entries: entries})
				require.NoError(t, err)
				assert.Equal(t, "info: foo bar\n", s)
			})
		})

		t.Run("the default template", func(t *testing.T) {
			t.Parallel()

			t.Run("all containers are fresh", func(t *testing.T) {
				t.Parallel()

				t.Run("should return an empty string", func(t *testing.T) {
					t.Parallel()

					assert.Empty(t, getTemplatedResult(t, ``, true, mockDataAllFresh))
				})
			})
		})

		t.Run("given an invalid custom template", func(t *testing.T) {
			t.Parallel()

			t.Run("should format the messages using the default template", func(t *testing.T) {
				t.Parallel()

				invNotif, err := createNotifierWithTemplate(t, `{{ intentionalSyntaxError`, true)
				require.Error(t, err)
				invMsg, err := invNotif.buildMessage(legacyMockData)
				require.NoError(t, err)

				defNotif, err := createNotifierWithTemplate(t, ``, true)
				require.NoError(t, err)
				defMsg, err := defNotif.buildMessage(legacyMockData)
				require.NoError(t, err)

				assert.Equal(t, defMsg, invMsg)
			})
		})

		t.Run("given a template that is using ToUpper function", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the text in UPPER CASE", func(t *testing.T) {
				t.Parallel()

				tplString := `{{range .}}{{ .Message | ToUpper }}{{end}}`
				assert.Equal(t, "FOO BAR", getTemplatedResult(t, tplString, true, legacyMockData))
			})
		})

		t.Run("given a template that is using ToLower function", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the text in lower case", func(t *testing.T) {
				t.Parallel()

				tplString := `{{range .}}{{ .Message | ToLower }}{{end}}`
				assert.Equal(t, "foo bar", getTemplatedResult(t, tplString, true, legacyMockData))
			})
		})

		t.Run("given a template that is using Title function", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the text in Title Case", func(t *testing.T) {
				t.Parallel()

				tplString := `{{range .}}{{ .Message | Title }}{{end}}`
				assert.Equal(t, "Foo Bar", getTemplatedResult(t, tplString, true, legacyMockData))
			})
		})
	})

	t.Run("using report templates", func(t *testing.T) {
		t.Parallel()

		t.Run("no custom template is provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should format the messages using the default template", func(t *testing.T) {
				t.Parallel()

				expected := `4 Scanned, 2 Updated, 0 Restarted, 1 Failed, 1 Fresh, 1 Skipped
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000
- updt2 (mock/updt2:latest): 01d120000000 updated to d0a120000000
- frsh1 (mock/frsh1:latest): Fresh
- skip1 (mock/skip1:latest): Skipped: unpossible
- fail1 (mock/fail1:latest): Failed: accidentally the whole container`
				data := mockDataFromStates(t,
					session.UpdatedState,
					session.FreshState,
					session.FailedState,
					session.SkippedState,
					session.UpdatedState,
				)
				assert.Equal(t, expected, getTemplatedResult(t, ``, false, data))
			})
			t.Run("should use image IDs for container update reporting", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.UpdatedState)
				result := getTemplatedResult(t, ``, false, data)

				// Verify that the result contains image ID formats, not container IDs
				// Image IDs in the mock data are like "01d110000000" and "d0a110000000"
				// Container IDs are like "c79110000000"
				assert.Contains(t, result, "01d110000000")
				assert.Contains(t, result, "d0a110000000")
				assert.NotContains(t, result, "c79110000000")
			})
		})

		t.Run("using a template referencing Title", func(t *testing.T) {
			t.Parallel()

			t.Run("should contain the title in the output", func(t *testing.T) {
				t.Parallel()

				expected := `Watchtower updates on Mock`
				data := mockDataFromStates(t, session.UpdatedState)
				assert.Equal(t, expected, getTemplatedResult(t, `{{ .Title }}`, false, data))
			})
		})

		t.Run("using a template referencing Host", func(t *testing.T) {
			t.Parallel()

			t.Run("should contain the hostname in the output", func(t *testing.T) {
				t.Parallel()

				expected := `Mock`
				data := mockDataFromStates(t, session.UpdatedState)
				assert.Equal(t, expected, getTemplatedResult(t, `{{ .Host }}`, false, data))
			})
		})

		t.Run("using a template referencing Git and OCI report fields", func(t *testing.T) {
			t.Parallel()

			t.Run("should render Changelog, GitRepo, and Source", func(t *testing.T) {
				t.Parallel()

				status := updatedAppStatus()
				status.SetGitMetadata(container.ReportMeta{
					GitRepo:         "https://github.com/org/app.git",
					GitRef:          "v1.2.3",
					Changelog:       "https://github.com/org/app/releases",
					Source:          "https://github.com/org/app",
					ImageURL:        "https://example.com/image",
					Documentation:   "https://example.com/docs",
					CurrentVersion:  "1.2.2",
					LatestVersion:   "1.2.3",
					CurrentRevision: "abc123",
				})

				report := &session.SingleContainerReport{
					UpdatedReports: []types.ContainerReport{status},
				}

				tpl, err := template.New("git").Parse(
					`{{ range .Report.Updated }}{{ .Name }} {{ .Changelog }}{{ if .Source }} ({{ .Source }}){{ end }} repo={{ .GitRepo }}{{ end }}`,
				)
				require.NoError(t, err)

				var buf bytes.Buffer

				err = tpl.Execute(&buf, notificationData{Report: report})
				require.NoError(t, err)
				assert.Equal(t, "app https://github.com/org/app/releases (https://github.com/org/app) repo=https://github.com/org/app.git", buf.String())
			})
		})

		t.Run("the default template", func(t *testing.T) {
			t.Parallel()

			t.Run("all containers are fresh", func(t *testing.T) {
				t.Parallel()

				t.Run("should return the summary", func(t *testing.T) {
					t.Parallel()

					expected := `1 Scanned, 0 Updated, 0 Restarted, 0 Failed, 1 Fresh, 0 Skipped
- frsh1 (mock/frsh1:latest): Fresh`
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, mockDataAllFresh))
				})
			})
			t.Run("at least one container was updated", func(t *testing.T) {
				t.Parallel()

				t.Run("should send a report", func(t *testing.T) {
					t.Parallel()

					expected := `1 Scanned, 1 Updated, 0 Restarted, 0 Failed, 0 Fresh, 0 Skipped
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000`
					data := mockDataFromStates(t, session.UpdatedState)
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, data))
				})
			})
			t.Run("at least one container failed to update", func(t *testing.T) {
				t.Parallel()

				t.Run("should send a report", func(t *testing.T) {
					t.Parallel()

					expected := `1 Scanned, 0 Updated, 0 Restarted, 1 Failed, 0 Fresh, 0 Skipped
- fail1 (mock/fail1:latest): Failed: accidentally the whole container`
					data := mockDataFromStates(t, session.FailedState)
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, data))
				})
			})
			t.Run("containers are restarted due to dependencies", func(t *testing.T) {
				t.Parallel()

				t.Run("should send a report with restarted containers", func(t *testing.T) {
					t.Parallel()

					expected := `2 Scanned, 1 Updated, 1 Restarted, 0 Failed, 0 Fresh, 0 Skipped
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000
- rstr1 (mock/rstr1:latest): Restarted`
					data := mockDataFromStates(t, session.UpdatedState, session.RestartedState)
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, data))
				})
			})
			t.Run("mixing updated and restarted containers", func(t *testing.T) {
				t.Parallel()

				t.Run("should show different states for updated vs restarted", func(t *testing.T) {
					t.Parallel()

					expected := `3 Scanned, 2 Updated, 1 Restarted, 0 Failed, 0 Fresh, 0 Skipped
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000
- updt2 (mock/updt2:latest): 01d120000000 updated to d0a120000000
- rstr1 (mock/rstr1:latest): Restarted`
					data := mockDataFromStates(t,
						session.UpdatedState,
						session.RestartedState,
						session.UpdatedState,
					)
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, data))
				})
			})
			t.Run("testing JSON output format", func(t *testing.T) {
				t.Parallel()

				t.Run("should include restarted containers in JSON response", func(t *testing.T) {
					t.Parallel()

					data := mockDataFromStates(t, session.UpdatedState, session.RestartedState)
					jsonResult := getTemplatedResult(t, `{{ . | ToJSON }}`, false, data)

					var result map[string]any
					require.NoError(t, json.Unmarshal([]byte(jsonResult), &result))

					report, ok := result["report"].(map[string]any)
					assert.True(t, ok)

					updated, ok := report["updated"].([]any)
					assert.True(t, ok)
					require.Len(t, updated, 1)

					restarted, ok := report["restarted"].([]any)
					assert.True(t, ok)
					require.Len(t, restarted, 1)
					assert.Contains(t, restarted[0], "state")
					assert.Equal(t, "Restarted", restarted[0].(map[string]any)["state"])
				})
			})
			t.Run("the report is nil", func(t *testing.T) {
				t.Parallel()

				t.Run("should return the logged entries", func(t *testing.T) {
					t.Parallel()

					expected := `The situation is under control
All the smoke might be covering up some problems
Turns out everything is on fire
`
					assert.Equal(t, expected, getTemplatedResult(t, ``, false, mockDataMultipleEntries))
				})
			})
		})
	})

	t.Run("batching notifications", func(t *testing.T) {
		t.Parallel()

		t.Run("no messages are queued", func(t *testing.T) {
			t.Parallel()

			t.Run("should not send any notification", func(t *testing.T) {
				t.Parallel()

				log, logBuffer := newBufferLogger()

				shoutrrr := createTestNotifier(
					log,
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)
				shoutrrr.StartNotification(false)
				shoutrrr.SendNotification(nil)
				shoutrrr.Close()
				assert.NotContains(t, logBuffer.String(), `Shoutrrr:`)
			})
		})
		t.Run("at least one message is queued", func(t *testing.T) {
			t.Parallel()

			t.Run("should send a notification", func(t *testing.T) {
				t.Parallel()

				log, logBuffer := newBufferLogger()

				shoutrrr := createTestNotifier(
					log,
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)

				shoutrrr.RegisterHook(log)
				defer shoutrrr.Close()

				shoutrrr.StartNotification(false)
				log.Info().Msg("This log message is sponsored by ContainrrrVPN")
				shoutrrr.SendNotification(nil)
				require.Eventually(t, func() bool {
					return strings.Contains(logBuffer.String(), `Shoutrrr: This log message is sponsored by ContainrrrVPN`)
				}, 5*time.Second, 10*time.Millisecond)
			})
		})
	})

	t.Run("the title data field is empty", func(t *testing.T) {
		t.Parallel()

		t.Run("should not have set the title param", func(t *testing.T) {
			t.Parallel()

			shoutrrr := createTestNotifier(newTestLogger(t), []string{"logger://"}, allButTrace, true, StaticData{
				Host:  "test.host",
				Title: "",
			}, false, time.Second)
			_, found := shoutrrr.params.Title()
			assert.False(t, found)
		})
	})

	t.Run("sending notifications with error handling", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle index guard when errs length exceeds URLs length", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("test error"), errors.New("extra error")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Contains(t, logBuffer.String(), `index_mismatch`)
		})

		t.Run("should categorize authentication errors", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("unauthorized access")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Regexp(t, `failure_type.*authentication`, logBuffer.String())
		})

		t.Run("should categorize network errors", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("connection timeout")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Regexp(t, `failure_type.*network`, logBuffer.String())
		})

		t.Run("should categorize rate limit errors", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("too many requests")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Regexp(t, `failure_type.*rate_limit`, logBuffer.String())
		})

		t.Run("should categorize unknown errors", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("some unknown error")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Regexp(t, `failure_type.*unknown`, logBuffer.String())
		})

		t.Run("should log summary with failure counts", func(t *testing.T) {
			t.Parallel()

			log, logBuffer := newBufferLogger()

			mockRouter := &mockRouter{
				sendErrors: []error{errors.New("auth error"), errors.New("network error")},
			}

			shoutrrr := createTestNotifier(
				log,
				[]string{"logger://", "logger://"},
				allButTrace,
				true,
				StaticData{},
				false,
				time.Duration(0),
			)
			shoutrrr.Router = mockRouter
			shoutrrr.RegisterHook(log)
			shoutrrr.StartNotification(false)
			log.Info().Msg("test message")
			shoutrrr.SendNotification(nil)

			shoutrrr.Close()
			assert.Regexp(t, `failed_count.*2`, logBuffer.String())
		})
	})

	t.Run("closing the notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("Close() is called multiple times", func(t *testing.T) {
			t.Parallel()

			t.Run("should be idempotent and not panic", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)
				shoutrrr.RegisterHook(newTestLogger(t))

				// First close should work normally
				shoutrrr.Close()

				// Subsequent closes should be no-ops
				shoutrrr.Close()
				shoutrrr.Close()

				// Should not panic
			})
		})

		t.Run("Close() is called without starting the goroutine", func(t *testing.T) {
			t.Parallel()

			t.Run("should not panic or block", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)
				// Note: Not calling RegisterHook(), so no goroutine is started

				// Close should work without blocking
				shoutrrr.Close()

				// Should not panic
			})
		})

		t.Run("operations are attempted after Close()", func(t *testing.T) {
			t.Parallel()

			t.Run("should handle gracefully without panicking", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)
				shoutrrr.RegisterHook(newTestLogger(t))

				// Close the notifier
				shoutrrr.Close()

				// These operations should not panic after close
				shoutrrr.StartNotification(false)
				shoutrrr.SendNotification(nil)
				shoutrrr.SendNotification(nil)

				// Fire should still work (but may not send if channel is closed)
				entry := &notificationEntry{Message: "test"}
				shoutrrr.Run(nil, zerolog.InfoLevel, entry.Message)

				err := error(nil)
				require.NoError(t, err)

				// Should not panic
			})
		})

		t.Run("Close() is called concurrently", func(t *testing.T) {
			t.Parallel()

			t.Run("should handle concurrent calls safely", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					allButTrace,
					true,
					StaticData{},
					false,
					time.Duration(0),
				)
				shoutrrr.RegisterHook(newTestLogger(t))

				// Start multiple goroutines calling Close concurrently
				done := make(chan bool, 10)

				for range 10 {
					go func() {
						shoutrrr.Close()

						done <- true
					}()
				}

				// Wait for all to complete
				for range 10 {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Fatal("Close did not return")
					}
				}

				// Should not panic and all should complete
			})
		})
	})

	t.Run("ShouldSendNotification", func(t *testing.T) {
		t.Parallel()

		t.Run("notification level is error", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true when report has failed containers", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.ErrorLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FailedState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.True(t, result)
			})

			t.Run("should return false when report has no failed containers", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.ErrorLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FreshState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.False(t, result)
			})
		})

		t.Run("notification level is warn", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true regardless of report content", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.WarnLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FreshState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.True(t, result)
			})
		})

		t.Run("notification level is info", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true regardless of report content", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.InfoLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FreshState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.True(t, result)
			})
		})

		t.Run("notification level is debug", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true regardless of report content", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.DebugLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FreshState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.True(t, result)
			})
		})

		t.Run("notification level is trace", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true regardless of report content", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.TraceLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				mockReport := newProgressReport(session.FreshState)
				result := shoutrrr.ShouldSendNotification(mockReport)
				assert.True(t, result)
			})
		})

		t.Run("report is nil", func(t *testing.T) {
			t.Parallel()

			t.Run("should return true", func(t *testing.T) {
				t.Parallel()

				shoutrrr := createTestNotifier(
					newTestLogger(t),
					[]string{"logger://"},
					zerolog.ErrorLevel,
					false,
					StaticData{},
					false,
					time.Duration(0),
				)

				result := shoutrrr.ShouldSendNotification(nil)
				assert.True(t, result)
			})
		})
	})

	t.Run("deduplicating entries for grouped notifications", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty slice for empty input", func(t *testing.T) {
			t.Parallel()

			result := deduplicateEntries([]*notificationEntry{})
			assert.Empty(t, result)
		})

		t.Run("should return single entry unchanged", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "abc123"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 1)
		})

		t.Run("should deduplicate 'Found new image' entries with same image and new ID", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Found new image", Data: map[string]any{"container": "app-a", "image": "nginx:latest", "new_id": "abc123"}},
				{Message: "Found new image", Data: map[string]any{"container": "app-b", "image": "nginx:latest", "new_id": "abc123"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 1)
			assert.Equal(t, "app-a", result[0].Data["container"])
		})

		t.Run("should keep 'Found new image' entries with different images", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "abc123"}},
				{Message: "Found new image", Data: map[string]any{"image": "redis:latest", "new_id": "def456"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 2)
		})

		t.Run("should keep 'Found new image' entries with same image but different new IDs", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "abc123"}},
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "def456"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 2)
		})

		t.Run("should deduplicate 'Removing image' entries with same image ID", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Removing image", Data: map[string]any{"container_name": "app-a", "image_id": "sha256:abc"}},
				{Message: "Removing image", Data: map[string]any{"container_name": "app-b", "image_id": "sha256:abc"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 1)
			assert.Equal(t, "app-a", result[0].Data["container_name"])
		})

		t.Run("should not deduplicate other message types", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Stopping container", Data: map[string]any{"container": "app-a"}},
				{Message: "Stopping container", Data: map[string]any{"container": "app-b"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 2)
		})

		t.Run("should handle mixed entry types correctly", func(t *testing.T) {
			t.Parallel()

			entries := []*notificationEntry{
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "abc123"}},
				{Message: "Stopping container", Data: map[string]any{"container": "app-a"}},
				{Message: "Found new image", Data: map[string]any{"image": "nginx:latest", "new_id": "abc123"}},
				{Message: "Removing image", Data: map[string]any{"image_id": "sha256:old"}},
				{Message: "Started new container", Data: map[string]any{"container": "app-a"}},
				{Message: "Removing image", Data: map[string]any{"image_id": "sha256:old"}},
			}
			result := deduplicateEntries(entries)
			require.Len(t, result, 4)
			assert.Equal(t, "Found new image", result[0].Message)
			assert.Equal(t, "Stopping container", result[1].Message)
			assert.Equal(t, "Removing image", result[2].Message)
			assert.Equal(t, "Started new container", result[3].Message)
		})
	})
}

// TestSlowNotificationNotSent verifies that a message is not reported as sent
// while the router is still sending it.
func TestSlowNotificationNotSent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shoutrrr, blockingRouter, err := sendNotificationsWithBlockingRouter(t)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for all goroutines to be blocked
		synctest.Wait()

		// The notification should not be sent because the router is blocked
		select {
		case <-blockingRouter.sent:
			t.Fatal("expected notification not to be sent")
		default:
			// Good, channel is empty
		}

		// Cancel cannot interrupt an in-flight Router.Send. Unlock so Send returns
		// and the worker can observe cancellation and exit.
		shoutrrr.cancel()

		blockingRouter.unlock <- true

		<-shoutrrr.done
	})
}

// TestSlowNotificationSent verifies that a message is sent once the router
// finishes sending it.
func TestSlowNotificationSent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shoutrrr, blockingRouter, err := sendNotificationsWithBlockingRouter(t)
		if err != nil {
			t.Fatal(err)
		}

		// Unlock the router
		blockingRouter.unlock <- true

		// Wait for the notification to be sent
		synctest.Wait()

		// The notification should be sent
		select {
		case sent := <-blockingRouter.sent:
			if !sent {
				t.Fatal("expected notification to be sent")
			}
		default:
			t.Fatal("expected notification to be sent")
		}

		// Cancel to clean up
		shoutrrr.cancel()
		// Wait for goroutine to exit
		<-shoutrrr.done
	})
}

// TestGracefulTerminationNotificationGoroutine verifies that canceling the
// notifier's context ends an idle worker.
func TestGracefulTerminationNotificationGoroutine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shoutrrr := createTestNotifier(
			newTestLogger(t),
			[]string{"logger://"},
			allButTrace,
			true,
			StaticData{},
			true, // stdout
			time.Duration(0),
		)

		// Start the notification goroutine manually
		go sendNotifications(shoutrrr)

		// Cancel the context directly while goroutine is waiting in select
		shoutrrr.cancel()

		// Wait for the goroutine to finish (done channel should be signaled)
		synctest.Wait()

		// Verify done channel received
		select {
		case <-shoutrrr.done:
			// Good
		default:
			t.Fatal("expected done channel to be signaled")
		}
	})
}

// TestGracefulTerminationDuringMessageProcessing verifies that the worker
// finishes the message it is sending and then ends when the context is
// canceled.
func TestGracefulTerminationDuringMessageProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shoutrrr, blockingRouter, err := sendNotificationsWithBlockingRouter(t)
		if err != nil {
			t.Fatal(err)
		}

		// Unlock the router to allow the message processing to complete
		blockingRouter.unlock <- true

		// Wait for the notification to be sent
		synctest.Wait()

		// Verify that the message was sent
		select {
		case sent := <-blockingRouter.sent:
			if !sent {
				t.Fatal("expected message to be sent")
			}
		default:
			t.Fatal("expected message to be sent")
		}

		// Cancel context to test graceful termination
		shoutrrr.cancel()

		// Wait for goroutine to finish
		synctest.Wait()

		// Verify done channel signaled
		select {
		case <-shoutrrr.done:
			// Good
		default:
			t.Fatal("expected done channel to be signaled")
		}
	})
}

// TestContextCancellationIndependentOfStopChannel verifies that canceling the
// context ends the worker without closing the stop channel.
func TestContextCancellationIndependentOfStopChannel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shoutrrr := createTestNotifier(
			newTestLogger(t),
			[]string{"logger://"},
			allButTrace,
			true,
			StaticData{},
			true, // stdout
			time.Duration(0),
		)

		// Start the notification goroutine manually
		go sendNotifications(shoutrrr)

		// Test that context cancellation works without closing stop channel
		shoutrrr.cancel()

		// Wait for goroutine to finish via done channel
		synctest.Wait()

		// Verify done channel received
		select {
		case <-shoutrrr.done:
			// Good
		default:
			t.Fatal("expected done channel to be signaled")
		}

		// Verify stop channel is still open (not closed by context cancellation)
		select {
		case <-shoutrrr.stop:
			t.Fatal("stop channel should not be closed by context cancellation")
		default:
			// Good, stop channel is still open
		}
	})
}

// mockRouter implements the router interface for testing error scenarios.
type mockRouter struct {
	sendErrors []error
}

func (m *mockRouter) Send(_ string, _ *shoutrrrTypes.Params) []error {
	return m.sendErrors
}

// blockingRouter simulates a notification router with blocking behavior for testing.
// It waits for an unlock signal before sending and signals completion via a channel.
type blockingRouter struct {
	unlock chan bool
	sent   chan bool
	ctx    context.Context //nolint:containedctx
}

func (b blockingRouter) Send(_ string, _ *shoutrrrTypes.Params) []error {
	select {
	case <-b.unlock:
		b.sent <- true
	case <-b.ctx.Done():
		// canceled, don't send
	}

	return nil
}

// sendNotificationsWithBlockingRouter creates a notifier with a blocking router for testing.
// It queues a message and returns the notifier and router to verify notification delays.
//
// TODO: Remove legacy template usage when legacy notification types are removed.
//
//nolint:godox
func sendNotificationsWithBlockingRouter(t *testing.T) (*shoutrrrTypeNotifier, *blockingRouter, error) {
	t.Helper()

	legacy := true
	ctx, cancel := context.WithCancel(context.Background())

	router := &blockingRouter{
		unlock: make(chan bool, 1),
		sent:   make(chan bool, 1),
		ctx:    ctx,
	}

	tpl, err := getShoutrrrTemplate(newTestLogger(t), "", legacy)
	if err != nil {
		cancel()

		return nil, nil, err
	}

	shoutrrr := &shoutrrrTypeNotifier{
		template:       tpl,
		messages:       make(chan string, 1),
		done:           make(chan struct{}),
		Router:         router,
		legacyTemplate: legacy,
		params:         &shoutrrrTypes.Params{},
		ctx:            ctx,
		cancel:         cancel,
		delay:          time.Duration(0),
	}

	entry := &notificationEntry{
		Message: "foo bar",
	}

	go sendNotifications(shoutrrr)

	shoutrrr.StartNotification(false)
	// Enqueue directly: Run(nil, ...) is fail-closed (cannot parse notify field).
	// These tests exercise the send worker, not hook field extraction.
	shoutrrr.entriesMutex.Lock()
	shoutrrr.entries = append(shoutrrr.entries, entry)
	shoutrrr.entriesMutex.Unlock()
	shoutrrr.SendNotification(nil)

	return shoutrrr, router, nil
}

// createNotifierWithTemplate creates a notifier with a specified template for testing.
// It returns the notifier and an error, falling back to a default template if parsing fails.
//
// TODO: Remove legacy parameter and default-legacy fallback when legacy notification types are removed.
//
//nolint:godox
func createNotifierWithTemplate(t *testing.T, tplString string, legacy bool) (*shoutrrrTypeNotifier, error) {
	t.Helper()

	tpl, err := getShoutrrrTemplate(newTestLogger(t), tplString, legacy)
	if err != nil {
		_ = err // template construction error ignored for this helper

		tplBase := template.New("").Funcs(templateFuncs)

		defaultKey := "default"
		if legacy {
			defaultKey = "default-legacy"
		}

		tpl = template.Must(tplBase.Parse(commonTemplates[defaultKey]))
		// Do not reset err.
		// Keep it to indicate the original parsing failure.
	}

	return &shoutrrrTypeNotifier{
		template:       tpl,
		legacyTemplate: legacy,
	}, err
}

// getTemplatedResult generates a templated message for testing.
// It builds and returns the message string, expecting no errors.
//
// TODO: Remove legacy parameter when legacy notification types are removed.
//
//nolint:godox
func getTemplatedResult(t *testing.T, tplString string, legacy bool, data notificationData) string {
	t.Helper()

	notifier, err := createNotifierWithTemplate(t, tplString, legacy)
	require.NoError(t, err)
	msg, err := notifier.buildMessage(data)
	require.NoError(t, err)

	return msg
}

// TestDefaultLegacyGitMessages verifies how the default legacy template
// renders Git monitoring messages.
func TestDefaultLegacyGitMessages(t *testing.T) {
	t.Parallel()

	tpl := template.Must(template.New("git").Funcs(templateFuncs).Parse(commonTemplates["default-legacy"]))

	var buf bytes.Buffer

	err := tpl.Execute(&buf, []*notificationEntry{
		{
			Message: "Found new Git revision",
			Data: map[string]any{
				"revision":     "git.example.com/org/watchtower-git-support-test@v1.2.1",
				"short_commit": "7ab67c773bf4",
			},
		},
		{
			Message: "Built image from Git URL context",
			Data: map[string]any{
				"image":    "wtgit-semver:git-7ab67c773bf4",
				"image_id": "27c196745ab8",
			},
		},
		{
			Message: "Stopping container",
			Data:    map[string]any{"container": "wtgit-semver", "id": "76caeb3ef2ce"},
		},
		{
			Message: "Started new container",
			Data:    map[string]any{"container": "wtgit-semver", "new_id": "0f492bb11f3b"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, strings.Join([]string{
		"Found new Git revision: git.example.com/org/watchtower-git-support-test@v1.2.1 (7ab67c773bf4)",
		"Built image: wtgit-semver:git-7ab67c773bf4 (27c196745ab8)",
		"Stopped stale container: wtgit-semver (76caeb3ef2ce)",
		"Started new container: wtgit-semver (0f492bb11f3b)",
	}, "\n"), strings.TrimSpace(buf.String()))
}

// TestDefaultLegacyGitSkipAndComposeMessages verifies how the default legacy
// template renders Git skip and Compose messages.
func TestDefaultLegacyGitSkipAndComposeMessages(t *testing.T) {
	t.Parallel()

	tpl := template.Must(template.New("git").Funcs(templateFuncs).Parse(commonTemplates["default-legacy"]))

	cases := []struct {
		name    string
		message string
		data    map[string]any
		want    string
	}{
		{
			name:    "compose build only",
			message: "Built Compose project",
			data:    map[string]any{"project": "wtgitweb", "service": "api"},
			want:    "Built Compose project: wtgitweb (api)",
		},
		{
			name:    "missing compose dir",
			message: "Compose project directory is not readable. Leaving the running container untouched",
			data: map[string]any{
				"container": "wtgit-missing",
				"error":     "compose project directory is not readable: /srv/missing",
				"image":     "wtgit-app:latest",
			},
			want: "Skipped wtgit-missing: compose directory is not readable",
		},
		{
			name:    "invalid policy",
			message: "Skipped container with an invalid git semver policy",
			data:    map[string]any{"container": "wtgit-app"},
			want:    "Skipped wtgit-app: invalid git semver policy",
		},
		{
			name:    "invalid host",
			message: "Skipped container with an invalid git-host",
			data:    map[string]any{"container": "wtgit-app"},
			want:    "Skipped wtgit-app: invalid git-host",
		},
		{
			name:    "rate limit retries exhausted",
			message: "Registry rate limit retries exhausted. Container failed for this cycle",
			data: map[string]any{
				"container": "sonarr",
				"error":     "image pull: registry rate limited: retry-after 347.256µs allowed 44000 per 1m0s",
				"image":     "lscr.io/linuxserver/sonarr:latest",
			},
			want: "Update failed for sonarr (lscr.io/linuxserver/sonarr:latest): image pull: registry rate limited: retry-after 347.256µs allowed 44000 per 1m0s",
		},
		{
			name:    "depends-on entry that names no container",
			message: "Ignoring depends-on entry that names no container",
			data:    map[string]any{"container": "web", "depends_on": "missing-db"},
			want:    "Ignored depends-on missing-db for web: no container has that name",
		},
		{
			name:    "self-dependency",
			message: "Skipping container update (self-dependency)",
			data:    map[string]any{"container": "app", "id": "abcdef123456"},
			want:    "Skipped app: depends on itself",
		},
		{
			name:    "circular dependency",
			message: "Skipping container update (circular dependency)",
			data:    map[string]any{"container": "app", "id": "abcdef123456"},
			want:    "Skipped app: circular dependency",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			err := tpl.Execute(&buf, []*notificationEntry{{Message: tc.message, Data: tc.data}})
			require.NoError(t, err)
			assert.Equal(t, tc.want, strings.TrimSpace(buf.String()))
			assert.NotContains(t, buf.String(), "|")
		})
	}
}

// TestShutdownGracePeriodConstant verifies that the shutdownGracePeriod constant is set to 50ms.
func TestShutdownGracePeriodConstant(t *testing.T) {
	expectedGracePeriod := 50 * time.Millisecond
	if shutdownGracePeriod != expectedGracePeriod {
		t.Fatalf("expected shutdownGracePeriod to be %v, got %v", expectedGracePeriod, shutdownGracePeriod)
	}
}

// TestCloseDoesNotHangWithBlockingRouter verifies that Close() completes without hanging
// when the router is blocked. This tests that the context cancellation properly unblocks
// the send call.
func TestCloseDoesNotHangWithBlockingRouter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a notifier with a blocking router
		shoutrrr, blockingRouter, err := sendNotificationsWithBlockingRouter(t)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for goroutine to be blocked
		synctest.Wait()

		// Verify router is blocked (not yet sent)
		select {
		case <-blockingRouter.sent:
			t.Fatal("message should not have been sent yet")
		default:
			// Good - router is blocked
		}

		// Close should complete without hanging - use a goroutine since it may block briefly
		closeDone := make(chan error, 1)

		go func() {
			shoutrrr.Close()

			closeDone <- nil
		}()

		// Wait for Close to complete (should be fast with synctest)
		select {
		case <-closeDone:
			// Good - Close completed
			t.Log("Close() completed successfully")
		case <-time.After(2 * time.Second):
			t.Fatal("Close() hung - context cancellation did not unblock the goroutine")
		}

		// Verify context was canceled
		select {
		case <-shoutrrr.ctx.Done():
			t.Log("Context was canceled")
		default:
			t.Fatal("expected context to be canceled")
		}

		// Clean up: unlock the router so the goroutine can exit gracefully
		blockingRouter.unlock <- true

		// Wait for the done channel to be signaled
		synctest.Wait()

		select {
		case <-shoutrrr.done:
			t.Log("Done channel was signaled")
		default:
			t.Fatal("expected done channel to be signaled")
		}
	})
}

// controlledRouter simulates a router that can be controlled via channels for testing.
// It waits for a continue signal before sending, allowing deterministic testing.
type controlledRouter struct {
	continueCh chan struct{}
	sent       chan bool
	ctx        context.Context //nolint:containedctx
}

func (c *controlledRouter) Send(_ string, _ *shoutrrrTypes.Params) []error {
	// Signal that we're waiting
	// Wait for continue signal or context cancellation
	select {
	case <-c.continueCh:
		c.sent <- true
	case <-c.ctx.Done():
		// canceled, don't send
	}

	return nil
}

// TestGracePeriodAllowsInFlightMessages verifies that in-flight messages have time to complete
// before context is canceled during the shutdown grace period.
func TestGracePeriodAllowsInFlightMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a controlled router
		ctx, cancel := context.WithCancel(context.Background())

		controlledR := &controlledRouter{
			continueCh: make(chan struct{}, 1),
			sent:       make(chan bool, 1),
			ctx:        ctx,
		}

		tpl, err := getShoutrrrTemplate(newTestLogger(t), "", true)
		if err != nil {
			cancel()
			t.Fatal(err)
		}

		shoutrrr := &shoutrrrTypeNotifier{
			template:       tpl,
			messages:       make(chan string, 1),
			done:           make(chan struct{}),
			stop:           make(chan struct{}),
			Router:         controlledR,
			legacyTemplate: true,
			params:         &shoutrrrTypes.Params{},
			ctx:            ctx,
			cancel:         cancel,
			delay:          time.Duration(0),
			receiving:      atomic.Bool{},
		}
		shoutrrr.receiving.Store(true)

		// Start the notification goroutine
		go sendNotifications(shoutrrr)

		// Queue a message
		shoutrrr.messages <- "test message"

		// Wait for goroutine to be blocked in router.Send
		synctest.Wait()

		// Now call Close - this should trigger the grace period wait
		// We let it proceed by signaling continueCh in a separate goroutine
		go func() {
			// Wait a bit to simulate the grace period passing
			synctest.Wait()
			// Then allow the message to be sent
			controlledR.continueCh <- struct{}{}
		}()

		// Close should complete
		shoutrrr.Close()

		// Wait for done channel
		synctest.Wait()

		// Note: Close() already waits for done channel internally (line 500 in shoutrrr.go)
		// so we don't need to wait for it here. Instead, we verify the message was sent.

		// Verify the message was actually sent by checking controlledR.sent
		timeout := 2 * time.Second
		select {
		case sent := <-controlledR.sent:
			if !sent {
				t.Fatal("expected message to be sent during grace period")
			}

			t.Log("Message was successfully sent during grace period")
		case <-time.After(timeout):
			t.Fatalf("expected sent channel to be signaled within %v timeout", timeout)
		}
	})
}

// TestSendNotifications_SendsQueuedMessagesAfterStop verifies that the worker
// sends every message still queued when the stop signal arrives. The worker
// picks at random between a queued message and the stop signal, so with many
// messages queued it reaches the shutdown drain with near certainty, and each
// message must be sent on either path.
func TestSendNotifications_SendsQueuedMessagesAfterStop(t *testing.T) {
	t.Parallel()

	const queued = 64

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	router := &countingRouter{}
	notifier := &shoutrrrTypeNotifier{
		messages: make(chan string, queued),
		done:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		Router:   router,
		params:   &shoutrrrTypes.Params{},
		ctx:      ctx,
		cancel:   cancel,
		localLog: newTestLogger(t),
	}

	for i := range queued {
		notifier.messages <- fmt.Sprintf("message %d", i)
	}

	close(notifier.stop)

	sendNotifications(notifier)

	assert.Equal(t, int32(queued), router.sends.Load())
}

// TestCloseWithNoGoroutine verifies that Close() works correctly when the
// notification goroutine was never started.
func TestCloseWithNoGoroutine(t *testing.T) {
	shoutrrr := createTestNotifier(
		newTestLogger(t),
		[]string{"logger://"},
		allButTrace,
		true,
		StaticData{},
		false,
		time.Duration(0),
	)
	// Note: Not calling RegisterHook(), so no goroutine is started

	// Close should complete immediately without blocking
	// Use goroutine+channel+select pattern to avoid flaky wall-clock timing.
	// Close() has a shutdownGracePeriod (50ms) wait, so we use 1s timeout
	// which is comfortably larger than 2x shutdownGracePeriod.
	closeDone := make(chan struct{})

	go func() {
		shoutrrr.Close()

		closeDone <- struct{}{}
	}()

	// Select with timeout - Close() should complete within reasonable time
	select {
	case <-closeDone:
		t.Log("Close() completed successfully with no goroutine")
	case <-time.After(1 * time.Second):
		t.Fatalf("Close() took too long without goroutine (timeout exceeded)")
	}
}

// TestCreateNotifier_FatalsOnBadURL verifies that createNotifier calls Fatal
// when given a docker-secret file path instead of a valid URL.
// The fatal path is tested via subprocess because zerolog.Fatal calls os.Exit.
func TestCreateNotifier_FatalsOnBadURL(t *testing.T) {
	if os.Getenv(shoutrrrFatalHelperEnv) != "" {
		// Child process: expect createNotifier to fatal on invalid URL.
		log := zerolog.Nop()
		createNotifier(
			&log,
			[]string{"docker-secret:/run/secrets/my_url"},
			zerolog.InfoLevel,
			"",
			true,
			StaticData{},
			false,
			0,
		)

		// If we reach here, the fatal did not exit.
		t.Fatal("expected createNotifier to fatal on bad URL")
	}

	// Parent process: run child and assert non-zero exit.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCreateNotifier_FatalsOnBadURL$")

	cmd.Env = append(os.Environ(), shoutrrrFatalHelperEnv+"=1")
	out, err := cmd.CombinedOutput()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("timed out waiting for createNotifier fatal path; output:\n%s", string(out))
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit for bad URL; output:\n%s", string(out))
	}
}

// TestLevelToString_WarnMapsToWarning ensures WarnLevel renders as the legacy
// template string "warning" so custom templates comparing .Level stay compatible.
func TestLevelToString_WarnMapsToWarning(t *testing.T) {
	t.Parallel()

	require.Equal(t, "warning", levelToString(zerolog.WarnLevel))
	require.Equal(t, "info", levelToString(zerolog.InfoLevel))
	require.Equal(t, "error", levelToString(zerolog.ErrorLevel))

	// Non-legacy template path: compare .Level with the legacy "warning" string.
	const tpl = `{{ range .Entries }}{{ if eq .Level "warning" }}hit{{ end }}{{ end }}`

	data := notificationData{
		Entries: []*notificationEntry{
			{Level: levelToString(zerolog.WarnLevel), Message: "caution"},
		},
	}
	notifier, err := createNotifierWithTemplate(t, tpl, false)
	require.NoError(t, err)

	result, err := notifier.buildMessage(data)
	require.NoError(t, err)
	require.Equal(t, "hit", result)
}

// TestRun_EventFieldMapPreservesLargeIntegers ensures large integer fields such as
// removed_orchestrators render exactly without float precision loss or scientific notation.
func TestRun_EventFieldMapPreservesLargeIntegers(t *testing.T) {
	t.Parallel()

	root := zerolog.New(io.Discard).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	n := createTestNotifier(
		newTestLogger(t),
		[]string{},
		zerolog.TraceLevel,
		true,
		StaticData{},
		false,
		0,
	)
	n.RegisterHook(&root)
	n.StartNotification(true)

	// Value exceeds float64 exact-integer range (2^53) so json.Unmarshal float path would corrupt it.
	const largeCount int64 = 9007199254740993

	root.Info().
		Int64("removed_orchestrators", largeCount).
		Msg("cleanup summary")

	n.entriesMutex.RLock()
	entries := append([]*notificationEntry(nil), n.entries...)
	n.entriesMutex.RUnlock()

	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].Data)

	got, ok := entries[0].Data["removed_orchestrators"].(int64)
	require.True(t, ok, "removed_orchestrators must be int64, got %T", entries[0].Data["removed_orchestrators"])
	require.Equal(t, largeCount, got)

	// Template must render the exact decimal digits (no scientific notation).
	const tpl = `{{ range .Entries }}{{ index .Data "removed_orchestrators" }}{{ end }}`

	notifier, err := createNotifierWithTemplate(t, tpl, false)
	require.NoError(t, err)

	rendered, err := notifier.buildMessage(notificationData{Entries: entries})
	require.NoError(t, err)
	require.Equal(t, "9007199254740993", rendered)

	n.Close()
}

// TestRun_EventFieldMapPreservesApplicationFields verifies eventFieldMap through
// the notification emission path: a known application field on a zerolog event
// is retained on notificationEntry.Data. Guards against silent drops when
// parsing Event.buf fails (fail-closed would leave the queue empty).
func TestRun_EventFieldMapPreservesApplicationFields(t *testing.T) {
	t.Parallel()

	root := zerolog.New(io.Discard).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	n := createTestNotifier(
		newTestLogger(t),
		[]string{},
		zerolog.TraceLevel,
		true,
		StaticData{},
		false,
		0,
	)
	n.RegisterHook(&root)
	n.StartNotification(true)

	const (
		wantContainer = "app-container-1"
		wantImage     = "nginx:latest"
		wantMsg       = "Found new image"
	)

	root.Info().
		Str("container", wantContainer).
		Str("image", wantImage).
		Msg(wantMsg)

	n.entriesMutex.RLock()
	entries := append([]*notificationEntry(nil), n.entries...)
	n.entriesMutex.RUnlock()

	require.Len(t, entries, 1, "event must be retained when Event.buf parses successfully")
	require.Equal(t, wantMsg, entries[0].Message)
	require.NotNil(t, entries[0].Data, "application fields must populate Data")
	require.Equal(t, wantContainer, entries[0].Data["container"])
	require.Equal(t, wantImage, entries[0].Data["image"])
	// Envelope keys are stripped by eventFieldMap.
	_, hasLevel := entries[0].Data[zerolog.LevelFieldName]
	_, hasTime := entries[0].Data[zerolog.TimestampFieldName]
	_, hasMessage := entries[0].Data[zerolog.MessageFieldName]

	require.False(t, hasLevel)
	require.False(t, hasTime)
	require.False(t, hasMessage)

	n.Close()
}

// TestRun_NotifyNoAndFailClosed verifies loop-prevention and fail-closed field extraction.
func TestRun_NotifyNoAndFailClosed(t *testing.T) {
	t.Parallel()

	n := createTestNotifier(
		newTestLogger(t),
		[]string{},
		zerolog.TraceLevel,
		true,
		StaticData{},
		false,
		0,
	)
	// Enable batching without starting the worker goroutine.
	n.entries = make([]*notificationEntry, 0, initialEntriesCapacity)

	// Fail-closed: nil event must not enqueue.
	n.Run(nil, zerolog.InfoLevel, "should not queue")
	n.entriesMutex.RLock()
	require.Empty(t, n.entries)
	n.entriesMutex.RUnlock()

	// Real zerolog path: notify=no child must not enqueue.
	root := zerolog.New(io.Discard).Level(zerolog.TraceLevel)
	n.RegisterHook(&root)
	// After RegisterHook the worker runs. Still batch via StartNotification.
	n.StartNotification(true)

	noLog := root.With().Str("notify", "no").Logger()
	noLog.Info().Msg("internal must not notify")

	n.entriesMutex.RLock()
	require.Empty(t, n.entries, "notify=no events must not be queued")
	n.entriesMutex.RUnlock()

	// Application log without notify=no should enqueue.
	root.Info().Str("container", "c1").Msg("Found new image")

	// Allow hook to run synchronously (hooks run in emitting goroutine).
	n.entriesMutex.RLock()
	count := len(n.entries)
	n.entriesMutex.RUnlock()
	require.Equal(t, 1, count, "app log without notify=no should queue")

	n.Close()
}

// TestRun_ConcurrentEnqueue verifies concurrent application logs are all queued
// (no global single-flight gate that would drop parallel Runs).
func TestRun_ConcurrentEnqueue(t *testing.T) {
	t.Parallel()

	root := zerolog.New(io.Discard).Level(zerolog.TraceLevel)
	n := createTestNotifier(
		newTestLogger(t),
		[]string{},
		zerolog.TraceLevel,
		true,
		StaticData{},
		false,
		0,
	)
	n.RegisterHook(&root)
	n.StartNotification(true)

	const workers = 32

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(i int) {
			defer wg.Done()

			root.Info().Int("i", i).Msg("concurrent event")
		}(i)
	}

	wg.Wait()

	n.entriesMutex.RLock()
	count := len(n.entries)
	n.entriesMutex.RUnlock()
	require.Equal(t, workers, count, "every concurrent log must enqueue")

	n.Close()
}

// TestSendNotification_SingleFocusKeepsBatchingActive verifies that when every
// queued entry matches the single-container focus, n.entries remains a non-nil
// empty slice so subsequent Run calls continue batching.
func TestSendNotification_SingleFocusKeepsBatchingActive(t *testing.T) {
	n := createTestNotifier(
		newTestLogger(t),
		[]string{"logger://"},
		zerolog.InfoLevel,
		true,
		StaticData{},
		false,
		0,
	)

	full := newProgressReport(session.UpdatedState)
	require.NotEmpty(t, full.Updated())

	updated := full.Updated()[0]
	report := &session.SingleContainerReport{
		UpdatedReports: []types.ContainerReport{updated},
	}

	n.StartNotification(true)
	n.entriesMutex.Lock()
	n.entries = []*notificationEntry{
		{
			Message: "Found new image",
			Data:    map[string]any{"container": updated.Name()},
		},
	}
	n.entriesMutex.Unlock()

	n.SendNotification(report)

	n.entriesMutex.RLock()
	entries := n.entries
	n.entriesMutex.RUnlock()

	require.NotNil(t, entries, "batching slice must remain non-nil after single-focus drain")
	require.Empty(t, entries)

	n.Close()
}

// TestCreateNotifier_AcceptsGotifyURLFromFileExpansion verifies that createNotifier
// succeeds when given a valid Gotify URL matching expanded secret content.
func TestCreateNotifier_AcceptsGotifyURLFromFileExpansion(t *testing.T) {
	notifier := createTestNotifier(
		newTestLogger(t),
		[]string{"gotify://gotify.example.com/token123"},
		zerolog.InfoLevel,
		true,
		StaticData{},
		false,
		0,
	)

	require.NotNil(t, notifier)
}

// TestReportCategoryCount verifies that reportCategoryCount adds up the
// scanned, updated, failed, skipped, and restarted containers, and is zero for
// a nil report.
func TestReportCategoryCount(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, reportCategoryCount(nil))

	report := categoryCountReport{
		scanned:   make([]types.ContainerReport, 2),
		updated:   make([]types.ContainerReport, 1),
		restarted: make([]types.ContainerReport, 1),
	}
	assert.Equal(t, 4, reportCategoryCount(report))
}

// TestSend_CanceledBeforeSend verifies that send does not call the router once
// the notifier's context is canceled.
func TestSend_CanceledBeforeSend(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	router := &countingRouter{}
	notifier := &shoutrrrTypeNotifier{
		Router: router,
		ctx:    ctx,
		cancel: cancel,
	}

	notifier.send("should not send")
	assert.Equal(t, int32(0), router.sends.Load())
}

// TestEventFieldMap_NilEvent verifies that eventFieldMap reports no fields for
// a nil event.
func TestEventFieldMap_NilEvent(t *testing.T) {
	t.Parallel()

	fields, _, ok := eventFieldMap(nil)
	assert.False(t, ok)
	assert.Nil(t, fields)
}

// TestEventFieldMap_NumbersAndTimestamp verifies that eventFieldMap reads
// numeric fields from an event whose timestamp field is not a valid time.
func TestEventFieldMap_NumbersAndTimestamp(t *testing.T) {
	t.Parallel()

	var captured map[string]any

	hook := zerolog.HookFunc(func(event *zerolog.Event, _ zerolog.Level, _ string) {
		fields, _, ok := eventFieldMap(event)
		if ok {
			captured = fields
		}
	})

	log := zerolog.New(io.Discard).Hook(hook)
	log.Info().
		Int64("count", 42).
		Float64("ratio", 1.5).
		Str(zerolog.TimestampFieldName, "not-a-timestamp").
		Msg("numbers")

	require.NotNil(t, captured)
	assert.Equal(t, int64(42), captured["count"])
	assert.InDelta(t, 1.5, captured["ratio"], 0.001)
}

type categoryCountReport struct {
	scanned, updated, failed, skipped, stale, fresh, restarted []types.ContainerReport
}

func (r categoryCountReport) Scanned() []types.ContainerReport   { return r.scanned }
func (r categoryCountReport) Updated() []types.ContainerReport   { return r.updated }
func (r categoryCountReport) Failed() []types.ContainerReport    { return r.failed }
func (r categoryCountReport) Skipped() []types.ContainerReport   { return r.skipped }
func (r categoryCountReport) Stale() []types.ContainerReport     { return r.stale }
func (r categoryCountReport) Fresh() []types.ContainerReport     { return r.fresh }
func (r categoryCountReport) Restarted() []types.ContainerReport { return r.restarted }
func (r categoryCountReport) All() []types.ContainerReport       { return nil }

type countingRouter struct {
	sends atomic.Int32
}

func (c *countingRouter) Send(_ string, _ *shoutrrrTypes.Params) []error {
	c.sends.Add(1)

	return nil
}

// TestCreateNotifier_InvalidTemplateFallsBackToDefault verifies that a
// notification template that fails to parse is replaced by the default
// template, so notifications render as with no template set instead of
// failing when the first notification is sent.
func TestCreateNotifier_InvalidTemplateFallsBackToDefault(t *testing.T) {
	data := goldenFixtures()["full"]

	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			invalid := createNotifier(newTestLogger(t), []string{}, zerolog.TraceLevel, "{{ .Broken", legacy, StaticData{}, false, 0)
			t.Cleanup(invalid.Close)

			fallback := createNotifier(newTestLogger(t), []string{}, zerolog.TraceLevel, "", legacy, StaticData{}, false, 0)
			t.Cleanup(fallback.Close)

			want, err := fallback.buildMessage(data)
			require.NoError(t, err)

			got, err := invalid.buildMessage(data)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}
