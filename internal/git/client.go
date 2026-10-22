package git

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rs/zerolog"
	"golang.org/x/mod/semver"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Options configures a Git watcher client. Credentials stay here, not on UpdateParams.
type Options struct {
	Token           string
	Username        string
	Password        string
	SSHKeyPath      string
	SSHKnownHosts   string
	Timeout         time.Duration
	CABundle        []byte
	InsecureSkipTLS bool
	Hosts           map[string]string
}

// CheckRequest is a remote staleness query for one associated container.
type CheckRequest struct {
	Repo       string
	Ref        string
	Policy     string
	Host       string
	LastCommit string
	LastTag    string
}

// CheckResult is the outcome of a remote Git check.
type CheckResult struct {
	Stale  bool
	Commit string
	Tag    string
	Kind   string
	// Ref is the branch or tag name used to clone (empty when unknown).
	Ref string
}

// Client inspects remotes and clones repositories for Git-sourced updates.
type Client struct {
	log      *zerolog.Logger
	opts     Options
	http     *http.Client
	lister   RefLister
	origins  map[string]url.URL
	mu       sync.Mutex
	seen     map[string]seenStamp
	rejected map[string]rejectedApply
}

// seenStamp is a remote tip observed without rebuilding the running container.
type seenStamp struct {
	commit string
	tag    string
}

// rejectedApply is a commit written onto Compose containers before apply failed.
//
// previousCommit and previousTag are the baseline to compare on the next check.
type rejectedApply struct {
	commit         string
	previousCommit string
	previousTag    string
}

// unacceptedRevision is a baseline that cannot match a real Git object.
//
// It forces a retry when a failed apply had no previous stamp to restore.
const unacceptedRevision = "unaccepted"

// RefLister lists remote refs for staleness checks.
type RefLister interface {
	List(ctx context.Context, repo string) ([]RemoteRef, error)
}

// RemoteRef is a named ref from ls-remote or a provider tag list.
type RemoteRef struct {
	Name string
	Hash string
}

// resolvedRef is a branch or tag resolved to a commit.
type resolvedRef struct {
	Name string
	Hash string
	Kind string
}

const (
	kindBranch        = "branch"
	kindTag           = "tag"
	defaultGitTimeout = 30 * time.Second
)

// New constructs a Git watcher client.
//
// Parameters:
//   - log: Process logger. A nop logger is used when nil.
//   - opts: Auth, timeout, and optional extra host→provider mappings.
//
// Returns:
//   - *Client: Ready client. Safe to hold when no container is watched.
func New(log *zerolog.Logger, opts Options) *Client {
	if log == nil {
		log = new(zerolog.Nop())
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultGitTimeout
	}

	client := &Client{
		log:     log,
		opts:    opts,
		http:    newGitHTTPClient(timeout, opts.InsecureSkipTLS, opts.CABundle),
		origins: make(map[string]url.URL),
	}
	client.lister = gitLister{client: client}

	return client
}

// Check reports whether the remote has advanced past the known running revision.
//
// An empty last-commit or tag is not stale. There is nothing to compare, so
// Watchtower must not rebuild only to write a stamp. Missing refs return an
// error so the container can be skipped without failing the session.
//
// Parameters:
//   - ctx: Cancellation and deadline.
//   - req: Repo, ref, policy, and last observed stamp.
//
// Returns:
//   - CheckResult: Stale flag and the remote commit/tag to build.
//   - error: Non-nil on auth, network, or missing-ref failures.
func (c *Client) Check(ctx context.Context, req CheckRequest) (CheckResult, error) {
	if c == nil {
		return CheckResult{}, nil
	}

	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	req.Ref = cmpOr(req.Ref, "main")
	req.Policy = cmpOr(req.Policy, types.GitPolicyNone)

	err := validateRef(req.Ref)
	if err != nil {
		return CheckResult{}, err
	}

	if req.Policy != types.GitPolicyNone {
		return c.checkTagPolicy(ctx, req)
	}

	return c.checkExactRef(ctx, req)
}

// Clone checks out repo at the resolved revision into a new temp directory.
//
// The caller must remove the returned directory.
//
// Parameters:
//   - ctx: Cancellation and deadline.
//   - repo: Clone URL.
//   - rev: Commit, tag, kind, and ref from Check.
//
// Returns:
//   - string: Temporary directory containing the worktree.
//   - error: Non-nil on clone or checkout failure. The directory is removed on error.
func (c *Client) Clone(ctx context.Context, repo string, rev CheckResult) (string, error) {
	if c == nil {
		return "", fmt.Errorf("%w: client is nil", errCloneFailed)
	}

	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	dir, err := os.MkdirTemp("", "watchtower-git-*")
	if err != nil {
		return "", fmt.Errorf("%w: mkdir: %w", errCloneFailed, err)
	}

	err = cloneCheckout(ctx, dir, repo, rev, c)
	if err != nil {
		_ = os.RemoveAll(dir)

		return "", err
	}

	return dir, nil
}

// CheckContainer runs Check using association and stamp labels from the container.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - client: Git monitor client.
//   - c: Container to inspect.
//   - params: Update parameters with mappings and defaults.
//
// Returns:
//   - CheckResult: Empty result when the container is not associated.
//   - error: Non-nil on monitor failure.
func CheckContainer(
	ctx context.Context,
	client *Client,
	c types.Container,
	params types.UpdateParams,
) (CheckResult, error) {
	assoc, ok := gitPkg.ResolveAssociation(c, params)
	if !ok {
		return CheckResult{}, nil
	}

	if raw := strings.TrimSpace(policyLabelValue(c)); raw != "" && !types.ValidGitPolicy(strings.ToLower(raw)) {
		return CheckResult{}, fmt.Errorf("%w: %s", ErrInvalidPolicy, raw)
	}

	if raw := strings.TrimSpace(assoc.Host); raw != "" {
		_, err := gitPkg.ParseAPIOrigin(raw)
		if err != nil {
			return CheckResult{}, fmt.Errorf("%w: %w", ErrInvalidHost, err)
		}
	}

	lastCommit, lastTag := gitPkg.Baseline(c)
	if commit, tag, rejected := client.unacceptedBaseline(ApplyStampKeyFrom(c), lastCommit); rejected {
		lastCommit = commit
		lastTag = tag
	}

	if lastCommit == "" && lastTag == "" {
		lastCommit, lastTag = client.recall(assoc.Repo, assoc.Ref, assoc.Policy)
	}

	result, err := client.Check(ctx, CheckRequest{
		Repo:       assoc.Repo,
		Ref:        assoc.Ref,
		Policy:     assoc.Policy,
		Host:       assoc.Host,
		LastCommit: lastCommit,
		LastTag:    lastTag,
	})
	if err != nil {
		return result, err
	}

	// Record an observed tip only when this session did not rebuild. A stale
	// result must keep the previous baseline so a failed apply can retry.
	if !result.Stale {
		client.remember(assoc.Repo, assoc.Ref, assoc.Policy, result.Commit, result.Tag)
	}

	return result, nil
}

// WithTimeout returns ctx with the Git timeout unless a tighter deadline exists.
//
// Parameters:
//   - ctx: Parent context.
//
// Returns:
//   - context.Context: Context that expires at the Git timeout.
//   - context.CancelFunc: Cancel function. A no-op when ctx already expires sooner.
func (c *Client) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := defaultGitTimeout
	if c != nil && c.opts.Timeout > 0 {
		timeout = c.opts.Timeout
	}

	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, timeout)
}

// ApplyStampKey identifies one Compose member whose commit stamp may be rejected.
//
// Project, Dir, and ConfigFiles are the project identity. Service and Number
// identify the container within that project. CheckContainer rebuilds the same
// key from labels, so accepting one member does not clear another.
type ApplyStampKey struct {
	Project     string
	Dir         string
	ConfigFiles string
	Service     string
	Number      string
}

// ApplyStampKeyFrom reads the Compose identity and member from container labels.
//
// Parameters:
//   - c: Container whose labels are read.
//
// Returns:
//   - ApplyStampKey: Project name, directory, config files, service, and replica number.
func ApplyStampKeyFrom(c types.Container) ApplyStampKey {
	if c == nil {
		return ApplyStampKey{}
	}

	dir, _ := c.GetLabel(gitPkg.ComposeDirLabel)

	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ApplyStampKey{}
	}

	projectName, _ := c.GetLabel(compose.ComposeProjectLabel)
	files, _ := c.GetLabel(compose.ComposeConfigFilesLabel)
	service, _ := c.GetLabel(compose.ComposeServiceLabel)
	number, _ := c.GetLabel(compose.ComposeContainerNumber)

	service = strings.TrimSpace(service)
	if service == "" {
		service = strings.TrimPrefix(strings.TrimSpace(c.Name()), "/")
	}

	return ApplyStampKey{
		Project:     strings.TrimSpace(projectName),
		Dir:         dir,
		ConfigFiles: strings.TrimSpace(files),
		Service:     service,
		Number:      strings.TrimSpace(number),
	}
}

// memberKey is the map key for one member. Fields are separated so values cannot collide.
//
// Returns:
//   - string: Stable key, or empty when the member cannot be identified.
func (k ApplyStampKey) memberKey() string {
	if k.Dir == "" || k.Service == "" {
		return ""
	}

	return strings.Join([]string{k.Project, k.Dir, k.ConfigFiles, k.Service, k.Number}, "\x00")
}

// RejectApply records that commit must not be treated as this member's running baseline.
//
// Compose writes the commit into container labels before up. A failed apply
// cannot remove that label. The next check of this member uses its own previous
// baseline. Another project or service in the same directory is left unchanged.
//
// Parameters:
//   - key: Compose project and member identity.
//   - commit: Commit written by the failed apply.
//   - previousCommit: This member's baseline from before the apply.
//   - previousTag: This member's tag baseline from before the apply.
//
// Returns:
//   - none.
func (c *Client) RejectApply(key ApplyStampKey, commit, previousCommit, previousTag string) {
	memberKey := key.memberKey()
	if c == nil || memberKey == "" || commit == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rejected == nil {
		c.rejected = make(map[string]rejectedApply)
	}

	c.rejected[memberKey] = rejectedApply{
		commit:         strings.ToLower(strings.TrimSpace(commit)),
		previousCommit: previousCommit,
		previousTag:    previousTag,
	}
}

// AcceptApply clears one member's rejected stamp after a later apply is accepted.
//
// Parameters:
//   - key: Compose project and member identity.
//   - commit: Commit that may now be treated as this member's running baseline.
//
// Returns:
//   - none.
func (c *Client) AcceptApply(key ApplyStampKey, commit string) {
	memberKey := key.memberKey()
	if c == nil || memberKey == "" || commit == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	rejected, ok := c.rejected[memberKey]
	if !ok || !sameRevision(commit, rejected.commit) {
		return
	}

	delete(c.rejected, memberKey)
}

// repositoryOperationError wraps a remote operation failure without exposing repository credentials.
//
// Parameters:
//   - operation: Operation being performed.
//   - repo: Repository URL or path associated with the operation.
//   - err: Original remote error.
//
// Returns:
//   - error: Sanitized operation error preserving recognized transport and context failures.
func repositoryOperationError(operation, repo string, err error) error {
	safeRepo := sanitizeRepository(repo)
	if safeRepo == "" {
		safeRepo = "repository"
	}

	classified := classifyTransport(err)
	switch {
	case errors.Is(classified, errAuthRequired),
		errors.Is(classified, errAuthFailed),
		errors.Is(classified, errRepoNotFound):
		return fmt.Errorf("%s %s: %w", operation, safeRepo, classified)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("%s %s: %w", operation, safeRepo, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s %s: %w", operation, safeRepo, context.DeadlineExceeded)
	}

	detail := sanitizeRepositoryMessage(repo, err.Error())
	if detail == "" {
		detail = "remote operation failed"
	}

	return fmt.Errorf("%w: %s %s: %s", errRepositoryOperation, operation, safeRepo, detail)
}

// checkExactRef compares one branch or tag to the last-commit stamp.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - req: Repo, ref, and last commit.
//
// Returns:
//   - CheckResult: Stale when a known revision differs from the remote tip.
//   - error: Non-nil when the ref cannot be resolved.
func (c *Client) checkExactRef(ctx context.Context, req CheckRequest) (CheckResult, error) {
	resolved, err := c.resolveRef(ctx, req.Repo, req.Ref, req.Host)
	if err != nil {
		return CheckResult{}, err
	}

	stale := req.LastCommit != "" && !sameRevision(req.LastCommit, resolved.Hash)

	return CheckResult{
		Stale:  stale,
		Commit: resolved.Hash,
		Tag:    tagName(resolved),
		Kind:   resolved.Kind,
		Ref:    resolved.Name,
	}, nil
}

// checkTagPolicy selects a newer semver tag allowed by req.Policy.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - req: Repo, policy, and last tag stamp.
//
// Returns:
//   - CheckResult: Stale when a newer allowed tag exists.
//   - error: Non-nil when tags cannot be listed.
func (c *Client) checkTagPolicy(ctx context.Context, req CheckRequest) (CheckResult, error) {
	tags, err := c.listTags(ctx, req.Repo, req.Host)
	if err != nil {
		return CheckResult{}, err
	}

	names := make([]string, 0, len(tags))
	byName := make(map[string]string, len(tags))

	for _, tag := range tags {
		names = append(names, tag.Name)
		byName[tag.Name] = tag.Hash
	}

	lastTag := baselineTag(req.LastTag)
	inferred := false
	// An empty or non-semver stamp still names a release when the commit
	// matches that tag. Use the tag so patch and minor do not jump to the
	// highest release.
	if lastTag == "" && req.LastCommit != "" {
		if matched := tagForCommit(byName, req.LastCommit); matched != "" {
			lastTag = matched
			inferred = true
		}
	}

	if lastTag == "" && req.LastCommit == "" {
		selected, ok := selectTag("", names, req.Policy)
		if !ok {
			// No usable semver tags. Observe the configured ref without
			// treating the missing baseline as a reason to rebuild.
			return c.checkExactRef(ctx, CheckRequest{
				Repo:       req.Repo,
				Ref:        req.Ref,
				Policy:     types.GitPolicyNone,
				Host:       req.Host,
				LastCommit: req.LastCommit,
			})
		}

		return CheckResult{
			Stale:  false,
			Commit: byName[selected],
			Tag:    selected,
			Kind:   kindTag,
			Ref:    selected,
		}, nil
	}

	selected, ok := selectTag(lastTag, names, req.Policy)
	if !ok {
		if inferred {
			return CheckResult{
				Stale:  false,
				Commit: byName[lastTag],
				Tag:    lastTag,
				Kind:   kindTag,
				Ref:    lastTag,
			}, nil
		}

		return CheckResult{Stale: false}, nil
	}

	hash := byName[selected]
	// A known revision that already is the selected tag is the baseline.
	// Remember the tag so the next session does not treat a missing stamp
	// as a reason to rebuild.
	if req.LastCommit != "" && sameRevision(req.LastCommit, hash) {
		return CheckResult{
			Stale:  false,
			Commit: hash,
			Tag:    selected,
			Kind:   kindTag,
			Ref:    selected,
		}, nil
	}

	return CheckResult{
		Stale:  true,
		Commit: hash,
		Tag:    selected,
		Kind:   kindTag,
		Ref:    selected,
	}, nil
}

// resolveRef resolves ref via the product API, then go-git ls-remote.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - repo: Clone URL.
//   - ref: Branch or tag name.
//   - apiOrigin: Optional HTTP API base URL from the git-host label.
//
// Returns:
//   - resolvedRef: Name, hash, and kind.
//   - error: Non-nil when the ref is missing or listing fails.
func (c *Client) resolveRef(ctx context.Context, repo, ref, apiOrigin string) (resolvedRef, error) {
	resolved, ok, err := c.resolveViaAPI(ctx, repo, ref, apiOrigin)
	if err != nil {
		if !errors.Is(err, errAPINotFound) {
			c.log.Debug().
				Err(fmt.Errorf(
					"%w: %s",
					errRepositoryOperation,
					sanitizeRepositoryMessage(repo, err.Error()),
				)).
				Str("repo", sanitizeRepository(repo)).
				Str("ref", ref).
				Msg("Git API resolve failed, using ls-remote")
		}
	} else if ok {
		return resolved, nil
	}

	refs, err := c.lister.List(ctx, repo)
	if err != nil {
		return resolvedRef{}, repositoryOperationError("ls-remote", repo, err)
	}

	return resolveListedRef(refs, ref)
}

// listTags lists remote tags via the product API, then go-git ls-remote.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - repo: Clone URL.
//   - apiOrigin: Optional HTTP API base URL from the git-host label.
//
// Returns:
//   - []RemoteRef: Short tag names and commit hashes.
//   - error: Non-nil when listing fails.
func (c *Client) listTags(ctx context.Context, repo, apiOrigin string) ([]RemoteRef, error) {
	tags, ok, err := c.listTagsViaAPI(ctx, repo, apiOrigin)
	if err != nil {
		c.log.Debug().
			Err(fmt.Errorf(
				"%w: %s",
				errRepositoryOperation,
				sanitizeRepositoryMessage(repo, err.Error()),
			)).
			Str("repo", sanitizeRepository(repo)).
			Msg("Git API list tags failed, using ls-remote")
	} else if ok {
		return tags, nil
	}

	refs, err := c.lister.List(ctx, repo)
	if err != nil {
		return nil, repositoryOperationError("ls-remote tags", repo, err)
	}

	byName := collectTagHashes(refs)

	listed := make([]RemoteRef, 0, len(byName))
	for name, hash := range byName {
		listed = append(listed, RemoteRef{Name: name, Hash: hash})
	}

	return listed, nil
}

// unacceptedBaseline returns the baseline to use when labelCommit was not accepted.
//
// Parameters:
//   - key: Compose project and member identity from the running container.
//   - labelCommit: Stamp or image revision currently visible on the container.
//
// Returns:
//   - string: This member's previous commit, or a sentinel when none exists.
//   - string: This member's previous tag.
//   - bool: True when labelCommit matches this member's rejected apply.
func (c *Client) unacceptedBaseline(key ApplyStampKey, labelCommit string) (string, string, bool) {
	memberKey := key.memberKey()
	if c == nil || memberKey == "" || labelCommit == "" {
		return "", "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	rejected, ok := c.rejected[memberKey]
	if !ok || !sameRevision(labelCommit, rejected.commit) {
		return "", "", false
	}

	if rejected.previousCommit == "" && rejected.previousTag == "" {
		return unacceptedRevision, "", true
	}

	return rejected.previousCommit, rejected.previousTag, true
}

// recall returns a remote tip observed earlier in this process.
//
// Parameters:
//   - repo: Associated clone URL.
//   - ref: Watched branch or tag.
//   - policy: Update policy used for the watch key.
//
// Returns:
//   - string: Last observed commit SHA, or empty.
//   - string: Last observed tag, or empty.
func (c *Client) recall(repo, ref, policy string) (string, string) {
	if c == nil {
		return "", ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	seen, ok := c.seen[seenKey(repo, ref, policy)]
	if !ok {
		return "", ""
	}

	return seen.commit, seen.tag
}

// remember stores a remote tip observed without rebuilding.
//
// Parameters:
//   - repo: Associated clone URL.
//   - ref: Watched branch or tag.
//   - policy: Update policy used for the watch key.
//   - commit: Remote commit SHA to remember.
//   - tag: Remote tag to remember, or empty.
func (c *Client) remember(repo, ref, policy, commit, tag string) {
	if c == nil || (commit == "" && tag == "") {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.seen == nil {
		c.seen = make(map[string]seenStamp)
	}

	c.seen[seenKey(repo, ref, policy)] = seenStamp{commit: commit, tag: tag}
}

// tagForCommit returns the highest release tag whose hash matches commit.
//
// Parameters:
//   - byName: Tag name to commit hash.
//   - commit: Known running revision.
//
// Returns:
//   - string: Matching tag name, or empty.
func tagForCommit(byName map[string]string, commit string) string {
	bestName := ""
	bestCanon := ""

	for name, hash := range byName {
		if !sameRevision(commit, hash) {
			continue
		}

		canon := canonicalize(name)
		if !semver.IsValid(canon) || semver.Prerelease(canon) != "" {
			continue
		}

		if bestCanon == "" || semver.Compare(canon, bestCanon) > 0 {
			bestCanon = canon
			bestName = name
		}
	}

	return bestName
}

// tagName returns the resolved name when the ref is a tag.
//
// Parameters:
//   - resolved: Resolved branch or tag.
//
// Returns:
//   - string: Tag name, or empty for a branch.
func tagName(resolved resolvedRef) string {
	if resolved.Kind == kindTag {
		return resolved.Name
	}

	return ""
}

// cmpOr returns value, or fallback when value is empty.
//
// Parameters:
//   - value: Preferred string.
//   - fallback: Used when value is empty.
//
// Returns:
//   - string: Non-empty preference.
func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// seenKey identifies one watched repo, ref, and policy in process memory.
//
// Parameters:
//   - repo: Associated clone URL.
//   - ref: Watched branch or tag.
//   - policy: Update policy.
//
// Returns:
//   - string: Map key unique to that watch target.
func seenKey(repo, ref, policy string) string {
	return repo + "\n" + ref + "\n" + policy
}

// sameRevision reports whether observed names the same Git object as remote.
//
// A hex prefix of at least 7 characters matches a longer SHA so git-<shortsha>
// image tags compare equal to the full remote tip.
//
// Parameters:
//   - observed: Known running revision.
//   - remote: Resolved remote tip.
//
// Returns:
//   - bool: True when both name the same object.
func sameRevision(observed, remote string) bool {
	observed = strings.ToLower(strings.TrimSpace(observed))

	remote = strings.ToLower(strings.TrimSpace(remote))
	if observed == "" || remote == "" {
		return false
	}

	if observed == remote {
		return true
	}

	if !isHexSHA(observed) || !isHexSHA(remote) {
		return false
	}

	const minRevisionPrefix = 7

	if len(observed) < minRevisionPrefix || len(remote) < minRevisionPrefix {
		return false
	}

	if len(observed) < len(remote) {
		return strings.HasPrefix(remote, observed)
	}

	return strings.HasPrefix(observed, remote)
}

// isHexSHA reports whether s is a non-empty hexadecimal string.
//
// Parameters:
//   - s: Candidate SHA or prefix.
//
// Returns:
//   - bool: True when s is only hexadecimal digits.
func isHexSHA(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}

	return true
}

// policyLabelValue returns the raw git-semver-policy label, or empty.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - string: Raw label value, or empty when unset.
func policyLabelValue(c types.Container) string {
	if c == nil {
		return ""
	}

	val, ok := c.GetLabel(gitPkg.SemverPolicyLabel)
	if !ok {
		return ""
	}

	return val
}
