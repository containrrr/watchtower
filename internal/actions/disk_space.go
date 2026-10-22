package actions

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// msgImageDiskSpaceExceeded is logged and sampled when image usage reaches the maximum.
	msgImageDiskSpaceExceeded = "Docker image usage exceeds configured maximum"

	// msgImageDiskSpaceWarn is logged and sampled when image usage reaches the warning threshold.
	msgImageDiskSpaceWarn = "Docker image usage exceeds configured warning threshold"

	// msgImageDiskSpaceOK is logged when image usage is below all configured thresholds.
	msgImageDiskSpaceOK = "Docker image usage within configured budget"
)

// checkImageDiskSpace enforces the optional Docker image-usage budget.
//
// When both thresholds are unset the check is skipped. Otherwise it queries
// image usage once, updates Prometheus gauges, warns when usage reaches the
// warning threshold, and returns errImageDiskSpaceExceeded when usage reaches
// the maximum. A daemon query failure is fail-closed.
//
// Parameters:
//   - log: Session logger.
//   - ctx: Context for cancellation and timeout control.
//   - client: Docker client used to query image usage.
//   - config: Update policy containing disk-space thresholds in bytes.
//
// Returns:
//   - error: Non-nil when the query fails or usage reaches the maximum.
func checkImageDiskSpace(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	config types.UpdateParams,
) error {
	if config.DiskSpaceMax == 0 && config.DiskSpaceWarn == 0 {
		return nil
	}

	usage, err := client.GetImageDiskUsage(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query Docker image disk usage")

		return fmt.Errorf("%w: %w", errImageDiskUsageFailed, err)
	}

	metrics.RecordImageDiskUsage(
		usage.TotalSize,
		usage.Reclaimable,
		config.DiskSpaceMax,
		config.DiskSpaceWarn,
	)

	clogVal := log.With().
		Int64("usage", usage.TotalSize).
		Int64("max", config.DiskSpaceMax).
		Int64("warn", config.DiskSpaceWarn).
		Int64("reclaimable", usage.Reclaimable).
		Int64("image_count", usage.TotalCount).
		Logger()
	clog := &clogVal

	if config.DiskSpaceMax > 0 && usage.TotalSize >= config.DiskSpaceMax {
		clog.Error().Msg(msgImageDiskSpaceExceeded)

		return fmt.Errorf(
			"%w: usage %d bytes, max %d bytes",
			errImageDiskSpaceExceeded,
			usage.TotalSize,
			config.DiskSpaceMax,
		)
	}

	if config.DiskSpaceWarn > 0 && usage.TotalSize >= config.DiskSpaceWarn {
		clog.Warn().Msg(msgImageDiskSpaceWarn)

		return nil
	}

	clog.Debug().Msg(msgImageDiskSpaceOK)

	return nil
}
