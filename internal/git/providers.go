package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"

	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	tagsPerPage      = 100
	giteaTagsPerPage = 50
	maxTagPages      = 50
	maxTagPeels      = 10
)

// resolveViaAPI resolves ref using the classified host's REST API.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - repo: Clone URL.
//   - ref: Branch or tag name.
//
// Returns:
//   - resolvedRef: Name, hash, and kind when found.
//   - bool: True when the API returned a hash.
//   - error: Non-nil on HTTP failure other than not found.
func (c *Client) resolveViaAPI(ctx context.Context, repo, ref, apiOrigin string) (resolvedRef, bool, error) {
	host, owner, name, ok := splitRepo(repo)
	if !ok {
		return resolvedRef{}, false, nil
	}

	kind, origin, ok := c.lookupAPI(ctx, host, apiOrigin)
	if !ok {
		return resolvedRef{}, false, nil
	}

	switch kind {
	case types.GitHostGitHub:
		resolved, found, err := c.githubRef(ctx, host, origin, owner, name, "heads/"+ref)
		if err != nil || found {
			if found {
				resolved.Kind = kindBranch
			}

			return resolved, found, err
		}

		resolved, found, err = c.githubRef(ctx, host, origin, owner, name, "tags/"+ref)
		if found {
			resolved.Kind = kindTag
		}

		return resolved, found, err
	case types.GitHostGitLab:
		return c.gitlabRef(ctx, host, origin, owner, name, ref)
	case types.GitHostGitea:
		return c.giteaRef(ctx, host, origin, owner, name, ref)
	default:
		return resolvedRef{}, false, nil
	}
}

// listTagsViaAPI lists tags using the classified host's REST API.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - repo: Clone URL.
//
// Returns:
//   - []RemoteRef: Tags when the host is classified.
//   - bool: True when the API was used.
//   - error: Non-nil on HTTP failure.
func (c *Client) listTagsViaAPI(ctx context.Context, repo, apiOrigin string) ([]RemoteRef, bool, error) {
	host, owner, name, ok := splitRepo(repo)
	if !ok {
		return nil, false, nil
	}

	kind, origin, ok := c.lookupAPI(ctx, host, apiOrigin)
	if !ok {
		return nil, false, nil
	}

	switch kind {
	case types.GitHostGitHub:
		return c.githubTags(ctx, host, origin, owner, name)
	case types.GitHostGitLab:
		return c.gitlabTags(ctx, host, origin, owner, name)
	case types.GitHostGitea:
		return c.giteaTags(ctx, host, origin, owner, name)
	default:
		return nil, false, nil
	}
}

// lookupAPI returns the HTTP API kind and origin for a clone host.
//
// A git-host label supplies the API origin. Without it, only github.com,
// gitlab.com, and codeberg.org use an HTTP API.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - cloneHost: Hostname from the clone URL.
//   - apiOrigin: Optional HTTP API base URL from the git-host label.
//
// Returns:
//   - string: GitHub, GitLab, or Gitea kind.
//   - url.URL: API origin.
//   - bool: True when an HTTP API should be used.
func (c *Client) lookupAPI(ctx context.Context, cloneHost, apiOrigin string) (string, url.URL, bool) {
	if apiOrigin != "" {
		parsed, err := gitPkg.ParseAPIOrigin(apiOrigin)
		if err != nil {
			return "", url.URL{}, false
		}

		// A label may select another port or path. A different hostname
		// would receive the process token, so ignore it and use ls-remote.
		if toHostname(parsed.Hostname()) != toHostname(cloneHost) {
			return "", url.URL{}, false
		}

		kind := types.ResolveGitHostKind(parsed.Hostname(), c.opts.Hosts)
		if kind == "" {
			kind = c.detectKind(ctx, parsed, cloneHost)
		}

		if kind == "" {
			return "", url.URL{}, false
		}

		return kind, parsed, true
	}

	kind := types.ResolveGitHostKind(cloneHost, nil)
	if kind == "" {
		return "", url.URL{}, false
	}

	return kind, c.originOf(cloneHost), true
}

// splitRepo parses a clone URL into host, owner, and repository name.
//
// Parameters:
//   - repo: HTTPS, SSH, or scp-like Git URL.
//
// Returns:
//   - string: Hostname.
//   - string: Owner or group path.
//   - string: Repository name.
//   - bool: True when the URL is usable.
func splitRepo(repo string) (string, string, string, bool) {
	endpoint, err := transport.NewEndpoint(repo)
	if err != nil || endpoint.Host == "" {
		return "", "", "", false
	}

	return splitOwnerRepo(toHostname(endpoint.Host), endpoint.Path)
}

// splitOwnerRepo splits a Git path into owner and repository name.
//
// Parameters:
//   - host: Hostname.
//   - path: Path after the host.
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

	if owner == "" || name == "" {
		return "", "", "", false
	}

	return host, owner, name, true
}

// githubRef resolves a GitHub git/ref path such as heads/main or tags/v1.0.0.
//
// Annotated tags are peeled to the commit SHA.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: github.com or GitHub Enterprise hostname.
//   - owner: Repository owner.
//   - repo: Repository name.
//   - ref: Path after git/ref.
//
// Returns:
//   - resolvedRef: Short name and commit hash.
//   - bool: True when the ref exists.
//   - error: Non-nil on HTTP failure.
func (c *Client) githubRef(ctx context.Context, host string, origin url.URL, owner, repo, ref string) (resolvedRef, bool, error) {
	endpoint := c.githubAPIAt(origin, host, "repos", owner, repo, "git", "ref", ref)

	var body struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"object"`
	}

	_, err := c.getJSONPage(ctx, endpoint.String(), &body, host)
	if errors.Is(err, errAPINotFound) {
		return resolvedRef{}, false, nil
	}

	if err != nil {
		return resolvedRef{}, false, err
	}

	object := body.Object
	sha := object.SHA
	// Annotated tags can target another tag object. Peel until the API
	// identifies the target as a commit, and reject trees and blobs.
	for range maxTagPeels {
		if object.Type != "tag" {
			break
		}

		if object.URL == "" || sameOriginNext(endpoint.String(), object.URL) == "" {
			return resolvedRef{}, false, nil
		}

		var tag struct {
			Object struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"object"`
		}

		_, err := c.getJSONPage(ctx, object.URL, &tag, host)
		if err != nil {
			//nolint:nilerr // Peel failure is not-found so ls-remote can try ^{}.
			return resolvedRef{}, false, nil
		}

		object = tag.Object

		sha = object.SHA
		if sha == "" {
			return resolvedRef{}, false, nil
		}
	}

	if object.Type != "commit" {
		return resolvedRef{}, false, nil
	}

	name := ref
	if after, ok := strings.CutPrefix(ref, "heads/"); ok {
		name = after
	} else if after, ok := strings.CutPrefix(ref, "tags/"); ok {
		name = after
	}

	return resolvedRef{Name: name, Hash: sha}, true, nil
}

// githubTags lists GitHub tags with pagination.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: GitHub hostname.
//   - owner: Repository owner.
//   - repo: Repository name.
//
// Returns:
//   - []RemoteRef: Tag names and commit hashes.
//   - bool: True when the API was used.
//   - error: Non-nil on HTTP failure.
func (c *Client) githubTags(ctx context.Context, host string, origin url.URL, owner, repo string) ([]RemoteRef, bool, error) {
	endpointURL := c.githubAPIAt(origin, host, "repos", owner, repo, "tags")
	query := endpointURL.Query()
	query.Set("per_page", strconv.Itoa(tagsPerPage))
	endpointURL.RawQuery = query.Encode()
	endpoint := endpointURL.String()

	type tagPage []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	var tags []RemoteRef

	for range maxTagPages {
		var body tagPage

		next, err := c.getJSONPage(ctx, endpoint, &body, host)
		if err != nil {
			return nil, false, err
		}

		for _, tag := range body {
			tags = append(tags, RemoteRef{Name: tag.Name, Hash: tag.Commit.SHA})
		}

		if next == "" {
			break
		}

		endpoint = next
	}

	return tags, true, nil
}

// gitlabRef resolves a GitLab branch, then tag, via repository metadata.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: GitLab hostname.
//   - owner: Group path.
//   - repo: Project name.
//   - ref: Branch or tag name.
//
// Returns:
//   - resolvedRef: Name, hash, and kind.
//   - bool: True when the ref exists.
//   - error: Non-nil on HTTP failure.
func (c *Client) gitlabRef(ctx context.Context, host string, origin url.URL, owner, repo, ref string) (resolvedRef, bool, error) {
	refKinds := [...]struct {
		resource string
		kind     string
	}{
		{resource: "branches", kind: kindBranch},
		{resource: "tags", kind: kindTag},
	}

	for _, refKind := range refKinds {
		endpoint := joinEscaped(
			c.gitlabAPIAt(origin, host),
			"projects",
			owner+"/"+repo,
			"repository",
			refKind.resource,
			ref,
		)

		var body struct {
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
		}

		_, err := c.getJSONPage(ctx, endpoint.String(), &body, host)
		if errors.Is(err, errAPINotFound) {
			continue
		}

		if err != nil {
			return resolvedRef{}, false, err
		}

		if body.Commit.ID == "" {
			continue
		}

		return resolvedRef{Name: ref, Hash: body.Commit.ID, Kind: refKind.kind}, true, nil
	}

	return resolvedRef{}, false, nil
}

// gitlabTags lists GitLab tags with pagination.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: GitLab hostname.
//   - owner: Group path.
//   - repo: Project name.
//
// Returns:
//   - []RemoteRef: Tag names and commit hashes.
//   - bool: True when the API was used.
//   - error: Non-nil on HTTP failure.
func (c *Client) gitlabTags(ctx context.Context, host string, origin url.URL, owner, repo string) ([]RemoteRef, bool, error) {
	endpointURL := joinEscaped(c.gitlabAPIAt(origin, host), "projects", owner+"/"+repo, "repository", "tags")
	query := endpointURL.Query()
	query.Set("per_page", strconv.Itoa(tagsPerPage))
	endpointURL.RawQuery = query.Encode()
	endpoint := endpointURL.String()

	type tagPage []struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}

	var tags []RemoteRef

	for range maxTagPages {
		var body tagPage

		next, err := c.getJSONPage(ctx, endpoint, &body, host)
		if err != nil {
			return nil, false, err
		}

		for _, tag := range body {
			tags = append(tags, RemoteRef{Name: tag.Name, Hash: tag.Commit.ID})
		}

		if next == "" {
			break
		}

		endpoint = next
	}

	return tags, true, nil
}

// giteaRef resolves a Gitea or Forgejo branch, then tag.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: Gitea hostname.
//   - owner: Owner.
//   - repo: Repository name.
//   - ref: Branch or tag name.
//
// Returns:
//   - resolvedRef: Name, hash, and kind.
//   - bool: True when the ref exists.
//   - error: Non-nil on HTTP failure.
func (c *Client) giteaRef(ctx context.Context, host string, origin url.URL, owner, repo, ref string) (resolvedRef, bool, error) {
	endpoint := c.giteaAPIAt(origin, host, "repos", owner, repo, "git", "refs", "heads", ref)

	resolved, found, err := c.giteaRefs(ctx, endpoint.String(), ref, kindBranch, host)
	if err != nil || found {
		return resolved, found, err
	}

	endpoint = c.giteaAPIAt(origin, host, "repos", owner, repo, "git", "refs", "tags", ref)

	return c.giteaRefs(ctx, endpoint.String(), ref, kindTag, host)
}

// giteaRefs decodes a Gitea git/refs response.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - endpoint: Full refs URL.
//   - ref: Short name to record.
//   - kind: kindBranch or kindTag.
//
// Returns:
//   - resolvedRef: Name, hash, and kind.
//   - bool: True when a SHA is present.
//   - error: Non-nil on HTTP failure.
func (c *Client) giteaRefs(ctx context.Context, endpoint, ref, kind, cloneHost string) (resolvedRef, bool, error) {
	var body []struct {
		Ref    string `json:"ref"`
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}

	_, err := c.getJSONPage(ctx, endpoint, &body, cloneHost)
	if errors.Is(err, errAPINotFound) {
		return resolvedRef{}, false, nil
	}

	if err != nil {
		return resolvedRef{}, false, err
	}

	var (
		commitHash string
		peeledHash string
	)

	for _, item := range body {
		if item.Object.SHA == "" {
			continue
		}

		if strings.HasSuffix(item.Ref, "^{}") {
			if item.Object.Type != "commit" {
				return resolvedRef{}, false, nil
			}

			peeledHash = item.Object.SHA

			continue
		}

		if item.Object.Type == "commit" && commitHash == "" {
			commitHash = item.Object.SHA
		}
	}

	if peeledHash != "" {
		return resolvedRef{Name: ref, Hash: peeledHash, Kind: kind}, true, nil
	}

	if commitHash == "" {
		return resolvedRef{}, false, nil
	}

	return resolvedRef{Name: ref, Hash: commitHash, Kind: kind}, true, nil
}

// giteaTags lists Gitea or Forgejo tags with page/limit pagination.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - host: Gitea hostname.
//   - owner: Owner.
//   - repo: Repository name.
//
// Returns:
//   - []RemoteRef: Tag names and commit hashes.
//   - bool: True when the API was used.
//   - error: Non-nil on HTTP failure.
func (c *Client) giteaTags(ctx context.Context, host string, origin url.URL, owner, repo string) ([]RemoteRef, bool, error) {
	type tagPage []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	var tags []RemoteRef

	endpointURL := c.giteaAPIAt(origin, host, "repos", owner, repo, "tags")
	query := endpointURL.Query()
	query.Set("limit", strconv.Itoa(giteaTagsPerPage))
	query.Set("page", "1")
	endpointURL.RawQuery = query.Encode()
	endpoint := endpointURL.String()

	for page := 1; page <= maxTagPages; page++ {
		var body tagPage

		next, err := c.getJSONPage(ctx, endpoint, &body, host)
		if err != nil {
			return nil, false, err
		}

		if len(body) == 0 {
			break
		}

		for _, tag := range body {
			tags = append(tags, RemoteRef{Name: tag.Name, Hash: tag.Commit.SHA})
		}

		if next == "" {
			if len(body) < giteaTagsPerPage {
				break
			}

			endpointURL = c.giteaAPIAt(origin, host, "repos", owner, repo, "tags")
			query = endpointURL.Query()
			query.Set("limit", strconv.Itoa(giteaTagsPerPage))
			query.Set("page", strconv.Itoa(page+1))
			endpointURL.RawQuery = query.Encode()
			endpoint = endpointURL.String()

			continue
		}

		nextURL, err := url.Parse(next)
		if err != nil {
			return nil, false, fmt.Errorf("parse Gitea pagination URL: %w", err)
		}

		nextQuery := nextURL.Query()
		nextQuery.Set("limit", strconv.Itoa(giteaTagsPerPage))
		nextURL.RawQuery = nextQuery.Encode()
		endpoint = nextURL.String()
	}

	return tags, true, nil
}

// getJSONPage GET decodes endpoint into dest and returns the next Link URL.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - endpoint: Absolute URL.
//   - dest: JSON destination.
//   - cloneHost: Clone URL hostname. Credentials are sent only for this host.
//
// Returns:
//   - string: Next page URL, or empty.
//   - error: Non-nil on HTTP or decode failure.
func (c *Client) getJSONPage(ctx context.Context, endpoint string, dest any, cloneHost string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.applyAuth(req, cloneHost)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)

		return "", errAPINotFound
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)

		return "", fmt.Errorf("%w: %s %s", errAPIStatus, resp.Status, endpoint)
	}

	decodeErr := json.NewDecoder(resp.Body).Decode(dest)
	if decodeErr != nil {
		return "", fmt.Errorf("decode json: %w", decodeErr)
	}

	return sameOriginNext(endpoint, nextLink(resp.Header.Get("Link"))), nil
}

// sameOriginNext returns next only when it shares scheme and host with current.
//
// Pagination Link headers are attacker-controlled on a compromised Git host.
// Following a different origin would send Authorization to that host.
//
// Parameters:
//   - current: URL that was just fetched.
//   - next: rel=next value from the Link header.
//
// Returns:
//   - string: next when same origin, or empty.
func sameOriginNext(current, next string) string {
	if next == "" {
		return ""
	}

	cur, err := url.Parse(current)
	if err != nil || cur.Host == "" {
		return ""
	}

	nxt, err := url.Parse(next)
	if err != nil || nxt.Host == "" {
		return ""
	}

	if nxt.Scheme != "http" && nxt.Scheme != "https" {
		return ""
	}

	if !strings.EqualFold(cur.Scheme, nxt.Scheme) || !strings.EqualFold(cur.Host, nxt.Host) {
		return ""
	}

	return next
}

// nextLink extracts the rel=next URL from a Link header.
//
// Parameters:
//   - header: Raw Link header.
//
// Returns:
//   - string: Next URL, or empty.
func nextLink(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) && !strings.Contains(part, "rel=next") {
			continue
		}

		start := strings.Index(part, "<")

		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}

	return ""
}
