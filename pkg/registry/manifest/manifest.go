package manifest

import (
	"fmt"
	"github.com/containrrr/watchtower/pkg/registry/helpers"
	"github.com/containrrr/watchtower/pkg/types"
	ref "github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
	url2 "net/url"
	"strings"
)

// BuildManifestURL from raw image data
func BuildManifestURL(container types.Container) (string, error) {

	normalizedName, err := ref.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		return "", err
	}

	host, err := helpers.NormalizeRegistry(normalizedName.String())
	img, tag := extractImageAndTag(strings.TrimPrefix(container.ImageName(), host+"/"))

	logrus.WithFields(logrus.Fields{
		"image":      img,
		"tag":        tag,
		"normalized": normalizedName,
		"host":       host,
	}).Debug("Parsing image ref")

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

func extractImageAndTag(imageName string) (string, string) {
	var img string
	var tag string

	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		if len(parts) > 2 {
			img = fmt.Sprintf("%s%s", parts[0], parts[1])
			tag = parts[3]
		} else {
			img = parts[0]
			tag = parts[1]
		}
	} else {
		img = imageName
		tag = "latest"
	}
	return img, tag
}
