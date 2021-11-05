package actions_test

import (
	"github.com/sirupsen/logrus"
	"testing"
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/pkg/container"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestActions(t *testing.T) {
	RegisterFailHandler(Fail)
	logrus.SetOutput(GinkgoWriter)
	RunSpecs(t, "Actions Suite")
}

var _ = Describe("the actions package", func() {
	Describe("the check prerequisites method", func() {
		When("given an empty array", func() {
			It("should not do anything", func() {
				client := CreateMockClient(
					&TestData{},
					// pullImages:
					false,
					// removeVolumes:
					false,
				)
				Expect(actions.CheckForMultipleWatchtowerInstances(client, false, "")).To(Succeed())
			})
		})
		When("given an array of one", func() {
			It("should not do anything", func() {
				client := CreateMockClient(
					&TestData{
						Containers: []container.Container{
							CreateMockContainer(
								"test-container",
								"test-container",
								"watchtower",
								time.Now()),
						},
					},
					// pullImages:
					false,
					// removeVolumes:
					false,
				)
				Expect(actions.CheckForMultipleWatchtowerInstances(client, false, "")).To(Succeed())
			})
		})
		When("given multiple containers", func() {
			var client MockClient
			BeforeEach(func() {
				client = CreateMockClient(
					&TestData{
						NameOfContainerToKeep: "test-container-02",
						Containers: []container.Container{
							CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"watchtower",
								time.Now().AddDate(0, 0, -1)),
							CreateMockContainer(
								"test-container-02",
								"test-container-02",
								"watchtower",
								time.Now()),
						},
					},
					// pullImages:
					false,
					// removeVolumes:
					false,
				)
			})

			It("should stop all but the latest one", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false, "")
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("deciding whether to cleanup images", func() {
			var client MockClient
			BeforeEach(func() {
				client = CreateMockClient(
					&TestData{
						Containers: []container.Container{
							CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"watchtower",
								time.Now().AddDate(0, 0, -1)),
							CreateMockContainer(
								"test-container-02",
								"test-container-02",
								"watchtower",
								time.Now()),
						},
					},
					// pullImages:
					false,
					// removeVolumes:
					false,
				)
			})
			It("should try to delete the image if the cleanup flag is true", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, true, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImage()).To(BeTrue())
			})
			It("should not try to delete the image if the cleanup flag is false", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImage()).To(BeFalse())
			})
		})
	})
})
