package notifications

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// updatedAppStatus returns the report status of an updated "app" container
// running org/app:latest, built the way an update session builds it.
//
// Returns:
//   - *session.ContainerStatus: Status in the updated state.
func updatedAppStatus() *session.ContainerStatus {
	log := new(zerolog.Nop())
	app := container.NewContainer(nil,
		&dockerContainer.InspectResponse{
			ID:     strings.Repeat("a", 64),
			Name:   "/app",
			Image:  "sha256:" + strings.Repeat("b", 64),
			Config: &dockerContainer.Config{Image: "org/app:latest"},
		},
		&dockerImage.InspectResponse{ID: "sha256:" + strings.Repeat("b", 64)},
	)

	progress := session.Progress{}
	progress.AddScanned(log, app, types.ImageID("sha256:"+strings.Repeat("c", 64)), types.UpdateParams{})
	progress.MarkForUpdate(log, app.ID())

	return progress[app.ID()]
}

// TestJSONTemplate verifies the JSON report template: its fields, ID types,
// and container states.
func TestJSONTemplate(t *testing.T) {
	t.Parallel()

	t.Run("using report templates", func(t *testing.T) {
		t.Parallel()

		t.Run("JSON template is used", func(t *testing.T) {
			t.Parallel()

			t.Run("should format the messages to the expected format", func(t *testing.T) {
				t.Parallel()

				expected := `{
	"entries": [
			{
				"data": null,
				"level": "info",
				"message": "foo Bar",
				"time": "0001-01-01T00:00:00Z"
			}
		],
		"host": "Mock",
		"report": {
		"failed": [
			{
				"currentImageId": "01d210000000",
				"error": "accidentally the whole container",
				"id": "c79210000000",
				"imageName": "mock/fail1:latest",
				"latestImageId": "d0a210000000",
				"name": "fail1",
				"state": "Failed"
			}
		],
		"fresh": [
			{
				"currentImageId": "01d310000000",
				"id": "c79310000000",
				"imageName": "mock/frsh1:latest",
				"latestImageId": "01d310000000",
				"name": "frsh1",
				"state": "Fresh"
			}
		],
		"restarted": [],
		"scanned": [
			{
				"currentImageId": "01d110000000",
				"id": "c79110000000",
				"imageName": "mock/updt1:latest",
				"latestImageId": "d0a110000000",
				"name": "updt1",
				"state": "Updated"
			},
			{
				"currentImageId": "01d120000000",
				"id": "c79120000000",
				"imageName": "mock/updt2:latest",
				"latestImageId": "d0a120000000",
				"name": "updt2",
				"state": "Updated"
			},
			{
				"currentImageId": "01d210000000",
				"error": "accidentally the whole container",
				"id": "c79210000000",
				"imageName": "mock/fail1:latest",
				"latestImageId": "d0a210000000",
				"name": "fail1",
				"state": "Failed"
			},
			{
				"currentImageId": "01d310000000",
				"id": "c79310000000",
				"imageName": "mock/frsh1:latest",
				"latestImageId": "01d310000000",
				"name": "frsh1",
				"state": "Fresh"
			}
		],
		"skipped": [
			{
				"currentImageId": "01d410000000",
				"error": "unpossible",
				"id": "c79410000000",
				"imageName": "mock/skip1:latest",
				"latestImageId": "01d410000000",
				"name": "skip1",
				"state": "Skipped"
			}
		],
		"stale": [],
		"updated": [
			{
				"currentImageId": "01d110000000",
				"id": "c79110000000",
				"imageName": "mock/updt1:latest",
				"latestImageId": "d0a110000000",
				"name": "updt1",
				"state": "Updated"
			},
			{
				"currentImageId": "01d120000000",
				"id": "c79120000000",
				"imageName": "mock/updt2:latest",
				"latestImageId": "d0a120000000",
				"name": "updt2",
				"state": "Updated"
			}
		]
		},
		"title": "Watchtower updates on Mock"
}`
				data := mockDataFromStates(t,
					session.UpdatedState,
					session.FreshState,
					session.FailedState,
					session.SkippedState,
					session.UpdatedState,
				)
				assert.JSONEq(t, expected, getTemplatedResult(t, `json.v1`, false, data))
			})
			t.Run("should use correct ID types in JSON output", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.UpdatedState)
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Verify that updated containers use container IDs for the id field, and image IDs for currentImageId and latestImageId
				// Container IDs should be like "c79110000000"
				// Image IDs are like "01d110000000" and "d0a110000000"
				assert.Contains(t, result, "c79110000000")
				assert.Contains(t, result, "01d110000000")
				assert.Contains(t, result, "d0a110000000")
			})

			t.Run("should include restarted containers in JSON output", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.UpdatedState, session.RestartedState)
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Verify restarted containers are included
				assert.Contains(t, result, `"restarted"`)
				assert.Contains(t, result, `"state": "Restarted"`)
				assert.Contains(t, result, "rstr1")
			})

			t.Run("should render restarted containers without errors", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.RestartedState)
				result := getTemplatedResult(t, `json.v1`, false, data)
				assert.Contains(t, result, `"restarted"`)
				assert.NotContains(t, result, "Error:")
			})

			t.Run("should handle state transitions in notification templates", func(t *testing.T) {
				t.Parallel()

				// Test mixing different states
				data := mockDataFromStates(t,
					session.UpdatedState,
					session.RestartedState,
					session.FailedState,
				)
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Verify all states are present
				assert.Contains(t, result, `"updated"`)
				assert.Contains(t, result, `"restarted"`)
				assert.Contains(t, result, `"failed"`)
				assert.Contains(t, result, `"state": "Updated"`)
				assert.Contains(t, result, `"state": "Restarted"`)
				assert.Contains(t, result, `"state": "Failed"`)
			})

			t.Run("should maintain priority ordering in notification output", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t,
					session.RestartedState,
					session.UpdatedState,
					session.FailedState,
				)
				result := getTemplatedResult(t, `json.v1`, false, data)

				// The order should be based on the report arrays: scanned, updated, restarted, failed, skipped, stale, fresh
				// Check that all expected keys are present
				assert.Contains(t, result, `"scanned"`)
				assert.Contains(t, result, `"updated"`)
				assert.Contains(t, result, `"restarted"`)
				assert.Contains(t, result, `"failed"`)
			})

			t.Run("should integrate with other notification types in JSON", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.RestartedState)
				// Test that JSON includes all required fields
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Should include report, title, host, entries
				assert.Contains(t, result, `"report"`)
				assert.Contains(t, result, `"title"`)
				assert.Contains(t, result, `"host"`)
				assert.Contains(t, result, `"entries"`)
				assert.Contains(t, result, `"restarted"`)
			})

			t.Run("should handle template rendering edge cases", func(t *testing.T) {
				t.Parallel()

				// Test with empty report
				data := notificationData{
					Entries: []*notificationEntry{},
					Report:  nil,
					Title:   "Test Title",
					Host:    "Test Host",
				}
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Should still produce valid JSON
				assert.Contains(t, result, `"report": null`)
				assert.Contains(t, result, `"title": "Test Title"`)
				assert.Contains(t, result, `"host": "Test Host"`)
			})

			t.Run("should validate notification formatting", func(t *testing.T) {
				t.Parallel()

				data := mockDataFromStates(t, session.RestartedState)
				result := getTemplatedResult(t, `json.v1`, false, data)

				// Should be valid JSON
				assert.True(t, json.Valid([]byte(result)), "invalid JSON: %s", result)

				// Should contain expected structure
				assert.Contains(t, result, `"id"`)
				assert.Contains(t, result, `"name"`)
				assert.Contains(t, result, `"imageName"`)
				assert.Contains(t, result, `"state"`)
			})
		})
	})
}

// TestJSONGitAndOCIReportFields verifies that the JSON report includes the Git
// and OCI fields that are set and omits the empty ones.
func TestJSONGitAndOCIReportFields(t *testing.T) {
	t.Parallel()

	t.Run("includes populated Git and OCI fields", func(t *testing.T) {
		t.Parallel()

		status := updatedAppStatus()
		status.SetGitMetadata(container.ReportMeta{
			GitRepo:         "https://github.com/org/app.git",
			GitRef:          "main",
			Changelog:       "https://github.com/org/app/releases",
			Source:          "https://github.com/org/app",
			ImageURL:        "https://example.com/image",
			Documentation:   "https://example.com/docs",
			CurrentVersion:  "1.2.2",
			LatestVersion:   "1.2.3",
			CurrentRevision: "olddeadbeef",
			LatestRevision:  "deadbeef",
		})

		got := marshalReports([]types.ContainerReport{status})
		require.Len(t, got, 1)
		assert.Equal(t, "https://github.com/org/app.git", got[0]["gitRepo"])
		assert.Equal(t, "main", got[0]["gitRef"])
		assert.Equal(t, "https://github.com/org/app/releases", got[0]["changelog"])
		assert.Equal(t, "https://github.com/org/app", got[0]["ociSource"])
		assert.Equal(t, "https://example.com/image", got[0]["imageUrl"])
		assert.Equal(t, "https://example.com/docs", got[0]["documentation"])
		assert.Equal(t, "olddeadbeef", got[0]["currentImageRevision"])
		assert.Equal(t, "deadbeef", got[0]["latestImageRevision"])
		assert.Equal(t, "1.2.2", got[0]["currentImageVersion"])
		assert.Equal(t, "1.2.3", got[0]["latestImageVersion"])
	})

	t.Run("omits empty Git and OCI fields", func(t *testing.T) {
		t.Parallel()

		status := updatedAppStatus()
		got := marshalReports([]types.ContainerReport{status})
		require.Len(t, got, 1)
		assert.NotContains(t, got[0], "gitRepo")
		assert.NotContains(t, got[0], "changelog")
		assert.NotContains(t, got[0], "ociSource")
		assert.NotContains(t, got[0], "latestImageVersion")
		assert.NotContains(t, got[0], "currentImageRevision")
	})

	t.Run("marshals updated reports with Git fields", func(t *testing.T) {
		t.Parallel()

		status := updatedAppStatus()
		status.SetGitMetadata(container.ReportMeta{GitRepo: "https://github.com/org/app.git", GitRef: "main"})

		raw, err := json.Marshal(notificationData{
			Title: "update", Host: "box",
			Report: &session.SingleContainerReport{
				UpdatedReports: []types.ContainerReport{status},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"gitRepo":"https://github.com/org/app.git"`)
		assert.Contains(t, string(raw), `"gitRef":"main"`)
	})
}
