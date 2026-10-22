package notify

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/tplprev/internal/report"
)

type stubReport struct {
	scanned []report.ContainerReport
	updated []report.ContainerReport
	failed  []report.ContainerReport
}

type stubContainerError struct {
	id    report.ContainerID
	name  string
	oldID report.ImageID
	newID report.ImageID
	image string
	state string
	err   string
	// meta makes each image and Git metadata accessor return its own name.
	meta bool
}

func (r stubReport) Scanned() []report.ContainerReport   { return r.scanned }
func (r stubReport) Updated() []report.ContainerReport   { return r.updated }
func (r stubReport) Failed() []report.ContainerReport    { return r.failed }
func (r stubReport) Skipped() []report.ContainerReport   { return nil }
func (r stubReport) Stale() []report.ContainerReport     { return nil }
func (r stubReport) Fresh() []report.ContainerReport     { return nil }
func (r stubReport) Restarted() []report.ContainerReport { return nil }
func (r stubReport) All() []report.ContainerReport       { return r.scanned }

func (c stubContainerError) ID() report.ContainerID             { return c.id }
func (c stubContainerError) Name() string                       { return c.name }
func (c stubContainerError) CurrentImageID() report.ImageID     { return c.oldID }
func (c stubContainerError) LatestImageID() report.ImageID      { return c.newID }
func (c stubContainerError) ImageName() string                  { return c.image }
func (c stubContainerError) Error() string                      { return c.err }
func (c stubContainerError) State() string                      { return c.state }
func (c stubContainerError) IsMonitorOnly() bool                { return false }
func (c stubContainerError) NewContainerID() report.ContainerID { return "" }
func (c stubContainerError) GitRepo() string                    { return c.metaValue("GitRepo") }
func (c stubContainerError) GitRef() string                     { return c.metaValue("GitRef") }
func (c stubContainerError) Changelog() string                  { return c.metaValue("Changelog") }
func (c stubContainerError) Source() string                     { return c.metaValue("Source") }
func (c stubContainerError) ImageURL() string                   { return c.metaValue("ImageURL") }
func (c stubContainerError) Documentation() string              { return c.metaValue("Documentation") }

func (c stubContainerError) CurrentImageVersion() string { return c.metaValue("CurrentImageVersion") }

func (c stubContainerError) LatestImageVersion() string { return c.metaValue("LatestImageVersion") }

func (c stubContainerError) CurrentImageRevision() string { return c.metaValue("CurrentImageRevision") }

func (c stubContainerError) LatestImageRevision() string { return c.metaValue("LatestImageRevision") }

// metaValue returns the name of the metadata accessor when meta is set, so a
// value written under the wrong JSON key is detected, and "" otherwise.
func (c stubContainerError) metaValue(accessor string) string {
	if !c.meta {
		return ""
	}

	return accessor
}

func TestDataMarshalJSON(t *testing.T) {
	t.Parallel()

	updated := stubContainerError{
		id:    "sha256:c79110000000aaaa",
		name:  "updt1",
		oldID: "sha256:01d110000000aaaa",
		newID: "sha256:d0a110000000aaaa",
		image: "mock/updt1:latest",
		state: "updated",
	}
	failed := stubContainerError{
		id:    "sha256:c79210000000aaaa",
		name:  "fail1",
		oldID: "sha256:01d210000000aaaa",
		newID: "sha256:d0a210000000aaaa",
		image: "mock/fail1:latest",
		state: "failed",
		err:   "execution failed",
	}

	payload := Data{
		Title: "Title", Host: "Host",
		Entries: []*Entry{
			{
				Message: "Found new image",
				Data:    map[string]any{"image": "mock/updt1:latest"},
				Time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				Level:   "info",
			},
		},
		Report: stubReport{
			scanned: []report.ContainerReport{updated, failed},
			updated: []report.ContainerReport{updated},
			failed:  []report.ContainerReport{failed},
		},
	}

	bytes, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(bytes, &decoded))

	assert.Equal(t, "Title", decoded["title"])
	assert.Equal(t, "Host", decoded["host"])

	report, ok := decoded["report"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, report["scanned"], 2)
	assert.Len(t, report["updated"], 1)
	assert.Len(t, report["failed"], 1)

	failedJSON, ok := report["failed"].([]any)
	require.True(t, ok)
	failedEntry, ok := failedJSON[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "execution failed", failedEntry["error"])
	assert.Equal(t, "c79210000000", failedEntry["id"])

	entries, ok := decoded["entries"].([]any)
	require.True(t, ok)
	require.Len(t, entries, 1)
}

func TestDataMarshalJSONNilReport(t *testing.T) {
	t.Parallel()

	payload := Data{
		Title: "Title", Host: "Host",
		Entries: []*Entry{},
	}

	bytes, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(bytes, &decoded))
	assert.Nil(t, decoded["report"])
}

// TestDataMarshalJSONContainerKeysMatchWatchtower checks that the preview's
// JSON output can carry the same container keys as Watchtower's. The expected
// list is generated from Watchtower's JSON output by `task tplprev:gen`.
func TestDataMarshalJSONContainerKeysMatchWatchtower(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/container-json-keys.txt")
	require.NoError(t, err)

	var want []string

	for line := range strings.Lines(string(contents)) {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			want = append(want, line)
		}
	}

	full := stubContainerError{
		id:    "sha256:c79110000000aaaa",
		name:  "updt1",
		oldID: "sha256:01d110000000aaaa",
		newID: "sha256:d0a110000000aaaa",
		image: "mock/updt1:latest",
		state: "updated",
		meta:  true,
	}
	failed := stubContainerError{
		id:    "sha256:c79210000000aaaa",
		name:  "fail1",
		image: "mock/fail1:latest",
		state: "failed",
		err:   "execution failed",
	}

	encoded, err := json.Marshal(Data{
		Report: stubReport{
			scanned: []report.ContainerReport{full, failed},
			updated: []report.ContainerReport{full},
			failed:  []report.ContainerReport{failed},
		},
	})
	require.NoError(t, err)

	var decoded struct {
		Report map[string][]map[string]any `json:"report"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	keys := map[string]struct{}{}

	for _, containers := range decoded.Report {
		for _, container := range containers {
			for key := range container {
				keys[key] = struct{}{}
			}
		}
	}

	assert.Equal(t, want, slices.Sorted(maps.Keys(keys)))

	// Each metadata key carries the value of the accessor Watchtower reads it from.
	require.Len(t, decoded.Report["updated"], 1)

	updated := decoded.Report["updated"][0]
	for key, accessor := range map[string]string{
		"gitRepo":              "GitRepo",
		"gitRef":               "GitRef",
		"changelog":            "Changelog",
		"ociSource":            "Source",
		"imageUrl":             "ImageURL",
		"documentation":        "Documentation",
		"currentImageVersion":  "CurrentImageVersion",
		"latestImageVersion":   "LatestImageVersion",
		"currentImageRevision": "CurrentImageRevision",
		"latestImageRevision":  "LatestImageRevision",
	} {
		assert.Equal(t, accessor, updated[key], "key %q", key)
	}
}
