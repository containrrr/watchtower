package types

import (
	"net"
	"strings"
)

// Git semver-policy names used by labels, --git-image mappings, and UpdateParams.
const (
	// GitPolicyNone follows a single branch or tag ref without advancing tags.
	GitPolicyNone = "none"
	// GitPolicyPatch advances to newer patch releases of the same minor version.
	GitPolicyPatch = "patch"
	// GitPolicyMinor advances to newer minor or patch releases of the same major version.
	GitPolicyMinor = "minor"
	// GitPolicyMajor advances to any newer semantic version.
	GitPolicyMajor = "major"
)

// Detected REST/changelog provider names. Unmapped hosts use go-git only.
const (
	// GitHostGitHub is GitHub.com or GitHub Enterprise Server.
	GitHostGitHub = "github"
	// GitHostGitLab is GitLab.com or self-hosted GitLab.
	GitHostGitLab = "gitlab"
	// GitHostGitea is Gitea (Forgejo uses the same API and changelog URLs).
	GitHostGitea = "gitea"
	// GitHostForgejo is treated as GitHostGitea.
	GitHostForgejo = "forgejo"
)

// GitImage is a non-secret per-image Git association.
type GitImage struct {
	Repo       string `json:"repo"`
	Ref        string `json:"ref"`
	Policy     string `json:"policy"`
	Dockerfile string `json:"dockerfile,omitempty"`
	Context    string `json:"context,omitempty"`
}

// ValidGitPolicy reports whether policy is one of none, patch, minor, or major.
//
// Parameters:
//   - policy: Policy name to check.
//
// Returns:
//   - bool: True when policy is recognized.
func ValidGitPolicy(policy string) bool {
	switch policy {
	case GitPolicyNone, GitPolicyPatch, GitPolicyMinor, GitPolicyMajor:
		return true
	default:
		return false
	}
}

// canonicalGitHostKind maps a provider name to github, gitlab, or gitea.
//
// Forgejo shares Gitea's API and release URL shape.
//
// Parameters:
//   - kind: Provider name.
//
// Returns:
//   - string: github, gitlab, gitea, or empty.
func canonicalGitHostKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case GitHostGitHub:
		return GitHostGitHub
	case GitHostGitLab:
		return GitHostGitLab
	case GitHostGitea, GitHostForgejo:
		return GitHostGitea
	default:
		return ""
	}
}

// ResolveGitHostKind classifies a repository hostname.
//
// github.com, gitlab.com, and codeberg.org are always classified.
// codeberg.org is a public Forgejo host and uses the Gitea/Forgejo API.
// Other hosts use extra mappings when present, typically from tests.
// An empty result means go-git only.
//
// Parameters:
//   - host: Hostname without scheme or port.
//   - extra: Optional hostname-to-provider map (keys lowercased).
//
// Returns:
//   - string: github, gitlab, gitea, or empty.
func ResolveGitHostKind(host string, extra map[string]string) string {
	host = strings.ToLower(strings.TrimSpace(host))

	host = strings.Trim(host, "[]")

	hostname, _, err := net.SplitHostPort(host)
	if err == nil {
		host = hostname
	}

	host = strings.TrimSuffix(host, ".")

	switch host {
	case "github.com":
		return GitHostGitHub
	case "gitlab.com":
		return GitHostGitLab
	case "codeberg.org":
		return GitHostGitea
	}

	if extra == nil {
		return ""
	}

	return canonicalGitHostKind(extra[host])
}
