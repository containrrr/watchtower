package git

import (
	"net/url"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	semverPartCount = 3
	patchIndex      = 2
)

// ChangelogVars substitutes placeholders in an explicit changelog template.
type ChangelogVars struct {
	Tag    string
	Commit string
}

// Changelog returns an explicit template, a derived releases URL, or empty.
//
// It does not read OCI annotations. Callers may fall back to OCI URL or docs.
// When vars.Tag is set, a derived URL points at that specific release rather
// than the release index.
//
// Parameters:
//   - c: Container that may carry a changelog or git-host label.
//   - repo: Associated or inferred Git clone URL.
//   - vars: Placeholder values for an explicit template and the derived release tag.
//
// Returns:
//   - string: Changelog URL, or empty when none can be derived.
func Changelog(c types.Container, repo string, vars ChangelogVars) string {
	if template := label(c, ChangelogURLLabel); template != "" {
		return applyVars(template, vars)
	}

	apiOrigin := ""
	if c != nil {
		apiOrigin = label(c, HostLabel)
	}

	return derivedReleasesURL(repo, apiOrigin, vars.Tag)
}

// applyVars substitutes {tag}, {commit}, and semver placeholders in template.
//
// Parameters:
//   - template: URL template from the changelog label.
//   - vars: Values to substitute. Unknown parts leave the placeholder intact.
//
// Returns:
//   - string: Template after substitution.
func applyVars(template string, vars ChangelogVars) string {
	major, minor, patch := splitSemverParts(vars.Tag)

	replacer := strings.NewReplacer(
		"{major}", orUnchanged(major, "{major}"),
		"{minor}", orUnchanged(minor, "{minor}"),
		"{patch}", orUnchanged(patch, "{patch}"),
		"{tag}", orUnchanged(vars.Tag, "{tag}"),
		"{commit}", orUnchanged(vars.Commit, "{commit}"),
	)

	return replacer.Replace(template)
}

// orUnchanged returns value, or placeholder when value is empty.
//
// Parameters:
//   - value: Substitution value.
//   - placeholder: Token to keep when value is empty.
//
// Returns:
//   - string: Value or the original placeholder.
func orUnchanged(value, placeholder string) string {
	if value == "" {
		return placeholder
	}

	return value
}

// splitSemverParts returns major, minor, and patch from a tag.
//
// Parameters:
//   - tag: Version tag, with or without a v prefix.
//
// Returns:
//   - string: Major component, or empty.
//   - string: Minor component, or empty.
//   - string: Patch component without prerelease suffix, or empty.
func splitSemverParts(tag string) (string, string, string) {
	canonical := canonicalizeSemver(tag)
	if !semver.IsValid(canonical) {
		return "", "", ""
	}

	trimmed := strings.TrimPrefix(semver.Canonical(canonical), "v")
	parts := strings.SplitN(trimmed, ".", semverPartCount)

	major, minor, patch := "", "", ""
	if len(parts) > 0 {
		major = parts[0]
	}

	if len(parts) > 1 {
		minor = parts[1]
	}

	if len(parts) > patchIndex {
		patch, _, _ = strings.Cut(parts[patchIndex], "-")
	}

	return major, minor, patch
}

// canonicalizeSemver returns a semver string golang.org/x/mod/semver accepts.
//
// Parameters:
//   - tag: Raw tag.
//
// Returns:
//   - string: Tag, optionally prefixed with v, or the original string if invalid.
func canonicalizeSemver(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}

	if semver.IsValid(tag) {
		return tag
	}

	if semver.IsValid("v" + tag) {
		return "v" + tag
	}

	return tag
}

// derivedReleasesURL builds a product-specific releases URL for a clone URL.
//
// When tag is a safe release tag, the URL points at that single release. An
// empty or unsafe tag yields the release index instead.
//
// Parameters:
//   - repo: Git clone URL.
//   - apiOrigin: Optional HTTP API base URL from the git-host label.
//   - tag: Exact release tag spelling, or empty for the release index.
//
// Returns:
//   - string: Releases URL, or empty for unknown hosts.
func derivedReleasesURL(repo, apiOrigin, tag string) string {
	host, owner, name, ok := parseRepo(repo)
	if !ok {
		return ""
	}

	cloneKind := types.ResolveGitHostKind(host, nil)
	kind := cloneKind
	base := "https://" + host

	if apiOrigin != "" {
		origin, err := ParseAPIOrigin(apiOrigin)
		if err != nil || !sameChangelogHost(host, origin) {
			return ""
		}

		if cloneKind != "" {
			base = strings.TrimRight(origin.Scheme+"://"+origin.Host, "/")
		} else {
			kind = changelogProvider(origin.Path, "")
			if kind == "" {
				return ""
			}

			base = strings.TrimRight(origin.Scheme+"://"+origin.Host+origin.Path, "/")
		}
	}

	if kind == "" {
		return ""
	}

	path := owner + "/" + name
	suffix := releaseTagSuffix(kind, tag)

	if suffix == "" {
		return ""
	}

	return base + "/" + path + suffix
}

// releaseTagSuffix returns the path suffix for a provider, pointing at one
// release when tag is safe.
//
// An empty result means the provider is unknown. A safe tag appends a versioned
// path. An empty or unsafe tag appends the release index so a malformed or
// hostile tag can never be spliced into a URL.
//
// Parameters:
//   - kind: Provider classification from the clone host or git-host label.
//   - tag: Exact release tag spelling, or empty for the release index.
//
// Returns:
//   - string: Path suffix beginning with a slash, or empty for an unknown provider.
func releaseTagSuffix(kind, tag string) string {
	switch kind {
	case types.GitHostGitHub, types.GitHostGitea:
		if safeReleaseTag(tag) {
			return "/releases/tag/" + tag
		}

		return "/releases"
	case types.GitHostGitLab:
		if safeReleaseTag(tag) {
			return "/-/releases/" + tag
		}

		return "/-/releases"
	default:
		return ""
	}
}

// safeReleaseTag reports whether a tag can be embedded in a release URL.
//
// Git permits slashes in tag names, so a hierarchical tag such as
// "release/1.2.3" stays usable. Empty, dot, and parent segments are rejected so
// a tag cannot alter the path structure of the derived URL. ASCII control
// characters, which Git forbids in ref names, are rejected so the derived URL
// always parses.
//
// Parameters:
//   - tag: Candidate release tag.
//
// Returns:
//   - bool: True when the tag is safe to embed.
func safeReleaseTag(tag string) bool {
	if tag == "" || strings.ContainsAny(tag, " ?#\\%") || strings.ContainsFunc(tag, isASCIIControl) {
		return false
	}

	for part := range strings.SplitSeq(tag, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}

	return true
}

// isASCIIControl reports whether r is an ASCII control character.
//
// Parameters:
//   - r: Rune to check.
//
// Returns:
//   - bool: True for U+0000 to U+001F and U+007F.
func isASCIIControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// sameChangelogHost reports whether a clone host and API origin share a hostname.
//
// Parameters:
//   - cloneHost: Hostname with an optional port from the clone URL.
//   - origin: Parsed API origin.
//
// Returns:
//   - bool: True when the hostnames match, ignoring ports.
func sameChangelogHost(cloneHost string, origin url.URL) bool {
	parsed, err := url.Parse("//" + cloneHost)
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Hostname(), origin.Hostname())
}

// changelogProvider identifies a provider from a documented API path prefix.
//
// Parameters:
//   - pathPrefix: API path prefix from git-host.
//   - known: Provider already identified from the clone hostname.
//
// Returns:
//   - string: GitHub, GitLab, Gitea, or empty.
func changelogProvider(pathPrefix, known string) string {
	if known != "" {
		return known
	}

	prefix := strings.ToLower(strings.Trim(pathPrefix, "/"))
	if prefix == "" || strings.Contains(prefix, "/") {
		return ""
	}

	switch prefix {
	case types.GitHostGitHub:
		return types.GitHostGitHub
	case types.GitHostGitLab:
		return types.GitHostGitLab
	case types.GitHostGitea, types.GitHostForgejo, "codeberg":
		return types.GitHostGitea
	default:
		return ""
	}
}

// parseRepo extracts host, owner, and repo name from a clone URL.
//
// Parameters:
//   - raw: HTTPS, SSH, or scp-like Git URL.
//
// Returns:
//   - string: Lowercase hostname.
//   - string: Owner or group path.
//   - string: Repository name without .git.
//   - bool: True when the URL has a usable owner and name.
func parseRepo(raw string) (string, string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", false
	}

	raw = strings.TrimSuffix(raw, ".git")

	// SCP-like remotes such as git@host:owner/name.git, with any user.
	if isSCPRemote(raw) {
		_, rest, _ := strings.Cut(raw, "@")
		hostPart, pathPart, _ := strings.Cut(rest, ":")

		return splitOwnerRepo(strings.ToLower(hostPart), pathPart)
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", "", "", false
	}

	return splitOwnerRepo(strings.ToLower(parsed.Host), strings.TrimPrefix(parsed.Path, "/"))
}

// isSCPRemote reports whether raw is an SCP-like remote with a user, such as
// deploy@host:owner/name.
//
// Like Git, it treats a URL without a scheme as SCP-like when a colon comes
// before the first slash. A non-empty user must come before that colon, and the
// host between them must not contain another @.
//
// Parameters:
//   - raw: Trimmed clone URL.
//
// Returns:
//   - bool: True when raw is user@host:path without a scheme.
func isSCPRemote(raw string) bool {
	if strings.Contains(raw, "://") {
		return false
	}

	atSign := strings.IndexByte(raw, '@')
	colon := strings.IndexByte(raw, ':')
	slash := strings.IndexByte(raw, '/')

	if atSign <= 0 || colon <= atSign || (slash >= 0 && colon > slash) {
		return false
	}

	return !strings.Contains(raw[atSign+1:colon], "@")
}

// splitOwnerRepo splits a Git path into owner and repository name.
//
// Nested groups keep slashes in owner (group/sub).
//
// Parameters:
//   - host: Hostname.
//   - path: Path after the host, with or without a leading slash.
//
// Returns:
//   - string: Host.
//   - string: Owner path.
//   - string: Repository name.
//   - bool: True when both owner and name are non-empty.
func splitOwnerRepo(host, path string) (string, string, string, bool) {
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")

	slash := strings.LastIndex(path, "/")
	if slash <= 0 || slash == len(path)-1 {
		return "", "", "", false
	}

	owner := path[:slash]
	name := path[slash+1:]

	if host == "" || !safeRepoPath(owner) || !safeRepoPath(name) {
		return "", "", "", false
	}

	return host, owner, name, true
}

// safeRepoPath reports whether a Git path segment can be used in a release URL.
//
// Parameters:
//   - value: Owner path or repository name.
//
// Returns:
//   - bool: False when value is empty or contains a "." or ".." segment.
func safeRepoPath(value string) bool {
	if value == "" {
		return false
	}

	for part := range strings.SplitSeq(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}

	return true
}
