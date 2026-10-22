package actions

import (
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func getCommonTestData() *mockActions.TestData {
	return &mockActions.TestData{
		NameOfContainerToKeep: "",
		Containers: []types.Container{
			mockActions.CreateMockContainer(
				"test-container-01",
				"test-container-01",
				"fake-image:latest",
				time.Now().AddDate(0, 0, -1),
			),
			mockActions.CreateMockContainer(
				"test-container-02",
				"test-container-02",
				"fake-image:latest",
				time.Now(),
			),
			mockActions.CreateMockContainer(
				"test-container-03",
				"test-container-03",
				"fake-image:latest",
				time.Now(),
			),
		},
	}
}

func getLinkedTestData(withImageInfo bool) *mockActions.TestData {
	staleContainer := mockActions.CreateMockContainer(
		"test-container-01",
		"/test-container-01",
		"fake-image1:latest",
		time.Now().AddDate(0, 0, -1),
	)

	var imageInfo *dockerImage.InspectResponse
	if withImageInfo {
		imageInfo = mockActions.CreateMockImageInfo("test-container-02")
	}

	linkingContainer := mockActions.CreateMockContainerWithLinks(
		"test-container-02",
		"/test-container-02",
		"fake-image2:latest",
		time.Now(),
		[]string{staleContainer.Name()},
		imageInfo,
	)

	return &mockActions.TestData{
		Staleness: map[string]bool{linkingContainer.Name(): false},
		Containers: []types.Container{
			staleContainer,
			linkingContainer,
		},
	}
}

func getNetworkModeTestData() *mockActions.TestData {
	staleContainer := mockActions.CreateMockContainer(
		"network-dependency",
		"/network-dependency",
		"fake-image:latest",
		time.Now().AddDate(0, 0, -1),
	)

	dependentContainer := mockActions.CreateMockContainerWithConfig(
		"network-dependent",
		"/network-dependent",
		"fake-image2:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image:        "fake-image2:latest",
			Labels:       make(map[string]string),
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	// Set network mode to container:network-dependency
	dependentContainer.ContainerInfo().HostConfig.NetworkMode = "container:network-dependency"

	return &mockActions.TestData{
		Staleness:  map[string]bool{staleContainer.Name(): true, dependentContainer.Name(): false},
		Containers: []types.Container{staleContainer, dependentContainer},
	}
}

func getComposeTestData() *mockActions.TestData {
	// Create a database container with service name "db" but container name "myproject_db_1"
	dbContainer := mockActions.CreateMockContainerWithConfig(
		"myproject_db_1",
		"/myproject_db_1",
		"postgres:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "postgres:latest",
			Labels: map[string]string{
				"com.docker.compose.service": "db",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	// Create a web container with service name "web" but container name "myproject_web_1"
	// that depends on "db"
	webContainer := mockActions.CreateMockContainerWithConfig(
		"myproject_web_1",
		"/myproject_web_1",
		"web:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "web:latest",
			Labels: map[string]string{
				"com.docker.compose.service":    "web",
				"com.docker.compose.depends_on": "db",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	return &mockActions.TestData{
		Staleness:  map[string]bool{dbContainer.Name(): true, webContainer.Name(): false},
		Containers: []types.Container{dbContainer, webContainer},
	}
}

func getComposeProjectPrefixedTestData() *mockActions.TestData {
	// Create a database container with project and service labels
	dbContainer := mockActions.CreateMockContainerWithConfig(
		"myapp-database-1",
		"/myapp-database-1",
		"postgres:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "postgres:latest",
			Labels: map[string]string{
				"com.docker.compose.project": "myapp",
				"com.docker.compose.service": "database",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	// Create a web container with project and service labels that depends on "database"
	webContainer := mockActions.CreateMockContainerWithConfig(
		"myapp-watchtower-test-app1-1",
		"/myapp-watchtower-test-app1-1",
		"nginx:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "nginx:latest",
			Labels: map[string]string{
				"com.docker.compose.project":    "myapp",
				"com.docker.compose.service":    "watchtower-test-app1",
				"com.docker.compose.depends_on": "database:service_started:true",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	return &mockActions.TestData{
		Staleness:  map[string]bool{dbContainer.Name(): true, webContainer.Name(): false},
		Containers: []types.Container{dbContainer, webContainer},
	}
}

func getComposeMultiHopTestData() *mockActions.TestData {
	// Create containers for a chain: cache -> db -> app
	// depends on db, db depends on cache
	cacheContainer := mockActions.CreateMockContainerWithConfig(
		"myproject_cache_1",
		"/myproject_cache_1",
		"redis:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "redis:latest",
			Labels: map[string]string{
				"com.docker.compose.service": "cache",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	dbContainer := mockActions.CreateMockContainerWithConfig(
		"myproject_db_1",
		"/myproject_db_1",
		"postgres:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "postgres:latest",
			Labels: map[string]string{
				"com.docker.compose.service":    "db",
				"com.docker.compose.depends_on": "cache",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	appContainer := mockActions.CreateMockContainerWithConfig(
		"myproject_app_1",
		"/myproject_app_1",
		"app:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "app:latest",
			Labels: map[string]string{
				"com.docker.compose.service":    "app",
				"com.docker.compose.depends_on": "db",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	return &mockActions.TestData{
		Staleness: map[string]bool{
			cacheContainer.Name(): true,
			dbContainer.Name():    false,
			appContainer.Name():   false,
		},
		Containers: []types.Container{cacheContainer, dbContainer, appContainer},
	}
}

// getComposeHyphenatedProjectTestData creates test data for a hyphenated Compose
// project name with explicit container_name declarations and real Compose-emitted
// depends_on label strings in "service:condition:bool" format.
func getComposeHyphenatedProjectTestData() *mockActions.TestData {
	// Base service with *explicit container_name* producing a bare runtime name.
	// Project label present so Links() will emit the project-prefixed form ("download-torrent-base").
	baseContainer := mockActions.CreateMockContainerWithConfig(
		"base",
		"/base",
		"redis:alpine",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "redis:alpine",
			Labels: map[string]string{
				"com.docker.compose.project":          "download-torrent",
				"com.docker.compose.service":          "base",
				"com.docker.compose.container-number": "1",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	// Dependent with bare container_name + real Compose-emitted depends_on label format.
	dependentContainer := mockActions.CreateMockContainerWithConfig(
		"dependent",
		"/dependent",
		"app:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "app:latest",
			Labels: map[string]string{
				"com.docker.compose.project":          "download-torrent",
				"com.docker.compose.service":          "dependent",
				"com.docker.compose.container-number": "1",
				"com.docker.compose.depends_on":       "base:service_started:false",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	return &mockActions.TestData{
		Staleness:  map[string]bool{baseContainer.Name(): true, dependentContainer.Name(): false},
		Containers: []types.Container{baseContainer, dependentContainer},
	}
}

// getHyphenatedProjectWithContainerNameTestData creates test data for a Compose
// project name containing multiple hyphens, using explicit container_name on all
// services and real Compose-emitted depends_on label strings.
func getHyphenatedProjectWithContainerNameTestData() *mockActions.TestData {
	// Uses a project name with multiple hyphens and explicit container_name values
	// to exercise the restart marking logic with realistic Compose label output.
	base := mockActions.CreateMockContainerWithConfig(
		"base",
		"/base",
		"redis:alpine",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "redis:alpine",
			// Set container-number so ResolveContainerIdentifier returns the
			// replica form while compose depends_on links remain non-replica.
			Labels: map[string]string{
				"com.docker.compose.project":          "my-app-project",
				"com.docker.compose.service":          "base",
				"com.docker.compose.container-number": "1",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	dependentSimple := mockActions.CreateMockContainerWithConfig(
		"dependent-simple",
		"/dependent-simple",
		"alpine:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "alpine:latest",
			Labels: map[string]string{
				"com.docker.compose.project":          "my-app-project",
				"com.docker.compose.service":          "dependent-simple",
				"com.docker.compose.container-number": "1",
				"com.docker.compose.depends_on":       "base:service_started:false",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	dependentNetwork := mockActions.CreateMockContainerWithConfig(
		"dependent-network",
		"/dependent-network",
		"alpine:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "alpine:latest",
			Labels: map[string]string{
				"com.docker.compose.project":          "my-app-project",
				"com.docker.compose.service":          "dependent-network",
				"com.docker.compose.container-number": "1",
				"com.docker.compose.depends_on":       "base:service_started:false",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)
	dependentNetwork.ContainerInfo().HostConfig.NetworkMode = "container:base"

	return &mockActions.TestData{
		Staleness: map[string]bool{
			base.Name():             true,
			dependentSimple.Name():  false,
			dependentNetwork.Name(): false,
		},
		Containers: []types.Container{base, dependentSimple, dependentNetwork},
	}
}

func createDependencyChain(names []string) []types.Container {
	containers := make([]types.Container, len(names))
	for i := range names {
		name := names[i]

		suffix := name
		if len(name) > 10 {
			suffix = name[10:]
		}

		image := "image-" + suffix + ":latest"

		labels := make(map[string]string)
		if i < len(names)-1 {
			labels["com.centurylinklabs.watchtower.depends-on"] = names[i+1]
		}

		containers[i] = mockActions.CreateMockContainerWithConfig(
			name,
			"/"+name,
			image,
			true,
			false,
			time.Now().AddDate(0, 0, -1),
			&dockerContainer.Config{
				Labels:       labels,
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
	}

	return containers
}

func getComposeDependsOnWithNetworkModeTestData() *mockActions.TestData {
	staleContainer := mockActions.CreateMockContainerWithConfig(
		"download-stack-vpn-1",
		"/download-stack-vpn-1",
		"gluetun:latest",
		true,
		false,
		time.Now().AddDate(0, 0, -1),
		&dockerContainer.Config{
			Image: "gluetun:latest",
			Labels: map[string]string{
				"com.docker.compose.project": "download-stack",
				"com.docker.compose.service": "vpn",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	dependentContainer := mockActions.CreateMockContainerWithConfig(
		"download-stack-web-1",
		"/download-stack-web-1",
		"nginx:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: "nginx:latest",
			Labels: map[string]string{
				"com.docker.compose.project":    "download-stack",
				"com.docker.compose.service":    "web",
				"com.docker.compose.depends_on": "vpn:service_started:false",
			},
			ExposedPorts: dockerNetwork.PortSet{},
		},
	)

	dependentContainer.ContainerInfo().HostConfig.NetworkMode = "container:download-stack-vpn-1"

	return &mockActions.TestData{
		Staleness:  map[string]bool{staleContainer.Name(): true, dependentContainer.Name(): false},
		Containers: []types.Container{staleContainer, dependentContainer},
	}
}
