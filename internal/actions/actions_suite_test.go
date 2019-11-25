package actions_test

import (
	"github.com/containrrr/watchtower/internal/actions"
	"testing"
	"time"

	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"

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
			&TestData {},
			dockerClient,
			pullImages,
			removeVolumes,
		)
	})

	Describe("the check prerequisites method", func() {
		When("given an empty array", func() {
			It("should not do anything", func() {
				client.TestData.Containers = []container.Container{}
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
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
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
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
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
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
				err := actions.CheckForMultipleWatchtowerInstances(client, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImage()).To(BeTrue())
			})
			It("should not try to delete the image if the cleanup flag is false", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
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

type mockClient struct {
	TestData      *TestData
	api           cli.CommonAPIClient
	pullImages    bool
	removeVolumes bool
}

type TestData struct {
	TriedToRemoveImage    bool
	NameOfContainerToKeep string
	Containers            []container.Container
}

func (client mockClient) ListContainers(f t.Filter) ([]container.Container, error) {
	return client.TestData.Containers, nil
}

func (client mockClient) StopContainer(c container.Container, d time.Duration) error {
	if c.Name() == client.TestData.NameOfContainerToKeep {
		return errors.New("tried to stop the instance we want to keep")
	}
	return nil
}
func (client mockClient) StartContainer(c container.Container) (string, error) {
	panic("Not implemented")
}

func (client mockClient) RenameContainer(c container.Container, s string) error {
	panic("Not implemented")
}

func (client mockClient) RemoveImage(c container.Container) error {
	client.TestData.TriedToRemoveImage = true
	return nil
}

func (client mockClient) GetContainer(containerID string) (container.Container, error) {
	return container.Container{}, nil
}

func (client mockClient) ExecuteCommand(containerID string, command string, timeout int) error {
	return nil
}

func (client mockClient) IsContainerStale(c container.Container) (bool, error) {
	panic("Not implemented")
}
