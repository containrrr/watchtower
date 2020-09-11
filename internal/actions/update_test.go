package actions_test

import (
	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/types"
	cli "github.com/docker/docker/client"
	"time"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the update action", func() {
	var dockerClient cli.CommonAPIClient
	var client MockClient

	BeforeEach(func() {
		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
	})

	When("watchtower has been instructed to clean up", func() {
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
							"fake-image:latest",
							time.Now().AddDate(0, 0, -1),
							make([]string, 0)),
						CreateMockContainer(
							"test-container-02",
							"test-container-02",
							"fake-image:latest",
							time.Now(),
							make([]string, 0)),
						CreateMockContainer(
							"test-container-02",
							"test-container-02",
							"fake-image:latest",
							time.Now(),
							make([]string, 0)),
					},
				},
				dockerClient,
				pullImages,
				removeVolumes,
			)
		})

		When("there are multiple containers using the same image", func() {
			It("should only try to remove the image once", func() {

				err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(1))
			})
		})

		When("there are multiple containers using different images", func() {
			It("should try to remove each of them", func() {
				client.TestData.Containers = append(
					client.TestData.Containers,
					CreateMockContainer(
						"unique-test-container",
						"unique-test-container",
						"unique-fake-image:latest",
						time.Now(),
						make([]string, 0),
					),
				)
				err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(2))
			})
		})	
	})

	When("there are containers with and without links", func() {
		links := [7][]string{
			{},
			{"k-container-01"},
			{"k-container-02"},
			{},
			{"t-container-01"},
			{"t-container-02"},
			{},
		}
		BeforeEach(func() {
			pullImages := false
			removeVolumes := false
			client = CreateMockClient(
				&TestData{
					NameOfContainerToKeep: "",
					Containers: []container.Container{
						CreateMockContainer(
							"k-container-03",
							"k-container-03",
							"fake-image:latest",
							time.Now().Add(time.Second * 4),
							links[2],),
						CreateMockContainer(
							"k-container-02",
							"k-container-02",
							"fake-image:latest",
							time.Now().Add(time.Second * 2),
							links[1],),
						CreateMockContainer(
							"k-container-01",
							"k-container-01",
							"fake-image:latest",
							time.Now(),
							links[0],),

						CreateMockContainer(
							"t-container-03",
							"t-container-03",
							"fake-image-2:latest",
							time.Now().Add(time.Second * 4),
							links[5],),
						CreateMockContainer(
							"t-container-02",
							"t-container-02",
							"fake-image-2:latest",
							time.Now().Add(time.Second * 2),
							links[4],),
						CreateMockContainer(
							"t-container-01",
							"t-container-01",
							"fake-image-2:latest",
							time.Now(),
							links[3],),

						CreateMockContainer(
							"x-container-01",
							"x-container-01",
							"fake-image-1:latest",
							time.Now(),
							links[6],),
						CreateMockContainer(
							"x-container-02",
							"x-container-02",
							"fake-image-1:latest",
							time.Now().Add(time.Second * 2),
							links[6],),
						CreateMockContainer(
							"x-container-03",
							"x-container-03",
							"fake-image-1:latest",
							time.Now().Add(time.Second * 4),
							links[6],),
					},
				},
				dockerClient,
				pullImages,
				removeVolumes,
			)
		})

		When("there are multiple containers with links", func() {
			It("should create appropriate dependency sorted lists", func() {
				dependencySortedGraphs, err := actions.PrepareContainerList(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())

				var output [][]string

				for _, i := range dependencySortedGraphs {
					var inner []string
					for _, j := range i {
						inner = append(inner, j.Name())
					}
					output = append(output,inner)
				}

				ExpectedOutput := [][]string{
					{"k-container-01", "k-container-02", "k-container-03",},
					{"t-container-01", "t-container-02", "t-container-03",},
					{"x-container-01",},
					{"x-container-02",},
					{"x-container-03",},
				}

				Expect(output).To(Equal(ExpectedOutput))
			})
		})

		When("there are multiple containers using the same image", func() {
			It("should stop and restart containers in a correct order", func() {
				err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())

				ExpectedStopOutput := []string{
					"k-container-03",
					"k-container-02",
					"k-container-01",
					"t-container-03",
					"t-container-02",
					"t-container-01",
					"x-container-01",
					"x-container-02",
					"x-container-03",
				}

				ExpectedRestartOutput := []string{
					"k-container-01",
					"k-container-02",
					"k-container-03",
					"t-container-01",
					"t-container-02",
					"t-container-03",
					"x-container-01",
					"x-container-02",
					"x-container-03",
				}

				Expect(client.TestData.StopOrder).To(Equal(ExpectedStopOutput))
				Expect(client.TestData.RestartOrder).To(Equal(ExpectedRestartOutput))
			})
		})
	})
})
