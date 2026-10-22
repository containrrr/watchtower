package preview

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/tplprev/internal/notify"
	"github.com/nicholas-fedor/tplprev/internal/templates"
)

func TestRenderDefaultReport(t *testing.T) {
	t.Parallel()

	result, err := Render(
		templates.Templates["default"],
		[]State{ScannedState, UpdatedState, FailedState},
		nil,
	)
	require.NoError(t, err)
	assert.Contains(t, result, "1 Scanned, 1 Updated")
	assert.Contains(t, result, "1 Failed")
	assert.Contains(t, result, "datamatrix")
	assert.Contains(t, result, "updated to")
	assert.Contains(t, result, "ref v1.2.4")
	assert.Contains(t, result, "https://github.com/org/app/releases")
}

func TestRenderDefaultLegacyEntriesRoot(t *testing.T) {
	t.Parallel()

	payload := notify.Data{
		Entries: []*notify.Entry{
			{
				Message: "Found new image",
				Data: map[string]any{
					"image":  "techwave/cyberscribe:latest",
					"new_id": "abc123def456",
				},
				Level: "info",
				Time:  time.Time{},
			},
		},
	}

	result, err := Execute(templates.Templates["default-legacy"], payload, false)
	require.NoError(t, err)
	assert.Contains(t, result, "Found new image: techwave/cyberscribe:latest (abc123def456)")
}

func TestRenderDefaultLegacyDiskSpaceMessages(t *testing.T) {
	t.Parallel()

	payload := notify.Data{
		Entries: []*notify.Entry{
			{
				Message: "Docker image usage exceeds configured maximum",
				Data: map[string]any{
					"usage":       int64(32000000000),
					"max":         int64(40000000000),
					"warn":        int64(32000000000),
					"reclaimable": int64(4000000000),
					"image_count": int64(12),
				},
				Level: "error",
			},
			{
				Message: "Failed to query Docker image disk usage",
				Data:    map[string]any{"error": "daemon disk usage unavailable"},
				Level:   "error",
			},
		},
	}

	result, err := Execute(templates.Templates["default-legacy"], payload, false)
	require.NoError(t, err)
	assert.Contains(
		t,
		result,
		"Docker image usage exceeds configured maximum: 32 GB of 40 GB used (4 GB reclaimable, 12 images)",
	)
	assert.Contains(t, result, "Failed to query Docker image disk usage: daemon disk usage unavailable")
	assert.NotContains(t, result, " | ")
}

func TestRenderJSONIncludesReport(t *testing.T) {
	t.Parallel()

	result, err := Render(
		templates.Templates["json.v1"],
		[]State{UpdatedState},
		[]LogLevel{InfoLevel},
	)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	require.Contains(t, decoded, "report")

	report, ok := decoded["report"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, report["updated"])
	assert.Equal(t, "Title", decoded["title"])
	assert.Equal(t, "Host", decoded["host"])
}

func TestRenderLegacyModeOmitsReportRoot(t *testing.T) {
	t.Parallel()

	result, err := Render(
		templates.Templates["json.v1"],
		nil,
		[]LogLevel{InfoLevel},
	)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(result), "["))
	assert.NotContains(t, result, `"report"`)
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()

	input := templates.Templates["json.v1"]
	states := []State{UpdatedState}
	levels := []LogLevel{InfoLevel}

	first, err := Render(input, states, levels)
	require.NoError(t, err)

	second, err := Render(input, states, levels)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestRenderDefaultTemplateWithoutReport(t *testing.T) {
	t.Parallel()

	result, err := Render(
		templates.Templates["default"],
		nil,
		[]LogLevel{InfoLevel},
	)
	require.NoError(t, err)
	assert.NotContains(t, result, "can't evaluate field Report")
	assert.NotEmpty(t, result)
}

func TestRenderPreviewDefaultTemplateWithoutReport(t *testing.T) {
	t.Parallel()

	const previewDefault = `{{- if .Report -}}
 {{- with .Report -}}
   {{len .Scanned}} Scanned
 {{- end -}}
{{- else -}}
 {{range .Entries -}}{{.Message}}{{"\n"}}{{- end -}}
{{- end -}}`

	result, err := Render(previewDefault, nil, []LogLevel{InfoLevel})
	require.NoError(t, err)
	assert.NotContains(t, result, "can't evaluate field Report")
	assert.NotEmpty(t, result)
}

func TestRenderParseError(t *testing.T) {
	t.Parallel()

	_, err := Render("{{ .Report", nil, nil)
	require.Error(t, err)
}

func TestRenderPorcelainJSONWithNilReport(t *testing.T) {
	t.Parallel()

	result, err := Render(
		templates.Templates["porcelain.json"],
		nil,
		nil,
	)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	require.Contains(t, decoded, "containers")

	containers, ok := decoded["containers"].([]any)
	require.True(t, ok)
	assert.Empty(t, containers)
}
