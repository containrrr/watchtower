package actions_test

import (
	"testing"
	"time"

	"github.com/containrrr/watchtower/internal/actions"

	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"

	"github.com/docker/docker/api/types"
	cli "github.com/docker/docker/client"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestActions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Actions Suite")
}

var _ = Describe("the actions package", func() {
	var dockerClient cli.CommonAPIClient
	var client MockClient
	BeforeSuite(func() {
		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
	})
	BeforeEach(func() {
		pullImages := false
		removeVolumes := false

		client = CreateMockClient(
			&TestData{},
			dockerClient,
			pullImages,
			removeVolumes,
		)
	})

	Describe("the check prerequisites method", func() {
		When("given an empty array", func() {
			It("should not do anything", func() {
				client.TestData.Containers = []container.Container{}
				err := actions.CheckForMultipleWatchtowerInstances(client, false, "")
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("given an array of one", func() {
			It("should not do anything", func() {
				client.TestData.Containers = []container.Container{
					CreateMockContainer(
						"test-container",
						"test-container",
						"watchtower",
						time.Now()),
				}
				err := actions.CheckForMultipleWatchtowerInstances(client, false, "")
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("given multiple containers", func() {
			BeforeEach(func() {
				pullImages := false
				removeVolumes := false
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
					dockerClient,
					pullImages,
					removeVolumes,
				)
			})

			It("should stop all but the latest one", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false, "")
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("deciding whether to cleanup images", func() {
			BeforeEach(func() {
				pullImages := false
				removeVolumes := false

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
					dockerClient,
					pullImages,
					removeVolumes,
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

func createMockContainer(id string, name string, image string, created time.Time) container.Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Image:   image,
			Name:    name,
			Created: created.String(),
		},
	}
	return *container.NewContainer(&content, nil)
}
