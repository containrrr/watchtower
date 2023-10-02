package data

import (
	"encoding/hex"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/containrrr/watchtower/pkg/types"
)

type previewData struct {
	rand           *rand.Rand
	lastTime       time.Time
	report         *report
	containerCount int
	Entries        []*logEntry
	StaticData     staticData
}

type staticData struct {
	Title string
	Host  string
}

// New initializes a new preview data struct
func New() *previewData {
	return &previewData{
		rand:           rand.New(rand.NewSource(1)),
		lastTime:       time.Now().Add(-30 * time.Minute),
		report:         nil,
		containerCount: 0,
		Entries:        []*logEntry{},
		StaticData: staticData{
			Title: "Title",
			Host:  "Host",
		},
	}
}

// AddFromState adds a container status entry to the report with the given state
func (pb *previewData) AddFromState(state State) {
	cid := types.ContainerID(pb.generateID())
	old := types.ImageID(pb.generateID())
	new := types.ImageID(pb.generateID())
	name := pb.generateName()
	image := pb.generateImageName(name)
	var err error
	if state == FailedState {
		err = errors.New(pb.randomEntry(errorMessages))
	} else if state == SkippedState {
		err = errors.New(pb.randomEntry(skippedMessages))
	}
	pb.addContainer(containerStatus{
		containerID:   cid,
		oldImage:      old,
		newImage:      new,
		containerName: name,
		imageName:     image,
		error:         err,
		state:         state,
	})
}

func (pb *previewData) addContainer(c containerStatus) {
	if pb.report == nil {
		pb.report = &report{}
	}
	switch c.state {
	case ScannedState:
		pb.report.scanned = append(pb.report.scanned, &c)
	case UpdatedState:
		pb.report.updated = append(pb.report.updated, &c)
	case FailedState:
		pb.report.failed = append(pb.report.failed, &c)
	case SkippedState:
		pb.report.skipped = append(pb.report.skipped, &c)
	case StaleState:
		pb.report.stale = append(pb.report.stale, &c)
	case FreshState:
		pb.report.fresh = append(pb.report.fresh, &c)
	default:
		return
	}
	pb.containerCount += 1
}

// AddLogEntry adds a preview log entry of the given level
func (pd *previewData) AddLogEntry(level LogLevel) {
	var msg string
	switch level {
	case FatalLevel:
		fallthrough
	case ErrorLevel:
		fallthrough
	case WarnLevel:
		msg = pd.randomEntry(logErrors)
	default:
		msg = pd.randomEntry(logMessages)
	}
	pd.Entries = append(pd.Entries, &logEntry{
		Message: msg,
		Data:    map[string]any{},
		Time:    pd.generateTime(),
		Level:   level,
	})
}

// Report returns a preview report
func (pb *previewData) Report() types.Report {
	return pb.report
}

func (pb *previewData) generateID() string {
	buf := make([]byte, 32)
	_, _ = pb.rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (pb *previewData) generateTime() time.Time {
	pb.lastTime = pb.lastTime.Add(time.Duration(pb.rand.Intn(30)) * time.Second)
	return pb.lastTime
}

func (pb *previewData) randomEntry(arr []string) string {
	return arr[pb.rand.Intn(len(arr))]
}

func (pb *previewData) generateName() string {
	index := pb.containerCount
	if index <= len(containerNames) {
		return "/" + containerNames[index]
	}
	suffix := index / len(containerNames)
	index %= len(containerNames)
	return "/" + containerNames[index] + strconv.FormatInt(int64(suffix), 10)
}

func (pb *previewData) generateImageName(name string) string {
	index := pb.containerCount % len(organizationNames)
	return organizationNames[index] + name + ":latest"
}
