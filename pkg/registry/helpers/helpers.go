package helpers

import (
	"fmt"
	url2 "net/url"
)

// ConvertToHostname strips a url from everything but the hostname part
func ConvertToHostname(url string) (string, string, error) {
	urlWithSchema := fmt.Sprintf("x://%s", url)
	u, err := url2.Parse(urlWithSchema)
	if err != nil {
		return "", "", err
	}
	hostName := u.Hostname()
	port := u.Port()

	return hostName, port, err
}

// NormalizeRegistry makes sure variations of DockerHubs registry
func NormalizeRegistry(registry string) (string, error) {
	hostName, port, err := ConvertToHostname(registry)
	if err != nil {
		return "", err
	}

	if hostName == "registry-1.docker.io" || hostName == "docker.io" {
		hostName = "index.docker.io"
	}

	if port != "" {
		return fmt.Sprintf("%s:%s", hostName, port), nil
	}
	return hostName, nil
}
