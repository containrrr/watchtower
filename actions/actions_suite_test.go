package actions_test

import (
	"errors"
	"testing"
	"time"

	"github.com/containrrr/watchtower/actions"
	"github.com/containrrr/watchtower/container"
	"github.com/containrrr/watchtower/container/mocks"
	"github.com/docker/docker/api/types"

	cli "github.com/docker/docker/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestActions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Actions Suite")
}

var _ = Describe("the actions package", func() {
	var dockerClient cli.CommonAPIClient
	var client mockClient
	BeforeSuite(func() {
		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
	})
	BeforeEach(func() {
		client = mockClient{
			api:        dockerClient,
			pullImages: false,
			TestData:   &TestData{},
		}
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
					createMockContainer(
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
				client = mockClient{
					api:        dockerClient,
					pullImages: false,
					TestData: &TestData{
						NameOfContainerToKeep: "test-container-02",
						Containers: []container.Container{
							createMockContainer(
								"test-container-01",
								"test-container-01",
								"watchtower",
								time.Now().AddDate(0, 0, -1)),
							createMockContainer(
								"test-container-02",
								"test-container-02",
								"watchtower",
								time.Now()),
						},
					},
				}
			})
			It("should stop all but the latest one", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("deciding whether to cleanup images", func() {
			BeforeEach(func() {
				client = mockClient{
					api:        dockerClient,
					pullImages: false,
					TestData: &TestData{
						Containers: []container.Container{
							createMockContainer(
								"test-container-01",
								"test-container-01",
								"watchtower",
								time.Now().AddDate(0, 0, -1)),
							createMockContainer(
								"test-container-02",
								"test-container-02",
								"watchtower",
								time.Now()),
						},
					},
				}
			})
			It("should try to delete the image if the cleanup flag is true", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImage).To(BeTrue())
			})
			It("should not try to delete the image if the cleanup flag is false", func() {
				err := actions.CheckForMultipleWatchtowerInstances(client, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImage).To(BeFalse())
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
	TestData   *TestData
	api        cli.CommonAPIClient
	pullImages bool
}

type TestData struct {
	TriedToRemoveImage    bool
	NameOfContainerToKeep string
	Containers            []container.Container
}

func (client mockClient) ListContainers(f container.Filter) ([]container.Container, error) {
	return client.TestData.Containers, nil
}

func (client mockClient) StopContainer(c container.Container, d time.Duration) error {
	if c.Name() == client.TestData.NameOfContainerToKeep {
		return errors.New("tried to stop the instance we want to keep")
	}
	return nil
}
func (client mockClient) StartContainer(c container.Container) error {
	panic("Not implemented")
}

func (client mockClient) RenameContainer(c container.Container, s string) error {
	panic("Not implemented")
}

func (client mockClient) RemoveImage(c container.Container) error {
	client.TestData.TriedToRemoveImage = true
	return nil
}

func (client mockClient) IsContainerStale(c container.Container) (bool, error) {
	panic("Not implemented")
}
