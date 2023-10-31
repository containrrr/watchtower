package updates

import (
	"github.com/containrrr/watchtower/pkg/types"
	"net/url"
	"strings"
)

type ModifyParamsFunc func(up *types.UpdateParams)
type InvokedFunc func(ModifyParamsFunc) types.Report

func parseImages(u *url.URL) []string {
	var images []string
	imageQueries, found := u.Query()["image"]
	if found {
		for _, image := range imageQueries {
			images = append(images, strings.Split(image, ",")...)
		}

	}
	return images
}
