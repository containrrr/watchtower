package actions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/git/project"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// applyCompose checkouts a Compose project once and applies stale associated services.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Cancellation and timeout.
//   - batch: Containers that share one Compose project identity.
//   - params: Update parameters including stash and no-restart.
//   - progress: Session progress, or nil.
//   - failed: Map to record per-container apply errors.
//
// Returns:
//   - none.
func (s *gitSession) applyCompose(
	log *zerolog.Logger,
	ctx context.Context,
	batch *composeBatch,
	params types.UpdateParams,
	progress *session.Progress,
	failed map[types.ContainerID]error,
) {
	if batch == nil || len(batch.members) == 0 {
		return
	}

	commit, err := sharedComposeCommit(batch.members)
	if err != nil {
		failComposeBatch(log, batch, params, failed, err)

		return
	}

	services := make([]string, 0, len(batch.members))
	labels := make(map[string]map[string]string, len(batch.members))
	byService := make(map[string][]composeMember, len(batch.members))

	for _, member := range batch.members {
		c := member.container

		name := compose.GetServiceName(containerLabels(c))
		if name == "" {
			failed[c.ID()] = errGitComposeService
			c.SetStale(false)
			log.Warn().
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Msg("Compose apply skipped. Container has no compose service label")

			continue
		}

		if _, seen := byService[name]; !seen {
			services = append(services, name)
			labels[name] = composeServiceLabels(c, params, member.result)
		}

		byService[name] = append(byService[name], member)
	}

	if len(services) == 0 {
		return
	}

	checkout := s.checkout
	if checkout == nil {
		checkout = project.Checkout
	}

	opts := project.CheckoutOptions{Stash: params.GitComposeStash}
	if s.client != nil {
		opts.AuthFor = s.client.Auth
		opts.CABundle, opts.InsecureSkipTLS = s.client.TLSSettings()
	}

	var prior string

	opts.Prior = &prior

	if len(batch.members) > 0 {
		member := batch.members[0]
		logFoundGitRevision(log, member.container, params, member.result.Tag, commit)
	}

	checkoutCtx, cancelCheckout := s.client.WithTimeout(ctx)
	err = checkout(checkoutCtx, batch.ref.Dir, commit, opts)

	cancelCheckout()

	if err != nil {
		failComposeBatch(log, batch, params, failed, fmt.Errorf("compose checkout: %w", err))

		return
	}

	// A nil checkout error means the worktree is already at the commit.
	// Do not discard that result because the derived timeout elapsed as it
	// returned. If the parent is canceled, do not start apply, and put the
	// worktree back so a bind mount does not keep the new tree.
	ctxErr := ctx.Err()
	if ctxErr != nil {
		cause := fmt.Errorf("compose checkout canceled: %w", ctxErr)

		restoreErr := project.RestoreHead(batch.ref.Dir, prior, params.GitComposeStash)
		if restoreErr != nil {
			cause = errors.Join(cause, fmt.Errorf("rollback checkout: %w", restoreErr))
		}

		failComposeBatch(log, batch, params, failed, cause)

		return
	}

	applier := s.apply
	if applier == nil {
		applier = compose.NewClient()
	}

	applyCtx, cancelApply := s.client.WithTimeout(ctx)
	applied, err := applier.Apply(applyCtx, compose.Request{
		Ref:       batch.ref,
		Services:  services,
		Labels:    labels,
		BuildOnly: params.NoRestart,
	})

	cancelApply()

	if err != nil {
		applyErr := composeApplyError(params, err)
		markComposeResultFailures(applied, progress, failed, applyErr)

		if !params.NoRestart {
			s.rejectComposeApply(batch)
		}

		failComposeBatchWithRuntime(log, batch, params, failed, applyErr, !params.NoRestart)

		return
	}

	appliedByService := make(map[string][]compose.Container, len(applied))
	for _, item := range applied {
		appliedByService[item.Service] = append(appliedByService[item.Service], item)
	}

	for _, name := range services {
		members := byService[name]

		items := appliedByService[name]
		if len(items) == 0 {
			for _, member := range members {
				c := member.container
				failed[c.ID()] = errGitComposeInstance
				c.SetStale(false)
				log.Warn().
					Str("container", c.Name()).
					Str("image", c.ImageName()).
					Str("service", name).
					Msg("Compose apply result omitted this service. Runtime state is unknown")
			}

			continue
		}

		byName := make(map[string]compose.Container, len(items))
		for _, item := range items {
			if item.Name != "" {
				byName[strings.TrimPrefix(item.Name, "/")] = item
			}
		}

		matched := make(map[types.ContainerID]struct{}, len(items))

		for _, member := range members {
			c := member.container

			item, matchedInstance := matchedComposeInstance(c, members, items, byName)
			if !matchedInstance || (!params.NoRestart && item.ID == "") {
				failed[c.ID()] = errGitComposeInstance
				c.SetStale(false)
				log.Warn().
					Str("container", c.Name()).
					Str("image", c.ImageName()).
					Str("service", name).
					Msg("Compose apply did not return an identifiable instance for this replica. Runtime state is unknown")

				continue
			}

			s.recordCompose(c, item, params, member.result, progress, log)
			matched[c.ID()] = struct{}{}
		}

		s.recordComposeReplicas(items, members, params, progress, log, matched)
	}

	if !params.NoRestart {
		s.acceptComposeApply(batch, failed)
	}

	if params.NoRestart {
		log.Info().
			Str("project", batch.ref.Name).
			Str("service", strings.Join(services, ", ")).
			Msg("Built Compose project")

		return
	}

	for _, member := range batch.members {
		if _, failed := failed[member.container.ID()]; failed {
			log.Warn().
				Str("project", batch.ref.Name).
				Str("service", strings.Join(services, ", ")).
				Msg("Compose apply finished with unconfirmed service results")

			return
		}
	}
}

// logComposeReplaced records the same stop and start lines a registry update uses.
//
// Parameters:
//   - log: Process logger.
//   - c: Container that was replaced.
//   - item: Compose instance after apply.
//
// Returns:
//   - none.
func logComposeReplaced(log *zerolog.Logger, c types.Container, item compose.Container) {
	if log == nil || c == nil || item.ID == "" || item.ID == c.ID() {
		return
	}

	name := c.Name()
	log.Info().
		Str("container", name).
		Str("id", c.ID().ShortID()).
		Msg("Stopping container")
	log.Info().
		Str("container", name).
		Str("image", c.ImageName()).
		Str("new_id", item.ID.ShortID()).
		Msg("Started new container")
}

// recordCompose records a successful Compose apply for one container.
//
// Parameters:
//   - c: Stale associated container.
//   - item: Compose instance after apply.
//   - params: Update parameters.
//   - result: Monitor result used for stamps.
//   - progress: Session progress, or nil.
//   - log: Process logger.
//
// Returns:
//   - none.
func (s *gitSession) recordCompose(
	c types.Container,
	item compose.Container,
	params types.UpdateParams,
	result git.CheckResult,
	progress *session.Progress,
	log *zerolog.Logger,
) {
	if !params.NoRestart && item.ID == "" {
		return
	}

	assoc, _ := gitPkg.ResolveAssociation(c, params)
	if concrete, ok := c.(*container.Container); ok {
		container.ApplyGitAssociation(concrete, assoc, gitPkg.PersistWatch(c))
		container.ApplyGitStamp(concrete, result.Commit, result.Tag)
	}

	s.mu.Lock()
	if params.NoRestart {
		if item.ImageID != "" {
			s.built[c.ID()] = item.ImageID
		} else {
			s.built[c.ID()] = types.ImageID("git:" + result.Commit)
		}
	} else {
		s.applied[c.ID()] = struct{}{}
		if item.ImageID != "" {
			s.built[c.ID()] = item.ImageID
		}
	}
	s.mu.Unlock()

	if !params.NoRestart {
		logComposeReplaced(log, c, item)
	}

	if progress == nil {
		return
	}

	if item.ImageID != "" {
		progress.SetLatestImage(log, c.ID(), item.ImageID)
	}

	// A Git rebuild has no pulled image, so no latest image annotations are
	// passed and LatestImageVersion stays empty.
	meta := progress.SetLatestImageMeta(log, c, params, container.ChangelogVars{
		Tag:    result.Tag,
		Commit: result.Commit,
	}, oci.Annotations{})

	logChangelog(log, c, params, meta)

	if item.ID == "" {
		return
	}

	status, ok := (*progress)[c.ID()]
	if !ok || status == nil {
		return
	}

	status.SetNewContainerID(item.ID)

	if !params.NoRestart {
		progress.MarkForUpdate(log, c.ID())
	}
}

// markComposeResultFailures records apply failures for result containers represented in session progress.
//
// Parameters:
//   - items: Compose results returned before the failure.
//   - progress: Session progress, or nil.
//   - failed: Map to record errors.
//   - err: Failure to record for each matching tracked container.
//
// Returns:
//   - none.
func markComposeResultFailures(
	items []compose.Container,
	progress *session.Progress,
	failed map[types.ContainerID]error,
	err error,
) {
	if progress == nil {
		return
	}

	statuses := make(map[string]struct{}, len(*progress))
	for _, status := range *progress {
		if status != nil {
			statuses[strings.TrimPrefix(status.Name(), "/")] = struct{}{}
		}
	}

	for _, item := range items {
		if item.Name == "" {
			continue
		}

		name := strings.TrimPrefix(item.Name, "/")
		if _, known := statuses[name]; known {
			for id, status := range *progress {
				if status != nil && strings.TrimPrefix(status.Name(), "/") == name {
					failed[id] = err
				}
			}
		}
	}
}

// recordComposeReplicas records successful Compose results for tracked replicas that were not already matched.
//
// Parameters:
//   - items: Compose results returned for the service.
//   - members: Containers that share the service name.
//   - params: Update parameters.
//   - progress: Session progress containing tracked replica statuses.
//   - log: Process logger.
//   - matched: Container IDs already recorded from direct instance matches.
//
// Returns:
//   - none.
func (s *gitSession) recordComposeReplicas(
	items []compose.Container,
	members []composeMember,
	params types.UpdateParams,
	progress *session.Progress,
	log *zerolog.Logger,
	matched map[types.ContainerID]struct{},
) {
	if len(members) == 0 {
		return
	}

	if matched == nil {
		matched = make(map[types.ContainerID]struct{}, len(items))
	}

	statuses := make(map[string]*session.ContainerStatus)
	if progress != nil {
		statuses = make(map[string]*session.ContainerStatus, len(*progress))
		for _, status := range *progress {
			if status != nil {
				statuses[strings.TrimPrefix(status.Name(), "/")] = status
			}
		}
	}

	representative := members[0]

	for _, item := range items {
		if item.Name == "" || item.ID == "" {
			continue
		}

		status, ok := statuses[strings.TrimPrefix(item.Name, "/")]
		if !ok {
			log.Debug().
				Str("service", item.Service).
				Str("replica", item.Name).
				Msg("Compose recreated a replica absent from filtered progress")

			continue
		}

		if _, alreadyMatched := matched[status.ID()]; alreadyMatched {
			continue
		}

		s.mu.Lock()
		if params.NoRestart {
			if item.ImageID != "" {
				s.built[status.ID()] = item.ImageID
			} else {
				s.built[status.ID()] = types.ImageID("git:" + representative.result.Commit)
			}
		} else {
			s.applied[status.ID()] = struct{}{}
			if item.ImageID != "" {
				s.built[status.ID()] = item.ImageID
			}
		}
		s.mu.Unlock()

		if item.ImageID != "" {
			progress.SetLatestImage(log, status.ID(), item.ImageID)
		}

		meta := container.ResolveReportMeta(representative.container, params, container.ChangelogVars{
			Tag:    representative.result.Tag,
			Commit: representative.result.Commit,
		}, oci.Annotations{})
		status.SetGitMetadata(meta)

		if !params.NoRestart {
			status.SetNewContainerID(item.ID)
			progress.MarkForUpdate(log, status.ID())
		}

		matched[status.ID()] = struct{}{}
	}
}

// acceptComposeApply clears the rejected stamp for members this apply confirmed.
//
// Members that were not confirmed keep their own rejection. Another project
// in the same directory is not cleared.
//
// Parameters:
//   - batch: Project whose apply returned success.
//   - failed: Members whose result was not accepted.
//
// Returns:
//   - none.
func (s *gitSession) acceptComposeApply(batch *composeBatch, failed map[types.ContainerID]error) {
	if s == nil || s.client == nil || batch == nil {
		return
	}

	for _, member := range batch.members {
		if _, ok := failed[member.container.ID()]; ok || member.result.Commit == "" {
			continue
		}

		s.client.AcceptApply(git.ApplyStampKeyFrom(member.container), member.result.Commit)
	}
}

// rejectComposeApply records that a failed apply must not accept its new stamp.
//
// Compose writes the commit into container labels before up, and those labels
// cannot be removed after create. The next check substitutes the previous
// baseline while that stamp remains rejected.
//
// Parameters:
//   - batch: Project whose apply failed after containers may have been replaced.
//
// Returns:
//   - none.
func (s *gitSession) rejectComposeApply(batch *composeBatch) {
	if s == nil || s.client == nil || batch == nil {
		return
	}

	for _, member := range batch.members {
		if member.result.Commit == "" {
			continue
		}

		previousCommit, previousTag := gitPkg.Baseline(member.container)
		s.client.RejectApply(
			git.ApplyStampKeyFrom(member.container),
			member.result.Commit,
			previousCommit,
			previousTag,
		)
	}
}

// composeApplyError annotates a Compose failure with the operation that failed.
//
// Parameters:
//   - params: Update parameters selecting build-only or apply mode.
//   - err: Compose failure to annotate.
//
// Returns:
//   - error: Failure annotated as a Compose build or apply error.
func composeApplyError(params types.UpdateParams, err error) error {
	if params.NoRestart {
		return fmt.Errorf("compose build failed: %w", err)
	}

	return fmt.Errorf("compose apply failed, Docker Compose may have partially recreated services: %w", err)
}

// failComposeBatch records a Compose update error on every pending batch member.
//
// Parameters:
//   - log: Process logger.
//   - batch: Project batch.
//   - params: Update parameters used to resolve repository and changelog fields.
//   - failed: Map to record errors.
//   - err: Apply or checkout error.
//
// Returns:
//   - none.
func failComposeBatch(
	log *zerolog.Logger,
	batch *composeBatch,
	params types.UpdateParams,
	failed map[types.ContainerID]error,
	err error,
) {
	failComposeBatchWithRuntime(log, batch, params, failed, err, false)
}

// failComposeBatchWithRuntime marks a failed Compose batch non-stale and logs possible runtime changes.
//
// Parameters:
//   - log: Process logger.
//   - batch: Project batch.
//   - params: Update parameters used to resolve repository and changelog fields.
//   - failed: Map to record errors.
//   - err: Failure to record when a member has no more specific error.
//   - runtimeMayHaveChanged: Whether the failed operation may already have replaced services.
//
// Returns:
//   - none.
func failComposeBatchWithRuntime(
	log *zerolog.Logger,
	batch *composeBatch,
	params types.UpdateParams,
	failed map[types.ContainerID]error,
	err error,
	runtimeMayHaveChanged bool,
) {
	message := "Compose update failed before container replacement"
	if runtimeMayHaveChanged {
		message = "Compose apply failed. Docker Compose may have partially recreated services"
	}

	for _, member := range batch.members {
		c := member.container

		failureErr, exists := failed[c.ID()]
		if !exists {
			failureErr = err
			failed[c.ID()] = err
		}

		c.SetStale(false)
		withGitMeta(log.Warn().
			Err(failureErr).
			Str("container", c.Name()).
			Str("image", c.ImageName()).
			Str("dir", batch.ref.Dir),
			c, params, member.result.Tag, member.result.Commit).
			Msg(message)
	}
}

// matchedComposeInstance finds the compose result for one container.
//
// A single container uses the only result. Replicas match by container name.
// An unmatched replica is not applied.
//
// Parameters:
//   - c: Container being recorded.
//   - members: Containers that share the service name.
//   - items: Compose results for that service.
//   - byName: Results keyed by container name without a leading slash.
//
// Returns:
//   - compose.Container: Matched instance.
//   - bool: False when the replica cannot be identified.
func matchedComposeInstance(
	c types.Container,
	members []composeMember,
	items []compose.Container,
	byName map[string]compose.Container,
) (compose.Container, bool) {
	if named, ok := byName[strings.TrimPrefix(c.Name(), "/")]; ok {
		return named, true
	}

	if len(members) == 1 && len(items) == 1 {
		return items[0], true
	}

	return compose.Container{}, false
}

// sharedComposeCommit returns the commit every member resolved.
//
// One project identity has one worktree, so a mismatch fails the batch.
//
// Parameters:
//   - members: Stale containers in one project directory.
//
// Returns:
//   - string: Shared commit.
//   - error: errGitComposeCommit when the commits differ or one is empty.
func sharedComposeCommit(members []composeMember) (string, error) {
	var commit string

	for _, member := range members {
		got := member.result.Commit
		if got == "" {
			return "", errGitComposeCommit
		}

		if commit == "" {
			commit = got

			continue
		}

		if got != commit {
			return "", errGitComposeCommit
		}
	}

	if commit == "" {
		return "", errGitComposeCommit
	}

	return commit, nil
}

// containerLabels returns the container config labels, or nil.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - map[string]string: Label map.
func containerLabels(c types.Container) map[string]string {
	if c == nil {
		return nil
	}

	info := c.ContainerInfo()
	if info == nil || info.Config == nil {
		return nil
	}

	return info.Config.Labels
}

// safeComposeAPIOrigin returns a credential-free API origin or an empty string for invalid input.
//
// Parameters:
//   - raw: API origin to normalize and redact.
//
// Returns:
//   - string: Normalized origin without credentials, query parameters, or a fragment.
func safeComposeAPIOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parseRaw := raw
	if !strings.Contains(parseRaw, "://") {
		parseRaw = "https://" + parseRaw
	}

	parsed, err := url.Parse(parseRaw)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""

	origin, err := gitPkg.ParseAPIOrigin(parsed.String())
	if err != nil {
		return ""
	}

	return strings.TrimRight(origin.String(), "/")
}

// composeServiceLabels is the stamp and association labels written onto new Compose containers.
//
// Parameters:
//   - c: Source container.
//   - params: Update parameters.
//   - result: Monitor result.
//
// Returns:
//   - map[string]string: Labels merged into the compose service.
func composeServiceLabels(c types.Container, params types.UpdateParams, result git.CheckResult) map[string]string {
	// The commit is written at create time because container labels cannot be
	// changed afterward. A failed apply rejects this stamp so the next check
	// retries from the previous baseline.
	out := map[string]string{
		gitPkg.LastCommitLabel: result.Commit,
	}

	if result.Tag != "" {
		out[gitPkg.LastTagLabel] = result.Tag
	}

	assoc, ok := gitPkg.ResolveAssociation(c, params)
	if ok && assoc.Repo != "" {
		meta := container.ResolveReportMeta(c, params, container.ChangelogVars{
			Tag:    result.Tag,
			Commit: result.Commit,
		}, oci.Annotations{})
		if meta.GitRepo != "" {
			out[gitPkg.RepoLabel] = meta.GitRepo
		}

		if assoc.Ref != "" {
			out[gitPkg.RefLabel] = assoc.Ref
		}

		if assoc.Policy != "" {
			out[gitPkg.SemverPolicyLabel] = assoc.Policy
		}

		if assoc.Host != "" {
			if origin := safeComposeAPIOrigin(assoc.Host); origin != "" {
				out[gitPkg.HostLabel] = origin
			}
		}

		if assoc.Dockerfile != "" {
			out[gitPkg.DockerfileLabel] = assoc.Dockerfile
		}

		if assoc.Context != "" {
			out[gitPkg.ContextLabel] = assoc.Context
		}

		if gitPkg.PersistWatch(c) {
			out[gitPkg.WatchLabel] = "true"
		}
	}

	if dir, present := c.GetLabel(gitPkg.ComposeDirLabel); present && dir != "" {
		out[gitPkg.ComposeDirLabel] = dir
	}

	return out
}
