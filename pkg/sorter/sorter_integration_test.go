package sorter_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

var _ = ginkgo.Describe("Container Sorting", func() {
	ginkgo.Describe("SortByDependencies", func() {
		ginkgo.When("sorting by dependencies", func() {
			ginkgo.It("sorts containers with no links first", func() {
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(2))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("c2")) // No links
				gomega.Expect(containers[1].Name()).To(gomega.Equal("c1")) // Depends on c2
			})

			ginkgo.It("handles multiple dependencies", func() {
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2", "c3"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string{"c3"})
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c3.EXPECT().Name().Return("c3")
				c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
				c3.EXPECT().Links(true).Return([]string(nil))
				c3.EXPECT().IsWatchtower().Return(false)
				c3.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2, c3}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(3))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("c3")) // No links
				gomega.Expect(containers[1].Name()).To(gomega.Equal("c2")) // Links to c3
				gomega.Expect(containers[2].Name()).To(gomega.Equal("c1")) // Links to c2, c3
			})

			ginkgo.It("detects circular references", func() {
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
				c2.EXPECT().Links(true).Return([]string{"c1"})
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("circular reference detected"))
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("c1 -> c2 -> c1"))
			})

			ginkgo.It("handles missing dependencies gracefully", func() {
				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c3 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c3.EXPECT().Name().Return("c3")
				c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
				c3.EXPECT().Links(true).Return([]string(nil))
				c3.EXPECT().IsWatchtower().Return(false)
				c3.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c3}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(2))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("c3")) // No links
				gomega.Expect(containers[1].Name()).To(gomega.Equal("c1")) // Links to missing c2
			})

			ginkgo.It("handles empty list", func() {
				containers := []types.Container{}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.BeEmpty())
			})

			ginkgo.It("places Watchtower containers last", func() {
				watchtower := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				watchtower.EXPECT().Name().Return("watchtower")
				watchtower.EXPECT().IsWatchtower().Return(true)

				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{watchtower, c1, c2}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(3))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("c2")) // No links
				gomega.Expect(containers[1].Name()).To(gomega.Equal("c1")) // Depends on c2
				gomega.Expect(containers[2].Name()).
					To(gomega.Equal("watchtower"))
				// Watchtower last
			})

			ginkgo.It("places multiple Watchtower containers last", func() {
				watchtower1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				watchtower1.EXPECT().Name().Return("watchtower1")
				watchtower1.EXPECT().IsWatchtower().Return(true)

				c1 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				watchtower2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				watchtower2.EXPECT().Name().Return("watchtower2")
				watchtower2.EXPECT().IsWatchtower().Return(true)

				c2 := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{watchtower1, c1, watchtower2, c2}
				err := sorter.SortByDependencies(newNopLogger(), containers, true)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(4))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("c2")) // No links
				gomega.Expect(containers[1].Name()).To(gomega.Equal("c1")) // Depends on c2
				// Watchtower containers at the end (order may vary)
				watchtowerNames := []string{containers[2].Name(), containers[3].Name()}
				gomega.Expect(watchtowerNames).To(gomega.ContainElement("watchtower1"))
				gomega.Expect(watchtowerNames).To(gomega.ContainElement("watchtower2"))
			})

			ginkgo.It("handles chained dependencies with slash-prefixed links", func() {
				newMockContainer := func(t ginkgo.GinkgoTInterface, name, id string, links []string, labels map[string]string) types.Container {
					c := mockTypes.NewMockContainer(t)
					c.EXPECT().Name().Return(name)
					c.EXPECT().ID().Return(types.ContainerID(id))
					c.EXPECT().Links(true).Return(links)
					c.EXPECT().IsWatchtower().Return(false)
					c.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
						Config: &dockerContainer.Config{
							Labels: labels,
						},
					})

					return c
				}

				makeSimpleChainContainers := func(t ginkgo.GinkgoTInterface) []types.Container {
					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/c"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b"}, map[string]string{})

					return []types.Container{c, b, a}
				}

				makeMultipleDepsContainers := func(t ginkgo.GinkgoTInterface) []types.Container {
					d := newMockContainer(t, "d", "id-d", nil, map[string]string{})
					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/d"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b", "/c"}, map[string]string{})

					return []types.Container{d, c, b, a}
				}

				makeDiamondContainers := func(t ginkgo.GinkgoTInterface) []types.Container {
					d := newMockContainer(t, "d", "id-d", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/d"}, map[string]string{})
					c := newMockContainer(t, "c", "id-c", []string{"/d"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b", "/c"}, map[string]string{})

					return []types.Container{d, b, c, a}
				}

				makeNoDepsContainers := func(t ginkgo.GinkgoTInterface) []types.Container {
					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", nil, map[string]string{})
					a := newMockContainer(t, "a", "id-a", nil, map[string]string{})

					return []types.Container{c, b, a}
				}

				testCases := []struct {
					name          string
					containers    func() []types.Container
					expectedOrder []string
				}{
					{
						name: "simple chain with slashes",
						containers: func() []types.Container {
							return makeSimpleChainContainers(ginkgo.GinkgoT())
						},
						expectedOrder: []string{"c", "b", "a"},
					},
					{
						name: "multiple dependencies with slashes",
						containers: func() []types.Container {
							return makeMultipleDepsContainers(ginkgo.GinkgoT())
						},
						expectedOrder: []string{"d", "c", "b", "a"},
					},
					{
						name: "diamond dependency graph",
						containers: func() []types.Container {
							return makeDiamondContainers(ginkgo.GinkgoT())
						},
						expectedOrder: []string{
							"d",
							"b",
							"c",
							"a",
						}, // D first, then B and C (order may vary), then A
					},
					{
						name: "no dependencies",
						containers: func() []types.Container {
							return makeNoDepsContainers(ginkgo.GinkgoT())
						},
						expectedOrder: []string{"c", "b", "a"},
					},
				}
				for _, tc := range testCases {
					ginkgo.By(tc.name, func() {
						containers := tc.containers()
						err := sorter.SortByDependencies(newNopLogger(), containers, true)
						gomega.Expect(err).ToNot(gomega.HaveOccurred())
						gomega.Expect(containers).To(gomega.HaveLen(len(tc.expectedOrder)))
						// For diamond, check that D is first, A is last, and B/C are in middle
						if tc.name == "diamond dependency graph" {
							gomega.Expect(containers[0].Name()).To(gomega.Equal("d"))
							gomega.Expect(containers[3].Name()).To(gomega.Equal("a"))
							middleNames := []string{containers[1].Name(), containers[2].Name()}
							gomega.Expect(middleNames).To(gomega.ContainElement("b"))
							gomega.Expect(middleNames).To(gomega.ContainElement("c"))
						} else {
							for i, name := range tc.expectedOrder {
								gomega.Expect(containers[i].Name()).To(gomega.Equal(name))
							}
						}
					})
				}
			})
		})
	})
})

// newNopLogger returns a logger that discards every event, for Ginkgo specs,
// benchmarks, and fuzz targets.
//
// Returns:
//   - *zerolog.Logger: Logger that discards all events.
func newNopLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}
