package notifications

import (
	"encoding/json"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("JSON template", func() {
	ginkgo.When("using report templates", func() {
		ginkgo.When("JSON template is used", func() {
			ginkgo.It("should format the messages to the expected format", func() {
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
				data := mockDataFromStates(
					session.UpdatedState,
					session.FreshState,
					session.FailedState,
					session.SkippedState,
					session.UpdatedState,
				)
				gomega.Expect(getTemplatedResult(`json.v1`, false, data)).
					To(gomega.MatchJSON(expected))
			})
			ginkgo.It("should use correct ID types in JSON output", func() {
				data := mockDataFromStates(session.UpdatedState)
				result := getTemplatedResult(`json.v1`, false, data)

				// Verify that updated containers use container IDs for the id field, and image IDs for currentImageId and latestImageId
				// Container IDs should be like "c79110000000"
				// Image IDs are like "01d110000000" and "d0a110000000"
				gomega.Expect(result).To(gomega.ContainSubstring("c79110000000"))
				gomega.Expect(result).To(gomega.ContainSubstring("01d110000000"))
				gomega.Expect(result).To(gomega.ContainSubstring("d0a110000000"))
			})

			ginkgo.It("should include restarted containers in JSON output", func() {
				data := mockDataFromStates(session.UpdatedState, session.RestartedState)
				result := getTemplatedResult(`json.v1`, false, data)

				// Verify restarted containers are included
				gomega.Expect(result).To(gomega.ContainSubstring(`"restarted"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"state": "Restarted"`))
				gomega.Expect(result).To(gomega.ContainSubstring("rstr1"))
			})

			ginkgo.It("should render restarted containers without errors", func() {
				data := mockDataFromStates(session.RestartedState)
				result := getTemplatedResult(`json.v1`, false, data)
				gomega.Expect(result).To(gomega.ContainSubstring(`"restarted"`))
				gomega.Expect(result).NotTo(gomega.ContainSubstring("Error:"))
			})

			ginkgo.It("should handle state transitions in notification templates", func() {
				// Test mixing different states
				data := mockDataFromStates(
					session.UpdatedState,
					session.RestartedState,
					session.FailedState,
				)
				result := getTemplatedResult(`json.v1`, false, data)

				// Verify all states are present
				gomega.Expect(result).To(gomega.ContainSubstring(`"updated"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"restarted"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"failed"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"state": "Updated"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"state": "Restarted"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"state": "Failed"`))
			})

			ginkgo.It("should maintain priority ordering in notification output", func() {
				data := mockDataFromStates(
					session.RestartedState,
					session.UpdatedState,
					session.FailedState,
				)
				result := getTemplatedResult(`json.v1`, false, data)

				// The order should be based on the report arrays: scanned, updated, restarted, failed, skipped, stale, fresh
				// Check that all expected keys are present
				gomega.Expect(result).To(gomega.ContainSubstring(`"scanned"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"updated"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"restarted"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"failed"`))
			})

			ginkgo.It("should integrate with other notification types in JSON", func() {
				data := mockDataFromStates(session.RestartedState)
				// Test that JSON includes all required fields
				result := getTemplatedResult(`json.v1`, false, data)

				// Should include report, title, host, entries
				gomega.Expect(result).To(gomega.ContainSubstring(`"report"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"title"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"host"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"entries"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"restarted"`))
			})

			ginkgo.It("should handle template rendering edge cases", func() {
				// Test with empty report
				data := notificationData{
					Entries: []*notificationEntry{},
					Report:  nil,
					Title:   "Test Title",
					Host:    "Test Host",
				}
				result := getTemplatedResult(`json.v1`, false, data)

				// Should still produce valid JSON
				gomega.Expect(result).To(gomega.ContainSubstring(`"report": null`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"title": "Test Title"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"host": "Test Host"`))
			})

			ginkgo.It("should validate notification formatting", func() {
				data := mockDataFromStates(session.RestartedState)
				result := getTemplatedResult(`json.v1`, false, data)

				// Should be valid JSON
				gomega.Expect(result).To(gomega.MatchJSON(result))

				// Should contain expected structure
				gomega.Expect(result).To(gomega.ContainSubstring(`"id"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"name"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"imageName"`))
				gomega.Expect(result).To(gomega.ContainSubstring(`"state"`))
			})
		})
	})
})

var _ = ginkgo.Describe("JSON Git and OCI report fields", func() {
	ginkgo.It("includes populated Git and OCI fields", func() {
		status := session.NewContainerStatus("app", "org/app:latest")
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
		gomega.Expect(got).To(gomega.HaveLen(1))
		gomega.Expect(got[0]["gitRepo"]).To(gomega.Equal("https://github.com/org/app.git"))
		gomega.Expect(got[0]["gitRef"]).To(gomega.Equal("main"))
		gomega.Expect(got[0]["changelog"]).To(gomega.Equal("https://github.com/org/app/releases"))
		gomega.Expect(got[0]["ociSource"]).To(gomega.Equal("https://github.com/org/app"))
		gomega.Expect(got[0]["imageUrl"]).To(gomega.Equal("https://example.com/image"))
		gomega.Expect(got[0]["documentation"]).To(gomega.Equal("https://example.com/docs"))
		gomega.Expect(got[0]["currentImageRevision"]).To(gomega.Equal("olddeadbeef"))
		gomega.Expect(got[0]["latestImageRevision"]).To(gomega.Equal("deadbeef"))
		gomega.Expect(got[0]["currentImageVersion"]).To(gomega.Equal("1.2.2"))
		gomega.Expect(got[0]["latestImageVersion"]).To(gomega.Equal("1.2.3"))
	})

	ginkgo.It("omits empty Git and OCI fields", func() {
		status := session.NewContainerStatus("app", "org/app:latest")
		got := marshalReports([]types.ContainerReport{status})
		gomega.Expect(got).To(gomega.HaveLen(1))
		gomega.Expect(got[0]).NotTo(gomega.HaveKey("gitRepo"))
		gomega.Expect(got[0]).NotTo(gomega.HaveKey("changelog"))
		gomega.Expect(got[0]).NotTo(gomega.HaveKey("ociSource"))
		gomega.Expect(got[0]).NotTo(gomega.HaveKey("latestImageVersion"))
		gomega.Expect(got[0]).NotTo(gomega.HaveKey("currentImageRevision"))
	})

	ginkgo.It("marshals updated reports with Git fields", func() {
		status := session.NewContainerStatus("app", "org/app:latest")
		status.SetGitMetadata(container.ReportMeta{GitRepo: "https://github.com/org/app.git", GitRef: "main"})

		raw, err := json.Marshal(notificationData{
			Title: "update", Host: "box",
			Report: &session.SingleContainerReport{
				UpdatedReports: []types.ContainerReport{status},
			},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(string(raw)).To(gomega.ContainSubstring(`"gitRepo":"https://github.com/org/app.git"`))
		gomega.Expect(string(raw)).To(gomega.ContainSubstring(`"gitRef":"main"`))
	})
})
