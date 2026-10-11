package notifications

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// goldenHost is the hostname used in every golden notification fixture.
const goldenHost = "golden-host"

// goldenEntryTime is the fixed timestamp of every golden log entry.
var goldenEntryTime = time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)

// Errors attached to the failed and skipped containers in the golden report.
var (
	errGoldenFailure = errors.New("pull access denied for org/fail1")
	errGoldenSkipped = errors.New("container is not running")
)

// legacyTemplateMessages lists every log message the default-legacy template
// formats specially, so each of its branches appears in the golden output.
var legacyTemplateMessages = []string{
	"Found new image",
	"Found new Git revision",
	"Stopping container",
	"Started new container",
	"Stopping linked container",
	"Started linked container",
	"Container updated",
	"Removing image",
	"Failed to list containers for image usage check, skipping removal",
	"Image is within cooldown period - not eligible for update",
	"Image age exceeds cooldown - eligible for update",
	"Image creation time unavailable - update check unavailable",
	"Detected multiple Watchtower instances - initiating cleanup",
	"Successfully removed all excess Watchtower containers",
	"Docker image usage budget enabled",
	"Docker image usage exceeds configured maximum",
	"Docker image usage exceeds configured warning threshold",
	"Failed to query Docker image disk usage",
	"Changelog",
	"Built image from Git URL context",
	"Built Compose project",
	"Git build failed. Leaving running container untouched",
	"Compose apply failed. Docker Compose may have partially recreated services",
	"Compose apply finished with unconfirmed service results",
	"Compose apply result omitted this service. Runtime state is unknown",
	"Compose apply did not return an identifiable instance for this replica. Runtime state is unknown",
	"Compose apply skipped. Container has no compose service label",
	"Compose batch dependencies are cyclic. Appending unresolved batches in first-seen order",
	"Compose project directory is not readable. Leaving the running container untouched",
	"Compose update failed before container replacement",
	"Could not dependency-sort Compose batches. Using the existing container order",
	"Could not order Compose batches by dependencies. Using first-seen order",
	"Skipped container with an invalid git-host",
	"Skipped container with an invalid git semver policy",
	"Skipping Watchtower self-update in run-once mode",
	"Only checking containers in scope",
	"Starting HTTP API server",
	"HTTP API server is enabled",
	"Registry rate limit retries exhausted. Container failed for this cycle",
	"Ignoring depends-on entry that names no container",
	"Skipping container update (self-dependency)",
	"Skipping container update (circular dependency)",
}

// Disk sizes used by the image-usage budget fixtures, in bytes.
const (
	goldenDiskMax         = int64(10 << 30)
	goldenDiskWarn        = int64(8 << 30)
	goldenDiskReclaimable = int64(2 << 30)
)

// goldenEntryData returns the structured fields attached to a golden log entry
// for message. Each message carries the fields its emitting code attaches, with
// values of the types that code uses, and the disk-usage entries carry usage
// consistent with the threshold they report.
func goldenEntryData(message string) map[string]any {
	diskUsage := func(usage int64) map[string]any {
		return map[string]any{
			"usage":       usage,
			"max":         goldenDiskMax,
			"warn":        goldenDiskWarn,
			"reclaimable": goldenDiskReclaimable,
			"image_count": int64(3),
		}
	}

	switch message {
	case "Found new image":
		return map[string]any{"container": "app", "image": "org/app:latest", "new_id": "d0a110000000"}
	case "Found new Git revision":
		return map[string]any{
			"container":    "app",
			"revision":     "github.com/org/app@v1.2.0",
			"short_commit": "0123456",
		}
	case "Stopping container", "Stopping linked container":
		return map[string]any{"container": "app", "id": "c79110000000"}
	case "Started new container", "Started linked container":
		return map[string]any{"container": "app", "new_id": "d0a110000000"}
	case "Container updated":
		return map[string]any{
			"container": "app",
			"image":     "org/app:latest",
			"old_id":    "01d110000000",
			"new_id":    "d0a110000000",
		}
	case "Removing image":
		return map[string]any{
			"container_name": "app",
			"image_name":     "org/app:latest",
			"image_id":       "01d110000000",
		}
	case "Failed to list containers for image usage check, skipping removal":
		return map[string]any{
			"image_name": "org/app:latest",
			"image_id":   "01d110000000",
			"error":      "cannot connect to the Docker daemon",
		}
	case "Image is within cooldown period - not eligible for update":
		return map[string]any{
			"image":       "org/app:latest",
			"image_age":   "2h",
			"cooldown":    "1d",
			"eligible_in": "22h",
			"eligible_at": "2026-10-08T10:00:00Z",
		}
	case "Image age exceeds cooldown - eligible for update":
		return map[string]any{"image": "org/app:latest", "image_age": "2d", "cooldown": "1d"}
	case "Image creation time unavailable - update check unavailable":
		return map[string]any{
			"image":    "org/app:latest",
			"cooldown": "1d",
			"error":    "manifest unknown",
		}
	case "Detected multiple Watchtower instances - initiating cleanup":
		return map[string]any{"count": 2}
	case "Successfully removed all excess Watchtower containers":
		return map[string]any{"removed_instances": 1}
	case "Docker image usage budget enabled":
		return map[string]any{"disk_space_max": goldenDiskMax, "disk_space_warn": goldenDiskWarn}
	case "Docker image usage exceeds configured maximum":
		return diskUsage(int64(11 << 30))
	case "Docker image usage exceeds configured warning threshold":
		return diskUsage(int64(9 << 30))
	case "Failed to query Docker image disk usage":
		return map[string]any{"error": "daemon disk usage unavailable"}
	case "Changelog":
		return map[string]any{
			"container": "app",
			"changelog": "https://github.com/org/app/releases/tag/v1.2.0",
		}
	case "Built image from Git URL context":
		return map[string]any{"container": "app", "image": "org/app:latest", "image_id": "b17d10000000"}
	case "Built Compose project":
		return map[string]any{"project": "stack", "service": "web"}
	case "Compose apply finished with unconfirmed service results":
		return map[string]any{"project": "stack", "service": "web, worker"}
	case "Compose apply result omitted this service. Runtime state is unknown",
		"Compose apply did not return an identifiable instance for this replica. Runtime state is unknown":
		return map[string]any{"container": "stack-web-1", "image": "org/web:latest", "service": "web"}
	case "Git build failed. Leaving running container untouched",
		"Compose project directory is not readable. Leaving the running container untouched":
		return map[string]any{"container": "app", "image": "org/app:latest", "error": "example failure"}
	case "Compose apply failed. Docker Compose may have partially recreated services",
		"Compose update failed before container replacement":
		return map[string]any{
			"container": "stack-web-1",
			"image":     "org/web:latest",
			"dir":       "/srv/stack",
			"error":     "example failure",
		}
	case "Compose apply skipped. Container has no compose service label",
		"Skipped container with an invalid git-host",
		"Skipped container with an invalid git semver policy",
		"Skipping Watchtower self-update in run-once mode":
		return map[string]any{"container": "app", "image": "org/app:latest"}
	case "Could not dependency-sort Compose batches. Using the existing container order",
		"Could not order Compose batches by dependencies. Using first-seen order":
		return map[string]any{"error": "circular reference detected"}
	case "Registry rate limit retries exhausted. Container failed for this cycle":
		return map[string]any{
			"container": "app",
			"image":     "org/app:latest",
			"error":     "image pull: registry rate limited: retry-after 347.256µs allowed 44000 per 1m0s",
		}
	case "Ignoring depends-on entry that names no container":
		return map[string]any{"container": "app", "depends_on": "missing-db"}
	case "Skipping container update (self-dependency)",
		"Skipping container update (circular dependency)":
		return map[string]any{"container": "app", "id": "abcdef123456"}
	case "Only checking containers in scope":
		return map[string]any{"scope": "production"}
	case "Starting HTTP API server", "HTTP API server is enabled":
		return map[string]any{"host": "0.0.0.0", "port": "8080", "tls": true}
	default:
		return nil
	}
}

// goldenEntries returns one log entry per specially formatted message, plus an
// unrecognized message with and without data, to cover the template fallbacks.
func goldenEntries() []*notificationEntry {
	levels := []string{"info", "warning", "error", "debug"}

	entries := make([]*notificationEntry, 0, len(legacyTemplateMessages)+2)

	for i, message := range legacyTemplateMessages {
		entries = append(entries, &notificationEntry{
			Message: message,
			Data:    goldenEntryData(message),
			Time:    goldenEntryTime.Add(time.Duration(i) * time.Second),
			Level:   levels[i%len(levels)],
		})
	}

	entries = append(entries,
		&notificationEntry{
			Message: "Unrecognized message with fields",
			Data:    map[string]any{"alpha": "one", "beta": 2},
			Time:    goldenEntryTime.Add(time.Hour),
			Level:   "info",
		},
		&notificationEntry{
			Message: "Unrecognized message without fields",
			Time:    goldenEntryTime.Add(2 * time.Hour),
			Level:   "warning",
		},
	)

	return entries
}

// goldenFullReport returns a report with containers in every state. The first
// updated container carries the full set of image and Git metadata templates can show.
func goldenFullReport() types.Report {
	nop := zerolog.Nop()
	log := &nop
	progress := session.Progress{}
	failures := map[types.ContainerID]error{}

	updated, newImage := mockActions.CreateContainerForProgress(0, 11, "updt%d")
	progress.AddScanned(log, updated, newImage, types.UpdateParams{})
	progress.MarkForUpdate(log, updated.ID())
	progress[updated.ID()].SetGitMetadata(container.ReportMeta{
		GitRepo:         "https://github.com/org/app.git",
		GitRef:          "v1.2.0",
		Changelog:       "https://github.com/org/app/releases/tag/v1.2.0",
		Source:          "https://github.com/org/app",
		ImageURL:        "https://example.com/org/app",
		Documentation:   "https://example.com/org/app/docs",
		CurrentVersion:  "1.1.0",
		LatestVersion:   "1.2.0",
		CurrentRevision: "1111111111111111111111111111111111111111",
		LatestRevision:  "2222222222222222222222222222222222222222",
	})

	secondUpdated, secondImage := mockActions.CreateContainerForProgress(1, 11, "updt%d")
	progress.AddScanned(log, secondUpdated, secondImage, types.UpdateParams{})
	progress.MarkForUpdate(log, secondUpdated.ID())

	fresh, _ := mockActions.CreateContainerForProgress(0, 31, "frsh%d")
	progress.AddScanned(log, fresh, fresh.ImageID(), types.UpdateParams{})

	failed, failedImage := mockActions.CreateContainerForProgress(0, 21, "fail%d")
	progress.AddScanned(log, failed, failedImage, types.UpdateParams{})
	failures[failed.ID()] = errGoldenFailure

	skipped, _ := mockActions.CreateContainerForProgress(0, 41, "skip%d")
	progress.AddSkipped(log, skipped, errGoldenSkipped, types.UpdateParams{})

	restarted, _ := mockActions.CreateContainerForProgress(0, 51, "rstr%d")
	progress.AddScanned(log, restarted, restarted.ImageID(), types.UpdateParams{})
	progress.MarkRestarted(log, restarted.ID())

	progress.UpdateFailed(log, failures)

	return progress.Report(log)
}

// goldenFixtures returns the notification data each built-in template is rendered against.
func goldenFixtures() map[string]Data {
	nop := zerolog.Nop()
	staticData := StaticData{
		Title: GetTitle(&nop, goldenHost, ""),
		Host:  goldenHost,
	}

	return map[string]Data{
		"full": {
			StaticData: staticData,
			Entries:    goldenEntries(),
			Report:     goldenFullReport(),
		},
		"empty": {
			StaticData: staticData,
			Entries:    []*notificationEntry{},
			Report:     session.Progress{}.Report(&nop),
		},
		"nil-report": {
			StaticData: staticData,
			Entries:    goldenEntries(),
			Report:     nil,
		},
	}
}

// TestBuiltInTemplateOutput snapshots the output of every built-in notification
// template for a full report, an empty report, and a missing report.
//
// The legacy template renders the log entries alone, as it does when report
// templates are disabled. Rendering errors are recorded in the golden file so a
// template that starts or stops failing for a fixture is also caught.
func TestBuiltInTemplateOutput(t *testing.T) {
	nop := zerolog.Nop()
	fixtures := goldenFixtures()

	for _, templateName := range slices.Sorted(maps.Keys(commonTemplates)) {
		legacy := templateName == "default-legacy"

		tpl, err := getShoutrrrTemplate(&nop, templateName, legacy)
		require.NoError(t, err, "template %q", templateName)

		notifier := &shoutrrrTypeNotifier{template: tpl, legacyTemplate: legacy}

		for _, fixtureName := range slices.Sorted(maps.Keys(fixtures)) {
			t.Run(templateName+"/"+fixtureName, func(t *testing.T) {
				output, err := notifier.buildMessage(fixtures[fixtureName])
				if err != nil {
					output = "error: " + err.Error()
				}

				if !strings.HasSuffix(output, "\n") {
					output += "\n"
				}

				golden.Assert(t, "template-"+templateName+"-"+fixtureName, []byte(output))
			})
		}
	}
}
