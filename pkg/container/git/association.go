package git

import (
	"cmp"
	"strings"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// DefaultDockerfile is used when no Dockerfile path is configured.
const DefaultDockerfile = "Dockerfile"

// Association is a Git monitor association from labels or image mapping.
type Association struct {
	Repo       string
	Ref        string
	Policy     string
	Host       string
	Dockerfile string
	Context    string

	// RefFromDefault is true when Ref came from the process default, not a
	// label or image mapping. ApplyGitAssociation does not persist those values.
	RefFromDefault bool
	// PolicyFromDefault is true when Policy came from the process default.
	PolicyFromDefault bool
	// DockerfileFromDefault is true when Dockerfile came from the process default.
	DockerfileFromDefault bool
	// ContextFromDefault is true when Context came from the process default.
	ContextFromDefault bool
}

// ResolveAssociation returns the monitor association for a container.
//
// Highest wins: Watchtower labels, then an exact ImageName() mapping.
// OCI annotations never associate a container.
//
// Parameters:
//   - c: Container to inspect.
//   - params: Update parameters with image mappings and defaults.
//
// Returns:
//   - Association: Repo, ref, policy, and user-specified build paths when associated.
//     Dockerfile and Context are empty when unset. Callers default to Dockerfile
//     and the repository root.
//   - bool: True when a git-repo label or image mapping is present.
func ResolveAssociation(c types.Container, params types.UpdateParams) (Association, bool) {
	if c == nil {
		return Association{}, false
	}

	// Container labels win over --git-image mappings.
	if repo := label(c, RepoLabel); repo != "" {
		ref, refDefault := sourced(refLabel(c), "", defaultRef(params))
		policy, policyDefault := sourced(policyLabel(c), "", defaultPolicy(params))
		dockerfile, dockerfileDefault := sourced(label(c, DockerfileLabel), "", params.GitDockerfile)
		context, contextDefault := sourced(label(c, ContextLabel), "", params.GitContext)

		return Association{
			Repo:                  repo,
			Ref:                   ref,
			Policy:                policy,
			Host:                  label(c, HostLabel),
			Dockerfile:            dockerfile,
			Context:               context,
			RefFromDefault:        refDefault,
			PolicyFromDefault:     policyDefault,
			DockerfileFromDefault: dockerfileDefault,
			ContextFromDefault:    contextDefault,
		}, true
	}

	mapping := imageMapping(c.ImageName(), params.GitImages)
	if mapping.Repo == "" {
		return Association{}, false
	}

	ref, refDefault := sourced("", mapping.Ref, defaultRef(params))
	policy, policyDefault := sourced("", mapping.Policy, defaultPolicy(params))
	dockerfile, dockerfileDefault := sourced(label(c, DockerfileLabel), mapping.Dockerfile, params.GitDockerfile)
	context, contextDefault := sourced(label(c, ContextLabel), mapping.Context, params.GitContext)

	return Association{
		Repo:                  mapping.Repo,
		Ref:                   ref,
		Policy:                policy,
		Host:                  label(c, HostLabel),
		Dockerfile:            dockerfile,
		Context:               context,
		RefFromDefault:        refDefault,
		PolicyFromDefault:     policyDefault,
		DockerfileFromDefault: dockerfileDefault,
		ContextFromDefault:    contextDefault,
	}, true
}

// sourced returns the first non-empty value and whether it came from fallback.
//
// Parameters:
//   - primary: Highest-precedence value, usually a container label.
//   - secondary: Image-mapping value. Empty when there is no mapping.
//   - fallback: Process default.
//
// Returns:
//   - string: Chosen value.
//   - bool: True when the chosen value is the process default.
func sourced(primary, secondary, fallback string) (string, bool) {
	if primary != "" {
		return primary, false
	}

	if secondary != "" {
		return secondary, false
	}

	return fallback, fallback != ""
}

// ShouldMonitor reports whether Git monitoring should run for the container.
//
// Watchtower itself is excluded so registry self-update remains the default path.
// The container must be associated via a git-repo label or git-image mapping.
// git-watch=true without association stays on the registry path.
//
// Parameters:
//   - log: Optional logger for the unassociated git-watch=true debug line.
//   - c: Container to inspect.
//   - params: Update parameters including the process-wide monitor default.
//
// Returns:
//   - bool: True when Git staleness checks and rebuilds apply.
func ShouldMonitor(log *zerolog.Logger, c types.Container, params types.UpdateParams) bool {
	if c == nil || c.IsWatchtower() {
		return false
	}

	_, associated := ResolveAssociation(c, params)
	watch := isWatch(c, params)

	// A watch override without association is a misconfig. Stay on the registry path.
	if watch && !associated {
		if log != nil {
			log.Debug().
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Msg("git-watch is set but container is not associated via git-repo or --git-image")
		}

		return false
	}

	return associated && watch
}

// PersistWatch reports whether git-watch should be written onto a replacement.
//
// Only an explicit enabling label is persisted. Process-wide --git-enable
// must keep applying when the label is absent.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - bool: True when the source had an explicit on value for git-watch.
func PersistWatch(c types.Container) bool {
	if c == nil {
		return false
	}

	val, ok := c.GetLabel(WatchLabel)
	if !ok {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "t", "true", "yes":
		return true
	default:
		return false
	}
}

// WatchEnabled reports whether the per-container or process-wide Git watcher is on.
//
// A present git-watch label overrides --git-enable. When the label is absent
// or blank, the process-wide default is used.
//
// Parameters:
//   - c: Container to inspect.
//   - params: Update parameters with EnableGitMonitoring.
//
// Returns:
//   - bool: True when the watcher should run, ignoring association.
func WatchEnabled(c types.Container, params types.UpdateParams) bool {
	return isWatch(c, params)
}

// label returns the trimmed container label value for key.
//
// Parameters:
//   - c: Container to inspect.
//   - key: Label key.
//
// Returns:
//   - string: Trimmed value, or empty when the label is missing.
func label(c types.Container, key string) string {
	if c == nil {
		return ""
	}

	val, ok := c.GetLabel(key)
	if !ok {
		return ""
	}

	return strings.TrimSpace(val)
}

// refLabel returns git-ref, falling back to the git-branch alias.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - string: Ref label value, or empty.
func refLabel(c types.Container) string {
	return cmp.Or(label(c, RefLabel), label(c, branchLabel))
}

// policyLabel returns a valid semver policy from the container label.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - string: Canonical policy, or empty when unset or invalid.
func policyLabel(c types.Container) string {
	policy := strings.ToLower(label(c, SemverPolicyLabel))
	if types.ValidGitPolicy(policy) {
		return policy
	}

	return ""
}

// imageMapping returns the git-image mapping for an exact ImageName() key.
//
// Parameters:
//   - imageName: Container image name including tag.
//   - images: Process-wide mappings.
//
// Returns:
//   - types.GitImage: Zero value when no exact key exists.
func imageMapping(imageName string, images map[string]types.GitImage) types.GitImage {
	if images == nil || imageName == "" {
		return types.GitImage{}
	}

	return images[imageName]
}

// isWatch reports whether the per-container or process-wide monitor is on.
//
// Parameters:
//   - c: Container to inspect.
//   - params: Update parameters with EnableGitMonitoring.
//
// Returns:
//   - bool: True when the watcher should run, ignoring association.
func isWatch(c types.Container, params types.UpdateParams) bool {
	val, ok := c.GetLabel(WatchLabel)
	if !ok || strings.TrimSpace(val) == "" {
		return params.EnableGitMonitoring
	}

	// Match Docker's loose boolean parsing used elsewhere on Watchtower labels.
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "t", "true", "yes":
		return true
	case "0", "f", "false", "no":
		return false
	default:
		return params.EnableGitMonitoring
	}
}

// defaultRef returns the process default ref, or main.
//
// Parameters:
//   - params: Update parameters.
//
// Returns:
//   - string: Default Git ref.
func defaultRef(params types.UpdateParams) string {
	if params.GitDefaultRef != "" {
		return params.GitDefaultRef
	}

	return "main"
}

// defaultPolicy returns the process default semver policy, or none.
//
// Parameters:
//   - params: Update parameters.
//
// Returns:
//   - string: Canonical policy.
func defaultPolicy(params types.UpdateParams) string {
	if types.ValidGitPolicy(params.GitSemverPolicy) {
		return params.GitSemverPolicy
	}

	return types.GitPolicyNone
}
