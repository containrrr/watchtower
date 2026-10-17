package git

import (
	"errors"
	"fmt"
)

// Errors returned by the Git watcher.
var (
	// errRefNotFound indicates the requested branch or tag does not exist.
	errRefNotFound = errors.New("git ref not found")
	// errInvalidRef indicates a ref name is not a safe Git reference.
	errInvalidRef = errors.New("invalid git ref")
	// errCloneFailed indicates clone or checkout failed.
	errCloneFailed = errors.New("git clone failed")
	// errAuthRequired indicates the remote rejected anonymous access.
	errAuthRequired = errors.New("git authentication required")
	// errAuthFailed indicates the configured credentials were rejected.
	errAuthFailed = errors.New("git authorization failed")
	// errRepoNotFound indicates the remote repository does not exist.
	errRepoNotFound = errors.New("git repository not found")
	// ErrInvalidPolicy indicates a git-semver-policy label is not none/patch/minor/major.
	ErrInvalidPolicy = errors.New("invalid git semver policy")
	// ErrInvalidHost indicates a git-host label is not an HTTP base URL.
	ErrInvalidHost = errors.New("invalid git-host")
	// errRepositoryOperation identifies a sanitized remote operation failure.
	errRepositoryOperation = errors.New("git repository operation failed")
)

// Errors for HTTP provider probes.
var (
	// errCrossHostRedirect indicates a Location header pointed at another origin.
	errCrossHostRedirect = errors.New("git http redirect left the original host")
	// errEmptyProbeURL indicates a fingerprint request had no host.
	errEmptyProbeURL = errors.New("empty probe url")
	// errAPINotFound indicates the provider has no such ref.
	errAPINotFound = fmt.Errorf("%w", errRefNotFound)
	// errAPIStatus indicates a non-success HTTP status from a Git provider.
	errAPIStatus = errors.New("git provider http error")
)
