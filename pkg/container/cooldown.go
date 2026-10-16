package container

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/registry"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// CooldownError represents a cooldown deferral with human-readable details
// (age, delay, remaining). It wraps
// ErrImageCooldown (or a fetch failure) so callers can use errors.Is/As
// while still extracting the fields needed for progress reports and
// rich notifications.
type CooldownError struct {
	Age        string
	Delay      string
	Remaining  string
	EligibleAt time.Time
	err        error
}

func (e *CooldownError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}

	return ErrImageCooldown.Error()
}

func (e *CooldownError) Unwrap() error { return e.err }

func (e *CooldownError) Is(target error) bool {
	if target == ErrImageCooldown {
		return e.err == nil || errors.Is(e.err, ErrImageCooldown)
	}

	return errors.Is(e.err, target)
}

// isOutsideCooldown reports whether the container's image is outside its
// cooldown window (safe to pull and update).
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - sourceContainer: Container whose image age to evaluate.
//   - params: Update parameters (global cooldown + LabelPrecedence).
//
// Returns:
//   - bool: true if safe to pull/update, false if deferred.
//   - error: non-nil when deferred (carries CooldownError for details).
func (c imageClient) isOutsideCooldown(
	ctx context.Context,
	sourceContainer types.Container,
	params types.UpdateParams,
) (bool, error) {
	delay, skip := shouldCheckCooldown(sourceContainer, params)
	if skip {
		return true, nil
	}

	clogVal := c.log.With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal

	creationTime, cdErr := fetchImageCreationTime(
		ctx,
		sourceContainer,
		delay,
		c.registrySettings,
		clog,
	)
	if cdErr != nil {
		return false, cdErr
	}

	return evalImageAge(creationTime, delay, clog)
}

// shouldCheckCooldown returns the effective cooldown delay and whether the
// check can be skipped (delay <= 0, no-pull, or monitor-only).
//
// Parameters:
//   - sourceContainer: Container to evaluate.
//   - params: Update parameters (global cooldown + LabelPrecedence).
//
// Returns:
//   - time.Duration: The effective cooldown delay (0 if skipped).
//   - bool: True if the cooldown check should be skipped entirely.
func shouldCheckCooldown(sourceContainer types.Container, params types.UpdateParams) (time.Duration, bool) {
	delay := sourceContainer.CooldownDelay(params)
	if delay <= 0 || sourceContainer.IsNoPull(params) || sourceContainer.IsMonitorOnly(params) {
		return 0, true
	}

	return delay, false
}

// fetchImageCreationTime resolves pull options and retrieves the image
// creation time from the registry. It returns a CooldownError on any failure.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - sourceContainer: Container whose image creation time to fetch.
//   - delay: Cooldown delay for error formatting.
//   - settings: Registry request settings.
//   - clog: Logger entry with container/image fields.
//
// Returns:
//   - time.Time: The image creation time.
//   - error: Non-nil CooldownError on failure.
func fetchImageCreationTime(
	ctx context.Context,
	sourceContainer types.Container,
	delay time.Duration,
	settings registryConfig.Registry,
	clog *zerolog.Logger,
) (time.Time, error) {
	pullOpts, err := registry.GetPullOptions(clog, sourceContainer.ImageName())
	if err != nil {
		clog.Info().
			Err(err).
			Msg("Failed to get pull options for cooldown check - update check unavailable")

		return time.Time{}, &CooldownError{
			Delay: util.FormatDuration(delay),
			err: fmt.Errorf(
				"could not get pull options for %s (cooldown: %s): %w",
				sourceContainer.ImageName(),
				util.FormatDuration(delay),
				err,
			),
		}
	}

	creationTime, err := registry.FetchImageCreationTime(clog,
		ctx,
		sourceContainer,
		pullOpts.RegistryAuth,
		settings,
	)
	if err != nil {
		clog.Info().
			Err(err).
			Msg("Image creation time unavailable - update check unavailable")

		return time.Time{}, &CooldownError{
			Delay: util.FormatDuration(delay),
			err: fmt.Errorf(
				"could not determine image age (cooldown: %s): %w",
				util.FormatDuration(delay),
				err,
			),
		}
	}

	return creationTime, nil
}

// evalImageAge compares image age against the cooldown delay, logging the
// result and returning (true, nil) when safe to pull or (false, CooldownError)
// when deferred.
//
// A creation time in the future gives no age, so the image counts as new and
// becomes eligible a cooldown delay after its creation time.
//
// Parameters:
//   - creationTime: The image creation timestamp.
//   - delay: The cooldown delay to compare against.
//   - clog: Logger entry with container/image fields.
//
// Returns:
//   - bool: True if the image age exceeds the cooldown window.
//   - error: Non-nil CooldownError when the image is within the cooldown window.
func evalImageAge(creationTime time.Time, delay time.Duration, clog *zerolog.Logger) (bool, error) {
	imageAge := time.Since(creationTime)

	if imageAge < 0 {
		logClockSkew(imageAge, delay, clog)
	}

	if imageAge <= delay {
		remaining := delay - imageAge
		ageStr := util.FormatDuration(imageAge)
		cooldownStr := util.FormatDuration(delay)
		remainingStr := util.FormatDuration(remaining)
		eligibleAt := time.Now().Add(remaining)

		clog.Info().
			Str("image_age", ageStr).
			Str("cooldown", cooldownStr).
			Str("eligible_in", remainingStr).
			Str("eligible_at", eligibleAt.Format(time.RFC3339)).
			Msg("Image is within cooldown period - not eligible for update")

		return false, buildCooldownError(imageAge, delay, eligibleAt)
	}

	logCooldownExceeded(imageAge, delay, clog)

	return true, nil
}

// buildCooldownError constructs a CooldownError with age, delay, and
// remaining fields populated from the image age and cooldown delay.
//
// Parameters:
//   - imageAge: Elapsed time since the image was created.
//   - delay: The configured cooldown delay.
//   - eligibleAt: Precomputed time when the container becomes eligible.
//
// Returns:
//   - *CooldownError: Populated with formatted age, delay, and remaining strings.
func buildCooldownError(imageAge, delay time.Duration, eligibleAt time.Time) *CooldownError {
	remaining := delay - imageAge

	return &CooldownError{
		Age:        util.FormatDuration(imageAge),
		Delay:      util.FormatDuration(delay),
		Remaining:  util.FormatDuration(remaining),
		EligibleAt: eligibleAt,
		err:        ErrImageCooldown,
	}
}

// logClockSkew logs a warning when the image creation time is in the future,
// indicating possible clock skew between the host and registry.
func logClockSkew(imageAge, delay time.Duration, clog *zerolog.Logger) {
	clog.Warn().
		Str("image_age", util.FormatDuration(imageAge)).
		Str("cooldown", util.FormatDuration(delay)).
		Msg("Image creation time is in the future (possible clock skew) - treating the image as new")
}

// CheckLocalImageCooldown applies cooldown-delay using the local image Created time.
//
// Unlike the registry path, this does not contact a registry. Missing image
// info skips the check so a session is not failed.
//
// Parameters:
//   - c: Container whose local image age is evaluated.
//   - params: Update parameters (global cooldown and label override).
//
// Returns:
//   - error: Non-nil CooldownError when the image is inside the cooldown window.
func CheckLocalImageCooldown(c types.Container, params types.UpdateParams) error {
	delay, skip := shouldCheckCooldown(c, params)
	if skip {
		return nil
	}

	created, ok := localImageCreated(c)
	if !ok {
		return nil
	}

	clog := nopLog()
	if concrete, isConcrete := c.(*Container); isConcrete {
		clog = concrete.logger()
	}

	_, err := evalImageAge(created, delay, clog)

	return err
}

func localImageCreated(c types.Container) (time.Time, bool) {
	if c == nil || !c.HasImageInfo() {
		return time.Time{}, false
	}

	info := c.ImageInfo()
	if info == nil || info.Created == "" {
		return time.Time{}, false
	}

	created, err := time.Parse(time.RFC3339Nano, info.Created)
	if err != nil {
		created, err = time.Parse(time.RFC3339, info.Created)
		if err != nil {
			return time.Time{}, false
		}
	}

	return created, true
}

// logCooldownExceeded logs an info message when the image age exceeds the
// cooldown window and the pull can proceed.
func logCooldownExceeded(imageAge, delay time.Duration, clog *zerolog.Logger) {
	clog.Info().
		Str("image_age", util.FormatDuration(imageAge)).
		Str("cooldown", util.FormatDuration(delay)).
		Msg("Image age exceeds cooldown - eligible for update")
}
