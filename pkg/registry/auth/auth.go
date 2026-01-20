package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/containrrr/watchtower/pkg/registry/helpers"
	"github.com/containrrr/watchtower/pkg/types"
	ref "github.com/distribution/reference"
	"github.com/sirupsen/logrus"
)

// ChallengeHeader is the HTTP Header containing challenge instructions
const ChallengeHeader = "WWW-Authenticate"

// GetToken fetches a token for the registry hosting the provided image
func GetToken(container types.Container, registryAuth string) (string, error) {
	normalizedRef, err := ref.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		return "", err
	}

	URL := GetChallengeURL(normalizedRef)
	logrus.WithField("URL", URL.String()).Debug("Built challenge URL")

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
		return GetBearerHeader(challenge, normalizedRef, registryAuth)
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
func GetBearerHeader(challenge string, imageRef ref.Named, registryAuth string) (string, error) {
	client := http.Client{}

	authURL, err := GetAuthURL(challenge, imageRef)
	if err != nil {
		return "", err
	}

	// Build cache key from the auth realm, service and scope
	cacheKey := authURL.String()

	// Check cache first
	if token := getCachedToken(cacheKey); token != "" {
		return fmt.Sprintf("Bearer %s", token), nil
	}

	var r *http.Request
	if r, err = http.NewRequest("GET", authURL.String(), nil); err != nil {
		return "", err
	}

	if registryAuth != "" {
		logrus.Debug("Credentials found.")
		r.Header.Add("Authorization", fmt.Sprintf("Basic %s", registryAuth))
	} else {
		logrus.Debug("No credentials found.")
	}

	var authResponse *http.Response
	if authResponse, err = client.Do(r); err != nil {
		return "", err
	}
	defer authResponse.Body.Close()

	body, _ := io.ReadAll(authResponse.Body)
	tokenResponse := &types.TokenResponse{}

	err = json.Unmarshal(body, tokenResponse)
	if err != nil {
		return "", err
	}

	// Cache token if ExpiresIn provided
	if tokenResponse.Token != "" {
		storeToken(cacheKey, tokenResponse.Token, tokenResponse.ExpiresIn)
	}

	return fmt.Sprintf("Bearer %s", tokenResponse.Token), nil
}

// token cache implementation
type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	tokenCache   = map[string]cachedToken{}
	tokenCacheMu = &sync.Mutex{}
)

// now is a package-level function returning current time. It is a variable so tests
// can override it for deterministic behavior.
var now = time.Now

// getCachedToken returns token string if present and not expired, otherwise empty
func getCachedToken(key string) string {
	tokenCacheMu.Lock()
	defer tokenCacheMu.Unlock()
	if ct, ok := tokenCache[key]; ok {
		if ct.expiresAt.IsZero() || now().Before(ct.expiresAt) {
			return ct.token
		}
		// expired
		delete(tokenCache, key)
	}
	return ""
}

// storeToken stores token with optional ttl (seconds). ttl<=0 means no expiry.
func storeToken(key, token string, ttl int) {
	tokenCacheMu.Lock()
	defer tokenCacheMu.Unlock()
	ct := cachedToken{token: token}
	if ttl > 0 {
		ct.expiresAt = now().Add(time.Duration(ttl) * time.Second)
	}
	tokenCache[key] = ct
}

// GetAuthURL from the instructions in the challenge
func GetAuthURL(challenge string, imageRef ref.Named) (*url.URL, error) {
	loweredChallenge := strings.ToLower(challenge)
	raw := strings.TrimPrefix(loweredChallenge, "bearer")

	pairs := strings.Split(raw, ",")
	values := make(map[string]string, len(pairs))

	for _, pair := range pairs {
		trimmed := strings.Trim(pair, " ")
		if key, val, ok := strings.Cut(trimmed, "="); ok {
			values[key] = strings.Trim(val, `"`)
		}
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

	scopeImage := ref.Path(imageRef)

	scope := fmt.Sprintf("repository:%s:pull", scopeImage)
	logrus.WithFields(logrus.Fields{"scope": scope, "image": imageRef.Name()}).Debug("Setting scope for auth token")
	q.Add("scope", scope)

	authURL.RawQuery = q.Encode()
	return authURL, nil
}

// GetChallengeURL returns the URL to check auth requirements
// for access to a given image
func GetChallengeURL(imageRef ref.Named) url.URL {
	host, _ := helpers.GetRegistryAddress(imageRef.Name())

	URL := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/v2/",
	}
	return URL
}
