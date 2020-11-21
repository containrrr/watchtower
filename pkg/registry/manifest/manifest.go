package manifest

import (
	"fmt"
	"github.com/containrrr/watchtower/pkg/registry/helpers"
	ref "github.com/docker/distribution/reference"
	apiTypes "github.com/docker/docker/api/types"
	url2 "net/url"
	"strings"
)

// BuildManifestURL from raw image data
func BuildManifestURL(image apiTypes.ImageInspect) (string, error) {
	img, tag := extractImageAndTag(image)
	hostName, err := ref.ParseNormalizedNamed(img)
	if err != nil {
		return "", err
	}

	host, err := helpers.NormalizeRegistry(hostName.Name())
	if err != nil {
		return "", err
	}
	img = strings.TrimPrefix(img, fmt.Sprintf("%s/", host))
	if !strings.Contains(img, "/") {
		img = "library/" + img
	}
	url := url2.URL{
		Scheme: "https",
		Host:   host,
		Path:   fmt.Sprintf("/v2/%s/manifests/%s", img, tag),
	}
	return url.String(), nil
}

func extractImageAndTag(image apiTypes.ImageInspect) (string, string) {
	var img string
	var tag string
	if strings.Contains(image.RepoTags[0], ":") {
		parts := strings.Split(image.RepoTags[0], ":")
		img = parts[0]
		tag = parts[1]
	} else {
		img = image.RepoTags[0]
		tag = "latest"
	}
	return img, tag
}
