package git

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"

	goGit "github.com/go-git/go-git/v5"
)

// gitLister lists refs via go-git ls-remote.
type gitLister struct {
	client *Client
}

// List lists remote refs via go-git ls-remote.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - repo: Clone URL.
//
// Returns:
//   - []RemoteRef: Names and hashes, including peeled tags.
//   - error: Classified transport error on failure.
func (l gitLister) List(ctx context.Context, repo string) ([]RemoteRef, error) {
	auth, err := l.client.authMethod(repo)
	if err != nil {
		return nil, err
	}

	remote := goGit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repo},
	})

	refs, err := remote.ListContext(ctx, l.client.listOptions(auth))
	if err != nil {
		return nil, wrapRemoteErr("ls-remote", err)
	}

	out := make([]RemoteRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, RemoteRef{
			Name: ref.Name().String(),
			Hash: ref.Hash().String(),
		})
	}

	return out, nil
}

// listOptions builds ls-remote options including peeled tags and TLS settings.
//
// Parameters:
//   - auth: go-git auth method, or nil.
//
// Returns:
//   - *goGit.ListOptions: Options passed to ListContext.
func (c *Client) listOptions(auth transport.AuthMethod) *goGit.ListOptions {
	return &goGit.ListOptions{
		Auth:            auth,
		PeelingOption:   goGit.AppendPeeled,
		InsecureSkipTLS: c.opts.InsecureSkipTLS,
		CABundle:        c.opts.CABundle,
	}
}

// wrapRemoteErr prefixes err with op and maps known transport sentinels.
//
// Parameters:
//   - op: Operation name for the wrap, such as ls-remote.
//   - err: Underlying error.
//
// Returns:
//   - error: Wrapped error, or nil when err is nil.
func wrapRemoteErr(op string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", op, classifyTransport(err))
}

// classifyTransport maps go-git transport errors to package sentinels.
//
// Parameters:
//   - err: Error from clone or ls-remote.
//
// Returns:
//   - error: errAuthRequired, errAuthFailed, errRepoNotFound, or err unchanged.
func classifyTransport(err error) error {
	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired):
		return errAuthRequired
	case errors.Is(err, transport.ErrAuthorizationFailed):
		return errAuthFailed
	case errors.Is(err, transport.ErrRepositoryNotFound):
		return errRepoNotFound
	default:
		return err
	}
}
