package actions

import (
	"context"
	"errors"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockContainerTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

var _ = ginkgo.Describe("ValidateRollingRestartDependencies", func() {
	var (
		mockClient *mockContainer.MockClient
		filter     types.Filter
	)

	ginkgo.BeforeEach(func() {
		mockClient = mockContainer.NewMockClient(ginkgo.GinkgoT())
		// Create a simple filter that accepts all containers
		filter = func(types.FilterableContainer) bool { return true }
	})

	ginkgo.When("ListContainers fails", func() {
		ginkgo.It("should return wrapped error", func() {
			expectedErr := errors.New("list containers failed")
			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(nil, expectedErr)

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).
				To(gomega.MatchError(gomega.ContainSubstring("list containers failed")))
		})
	})

	ginkgo.When("no containers are found", func() {
		ginkgo.It("should return nil", func() {
			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil)

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.When("containers have no links", func() {
		ginkgo.It("should return nil", func() {
			mockContainer1 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			mockContainer2 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())

			mockContainer1.EXPECT().Links(true).Return([]string{})
			mockContainer2.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				mockContainer1,
				mockContainer2,
			}, nil)

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.When("a container has links", func() {
		ginkgo.It("should return error with container name and links", func() {
			mockContainer1 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			mockContainer2 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())

			mockContainer1.EXPECT().Links(true).Return([]string{})
			mockContainer2.EXPECT().Links(true).Return([]string{"db", "redis"})
			mockContainer2.EXPECT().Name().Return("web-container")

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				mockContainer1,
				mockContainer2,
			}, nil)

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring(`"web-container" depends on [db redis]`))
		})
	})
})

var _ = ginkgo.Describe("Scope Validation Error Cases", func() {
	ginkgo.When("scope labels are malformed", func() {
		ginkgo.It("should handle empty scope labels gracefully", func() {
			// Test that empty scope labels are treated as unscoped
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Container with empty scope label
			container := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			container.EXPECT().
				Scope().
				Return("", false).
				Maybe()
				// No scope label present - called multiple times for logging/validation
			container.EXPECT().Links(true).Return([]string{}) // No links for validation

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{container}, nil)

			// Create unscoped filter (empty scope should default to unscoped)
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()
				if !hasScope || scope == "" {
					return true // Unscoped containers pass
				}

				return false
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle containers with invalid scope label formats", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Container with malformed scope label (e.g., contains special chars)
			container1 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			container2 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())

			container1.EXPECT().Scope().Return("valid-scope", true).Maybe()
			container1.EXPECT().Links(true).Return([]string{})
			container2.EXPECT().Scope().Return("invalid$scope@123", true).Maybe() // Malformed scope
			container2.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				container1,
				container2,
			}, nil)

			// Filter that matches malformed scope
			filter := func(c types.FilterableContainer) bool {
				scope, _ := c.Scope()

				return scope == "invalid$scope@123"
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle scope labels with whitespace and normalization", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Container with scope label containing whitespace
			container := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			container.EXPECT().
				Scope().
				Return("  spaced scope  ", true).Maybe()
			// Scope with leading/trailing spaces
			container.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{container}, nil)

			// Filter that expects trimmed scope
			filter := func(c types.FilterableContainer) bool {
				scope, _ := c.Scope()

				return scope == "  spaced scope  " // Exact match including spaces
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.When("scope validation encounters errors", func() {
		ginkgo.It("should handle scope label retrieval errors gracefully", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Container that might cause issues during scope checking
			container := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			container.EXPECT().Scope().Return("", false).Maybe() // Simulate missing scope label
			container.EXPECT().Links(true).Return([]string{})    // No validation issues

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{container}, nil)

			// Filter that handles missing scopes
			filter := func(c types.FilterableContainer) bool {
				_, hasScope := c.Scope()

				return !hasScope // Include containers without scope labels
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should validate scope consistency across operations", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Multiple containers that should have consistent scope behavior
			container1 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			container2 := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())

			container1.EXPECT().Scope().Return("consistent-scope", true).Maybe()
			container1.EXPECT().Links(true).Return([]string{})
			container2.EXPECT().Scope().Return("consistent-scope", true).Maybe()
			container2.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				container1,
				container2,
			}, nil)

			// Filter requiring scope consistency
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()

				return hasScope && scope == "consistent-scope"
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle scope label conflicts during validation", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Container with conflicting scope information
			container := mockContainerTypes.NewMockContainer(ginkgo.GinkgoT())
			// Simulate potential inconsistency - scope exists but is empty
			container.EXPECT().Scope().Return("", true).Maybe() // Has scope label but it's empty
			container.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{container}, nil)

			// Filter that detects empty scope labels
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()

				return hasScope && scope == "" // Allow empty scoped containers
			}

			err := ValidateRollingRestartDependencies(newNopLogger(), context.Background(), mockClient, filter, true)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
