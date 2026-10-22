package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	goGit "github.com/go-git/go-git/v5"
)

// cloneCheckout clones repo into dir and checks out rev.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - dir: Empty destination directory.
//   - repo: Clone URL.
//   - rev: Commit, tag, and kind from Check.
//   - client: Git client for auth and TLS options.
//
// Returns:
//   - error: errCloneFailed wrapping the underlying cause.
func cloneCheckout(ctx context.Context, dir, repo string, rev CheckResult, client *Client) error {
	auth, err := client.authMethod(repo)
	if err != nil {
		return fmt.Errorf("%w: %w", errCloneFailed, err)
	}

	repoObj, err := goGit.PlainCloneContext(ctx, dir, false, client.cloneOptions(repo, rev, auth))
	if err != nil {
		return fmt.Errorf("%w: %w", errCloneFailed, repositoryOperationError("clone", repo, err))
	}

	target := revisionTarget(rev)

	hash, err := repoObj.ResolveRevision(plumbing.Revision(target))
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %w", errCloneFailed, target, err)
	}

	worktree, err := repoObj.Worktree()
	if err != nil {
		return fmt.Errorf("%w: worktree: %w", errCloneFailed, err)
	}

	err = worktree.Checkout(&goGit.CheckoutOptions{
		Hash:  *hash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("%w: checkout %s: %w", errCloneFailed, hash.String(), err)
	}

	return nil
}

// cloneOptions builds a shallow, single-branch clone request.
//
// Parameters:
//   - repo: Clone URL.
//   - rev: Used to set ReferenceName when a branch or tag is known.
//   - auth: go-git auth method, or nil.
//
// Returns:
//   - *goGit.CloneOptions: Options for PlainCloneContext.
func (c *Client) cloneOptions(repo string, rev CheckResult, auth transport.AuthMethod) *goGit.CloneOptions {
	opts := &goGit.CloneOptions{
		URL:             repo,
		Auth:            auth,
		Depth:           1,
		SingleBranch:    true,
		Tags:            goGit.NoTags,
		NoCheckout:      true,
		InsecureSkipTLS: c.opts.InsecureSkipTLS,
		CABundle:        c.opts.CABundle,
	}

	if name := cloneReference(rev); name != "" {
		opts.ReferenceName = name
	}

	return opts
}
