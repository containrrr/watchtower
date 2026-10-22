package actions

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/registry"
	"github.com/nicholas-fedor/watchtower/internal/release"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// releaseTagLookupTimeout bounds the optional release tag lookup.
//
// The lookup is a convenience, so a slow or unresponsive registry must not stall
// the session. Each request already carries a registry client timeout, but this
// bounds the whole sweep so two candidates cannot double the wait.
const releaseTagLookupTimeout = 30 * time.Second

// latestImageMetadata describes a container's new image for report metadata.
//
// The annotations come from a local daemon inspect, not a registry request, so
// gathering them is free of rate-limit cost and always safe to do.
type latestImageMetadata struct {
	// annotations are the OCI labels of the newly pulled image.
	annotations oci.Annotations
	// digest is the manifest digest of the new image, or empty when the
	// staleness check could not determine one.
	digest string
}

// resolveLatestImageMeta computes the report metadata for a container's new
// image without touching session state.
//
// It performs the local image inspect and, only when the changelog feature is
// enabled, the release tag lookup. Callers run this before taking any lock
// shared across staleness workers, because a registry request under that lock
// would serialize every parallel check behind one container's round trip.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for the release tag lookup.
//   - resolver: Release tag resolver shared across the session.
//   - c: Container whose new image is described.
//   - config: Update parameters carrying the changelog enable gate.
//   - latest: Annotations and manifest digest of the new image.
//
// Returns:
//   - container.ChangelogVars: Placeholder values to apply.
//   - oci.Annotations: Annotations of the new image.
func resolveLatestImageMeta(
	log *zerolog.Logger,
	ctx context.Context,
	resolver *release.Resolver,
	c types.Container,
	config types.UpdateParams,
	latest latestImageMetadata,
) (container.ChangelogVars, oci.Annotations) {
	clogVal := log.With().
		Str("container", c.Name()).
		Str("image", c.ImageName()).
		Logger()
	clog := &clogVal

	concrete, ok := c.(*container.Container)
	if !ok || !concrete.IsChangelogEnabled(config) {
		// The gate is off. Still record the new image's version and revision so
		// a custom template can print a transition, but never spend a request
		// to confirm a release tag.
		return container.ChangelogVars{}, latest.annotations
	}

	return container.ChangelogVars{
		Tag: resolveReleaseTag(clog, ctx, resolver, c, latest),
	}, latest.annotations
}

// applyLatestImageMeta records pre-resolved new image metadata on a container
// status and emits the changelog notification entry.
//
// Callers must resolve the metadata with resolveLatestImageMeta first, then
// call this while holding whatever lock guards session state.
//
// Parameters:
//   - log: Process logger.
//   - progress: Session progress map.
//   - c: Container whose status should be updated.
//   - config: Update parameters carrying the changelog enable gate.
//   - vars: Placeholder values from resolveLatestImageMeta.
//   - annotations: Annotations of the new image.
//
// Returns:
//   - none.
func applyLatestImageMeta(
	log *zerolog.Logger,
	progress *session.Progress,
	c types.Container,
	config types.UpdateParams,
	vars container.ChangelogVars,
	annotations oci.Annotations,
) {
	if progress == nil || c == nil {
		return
	}

	meta := progress.SetLatestImageMeta(log, c, config, vars, annotations)

	logChangelog(log, c, config, meta)
}

// resolveReleaseTag confirms which release tag matches the pulled image, or
// returns empty when the lookup is unnecessary or unsuccessful.
//
// An empty result is not a failure. The caller falls back to the unversioned
// release index, which is the link Watchtower shipped before this feature.
//
// Parameters:
//   - clog: Logger scoped to the container.
//   - ctx: Context for the lookup.
//   - resolver: Release tag resolver shared across the session.
//   - c: Container whose image supplies the repository and registry host.
//   - latest: Annotations and manifest digest of the new image.
//
// Returns:
//   - string: The confirmed release tag, or empty.
func resolveReleaseTag(
	clog *zerolog.Logger,
	ctx context.Context,
	resolver *release.Resolver,
	c types.Container,
	latest latestImageMetadata,
) string {
	version := latest.annotations.Version
	if version == "" || latest.digest == "" {
		clog.Debug().
			Str("version", version).
			Str("digest", latest.digest).
			Msg("Skipping release tag lookup without an OCI version or new digest")

		return ""
	}

	opts, err := registry.GetPullOptions(clog, c.ImageName())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Skipping release tag lookup without registry credentials")

		return ""
	}

	return resolver.ResolveTag(clog, ctx, c, opts.RegistryAuth, version, latest.digest)
}
