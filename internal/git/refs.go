package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// validateRef reports whether ref is a hash or a safe branch or tag name.
//
// Parameters:
//   - ref: User-supplied branch, tag, or commit.
//
// Returns:
//   - error: errInvalidRef when the name is unsafe.
func validateRef(ref string) error {
	if ref == "" || plumbing.IsHash(ref) {
		return nil
	}

	name := refName(ref, kindBranch)
	if name.Validate() == nil && name.IsSafe() {
		return nil
	}

	name = refName(ref, kindTag)
	if name.Validate() == nil && name.IsSafe() {
		return nil
	}

	return fmt.Errorf("%w: %s", errInvalidRef, ref)
}

// resolveListedRef matches ref against ls-remote names, preferring peeled tags.
//
// Parameters:
//   - refs: Remote refs from List.
//   - ref: Requested branch or tag name.
//
// Returns:
//   - resolvedRef: Name, commit hash, and kind.
//   - error: errRefNotFound when neither a branch nor a tag matches.
func resolveListedRef(refs []RemoteRef, ref string) (resolvedRef, error) {
	branch := refName(ref, kindBranch)
	tag := refName(ref, kindTag)
	peeled := plumbing.ReferenceName(tag.String() + "^{}")

	for _, remote := range refs {
		if plumbing.ReferenceName(remote.Name) == branch {
			return resolvedRef{Name: ref, Hash: remote.Hash, Kind: kindBranch}, nil
		}
	}

	var tagHash string

	// Prefer the peeled commit hash over the annotated-tag object.
	for _, remote := range refs {
		name := plumbing.ReferenceName(remote.Name)
		if name == peeled {
			return resolvedRef{Name: ref, Hash: remote.Hash, Kind: kindTag}, nil
		}

		if name == tag {
			tagHash = remote.Hash
		}
	}

	if tagHash != "" {
		return resolvedRef{Name: ref, Hash: tagHash, Kind: kindTag}, nil
	}

	return resolvedRef{}, fmt.Errorf("%w: %s", errRefNotFound, ref)
}

// collectTagHashes maps short tag names to commit hashes.
//
// Peeled refs overwrite the annotated-tag object hash.
//
// Parameters:
//   - refs: Remote refs from List.
//
// Returns:
//   - map[string]string: Tag name to commit SHA.
func collectTagHashes(refs []RemoteRef) map[string]string {
	byName := make(map[string]string)

	for _, remote := range refs {
		raw := remote.Name
		peeled := strings.HasSuffix(raw, "^{}")
		raw = strings.TrimSuffix(raw, "^{}")

		name := plumbing.ReferenceName(raw)
		if !name.IsTag() {
			continue
		}

		short := name.Short()
		if peeled {
			byName[short] = remote.Hash

			continue
		}

		if _, exists := byName[short]; !exists {
			byName[short] = remote.Hash
		}
	}

	return byName
}

// refName builds a full reference name from a short or already-qualified ref.
//
// Parameters:
//   - ref: Short name or refs/... path.
//   - kind: kindBranch or kindTag.
//
// Returns:
//   - plumbing.ReferenceName: Qualified name.
func refName(ref, kind string) plumbing.ReferenceName {
	if strings.HasPrefix(ref, "refs/") {
		return plumbing.ReferenceName(ref)
	}

	if kind == kindTag {
		return plumbing.NewTagReferenceName(ref)
	}

	return plumbing.NewBranchReferenceName(ref)
}

// cloneReference returns the branch or tag name used for a shallow clone.
//
// Parameters:
//   - rev: Check result with Kind, Tag, and Ref.
//
// Returns:
//   - plumbing.ReferenceName: Empty when only a SHA is known.
func cloneReference(rev CheckResult) plumbing.ReferenceName {
	switch rev.Kind {
	case kindTag:
		name := rev.Tag
		if name == "" {
			name = rev.Ref
		}

		if name == "" {
			return ""
		}

		return refName(name, kindTag)
	case kindBranch:
		if rev.Ref == "" {
			return ""
		}

		return refName(rev.Ref, kindBranch)
	default:
		return ""
	}
}

// revisionTarget picks the string ResolveRevision should parse.
//
// Parameters:
//   - rev: Check result.
//
// Returns:
//   - string: Commit, then tag, then ref.
func revisionTarget(rev CheckResult) string {
	if rev.Commit != "" {
		return rev.Commit
	}

	if rev.Tag != "" {
		return rev.Tag
	}

	return rev.Ref
}
