package registry_test

import (
	"github.com/containrrr/watchtower/internal/actions/mocks"
	unit "github.com/containrrr/watchtower/pkg/registry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"
)

var _ = Describe("Registry", func() {
	Describe("WarnOnAPIConsumption", func() {
		When("Given a container with an image from ghcr.io", func() {
			It("should want to warn", func() {
				Expect(testContainerWithImage("ghcr.io/containrrr/watchtower")).To(BeTrue())
			})
		})
		When("Given a container with an image implicitly from dockerhub", func() {
			It("should want to warn", func() {
				Expect(testContainerWithImage("docker:latest")).To(BeTrue())
			})
		})
		When("Given a container with an image explicitly from dockerhub", func() {
			It("should want to warn", func() {
				Expect(testContainerWithImage("registry-1.docker.io/docker:latest")).To(BeTrue())
				Expect(testContainerWithImage("index.docker.io/docker:latest")).To(BeTrue())
				Expect(testContainerWithImage("docker.io/docker:latest")).To(BeTrue())
			})

		})
		When("Given a container with an image from some other registry", func() {
			It("should not want to warn", func() {
				Expect(testContainerWithImage("docker.fsf.org/docker:latest")).To(BeFalse())
				Expect(testContainerWithImage("altavista.com/docker:latest")).To(BeFalse())
				Expect(testContainerWithImage("gitlab.com/docker:latest")).To(BeFalse())
			})
		})
	})
})

func testContainerWithImage(imageName string) bool {
	container := mocks.CreateMockContainer("", "", imageName, time.Now())
	return unit.WarnOnAPIConsumption(container)
}
