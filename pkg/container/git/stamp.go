package git

import (
	"strings"
	"unicode"

	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// gitImageTagPrefix is the tag prefix Watchtower writes as name:git-<shortsha>.
const gitImageTagPrefix = "git-"

// minGitSHAPrefix is the shortest hex prefix treated as a Git object name.
const minGitSHAPrefix = 7

// LastStamp returns the last observed commit and tag from container labels.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - string: Last stamped commit SHA, or empty.
//   - string: Last stamped tag, or empty.
func LastStamp(c types.Container) (string, string) {
	if c == nil {
		return "", ""
	}

	return label(c, LastCommitLabel), label(c, LastTagLabel)
}

// Baseline returns the running revision to compare against a remote tip.
//
// Stamp labels win. When they are absent, the image's OCI revision/version
// or a name:git-<shortsha> tag is used. An empty result means Watchtower
// does not know what is running and must not rebuild only to write a stamp.
//
// Parameters:
//   - c: Container to inspect.
//
// Returns:
//   - string: Known commit SHA or short SHA, or empty.
//   - string: Known tag, or empty.
func Baseline(c types.Container) (string, string) {
	commit, tag := LastStamp(c)
	if commit != "" || tag != "" {
		return commit, tag
	}

	return imageRevision(c)
}

// imageRevision returns a commit and tag inferred from the running image.
//
// Parameters:
//   - c: Container whose image name and inspect labels are read.
//
// Returns:
//   - string: OCI revision or git-<shortsha> tag, or empty.
//   - string: OCI image version when it looks like a semver tag, or empty.
func imageRevision(c types.Container) (string, string) {
	if c == nil {
		return "", ""
	}

	ann := oci.Read(c)
	commit := strings.TrimSpace(ann.Revision)

	tag := ""
	if oci.LooksLikeTag(ann.Version) {
		tag = strings.TrimSpace(ann.Version)
	}

	if commit == "" {
		commit = gitSHAFromRef(c.ImageName())
	}

	if commit == "" && c.HasImageInfo() {
		if info := c.ImageInfo(); info != nil {
			for _, ref := range info.RepoTags {
				commit = gitSHAFromRef(ref)
				if commit != "" {
					break
				}
			}
		}
	}

	return commit, tag
}

// gitSHAFromRef returns the hex SHA from a name:git-<sha> image reference.
//
// Parameters:
//   - ref: Image name including tag.
//
// Returns:
//   - string: Hex SHA from a git- tag, or empty.
func gitSHAFromRef(ref string) string {
	tag := imageTag(ref)
	if !strings.HasPrefix(tag, gitImageTagPrefix) {
		return ""
	}

	sha := tag[len(gitImageTagPrefix):]
	if !isHexSHA(sha) || len(sha) < minGitSHAPrefix {
		return ""
	}

	return sha
}

// imageTag returns the tag of a name:tag reference. Host ports are ignored.
//
// Parameters:
//   - ref: Image name including tag.
//
// Returns:
//   - string: Tag, or empty when the reference has none.
func imageTag(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "@") {
		return ""
	}

	slash := strings.LastIndex(ref, "/")

	colon := strings.LastIndex(ref, ":")
	if colon <= slash {
		return ""
	}

	return ref[colon+1:]
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
