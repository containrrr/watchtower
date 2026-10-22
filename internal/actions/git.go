package actions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/git/apply"
	"github.com/nicholas-fedor/watchtower/internal/git/project"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// gitShortSHALen is the number of hex characters used in name:git-<shortsha> tags and notifications.
const gitShortSHALen = 12

// gitSession holds watcher results for one Update() invocation.
type gitSession struct {
	client   *git.Client
	apply    compose.Applier
	checkout func(ctx context.Context, dir, commit string, opts project.CheckoutOptions) error
	mu       sync.Mutex
	results  map[types.ContainerID]git.CheckResult
	built    map[types.ContainerID]types.ImageID
	applied  map[types.ContainerID]struct{}
}

// composeMember is one stale container and the check result that selected its commit.
type composeMember struct {
	container types.Container
	result    git.CheckResult
}

// composeBatch is one on-disk Compose project to checkout and apply.
type composeBatch struct {
	ref     compose.ProjectRef
	members []composeMember
}

type composeBatchKey struct {
	name        string
	dir         string
	configFiles string
}

// composeBatchKeyFor returns the identity used to group one Compose project batch.
//
// Parameters:
//   - ref: Resolved Compose project reference.
//
// Returns:
//   - composeBatchKey: Identity derived from the project name, directory, and config files.
func composeBatchKeyFor(ref compose.ProjectRef) composeBatchKey {
	return composeBatchKey{
		name:        ref.Name,
		dir:         ref.Dir,
		configFiles: strings.Join(ref.ConfigFiles, "\x00"),
	}
}

// newGitSession constructs an empty session for one Update() call.
//
// Parameters:
//   - client: Git monitor client. May be unused when no container is monitored.
//
// Returns:
//   - *gitSession: Session with empty result and built maps.
func newGitSession(client *git.Client) *gitSession {
	return &gitSession{
		client:   client,
		apply:    compose.NewClient(),
		checkout: project.Checkout,
		results:  make(map[types.ContainerID]git.CheckResult),
		built:    make(map[types.ContainerID]types.ImageID),
		applied:  make(map[types.ContainerID]struct{}),
	}
}

// store records a monitor result for containerID.
//
// Parameters:
//   - containerID: Container identity.
//   - result: Check outcome from the Git client.
//
// Returns:
//   - none.
func (s *gitSession) store(containerID types.ContainerID, result git.CheckResult) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.results[containerID] = result
}

// result returns the stored check for containerID.
//
// Parameters:
//   - containerID: Container identity.
//
// Returns:
//   - git.CheckResult: Stored result.
//   - bool: True when a result was stored.
func (s *gitSession) result(containerID types.ContainerID) (git.CheckResult, bool) {
	if s == nil {
		return git.CheckResult{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, ok := s.results[containerID]

	return result, ok
}

// watching reports whether Git monitoring applies to c in this session.
//
// Parameters:
//   - log: Process logger.
//   - c: Container to inspect.
//   - params: Update parameters.
//
// Returns:
//   - bool: True when the session should check Git instead of the registry.
func (s *gitSession) watching(log *zerolog.Logger, c types.Container, params types.UpdateParams) bool {
	if s == nil || s.client == nil {
		return false
	}

	return gitPkg.ShouldMonitor(log, c, params)
}

// check runs a Git staleness check and stores the result.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - c: Associated container.
//   - params: Update parameters including cooldown.
//
// Returns:
//   - bool: True when the remote ref has advanced past the known running revision.
//   - types.ImageID: Synthetic Git image id used by the update pipeline.
//   - error: Non-nil on monitor or cooldown failure.
func (s *gitSession) check(
	ctx context.Context,
	c types.Container,
	params types.UpdateParams,
) (bool, types.ImageID, error) {
	result, err := git.CheckContainer(ctx, s.client, c, params)
	if err != nil {
		return false, "", fmt.Errorf("git check: %w", err)
	}

	s.store(c.ID(), result)

	if !result.Stale {
		return false, c.ImageID(), nil
	}

	err = container.CheckLocalImageCooldown(c, params)
	if err != nil {
		return false, "", fmt.Errorf("git cooldown: %w", err)
	}

	newest := types.ImageID("git:" + result.Commit)
	if newest == "git:" {
		newest = types.ImageID("git:unknown")
	}

	return true, newest, nil
}

// prepareRebuilds applies stale Git-associated containers.
//
// compose-dir or --compose-project selects a local path context. Watchtower
// checks out the Compose project directory, then runs compose up or compose build.
// Git labels alone use a Git URL context. The Docker daemon clones that URL.
//
// Failures are recorded in failed and those containers are unmarked stale so
// the current session does not stop them. Checkout and build failures leave the
// running instance untouched. Compose apply failures report that Docker Compose
// may have partially recreated the batch. An unreadable compose project
// directory is not recorded in failed. That container is unmarked stale, marked
// skipped when progress is set, and is not rebuilt from a Git URL.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Cancellation and timeout.
//   - docker: Docker client used for ImageBuild.
//   - containers: Candidates already marked stale.
//   - params: Update parameters.
//   - progress: Session progress, or nil.
//   - failed: Map to record per-container build errors.
//
// Returns:
//   - none.
func (s *gitSession) prepareRebuilds(
	log *zerolog.Logger,
	ctx context.Context,
	docker container.Client,
	containers []types.Container,
	params types.UpdateParams,
	progress *session.Progress,
	failed map[types.ContainerID]error,
) {
	if s == nil || s.client == nil {
		return
	}

	var remote []types.Container

	ordered := orderedComposeContainers(log, containers, params.UseComposeDependsOn)
	batches := make(map[composeBatchKey]*composeBatch)
	orderedBatches := make([]*composeBatch, 0)

	for _, c := range ordered {
		if !s.needsApply(c, params) {
			continue
		}

		result, _ := s.result(c.ID())

		ref, err := compose.ResolveProjectDir(containerLabels(c), params.ComposeProjects)
		if err != nil {
			c.SetStale(false)

			if progress != nil {
				progress.AddSkipped(log, c, err, params)
			}

			log.Warn().
				Err(err).
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Msg("Compose project directory is not readable. Leaving the running container untouched")

			continue
		}

		if ref.Dir == "" {
			remote = append(remote, c)

			continue
		}

		key := composeBatchKeyFor(ref)

		batch, ok := batches[key]
		if !ok {
			batch = &composeBatch{ref: ref}
			batches[key] = batch
			orderedBatches = append(orderedBatches, batch)
		}

		batch.members = append(batch.members, composeMember{container: c, result: result})
	}

	orderedBatches = orderComposeBatches(log, orderedBatches, containers, params.UseComposeDependsOn)

	for _, batch := range orderedBatches {
		s.applyCompose(log, ctx, batch, params, progress, failed)
	}

	for _, c := range remote {
		result, _ := s.result(c.ID())

		err := s.buildOne(log, ctx, docker, c, result, params, progress)
		if err != nil {
			failed[c.ID()] = err
			c.SetStale(false)
			withGitMeta(log.Warn().
				Err(err).
				Str("container", c.Name()).
				Str("image", c.ImageName()),
				c, params, result.Tag, result.Commit).
				Msg("Git build failed. Leaving running container untouched")
		}
	}
}

// orderedComposeContainers returns a dependency-ordered copy of the containers.
//
// Parameters:
//   - log: Process logger.
//   - containers: Containers to order.
//   - useComposeDependsOn: Whether to include Compose depends_on relationships when sorting.
//
// Returns:
//   - []types.Container: Containers in dependency order, or a copy of the original order if sorting fails.
func orderedComposeContainers(
	log *zerolog.Logger,
	containers []types.Container,
	useComposeDependsOn bool,
) []types.Container {
	ordered := slices.Clone(containers)
	if len(ordered) == 0 || len(ordered) == 1 {
		return ordered
	}

	err := sorter.SortByDependencies(log, ordered, useComposeDependsOn)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Could not dependency-sort Compose batches. Using the existing container order")

		return slices.Clone(containers)
	}

	return ordered
}

// orderComposeBatches orders project batches by cross-project dependencies.
//
// First-seen order is wrong when a project contains both an independent service
// and a service that depends on another project. A batch runs after every batch
// that contains a dependency of any of its members. Independent batches keep
// their relative order. A sort failure leaves the existing order in place.
//
// Parameters:
//   - log: Process logger.
//   - batches: Batches in first-seen order.
//   - containers: Containers used to resolve dependency links.
//   - useComposeDependsOn: Whether to include Compose depends_on relationships.
//
// Returns:
//   - []*composeBatch: Batches in dependency order.
func orderComposeBatches(
	log *zerolog.Logger,
	batches []*composeBatch,
	containers []types.Container,
	useComposeDependsOn bool,
) []*composeBatch {
	const minBatchesToOrder = 2
	if len(batches) < minBatchesToOrder {
		return batches
	}

	dependencies, err := sorter.ContainerDependencies(log, containers, useComposeDependsOn)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Could not order Compose batches by dependencies. Using first-seen order")

		return batches
	}

	batchOf := make(map[types.ContainerID]int, len(containers))

	for i, batch := range batches {
		for _, member := range batch.members {
			batchOf[member.container.ID()] = i
		}
	}

	indegree := make([]int, len(batches))
	after := make([][]int, len(batches))
	seen := make(map[[2]int]struct{})

	for i, batch := range batches {
		for _, member := range batch.members {
			for _, dependencyID := range dependencies[member.container.ID()] {
				predecessor, ok := batchOf[dependencyID]
				if !ok || predecessor == i {
					continue
				}

				edge := [2]int{predecessor, i}
				if _, exists := seen[edge]; exists {
					continue
				}

				seen[edge] = struct{}{}

				after[predecessor] = append(after[predecessor], i)
				indegree[i]++
			}
		}
	}

	ready := make([]int, 0, len(batches))

	for i, degree := range indegree {
		if degree == 0 {
			ready = append(ready, i)
		}
	}

	ordered := make([]*composeBatch, 0, len(batches))
	for len(ready) > 0 {
		pick := 0
		for i := 1; i < len(ready); i++ {
			if ready[i] < ready[pick] {
				pick = i
			}
		}

		current := ready[pick]
		ready = append(ready[:pick], ready[pick+1:]...)
		ordered = append(ordered, batches[current])

		for _, next := range after[current] {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
			}
		}
	}

	if len(ordered) == len(batches) {
		return ordered
	}

	log.Warn().
		Msg("Compose batch dependencies are cyclic. Appending unresolved batches in first-seen order")

	for i, batch := range batches {
		if indegree[i] > 0 {
			ordered = append(ordered, batch)
		}
	}

	return ordered
}

// needsApply reports whether c is a stale Git-associated container that may be rebuilt.
//
// Parameters:
//   - c: Candidate container.
//   - params: Update parameters.
//
// Returns:
//   - bool: True when a Git apply backend should run.
func (s *gitSession) needsApply(c types.Container, params types.UpdateParams) bool {
	if c == nil || !c.IsStale() {
		return false
	}

	result, ok := s.result(c.ID())
	if !ok || !result.Stale {
		return false
	}

	return !c.IsNoPull(params) && !c.IsMonitorOnly(params)
}

// buildOne builds one image from a Git URL context and stamps the container.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Cancellation and timeout.
//   - docker: Docker client.
//   - c: Stale associated container.
//   - result: Monitor result with the target commit.
//   - params: Update parameters.
//   - progress: Session progress, or nil.
//
// Returns:
//   - error: Non-nil when association, remote URL, or the host build fails.
func (s *gitSession) buildOne(
	log *zerolog.Logger,
	ctx context.Context,
	docker container.Client,
	c types.Container,
	result git.CheckResult,
	params types.UpdateParams,
	progress *session.Progress,
) error {
	assoc, associated := gitPkg.ResolveAssociation(c, params)
	if !associated {
		return errGitNotAssociated
	}

	dockerfile, err := apply.SafeRelPath(assoc.Dockerfile)
	if err != nil {
		return fmt.Errorf("git dockerfile: %w", err)
	}

	if dockerfile == "" {
		dockerfile = container.DefaultGitDockerfile
	}

	remote, err := apply.RemoteContext(assoc.Repo, result.Commit, assoc.Context)
	if err != nil {
		return fmt.Errorf("git remote context: %w", err)
	}

	user, token := s.client.BuildAuth(assoc.Repo)

	remote, err = apply.WithAuth(remote, user, token)
	if err != nil {
		return fmt.Errorf("git remote auth: %w", err)
	}

	originalName := c.ImageName()
	tag := gitImageTag(originalName, result.Commit)
	tags := []string{tag}

	if originalName != "" && originalName != tag && validDockerTag(originalName) {
		tags = append(tags, originalName)
	}

	logFoundGitRevision(log, c, params, result.Tag, result.Commit)

	buildCtx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	imageID, err := docker.BuildRemoteImage(buildCtx, remote, dockerfile, tags)
	if err != nil {
		return fmt.Errorf("build git image: %w", err)
	}

	if concrete, ok := c.(*container.Container); ok {
		container.ApplyGitAssociation(concrete, assoc, gitPkg.PersistWatch(c))

		concrete.SetImageName(tag)
		container.ApplyGitStamp(concrete, result.Commit, result.Tag)
	}

	s.mu.Lock()
	s.built[c.ID()] = imageID
	s.mu.Unlock()

	if progress != nil {
		progress.SetLatestImage(log, c.ID(), imageID)
		// A Git rebuild has no pulled image, so no latest image annotations are
		// passed and LatestImageVersion stays empty. The resolved Git tag is the
		// only new version available for this path.
		meta := progress.SetLatestImageMeta(log, c, params, container.ChangelogVars{
			Tag:    result.Tag,
			Commit: result.Commit,
		}, oci.Annotations{})

		logChangelog(log, c, params, meta)
	}

	withGitMeta(log.Info().
		Str("container", c.Name()).
		Str("image", familiarImage(tag)).
		Str("image_id", imageID.ShortID()),
		c, params, result.Tag, result.Commit).
		Msg("Built image from Git URL context")

	return nil
}

// withGitMeta adds the repository, ref, selected tag, commit, short commit, and changelog when they are known.
//
// Parameters:
//   - evt: Log event to extend.
//   - c: Container the event describes.
//   - params: Update parameters used to resolve the report fields.
//   - tag: New tag for changelog placeholders.
//   - commit: New commit for changelog placeholders.
//
// Returns:
//   - *zerolog.Event: evt, or nil when evt is nil.
func withGitMeta(
	evt *zerolog.Event,
	c types.Container,
	params types.UpdateParams,
	tag, commit string,
) *zerolog.Event {
	if evt == nil {
		return nil
	}

	meta := container.ResolveReportMeta(c, params, container.ChangelogVars{
		Tag:    tag,
		Commit: commit,
	}, oci.Annotations{})
	if meta.GitRepo != "" {
		evt = evt.Str("repo", meta.GitRepo)
	}

	if meta.GitRef != "" {
		evt = evt.Str("ref", meta.GitRef)
	}

	if tag != "" {
		evt = evt.Str("tag", tag)
	}

	if commit != "" {
		evt = evt.Str("commit", commit)
		if short := shortGitSHA(commit); short != "" && short != commit {
			evt = evt.Str("short_commit", short)
		}
	}

	if meta.Changelog != "" {
		evt = evt.Str("changelog", meta.Changelog)
	}

	return evt
}

// skipRecreate reports whether inspect recreate should be skipped for c.
//
// Compose-applied containers are always skipped. Git URL context builds are
// skipped only when no-restart is set. Watchtower itself still uses the inspect path.
//
// Parameters:
//   - c: Container to inspect.
//   - params: Update parameters.
//
// Returns:
//   - bool: True when this session already replaced or must not replace c.
func (s *gitSession) skipRecreate(c types.Container, params types.UpdateParams) bool {
	if s == nil || c.IsWatchtower() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.applied[c.ID()]; ok {
		return true
	}

	if !params.NoRestart {
		return false
	}

	_, built := s.built[c.ID()]

	return built
}

// excludeApplied drops containers already recreated by Compose apply.
//
// Parameters:
//   - containers: Full candidate list.
//
// Returns:
//   - []types.Container: Containers that may still use inspect recreate.
func (s *gitSession) excludeApplied(containers []types.Container) []types.Container {
	if s == nil {
		return containers
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.applied) == 0 {
		return containers
	}

	kept := make([]types.Container, 0, len(containers))

	for _, c := range containers {
		if _, ok := s.applied[c.ID()]; ok {
			continue
		}

		kept = append(kept, c)
	}

	return kept
}

// excludeNoRestart drops Git-built containers from the recreate list when no-restart is set.
//
// Parameters:
//   - containers: Full candidate list.
//   - params: Update parameters.
//
// Returns:
//   - []types.Container: Containers that may still be recreated.
func (s *gitSession) excludeNoRestart(
	containers []types.Container,
	params types.UpdateParams,
) []types.Container {
	if s == nil || !params.NoRestart {
		return containers
	}

	kept := make([]types.Container, 0, len(containers))

	for _, c := range containers {
		if s.skipRecreate(c, params) {
			continue
		}

		kept = append(kept, c)
	}

	return kept
}

// validDockerTag reports whether imageName is a valid Docker reference without a digest.
//
// Parameters:
//   - imageName: Docker image reference to validate.
//
// Returns:
//   - bool: True when the reference is valid and does not use a digest.
func validDockerTag(imageName string) bool {
	ref, err := reference.ParseDockerRef(imageName)
	if err != nil {
		return false
	}

	_, isDigest := ref.(reference.Digested)

	return !isDigest
}

// gitImageTag builds name:git-<shortsha> from the current image name.
//
// Parameters:
//   - imageName: Current image reference.
//   - commit: Full commit SHA.
//
// Returns:
//   - string: Tag applied to the built image.
func gitImageTag(imageName, commit string) string {
	short := commit
	if len(short) > gitShortSHALen {
		short = short[:gitShortSHALen]
	}

	named, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		base := imageName
		if i := strings.LastIndex(base, ":"); i > 0 && !strings.Contains(base[i:], "/") {
			base = base[:i]
		}

		return base + ":git-" + short
	}

	return named.Name() + ":git-" + short
}

// logFoundGitRevision records that a hosted ref advanced, before a build or checkout.
//
// Parameters:
//   - log: Process logger.
//   - c: Container whose repository is being checked.
//   - params: Update parameters used to resolve the repository and ref.
//   - tag: Selected semver tag, or empty for a branch tip.
//   - commit: Target commit SHA.
//
// Returns:
//   - none.
func logFoundGitRevision(log *zerolog.Logger, c types.Container, params types.UpdateParams, tag, commit string) {
	if log == nil || c == nil {
		return
	}

	assoc, _ := gitPkg.ResolveAssociation(c, params)

	ref := tag
	if ref == "" {
		ref = assoc.Ref
	}

	identity := gitNoticePath(assoc.Repo)
	if ref != "" && identity != "" {
		identity += "@" + ref
	}

	log.Info().
		Str("container", c.Name()).
		Str("revision", identity).
		Str("short_commit", shortGitSHA(commit)).
		Msg("Found new Git revision")
}

// gitNoticePath returns the host and repository path used in a notification.
//
// Parameters:
//   - repo: Clone URL, including https, ssh, or git@host:path forms.
//
// Returns:
//   - string: Host and path without a scheme or .git suffix.
func gitNoticePath(repo string) string {
	repo = strings.TrimSpace(repo)

	repo = strings.TrimSuffix(repo, ".git")
	if rest, ok := strings.CutPrefix(repo, "git@"); ok {
		host, path, found := strings.Cut(rest, ":")
		if found {
			return host + "/" + strings.TrimPrefix(path, "/")
		}
	}

	parsed, err := url.Parse(repo)
	if err == nil && parsed.Host != "" {
		return parsed.Host + "/" + strings.TrimPrefix(parsed.Path, "/")
	}

	return strings.TrimPrefix(repo, "/")
}

// familiarImage returns the image name a notification should show.
//
// Parameters:
//   - imageName: Docker image reference, possibly with the default registry prefix.
//
// Returns:
//   - string: Familiar name and tag, or imageName when it cannot be parsed.
func familiarImage(imageName string) string {
	ref, err := reference.ParseDockerRef(imageName)
	if err != nil {
		return imageName
	}

	return reference.FamiliarString(ref)
}

// shortGitSHA returns the notification form of a commit SHA.
//
// Parameters:
//   - commit: Full or already-short commit SHA.
//
// Returns:
//   - string: At most gitShortSHALen characters, or commit when it is already short.
func shortGitSHA(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) <= gitShortSHALen {
		return commit
	}

	return commit[:gitShortSHALen]
}

// warnGitConfigSkip logs one warning for a Git label that fails closed.
//
// Invalid policy and git-host values are otherwise recorded only at debug.
// The warning omits the raw git-host value so a credential in that label is not notified.
//
// Parameters:
//   - log: Process logger.
//   - name: Container name.
//   - image: Container image name.
//   - err: Staleness check error.
//
// Returns:
//   - none.
func warnGitConfigSkip(log *zerolog.Logger, name, image string, err error) {
	if log == nil || err == nil {
		return
	}

	event := log.Warn().Str("container", name).Str("image", image)

	switch {
	case errors.Is(err, git.ErrInvalidPolicy):
		event.Msg("Skipped container with an invalid git semver policy")
	case errors.Is(err, git.ErrInvalidHost):
		event.Msg("Skipped container with an invalid git-host")
	}
}
