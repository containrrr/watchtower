package actions

import (
	"context"
	"errors"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("image disk space gate", func() {
	ginkgo.It("skips the check when both thresholds are unset", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

		_, _, err := update(
			testLogger(),
			context.Background(),
			client,
			defaultTestUpdateParams(filters.NoFilter),
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.GetImageDiskUsageCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.BeNumerically(">", 0))
	})

	ginkgo.It("continues when usage is below the warning threshold", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 1_000, TotalCount: 1},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		_, _, err := update(testLogger(), context.Background(), client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.GetImageDiskUsageCount.Load()).To(gomega.Equal(int32(1)))
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.BeNumerically(">", 0))
	})

	ginkgo.It("warns and continues when usage reaches the warning threshold", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 8_000, TotalCount: 2},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		_, _, err := update(testLogger(), context.Background(), client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.BeNumerically(">", 0))
	})

	ginkgo.It("allows warn-only mode without blocking", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 50_000, TotalCount: 3},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceWarn = 1_000

		_, _, err := update(testLogger(), context.Background(), client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.BeNumerically(">", 0))
	})

	ginkgo.It("blocks the session when usage reaches the maximum", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{
				TotalSize:   10_000,
				Reclaimable: 2_000,
				TotalCount:  4,
			},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		_, _, err := update(testLogger(), context.Background(), client, params)
		gomega.Expect(err).To(gomega.MatchError(errImageDiskSpaceExceeded))
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.Equal(int32(0)))
	})

	ginkgo.It("fails closed when image disk usage cannot be queried", func() {
		client := mockActions.CreateMockClient(&mockActions.TestData{
			GetImageDiskUsageError: errors.New("df unavailable"),
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000

		_, _, err := update(testLogger(), context.Background(), client, params)
		gomega.Expect(err).To(gomega.MatchError(errImageDiskUsageFailed))
		gomega.Expect(client.TestData.ListContainersCount.Load()).To(gomega.Equal(int32(0)))
	})
})
