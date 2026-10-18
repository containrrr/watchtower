// Package git checks whether the Git source associated with a container has
// changed.
//
// Labels or Git image mappings associate a container with a repository. The
// check compares the remote branch tip, or the newest tag the container's
// semver policy allows, with the revision the container runs, and reports
// whether the remote has advanced. Refs are resolved through the GitHub,
// GitLab, or Gitea API when the host is known, and through git ls-remote
// otherwise. The package also clones a repository at a resolved revision for a
// rebuild.
package git
