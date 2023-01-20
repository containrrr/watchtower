package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/containrrr/watchtower/pkg/registry/auth"
	"github.com/containrrr/watchtower/pkg/types"
	"github.com/sirupsen/logrus"
)

// ChallengeHeader is the HTTP Header containing challenge instructions
const ChallengeHeader = "WWW-Authenticate"

// GetToken fetches a token for the registry hosting the provided image
func (rc *Client) GetToken(ctx context.Context, container types.Container, registryAuth string) (string, error) {
	var err error
	var URL url.URL

	if URL, err = auth.GetChallengeURL(container.ImageName()); err != nil {
		return "", err
	}
	logrus.WithField("URL", URL.String()).Debug("Building challenge URL")

	var req *http.Request
	if req, err = rc.GetChallengeRequest(ctx, URL); err != nil {
		return "", err
	}

	var res *http.Response
	if res, err = rc.httpClient.Do(req); err != nil {
		return "", err
	}
	defer res.Body.Close()
	v := res.Header.Get(ChallengeHeader)

	logrus.WithFields(logrus.Fields{
		"status": res.Status,
		"header": v,
	}).Debug("Got response to challenge request")

	challenge := strings.ToLower(v)
	if strings.HasPrefix(challenge, "basic") {
		if registryAuth == "" {
			return "", fmt.Errorf("no credentials available")
		}

		return fmt.Sprintf("Basic %s", registryAuth), nil
	}
	if strings.HasPrefix(challenge, "bearer") {
		return rc.GetBearerHeader(ctx, challenge, container.ImageName(), registryAuth)
	}

	return "", errors.New("unsupported challenge type from registry")
}

// GetChallengeRequest creates a request for getting challenge instructions
func (rc *Client) GetChallengeRequest(ctx context.Context, URL url.URL) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", URL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Watchtower (Docker)")
	return req, nil
}

// GetBearerHeader tries to fetch a bearer token from the registry based on the challenge instructions
func (rc *Client) GetBearerHeader(ctx context.Context, challenge string, img string, registryAuth string) (string, error) {
	if strings.Contains(img, ":") {
		img = strings.Split(img, ":")[0]
	}
	authURL, err := auth.GetAuthURL(challenge, img)

	if err != nil {
		return "", err
	}

	var r *http.Request
	if r, err = http.NewRequestWithContext(ctx, "GET", authURL.String(), nil); err != nil {
		return "", err
	}

	if registryAuth != "" {
		logrus.Debug("Credentials found.")
		logrus.Tracef("Credentials: %v", registryAuth)
		r.Header.Add("Authorization", fmt.Sprintf("Basic %s", registryAuth))
	} else {
		logrus.Debug("No credentials found.")
	}

	var authResponse *http.Response
	if authResponse, err = rc.httpClient.Do(r); err != nil {
		return "", err
	}

	body, _ := ioutil.ReadAll(authResponse.Body)
	tokenResponse := &types.TokenResponse{}

	err = json.Unmarshal(body, tokenResponse)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Bearer %s", tokenResponse.Token), nil
}
