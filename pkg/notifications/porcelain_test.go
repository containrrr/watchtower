package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestToPorcelainReport_NilInput(t *testing.T) {
	t.Parallel()

	report := toPorcelainReport(nil)

	assert.Empty(t, report.Containers)
	assert.Empty(t, report.Containers)
}

func TestToPorcelainJSON_NilInput(t *testing.T) {
	t.Parallel()

	result := toPorcelainJSON(nil)

	require.NotEmpty(t, result)
	assert.JSONEq(t, `{
  "containers": []
}`, result)
}

func TestToPorcelainJSON_GitAndOCIFields(t *testing.T) {
	t.Parallel()

	status := updatedAppStatus()
	status.SetGitMetadata(container.ReportMeta{
		GitRepo:         "https://github.com/org/app.git",
		GitRef:          "main",
		Changelog:       "https://github.com/org/app/releases",
		Source:          "https://github.com/org/app",
		CurrentVersion:  "1.2.2",
		LatestVersion:   "1.2.3",
		CurrentRevision: "olddeadbeef",
		LatestRevision:  "deadbeef",
	})

	jsonOut := toPorcelainJSON(&session.SingleContainerReport{
		UpdatedReports: []types.ContainerReport{status},
	})
	assert.Contains(t, jsonOut, `"git_repo": "https://github.com/org/app.git"`)
	assert.Contains(t, jsonOut, `"changelog": "https://github.com/org/app/releases"`)
	assert.Contains(t, jsonOut, `"oci_source": "https://github.com/org/app"`)
	assert.Contains(t, jsonOut, `"current_revision": "olddeadbeef"`)
	assert.Contains(t, jsonOut, `"latest_revision": "deadbeef"`)
	assert.Contains(t, jsonOut, `"current_image_version": "1.2.2"`)
	assert.Contains(t, jsonOut, `"latest_image_version": "1.2.3"`)
}
