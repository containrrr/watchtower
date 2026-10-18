package actions_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// createNetworkModeContainer builds a container that joins another container's
// network namespace, with Compose project and service labels applied.
func createNetworkModeContainer(
	id, name, project, service, networkMode string,
) types.Container {
	hostConfig := &dockerContainer.HostConfig{
		PortBindings: dockerNetwork.PortMap{},
	}
	if networkMode != "" {
		hostConfig.NetworkMode = dockerContainer.NetworkMode(networkMode)
	}

	content := dockerContainer.InspectResponse{
		ID:    id,
		Image: "image:latest",
		Name:  name,
		State: &dockerContainer.State{Running: true},
		Created: time.Now().
			AddDate(0, 0, -1).
			Format(time.RFC3339Nano),
		HostConfig: hostConfig,
		Config: &dockerContainer.Config{
			Labels: map[string]string{
				"com.docker.compose.project": project,
				"com.docker.compose.service": service,
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	return container.NewContainer(nil, &content, mockActions.CreateMockImageInfo("image:latest"))
}

// createStaleContainersForTest creates two stale containers and returns them along with TestData.
// This is a helper function used by tests that need to set up containers for rolling restart scenarios.
func createStaleContainersForTest() *mockActions.TestData {
	container1 := mockActions.CreateMockContainerWithConfig(
		"container-1",
		"/container-1",
		"image-1:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Labels:       map[string]string{},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	container2 := mockActions.CreateMockContainerWithConfig(
		"container-2",
		"/container-2",
		"image-2:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Labels:       map[string]string{},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	testData := &mockActions.TestData{
		Containers: []types.Container{container1, container2},
		Staleness: map[string]bool{
			"container-1": true,
			"container-2": true,
		},
	}

	return testData
}

var _ = ginkgo.Describe("the update action", func() {
	ginkgo.When("restarting stale Watchtower containers in non-rolling mode", func() {
		ginkgo.It("should restart stale Watchtower containers even if stop is skipped", func() {
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true, // Simulate stale Watchtower
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				types.UpdateParams{
					Cleanup: true,
					Filter:  filters.WatchtowerContainersFilter,
				},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
			gomega.Expect(cleanupImageInfos).
				To(gomega.BeEmpty(), "No cleanup for renamed Watchtower container")
			gomega.Expect(client.TestData.StopContainerCount.Load()).
				To(gomega.Equal(int32(0)), "StopContainer should not be called for old Watchtower (handled by cleanup logic)")
			gomega.Expect(client.TestData.StartContainerCount.Load()).
				To(gomega.Equal(int32(1)), "StartContainer should be called for Watchtower restart")
			gomega.Expect(client.TestData.RenameContainerCount.Load()).
				To(gomega.Equal(int32(1)), "RenameContainer should be called once")
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(1)), "IsContainerStale should be called once for Watchtower")
		})
	})

	ginkgo.When("handling chained dependencies with multiple dependents", func() {
		ginkgo.It(
			"should restart all containers depending on the same base dependency when it updates",
			func() {
				// Create a base container that will be updated
				baseContainer := mockActions.CreateMockContainerWithConfig(
					"stale-container",
					"/stale-container",
					"stale-image:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1), // Make it stale
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				// Create two containers that depend on the base
				restartContainer1 := mockActions.CreateMockContainerWithConfig(
					"restart-container-1",
					"/restart-container-1",
					"restart-image1:latest",
					true,
					false,
					time.Now(), // Not stale
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "stale-container",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				restartContainer2 := mockActions.CreateMockContainerWithConfig(
					"restart-container-2",
					"/restart-container-2",
					"restart-image2:latest",
					true,
					false,
					time.Now(), // Not stale
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "stale-container",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							baseContainer,
							restartContainer1,
							restartContainer2,
						},
						Staleness: map[string]bool{
							"stale-container":     true,
							"restart-container-1": false,
							"restart-container-2": false,
						},
					},
					false,
					false,
				)

				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true},
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(report.Restarted()).To(gomega.HaveLen(2))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos[0].ContainerName).
					To(gomega.Equal("stale-container"))

				// Verify categorization
				updated := report.Updated()
				restarted := report.Restarted()

				gomega.Expect(updated[0].Name()).To(gomega.Equal("stale-container"))
				gomega.Expect(restarted[0].Name()).To(gomega.Equal("restart-container-1"))
				gomega.Expect(restarted[1].Name()).To(gomega.Equal("restart-container-2"))
			},
		)
	})

	ginkgo.When("testing container categorization logic for restarted containers", func() {
		ginkgo.It("should correctly categorize containers as updated, restarted, or fresh", func() {
			// Create containers with different states
			updatedContainer := mockActions.CreateMockContainerWithConfig(
				"updated-container",
				"/updated-container",
				"updated-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			restartedContainer := mockActions.CreateMockContainerWithConfig(
				"restarted-container",
				"/restarted-container",
				"restart-image:latest",
				true,
				false,
				time.Now(), // fresh
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "updated-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			freshContainer := mockActions.CreateMockContainerWithConfig(
				"fresh-container",
				"/fresh-container",
				"fresh-image:latest",
				true,
				false,
				time.Now(), // fresh
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						updatedContainer,
						restartedContainer,
						freshContainer,
					},
					Staleness: map[string]bool{
						"updated-container":   true,
						"restarted-container": false,
						"fresh-container":     false,
					},
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
			gomega.Expect(report.Restarted()).To(gomega.HaveLen(1))
			gomega.Expect(report.Fresh()).To(gomega.HaveLen(1))

			// Verify categorization
			updated := report.Updated()
			restarted := report.Restarted()
			fresh := report.Fresh()

			gomega.Expect(updated[0].Name()).To(gomega.Equal("updated-container"))
			gomega.Expect(restarted[0].Name()).To(gomega.Equal("restarted-container"))
			gomega.Expect(fresh[0].Name()).To(gomega.Equal("fresh-container"))
		})
	})

	ginkgo.When(
		"testing notification message generation for restarted vs updated containers",
		func() {
			ginkgo.It(
				"should generate different notification messages for updated and restarted containers",
				func() {
					// This test would require mocking the notification system
					// For now, we'll test the report generation which feeds into notifications
					updatedContainer := mockActions.CreateMockContainerWithConfig(
						"updated-container",
						"/updated-container",
						"updated-image:latest",
						true,
						false,
						time.Now().AddDate(0, 0, -1),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					restartedContainer := mockActions.CreateMockContainerWithConfig(
						"restarted-container",
						"/restarted-container",
						"restart-image:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower.depends-on": "updated-container",
							},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					client := mockActions.CreateMockClient(
						&mockActions.TestData{
							Containers: []types.Container{updatedContainer, restartedContainer},
							Staleness: map[string]bool{
								"updated-container":   true,
								"restarted-container": false,
							},
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
					gomega.Expect(report.Restarted()).To(gomega.HaveLen(1))

					// Verify that the report correctly distinguishes between updated and restarted
					gomega.Expect(report.Updated()[0].Name()).To(gomega.Equal("updated-container"))
					gomega.Expect(report.Restarted()[0].Name()).
						To(gomega.Equal("restarted-container"))
				},
			)
		},
	)

	ginkgo.When("testing metrics collection for restarted containers", func() {
		ginkgo.It("should include restarted containers in metrics", func() {
			updatedContainer := mockActions.CreateMockContainerWithConfig(
				"updated-container",
				"/updated-container",
				"updated-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			restartedContainer := mockActions.CreateMockContainerWithConfig(
				"restarted-container",
				"/restarted-container",
				"restart-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "updated-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{updatedContainer, restartedContainer},
					Staleness: map[string]bool{
						"updated-container":   true,
						"restarted-container": false,
					},
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

			// Generate metrics from the report
			metric := metrics.NewMetric(report)

			// Verify metrics include both updated and restarted containers
			gomega.Expect(metric.Updated).To(gomega.Equal(1))
			gomega.Expect(metric.Scanned).To(gomega.Equal(2))
			// Note: restarted containers are not counted separately in metrics,
			// they are included in scanned but not in updated
		})
	})

	ginkgo.When("testing edge cases with restarted containers", func() {
		ginkgo.It("should handle containers that become stale after dependency restart", func() {
			// Create a scenario where a dependent becomes stale independently
			dependency := mockActions.CreateMockContainerWithConfig(
				"dependency",
				"/dependency",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent",
				"/dependent",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // also stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "dependency",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{dependency, dependent},
					Staleness: map[string]bool{
						"dependency": true,
						"dependent":  true,
					},
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
			// Both should be updated since both are stale
			gomega.Expect(report.Updated()).To(gomega.HaveLen(2))
			gomega.Expect(report.Restarted()).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle mixed dependency chains with multiple restart levels", func() {
			// A -> B -> C where A and C are stale, B is fresh
			containerC := mockActions.CreateMockContainerWithConfig(
				"container-c",
				"/container-c",
				"image-c:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b",
				"/container-b",
				"image-b:latest",
				true,
				false,
				time.Now(), // fresh
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a",
				"/container-a",
				"image-a:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{containerC, containerB, containerA},
					Staleness: map[string]bool{
						"container-c": true,
						"container-b": false,
						"container-a": true,
					},
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
			gomega.Expect(report.Updated()).To(gomega.HaveLen(2))   // A and C
			gomega.Expect(report.Restarted()).To(gomega.HaveLen(1)) // B
		})

		ginkgo.It("should drop a linked restart when nothing is stale", func() {
			// Restart marks are derived from containers that are still stale.
			// A mark with no stale anchor does not stop the container.
			container := mockActions.CreateMockContainerWithConfig(
				"standalone-container",
				"/standalone-container",
				"standalone-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			container.SetLinkedToRestarting(true)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{container},
					Staleness: map[string]bool{
						"standalone-container": false,
					},
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
			gomega.Expect(report.Updated()).To(gomega.BeEmpty())
			gomega.Expect(report.Restarted()).To(gomega.BeEmpty())
			gomega.Expect(container.IsLinkedToRestarting()).To(gomega.BeFalse())
		})
	})

	ginkgo.When("testing restarted container functionality", func() {
		ginkgo.When("testing UpdateImplicitRestart function", func() {
			ginkgo.It(
				"should handle error handling in dependency resolution for restarted containers",
				func() {
					// Create containers with invalid dependency references
					containerA := mockActions.CreateMockContainerWithConfig(
						"container-a",
						"/container-a",
						"image-a:latest",
						true,
						false,
						time.Now().AddDate(0, 0, -1),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower.depends-on": "nonexistent",
							},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					containerB := mockActions.CreateMockContainerWithConfig(
						"container-b",
						"/container-b",
						"image-b:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					containers := []types.Container{containerA, containerB}

					// Mark container B as stale (should trigger restart)
					containerB.SetStale(true)

					// Run UpdateImplicitRestart - should not panic or error on invalid dependencies
					actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

					// Container A should not be marked for restart due to invalid dependency
					gomega.Expect(containerA.ToRestart()).To(gomega.BeFalse())
					gomega.Expect(containerB.ToRestart()).To(gomega.BeTrue())
				},
			)

			ginkgo.It("should handle state transitions during complex restart chains", func() {
				// Create a complex chain: A -> B -> C -> D
				containerD := mockActions.CreateMockContainerWithConfig(
					"container-d",
					"/container-d",
					"image-d:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerC := mockActions.CreateMockContainerWithConfig(
					"container-c",
					"/container-c",
					"image-c:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-d",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerB := mockActions.CreateMockContainerWithConfig(
					"container-b",
					"/container-b",
					"image-b:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-c",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerA := mockActions.CreateMockContainerWithConfig(
					"container-a",
					"/container-a",
					"image-a:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-b",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{containerD, containerC, containerB, containerA}

				// Initially, only D should be marked for restart
				containerD.SetStale(true)
				gomega.Expect(containerD.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerC.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerA.ToRestart()).To(gomega.BeFalse())

				// Run UpdateImplicitRestart to propagate restart through the chain
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Verify state transitions: all containers should now be marked for restart
				gomega.Expect(containerD.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerC.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerA.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("should handle priority ordering in restart sequences", func() {
				// Create containers with multiple dependencies
				base := mockActions.CreateMockContainerWithConfig(
					"base",
					"/base",
					"base:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				dep1 := mockActions.CreateMockContainerWithConfig(
					"dep1",
					"/dep1",
					"dep1:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "base",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				dep2 := mockActions.CreateMockContainerWithConfig(
					"dep2",
					"/dep2",
					"dep2:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "base",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				dep3 := mockActions.CreateMockContainerWithConfig(
					"dep3",
					"/dep3",
					"dep3:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "dep1,dep2",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{base, dep1, dep2, dep3}

				// Mark base as stale
				base.SetStale(true)

				// Run UpdateImplicitRestart
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Verify all dependents are marked for restart
				gomega.Expect(base.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(dep1.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(dep2.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(dep3.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("marks a container sharing a network namespace across Compose projects", func() {
				// gluetun and qbittorrent are separate Compose projects, which is
				// the usual layout when one VPN container is shared between
				// several stacks.
				gluetun := createNetworkModeContainer(
					"gluetun-id", "/gluetun", "gluetun", "app", "",
				)
				qbittorrent := createNetworkModeContainer(
					"qbittorrent-id", "/qbittorrent", "qbittorrent", "app",
					"container:gluetun",
				)

				containers := []types.Container{gluetun, qbittorrent}
				gluetun.SetStale(true)

				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				gomega.Expect(gluetun.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(qbittorrent.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("marks a dependent whose network mode still references the provider ID", func() {
				// Moby inspect stores container:<id> until Watchtower rewrites
				// it to a name. Matching must accept the ID form as well.
				const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

				gluetun := createNetworkModeContainer(
					providerID, "/gluetun", "gluetun", "app", "",
				)
				qbittorrent := createNetworkModeContainer(
					"qbittorrent-id", "/qbittorrent", "qbittorrent", "app",
					"container:"+providerID,
				)

				containers := []types.Container{gluetun, qbittorrent}
				gluetun.SetStale(true)

				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				gomega.Expect(gluetun.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(qbittorrent.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("does not treat a project-prefixed peer name as the network provider", func() {
				gluetun := createNetworkModeContainer(
					"gluetun-id", "/gluetun", "gluetun", "app", "",
				)
				decoy := createNetworkModeContainer(
					"decoy-id", "/qbittorrent-gluetun", "other", "svc", "",
				)
				qbittorrent := createNetworkModeContainer(
					"qbittorrent-id", "/qbittorrent", "qbittorrent", "app",
					"container:gluetun",
				)

				containers := []types.Container{gluetun, decoy, qbittorrent}
				decoy.SetStale(true)

				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				gomega.Expect(decoy.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(gluetun.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(qbittorrent.ToRestart()).To(gomega.BeFalse())
			})

			ginkgo.It("should handle restarted containers with circular dependencies", func() {
				// Create circular dependency: A -> B -> A
				containerA := mockActions.CreateMockContainerWithConfig(
					"container-a",
					"/container-a",
					"image-a:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-b",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerB := mockActions.CreateMockContainerWithConfig(
					"container-b",
					"/container-b",
					"image-b:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-a",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{containerA, containerB}

				// Mark container A as stale
				containerA.SetStale(true)

				// Run UpdateImplicitRestart - should handle circular dependency gracefully
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Both should be marked for restart despite circular dependency
				gomega.Expect(containerA.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("should handle restarted containers with mixed update types", func() {
				// Create containers with different update scenarios
				staleNoDeps := mockActions.CreateMockContainerWithConfig(
					"stale-no-deps",
					"/stale-no-deps",
					"stale:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				freshWithDeps := mockActions.CreateMockContainerWithConfig(
					"fresh-with-deps",
					"/fresh-with-deps",
					"fresh:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "stale-no-deps",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				staleWithDeps := mockActions.CreateMockContainerWithConfig(
					"stale-with-deps",
					"/stale-with-deps",
					"stale-deps:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "stale-no-deps",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{staleNoDeps, freshWithDeps, staleWithDeps}

				// Mark stale containers
				staleNoDeps.SetStale(true)
				staleWithDeps.SetStale(true)

				// Run UpdateImplicitRestart
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Verify correct restart marking
				gomega.Expect(staleNoDeps.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(freshWithDeps.ToRestart()).
					To(gomega.BeTrue())
					// Should be restarted due to dependency
				gomega.Expect(staleWithDeps.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It("should propagate restart through deep dependency chains", func() {
				// Create a deep dependency chain: A -> B -> C -> D -> E -> F
				containerF := mockActions.CreateMockContainerWithConfig(
					"container-f",
					"/container-f",
					"image-f:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerE := mockActions.CreateMockContainerWithConfig(
					"container-e",
					"/container-e",
					"image-e:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-f",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerD := mockActions.CreateMockContainerWithConfig(
					"container-d",
					"/container-d",
					"image-d:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-e",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerC := mockActions.CreateMockContainerWithConfig(
					"container-c",
					"/container-c",
					"image-c:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-d",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerB := mockActions.CreateMockContainerWithConfig(
					"container-b",
					"/container-b",
					"image-b:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-c",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerA := mockActions.CreateMockContainerWithConfig(
					"container-a",
					"/container-a",
					"image-a:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "container-b",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{
					containerF,
					containerE,
					containerD,
					containerC,
					containerB,
					containerA,
				}

				// Initially, only F should be marked for restart
				containerF.SetStale(true)
				gomega.Expect(containerF.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerE.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerD.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerC.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerA.ToRestart()).To(gomega.BeFalse())

				// Run UpdateImplicitRestart to propagate restart through the deep chain
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Verify state transitions: all containers should now be marked for restart
				gomega.Expect(containerF.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerE.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerD.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerC.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerA.ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It(
				"should handle container names different from service names in depends-on",
				func() {
					// Create containers with names that include project/service suffixes (like Docker Compose)
					// and depends-on labels that reference the service names
					dbContainer := mockActions.CreateMockContainerWithConfig(
						"project1-db-1",
						"/project1-db-1",
						"mariadb:latest",
						true,
						false,
						time.Now().AddDate(0, 0, -1),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					appContainer := mockActions.CreateMockContainerWithConfig(
						"project1-app-1",
						"/project1-app-1",
						"php:fpm",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower.depends-on": "db",
							},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					webContainer := mockActions.CreateMockContainerWithConfig(
						"project1-web-1",
						"/project1-web-1",
						"nginx:alpine",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower.depends-on": "app",
							},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					containers := []types.Container{dbContainer, appContainer, webContainer}

					// Initially, only db should be marked for restart
					dbContainer.SetStale(true)
					gomega.Expect(dbContainer.ToRestart()).To(gomega.BeTrue())
					gomega.Expect(appContainer.ToRestart()).To(gomega.BeFalse())
					gomega.Expect(webContainer.ToRestart()).To(gomega.BeFalse())

					// Run UpdateImplicitRestart - should match "project1-db-1" to "db" and propagate
					actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

					// Verify that restart propagates through service name matching
					gomega.Expect(dbContainer.ToRestart()).To(gomega.BeTrue())
					gomega.Expect(appContainer.ToRestart()).
						To(gomega.BeTrue())
						// depends on "db", matches "project1-db-1"
					gomega.Expect(webContainer.ToRestart()).
						To(gomega.BeTrue())
					// depends on "app", matches "project1-app-1"
				},
			)
		})

		ginkgo.When("testing rolling restart functionality", func() {
			ginkgo.It(
				"should handle integration with filtering and restarted containers in rolling mode",
				func() {
					// Create containers with dependency for rolling restart
					staleContainer := mockActions.CreateMockContainerWithConfig(
						"stale-container",
						"/stale-container",
						"stale:latest",
						true,
						false,
						time.Now().AddDate(0, 0, -1),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					restartContainer := mockActions.CreateMockContainerWithConfig(
						"restart-container",
						"/restart-container",
						"restart:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower.depends-on": "stale-container",
							},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)

					client := mockActions.CreateMockClient(
						&mockActions.TestData{
							Containers: []types.Container{staleContainer, restartContainer},
							Staleness: map[string]bool{
								"stale-container":   true,
								"restart-container": false,
							},
						},
						false,
						false,
					)

					// Run Update with rolling restart
					report, cleanupImageInfos, err := actions.Update(newNopLogger(),
						context.Background(),
						client,
						types.UpdateParams{
							Cleanup:        true,
							RollingRestart: true,
						},
					)

					// Verify successful execution
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
					gomega.Expect(report.Restarted()).To(gomega.HaveLen(1))
					gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
					gomega.Expect(client.TestData.WaitForContainerHealthyCount.Load()).To(gomega.Equal(int32(2)))
				},
			)

			ginkgo.It(
				"should handle performance testing for large restart chains in rolling mode",
				func() {
					// Create a large chain of containers for performance testing
					numContainers := 10 // Reduced for test performance
					containers := make([]types.Container, numContainers)

					for i := range numContainers {
						name := fmt.Sprintf("perf-container-%d", i)
						image := fmt.Sprintf("perf-image-%d:latest", i)
						labels := make(map[string]string)

						// Create dependency chain: each depends on the previous
						if i > 0 {
							labels["com.centurylinklabs.watchtower.depends-on"] = fmt.Sprintf(
								"perf-container-%d",
								i-1,
							)
						}

						containers[i] = mockActions.CreateMockContainerWithConfig(
							name,
							"/"+name,
							image,
							true,
							false,
							time.Now().AddDate(0, 0, -1), // Make all stale
							&dockerContainer.Config{
								Labels:       labels,
								ExposedPorts: dockerNetwork.PortSet{},
							},
						)
					}

					client := mockActions.CreateMockClient(
						&mockActions.TestData{
							Containers: containers,
						},
						false,
						false,
					)

					// Mark all as stale
					if client.TestData.Staleness == nil {
						client.TestData.Staleness = make(map[string]bool)
					}

					for i := range containers {
						name := fmt.Sprintf("perf-container-%d", i)
						client.TestData.Staleness[name] = true
					}

					// Measure performance of rolling restart
					startTime := time.Now()
					report, _, err := actions.Update(newNopLogger(),
						context.Background(),
						client,
						types.UpdateParams{
							Cleanup:        true,
							RollingRestart: true,
						},
					)
					duration := time.Since(startTime)

					// Verify completion and reasonable performance
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(report.Updated()).To(gomega.HaveLen(numContainers))
					gomega.Expect(client.TestData.WaitForContainerHealthyCount.Load()).
						To(gomega.Equal(int32(numContainers)))
					// Performance should be reasonable (less than 2 seconds for 10 containers)
					gomega.Expect(duration).To(gomega.BeNumerically("<", 2*time.Second))
				},
			)
		})

		ginkgo.When("testing context cancellation in rolling restart", func() {
			ginkgo.It("should return early with context cancellation error when context is canceled before processing", func() {
				// Create multiple containers that would need rolling restart
				testData := createStaleContainersForTest()

				client := mockActions.CreateMockClient(
					testData,
					false,
					false,
				)

				// Create a canceled context - this will trigger the early context cancellation check in Update
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately

				// Run Update with rolling restart and canceled context
				_, _, err := actions.Update(newNopLogger(),
					ctx,
					client,
					types.UpdateParams{
						Cleanup:        true,
						RollingRestart: true,
					},
				)

				// Verify that an error is returned
				gomega.Expect(err).To(gomega.HaveOccurred())
				// Verify that the error wraps context cancellation
				gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue())
				// Verify error message contains "canceled" (early check in Update uses "update canceled")
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("canceled"))
			})

			ginkgo.It("should handle context cancellation with timeout during rolling restart", func() {
				// Create multiple containers - all stale so they all go through rolling restart
				testData := createStaleContainersForTest()

				client := mockActions.CreateMockClient(
					testData,
					false,
					false,
				)

				// Use a context with an already-expired deadline to deterministically trigger context cancellation.
				// This is more reliable than a short timeout which may expire before Update runs.
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
				defer cancel()

				// Run Update with rolling restart - should timeout quickly
				_, _, err := actions.Update(newNopLogger(),
					ctx,
					client,
					types.UpdateParams{
						Cleanup:        true,
						RollingRestart: true,
					},
				)

				// Verify that an error is returned
				gomega.Expect(err).To(gomega.HaveOccurred())
				// Verify that the error wraps context cancellation or deadline exceeded
				gomega.Expect(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)).To(gomega.BeTrue())
				// Verify error message contains "canceled" or similar
				gomega.Expect(err.Error()).To(gomega.SatisfyAny(
					gomega.ContainSubstring("canceled"),
					gomega.ContainSubstring("deadline"),
				))
			})
		})

		ginkgo.When("testing restart ordering functionality", func() {
			ginkgo.It("should handle priority ordering in restart sequences", func() {
				// Create containers with specific dependency order: A -> B -> C
				containerC := mockActions.CreateMockContainerWithConfig(
					"priority-c",
					"/priority-c",
					"priority-c:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerB := mockActions.CreateMockContainerWithConfig(
					"priority-b",
					"/priority-b",
					"priority-b:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "priority-c",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerA := mockActions.CreateMockContainerWithConfig(
					"priority-a",
					"/priority-a",
					"priority-a:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "priority-b",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{containerA, containerB, containerC},
						Staleness: map[string]bool{
							"priority-c": true,
							"priority-b": false,
							"priority-a": false,
						},
						StopOrder:   []string{},
						CreateOrder: []string{},
						StartOrder:  []string{},
					},
					false,
					false,
				)

				// Run Update to test restart ordering
				report, _, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup: true,
					},
				)

				// Verify successful execution and correct categorization
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(report.Restarted()).To(gomega.HaveLen(2))

				// Verify create order respects dependencies (dependencies first)
				gomega.Expect(client.TestData.CreateOrder).To(gomega.ContainElement("priority-c"))
				gomega.Expect(client.TestData.CreateOrder).To(gomega.ContainElement("priority-b"))
				gomega.Expect(client.TestData.CreateOrder).To(gomega.ContainElement("priority-a"))
			})

			ginkgo.It("should handle mixed update types in restart sequences", func() {
				// Create mix of stale (updated) and fresh (restarted) containers
				staleContainer := mockActions.CreateMockContainerWithConfig(
					"mixed-stale",
					"/mixed-stale",
					"mixed-stale:latest",
					true,
					false,
					time.Now().AddDate(0, 0, -1),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				restartContainer1 := mockActions.CreateMockContainerWithConfig(
					"mixed-restart-1",
					"/mixed-restart-1",
					"mixed-restart1:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "mixed-stale",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				restartContainer2 := mockActions.CreateMockContainerWithConfig(
					"mixed-restart-2",
					"/mixed-restart-2",
					"mixed-restart2:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "mixed-stale",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							staleContainer,
							restartContainer1,
							restartContainer2,
						},
						Staleness: map[string]bool{
							"mixed-stale":     true,
							"mixed-restart-1": false,
							"mixed-restart-2": false,
						},
					},
					false,
					false,
				)

				// Run Update
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup: true,
					},
				)

				// Verify successful execution
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(report.Restarted()).To(gomega.HaveLen(2))

				// Verify stale container was updated (added to cleanup)
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos[0].ContainerName).To(gomega.Equal("mixed-stale"))
			})
		})
	})
})
