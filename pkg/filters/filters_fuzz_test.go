package filters

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func fuzzLog() *zerolog.Logger {
	n := zerolog.Nop()

	return &n
}

// fuzzableContainer is a minimal FilterableContainer implementation for fuzzing
// filter functions. It stores labels in a map and returns fixed values for
// other fields.
type fuzzableContainer struct {
	name       string
	imageName  string
	enabled    bool
	scope      string
	hasScope   bool
	labels     map[string]string
	watchtower bool
}

func newFuzzableContainer(name, imageName string, enabled bool, scope string, hasScope bool, labels map[string]string) fuzzableContainer {
	if labels == nil {
		labels = make(map[string]string)
	}

	return fuzzableContainer{
		name:       name,
		imageName:  imageName,
		enabled:    enabled,
		scope:      scope,
		hasScope:   hasScope,
		labels:     labels,
		watchtower: false,
	}
}

func (c fuzzableContainer) Name() string {
	return c.name
}

func (c fuzzableContainer) ImageName() string {
	return c.imageName
}

func (c fuzzableContainer) Enabled() (bool, bool) {
	return c.enabled, true
}

func (c fuzzableContainer) Scope() (string, bool) {
	return c.scope, c.hasScope
}

func (c fuzzableContainer) GetLabel(key string) (string, bool) {
	value, ok := c.labels[key]

	return value, ok
}

func (c fuzzableContainer) IsWatchtower() bool {
	return c.watchtower
}

var _ types.FilterableContainer = fuzzableContainer{}

func FuzzParseLabelPairs(f *testing.F) {
	f.Add("key=value")
	f.Add("key=")
	f.Add("  key  =  value  ")
	f.Add("key=value=extra")
	f.Add("nokey")
	f.Add("=value")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		_, err := parseLabelPairs([]string{input})
		if err != nil {
			return
		}
	})
}

func FuzzFilterByEnabledLabels(f *testing.F) {
	f.Add("app", "nginx:latest", "com.example.enabled=true", "com.example.enabled=true")
	f.Add("web", "redis:alpine", "com.example.env=prod", "")
	f.Add("db", "postgres:15", "com.example.env=prod", "com.example.env=staging")
	f.Add("app", "nginx:latest", "", "com.example.enabled=true")

	f.Fuzz(func(t *testing.T, containerName, imageName, filterLabelPair, containerLabelPair string) {
		filterLabels, err := parseLabelPairs([]string{filterLabelPair})
		if err != nil {
			return
		}

		if len(filterLabels) == 0 {
			return
		}

		containerLabels, err := parseLabelPairs([]string{containerLabelPair})
		if err != nil {
			return
		}

		c := newFuzzableContainer(containerName, imageName, true, "", false, containerLabels)
		filter := FilterByEnabledLabels(fuzzLog(), filterLabels, NoFilter)
		_ = filter(c)
	})
}

func FuzzFilterByDisabledLabels(f *testing.F) {
	f.Add("app", "nginx:latest", "com.example.disabled=true", "com.example.disabled=true")
	f.Add("web", "redis:alpine", "com.example.env=prod", "")
	f.Add("db", "postgres:15", "com.example.env=prod", "com.example.env=staging")
	f.Add("app", "nginx:latest", "", "com.example.disabled=true")

	f.Fuzz(func(t *testing.T, containerName, imageName, filterLabelPair, containerLabelPair string) {
		filterLabels, err := parseLabelPairs([]string{filterLabelPair})
		if err != nil {
			return
		}

		if len(filterLabels) == 0 {
			return
		}

		containerLabels, err := parseLabelPairs([]string{containerLabelPair})
		if err != nil {
			return
		}

		c := newFuzzableContainer(containerName, imageName, true, "", false, containerLabels)
		filter := FilterByDisabledLabels(fuzzLog(), filterLabels, NoFilter)
		_ = filter(c)
	})
}

func FuzzMatchImageAndTag(f *testing.F) {
	f.Add("nginx:latest", "nginx")
	f.Add("redis:alpine", "redis:alpine")
	f.Add("myapp:1.0", "myapp:1.0")
	f.Add("registry:5000/app:v1", "registry:5000/app:v1")
	f.Add("postgres:15", "postgres")

	f.Fuzz(func(t *testing.T, containerImage, targetImage string) {
		_ = matchImageAndTag(containerImage, targetImage)
	})
}
