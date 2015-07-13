package updater

import (
	"github.com/samalba/dockerclient"
)

// Ideally, we'd just be able to take the ContainerConfig from the old container
// and use it as the starting point for creating the new container; however,
// the ContainerConfig that comes back from the Inspect call merges the default
// configuration (the stuff specified in the metadata for the image itself)
// with the overridden configuration (the stuff that you might specify as part
// of the "docker run"). In order to avoid unintentionally overriding the
// defaults in the new image we need to separate the override options from the
// default options. To do this we have to compare the ContainerConfig for the
// running container with the ContainerConfig from the image that container was
// started from. This function returns a ContainerConfig which contains just
// the override options.
func GenerateContainerConfig(oldContainerInfo *dockerclient.ContainerInfo, oldImageConfig *dockerclient.ContainerConfig) *dockerclient.ContainerConfig {
	config := oldContainerInfo.Config

	if config.WorkingDir == oldImageConfig.WorkingDir {
		config.WorkingDir = ""
	}

	if config.User == oldImageConfig.User {
		config.User = ""
	}

	if sliceEqual(config.Cmd, oldImageConfig.Cmd) {
		config.Cmd = []string{}
	}

	if sliceEqual(config.Entrypoint, oldImageConfig.Entrypoint) {
		config.Entrypoint = []string{}
	}

	config.Env = arraySubtract(config.Env, oldImageConfig.Env)

	config.Labels = stringMapSubtract(config.Labels, oldImageConfig.Labels)

	config.Volumes = structMapSubtract(config.Volumes, oldImageConfig.Volumes)

	config.ExposedPorts = structMapSubtract(config.ExposedPorts, oldImageConfig.ExposedPorts)
	for p, _ := range oldContainerInfo.HostConfig.PortBindings {
		config.ExposedPorts[p] = struct{}{}
	}

	return config
}

func sliceEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}

	return true
}

func stringMapSubtract(m1, m2 map[string]string) map[string]string {
	m := map[string]string{}

	for k1, v1 := range m1 {
		if v2, ok := m2[k1]; ok {
			if v2 != v1 {
				m[k1] = v1
			}
		} else {
			m[k1] = v1
		}
	}

	return m
}

func structMapSubtract(m1, m2 map[string]struct{}) map[string]struct{} {
	m := map[string]struct{}{}

	for k1, v1 := range m1 {
		if _, ok := m2[k1]; !ok {
			m[k1] = v1
		}
	}

	return m
}

func arraySubtract(a1, a2 []string) []string {
	a := []string{}

	for _, e1 := range a1 {
		found := false

		for _, e2 := range a2 {
			if e1 == e2 {
				found = true
				break
			}
		}

		if !found {
			a = append(a, e1)
		}
	}

	return a
}
