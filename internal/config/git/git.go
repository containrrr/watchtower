// Package git holds Git association and watcher settings.
package git

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// DefaultRef is used when a mapping or label omits a Git ref.
	DefaultRef = "main"
	// DefaultPolicy is used when a mapping or label omits a semver policy.
	DefaultPolicy = types.GitPolicyNone
)

var (
	// ErrInvalidGitImage indicates a --git-image value could not be parsed.
	ErrInvalidGitImage = errors.New("invalid --git-image value")
	// errInvalidGitPolicy indicates a semver policy is not none/patch/minor/major.
	errInvalidGitPolicy = errors.New("invalid git semver policy")
	// ErrInvalidComposeProject indicates a --compose-project value is not name=/path.
	ErrInvalidComposeProject = errors.New("invalid --compose-project value")
)

// Git holds process-wide Git metadata and watcher settings.
type Git struct {
	// Enable is the process-wide watcher default (--git-enable / WATCHTOWER_GIT_ENABLE).
	Enable bool
	// DefaultRef is the ref used when a mapping or label omits one.
	DefaultRef string
	// SemverPolicy is the semver policy used when a mapping or label omits one.
	SemverPolicy string
	// Timeout is the Git network timeout (--git-timeout / WATCHTOWER_GIT_TIMEOUT).
	Timeout time.Duration
	// Token is the HTTPS auth token. Never projected onto UpdateParams.
	Token string
	// Username is the basic-auth username. Never projected onto UpdateParams.
	Username string
	// Password is the basic-auth password. Never projected onto UpdateParams.
	Password string
	// SSHKeyPath is the path to an SSH private key. Never projected onto UpdateParams.
	SSHKeyPath string
	// SSHKnownHosts is the path to an SSH known_hosts file. Never projected onto UpdateParams.
	SSHKnownHosts string
	// CABundle is the PEM CA bundle for Git HTTPS. Never projected onto UpdateParams.
	CABundle []byte
	// InsecureSkipTLS skips TLS verification for Git HTTPS.
	InsecureSkipTLS bool
	// Dockerfile is the process-wide default Dockerfile path relative to the build context.
	Dockerfile string
	// Context is the process-wide default subdirectory of a Git URL context used as the Docker build context.
	Context string
	// Images maps exact image names (including tag) to Git associations.
	Images map[string]types.GitImage
	// ComposeStash saves and restores local Compose project files around checkout.
	ComposeStash bool
	// ComposeProjects maps Compose project name to a directory inside Watchtower.
	ComposeProjects map[string]string
}

// parseImageMapping parses one --git-image value of the form image=repo[#ref][@policy].
//
// Policy suffixes are recognized only when they are a known policy name so
// userinfo in SSH URLs (git@host:path) is not treated as a policy.
//
// Parameters:
//   - raw: Raw mapping string.
//
// Returns:
//   - image: Exact image name including tag.
//   - mapping: Parsed repo, ref, and policy.
//   - error: Non-nil when the value is malformed or the policy is unknown.
func parseImageMapping(raw string) (string, types.GitImage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", types.GitImage{}, fmt.Errorf("%w: empty value", ErrInvalidGitImage)
	}

	image, rest, ok := strings.Cut(raw, "=")
	if !ok {
		return "", types.GitImage{}, fmt.Errorf("%w: %q (want image=repo[#ref][@policy])", ErrInvalidGitImage, raw)
	}

	image = strings.TrimSpace(image)
	rest = strings.TrimSpace(rest)

	if image == "" || rest == "" {
		return "", types.GitImage{}, fmt.Errorf("%w: %q (want image=repo[#ref][@policy])", ErrInvalidGitImage, raw)
	}

	policy := DefaultPolicy

	if at := strings.LastIndex(rest, "@"); at >= 0 {
		maybePolicy := strings.ToLower(strings.TrimSpace(rest[at+1:]))
		if types.ValidGitPolicy(maybePolicy) {
			policy = maybePolicy
			rest = rest[:at]
		} else if isPolicyToken(maybePolicy) {
			return "", types.GitImage{}, fmt.Errorf("%w: %q", errInvalidGitPolicy, maybePolicy)
		}
	}

	ref := DefaultRef
	if hash := strings.LastIndex(rest, "#"); hash >= 0 {
		ref = strings.TrimSpace(rest[hash+1:])
		rest = rest[:hash]

		if ref == "" {
			ref = DefaultRef
		}
	}

	repo := strings.TrimSpace(rest)
	if repo == "" {
		return "", types.GitImage{}, fmt.Errorf("%w: %q (empty repo)", ErrInvalidGitImage, raw)
	}

	return image, types.GitImage{
		Repo:   repo,
		Ref:    ref,
		Policy: policy,
	}, nil
}

// ParseImageMappings parses repeatable --git-image values.
//
// Duplicate image keys keep the last mapping.
//
// Parameters:
//   - values: Raw mapping strings.
//
// Returns:
//   - map[string]types.GitImage: Image name to mapping.
//   - error: Non-nil when any value is invalid.
func ParseImageMappings(values []string) (map[string]types.GitImage, error) {
	if len(values) == 0 {
		return map[string]types.GitImage{}, nil
	}

	images := make(map[string]types.GitImage, len(values))

	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		image, mapping, err := parseImageMapping(raw)
		if err != nil {
			return nil, err
		}

		images[image] = mapping
	}

	return images, nil
}

// isPolicyToken reports whether s looks like an explicit @policy suffix.
//
// Userinfo in SSH URLs (git@host) is not treated as a policy.
//
// Parameters:
//   - s: Text after @ in a git-image mapping.
//
// Returns:
//   - bool: True when s is a bare policy name.
func isPolicyToken(s string) bool {
	if s == "" {
		return false
	}

	return !strings.ContainsAny(s, "/:@")
}

// ParseComposeProjects parses repeatable --compose-project values of the form name=/path.
//
// Parameters:
//   - values: Raw name=/path strings.
//
// Returns:
//   - map[string]string: Project name to directory.
//   - error: Non-nil when a value is not name=/path.
func ParseComposeProjects(values []string) (map[string]string, error) {
	out := make(map[string]string, len(values))

	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		name, dir, ok := strings.Cut(raw, "=")
		name = strings.TrimSpace(name)
		dir = strings.TrimSpace(dir)

		if !ok || name == "" || dir == "" {
			return nil, fmt.Errorf("%w: %q", ErrInvalidComposeProject, raw)
		}

		out[name] = dir
	}

	return out, nil
}
