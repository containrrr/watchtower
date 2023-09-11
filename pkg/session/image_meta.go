package session

import "strings"

type imageMeta map[string]string

const openContainersPrefix = "org.opencontainers.image."

func imageMetaFromLabels(labels map[string]string) imageMeta {
	im := make(imageMeta)
	for key, value := range labels {
		if strings.HasPrefix(key, openContainersPrefix) {
			strippedKey := key[len(openContainersPrefix):]
			im[strippedKey] = value
		}
	}
	return im
}

func (im imageMeta) Authors() string {
	return im["authors"]
}

func (im imageMeta) Created() string {
	return im["created"]
}

func (im imageMeta) Description() string {
	return im["description"]
}

func (im imageMeta) Documentation() string {
	return im["documentation"]
}

func (im imageMeta) Licenses() string {
	return im["licenses"]
}

func (im imageMeta) Revision() string {
	return im["revision"]
}

func (im imageMeta) Source() string {
	return im["source"]
}

func (im imageMeta) Title() string {
	return im["title"]
}

func (im imageMeta) Url() string {
	return im["url"]
}

func (im imageMeta) Version() string {
	return im["version"]
}
