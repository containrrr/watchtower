package preview

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/nicholas-fedor/tplprev/internal/notify"
	"github.com/nicholas-fedor/tplprev/internal/report"
)

// PreviewData represents a generator for preview data, including container statuses and log entries.
type PreviewData struct {
	rand           *rand.Rand
	lastTime       time.Time
	report         *sessionReport
	containerCount int
	entries        []*notify.Entry
	static         notify.StaticData
}

const (
	idLength                = 32
	maxTimeIncrementSeconds = 30
	previewInstanceCount    = 2
	previewRemovedInstances = 1
	previewAPIPort          = "8080"
	previewCooldown         = "24h"
	previewEligibleIn       = "6h"
	previewEligibleAfter    = 6 * time.Hour
	previewDiskUsage        = int64(32_000_000_000)
	previewDiskMax          = int64(40_000_000_000)
	previewDiskWarn         = int64(32_000_000_000)
	previewDiskReclaimable  = int64(4_000_000_000)
	previewDiskImageCount   = int64(12)
)

var (
	errExecutionFailed = errors.New("execution failed")
	errSkipped         = errors.New("container skipped")

	// previewStartTime is the fixed session start so generated timestamps stay stable.
	previewStartTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
)

// New initializes a PreviewData generator with a fixed seed for deterministic output.
func New() *PreviewData {
	//nolint:gosec // math/rand is used for deterministic preview output.
	return &PreviewData{
		rand:           rand.New(rand.NewSource(1)),
		lastTime:       previewStartTime,
		report:         &sessionReport{},
		containerCount: 0,
		entries:        []*notify.Entry{},
		static: notify.StaticData{
			Title: "Title",
			Host:  "Host",
		},
	}
}

// AddFromState adds a container status entry to the report with the specified state.
//
// Parameters:
//   - state: Container report state to simulate.
func (p *PreviewData) AddFromState(state State) {
	cid := report.ContainerID(p.generateID())
	oldImageID := report.ImageID(p.generateID())
	newImageID := report.ImageID(p.generateID())
	name := p.generateName()
	image := p.generateImageName(name)

	var err error

	//nolint:exhaustive // Only failed and skipped states carry a simulated error.
	switch state {
	case FailedState:
		err = fmt.Errorf("%w: %s", errExecutionFailed, p.randomEntry(errorMessages))
	case SkippedState:
		err = fmt.Errorf("%w: %s", errSkipped, p.randomEntry(skippedMessages))
	}

	status := &containerStatus{
		containerID:    cid,
		oldImage:       oldImageID,
		newImage:       newImageID,
		containerName:  name,
		imageName:      image,
		containerError: err,
		state:          state,
	}
	if state == UpdatedState {
		status.gitRepo = "https://github.com/org/app.git"
		status.gitRef = "v1.2.4"
		status.changelog = "https://github.com/org/app/releases/tag/v1.2.4"
		status.source = "https://github.com/org/app"
		status.imageURL = "https://github.com/org/app/pkgs/container/app"
		status.documentation = "https://github.com/org/app#readme"
		status.currentImageVersion = "1.2.3"
		status.latestImageVersion = "1.2.4"
		status.currentImageRevision = "1111111111"
		status.latestImageRevision = "2222222222"
	}

	switch state {
	case ScannedState:
		p.report.scanned = append(p.report.scanned, status)
	case UpdatedState:
		p.report.updated = append(p.report.updated, status)
	case FailedState:
		p.report.failed = append(p.report.failed, status)
	case SkippedState:
		p.report.skipped = append(p.report.skipped, status)
	case RestartedState:
		p.report.restarted = append(p.report.restarted, status)
	case StaleState:
		p.report.stale = append(p.report.stale, status)
	case FreshState:
		p.report.fresh = append(p.report.fresh, status)
	}

	p.containerCount++
}

// AddLogEntry adds a preview log entry with the specified level.
//
// Parameters:
//   - level: Log level whose message pool should be sampled.
func (p *PreviewData) AddLogEntry(level LogLevel) {
	var msg string

	switch level {
	case FatalLevel, ErrorLevel:
		msg = p.randomEntry(errorMessages)
	case WarnLevel:
		msg = p.randomEntry(warningMessages)
	case TraceLevel, DebugLevel, InfoLevel, PanicLevel:
		msg = p.randomEntry(infoMessages)
	default:
		msg = p.randomEntry(infoMessages)
	}

	name, image := p.logSubject()

	p.entries = append(p.entries, &notify.Entry{
		Message: msg,
		Data:    p.dataForMessage(msg, name, image),
		Time:    p.generateTime(),
		Level:   level.String(),
	})
}

// NotificationData returns the generated notification template data.
//
// Report is nil when no container states were added so templates treat the
// preview as legacy/log mode.
//
// Returns:
//   - notify.Data: Simulated notification data.
func (p *PreviewData) NotificationData() notify.Data {
	payload := notify.Data{
		StaticData: p.static,
		Entries:    p.entries,
	}

	if p.containerCount > 0 {
		payload.Report = p.report
	}

	return payload
}

func (p *PreviewData) generateID() string {
	buf := make([]byte, idLength)
	_, _ = p.rand.Read(buf)

	return hex.EncodeToString(buf)
}

func (p *PreviewData) generateTime() time.Time {
	p.lastTime = p.lastTime.Add(time.Duration(p.rand.Intn(maxTimeIncrementSeconds)) * time.Second)

	return p.lastTime
}

func (p *PreviewData) randomEntry(arr []string) string {
	return arr[p.rand.Intn(len(arr))]
}

func (p *PreviewData) generateName() string {
	index := p.containerCount
	if index < len(containerNames) {
		return containerNames[index]
	}

	suffix := index / len(containerNames)
	index %= len(containerNames)

	return containerNames[index] + strconv.FormatInt(int64(suffix), 10)
}

func (p *PreviewData) generateImageName(name string) string {
	index := p.containerCount % len(organizationNames)

	return organizationNames[index] + "/" + name + ":latest"
}

func (p *PreviewData) logSubject() (string, string) {
	index := len(p.entries)
	name := containerNames[index%len(containerNames)]
	org := organizationNames[index%len(organizationNames)]

	return name, org + "/" + name + ":latest"
}

func (p *PreviewData) dataForMessage(msg, name, image string) map[string]any {
	cid := report.ContainerID(p.generateID()).ShortID()
	newID := report.ImageID(p.generateID()).ShortID()
	oldID := report.ImageID(p.generateID()).ShortID()

	switch msg {
	case "Found new image":
		return map[string]any{"image": image, "new_id": newID}
	case "Stopping container", "Stopping linked container":
		return map[string]any{"container": name, "id": cid}
	case "Started new container", "Started linked container":
		return map[string]any{"container": name, "new_id": newID}
	case "Removing image":
		return map[string]any{"image_name": image, "image_id": oldID}
	case "Failed to list containers for image usage check, skipping removal":
		return map[string]any{
			"image_name": image,
			"image_id":   oldID,
			"error":      "cannot verify image usage",
		}
	case "Container updated":
		return map[string]any{
			"container": name,
			"image":     image,
			"old_id":    oldID,
			"new_id":    newID,
		}
	case "Detected multiple Watchtower instances - initiating cleanup":
		return map[string]any{"count": previewInstanceCount}
	case "Successfully removed all excess Watchtower containers":
		return map[string]any{"removed_instances": previewRemovedInstances}
	case "Image is within cooldown period - not eligible for update":
		return map[string]any{
			"image":       image,
			"cooldown":    previewCooldown,
			"eligible_in": previewEligibleIn,
			"eligible_at": p.lastTime.Add(previewEligibleAfter).Format(time.RFC3339),
		}
	case "Image age exceeds cooldown - eligible for update",
		"Image creation time unavailable - update check unavailable":
		return map[string]any{"image": image, "cooldown": previewCooldown}
	case "Starting HTTP API server", "HTTP API server is enabled":
		return map[string]any{"tls": false, "host": "0.0.0.0", "port": previewAPIPort}
	case "Only checking containers in scope":
		return map[string]any{"scope": "prod"}
	case "Docker image usage exceeds configured maximum":
		return map[string]any{
			"usage":       previewDiskMax,
			"max":         previewDiskMax,
			"warn":        previewDiskWarn,
			"reclaimable": previewDiskReclaimable,
			"image_count": previewDiskImageCount,
		}
	case "Docker image usage exceeds configured warning threshold":
		return map[string]any{
			"usage":       previewDiskUsage,
			"max":         previewDiskMax,
			"warn":        previewDiskWarn,
			"reclaimable": previewDiskReclaimable,
			"image_count": previewDiskImageCount,
		}
	case "Failed to query Docker image disk usage":
		return map[string]any{"error": "daemon disk usage unavailable"}
	case "Docker image usage budget enabled":
		return map[string]any{
			"disk_space_max":  previewDiskMax,
			"disk_space_warn": previewDiskWarn,
		}
	default:
		return nil
	}
}
