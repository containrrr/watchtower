package sorter

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// deployed describes a container of a test deployment, with every input that
// Container.Links reads dependencies from.
type deployed struct {
	// name is the container name, without the leading slash.
	name string
	// project is the Compose project label. Empty means unset.
	project string
	// service is the Compose service label. Empty means unset.
	service string
	// number is the Compose container-number label. Empty means unset.
	number string
	// dependsOn is the Watchtower depends-on label.
	dependsOn string
	// composeDependsOn is the Compose depends_on label.
	composeDependsOn string
	// links are legacy links in name:alias form.
	links []string
	// networkMode is the network mode, such as container:vpn.
	networkMode string
	// volumesFrom are volumes-from specs, such as data:ro.
	volumesFrom []string
	// watchtower marks the container as a Watchtower instance.
	watchtower bool
}

// dockerID returns a stable 64-character hex container ID for a name.
func dockerID(name string) string {
	sum := sha256.Sum256([]byte(name))

	return hex.EncodeToString(sum[:])
}

// container builds the container the description stands for.
func (d deployed) container() types.Container {
	labels := map[string]string{}

	for key, value := range map[string]string{
		"com.docker.compose.project":                d.project,
		"com.docker.compose.service":                d.service,
		"com.docker.compose.container-number":       d.number,
		"com.docker.compose.depends_on":             d.composeDependsOn,
		"com.centurylinklabs.watchtower.depends-on": d.dependsOn,
	} {
		if value != "" {
			labels[key] = value
		}
	}

	if d.watchtower {
		labels["com.centurylinklabs.watchtower"] = "true"
	}

	return container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:   dockerID(d.name),
		Name: "/" + d.name,
		HostConfig: &dockerContainer.HostConfig{
			Links:       d.links,
			NetworkMode: dockerContainer.NetworkMode(d.networkMode),
			VolumesFrom: d.volumesFrom,
		},
		Config: &dockerContainer.Config{Labels: labels},
	}, nil)
}

// deploy builds the containers of a test deployment.
func deploy(specs ...deployed) []types.Container {
	containers := make([]types.Container, 0, len(specs))
	for _, spec := range specs {
		containers = append(containers, spec.container())
	}

	return containers
}

// pick returns the containers of a deployment with the given names, in the
// order named.
func pick(containers []types.Container, names ...string) []types.Container {
	picked := make([]types.Container, 0, len(names))

	for _, name := range names {
		i := slices.IndexFunc(containers, func(c types.Container) bool { return c.Name() == name })
		if i >= 0 {
			picked = append(picked, containers[i])
		}
	}

	return picked
}

// containerNames returns the names of containers, in order.
func containerNames(containers []types.Container) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name())
	}

	return names
}
