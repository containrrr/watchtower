package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/containrrr/watchtower/internal/meta"
	"github.com/containrrr/watchtower/pkg/registry/manifest"
	"github.com/containrrr/watchtower/pkg/types"
	"github.com/sirupsen/logrus"
)

// ContentDigestHeader is the key for the key-value pair containing the digest header
const ContentDigestHeader = "Docker-Content-Digest"

// CompareDigest retrieves the latest digest for the container image from the registry
// and returns whether it matches any of the containers current image's digest
func (rc *Client) CompareDigest(ctx context.Context, container types.Container, registryAuth string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, rc.Timeout)
	defer cancel()

	if !container.HasImageInfo() {
		return false, errors.New("container image info missing")
	}

	var digest string

	registryAuth = TransformAuth(registryAuth)
	token, err := rc.GetToken(ctx, container, registryAuth)
	if err != nil {
		return false, err
	}

	digestURL, err := manifest.BuildManifestURL(container)
	if err != nil {
		return false, err
	}

	if digest, err = rc.GetDigest(ctx, digestURL, token); err != nil {
		return false, err
	}

	logrus.WithField("remote", digest).Debug("Found a remote digest to compare with")

	for _, dig := range container.ImageInfo().RepoDigests {
		localDigest := strings.Split(dig, "@")[1]
		fields := logrus.Fields{"local": localDigest, "remote": digest}
		logrus.WithFields(fields).Debug("Comparing")

		if localDigest == digest {
			logrus.Debug("Found a match")
			return true, nil
		}
	}

	return false, nil
}

// TransformAuth from a base64 encoded json object to base64 encoded string
func TransformAuth(registryAuth string) string {
	b, _ := base64.StdEncoding.DecodeString(registryAuth)
	credentials := &types.RegistryCredentials{}
	_ = json.Unmarshal(b, credentials)

	if credentials.Username != "" && credentials.Password != "" {
		ba := []byte(fmt.Sprintf("%s:%s", credentials.Username, credentials.Password))
		registryAuth = base64.StdEncoding.EncodeToString(ba)
	}

	return registryAuth
}

// GetDigest from registry using a HEAD request to prevent rate limiting
func (rc *Client) GetDigest(ctx context.Context, url string, token string) (string, error) {

	req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	req.Header.Set("User-Agent", meta.UserAgent)

	if token != "" {
		logrus.WithField("token", token).Trace("Setting request token")
	} else {
		return "", errors.New("could not fetch token")
	}

	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v1+json")

	logrus.WithField("url", url).Debug("Doing a HEAD request to fetch a digest")

	res, err := rc.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		wwwAuthHeader := res.Header.Get("www-authenticate")
		if wwwAuthHeader == "" {
			wwwAuthHeader = "not present"
		}
		return "", fmt.Errorf("registry responded to head request with %q, auth: %q", res.Status, wwwAuthHeader)
	}
	return res.Header.Get(ContentDigestHeader), nil
}
