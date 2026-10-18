package actions_test

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("updating a stale container beside a dependency cycle", func() {
	ginkgo.It("updates the stale container and does not fail the session", func() {
		containerA := mockActions.CreateMockContainerWithConfig(
			"cycle-a",
			"/cycle-a",
			"fake-image-a:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "cycle-b,cycle-c",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		containerB := mockActions.CreateMockContainerWithConfig(
			"cycle-b",
			"/cycle-b",
			"fake-image-b:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "cycle-a",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		containerC := mockActions.CreateMockContainerWithConfig(
			"cycle-c",
			"/cycle-c",
			"fake-image-c:latest",
			true,
			false,
			time.Now().AddDate(0, 0, -1),
			&dockerContainer.Config{
				Labels:       map[string]string{},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{containerA, containerB, containerC},
				Staleness:  map[string]bool{"cycle-c": true},
			},
			false,
			false,
		)

		report, _, err := actions.Update(newNopLogger(),
			context.Background(),
			client,
			types.UpdateParams{Cleanup: true},
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
		gomega.Expect(report.Updated()[0].Name()).To(gomega.Equal("cycle-c"))
	})
})
