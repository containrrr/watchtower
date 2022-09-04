package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/containrrr/watchtower/pkg/registry/helpers"
	"github.com/containrrr/watchtower/pkg/types"
	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

// ChallengeHeader is the HTTP Header containing challenge instructions
const ChallengeHeader = "WWW-Authenticate"

// GetToken fetches a token for the registry hosting the provided image
func GetToken(container types.Container, registryAuth string) (string, error) {
	var err error
	var URL url.URL

	if URL, err = GetChallengeURL(container.ImageName()); err != nil {
		return "", err
	}
	logrus.WithField("URL", URL.String()).Debug("Building challenge URL")

	var req *http.Request
	if req, err = GetChallengeRequest(URL); err != nil {
		return "", err
	}

	client := &http.Client{}
	var res *http.Response
	if res, err = client.Do(req); err != nil {
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
		return GetBearerHeader(challenge, container.ImageName(), registryAuth)
	}

	return "", errors.New("unsupported challenge type from registry")
}

// GetChallengeRequest creates a request for getting challenge instructions
func GetChallengeRequest(URL url.URL) (*http.Request, error) {
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Watchtower (Docker)")
	return req, nil
}

// GetBearerHeader tries to fetch a bearer token from the registry based on the challenge instructions
func GetBearerHeader(challenge string, img string, registryAuth string) (string, error) {
	client := http.Client{}
	if strings.Contains(img, ":") {
		img = strings.Split(img, ":")[0]
	}
	authURL, err := GetAuthURL(challenge, img)

	if err != nil {
		return "", err
	}

	var r *http.Request
	if r, err = http.NewRequest("GET", authURL.String(), nil); err != nil {
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
	if authResponse, err = client.Do(r); err != nil {
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

// GetAuthURL from the instructions in the challenge
func GetAuthURL(challenge string, img string) (*url.URL, error) {
	loweredChallenge := strings.ToLower(challenge)
	raw := strings.TrimPrefix(loweredChallenge, "bearer")

	pairs := strings.Split(raw, ",")
	values := make(map[string]string, len(pairs))

	for _, pair := range pairs {
		trimmed := strings.Trim(pair, " ")
		kv := strings.Split(trimmed, "=")
		key := kv[0]
		val := strings.Trim(kv[1], "\"")
		values[key] = val
	}
	logrus.WithFields(logrus.Fields{
		"realm":   values["realm"],
		"service": values["service"],
	}).Debug("Checking challenge header content")
	if values["realm"] == "" || values["service"] == "" {

		return nil, fmt.Errorf("challenge header did not include all values needed to construct an auth url")
	}

	authURL, _ := url.Parse(values["realm"])
	q := authURL.Query()
	q.Add("service", values["service"])

	scopeImage := GetScopeFromImageName(img, values["service"])

	scope := fmt.Sprintf("repository:%s:pull", scopeImage)
	logrus.WithFields(logrus.Fields{"scope": scope, "image": img}).Debug("Setting scope for auth token")
	q.Add("scope", scope)

	authURL.RawQuery = q.Encode()
	return authURL, nil
}

// GetScopeFromImageName normalizes an image name for use as scope during auth and head requests
func GetScopeFromImageName(img, svc string) string {
	parts := strings.Split(img, "/")

	if len(parts) > 2 {
		if strings.Contains(svc, "docker.io") {
			return fmt.Sprintf("%s/%s", parts[1], strings.Join(parts[2:], "/"))
		}
		return strings.Join(parts, "/")
	}

	if len(parts) == 2 {
		if strings.Contains(parts[0], "docker.io") {
			return fmt.Sprintf("library/%s", parts[1])
		}
		return strings.Replace(img, svc+"/", "", 1)
	}

	if strings.Contains(svc, "docker.io") {
		return fmt.Sprintf("library/%s", parts[0])
	}
	return img
}

// GetChallengeURL creates a URL object based on the image info
func GetChallengeURL(img string) (url.URL, error) {

	normalizedNamed, _ := reference.ParseNormalizedNamed(img)
	host, err := helpers.NormalizeRegistry(normalizedNamed.String())
	if err != nil {
		return url.URL{}, err
	}

	URL := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v2/",
	}
	return URL, nil
}
