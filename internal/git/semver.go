package git

import (
	"strings"

	"golang.org/x/mod/semver"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// selectTag returns the highest policy-allowed tag newer than lastTag.
//
// Non-semver tags and pre-releases are ignored. Build metadata does not
// affect ordering. When none remain, ok is false (not stale).
//
// Parameters:
//   - lastTag: Last observed tag (empty selects the highest allowed tag).
//   - tags: Remote tag names.
//   - policy: none/patch/minor/major.
//
// Returns:
//   - string: Selected tag name as advertised by the remote.
//   - bool: True when a newer allowed tag exists.
func selectTag(lastTag string, tags []string, policy string) (string, bool) {
	if policy == "" || policy == types.GitPolicyNone {
		return "", false
	}

	last := canonicalize(lastTag)
	if last != "" && !semver.IsValid(last) {
		last = ""
	}

	bestName := ""
	bestCanon := ""

	for _, tag := range tags {
		canon := canonicalize(tag)
		if !semver.IsValid(canon) || semver.Prerelease(canon) != "" {
			continue
		}

		if last != "" && !allowedBump(last, canon, policy) {
			continue
		}

		if last != "" && semver.Compare(canon, last) <= 0 {
			continue
		}

		if bestCanon == "" || semver.Compare(canon, bestCanon) > 0 {
			bestCanon = canon
			bestName = tag
		}
	}

	if bestName == "" {
		return "", false
	}

	return bestName, true
}

// allowedBump reports whether candidate is a newer release allowed by policy.
//
// Parameters:
//   - current: Canonical last tag.
//   - candidate: Canonical candidate tag.
//   - policy: patch, minor, or major.
//
// Returns:
//   - bool: True when candidate is newer and within the policy window.
func allowedBump(current, candidate, policy string) bool {
	if semver.Compare(candidate, current) <= 0 {
		return false
	}

	switch policy {
	case types.GitPolicyPatch:
		return semver.MajorMinor(current) == semver.MajorMinor(candidate)
	case types.GitPolicyMinor:
		return semver.Major(current) == semver.Major(candidate)
	case types.GitPolicyMajor:
		return true
	default:
		return false
	}
}

// canonicalize returns a semver string golang.org/x/mod/semver accepts.
//
// Parameters:
//   - tag: Raw tag name.
//
// Returns:
//   - string: Tag, optionally prefixed with v, or the original string if invalid.
func canonicalize(tag string) string {
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

// baselineTag returns lastTag when it is a semver tag, including pre-releases.
//
// Non-semver stamps such as nightly are not a tag baseline.
//
// Parameters:
//   - lastTag: Last observed tag.
//
// Returns:
//   - string: Trimmed lastTag, or empty when it is not semver.
func baselineTag(lastTag string) string {
	lastTag = strings.TrimSpace(lastTag)
	if !semver.IsValid(canonicalize(lastTag)) {
		return ""
	}

	return lastTag
}
