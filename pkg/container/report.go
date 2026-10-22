package container

import (
	"net/url"
	"strings"

	"github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// ReportMeta holds Git and OCI fields exposed on session reports.
//
// Project-scoped fields resolve from the latest known image so a notification
// describes what the container is moving to.
type ReportMeta struct {
	GitRepo         string
	GitRef          string
	Changelog       string
	Source          string
	ImageURL        string
	Documentation   string
	CurrentVersion  string
	LatestVersion   string
	CurrentRevision string
	LatestRevision  string
}

// ChangelogVars substitutes placeholders in an explicit changelog template.
type ChangelogVars = git.ChangelogVars

// ResolveReportMeta builds notification fields from Git association and OCI annotations.
//
// Watcher association is not inferred from OCI source. The latest annotations
// describe a newly pulled image. An empty latest falls back to the running
// image, so callers that know of no update see the current image's metadata.
//
// Parameters:
//   - c: Container to inspect.
//   - params: Update parameters with image mappings and host classifications.
//   - vars: Optional new-version values for changelog placeholders.
//   - latest: OCI annotations from a newly pulled image, or empty for none.
//
// Returns:
//   - ReportMeta: Resolved metadata. Empty strings when unknown.
func ResolveReportMeta(
	c types.Container,
	params types.UpdateParams,
	vars ChangelogVars,
	latest oci.Annotations,
) ReportMeta {
	if c == nil {
		return ReportMeta{}
	}

	cur := oci.Read(c)
	anns := preferAnnotations(latest, cur)
	mapping := types.GitImage{}

	if params.GitImages != nil {
		mapping = params.GitImages[c.ImageName()]
	}

	assoc, associated := git.ResolveAssociation(c, params)

	// Report repo may fall back to OCI source. That does not enable the watcher.
	repo := redactGitURL(assoc.Repo)
	if repo == "" {
		repo = redactGitURL(mapping.Repo)
	}

	if repo == "" {
		repo = redactMetadataURL(anns.Source)
	}

	// The ref is the configured Git ref only. An OCI version is reported
	// separately, so a version is never mistaken for a ref.
	ref := ""
	if associated {
		ref = assoc.Ref
	} else if mapping.Ref != "" {
		ref = mapping.Ref
	}

	changelog := redactMetadataURL(git.Changelog(c, repo, vars))
	if changelog == "" {
		changelog = redactMetadataURL(anns.URL)
	}

	if changelog == "" {
		changelog = redactMetadataURL(anns.Documentation)
	}

	return ReportMeta{
		GitRepo:         repo,
		GitRef:          ref,
		Changelog:       changelog,
		Source:          redactMetadataURL(anns.Source),
		ImageURL:        redactMetadataURL(anns.URL),
		Documentation:   redactMetadataURL(anns.Documentation),
		CurrentVersion:  cur.Version,
		LatestVersion:   latest.Version,
		CurrentRevision: cur.Revision,
		LatestRevision:  latest.Revision,
	}
}

// preferAnnotations returns the latest image annotations, falling back to the
// current image for each field the newer image does not carry.
//
// The two images are compared per field because a maintained image may set a
// new version while a rebuilt one drops an annotation the old one still had.
//
// Parameters:
//   - latest: OCI annotations from a newly pulled image, possibly empty.
//   - cur: OCI annotations from the running image.
//
// Returns:
//   - oci.Annotations: Per-field newest known values.
func preferAnnotations(latest, cur oci.Annotations) oci.Annotations {
	return oci.Annotations{
		Source:        firstNonEmpty(latest.Source, cur.Source),
		URL:           firstNonEmpty(latest.URL, cur.URL),
		Documentation: firstNonEmpty(latest.Documentation, cur.Documentation),
		Revision:      firstNonEmpty(latest.Revision, cur.Revision),
		Version:       firstNonEmpty(latest.Version, cur.Version),
	}
}

// firstNonEmpty returns value when it is non-empty, otherwise fallback.
//
// Parameters:
//   - value: Preferred value.
//   - fallback: Value used when value is empty.
//
// Returns:
//   - string: First non-empty argument.
func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}

	return fallback
}

// redactGitURL removes credentials and sensitive query values from a Git URL.
//
// The host, port, path, non-sensitive query, and fragment remain available for identity. An
// unparseable URL is rejected so metadata cannot accidentally echo a secret.
//
// Parameters:
//   - raw: Git clone URL.
//
// Returns:
//   - string: Safe URL, or empty when it cannot be parsed.
func redactGitURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "://") {
		if isNativeRepositoryPath(raw) {
			return raw
		}

		if strings.Contains(raw, "@") && strings.Contains(raw, ":") {
			return redactSCPGitURL(raw)
		}

		return redactUnqualifiedPath(raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Host == "" && parsed.Scheme != "file") {
		return ""
	}

	if parsed.User != nil {
		if parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "file" {
			parsed.User = nil
		} else if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.User(parsed.User.Username())
		}
	}

	parsed.RawQuery = redactQuery(parsed.RawQuery)
	parsed.ForceQuery = parsed.RawQuery != ""

	return parsed.String()
}

// isNativeRepositoryPath reports whether raw is an explicit local filesystem path.
//
// Parameters:
//   - raw: Repository value.
//
// Returns:
//   - bool: True for absolute, explicitly relative, home-relative, or Windows paths.
func isNativeRepositoryPath(raw string) bool {
	return strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, "./") ||
		strings.HasPrefix(raw, "../") ||
		strings.HasPrefix(raw, "~/") ||
		strings.HasPrefix(raw, `C:\`) ||
		strings.HasPrefix(raw, "C:/") ||
		strings.HasPrefix(raw, `\\`) ||
		raw == "." ||
		raw == ".."
}

// redactUnqualifiedPath removes sensitive query values from a local or bare repository path.
//
// Parameters:
//   - raw: Local path or bare repository identifier.
//
// Returns:
//   - string: Path with non-sensitive query data preserved.
func redactUnqualifiedPath(raw string) string {
	base, fragment, hasFragment := strings.Cut(raw, "#")

	path, query, hasQuery := strings.Cut(base, "?")
	if !hasQuery {
		return raw
	}

	sanitizedQuery := redactQuery(query)
	if sanitizedQuery != "" {
		path += "?" + sanitizedQuery
	}

	if hasFragment {
		path += "#" + fragment
	}

	return path
}

// redactQuery removes credential-like query parameters while preserving benign routing data.
//
// Parameters:
//   - raw: URL query string without the leading question mark.
//
// Returns:
//   - string: Sanitized encoded query string.
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, "&")
	kept := make([]string, 0, len(parts))

	for _, part := range parts {
		keyPart, _, _ := strings.Cut(part, "=")

		key, err := url.QueryUnescape(keyPart)
		if err != nil {
			continue
		}

		if isSensitiveQueryKey(key) {
			continue
		}

		kept = append(kept, part)
	}

	return strings.Join(kept, "&")
}

// isSensitiveQueryKey reports whether a query parameter commonly contains credentials.
//
// Parameters:
//   - key: URL query parameter name.
//
// Returns:
//   - bool: True when the parameter should be removed.
func isSensitiveQueryKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})

	for _, part := range parts {
		switch part {
		case "token", "password", "passwd", "secret", "credential", "credentials", "signature", "sig", "auth", "authorization", "session", "jwt", "bearer":
			return true
		}
	}

	compact := strings.NewReplacer("_", "", "-", "", ".", "").Replace(normalized)

	return strings.Contains(compact, "oauth") || strings.Contains(compact, "apikey") || strings.Contains(compact, "accesskey") || strings.Contains(compact, "privatekey") ||
		strings.Contains(compact, "authtoken")
}

// redactSCPGitURL removes credentials and sensitive query values from an SCP-style Git URL.
//
// Parameters:
//   - raw: SCP-like Git URL.
//
// Returns:
//   - string: Safe URL, or empty when it is malformed.
func redactSCPGitURL(raw string) string {
	safe := redactUnqualifiedPath(raw)
	withoutFragment, fragment, hasFragment := strings.Cut(safe, "#")
	withoutQuery, query, hasQuery := strings.Cut(withoutFragment, "?")

	user, remote, found := strings.Cut(withoutQuery, "@")
	if !found || user == "" || scpUserIsCredential(user) {
		return ""
	}

	separator := strings.LastIndexByte(remote, ':')
	if separator <= 0 || separator == len(remote)-1 {
		return ""
	}

	host := remote[:separator]
	path := remote[separator+1:]

	parsedHost, err := url.Parse("//" + host)
	if err != nil || parsedHost.Host == "" || parsedHost.User != nil || parsedHost.Path != "" {
		return ""
	}

	sanitized := user + "@" + host + ":" + path
	if hasQuery && query != "" {
		sanitized += "?" + query
	}

	if hasFragment {
		sanitized += "#" + fragment
	}

	return sanitized
}

// scpUserIsCredential reports whether an SCP user is a secret rather than an account name.
//
// git and other short account names are kept. A token used as the user is not
// stored on a replacement container.
//
// Parameters:
//   - user: User component of an SCP URL.
//
// Returns:
//   - bool: True when the user must not be persisted.
func scpUserIsCredential(user string) bool {
	if user == "" || strings.Contains(user, ":") {
		return true
	}

	lower := strings.ToLower(user)
	for _, marker := range []string{"token", "secret", "password", "passwd", "credential"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	for _, prefix := range []string{
		"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "glpat-", "gldt-", "glrt-",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	if len(user) >= 20 && isHexString(user) {
		return true
	}

	return false
}

// isHexString reports whether value contains only hexadecimal characters.
//
// Parameters:
//   - value: Candidate token.
//
// Returns:
//   - bool: True when every character is hexadecimal.
func isHexString(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}

	return true
}

// redactMetadataURL sanitizes URL-like report metadata while preserving plain
// annotation values.
//
// Parameters:
//   - raw: Metadata value.
//
// Returns:
//   - string: Sanitized URL, or the original plain value.
func redactMetadataURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") || strings.HasPrefix(value, "git@") || strings.Contains(value, "?") {
		return redactGitURL(value)
	}

	authority, _, _ := strings.Cut(value, "/")
	if strings.Contains(authority, "@") {
		return redactGitURL(value)
	}

	return value
}
