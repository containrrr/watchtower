package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	ref "github.com/containers/image/v5/docker/reference"
	"github.com/containrrr/watchtower/pkg/logger"
	"github.com/containrrr/watchtower/pkg/registry/helpers"
	"github.com/containrrr/watchtower/pkg/types"
	apiTypes "github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"strings"
)

// ChallengeHeader is the HTTP Header containing challenge instructions
const ChallengeHeader = "WWW-Authenticate"

// GetToken fetches a token for the registry hosting the provided image
func GetToken(ctx context.Context, image apiTypes.ImageInspect, credentials *types.RegistryCredentials) (string, error) {
	var err error
	log := logger.GetLogger(ctx)

	img := strings.Split(image.RepoTags[0], ":")[0]
	var url url2.URL
	if url, err = GetChallengeURL(img); err != nil {
		return "", err
	}

	var req *http.Request
	if req, err = GetChallengeRequest(url); err != nil {
		return "", err
	}

	var client = http.Client{}
	var res *http.Response
	if res, err = client.Do(req); err != nil {
		return "", err
	}

	v := res.Header.Get(ChallengeHeader)

	log.WithFields(logrus.Fields{
		"status": res.Status,
		"header": v,
	}).Debug("Got response to challenge request")
	challenge := strings.ToLower(v)
	if strings.HasPrefix(challenge, "basic") {
		return "", errors.New("basic auth not implemented yet")
	}
	if strings.HasPrefix(challenge, "bearer") {
		log.Debug("Fetching bearer token")
		return GetBearerToken(ctx, challenge, img, err, credentials)
	}

	return "", errors.New("unsupported challenge type from registry")
}

// GetChallengeRequest creates a request for getting challenge instructions
func GetChallengeRequest(url url2.URL) (*http.Request, error) {

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Watchtower (Docker)")
	return req, nil
}

// GetBearerToken tries to fetch a bearer token from the registry based on the challenge instructions
func GetBearerToken(ctx context.Context, challenge string, img string, err error, credentials *types.RegistryCredentials) (string, error) {
	log := logger.GetLogger(ctx)
	client := http.Client{}
	authURL := GetAuthURL(challenge, img)

	var r *http.Request
	if r, err = http.NewRequest("GET", authURL.String(), nil); err != nil {
		return "", err
	}

	if credentials.Username != "" && credentials.Password != "" {
		log.WithField("credentials", credentials).Debug("Found credentials. Adding basic auth.")
		r.SetBasicAuth(credentials.Username, credentials.Password)
	} else {
		log.Debug("No credentials found. Doing an anonymous request.")
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

	return tokenResponse.Token, nil
}

// GetAuthURL from the instructions in the challenge
func GetAuthURL(challenge string, img string) *url2.URL {
	raw := strings.TrimPrefix(challenge, "bearer")
	pairs := strings.Split(raw, ",")
	values := make(map[string]string, 0)
	for _, pair := range pairs {
		trimmed := strings.Trim(pair, " ")
		kv := strings.Split(trimmed, "=")
		key := kv[0]
		val := strings.Trim(kv[1], "\"")
		values[key] = val
	}

	authURL, _ := url2.Parse(fmt.Sprintf("%s", values["realm"]))
	q := authURL.Query()
	q.Add("service", values["service"])
	scopeImage := strings.TrimPrefix(img, values["service"])
	scope := fmt.Sprintf("repository:%s:pull", scopeImage)
	q.Add("scope", scope)

	authURL.RawQuery = q.Encode()
	return authURL
}

// GetChallengeURL creates a URL object based on the image info
func GetChallengeURL(img string) (url2.URL, error) {
	normalizedNamed, _ := ref.ParseNormalizedNamed(img)
	host, err := helpers.NormalizeRegistry(normalizedNamed.Name())
	if err != nil {
		return url2.URL{}, err
	}

	url := url2.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v2/",
	}
	return url, nil
}
