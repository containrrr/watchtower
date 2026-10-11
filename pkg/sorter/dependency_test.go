package sorter

import (
	"errors"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

var _ = ginkgo.Describe("DependencySorter", func() {
	ginkgo.Describe("Sort", func() {
		ginkgo.It("should sort a volumes-from dependent after a source referenced by Docker ID", func() {
			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			app := concreteLinkedContainer(appID, "/nextcloud-app", "nextcloud", "app", "")
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app}
			err := DependencySorter{}.Sort(testLog(), containers, false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal("nextcloud-app"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("nextcloud-nginx"))
		})

		ginkgo.It("should sort a volumes-from dependent after a source in another Compose project", func() {
			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			app := concreteLinkedContainer(appID, "/shared-app", "data", "app", "")
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app}
			err := DependencySorter{}.Sort(testLog(), containers, false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal("shared-app"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("nextcloud-nginx"))
		})

		ginkgo.It("should sort a volumes-from chain after compose depends_on", func() {
			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			postgres := concreteLinkedContainer(
				"postgres-id",
				"/nextcloud-db",
				"nextcloud",
				"db",
				"",
			)
			app := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:         appID,
				Name:       "/nextcloud-app",
				HostConfig: &dockerContainer.HostConfig{},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project":    "nextcloud",
					"com.docker.compose.service":    "app",
					"com.docker.compose.depends_on": "db",
				}},
			}, nil)
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app, postgres}
			err := DependencySorter{}.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal("nextcloud-db"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("nextcloud-app"))
			gomega.Expect(containers[2].Name()).To(gomega.Equal("nextcloud-nginx"))
		})

		ginkgo.It("should sort a volumes-from chain after watchtower depends-on", func() {
			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			postgres := concreteLinkedContainer(
				"postgres-id",
				"/nextcloud-db",
				"nextcloud",
				"db",
				"",
			)
			app := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:         appID,
				Name:       "/nextcloud-app",
				HostConfig: &dockerContainer.HostConfig{},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project":                "nextcloud",
					"com.docker.compose.service":                "app",
					"com.centurylinklabs.watchtower.depends-on": "nextcloud-db",
				}},
			}, nil)
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app, postgres}
			err := DependencySorter{}.Sort(testLog(), containers, false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal("nextcloud-db"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("nextcloud-app"))
			gomega.Expect(containers[2].Name()).To(gomega.Equal("nextcloud-nginx"))
		})

		ginkgo.It("should sort a network_mode dependent after a provider referenced by Docker ID", func() {
			const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

			gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
			qbittorrent := concreteLinkedContainer(
				"qbittorrent-id",
				"/qbittorrent",
				"qbittorrent",
				"app",
				"container:"+providerID,
			)

			containers := []types.Container{qbittorrent, gluetun}
			err := DependencySorter{}.Sort(testLog(), containers, false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal("gluetun"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("qbittorrent"))
		})

		ginkgo.It("should resolve container ID network_mode to the ID owner when a peer reuses that ID as its name", func() {
			const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

			gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
			decoy := concreteLinkedContainer("decoy-id", "/"+providerID, "other", "svc", "")
			qbittorrent := concreteLinkedContainer(
				"qbittorrent-id",
				"/qbittorrent",
				"qbittorrent",
				"app",
				"container:"+providerID,
			)

			_, indegree, adjacency, _, err := buildDependencyGraph(
				testLog(),
				[]types.Container{qbittorrent, decoy, gluetun},
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(indegree["qbittorrent-app"]).To(gomega.Equal(1))
			gomega.Expect(adjacency["gluetun-vpn"]).To(gomega.ContainElement("qbittorrent-app"))
			gomega.Expect(adjacency["other-svc"]).NotTo(gomega.ContainElement("qbittorrent-app"))
		})

		ginkgo.It("should sort containers with no dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c3.EXPECT().Name().Return("c3")
			c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().IsWatchtower().Return(false)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c3", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2, c3}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(3))
			// Order may vary since no dependencies
		})

		ginkgo.It("should sort containers with simple dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(2))
			gomega.Expect(containers[0].Name()).To(gomega.Equal("c2"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It("should sort containers with complex dependency chains", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2", "c3"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return([]string{"c3"})
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c3.EXPECT().Name().Return("c3")
			c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().IsWatchtower().Return(false)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c3", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2, c3}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(3))
			gomega.Expect(containers[0].Name()).To(gomega.Equal("c3"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("c2"))
			gomega.Expect(containers[2].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It("should detect circular dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("circular reference detected"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("c1 -> c2 -> c1"))
		})

		ginkgo.It("should ignore Compose depends_on but honor Watchtower labels when useComposeDependsOn is false", func() {
			// c1 has a Watchtower explicit depends-on label referencing c2.
			// Links(false) still returns c2 because Watchtower labels are checked
			// before the Compose depends_on label in the Links() method.
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(false).Return([]string{"c2"}) // Watchtower label honored
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// c2 has no dependencies.
			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(false).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(2))
			// c1 depends on c2, so c2 must come before c1
			gomega.Expect(containers[0].Name()).To(gomega.Equal("c2"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It("should place Watchtower containers last", func() {
			watchtower := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			watchtower.EXPECT().Name().Return("watchtower")
			watchtower.EXPECT().IsWatchtower().Return(true)

			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{watchtower, c1, c2}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(3))
			gomega.Expect(containers[0].Name()).To(gomega.Equal("c2"))
			gomega.Expect(containers[1].Name()).To(gomega.Equal("c1"))
			gomega.Expect(containers[2].Name()).To(gomega.Equal("watchtower"))
		})

		ginkgo.It("should handle empty container list", func() {
			containers := []types.Container{}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle single container", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1}
			ds := DependencySorter{}
			err := ds.Sort(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))
			gomega.Expect(containers[0].Name()).To(gomega.Equal("c1"))
		})
	})

	ginkgo.Describe("sortByDependencies", func() {
		ginkgo.It("should sort containers with no dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
		})

		ginkgo.It("should sort containers with simple dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			gomega.Expect(result[0].Name()).To(gomega.Equal("c2"))
			gomega.Expect(result[1].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It("should sort containers with complex chains", func() {
			a := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			a.EXPECT().Name().Return("a")
			a.EXPECT().ID().Return(types.ContainerID("id-a"))
			a.EXPECT().Links(true).Return([]string{"b"})
			a.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/a", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			b := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			b.EXPECT().Name().Return("b")
			b.EXPECT().ID().Return(types.ContainerID("id-b"))
			b.EXPECT().Links(true).Return([]string{"c"})
			b.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/b", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c.EXPECT().Name().Return("c")
			c.EXPECT().ID().Return(types.ContainerID("id-c"))
			c.EXPECT().Links(true).Return(nil)
			c.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{a, b, c}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(3))
			gomega.Expect(result[0].Name()).To(gomega.Equal("c"))
			gomega.Expect(result[1].Name()).To(gomega.Equal("b"))
			gomega.Expect(result[2].Name()).To(gomega.Equal("a"))
		})

		ginkgo.It("should detect circular dependencies", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			_, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("circular reference detected"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("c1 -> c2 -> c1"))
		})

		ginkgo.It("should handle empty list", func() {
			containers := []types.Container{}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle single container", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It("should handle disconnected components", func() {
			// Component 1: A -> B
			a := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			a.EXPECT().Name().Return("a")
			a.EXPECT().ID().Return(types.ContainerID("id-a"))
			a.EXPECT().Links(true).Return([]string{"b"})
			a.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/a", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			b := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			b.EXPECT().Name().Return("b")
			b.EXPECT().ID().Return(types.ContainerID("id-b"))
			b.EXPECT().Links(true).Return(nil)
			b.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/b", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// Component 2: C -> D
			c := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c.EXPECT().Name().Return("c")
			c.EXPECT().ID().Return(types.ContainerID("id-c"))
			c.EXPECT().Links(true).Return([]string{"d"})
			c.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			d := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			d.EXPECT().Name().Return("d")
			d.EXPECT().ID().Return(types.ContainerID("id-d"))
			d.EXPECT().Links(true).Return(nil)
			d.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/d", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{a, b, c, d}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(4))

			resultNames := make([]string, len(result))
			for i, c := range result {
				resultNames[i] = c.Name()
			}

			assertOrderBefore(resultNames, "b", "a")
			assertOrderBefore(resultNames, "d", "c")
		})

		ginkgo.It("should skip self-referencing containers without creating cycles", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c1"}) // Self-reference
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).
				ToNot(gomega.HaveOccurred())
				// Self-reference is filtered, not a cycle
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Name()).To(gomega.Equal("c1"))
		})

		ginkgo.It(
			"should skip self-reference via prefix matching for replica-named containers",
			func() {
				// Container named "myapp-1" linking to "myapp" should NOT create a self-dependency
				// The prefix matching would match "myapp-1" when looking for "myapp" replicas,
				// but the self-reference guard should skip it
				myapp := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				myapp.EXPECT().Name().Return("myapp-1").Maybe()
				myapp.EXPECT().ID().Return(types.ContainerID("id-myapp")).Maybe()
				myapp.EXPECT().Links(true).Return([]string{"myapp"})
				myapp.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{
						Name:   "/myapp-1",
						Config: &dockerContainer.Config{Labels: map[string]string{}},
					})

				containers := []types.Container{myapp}
				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containerMap).To(gomega.HaveLen(1))

				// Self-reference via prefix matching should be skipped
				// indegree["myapp-1"] should be 0 (not incremented)
				gomega.Expect(indegree["myapp-1"]).To(gomega.Equal(0))
				// adjacency["myapp-1"] should NOT contain "myapp-1"
				gomega.Expect(adjacency["myapp-1"]).ToNot(gomega.ContainElement("myapp-1"))
			},
		)

		ginkgo.It(
			"should skip self-reference via prefix matching for db replica-named containers",
			func() {
				// Container named "db-1" linking to "db" should NOT create a self-dependency
				// Similar to the myapp case, this tests another common naming pattern
				db := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db.EXPECT().Name().Return("db-1").Maybe()
				db.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
				db.EXPECT().Links(true).Return([]string{"db"})
				db.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{
						Name:   "/db-1",
						Config: &dockerContainer.Config{Labels: map[string]string{}},
					})

				containers := []types.Container{db}
				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containerMap).To(gomega.HaveLen(1))

				// Self-reference via prefix matching should be skipped
				// indegree["db-1"] should be 0 (not incremented)
				gomega.Expect(indegree["db-1"]).To(gomega.Equal(0))
				// adjacency["db-1"] should NOT contain "db-1"
				gomega.Expect(adjacency["db-1"]).ToNot(gomega.ContainElement("db-1"))
			},
		)

		ginkgo.It("should handle containers with project-prefixed names and service links", func() {
			// test-db-1 (no dependencies)
			db := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			db.EXPECT().Name().Return("test-db-1")
			db.EXPECT().ID().Return(types.ContainerID("id-db"))
			db.EXPECT().Links(true).Return(nil)
			db.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/test-db-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// app-1 depends on "db"
			app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			app.EXPECT().Name().Return("app-1")
			app.EXPECT().ID().Return(types.ContainerID("id-app"))
			app.EXPECT().Links(true).Return([]string{"db"})
			app.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/app-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// test-web-1 depends on "app"
			web := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			web.EXPECT().Name().Return("test-web-1")
			web.EXPECT().ID().Return(types.ContainerID("id-web"))
			web.EXPECT().Links(true).Return([]string{"app"})
			web.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/test-web-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{web, app, db} // Unsorted order
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(3))
			// Dependencies first: db, then app (depends on db), then web (depends on app)
			gomega.Expect(result[0].Name()).To(gomega.Equal("test-db-1"))
			gomega.Expect(result[1].Name()).To(gomega.Equal("app-1"))
			gomega.Expect(result[2].Name()).To(gomega.Equal("test-web-1"))
		})

		ginkgo.It("should handle containers with service names", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("container1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"web"}) // Link to service name
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("container2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web"}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			gomega.Expect(result[0].Name()).To(gomega.Equal("container2")) // web service
			gomega.Expect(result[1].Name()).To(gomega.Equal("container1")) // depends on web
		})

		ginkgo.It("should handle containers with replicas without collision", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("myproject-web-1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/myproject-web-1", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web", "com.docker.compose.project": "myproject"}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("myproject-web-2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/myproject-web-2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web", "com.docker.compose.project": "myproject"}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			// Order may vary since no dependencies
		})

		ginkgo.It(
			"should handle prefix matching linking service name to multiple replicas deterministically",
			func() {
				// App container that depends on "db" service
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().Name().Return("app").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"db"})
				app.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/app", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				// Multiple db replicas
				db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db1.EXPECT().Name().Return("db-1").Maybe()
				db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
				db1.EXPECT().Links(true).Return(nil)
				db1.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/db-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db2.EXPECT().Name().Return("db-2").Maybe()
				db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
				db2.EXPECT().Links(true).Return(nil)
				db2.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/db-2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				db3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db3.EXPECT().Name().Return("db-3").Maybe()
				db3.EXPECT().ID().Return(types.ContainerID("id-db3")).Maybe()
				db3.EXPECT().Links(true).Return(nil)
				db3.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/db-3", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				containers := []types.Container{app, db1, db2, db3}
				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containerMap).To(gomega.HaveLen(4))

				// Verify app depends on all db replicas
				gomega.Expect(indegree["app"]).To(gomega.Equal(3))
				gomega.Expect(adjacency["db-1"]).To(gomega.ContainElement("app"))
				gomega.Expect(adjacency["db-2"]).To(gomega.ContainElement("app"))
				gomega.Expect(adjacency["db-3"]).To(gomega.ContainElement("app"))

				// Verify db replicas have no dependencies
				gomega.Expect(indegree["db-1"]).To(gomega.Equal(0))
				gomega.Expect(indegree["db-2"]).To(gomega.Equal(0))
				gomega.Expect(indegree["db-3"]).To(gomega.Equal(0))
			},
		)

		ginkgo.It("should handle containers with service names containing leading slashes", func() {
			// Test that containers with service names containing leading slashes are handled correctly
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"web-service"}) // Link to normalized service name
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("container2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "/web-service"}}})

				// Malformed service name with leading slash

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			gomega.Expect(result[0].Name()).
				To(gomega.Equal("container2"))
				// web-service container first
			gomega.Expect(result[1].Name()).To(gomega.Equal("container1")) // depends on web-service
		})

		ginkgo.It(
			"should handle containers with container names containing leading slashes",
			func() {
				// Test containers with container names containing leading slashes
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("container1").Maybe()
				c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
				c1.EXPECT().Links(true).Return([]string{"/web"}) // Link with leading slash
				c1.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("web").Maybe()
				c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
				c2.EXPECT().Links(true).Return(nil)
				c2.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/web", Config: &dockerContainer.Config{Labels: map[string]string{}}})

					// Container name with leading slash

				containers := []types.Container{c1, c2}
				result, err := sortByDependencies(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(result).To(gomega.HaveLen(2))
				gomega.Expect(result[0].Name()).
					To(gomega.Equal("web"))
					// web container first
				gomega.Expect(result[1].Name()).To(gomega.Equal("container1")) // depends on web
			},
		)

		ginkgo.It("should handle containers with empty names falling back to IDs", func() {
			// Test containers with empty names that fall back to container IDs
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("").Maybe() // Empty name
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"id-c2"}) // Link to other container's ID
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("").Maybe() // Empty name
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			gomega.Expect(result[0].ID()).
				To(gomega.Equal(types.ContainerID("id-c2")))
				// dependency first
			gomega.Expect(result[1].ID()).
				To(gomega.Equal(types.ContainerID("id-c1")))
			// dependent second
		})

		ginkgo.It("should handle containers with malformed link targets", func() {
			// Test containers with links to non-existent containers
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().
				Links(true).
				Return([]string{"non-existent", "/also-non-existent"})
				// Links to non-existent containers
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("container2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			// Order may vary since no valid dependencies
		})

		ginkgo.It("should handle containers with network mode dependencies", func() {
			// Test containers with network mode dependencies
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"web"}) // Mock includes network dependency
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/container1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("web").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/web",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(2))
			gomega.Expect(result[0].Name()).To(gomega.Equal("web"))        // web container first
			gomega.Expect(result[1].Name()).To(gomega.Equal("container1")) // depends on web
		})

		ginkgo.It("should handle containers with mixed dependency sources", func() {
			// Test containers with dependencies from labels, links, and network mode
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("app").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			c1.EXPECT().
				Links(true).
				Return([]string{"db", "cache", "redis", "proxy"})
				// Mock all dependencies
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("db").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/db",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c3.EXPECT().Name().Return("cache").Maybe()
			c3.EXPECT().ID().Return(types.ContainerID("id-cache")).Maybe()
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/cache",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c4 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c4.EXPECT().Name().Return("redis").Maybe()
			c4.EXPECT().ID().Return(types.ContainerID("id-redis")).Maybe()
			c4.EXPECT().Links(true).Return(nil)
			c4.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/redis",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c5 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c5.EXPECT().Name().Return("proxy").Maybe()
			c5.EXPECT().ID().Return(types.ContainerID("id-proxy")).Maybe()
			c5.EXPECT().Links(true).Return(nil)
			c5.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/proxy",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{c1, c2, c3, c4, c5}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(5))
			// Dependencies should come before dependents
			resultNames := make([]string, len(result))
			for i, c := range result {
				resultNames[i] = c.Name()
			}
			// app should be last
			gomega.Expect(resultNames[len(resultNames)-1]).To(gomega.Equal("app"))
		})

		ginkgo.It(
			"should skip containers with self-referencing dependencies without creating cycles",
			func() {
				// Test containers that reference themselves (self-reference is filtered as safety net)
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("container1").Maybe()
				c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
				c1.EXPECT().Links(true).Return([]string{"container1"}) // Self-reference
				c1.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				containers := []types.Container{c1}
				result, err := sortByDependencies(testLog(), containers, true)
				gomega.Expect(err).
					ToNot(gomega.HaveOccurred())
					// Self-reference is filtered, not a cycle
				gomega.Expect(result).To(gomega.HaveLen(1))
				gomega.Expect(result[0].Name()).To(gomega.Equal("container1"))
			},
		)

		ginkgo.It("should handle large number of containers with no dependencies", func() {
			// Test performance and correctness with many containers
			containers := make([]types.Container, 100)

			for i := range 100 {
				c := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c.EXPECT().Name().Return(fmt.Sprintf("container%d", i)).Maybe()
				c.EXPECT().ID().Return(types.ContainerID(fmt.Sprintf("id-c%d", i))).Maybe()
				c.EXPECT().Links(true).Return(nil)
				c.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: fmt.Sprintf("/container%d", i), Config: &dockerContainer.Config{Labels: map[string]string{}}})
				containers[i] = c
			}

			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(100))
		})

		ginkgo.It("should handle containers with empty identifiers", func() {
			// This tests that containers with empty names fall back to using container ID as identifier
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("") // Empty name
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("") // Empty name
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred()) // Should not detect false cycle
			gomega.Expect(result).To(gomega.HaveLen(2))
		})

		ginkgo.It("should resolve a service name shared by two projects to both", func() {
			// app depending on "db"
			app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "db"},
				},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// db1 from project1
			db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_db_1",
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.service": "db",
					"com.docker.compose.project": "project1",
				}},
			})
			db1.EXPECT().Name().Return("project1_db_1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)

			// db2 from project2
			db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project2_db_1",
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.service": "db",
					"com.docker.compose.project": "project2",
				}},
			})
			db2.EXPECT().Name().Return("project2_db_1").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db1, db2}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// "db" names the db service of both projects, so app depends on each.
			gomega.Expect(containerMap).To(gomega.HaveLen(3))
			gomega.Expect(indegree["app"]).To(gomega.Equal(2))
			gomega.Expect(adjacency).To(gomega.Equal(map[string][]string{
				"project1-db": {"app"},
				"project2-db": {"app"},
			}))
		})

		ginkgo.It("should detect circular dependencies via watchtower labels", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/c1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "c2"},
				},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/c2",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "c1"},
				},
			})
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})

			containers := []types.Container{c1, c2}

			_, err := sortByDependencies(testLog(), containers, true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("circular reference detected"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("c1 -> c2 -> c1"))
		})

		ginkgo.It("should handle dependencies to filtered containers", func() {
			app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "db"},
				},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// Only app in containers list, db not included
			containers := []types.Container{app}

			containerMap, indegree, _, _, err := buildDependencyGraph(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containerMap).To(gomega.HaveLen(1))
			gomega.Expect(indegree["app"]).To(gomega.Equal(0))
		})
	})
})

var _ = ginkgo.Describe("ResolveContainerIdentifier", func() {
	ginkgo.It("should return service name when present", func() {
		mockContainer := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{"com.docker.compose.service": "web"},
			},
		})
		result := container.ResolveContainerIdentifier(mockContainer)
		gomega.Expect(result).To(gomega.Equal("web"))
	})

	ginkgo.It("should return container name when no service label", func() {
		mockContainer := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		gomega.Expect(result).To(gomega.Equal("container1"))
	})

	ginkgo.It("should return container name when labels are nil", func() {
		mockContainer := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: nil,
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		gomega.Expect(result).To(gomega.Equal("container1"))
	})

	ginkgo.It("should return container name when service label is empty", func() {
		mockContainer := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{"com.docker.compose.service": ""},
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		gomega.Expect(result).To(gomega.Equal("container1"))
	})
})

var _ = ginkgo.Describe("Prefix Matching Issues", func() {
	ginkgo.It(
		"should not match containers with similar names that are not Docker Compose replicas (issue-1161)",
		func() {
			// App container that depends on "watchtower-test-database"
			app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			app.EXPECT().Name().Return("watchtower-test-app1").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"watchtower-test-database"})
			app.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/watchtower-test-app1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			// First database container (exact match)
			db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			db1.EXPECT().Name().Return("watchtower-test-database").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)
			db1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/watchtower-test-database",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			// Second database container with similar name (should NOT be matched)
			db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			db2.EXPECT().Name().Return("watchtower-test-database2").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)
			db2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/watchtower-test-database2",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{app, db1, db2}
			containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containerMap).To(gomega.HaveLen(3))

			// Verify app depends ONLY on watchtower-test-database, not on watchtower-test-database2
			gomega.Expect(indegree["watchtower-test-app1"]).To(gomega.Equal(1))
			gomega.Expect(adjacency["watchtower-test-database"]).
				To(gomega.ContainElement("watchtower-test-app1"))
			gomega.Expect(adjacency["watchtower-test-database2"]).To(gomega.BeEmpty())

			// Verify db1 and db2 have no dependencies
			gomega.Expect(indegree["watchtower-test-database"]).To(gomega.Equal(0))
			gomega.Expect(indegree["watchtower-test-database2"]).To(gomega.Equal(0))
		},
	)

	ginkgo.It("should match Docker Compose-style numeric replica suffixes", func() {
		// App container that depends on "myapp-db"
		app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		app.EXPECT().Name().Return("myapp-app1").Maybe()
		app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
		app.EXPECT().Links(true).Return([]string{"myapp-db"})
		app.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-app1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		// Database replicas with numeric suffixes (should be matched)
		db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		db1.EXPECT().Name().Return("myapp-db-1").Maybe()
		db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
		db1.EXPECT().Links(true).Return(nil)
		db1.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-db-1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		db2.EXPECT().Name().Return("myapp-db-2").Maybe()
		db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
		db2.EXPECT().Links(true).Return(nil)
		db2.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-db-2",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		containers := []types.Container{app, db1, db2}
		containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(containerMap).To(gomega.HaveLen(3))

		// Verify app depends on BOTH db replicas (since they have numeric suffixes)
		gomega.Expect(indegree["myapp-app1"]).To(gomega.Equal(2))
		gomega.Expect(adjacency["myapp-db-1"]).To(gomega.ContainElement("myapp-app1"))
		gomega.Expect(adjacency["myapp-db-2"]).To(gomega.ContainElement("myapp-app1"))
	})
})

var _ = ginkgo.Describe("Identifier Collision Issues", func() {
	ginkgo.Describe("ResolveContainerIdentifier", func() {
		ginkgo.It(
			"should return different identifiers for containers from different projects with same service name",
			func() {
				// Container from project "app1" with service "web"
				mockContainer1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				mockContainer1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service":     "web",
							"com.docker.compose.project":     "app1",
							"com.docker.compose.version":     "3.8",
							"com.docker.compose.config-hash": "abc123",
						},
					},
				})
				mockContainer1.EXPECT().Name().Return("app1_web_1")

				// Container from project "app2" with same service "web"
				mockContainer2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				mockContainer2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service":     "web",
							"com.docker.compose.project":     "app2",
							"com.docker.compose.version":     "3.8",
							"com.docker.compose.config-hash": "def456",
						},
					},
				})
				mockContainer2.EXPECT().Name().Return("app2_web_1")

				result1 := container.ResolveContainerIdentifier(mockContainer1)
				result2 := container.ResolveContainerIdentifier(mockContainer2)

				// Containers from different projects should return different identifiers
				gomega.Expect(result1).To(gomega.Equal("app1-web"))
				gomega.Expect(result2).To(gomega.Equal("app2-web"))
				gomega.Expect(result1).ToNot(gomega.Equal(result2))
			},
		)
	})

	ginkgo.Describe("buildDependencyGraph", func() {
		ginkgo.It(
			"should treat containers from different projects as separate entities in dependency graph",
			func() {
				// Create two containers from different projects with same service name
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/app1_web_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "web",
							"com.docker.compose.project": "app1",
						},
					},
				})
				c1.EXPECT().Name().Return("app1_web_1").Maybe()
				c1.EXPECT().Links(true).Return(nil)

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/app2_web_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "web",
							"com.docker.compose.project": "app2",
						},
					},
				})
				c2.EXPECT().Name().Return("app2_web_1").Maybe()
				c2.EXPECT().Links(true).Return(nil)

				containers := []types.Container{c1, c2}

				containerMap, indegree, _, normalizedMap, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Containers from different projects should have different identifiers
				ident1 := container.ResolveContainerIdentifier(c1)
				ident2 := container.ResolveContainerIdentifier(c2)

				gomega.Expect(ident1).To(gomega.Equal("app1-web"))
				gomega.Expect(ident2).To(gomega.Equal("app2-web"))

				// Both containers should be in the map as separate entities
				gomega.Expect(containerMap).To(gomega.HaveLen(2))
				gomega.Expect(containerMap["app1-web"]).To(gomega.Equal(c1))
				gomega.Expect(containerMap["app2-web"]).To(gomega.Equal(c2))

				// Verify indegree reflects separate entities
				gomega.Expect(indegree).To(gomega.HaveLen(2))
				gomega.Expect(indegree["app1-web"]).To(gomega.Equal(0))
				gomega.Expect(indegree["app2-web"]).To(gomega.Equal(0))

				// The normalizedMap should show containers map to their respective identifiers
				gomega.Expect(normalizedMap).To(gomega.HaveLen(2))
				gomega.Expect(normalizedMap[c1]).To(gomega.Equal("app1-web"))
				gomega.Expect(normalizedMap[c2]).To(gomega.Equal("app2-web"))
			},
		)

		ginkgo.It(
			"should maintain backward compatibility for containers without project labels",
			func() {
				// Container with service label but no project label
				mockContainer := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "web",
							// No project label
						},
					},
				})

				result := container.ResolveContainerIdentifier(mockContainer)

				// Should return just the service name, same as before
				gomega.Expect(result).To(gomega.Equal("web"))
			},
		)

		ginkgo.It(
			"should support exact matching for cross-project dependencies",
			func() {
				// Container that links to exact container name
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				app.EXPECT().Name().Return("app").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"project1-db"})

				// DB container from project1
				db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project1_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project1",
						},
					},
				})
				db1.EXPECT().Name().Return("project1_db_1").Maybe()
				db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
				db1.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, db1}

				containerMap, indegree, adjacency, normalizedMap, err := buildDependencyGraph(testLog(), containers,
					true,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify identifiers
				gomega.Expect(container.ResolveContainerIdentifier(app)).To(gomega.Equal("app"))
				gomega.Expect(container.ResolveContainerIdentifier(db1)).
					To(gomega.Equal("project1-db"))

				// All containers should be in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(2))

				// app should have indegree 1 (depends on db1)
				gomega.Expect(indegree["app"]).To(gomega.Equal(1))

				// db1 should have app as dependent
				gomega.Expect(adjacency["project1-db"]).To(gomega.ContainElement("app"))

				// Verify normalizedMap
				gomega.Expect(normalizedMap).To(gomega.HaveLen(2))
			},
		)
	})

	ginkgo.Describe("IdentifierCollisionError", func() {
		ginkgo.It("should format error message correctly", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().Name().Return("container1")
			c1.EXPECT().ID().Return(types.ContainerID("id1"))

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().Name().Return("container2")
			c2.EXPECT().ID().Return(types.ContainerID("id2"))

			containers := []types.Container{c1, c2}

			err := IdentifierCollisionError{
				DuplicateIdentifier: "test-id",
				AffectedContainers:  containers,
			}

			expected := "identifier collision detected: 'test-id' used by containers: container1 (id1), container2 (id2)"
			gomega.Expect(err.Error()).To(gomega.Equal(expected))
		})
	})

	ginkgo.Describe("buildDependencyGraph with collisions", func() {
		ginkgo.It(
			"should return IdentifierCollisionError when containers have same normalized identifier",
			func() {
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/c1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				c1.EXPECT().Name().Return("c1").Maybe()
				c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/c1", // Same name
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				c2.EXPECT().Name().Return("c1").Maybe()
				c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()

				containers := []types.Container{c1, c2}

				_, _, _, _, err := buildDependencyGraph(testLog(), containers, true)

				gomega.Expect(err).To(gomega.HaveOccurred())

				var collisionErr IdentifierCollisionError
				gomega.Expect(err).To(gomega.BeAssignableToTypeOf(collisionErr))
				gomega.Expect(func() IdentifierCollisionError {
					var target IdentifierCollisionError

					_ = errors.As(err, &target)

					return target
				}().DuplicateIdentifier).
					To(gomega.Equal("c1"))
				gomega.Expect(func() IdentifierCollisionError {
					var target IdentifierCollisionError

					_ = errors.As(err, &target)

					return target
				}().AffectedContainers).
					To(gomega.HaveLen(2))
			},
		)

		ginkgo.It("should succeed when containers have different identifiers", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()
			c1.EXPECT().Links(true).Return(nil)

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c2",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c2.EXPECT().Name().Return("c2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{c1, c2}

			containerMap, indegree, adjacency, normalizedMap, err := buildDependencyGraph(testLog(), containers,
				true,
			)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containerMap).To(gomega.HaveLen(2))
			gomega.Expect(indegree).To(gomega.HaveLen(2))
			gomega.Expect(adjacency).To(gomega.BeEmpty()) // No links set up
			gomega.Expect(normalizedMap).To(gomega.HaveLen(2))
		})
	})

	ginkgo.Describe("sortByDependencies with collisions", func() {
		ginkgo.It("should propagate IdentifierCollisionError from buildDependencyGraph", func() {
			c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()

			c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1", // Same name
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c2.EXPECT().Name().Return("c1").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()

			containers := []types.Container{c1, c2}

			_, err := sortByDependencies(testLog(), containers, true)

			gomega.Expect(err).To(gomega.HaveOccurred())

			var collisionErr IdentifierCollisionError
			gomega.Expect(err).To(gomega.BeAssignableToTypeOf(collisionErr))
		})
	})
})

var _ = ginkgo.Describe("isPositiveInteger", func() {
	ginkgo.DescribeTable("validates positive integers correctly",
		func(input string, expected bool) {
			result := IsPositiveInteger(input)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry("should return true for single digit positive integer", "1", true),
		ginkgo.Entry("should return true for multi-digit positive integer", "123", true),
		ginkgo.Entry("should return true for large positive integer", "999", true),
		ginkgo.Entry("should return true for zero-prefixed positive integer", "01", true),
		ginkgo.Entry("should return false for zero", "0", false),
		ginkgo.Entry("should return false for negative number", "-1", false),
		ginkgo.Entry("should return false for alphabetic string", "abc", false),
		ginkgo.Entry("should return false for alphanumeric string", "1a", false),
		ginkgo.Entry("should return false for empty string", "", false),
		ginkgo.Entry("should return false for decimal number", "1.5", false),
	)
})

var _ = ginkgo.Describe("extractServiceName", func() {
	ginkgo.DescribeTable("extracts service name from container identifier",
		func(input, expected string) {
			result := ExtractServiceName(input)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry(
			"should return simple identifier without project prefix as-is",
			"postgres",
			"postgres",
		),
		ginkgo.Entry(
			"should extract service name from project-service identifier",
			"postgresql-postgres",
			"postgres",
		),
		ginkgo.Entry(
			"should extract service name from project-service-replica identifier",
			"project-service-1",
			"service",
		),
		ginkgo.Entry(
			"should extract service name from complex project-service-replica identifier",
			"myapp-database-2",
			"database",
		),
		ginkgo.Entry(
			"should return empty string for empty input",
			"",
			"",
		),
		ginkgo.Entry(
			"should handle single character identifier",
			"a",
			"a",
		),
		ginkgo.Entry(
			"should handle identifier with multiple dashes",
			"my-complex-app-name-service-3",
			"service",
		),
		ginkgo.Entry(
			"should handle two-part identifier without replica",
			"myapp-web",
			"web",
		),
		ginkgo.Entry(
			"should handle two-part identifier with replica",
			"web-1",
			"web",
		),
		ginkgo.Entry(
			"should return last part when no replica suffix",
			"project-web",
			"web",
		),
		ginkgo.Entry(
			"should handle replica number 0 (not positive, so treat as name)",
			"service-0",
			"0",
		),
		ginkgo.Entry(
			"should handle large replica number",
			"project-service-999",
			"service",
		),
	)
})

var _ = ginkgo.Describe("Service-Only Matching", func() {
	ginkgo.Describe("buildDependencyGraph", func() {
		ginkgo.It(
			"should match single unambiguous cross-project service by service name",
			func() {
				// App container from project1 that depends on "db" service
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project1_app_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "app",
							"com.docker.compose.project": "project1",
						},
					},
				})
				app.EXPECT().Name().Return("project1_app_1").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"db"}) // Link to just "db" service name

				// DB container from project2 (different project, but only one "db" service)
				db := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project2_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project2",
						},
					},
				})
				db.EXPECT().Name().Return("project2_db_1").Maybe()
				db.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
				db.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, db}

				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify containers are in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(2))

				// App should have indegree 1 (depends on db via service-only match)
				gomega.Expect(indegree["project1-app"]).To(gomega.Equal(1))

				// DB should have app as dependent
				gomega.Expect(adjacency["project2-db"]).To(gomega.ContainElement("project1-app"))
			},
		)

		ginkgo.It(
			"should match watchtower label referencing service name without project prefix",
			func() {
				// App container with watchtower depends-on label referencing just "postgres"
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/myapp",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "postgres",
						},
					},
				})
				app.EXPECT().Name().Return("myapp").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"postgres"})

				// Postgres container with project prefix in identifier
				postgres := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				postgres.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/postgresql_postgres_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "postgres",
							"com.docker.compose.project": "postgresql",
						},
					},
				})
				postgres.EXPECT().Name().Return("postgresql_postgres_1").Maybe()
				postgres.EXPECT().ID().Return(types.ContainerID("id-postgres")).Maybe()
				postgres.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, postgres}

				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify containers are in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(2))

				// App should have indegree 1 (depends on postgres via service-only match)
				gomega.Expect(indegree["myapp"]).To(gomega.Equal(1))

				// Postgres should have app as dependent
				gomega.Expect(adjacency["postgresql-postgres"]).To(gomega.ContainElement("myapp"))
			},
		)

		ginkgo.It(
			"should match every project's service when several share the service name",
			func() {
				// App container that depends on "db" service
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				app.EXPECT().Name().Return("app").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"db"})

				// DB container from project1
				db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project1_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project1",
						},
					},
				})
				db1.EXPECT().Name().Return("project1_db_1").Maybe()
				db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
				db1.EXPECT().Links(true).Return(nil)

				// DB container from project2 (same service name, different project)
				db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project2_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project2",
						},
					},
				})
				db2.EXPECT().Name().Return("project2_db_1").Maybe()
				db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
				db2.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, db1, db2}

				containerMap, indegree, _, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify all containers are in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(3))

				// App depends on the db service of both projects
				gomega.Expect(indegree["app"]).To(gomega.Equal(2))

				// Both db containers should have indegree 0
				gomega.Expect(indegree["project1-db"]).To(gomega.Equal(0))
				gomega.Expect(indegree["project2-db"]).To(gomega.Equal(0))
			},
		)

		ginkgo.It(
			"should prefer exact match over service-only match",
			func() {
				// App container that links to exact identifier "project1-db"
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				app.EXPECT().Name().Return("app").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"project1-db"})

				// DB container from project1
				db1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project1_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project1",
						},
					},
				})
				db1.EXPECT().Name().Return("project1_db_1").Maybe()
				db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
				db1.EXPECT().Links(true).Return(nil)

				// DB container from project2 (should NOT be matched)
				db2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project2_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project2",
						},
					},
				})
				db2.EXPECT().Name().Return("project2_db_1").Maybe()
				db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
				db2.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, db1, db2}

				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify all containers are in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(3))

				// App should have indegree 1 (exact match to project1-db only)
				gomega.Expect(indegree["app"]).To(gomega.Equal(1))

				// Only project1-db should have app as dependent
				gomega.Expect(adjacency["project1-db"]).To(gomega.ContainElement("app"))
				gomega.Expect(adjacency["project2-db"]).ToNot(gomega.ContainElement("app"))
			},
		)

		ginkgo.It(
			"should prefer replica match over service-only match",
			func() {
				// App container that links to "db" (should match db-1 replica, not project1-db)
				app := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				app.EXPECT().Name().Return("app").Maybe()
				app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
				app.EXPECT().Links(true).Return([]string{"db"})

				// DB replica container (db-1 pattern)
				dbReplica := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				dbReplica.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name:   "/db-1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})
				dbReplica.EXPECT().Name().Return("db-1").Maybe()
				dbReplica.EXPECT().ID().Return(types.ContainerID("id-dbreplica")).Maybe()
				dbReplica.EXPECT().Links(true).Return(nil)

				// DB container with project prefix (service name also "db")
				dbProject := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				dbProject.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Name: "/project_db_1",
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "db",
							"com.docker.compose.project": "project",
						},
					},
				})
				dbProject.EXPECT().Name().Return("project_db_1").Maybe()
				dbProject.EXPECT().ID().Return(types.ContainerID("id-dbproject")).Maybe()
				dbProject.EXPECT().Links(true).Return(nil)

				containers := []types.Container{app, dbReplica, dbProject}

				containerMap, indegree, adjacency, _, err := buildDependencyGraph(testLog(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Verify all containers are in the map
				gomega.Expect(containerMap).To(gomega.HaveLen(3))

				// App should have indegree 1 (replica match to db-1)
				gomega.Expect(indegree["app"]).To(gomega.Equal(1))

				// db-1 should have app as dependent (replica match)
				gomega.Expect(adjacency["db-1"]).To(gomega.ContainElement("app"))
			},
		)
	})
})

func assertOrderBefore(names []string, first, second string) {
	gomega.Expect(indexOf(names, first)).To(gomega.BeNumerically("<", indexOf(names, second)))
}

func indexOf(names []string, target string) int {
	for i, name := range names {
		if name == target {
			return i
		}
	}

	return -1
}

// mockLinkedContainer builds a mock with Compose labels and fixed Links() output.
// useComposeLinks selects Links(true) vs Links(false) for the useComposeDependsOn path.
func mockLinkedContainer(
	name, id, project, service string,
	links []string,
	useComposeLinks bool,
) *mockTypes.MockContainer {
	c := mockTypes.NewMockContainer(ginkgo.GinkgoT())
	c.EXPECT().Name().Return(name).Maybe()
	c.EXPECT().ID().Return(types.ContainerID(id)).Maybe()
	c.EXPECT().IsWatchtower().Return(false).Maybe()

	if useComposeLinks {
		c.EXPECT().Links(true).Return(links).Maybe()
	} else {
		c.EXPECT().Links(false).Return(links).Maybe()
	}

	labels := map[string]string{}
	if project != "" {
		labels["com.docker.compose.project"] = project
	}

	if service != "" {
		labels["com.docker.compose.service"] = service
	}

	c.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Name:   "/" + name,
		Config: &dockerContainer.Config{Labels: labels},
	}).Maybe()

	return c
}

// concreteLinkedContainer builds a *container.Container so Docker ID aliases
// in buildLinkMatchIndexes are exercised. networkMode is the HostConfig
// NetworkMode value, such as container:<id>.
func concreteLinkedContainer(
	id, name, project, service, networkMode string,
) types.Container {
	labels := map[string]string{}
	if project != "" {
		labels["com.docker.compose.project"] = project
	}

	if service != "" {
		labels["com.docker.compose.service"] = service
	}

	hostConfig := &dockerContainer.HostConfig{}
	if networkMode != "" {
		hostConfig.NetworkMode = dockerContainer.NetworkMode(networkMode)
	}

	return container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:         id,
		Name:       name,
		HostConfig: hostConfig,
		Config:     &dockerContainer.Config{Labels: labels},
	}, nil)
}

var _ = ginkgo.Describe("FindMatchingIdentifiers", func() {
	// Exhaustive link x identifier permutations across Compose stacks, multi-segment
	// services, replicas, bare container names, and ambiguous cross-project cases.
	ginkgo.DescribeTable(
		"matches identifiers for each link form",
		func(link string, identifiers, expected []string) {
			matches := FindMatchingIdentifiers(link, identifiers)
			if expected == nil {
				gomega.Expect(matches).To(gomega.BeEmpty())

				return
			}

			gomega.Expect(matches).To(gomega.Equal(expected))
		},
		// Exact
		ginkgo.Entry("exact project-service key",
			"project-db", []string{"project-db", "other-db"}, []string{"project-db"}),
		ginkgo.Entry("exact bare container name",
			"net-proxy", []string{"net-proxy", "web-app"}, []string{"net-proxy"}),
		// Replicas
		ginkgo.Entry("replica suffixes from project-service link",
			"project-db", []string{"project-db-1", "project-db-2", "other"},
			[]string{"project-db-1", "project-db-2"}),
		ginkgo.Entry("non-numeric hyphen suffix is not a replica",
			"project-db", []string{"project-db-backup"}, nil),
		ginkgo.Entry("numeric-looking middle segment is not a replica of shorter prefix",
			"project", []string{"project-db-1"}, nil),
		// Unhyphenated service-only (ExtractServiceName)
		ginkgo.Entry("unhyphenated service against single project-service key",
			"db", []string{"project1-db"}, []string{"project1-db"}),
		ginkgo.Entry("unhyphenated service against multi-segment project-service key",
			"proxy", []string{"myproject-net-proxy"}, []string{"myproject-net-proxy"}),
		ginkgo.Entry("unhyphenated service ambiguous across two projects",
			"db", []string{"project1-db", "project2-db"}, nil),
		ginkgo.Entry("unhyphenated service with three projects still ambiguous",
			"cache", []string{"a-cache", "b-cache", "c-cache"}, nil),
		// Multi-segment link → project-service suffix
		ginkgo.Entry("multi-segment link against project-service key",
			"net-proxy", []string{"myproject-net-proxy", "myproject-web"},
			[]string{"myproject-net-proxy"}),
		ginkgo.Entry("multi-segment link sole candidate",
			"net-proxy", []string{"myproject-net-proxy"}, []string{"myproject-net-proxy"}),
		ginkgo.Entry("multi-segment link must not match trailing-token-only peer",
			"net-proxy", []string{"myproject-other-proxy"}, nil),
		ginkgo.Entry("multi-segment link must not match substring service",
			"net-proxy", []string{"myproject-net"}, nil),
		ginkgo.Entry("multi-segment link must not match longer multi-segment sibling",
			"net-proxy", []string{"myproject-net-proxy-extra"}, nil),
		ginkgo.Entry("multi-segment link ambiguous across two projects",
			"net-proxy", []string{"stack-a-net-proxy", "stack-b-net-proxy"}, nil),
		ginkgo.Entry("multi-segment link prefers exact over suffix when both present",
			"net-proxy", []string{"net-proxy", "myproject-net-proxy"}, []string{"net-proxy"}),
		// Mixed multi-stack identifier pools
		ginkgo.Entry("unhyphenated db among multi-stack unrelated services",
			"db", []string{"frontend-web", "backend-api", "data-db"}, []string{"data-db"}),
		ginkgo.Entry("hyphenated link among multi-stack noise",
			"net-proxy",
			[]string{"frontend-web", "backend-api", "infra-net-proxy", "infra-other-proxy"},
			[]string{"infra-net-proxy"}),
		ginkgo.Entry("hyphenated link with same trailing token in another stack",
			"auth-gateway",
			[]string{"stack-a-auth-gateway", "stack-b-edge-gateway"},
			[]string{"stack-a-auth-gateway"}),
		// Empty / edge
		ginkgo.Entry("empty link",
			"", []string{"db"}, nil),
		ginkgo.Entry("empty identifiers",
			"db", []string{}, nil),
		ginkgo.Entry("no candidates at all",
			"missing", []string{"a", "b"}, nil),
	)
})

// identifierSet builds a bool set from identifier strings for resolveLinkToCanonicalKeys tests.
func identifierSet(ids ...string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}

	return set
}

var _ = ginkgo.Describe("buildLinkMatchIndexes", func() {
	ginkgo.It("should map unique bare names to canonical identifiers", func() {
		dep := mockLinkedContainer("net-proxy", "id-proxy", "myproject", "net-proxy", nil, true)
		app := mockLinkedContainer("web-app", "id-web", "myproject", "web", nil, true)

		containerMap := map[string]types.Container{
			"myproject-net-proxy": dep,
			"myproject-web":       app,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical["myproject-net-proxy"]).To(gomega.Equal("myproject-net-proxy"))
		gomega.Expect(aliasToCanonical["net-proxy"]).To(gomega.Equal("myproject-net-proxy"))
		gomega.Expect(aliasToCanonical["web-app"]).To(gomega.Equal("myproject-web"))
		gomega.Expect(idSet).To(gomega.HaveKey("myproject-net-proxy"))
		gomega.Expect(idSet).To(gomega.HaveKey("myproject-web"))
		gomega.Expect(idSet).To(gomega.HaveKey("net-proxy"))
		gomega.Expect(idSet).To(gomega.HaveKey("web-app"))
	})

	ginkgo.It("should omit bare name alias when it collides across containers", func() {
		c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		c1.EXPECT().Name().Return("shared").Maybe()

		c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		c2.EXPECT().Name().Return("shared").Maybe()

		containerMap := map[string]types.Container{
			"project1-service": c1,
			"project2-service": c2,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		_, hasBare := aliasToCanonical["shared"]
		gomega.Expect(hasBare).To(gomega.BeFalse())
		gomega.Expect(aliasToCanonical["project1-service"]).To(gomega.Equal("project1-service"))
		gomega.Expect(aliasToCanonical["project2-service"]).To(gomega.Equal("project2-service"))
	})

	ginkgo.It("should preserve canonical self-mapping when bare name equals another canonical key", func() {
		// Container A is keyed by "shared".
		// Container B's bare name is also "shared".
		// B must not overwrite or delete A's canonical self-mapping.
		canonicalOwner := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		canonicalOwner.EXPECT().Name().Return("shared").Maybe()

		bareClaimant := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		bareClaimant.EXPECT().Name().Return("shared").Maybe()

		containerMap := map[string]types.Container{
			"shared":          canonicalOwner,
			"project-service": bareClaimant,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical["shared"]).To(gomega.Equal("shared"))
		gomega.Expect(aliasToCanonical["project-service"]).To(gomega.Equal("project-service"))
		gomega.Expect(idSet).To(gomega.HaveKey("shared"))
		gomega.Expect(idSet).To(gomega.HaveKey("project-service"))
		gomega.Expect(idSet).To(gomega.HaveLen(2))
	})

	ginkgo.It("should map a Docker ID alias for concrete containers", func() {
		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical[providerID]).To(gomega.Equal("gluetun-vpn"))
		gomega.Expect(idSet).To(gomega.HaveKey(providerID))
	})

	ginkgo.It("should skip an empty Docker ID", func() {
		gluetun := concreteLinkedContainer("", "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical["gluetun-vpn"]).To(gomega.Equal("gluetun-vpn"))
		gomega.Expect(aliasToCanonical["gluetun"]).To(gomega.Equal("gluetun-vpn"))
		gomega.Expect(idSet).To(gomega.HaveLen(2))
	})

	ginkgo.It("should skip a Docker ID that equals the canonical identifier", func() {
		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/"+providerID, "", "", "")

		containerMap := map[string]types.Container{
			providerID: gluetun,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical[providerID]).To(gomega.Equal(providerID))
	})

	ginkgo.It("should not overwrite an existing canonical key with a Docker ID", func() {
		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		owner := concreteLinkedContainer("owner-id", "/owner", "owner", "svc", "")
		claimant := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			providerID:    owner,
			"gluetun-vpn": claimant,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical[providerID]).To(gomega.Equal(providerID))
	})

	ginkgo.It("should retain a Docker ID alias when another container uses that ID as its name", func() {
		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
		decoy := concreteLinkedContainer("decoy-id", "/"+providerID, "other", "svc", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
			"other-svc":   decoy,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(testLog(), containerMap)
		gomega.Expect(aliasToCanonical[providerID]).To(gomega.Equal("gluetun-vpn"))
	})
})

var _ = ginkgo.Describe("resolveLinkToCanonicalKeys", func() {
	ginkgo.It("should resolve bare multi-segment container name to project-service key", func() {
		idSet := identifierSet("myproject-net-proxy", "myproject-web", "net-proxy", "web-app")
		alias := map[string]string{
			"myproject-net-proxy": "myproject-net-proxy",
			"myproject-web":       "myproject-web",
			"net-proxy":           "myproject-net-proxy",
			"web-app":             "myproject-web",
		}

		keys := resolveLinkToCanonicalKeys("net-proxy", idSet, alias)
		gomega.Expect(keys).To(gomega.Equal([]string{"myproject-net-proxy"}))
	})

	ginkgo.It("should resolve project-service suffix match without bare-name alias", func() {
		idSet := identifierSet("myproject-net-proxy")
		alias := map[string]string{"myproject-net-proxy": "myproject-net-proxy"}

		keys := resolveLinkToCanonicalKeys("net-proxy", idSet, alias)
		gomega.Expect(keys).To(gomega.Equal([]string{"myproject-net-proxy"}))
	})

	ginkgo.It("should return nil for empty or unmatched links", func() {
		idSet := identifierSet("myproject-db")
		alias := map[string]string{"myproject-db": "myproject-db"}

		gomega.Expect(resolveLinkToCanonicalKeys("", idSet, alias)).To(gomega.BeNil())
		gomega.Expect(resolveLinkToCanonicalKeys("missing", idSet, alias)).To(gomega.BeNil())
	})

	ginkgo.It("should dedupe when bare alias and canonical both match", func() {
		idSet := identifierSet("myproject-db", "db")
		alias := map[string]string{
			"myproject-db": "myproject-db",
			"db":           "myproject-db",
		}

		keys := resolveLinkToCanonicalKeys("db", idSet, alias)
		gomega.Expect(keys).To(gomega.Equal([]string{"myproject-db"}))
	})
})

var _ = ginkgo.Describe("dependency link form permutations", func() {
	// Links() values from Watchtower depends-on, Compose depends_on, Docker links,
	// and network_mode must resolve against ResolveContainerIdentifier graph keys.
	ginkgo.DescribeTable(
		"sorts dependency before dependent for each link form",
		func(depName, depProject, depService, dependentName, dependentProject, dependentService string, links []string, useCompose bool) {
			dep := mockLinkedContainer(depName, "id-dep", depProject, depService, nil, useCompose)
			dependent := mockLinkedContainer(
				dependentName,
				"id-dependent",
				dependentProject,
				dependentService,
				links,
				useCompose,
			)

			containers := []types.Container{dependent, dep}
			err := DependencySorter{}.Sort(testLog(), containers, useCompose)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers[0].Name()).To(gomega.Equal(depName),
				"dependency must sort before dependent")
			gomega.Expect(containers[1].Name()).To(gomega.Equal(dependentName))
		},
		ginkgo.Entry(
			"Watchtower depends-on bare container_name against project-service key",
			"net-proxy", "myproject", "net-proxy",
			"web-app", "myproject", "web",
			[]string{"net-proxy"}, true,
		),
		ginkgo.Entry(
			"Watchtower depends-on bare name without Compose labels",
			"database", "", "",
			"web", "", "",
			[]string{"database"}, true,
		),
		ginkgo.Entry(
			"Watchtower depends-on with useComposeDependsOn false",
			"postgres", "stack", "postgres",
			"api", "stack", "api",
			[]string{"postgres"}, false,
		),
		ginkgo.Entry(
			"Compose depends_on project-qualified service link",
			"myproject-database", "myproject", "database",
			"myproject-web", "myproject", "web",
			[]string{"myproject-database"}, true,
		),
		ginkgo.Entry(
			"Compose depends_on bare service name against project-service key",
			"myproject-database", "myproject", "database",
			"myproject-web", "myproject", "web",
			[]string{"database"}, true,
		),
		ginkgo.Entry(
			"Compose depends_on multi-segment service under project prefix",
			"myproject-net-proxy", "myproject", "net-proxy",
			"myproject-web", "myproject", "web",
			[]string{"net-proxy"}, true,
		),
		ginkgo.Entry(
			"network_mode HostConfig bare container name",
			"vpn", "stack", "vpn",
			"client", "stack", "client",
			[]string{"vpn"}, true,
		),
		ginkgo.Entry(
			"network_mode with useComposeDependsOn false",
			"vpn", "stack", "vpn",
			"client", "stack", "client",
			[]string{"vpn"}, false,
		),
		ginkgo.Entry(
			"legacy Docker link bare container name",
			"db", "", "",
			"app", "", "",
			[]string{"db"}, true,
		),
		ginkgo.Entry(
			"cross-project Watchtower depends-on by container name",
			"app1-database", "app1", "database",
			"app2-web", "app2", "web",
			[]string{"app1-database"}, true,
		),
		ginkgo.Entry(
			"Compose service name without container_name override",
			"stack-cache", "stack", "cache",
			"stack-worker", "stack", "worker",
			[]string{"cache"}, true,
		),
		ginkgo.Entry(
			"normalized bare service name after Links processing",
			"redis", "cache", "redis",
			"app", "cache", "app",
			[]string{"redis"}, true,
		),
	)

	ginkgo.It("should order multiple dependents after a shared network provider", func() {
		vpn := mockLinkedContainer("vpn", "id-vpn", "stack", "vpn", nil, true)
		clientA := mockLinkedContainer("client-a", "id-a", "stack", "client-a", []string{"vpn"}, true)
		clientB := mockLinkedContainer("client-b", "id-b", "stack", "client-b", []string{"vpn"}, true)

		containers := []types.Container{clientA, clientB, vpn}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(containers[0].Name()).To(gomega.Equal("vpn"))
		names := []string{containers[1].Name(), containers[2].Name()}
		gomega.Expect(names).To(gomega.ConsistOf("client-a", "client-b"))
	})

	ginkgo.It("should order when dependent has multiple dependencies", func() {
		db := mockLinkedContainer("db", "id-db", "app", "db", nil, true)
		cache := mockLinkedContainer("cache", "id-cache", "app", "cache", nil, true)
		web := mockLinkedContainer("web", "id-web", "app", "web", []string{"db", "cache"}, true)

		containers := []types.Container{web, db, cache}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(containers[2].Name()).To(gomega.Equal("web"))

		ordered := []string{containers[0].Name(), containers[1].Name(), containers[2].Name()}
		assertOrderBefore(ordered, "db", "web")
		assertOrderBefore(ordered, "cache", "web")
	})

	ginkgo.It("should match Compose replica identifiers from a service link", func() {
		db1 := mockLinkedContainer("myproject-db-1", "id-db1", "myproject", "db", nil, true)
		db2 := mockLinkedContainer("myproject-db-2", "id-db2", "myproject", "db", nil, true)
		app := mockLinkedContainer("myproject-app", "id-app", "myproject", "app", []string{"myproject-db"}, true)

		containers := []types.Container{app, db1, db2}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(containers[2].Name()).To(gomega.Equal("myproject-app"))
		gomega.Expect([]string{containers[0].Name(), containers[1].Name()}).
			To(gomega.ConsistOf("myproject-db-1", "myproject-db-2"))
	})

	ginkgo.It("should not create edges for ambiguous same service name across projects", func() {
		db1 := mockLinkedContainer("project1-db", "id-1", "project1", "db", nil, true)
		db2 := mockLinkedContainer("project2-db", "id-2", "project2", "db", nil, true)
		app := mockLinkedContainer("project3-app", "id-3", "project3", "app", []string{"db"}, true)

		containers := []types.Container{app, db1, db2}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		names := []string{containers[0].Name(), containers[1].Name(), containers[2].Name()}
		gomega.Expect(names).To(gomega.ConsistOf("project1-db", "project2-db", "project3-app"))
	})

	ginkgo.It("should ignore missing dependency targets without failing sort", func() {
		app := mockLinkedContainer("app", "id-app", "myproject", "app", []string{"missing-dep"}, true)
		standalone := mockLinkedContainer("other", "id-other", "myproject", "other", nil, true)

		containers := []types.Container{app, standalone}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect([]string{containers[0].Name(), containers[1].Name()}).
			To(gomega.ConsistOf("app", "other"))
	})

	ginkgo.It("should keep Watchtower last when linked non-Watchtower containers sort first", func() {
		db := mockLinkedContainer("db", "id-db", "myproject", "db", nil, true)
		web := mockLinkedContainer("web", "id-web", "myproject", "web", []string{"db"}, true)

		wt := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		wt.EXPECT().Name().Return("watchtower").Maybe()
		wt.EXPECT().ID().Return(types.ContainerID("id-wt")).Maybe()
		wt.EXPECT().IsWatchtower().Return(true).Maybe()
		wt.EXPECT().Links(true).Return(nil).Maybe()
		wt.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Name:   "/watchtower",
			Config: &dockerContainer.Config{Labels: map[string]string{}},
		}).Maybe()

		containers := []types.Container{wt, web, db}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(containers[0].Name()).To(gomega.Equal("db"))
		gomega.Expect(containers[1].Name()).To(gomega.Equal("web"))
		gomega.Expect(containers[2].Name()).To(gomega.Equal("watchtower"))
	})

	ginkgo.It("should order independent stacks without cross-linking same service names", func() {
		// Two Compose projects each have db→web.
		// Bare link "db" is ambiguous and must not couple stacks.
		// Each web only orders relative to its own db when links use project-qualified or unique names.
		dbA := mockLinkedContainer("stack-a-db", "id-da", "stack-a", "db", nil, true)
		webA := mockLinkedContainer(
			"stack-a-web", "id-wa", "stack-a", "web", []string{"stack-a-db"}, true,
		)
		dbB := mockLinkedContainer("stack-b-db", "id-db", "stack-b", "db", nil, true)
		webB := mockLinkedContainer(
			"stack-b-web", "id-wb", "stack-b", "web", []string{"stack-b-db"}, true,
		)

		containers := []types.Container{webA, webB, dbA, dbB}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(ordered, "stack-a-db", "stack-a-web")
		assertOrderBefore(ordered, "stack-b-db", "stack-b-web")
	})

	ginkgo.It("should order cross-stack Watchtower depends-on by foreign container_name", func() {
		// Provider stack owns the named network peer.
		// Consumer stack depends on it via Watchtower depends-on / network_mode container name.
		provider := mockLinkedContainer(
			"shared-net-proxy", "id-prov", "infra", "net-proxy", nil, true,
		)
		consumer := mockLinkedContainer(
			"app-web", "id-cons", "app", "web", []string{"shared-net-proxy"}, true,
		)
		unrelated := mockLinkedContainer(
			"other-db", "id-other", "other", "db", nil, true,
		)

		containers := []types.Container{consumer, unrelated, provider}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(ordered, "shared-net-proxy", "app-web")
		gomega.Expect(ordered).To(gomega.ConsistOf("shared-net-proxy", "app-web", "other-db"))
	})

	ginkgo.It("should not couple stacks when multi-segment services share a trailing token", func() {
		// stack-a has service net-proxy
		// stack-b has service other-proxy.
		// A dependent linking to net-proxy must only wait on stack-a's peer, not other-proxy.
		proxyA := mockLinkedContainer(
			"stack-a-net-proxy", "id-pa", "stack-a", "net-proxy", nil, true,
		)
		proxyB := mockLinkedContainer(
			"stack-b-other-proxy", "id-pb", "stack-b", "other-proxy", nil, true,
		)
		client := mockLinkedContainer(
			"stack-a-client", "id-cl", "stack-a", "client", []string{"net-proxy"}, true,
		)

		containers := []types.Container{client, proxyB, proxyA}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(ordered, "stack-a-net-proxy", "stack-a-client")
		// other-proxy must not become a dependency of client (no edge).
		// If wrongly linked, client would have indegree 2 and sort after both proxies with a fixed relative order.
		// Assert client is not forced after proxyB only when proxyA is already before client.
		gomega.Expect(indexOf(ordered, "stack-a-client")).
			To(gomega.BeNumerically(">", indexOf(ordered, "stack-a-net-proxy")))
	})

	ginkgo.It("should order multi-segment container_name dependency among multi-stack peers", func() {
		// Explicit container_name equals multi-segment service and Compose keys differ.
		provider := mockLinkedContainer(
			"net-proxy", "id-np", "media", "net-proxy", nil, true,
		)
		dependent := mockLinkedContainer(
			"web-app", "id-wa", "media", "web", []string{"net-proxy"}, true,
		)
		foreign := mockLinkedContainer(
			"net-proxy-other", "id-fo", "other", "net-proxy-other", nil, true,
		)
		// Same trailing token in another stack must not steal the edge.
		decoy := mockLinkedContainer(
			"edge-proxy", "id-de", "edge", "edge-proxy", nil, true,
		)

		containers := []types.Container{dependent, foreign, decoy, provider}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(ordered, "net-proxy", "web-app")
	})

	ginkgo.It("should order chain across three stacks with mixed link forms", func() {
		// data.db ← (compose service) app.worker ← (watchtower container_name) edge.client
		db := mockLinkedContainer("data-db", "id-db", "data", "db", nil, true)
		worker := mockLinkedContainer(
			"app-worker", "id-wk", "app", "worker", []string{"db"}, true,
		)
		// Watchtower depends-on uses worker's container_name.
		client := mockLinkedContainer(
			"edge-client", "id-cl", "edge", "client", []string{"app-worker"}, true,
		)

		containers := []types.Container{client, worker, db}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(ordered, "data-db", "app-worker")
		assertOrderBefore(ordered, "app-worker", "edge-client")
		assertOrderBefore(ordered, "data-db", "edge-client")
	})

	ginkgo.It("should not create edge for hyphenated link against trailing-token-only peer", func() {
		// Regression: ExtractServiceName("net-proxy") and ExtractServiceName("other-proxy")
		// both yield "proxy".
		// Matching must not treat them as the same dependency.
		decoy := mockLinkedContainer(
			"other-proxy", "id-decoy", "other", "other-proxy", nil, true,
		)
		dependent := mockLinkedContainer(
			"web", "id-web", "app", "web", []string{"net-proxy"}, true,
		)

		containers := []types.Container{dependent, decoy}
		err := DependencySorter{}.Sort(testLog(), containers, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		// No dependency edge: both indegree 0. Order is reverse-alpha among keys.
		gomega.Expect([]string{containers[0].Name(), containers[1].Name()}).
			To(gomega.ConsistOf("web", "other-proxy"))
	})
})
