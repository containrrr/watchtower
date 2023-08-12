package session

import "strings"

type imageMeta map[string]string

func imageMetaFromLabels(labels map[string]string) imageMeta {
	im := make(imageMeta)
	for key, value := range labels {
		if suffix, found := strings.CutPrefix(key, "org.opencontainers.image."); found {
			im[suffix] = value
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
