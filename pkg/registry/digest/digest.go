package digest

import (
	"encoding/json"
	"errors"
	"fmt"
	ref "github.com/containers/image/v5/docker/reference"
	apiTypes "github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"

	"io/ioutil"
	"net/http"
	url2 "net/url"
	"strings"
)

const (
	ManifestListV2ContentType = "application/vnd.docker.distribution.manifest.list.v2+json"
	ChallengeHeader           = "WWW-Authenticate"
	ContentDigestHeader       = "Docker-Content-Digest"
)

// CompareDigest ...
func CompareDigest(image apiTypes.ImageInspect, credentials *RegistryCredentials) (bool, error) {
	var digest string

	token, err := GetToken(image, credentials)
	if err != nil {
		return false, err
	}

	digestURL, err := BuildManifestURL(image)
	if err != nil {
		return false, err
	}

	if digest, err = GetDigest(digestURL, token); err != nil {
		return false, err
	}

	log.WithField("Remote Digest", digest).Debug()
	log.WithField("Local Image ID", image.ID).Debug()

	if image.ID == digest {
		return true, nil
	}

	for _, dig := range image.RepoDigests {
		localDigest := strings.Split(dig, "@")[1]
		log.WithField("Local Digest", localDigest).Debug("Comparing with local digest")
		if localDigest == digest {
			return true,nil
		}
	}

	return false, nil
}

func GetDigest(url string, token string) (string, error) {
	client := &http.Client{}
	if token != "" {
		log.WithField("token", token).Debug("Setting request bearer token")
	} else {
		return "", errors.New("could not fetch token")
	}

	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Add("Authorization", "Bearer " + token)
	req.Header.Add("Accept", ManifestListV2ContentType)
	log.WithField("url", url)

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", errors.New(
			fmt.Sprintf("registry responded to head request with %d", res.StatusCode),
		)
	}
	return res.Header.Get(ContentDigestHeader), nil
}

func GetToken(image apiTypes.ImageInspect, credentials *RegistryCredentials) (string, error){
	img := strings.Split(image.RepoTags[0], ":")[0]
	url := GetChallengeURL(img)

	res, err := DoChallengeRequest(url)
	if err != nil {
		return "", err
	}

	v := res.Header.Get(ChallengeHeader)
	challenge := strings.ToLower(v)
	if strings.HasPrefix(challenge, "basic") {
		return "", errors.New("basic auth not implemented yet")
	}
	if strings.HasPrefix(challenge, "bearer") {
		return GetBearerToken(challenge, img, err, credentials)
	}

	return "", errors.New("unsupported challenge type from registry")
}

func DoChallengeRequest(url url2.URL) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url.String(), nil)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Watchtower (Docker)")
	client := http.Client{}
	return client.Do(req)
}

func GetBearerToken(challenge string, img string, err error, credentials *RegistryCredentials) (string, error) {
	client := http.Client{}
	authURL := GetAuthURL(challenge, img)

	var r *http.Request
	if r, err = http.NewRequest("GET", authURL.String(), nil); err != nil {
		return "", err
	}

	if credentials.Username != "" && credentials.Password != "" {
		r.SetBasicAuth(credentials.Username, credentials.Password)
	}

	var authResponse *http.Response
	if authResponse, err = client.Do(r); err != nil {
		return "", err
	}

	body, _ := ioutil.ReadAll(authResponse.Body)
	tokenResponse := &TokenResponse{}

	err = json.Unmarshal(body, tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}

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

func GetChallengeURL(img string) url2.URL {
	normalizedNamed, _ := ref.ParseNormalizedNamed(img)

	url := url2.URL{
		Scheme: "https",
		Host:   normalizeRegistry(normalizedNamed.Name()),
		Path:   "/v2/",
	}
	return url
}

type TokenResponse struct {
	Token string `json:"token"`
}

type RegistryCredentials struct {
	Username string
	Password string // usually a token rather than an actual password
}

func BuildManifestURL(image apiTypes.ImageInspect) (string, error) {
	parts := strings.Split(image.RepoTags[0], ":")
	img := parts[0]
	tag := parts[1]

	hostName, err := ref.ParseNormalizedNamed(img)
	if err != nil {
		return "", err
	}

	host := normalizeRegistry(hostName.Name())
	img = strings.TrimPrefix(img, host)
	url := url2.URL{
		Scheme: "https",
		Host: host,
		Path: fmt.Sprintf("/v2/%s/manifests/%s", img, tag),
	}
	return url.String(), nil
}


// Copied from github.com/docker/docker/registry/auth.go
func convertToHostname(url string) string {
	stripped := url
	if strings.HasPrefix(url, "http://") {
		stripped = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		stripped = strings.TrimPrefix(url, "https://")
	}

	nameParts := strings.SplitN(stripped, "/", 2)

	return nameParts[0]
}

// Copied from https://github.com/containers/image/pkg/docker/config/config.go
func normalizeRegistry(registry string) string {
	normalized := convertToHostname(registry)
	switch normalized {
	case "registry-1.docker.io", "docker.io":
		return "index.docker.io"
	}
	return normalized
}