package notifications

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestFormatDiskSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "int64 gigabytes", value: int64(10_000_000_000), want: "10 GB"},
		{name: "int megabytes", value: 2_000_000, want: "2 MB"},
		{name: "float64 gigabytes", value: float64(8_000_000_000), want: "8 GB"},
		{name: "json number", value: json.Number("40000000000"), want: "40 GB"},
		{name: "zero", value: int64(0), want: "0 B"},
		{name: "nil", value: nil, want: "unknown"},
		{name: "string", value: "10GB", want: "unknown"},
		{name: "uint64 overflow", value: uint64(math.MaxInt64) + 1, want: "unknown"},
		{name: "float64 nan", value: math.NaN(), want: "unknown"},
		{name: "float64 max int64", value: float64(1 << 63), want: "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, formatDiskSpace(tc.value))
		})
	}
}

func TestVersionInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		oldVersion string
		newVersion string
		want       string
	}{
		{name: "transition", oldVersion: "v0.15.0", newVersion: "v0.16.0", want: "v0.15.0 -> v0.16.0"},
		{name: "only new label", newVersion: "v0.16.0", want: "v0.16.0"},
		{name: "only old label", oldVersion: "v0.15.0", want: "v0.15.0"},
		{name: "identical labels", oldVersion: "v1.0.0", newVersion: "v1.0.0", want: "v1.0.0"},
		{name: "no labels", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status := updatedAppStatus()
			status.SetGitMetadata(container.ReportMeta{
				CurrentVersion: tc.oldVersion,
				LatestVersion:  tc.newVersion,
			})

			assert.Equal(t, tc.want, versionInfo(status))
		})
	}
}

func TestVersionInfo_NilReport(t *testing.T) {
	t.Parallel()

	assert.Empty(t, versionInfo(nil))
}

// TestDefaultTemplateRendersVersionInfo verifies the report-mode "default" template
// surfaces the OCI version transition, which is the path --notification-report uses.
func TestDefaultTemplateRendersVersionInfo(t *testing.T) {
	t.Parallel()

	status := updatedAppStatus()
	status.SetGitMetadata(container.ReportMeta{
		CurrentVersion: "v0.15.0",
		LatestVersion:  "v0.16.0",
	})

	tpl, err := template.New("").Funcs(templateFuncs).Parse(commonTemplates["default"])
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, tpl.Execute(&out, notificationData{
		Report: &session.SingleContainerReport{
			UpdatedReports: []types.ContainerReport{status},
		},
	}))

	assert.Contains(t, out.String(), "(v0.15.0 -> v0.16.0)")
}

// TestDefaultLegacyFoundNewImageVersions verifies the log-mode template renders the
// version fields HasNewImage attaches to the "Found new image" event.
func TestDefaultLegacyFoundNewImageVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "transition",
			data: map[string]any{"old_version": "v0.15.0", "new_version": "v0.16.0"},
			want: "Found new image: org/app:latest (b7734c440450) v0.15.0 -> v0.16.0",
		},
		{
			name: "only new label",
			data: map[string]any{"new_version": "v0.16.0"},
			want: "Found new image: org/app:latest (b7734c440450) -> v0.16.0",
		},
		{
			name: "no labels",
			data: map[string]any{},
			want: "Found new image: org/app:latest (b7734c440450)",
		},
	}

	tpl := template.Must(template.New("").Funcs(templateFuncs).Parse(commonTemplates["default-legacy"]))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.data["image"] = "org/app:latest"
			tc.data["new_id"] = "b7734c440450"

			var out strings.Builder
			require.NoError(t, tpl.Execute(&out, []*notificationEntry{
				{Message: "Found new image", Data: tc.data},
			}))

			assert.Equal(t, tc.want, strings.TrimSpace(out.String()))
		})
	}
}
