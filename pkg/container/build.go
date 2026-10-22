package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var errImageBuildFailed = errors.New("image build failed")

// BuildRemoteImage builds an image from a Git URL context.
//
// The Docker daemon clones the repository. Watchtower does not.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - remote: Docker Git context URL.
//   - dockerfile: Dockerfile path relative to that context. Empty means Dockerfile.
//   - tags: Image tags to apply.
//
// Returns:
//   - types.ImageID: Built image ID.
//   - error: Non-nil when the build or inspect fails.
func (c *client) BuildRemoteImage(ctx context.Context, remote, dockerfile string, tags []string) (types.ImageID, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", errGitRemoteEmpty
	}

	dockerfile, err := confinedDockerfile(dockerfile)
	if err != nil {
		return "", err
	}

	clogVal := c.logger().With().
		Str("remote", redactedRemote(remote)).
		Str("dockerfile", dockerfile).
		Strs("tags", tags).
		Logger()
	clog := &clogVal

	result, err := c.api.ImageBuild(ctx, http.NoBody, dockerClient.ImageBuildOptions{
		RemoteContext: remote,
		Tags:          tags,
		Remove:        true,
		Dockerfile:    dockerfile,
	})
	if err != nil {
		return "", fmt.Errorf("image build: %w", redactBuildError(err, remote))
	}
	defer result.Body.Close()

	imageID, err := consumeBuildStream(result.Body)
	if err != nil {
		return "", redactBuildError(err, remote)
	}

	if imageID == "" && len(tags) > 0 {
		inspected, inspectErr := c.api.ImageInspect(ctx, tags[0])
		if inspectErr != nil {
			return "", fmt.Errorf("inspect built image: %w", inspectErr)
		}

		imageID = types.ImageID(inspected.ID)
	}

	if imageID == "" {
		return "", errBuildImageIDMissing
	}

	clog.Debug().
		Str("image_id", imageID.ShortID()).
		Msg("Built image from Git URL context")

	return imageID, nil
}

// redactedRemote replaces URL userinfo for logs.
//
// Parameters:
//   - remote: Git context URL that may contain credentials.
//
// Returns:
//   - string: Same URL with userinfo replaced by xxxxx.
func redactedRemote(remote string) string {
	parsed, err := url.Parse(remote)
	if err != nil || parsed.User == nil {
		return remote
	}

	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword("xxxxx", "xxxxx")
	} else {
		parsed.User = url.User("xxxxx")
	}

	return parsed.String()
}

// userinfoInError matches scheme://user@ and scheme://user:password@ in a daemon error.
var userinfoInError = regexp.MustCompile(`://[^/\s@]+@`)

// redactBuildError removes Git credentials from a build error.
//
// The Docker daemon receives the credentialed remote and may echo it.
// The original error is returned unchanged when it contains no userinfo.
//
// Parameters:
//   - err: Build client or stream error.
//   - remote: Credentialed Git context URL sent to the daemon.
//
// Returns:
//   - error: err, or a new error with userinfo replaced.
func redactBuildError(err error, remote string) error {
	if err == nil {
		return nil
	}

	msg := redactUserinfo(err.Error(), remote)
	if msg == err.Error() {
		return err
	}

	return fmt.Errorf("%w: %s", errImageBuildFailed, msg)
}

// redactUserinfo replaces the credentialed remote and any leftover userinfo.
//
// Parameters:
//   - msg: Error text.
//   - remote: Credentialed Git context URL.
//
// Returns:
//   - string: msg with credentials replaced by xxxxx.
func redactUserinfo(msg, remote string) string {
	if remote != "" {
		msg = strings.ReplaceAll(msg, remote, redactedRemote(remote))
	}

	return userinfoInError.ReplaceAllString(msg, "://xxxxx:xxxxx@")
}

// confinedDockerfile rejects Dockerfile paths that leave the build context.
//
// Parameters:
//   - dockerfile: Path relative to the context. Empty means Dockerfile.
//
// Returns:
//   - string: Clean slash-separated relative path.
//   - error: Non-nil when the path is absolute or walks above the context.
func confinedDockerfile(dockerfile string) (string, error) {
	dockerfile = strings.TrimSpace(dockerfile)
	if dockerfile == "" {
		return DefaultGitDockerfile, nil
	}

	cleaned := path.Clean(filepath.ToSlash(dockerfile))
	if path.IsAbs(cleaned) || !filepath.IsLocal(filepath.FromSlash(cleaned)) {
		return "", fmt.Errorf("%w: %s", errGitDockerfileEscape, dockerfile)
	}

	return cleaned, nil
}

// consumeBuildStream reads a Docker build stream and returns the last image ID.
//
// Parameters:
//   - body: Build response body.
//
// Returns:
//   - types.ImageID: Last aux ID, or empty.
//   - error: Non-nil when the stream reports a build error.
func consumeBuildStream(body io.Reader) (types.ImageID, error) {
	decoder := json.NewDecoder(body)

	var lastID string

	for {
		var line struct {
			Stream string          `json:"stream"`
			Error  string          `json:"error"`
			Aux    json.RawMessage `json:"aux"`
		}

		err := decoder.Decode(&line)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return "", fmt.Errorf("read build stream: %w", err)
		}

		if line.Error != "" {
			return "", fmt.Errorf("%w: %s", errImageBuildFailed, line.Error)
		}

		if id := auxImageID(line.Aux); id != "" {
			lastID = id
		}
	}

	return types.ImageID(lastID), nil
}

// auxImageID returns the image ID from a Docker build aux payload.
//
// BuildKit may send a string trace instead of an object. Those are ignored.
//
// Parameters:
//   - aux: Raw aux JSON value.
//
// Returns:
//   - string: Image ID, or empty.
func auxImageID(aux json.RawMessage) string {
	aux = bytes.TrimSpace(aux)
	if len(aux) == 0 || aux[0] != '{' {
		return ""
	}

	var payload struct {
		ID string `json:"ID"`
	}

	err := json.Unmarshal(aux, &payload)
	if err != nil {
		return ""
	}

	return payload.ID
}
