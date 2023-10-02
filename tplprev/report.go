package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/containrrr/watchtower/pkg/types"
)

type reportBuilder struct {
	rand   *rand.Rand
	report Report
}

func ReportBuilder() *reportBuilder {
	return &reportBuilder{
		report: Report{},
		rand:   rand.New(rand.NewSource(1)),
	}
}

type buildAction func(*reportBuilder)

func (rb *reportBuilder) Build() types.Report {
	return &rb.report
}

func (rb *reportBuilder) AddNContainers(n int, state State) {
	fmt.Printf("Adding %v containers with state %v", n, state)
	for i := 0; i < n; i++ {
		cid := types.ContainerID(rb.generateID())
		old := types.ImageID(rb.generateID())
		new := types.ImageID(rb.generateID())
		name := rb.generateName()
		image := rb.generateImageName(name)
		var err error
		if state == FailedState {
			err = errors.New(rb.randomEntry(errorMessages))
		} else if state == SkippedState {
			err = errors.New(rb.randomEntry(skippedMessages))
		}
		rb.AddContainer(ContainerStatus{
			containerID:   cid,
			oldImage:      old,
			newImage:      new,
			containerName: name,
			imageName:     image,
			error:         err,
			state:         state,
		})
	}
}

func (rb *reportBuilder) AddContainer(c ContainerStatus) {
	switch c.state {
	case ScannedState:
		rb.report.scanned = append(rb.report.scanned, &c)
	case UpdatedState:
		rb.report.updated = append(rb.report.updated, &c)
	case FailedState:
		rb.report.failed = append(rb.report.failed, &c)
	case SkippedState:
		rb.report.skipped = append(rb.report.skipped, &c)
	case StaleState:
		rb.report.stale = append(rb.report.stale, &c)
	case FreshState:
		rb.report.fresh = append(rb.report.fresh, &c)
	}
}

func (rb *reportBuilder) generateID() string {
	buf := make([]byte, 32)
	_, _ = rb.rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (rb *reportBuilder) randomEntry(arr []string) string {
	return arr[rb.rand.Intn(len(arr))]
}

func (rb *reportBuilder) generateName() string {
	index := rb.containerCount()
	if index <= len(containerNames) {
		return containerNames[index]
	}
	suffix := index / len(containerNames)
	index %= len(containerNames)
	return containerNames[index] + strconv.FormatInt(int64(suffix), 10)
}

func (rb *reportBuilder) generateImageName(name string) string {
	index := rb.containerCount()
	return companyNames[index%len(companyNames)] + "/" + strings.ToLower(name) + ":latest"
}

func (rb *reportBuilder) containerCount() int {
	return len(rb.report.scanned) +
		len(rb.report.updated) +
		len(rb.report.failed) +
		len(rb.report.skipped) +
		len(rb.report.stale) +
		len(rb.report.fresh)
}

type State string

const (
	ScannedState State = "scanned"
	UpdatedState State = "updated"
	FailedState  State = "failed"
	SkippedState State = "skipped"
	StaleState   State = "stale"
	FreshState   State = "fresh"
)

type Report struct {
	scanned []types.ContainerReport
	updated []types.ContainerReport
	failed  []types.ContainerReport
	skipped []types.ContainerReport
	stale   []types.ContainerReport
	fresh   []types.ContainerReport
}

func (r *Report) Scanned() []types.ContainerReport {
	return r.scanned
}
func (r *Report) Updated() []types.ContainerReport {
	return r.updated
}
func (r *Report) Failed() []types.ContainerReport {
	return r.failed
}
func (r *Report) Skipped() []types.ContainerReport {
	return r.skipped
}
func (r *Report) Stale() []types.ContainerReport {
	return r.stale
}
func (r *Report) Fresh() []types.ContainerReport {
	return r.fresh
}

func (r *Report) All() []types.ContainerReport {
	allLen := len(r.scanned) + len(r.updated) + len(r.failed) + len(r.skipped) + len(r.stale) + len(r.fresh)
	all := make([]types.ContainerReport, 0, allLen)

	presentIds := map[types.ContainerID][]string{}

	appendUnique := func(reports []types.ContainerReport) {
		for _, cr := range reports {
			if _, found := presentIds[cr.ID()]; found {
				continue
			}
			all = append(all, cr)
			presentIds[cr.ID()] = nil
		}
	}

	appendUnique(r.updated)
	appendUnique(r.failed)
	appendUnique(r.skipped)
	appendUnique(r.stale)
	appendUnique(r.fresh)
	appendUnique(r.scanned)

	sort.Sort(sortableContainers(all))

	return all
}

type sortableContainers []types.ContainerReport

// Len implements sort.Interface.Len
func (s sortableContainers) Len() int { return len(s) }

// Less implements sort.Interface.Less
func (s sortableContainers) Less(i, j int) bool { return s[i].ID() < s[j].ID() }

// Swap implements sort.Interface.Swap
func (s sortableContainers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
