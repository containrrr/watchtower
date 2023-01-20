package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/containrrr/watchtower/pkg/registry/helpers"
	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

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
