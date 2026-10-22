package check

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// ContainerCheck holds the update availability result for a single container.
//
// This endpoint never pulls an image, so the latest* fields are always empty.
type ContainerCheck struct {
	Name                 string    `json:"name"`
	Image                string    `json:"image"`
	ImageID              string    `json:"image_id"`
	Digest               string    `json:"digest"`
	UpdateAvailable      bool      `json:"update_available"`
	LatestImageID        string    `json:"latest_image_id"`
	GitCommit            string    `json:"git_commit,omitempty"`
	LatestDigest         string    `json:"latest_digest"`
	Error                string    `json:"error,omitempty"`
	Timestamp            time.Time `json:"timestamp"`
	UpdateSource         string    `json:"update_source,omitempty"`
	GitRepo              string    `json:"git_repo,omitempty"`
	GitRef               string    `json:"git_ref,omitempty"`
	Changelog            string    `json:"changelog,omitempty"`
	OCISource            string    `json:"oci_source,omitempty"`
	ImageURL             string    `json:"image_url,omitempty"`
	Documentation        string    `json:"documentation,omitempty"`
	CurrentImageVersion  string    `json:"current_image_version,omitempty"`
	CurrentImageRevision string    `json:"current_revision,omitempty"`
	LatestImageVersion   string    `json:"latest_image_version,omitempty"`
	LatestImageRevision  string    `json:"latest_revision,omitempty"`
}

// CheckResponse is the response returned by POST /v1/check.
//
// Parameters/returns are documented by the handler's Swagger annotations.
type CheckResponse struct {
	Containers []ContainerCheck `json:"containers"`
	Count      int              `json:"count"`
	Timestamp  string           `json:"timestamp"`
	APIVersion string           `json:"api_version"`
}

// CheckFunc performs the update availability check for all watched containers.
type CheckFunc func(ctx context.Context, images, names []string) ([]ContainerCheck, error)

// extractFilterParams parses comma-separated query parameters into a slice.
// Supports both repeated params (?name=a&name=b) and comma-separated values (?name=a,b).
func extractFilterParams(c fiber.Ctx, key string) []string {
	var results []string

	queryArgs := c.Request().URI().QueryArgs()
	values := queryArgs.PeekMulti(key)

	for _, v := range values {
		parts := strings.SplitSeq(string(v), ",")
		for p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				results = append(results, trimmed)
			}
		}
	}

	return results
}

// CheckForUpdates checks all watched containers for available image updates.
//
// Associated Git-watched containers use the same watch split as scheduled
// updates: CheckContainer compares the hosted Git ref and does not clone or
// build. Other containers query the registry for the latest digest (HEAD with
// GET fallback) without pulling image layers.
// If no-pull is configured, then only the local cache is used for registry checks.
// Configured image cooldown checks are not applied.
// The provided filter determines which containers are included.
// nil or a pass-through filter includes all.
//
// Parameters:
//   - ctx: Context for the Docker API call.
//   - client: Docker client.
//   - filter: Combined container filter (general + image + name constraints).
//   - params: Update parameters (NoPull, LabelPrecedence).
//
// Returns:
//   - []ContainerCheck: Update availability for each matching container.
//   - error: Non-nil if listing containers fails.
func CheckForUpdates(log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	filter types.Filter,
	params types.UpdateParams,
	gitClient *git.Client,
) ([]ContainerCheck, error) {
	containers, err := client.ListContainers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	results := make([]ContainerCheck, 0, len(containers))
	now := time.Now().UTC()

	for _, c := range containers {
		if filter != nil && !filter(c) {
			continue
		}

		result := newContainerCheck(c, now)

		applyReportMetadata(c, params, &result)
		applyLocalDigest(c, &result)

		if isGitWatched(log, c, params, gitClient) {
			checkGitSource(ctx, log, c, params, gitClient, &result)
		} else {
			checkRegistrySource(ctx, log, client, c, params, &result)
		}

		results = append(results, result)
	}

	return results, nil
}

// newContainerCheck builds the identity fields every result carries.
//
// Parameters:
//   - c: Container being checked.
//   - now: Timestamp shared by every result in one run.
//
// Returns:
//   - ContainerCheck: Populated identity fields, source and availability left unset.
func newContainerCheck(c types.Container, now time.Time) ContainerCheck {
	return ContainerCheck{
		Name:      c.Name(),
		Image:     c.ImageName(),
		ImageID:   string(c.ImageID()),
		Timestamp: now,
	}
}

// applyReportMetadata copies resolved Git and OCI metadata onto a result.
//
// Only a concrete container exposes its labels, so a stubbed container keeps
// the empty metadata the response would otherwise report.
//
// Parameters:
//   - c: Container being checked.
//   - params: Update parameters.
//   - result: Result to populate in place.
//
// Returns:
//   - none.
func applyReportMetadata(c types.Container, params types.UpdateParams, result *ContainerCheck) {
	if _, concrete := c.(*container.Container); !concrete {
		return
	}

	meta := container.ResolveReportMeta(c, params, container.ChangelogVars{}, oci.Annotations{})
	result.GitRepo = meta.GitRepo
	result.GitRef = meta.GitRef
	result.Changelog = meta.Changelog
	result.OCISource = meta.Source
	result.ImageURL = meta.ImageURL
	result.Documentation = meta.Documentation
	result.CurrentImageVersion = meta.CurrentVersion
	result.CurrentImageRevision = meta.CurrentRevision
}

// applyLocalDigest records the local image digest when one can be resolved.
//
// Parameters:
//   - c: Container being checked.
//   - result: Result to populate in place.
//
// Returns:
//   - none.
func applyLocalDigest(c types.Container, result *ContainerCheck) {
	info := c.ImageInfo()
	if info == nil {
		return
	}

	result.Digest = container.ExtractImageDigest(info.RepoDigests, c.ImageName())
}

// isGitWatched reports whether a container is checked against its hosted Git
// source rather than its registry image.
//
// A nil Git client or a container without readable labels cannot take the Git
// path, so both fall back to the registry check.
//
// Parameters:
//   - log: Process logger.
//   - c: Container being checked.
//   - params: Update parameters.
//   - gitClient: Git client, or nil when Git monitoring is unavailable.
//
// Returns:
//   - bool: True when the Git source check applies.
func isGitWatched(
	log *zerolog.Logger,
	c types.Container,
	params types.UpdateParams,
	gitClient *git.Client,
) bool {
	if gitClient == nil {
		return false
	}

	if _, concrete := c.(*container.Container); !concrete {
		return false
	}

	return gitPkg.ShouldMonitor(log, c, params)
}

// checkGitSource compares the hosted Git ref for a container.
//
// It mirrors the scheduled update path: no-pull reports the same skip, and a
// resolved tag refines the changelog link to that release.
//
// Parameters:
//   - ctx: Context for the Git request.
//   - log: Process logger.
//   - c: Container being checked.
//   - params: Update parameters.
//   - gitClient: Git client.
//   - result: Result to populate in place.
//
// Returns:
//   - none.
func checkGitSource(
	ctx context.Context,
	log *zerolog.Logger,
	c types.Container,
	params types.UpdateParams,
	gitClient *git.Client,
	result *ContainerCheck,
) {
	result.UpdateSource = "git"

	// no-pull skips the Git rebuild. Report the same skip here.
	if c.IsNoPull(params) {
		return
	}

	checkResult, err := git.CheckContainer(ctx, gitClient, c, params)
	if err != nil {
		result.Error = err.Error()
		logCheckFailure(log, c, err, "Failed to check container Git source")

		return
	}

	result.UpdateAvailable = checkResult.Stale
	result.GitCommit = checkResult.Commit

	if checkResult.Tag != "" {
		result.Changelog = container.ResolveReportMeta(
			c,
			params,
			container.ChangelogVars{
				Tag:    checkResult.Tag,
				Commit: checkResult.Commit,
			},
			oci.Annotations{},
		).Changelog
	}
}

// checkRegistrySource compares the remote digest for a container.
//
// It uses the same watch split as a scheduled update, so no-pull and cooldown
// behavior match without pulling image layers.
//
// Parameters:
//   - ctx: Context for the registry request.
//   - log: Process logger.
//   - client: Docker client.
//   - c: Container being checked.
//   - params: Update parameters.
//   - result: Result to populate in place.
//
// Returns:
//   - none.
func checkRegistrySource(
	ctx context.Context,
	log *zerolog.Logger,
	client container.Client,
	c types.Container,
	params types.UpdateParams,
	result *ContainerCheck,
) {
	result.UpdateSource = "registry"

	available, latestID, latestDigest, err := client.CheckContainerUpdate(ctx, c, params)
	if err != nil {
		result.Error = err.Error()
		logCheckFailure(log, c, err, "Failed to check container for updates")

		return
	}

	result.UpdateAvailable = available

	if latestDigest != "" {
		result.LatestDigest = latestDigest
	}

	if latestID != "" {
		result.LatestImageID = string(latestID)
	}
}

// logCheckFailure records a per-container check failure.
//
// The notify field keeps the entry out of the notification stream, because a
// single failing container is already reported in the response body.
//
// Parameters:
//   - log: Process logger.
//   - c: Container that failed to check.
//   - err: Failure to record.
//   - message: Log message describing the failing check.
//
// Returns:
//   - none.
func logCheckFailure(log *zerolog.Logger, c types.Container, err error, message string) {
	log.Debug().
		Err(err).
		Str("container", c.Name()).
		Str("image", c.ImageName()).
		Str("notify", "no").
		Msg(message)
}
