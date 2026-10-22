package container

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerMount "github.com/moby/moby/api/types/mount"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// testContainerName is used for testing self-referencing dependency scenarios.
const testContainerName = "gluetun"

// webContainerName is used for testing filterSelfReferences scenarios.
const webContainerName = "web"

// TestContainerModel verifies the container model: configuration validation,
// the create and host configuration rebuilt from inspect data, label and
// metadata reads, the identifier used for dependencies, no-pull settings, and
// the links taken from labels, volumes-from, network_mode, and legacy links.
func TestContainerModel(t *testing.T) {
	t.Parallel()

	t.Run("Configuration Validation", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error when image info is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings())
			c.imageInfo = nil
			err := c.VerifyConfiguration()
			assert.Equal(t, errNoImageInfo, err)
		})

		t.Run("returns an error when container info is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings())
			c.containerInfo = nil
			err := c.VerifyConfiguration()
			assert.Equal(t, errNoContainerInfo, err)
		})

		t.Run("returns an error when config is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings())
			c.containerInfo.Config = nil
			err := c.VerifyConfiguration()
			assert.Equal(t, errInvalidConfig, err)
		})

		t.Run("returns an error when host config is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings())
			c.containerInfo.HostConfig = nil
			err := c.VerifyConfiguration()
			assert.Equal(t, errInvalidConfig, err)
		})

		t.Run("succeeds with no port bindings", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings())
			err := c.VerifyConfiguration()
			require.NoError(t, err)
		})

		t.Run("initializes exposed ports when nil with port bindings", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings("80/tcp"))
			c.containerInfo.Config.ExposedPorts = nil
			require.NoError(t, c.VerifyConfiguration())
			require.NotNil(t, c.containerInfo.Config.ExposedPorts)
			assert.Empty(t, c.containerInfo.Config.ExposedPorts)
		})

		t.Run("succeeds with non-nil exposed ports and port bindings", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings("80/tcp"))
			c.containerInfo.Config.ExposedPorts = map[dockerNetwork.Port]struct{}{
				dockerNetwork.MustParsePort("80/tcp"): {},
			}
			err := c.VerifyConfiguration()
			require.NoError(t, err)
		})

		t.Run("removes port bindings with empty port string", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings("80/tcp"))

			var emptyPort dockerNetwork.Port

			c.containerInfo.HostConfig.PortBindings[emptyPort] = []dockerNetwork.PortBinding{}
			c.containerInfo.Config.ExposedPorts = map[dockerNetwork.Port]struct{}{
				emptyPort:                             {},
				dockerNetwork.MustParsePort("80/tcp"): {},
			}
			err := c.VerifyConfiguration()
			require.NoError(t, err)

			_, exists := c.containerInfo.HostConfig.PortBindings[emptyPort]
			assert.False(t, exists)
			_, exists = c.containerInfo.HostConfig.PortBindings[dockerNetwork.MustParsePort("80/tcp")]
			assert.True(t, exists)
			_, exists = c.containerInfo.Config.ExposedPorts[emptyPort]
			assert.False(t, exists)
			_, exists = c.containerInfo.Config.ExposedPorts[dockerNetwork.MustParsePort("80/tcp")]
			assert.True(t, exists)
		})

		t.Run("removes port bindings with empty port number", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings("80/tcp"))

			var malformedPort dockerNetwork.Port

			c.containerInfo.HostConfig.PortBindings[malformedPort] = []dockerNetwork.PortBinding{}
			c.containerInfo.Config.ExposedPorts = map[dockerNetwork.Port]struct{}{
				malformedPort: {},
			}
			err := c.VerifyConfiguration()
			require.NoError(t, err)

			_, exists := c.containerInfo.HostConfig.PortBindings[malformedPort]
			assert.False(t, exists)
			_, exists = c.containerInfo.Config.ExposedPorts[malformedPort]
			assert.False(t, exists)
			_, exists = c.containerInfo.HostConfig.PortBindings[dockerNetwork.MustParsePort("80/tcp")]
			assert.True(t, exists)
		})

		t.Run("preserves valid port bindings unchanged", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithPortBindings("8080/tcp", "443/tcp"))
			c.containerInfo.Config.ExposedPorts = map[dockerNetwork.Port]struct{}{
				dockerNetwork.MustParsePort("8080/tcp"): {},
				dockerNetwork.MustParsePort("443/tcp"):  {},
			}
			err := c.VerifyConfiguration()
			require.NoError(t, err)
			require.Len(t, c.containerInfo.HostConfig.PortBindings, 2)
			require.Len(t, c.containerInfo.Config.ExposedPorts, 2)
		})
	})

	t.Run("Create Configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("when container and image healthcheck configs are identical", func(t *testing.T) {
			t.Parallel()

			t.Run("returns an empty healthcheck config", func(t *testing.T) {
				t.Parallel()

				tests := []dockerContainer.HealthConfig{
					{Test: []string{"/usr/bin/sleep", "1s"}},
					{Timeout: 30},
					{StartPeriod: 30},
					{Retries: 30},
				}
				for _, healthConfig := range tests {
					c := MockContainer(
						WithHealthcheck(healthConfig),
						WithImageHealthcheck(healthConfig),
					)
					assert.Equal(t, &dockerContainer.HealthConfig{}, c.GetCreateConfig().Healthcheck)
				}
			})
		})

		t.Run("returns container healthcheck when configs differ", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(
				WithHealthcheck(dockerContainer.HealthConfig{
					Test:        []string{"/usr/bin/sleep", "1s"},
					Interval:    30,
					Timeout:     30,
					StartPeriod: 10,
					Retries:     2,
				}),
				WithImageHealthcheck(dockerContainer.HealthConfig{
					Test:        []string{"/usr/bin/sleep", "10s"},
					Interval:    10,
					Timeout:     60,
					StartPeriod: 30,
					Retries:     10,
				}),
			)
			assert.Equal(t, &dockerContainer.HealthConfig{
				Test:        []string{"/usr/bin/sleep", "1s"},
				Interval:    30,
				Timeout:     30,
				StartPeriod: 10,
				Retries:     2,
			}, c.GetCreateConfig().Healthcheck)
		})

		t.Run("handles empty container healthcheck config without panic", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithImageHealthcheck(dockerContainer.HealthConfig{
				Test:        []string{"/usr/bin/sleep", "10s"},
				Interval:    10,
				Timeout:     60,
				StartPeriod: 30,
				Retries:     10,
			}))
			assert.Nil(t, c.GetCreateConfig().Healthcheck)
		})

		t.Run("handles empty image healthcheck config without panic", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithHealthcheck(dockerContainer.HealthConfig{
				Test:        []string{"/usr/bin/sleep", "1s"},
				Interval:    30,
				Timeout:     30,
				StartPeriod: 10,
				Retries:     2,
			}))
			assert.Equal(t, &dockerContainer.HealthConfig{
				Test:        []string{"/usr/bin/sleep", "1s"},
				Interval:    30,
				Timeout:     30,
				StartPeriod: 10,
				Retries:     2,
			}, c.GetCreateConfig().Healthcheck)
		})

		t.Run("UTS mode hostname handling", func(t *testing.T) {
			t.Parallel()

			t.Run("clears hostname when UTS mode is non-empty", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(
					WithUTSMode("host"),
					WithHostname("test-hostname"),
				)
				config := c.GetCreateConfig()
				assert.Empty(t, config.Hostname, "Hostname should be cleared when UTS mode is set")
			})

			t.Run("preserves hostname when UTS mode is empty", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(
					WithUTSMode(""),
					WithHostname("test-hostname"),
				)
				config := c.GetCreateConfig()
				assert.Equal(t, "test-hostname", config.Hostname, "Hostname should be preserved when UTS mode is empty")
			})
		})

		t.Run("uses container config when image config is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithImageName("ghcr.io/home-assistant/home-assistant:latest"))
			c.imageInfo.Config = nil
			c.containerInfo.Config.WorkingDir = "/config"
			c.containerInfo.Config.Env = []string{"TZ=Europe/Amsterdam"}

			config := c.GetCreateConfig()

			assert.Equal(t, "ghcr.io/home-assistant/home-assistant:latest", config.Image)
			assert.Equal(t, "/config", config.WorkingDir)
			assert.Equal(t, []string{"TZ=Europe/Amsterdam"}, config.Env)
		})

		t.Run("returns minimal config when containerInfo is nil", func(t *testing.T) {
			t.Parallel()

			c := MockContainer(WithImageName("test-image"))
			c.containerInfo = nil
			config := c.GetCreateConfig()
			assert.Equal(t, "unknown:latest", config.Image)
			assert.Equal(t, &dockerContainer.Config{Image: "unknown:latest"}, config)
		})
	})

	t.Run("Metadata Retrieval", func(t *testing.T) {
		t.Parallel()

		// newMetadataContainer returns an enabled Watchtower container.
		newMetadataContainer := func() *Container {
			return MockContainer(WithLabels(map[string]string{
				"com.centurylinklabs.watchtower.enable": "true",
				"com.centurylinklabs.watchtower":        "true",
			}))
		}

		t.Run("returns correct container name", func(t *testing.T) {
			t.Parallel()

			container := newMetadataContainer()
			name := container.Name()
			assert.Equal(t, "test-watchtower", name)
			assert.NotEqual(t, "wrong-name", name)
		})

		t.Run("returns correct container ID", func(t *testing.T) {
			t.Parallel()

			container := newMetadataContainer()
			id := container.ID()
			assert.EqualValues(t, "container_id", id)
			assert.NotEqualValues(t, "wrong-id", id)
		})

		t.Run("returns true for enabled label when set to true", func(t *testing.T) {
			t.Parallel()

			container := newMetadataContainer()
			enabled, exists := container.Enabled()
			assert.True(t, enabled)
			assert.True(t, exists)
		})

		t.Run("returns false and true for enabled label when set to false", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.centurylinklabs.watchtower.enable": "false",
			}))
			enabled, exists := container.Enabled()
			assert.False(t, enabled)
			assert.True(t, exists)
		})

		t.Run("returns false and false when enabled label is not set", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{"lol": "false"}))
			enabled, exists := container.Enabled()
			assert.False(t, enabled)
			assert.False(t, exists)
		})

		t.Run("returns false and false for invalid enabled label", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.centurylinklabs.watchtower.enable": "falsy",
			}))
			enabled, exists := container.Enabled()
			assert.False(t, enabled)
			assert.False(t, exists)
		})

		t.Run("checking Watchtower instance", func(t *testing.T) {
			t.Parallel()

			t.Run("returns true when Watchtower label is true", func(t *testing.T) {
				t.Parallel()

				container := newMetadataContainer()
				isWatchtower := container.IsWatchtower()
				assert.True(t, isWatchtower)
			})

			t.Run("returns false when Watchtower label is false", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower": "false",
				}))
				isWatchtower := container.IsWatchtower()
				assert.False(t, isWatchtower)
			})

			t.Run("returns false when Watchtower label is not set", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{"funny.label": "false"}))
				isWatchtower := container.IsWatchtower()
				assert.False(t, isWatchtower)
			})

			t.Run("returns false when no labels are set", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{}))
				isWatchtower := container.IsWatchtower()
				assert.False(t, isWatchtower)
			})
		})

		t.Run("fetching stop signal", func(t *testing.T) {
			t.Parallel()

			t.Run("returns signal when set", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.stop-signal": "SIGKILL",
				}))
				assert.Equal(t, "SIGKILL", container.StopSignal())
			})

			t.Run("returns SIGTERM when signal is not set", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{}))
				assert.Equal(t, "SIGTERM", container.StopSignal())
			})
		})

		t.Run("fetching image name", func(t *testing.T) {
			t.Parallel()

			t.Run("uses zodiac label when present", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.zodiac.original-image": "the-original-image",
				}))
				assert.Equal(t, "the-original-image:latest", container.ImageName())
			})

			t.Run("returns image name from config", func(t *testing.T) {
				t.Parallel()

				name := "image-name:3"
				container := MockContainer(WithImageName(name))
				assert.Equal(t, name, container.ImageName())
			})

			t.Run("appends latest tag when no tag is supplied", func(t *testing.T) {
				t.Parallel()

				name := "image-name"
				container := MockContainer(WithImageName(name))
				assert.Equal(t, name+":latest", container.ImageName())
			})

			t.Run("returns unknown:latest for a nil receiver", func(t *testing.T) {
				t.Parallel()

				var nilContainer *Container

				assert.Equal(t, "unknown:latest", nilContainer.ImageName())
			})
		})

		t.Run("setting image name", func(t *testing.T) {
			t.Parallel()

			t.Run("is a no-op on a nil receiver", func(t *testing.T) {
				t.Parallel()

				var nilContainer *Container

				assert.NotPanics(t, func() {
					nilContainer.SetImageName("app:v2")
				})
			})

			t.Run("pins a tagged name on config and cache", func(t *testing.T) {
				t.Parallel()

				container := NewContainer(nopLog(), &dockerContainer.InspectResponse{
					Name: "/app",
					Config: &dockerContainer.Config{
						Image: "app:v1",
					},
				}, nil)

				container.SetImageName("app:v2")

				require.NotNil(t, container.ContainerInfo())
				assert.Equal(t, "app:v2", container.ContainerInfo().Config.Image)
				assert.Equal(t, "app:v2", container.ImageName())
			})

			t.Run("appends latest when the pinned name is untagged", func(t *testing.T) {
				t.Parallel()

				container := NewContainer(nopLog(), &dockerContainer.InspectResponse{
					Name: "/app",
					Config: &dockerContainer.Config{
						Image: "app:v1",
					},
				}, nil)

				container.SetImageName("app")

				assert.Equal(t, "app:latest", container.ContainerInfo().Config.Image)
				assert.Equal(t, "app:latest", container.ImageName())
			})

			t.Run("appends latest without treating a registry port as a tag", func(t *testing.T) {
				t.Parallel()

				container := NewContainer(nopLog(), &dockerContainer.InspectResponse{
					Name: "/app",
					Config: &dockerContainer.Config{
						Image: "app:v1",
					},
				}, nil)

				container.SetImageName("registry.example:5000/app")

				assert.Equal(t, "registry.example:5000/app:latest", container.ContainerInfo().Config.Image)
				assert.Equal(t, "registry.example:5000/app:latest", container.ImageName())
			})
		})

		t.Run("fetching image ID", func(t *testing.T) {
			t.Parallel()

			t.Run("returns image ID when imageInfo is available", func(t *testing.T) {
				t.Parallel()

				container := newMetadataContainer()
				imageID := container.ImageID()
				assert.Equal(t, types.ImageID("image_id"), imageID)
			})

			t.Run("returns empty string for ImageID when imageInfo is nil", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithPortBindings())
				container.imageInfo = nil
				imageID := container.ImageID()
				assert.Equal(t, types.ImageID(""), imageID)
			})
		})

		t.Run("fetching container links", func(t *testing.T) {
			t.Parallel()

			t.Run("compose depends-on label is present", func(t *testing.T) {
				t.Parallel()

				t.Run("returns single dependent container", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "postgres",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					require.Len(t, links, 1)
				})

				t.Run("returns multiple dependent containers", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "postgres,redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
					require.Len(t, links, 2)
				})

				t.Run("trims whitespace from service names", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": " postgres , redis ",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
					require.Len(t, links, 2)
				})

				t.Run("normalizes container names with slashes", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						compose.ComposeDependsOnLabel: "/postgres,/redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
				})

				t.Run("watchtower depends-on label takes precedence over compose depends_on", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on":             "postgres",
						"com.centurylinklabs.watchtower.depends-on": "redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "redis")
					assert.NotContains(t, links, "postgres")
					require.Len(t, links, 1)
				})

				t.Run("returns empty links for blank compose depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "",
					}))
					assert.Empty(t, container.Links(true))
				})

				t.Run("parses colon-separated service:condition:required format", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "postgres:service_started:required,redis:service_healthy",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
					require.Len(t, links, 2)
				})
			})

			t.Run("depends-on label is present", func(t *testing.T) {
				t.Parallel()

				t.Run("returns single dependent container", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					require.Len(t, links, 1)
				})

				t.Run("returns multiple dependent containers", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres,redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
					require.Len(t, links, 2)
				})

				t.Run("normalizes container names with slashes", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "/postgres,/redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
				})

				t.Run("returns empty links for blank depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "",
					}))
					assert.Empty(t, container.Links(true))
				})

				t.Run("does not prefix container names with project name for cross-project dependencies", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.project":                "myproject",
						"com.centurylinklabs.watchtower.depends-on": "otherproject-db,external-service",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "otherproject-db")
					assert.Contains(t, links, "external-service")
					require.Len(t, links, 2)
					assert.NotContains(t, links, "myproject-otherproject-db")
					assert.NotContains(t, links, "myproject-external-service")
				})

				t.Run("handles cross-project dependencies with single container", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.project":                "webapp",
						"com.centurylinklabs.watchtower.depends-on": "database",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "database")
					require.Len(t, links, 1)
					assert.NotContains(t, links, "webapp-database")
				})

				t.Run("supports cross-project dependencies without project labels", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "standalone-db,external-api",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "standalone-db")
					assert.Contains(t, links, "external-api")
					require.Len(t, links, 2)
				})

				t.Run("handles self-referencing dependencies gracefully", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres,test-watchtower,redis",
					}))
					links := container.Links(true)
					assert.Contains(t, links, "postgres")
					assert.Contains(t, links, "redis")
					assert.NotContains(t, links, "test-watchtower")
					require.Len(t, links, 2)
				})

				t.Run("does not create self-reference when container name matches dependency", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": testContainerName,
					}))
					container.containerInfo.Name = testContainerName
					container.normalizedName = testContainerName

					links := container.Links(true)
					assert.Empty(t, links, "Container with name matching dependency should not include self in links")
				})

				t.Run("filters self-references from compose depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": testContainerName,
					}))
					container.containerInfo.Name = testContainerName
					container.normalizedName = testContainerName

					links := container.Links(true)
					assert.Empty(t, links, "Compose depends-on with self-reference should be filtered out")
				})

				t.Run("parses malformed watchtower labels", func(t *testing.T) {
					t.Parallel()

					tests := []struct {
						name     string
						label    string
						expected []string
					}{
						{name: "empty entries", label: ",postgres,,redis,", expected: []string{"postgres", "redis"}},
						{name: "extra spaces", label: " postgres , redis ", expected: []string{"postgres", "redis"}},
						{name: "empty string", label: "", expected: []string{}},
						{name: "multiple dependencies", label: "postgres,redis,mysql", expected: []string{"postgres", "redis", "mysql"}},
					}

					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							t.Parallel()

							label, expected := tt.label, tt.expected

							container := MockContainer(WithLabels(map[string]string{
								"com.centurylinklabs.watchtower.depends-on": label,
							}))
							links := container.Links(true)
							assert.Equal(t, expected, links)
						})
					}
				})

				t.Run("normalizes invalid container name dependencies", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres db, redis",
					}))
					links := container.Links(true)
					assert.Equal(t, []string{"postgres db", "redis"}, links)
				})

				t.Run("handles very long dependency lists", func(t *testing.T) {
					t.Parallel()

					var deps []string
					for i := 1; i <= 100; i++ {
						deps = append(deps, fmt.Sprintf("dep%d", i))
					}

					label := strings.Join(deps, ",")
					container := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": label,
					}))
					links := container.Links(true)
					require.Len(t, links, 100)
					assert.Contains(t, links, "dep1")
					assert.Contains(t, links, "dep50")
					assert.Contains(t, links, "dep100")
				})
			})

			t.Run("when UseComposeDependsOn is disabled", func(t *testing.T) {
				t.Parallel()

				t.Run("returns empty links for compose depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "postgres,redis",
					}))
					links := container.Links(false)
					assert.Empty(t, links, "Compose depends-on should be ignored when disabled")
				})

				t.Run("still returns links from watchtower depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on":             "postgres",
						"com.centurylinklabs.watchtower.depends-on": "redis",
					}))
					links := container.Links(false)
					assert.Contains(t, links, "redis")
					assert.NotContains(t, links, "postgres")
					require.Len(t, links, 1)
				})

				t.Run("returns links from host config when compose is disabled", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(WithLabels(map[string]string{
						"com.docker.compose.depends_on": "postgres",
					}), WithLinks([]string{
						"redis:test-watchtower",
					}))
					links := container.Links(false)
					assert.Contains(t, links, "redis")
					assert.NotContains(t, links, "postgres")
					require.Len(t, links, 1)
				})
			})

			t.Run("returns links from host config when depends-on labels are absent", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLinks([]string{
					"redis:test-watchtower",
					"postgres:test-watchtower",
				}))
				links := container.Links(true)
				assert.Contains(t, links, "redis")
				assert.Contains(t, links, "postgres")
				require.Len(t, links, 2)
			})

			t.Run("warns and skips invalid link format without colon", func(t *testing.T) {
				t.Parallel()

				logOutput := &bytes.Buffer{}
				w := logging.LogfmtWriter(logOutput)
				l := zerolog.New(w).Level(zerolog.DebugLevel).With().Timestamp().Logger()
				container := MockContainer(WithLinks([]string{"invalidlink"}))
				container.log = &l
				links := container.Links(true)
				assert.Empty(t, links)
				assert.Contains(t, logOutput.String(), "Invalid link format in host config, expected 'name:alias'")
			})

			t.Run("warns and skips link with empty container name", func(t *testing.T) {
				t.Parallel()

				logOutput := &bytes.Buffer{}
				w := logging.LogfmtWriter(logOutput)
				l := zerolog.New(w).Level(zerolog.DebugLevel).With().Timestamp().Logger()
				container := MockContainer(WithLinks([]string{":alias"}))
				container.log = &l
				links := container.Links(true)
				assert.Empty(t, links)
				assert.Contains(t, logOutput.String(), "Invalid link format in host config, missing container name")
			})

			t.Run("normalizes container names with leading slashes", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLinks([]string{"/redis:test-watchtower"}))
				links := container.Links(true)
				assert.Contains(t, links, "redis")
			})

			t.Run("does not prefix already prefixed container names", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(
					WithLinks([]string{"myproject-redis:test-watchtower"}),
					WithLabels(map[string]string{"com.docker.compose.project": "myproject"}),
				)
				links := container.Links(true)
				assert.Contains(t, links, "myproject-redis")
			})

			t.Run("does not prefix when project name is empty", func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithLinks([]string{"redis:test-watchtower"}))
				links := container.Links(true)
				assert.Contains(t, links, "redis")
			})

			t.Run("network mode", func(t *testing.T) {
				t.Parallel()

				t.Run("includes network mode dependencies", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithNetworkMode("container:other"),
						WithLabels(map[string]string{"com.docker.compose.project": "myproject"}),
					)
					links := container.Links(true)
					assert.Contains(t, links, "other")
					assert.NotContains(t, links, "myproject-other")
				})

				t.Run("includes the bare network mode container name", func(t *testing.T) {
					t.Parallel()

					// The network mode holds a real container name, including when
					// that container belongs to another Compose project.
					container := MockContainer(
						WithNetworkMode("container:gluetun"),
						WithLabels(map[string]string{"com.docker.compose.project": "qbittorrent"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"gluetun"}, links)
				})

				t.Run("strips a leading slash from the network mode container name", func(t *testing.T) {
					t.Parallel()

					// Inspect rewrite stores container: plus the Docker name, which
					// includes a leading slash.
					container := MockContainer(
						WithNetworkMode("container:/gluetun"),
						WithLabels(map[string]string{"com.docker.compose.project": "qbittorrent"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"gluetun"}, links)
				})

				t.Run("does not double-prefix an already project-qualified network mode name", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithNetworkMode("container:gluetun-vpn-1"),
						WithLabels(map[string]string{"com.docker.compose.project": "gluetun"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"gluetun-vpn-1"}, links)
				})

				t.Run("does not prefix a network mode container ID", func(t *testing.T) {
					t.Parallel()

					// Moby inspect stores container:<id> after create. Prefixing
					// that ID with the dependent's project would never match.
					const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

					container := MockContainer(
						WithNetworkMode("container:"+providerID),
						WithLabels(map[string]string{"com.docker.compose.project": "qbittorrent"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{providerID}, links)
				})
			})

			t.Run("volumes-from", func(t *testing.T) {
				t.Parallel()

				t.Run("includes volumes-from container names without a project prefix", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithVolumesFrom([]string{"app:rw"}),
						WithLabels(map[string]string{"com.docker.compose.project": "nextcloud"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"app"}, links)
				})

				t.Run("includes volumes-from container IDs without a project prefix", func(t *testing.T) {
					t.Parallel()

					const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

					container := MockContainer(
						WithVolumesFrom([]string{appID}),
						WithLabels(map[string]string{"com.docker.compose.project": "nextcloud"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{appID}, links)
				})

				t.Run("strips a leading slash from a volumes-from container name", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithVolumesFrom([]string{"/nextcloud-app:ro"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"nextcloud-app"}, links)
				})

				t.Run("includes multiple volumes-from identities", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithVolumesFrom([]string{"app:rw", "db:ro"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"app", "db"}, links)
				})

				t.Run("skips empty volumes-from specs", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithVolumesFrom([]string{"", "app:rw"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"app"}, links)
				})

				t.Run("does not treat a non-mode colon suffix as a volumes-from mode", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithVolumesFrom([]string{"app:latest"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"app:latest"}, links)
				})

				t.Run("includes volumes-from along with a watchtower depends-on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithLabels(map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "redis",
						}),
						WithVolumesFrom([]string{"app:rw"}),
					)
					links := container.Links(true)
					assert.Equal(t, []string{"redis", "app"}, links)
				})

				t.Run("includes volumes-from along with a compose depends_on label", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithLabels(map[string]string{
							"com.docker.compose.depends_on": "postgres",
						}),
						WithVolumesFrom([]string{"app:rw"}),
					)
					links := container.Links(true)
					assert.Equal(t, []string{"postgres", "app"}, links)
				})

				t.Run("does not duplicate a volumes-from identity already listed by depends-on", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithLabels(map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "app",
						}),
						WithVolumesFrom([]string{"app:rw"}),
					)
					links := container.Links(true)
					assert.Equal(t, []string{"app"}, links)
				})

				t.Run("includes network mode and volumes-from identities together", func(t *testing.T) {
					t.Parallel()

					container := MockContainer(
						WithNetworkMode("container:gluetun"),
						WithVolumesFrom([]string{"app:rw"}),
						WithLabels(map[string]string{"com.docker.compose.project": "qbittorrent"}),
					)
					links := container.Links(true)
					assert.ElementsMatch(t, []string{"gluetun", "app"}, links)
				})
			})
		})

		t.Run("returns pre and post update timeout values", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout":  "3",
				"com.centurylinklabs.watchtower.lifecycle.post-update-timeout": "5",
			}))
			assert.Equal(t, 3, container.PreUpdateTimeout())
			assert.Equal(t, 5, container.PostUpdateTimeout())
		})
	})

	t.Run("ResolveContainerIdentifier", func(t *testing.T) {
		t.Parallel()

		t.Run("returns service name when compose service label is present", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.docker.compose.service": "web-service",
			}))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "web-service", identifier)
		})

		t.Run("returns container name when compose service label is absent", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"other.label": "value",
			}))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "test-watchtower", identifier)
		})

		t.Run("returns container name when labels are empty", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{}))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "test-watchtower", identifier)
		})

		t.Run("returns container name when labels are nil", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(nil))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "test-watchtower", identifier)
		})

		t.Run("returns service name when compose service label has value", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.docker.compose.service": "api",
			}))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "api", identifier)
		})

		t.Run("handles multiple labels with service name present", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{
				"com.docker.compose.service":     "db-service",
				"com.docker.compose.project":     "myproject",
				"com.centurylinklabs.watchtower": "true",
			}))
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "myproject-db-service", identifier)
		})

		t.Run("returns container name when ContainerInfo returns nil", func(t *testing.T) {
			t.Parallel()

			container := mockTypes.NewMockContainer(t)
			container.EXPECT().ContainerInfo().Return(nil)
			container.EXPECT().Name().Return("test-watchtower")
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "test-watchtower", identifier)
		})

		t.Run("returns container name when ContainerInfo.Config is nil", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithLabels(map[string]string{}))
			container.containerInfo.Config = nil
			identifier := ResolveContainerIdentifier(container)
			assert.Equal(t, "test-watchtower", identifier)
		})

		t.Run("returns container name for replica containers", func(t *testing.T) {
			t.Parallel()

			mockContainer := mockTypes.NewMockContainer(t)
			mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service":     "web",
						"com.docker.compose.project":     "myproject",
						"com.docker.compose.version":     "3.8",
						"com.docker.compose.config-hash": "abc123",
					},
				},
			})
			mockContainer.EXPECT().Name().Return("myproject-web-1")
			result := ResolveContainerIdentifier(mockContainer)
			assert.Equal(t, "myproject-web-1", result)
		})

		t.Run("replica container identifiers", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				title       string
				name        string
				labels      map[string]string
				expected    string
				description string
			}{
				{title: "single replica returns unique name", name: "myproject-web-1", labels: map[string]string{
					"com.docker.compose.service":          "web",
					"com.docker.compose.project":          "myproject",
					"com.docker.compose.container-number": "1",
				}, expected: "myproject-web-1", description: "Replica should return full name to ensure uniqueness"},
				{title: "multiple replicas with different numbers", name: "myproject-web-2", labels: map[string]string{
					"com.docker.compose.service":          "web",
					"com.docker.compose.project":          "myproject",
					"com.docker.compose.container-number": "2",
				}, expected: "myproject-web-2", description: "Each replica should have unique identifier"},
				{title: "replica without container number label", name: "app-db-3", labels: map[string]string{
					"com.docker.compose.service": "db",
					"com.docker.compose.project": "app",
				}, expected: "app-db-3", description: "Should return full name even without container number to avoid collisions"},
				{title: "non-replica with container number", name: "mydb", labels: map[string]string{
					"com.docker.compose.service":          "db",
					"com.docker.compose.project":          "my",
					"com.docker.compose.container-number": "1",
				}, expected: "my-db-1", description: "Non-replica should use project-service-number format"},
				{title: "replica with missing service label", name: "myproject-orphan-1", labels: map[string]string{
					"com.docker.compose.project": "myproject",
				}, expected: "myproject-orphan-1", description: "Container with project prefix but no service should return full name"},
				{title: "container with service but no project", name: "unknown-web-2", labels: map[string]string{
					"com.docker.compose.service": "web",
				}, expected: "web", description: "Non-replica container with service label returns service name"},
				{title: "collision prevention for replicas", name: "test-app-1", labels: map[string]string{
					"com.docker.compose.service":          "app",
					"com.docker.compose.project":          "test",
					"com.docker.compose.container-number": "1",
				}, expected: "test-app-1", description: "Should return unique name instead of potentially colliding 'test-app'"},
				{title: "collision prevention for replicas with same number", name: "prod-api-1", labels: map[string]string{
					"com.docker.compose.service":          "api",
					"com.docker.compose.project":          "prod",
					"com.docker.compose.container-number": "1",
				}, expected: "prod-api-1", description: "Multiple replicas with same number should still be unique"},
			}

			for _, tt := range tests {
				t.Run(tt.title, func(t *testing.T) {
					t.Parallel()

					name, labels, expected, description := tt.name, tt.labels, tt.expected, tt.description

					container := MockContainer(WithLabels(labels))
					container.containerInfo.Name = name
					container.normalizedName = name
					result := ResolveContainerIdentifier(container)
					assert.Equal(t, expected, result, "Test case: %s", description)
				})
			}
		})

		t.Run("replica collision scenarios", func(t *testing.T) {
			t.Parallel()

			t.Run("prevents collision between replicas without unique identifiers", func(t *testing.T) {
				t.Parallel()

				replica1 := mockTypes.NewMockContainer(t)
				replica1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "service",
							"com.docker.compose.project": "myapp",
						},
					},
				})
				replica1.EXPECT().Name().Return("myapp-service-1")

				replica2 := mockTypes.NewMockContainer(t)
				replica2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service": "service",
							"com.docker.compose.project": "myapp",
						},
					},
				})
				replica2.EXPECT().Name().Return("myapp-service-2")

				id1 := ResolveContainerIdentifier(replica1)
				id2 := ResolveContainerIdentifier(replica2)

				assert.Equal(t, "myapp-service-1", id1)
				assert.Equal(t, "myapp-service-2", id2)
				assert.NotEqual(t, id2, id1)
			})

			t.Run("handles replicas with and without container number labels", func(t *testing.T) {
				t.Parallel()

				replicaWithNumber := mockTypes.NewMockContainer(t)
				replicaWithNumber.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{
							"com.docker.compose.service":          "worker",
							"com.docker.compose.project":          "queue",
							"com.docker.compose.container-number": "5",
						},
					},
				})
				replicaWithNumber.EXPECT().Name().Return("queue-worker-5")

				replicaWithoutNumber := mockTypes.NewMockContainer(t)
				replicaWithoutNumber.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{
						Config: &dockerContainer.Config{
							Labels: map[string]string{
								"com.docker.compose.service": "worker",
								"com.docker.compose.project": "queue",
							},
						},
					})
				replicaWithoutNumber.EXPECT().Name().Return("queue-worker-6")

				idWith := ResolveContainerIdentifier(replicaWithNumber)
				idWithout := ResolveContainerIdentifier(replicaWithoutNumber)

				assert.Equal(t, "queue-worker-5", idWith)
				assert.Equal(t, "queue-worker-6", idWithout)
			})
		})

		t.Run("filterSelfReferences", func(t *testing.T) {
			t.Parallel()

			t.Run("filters out basic self-references from links", func(t *testing.T) {
				t.Parallel()

				links := []string{"db", webContainerName, "cache"}
				containerName := webContainerName
				result := filterSelfReferences(links, containerName)
				assert.Equal(t, []string{"db", "cache"}, result)
			})

			t.Run("filters out multiple self-references", func(t *testing.T) {
				t.Parallel()

				links := []string{"first", webContainerName, "second", webContainerName, "third"}
				containerName := webContainerName
				result := filterSelfReferences(links, containerName)
				assert.Equal(t, []string{"first", "second", "third"}, result)
			})

			t.Run("returns empty slice when all links are self-references", func(t *testing.T) {
				t.Parallel()

				links := []string{webContainerName}
				containerName := webContainerName
				result := filterSelfReferences(links, containerName)
				assert.Empty(t, result)
			})

			t.Run("preserves order of non-self links", func(t *testing.T) {
				t.Parallel()

				links := []string{"db", webContainerName, "cache", webContainerName, "redis"}
				containerName := webContainerName
				result := filterSelfReferences(links, containerName)
				assert.Equal(t, []string{"db", "cache", "redis"}, result)
			})

			t.Run("returns original slice unchanged when no self-references exist", func(t *testing.T) {
				t.Parallel()

				links := []string{"db", "cache"}
				containerName := webContainerName
				result := filterSelfReferences(links, containerName)
				assert.Equal(t, []string{"db", "cache"}, result)
			})
		})
	})

	t.Run("No-Pull Configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("no-pull argument is not set", func(t *testing.T) {
			t.Parallel()

			t.Run("returns true when no-pull label is true", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.no-pull": "true",
				}))
				assert.True(t, c.IsNoPull(types.UpdateParams{}))
			})

			t.Run("returns false when no-pull label is false", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.no-pull": "false",
				}))
				assert.False(t, c.IsNoPull(types.UpdateParams{}))
			})

			t.Run("returns false for invalid no-pull label", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.no-pull": "maybe",
				}))
				assert.False(t, c.IsNoPull(types.UpdateParams{}))
			})

			t.Run("returns false when no-pull label is unset", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{}))
				assert.False(t, c.IsNoPull(types.UpdateParams{}))
			})
		})

		t.Run("no-pull argument is true", func(t *testing.T) {
			t.Parallel()

			t.Run("returns true when no-pull label is true", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.no-pull": "true",
				}))
				assert.True(t, c.IsNoPull(types.UpdateParams{NoPull: true}))
			})

			t.Run("returns true when no-pull label is false", func(t *testing.T) {
				t.Parallel()

				c := MockContainer(WithLabels(map[string]string{
					"com.centurylinklabs.watchtower.no-pull": "false",
				}))
				assert.True(t, c.IsNoPull(types.UpdateParams{NoPull: true}))
			})

			t.Run("label-take-precedence is true", func(t *testing.T) {
				t.Parallel()

				t.Run("returns true when no-pull label is true", func(t *testing.T) {
					t.Parallel()

					c := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.no-pull": "true",
					}))
					assert.True(t, c.IsNoPull(types.UpdateParams{LabelPrecedence: true, NoPull: true}))
				})

				t.Run("returns false when no-pull label is false", func(t *testing.T) {
					t.Parallel()

					c := MockContainer(WithLabels(map[string]string{
						"com.centurylinklabs.watchtower.no-pull": "false",
					}))
					assert.False(t, c.IsNoPull(types.UpdateParams{LabelPrecedence: true, NoPull: true}))
				})
			})
		})
	})

	t.Run("Host Config Creation", func(t *testing.T) {
		t.Parallel()

		t.Run("preserves volume mount subpath in host config", func(t *testing.T) {
			t.Parallel()

			volumeMount := dockerMount.Mount{
				Type:   dockerMount.TypeVolume,
				Source: "test_volume",
				Target: "/config/nest",
				VolumeOptions: &dockerMount.VolumeOptions{
					Subpath: "ha/nest",
				},
			}

			container := MockContainer(WithMounts([]dockerMount.Mount{volumeMount}))
			hostConfig := container.GetCreateHostConfig()

			require.Len(t, hostConfig.Mounts, 1, "Expected exactly one mount")
			mount := hostConfig.Mounts[0]
			assert.Equal(t, dockerMount.TypeVolume, mount.Type, "Mount type should be volume")
			assert.Equal(t, "test_volume", mount.Source, "Mount source should match")
			assert.Equal(t, "/config/nest", mount.Target, "Mount target should match")
			require.NotNil(t, mount.VolumeOptions, "VolumeOptions should be set")
			assert.Equal(t, "ha/nest", mount.VolumeOptions.Subpath, "Subpath should be preserved")
		})

		t.Run("defaults empty device CgroupPermissions to 'rwm' for Podman compatibility", func(t *testing.T) {
			t.Parallel()

			devices := []dockerContainer.DeviceMapping{
				{
					PathOnHost:        "/dev/net/tun",
					PathInContainer:   "/dev/net/tun",
					CgroupPermissions: "", // empty, as returned by Podman inspect
				},
				{
					PathOnHost:        "/dev/dri/renderD128",
					PathInContainer:   "/dev/dri/renderD128",
					CgroupPermissions: "rw", // explicit value should be preserved
				},
			}

			container := MockContainer(WithDevices(devices))
			hostConfig := container.GetCreateHostConfig()

			require.Len(t, hostConfig.Devices, 2)

			// Empty permissions must be defaulted (this fixes the Podman "empty device mode" error)
			assert.Equal(t, "/dev/net/tun", hostConfig.Devices[0].PathOnHost)
			assert.Equal(t, "rwm", hostConfig.Devices[0].CgroupPermissions, "empty CgroupPermissions should default to 'rwm'")

			// Explicit permissions must be left untouched
			assert.Equal(t, "/dev/dri/renderD128", hostConfig.Devices[1].PathOnHost)
			assert.Equal(t, "rw", hostConfig.Devices[1].CgroupPermissions, "explicit CgroupPermissions must be preserved")
		})

		t.Run("returns a copy of VolumesFrom so create config does not alias inspect data", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(WithVolumesFrom([]string{"app:rw"}))
			hostConfig := container.GetCreateHostConfig()
			hostConfig.VolumesFrom[0] = "mutated"

			assert.Equal(t, []string{"app:rw"}, container.ContainerInfo().HostConfig.VolumesFrom)
		})
	})

	t.Run("parseVolumesFromSpec", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			spec     string
			wantName string
			wantMode string
		}{
			{name: "empty spec", spec: "", wantName: "", wantMode: ""},
			{name: "bare name", spec: "app", wantName: "app", wantMode: ""},
			{name: "name with rw", spec: "app:rw", wantName: "app", wantMode: "rw"},
			{name: "name with ro", spec: "app:ro", wantName: "app", wantMode: "ro"},
			{name: "name with SELinux z", spec: "app:z", wantName: "app", wantMode: "z"},
			{name: "name with SELinux Z", spec: "app:Z", wantName: "app", wantMode: "Z"},
			{name: "combined ro and z", spec: "app:ro,z", wantName: "app", wantMode: "ro,z"},
			{
				name:     "full container ID",
				spec:     "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7",
				wantName: "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7",
				wantMode: "",
			},
			{
				name:     "full container ID with rw",
				spec:     "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7:rw",
				wantName: "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7",
				wantMode: "rw",
			},
			{name: "non-mode suffix stays part of the name", spec: "app:latest", wantName: "app:latest", wantMode: ""},
			{name: "trailing colon is not a mode", spec: "app:", wantName: "app:", wantMode: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				spec, wantName, wantMode := tt.spec, tt.wantName, tt.wantMode

				name, mode := parseVolumesFromSpec(spec)
				assert.Equal(t, wantName, name)
				assert.Equal(t, wantMode, mode)
			})
		}
	})
}

// TestContainer_Links_NoConfig verifies that a container without inspect
// config data still reports its host-config dependencies instead of panicking.
func TestContainer_Links_NoConfig(t *testing.T) {
	t.Parallel()

	c := &Container{
		normalizedName: "app",
		containerInfo: &dockerContainer.InspectResponse{
			Name: "/app",
			HostConfig: &dockerContainer.HostConfig{
				NetworkMode: "container:vpn",
			},
		},
	}

	assert.Equal(t, []string{"vpn"}, c.Links(false))
}

// TestContainer_Links_DoesNotLog verifies that reading a container's
// dependencies writes no log entries, even at debug level. The dependency
// graph logs each container's dependencies once per scan instead.
func TestContainer_Links_DoesNotLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		labels              map[string]string
		networkMode         dockerContainer.NetworkMode
		useComposeDependsOn bool
		want                []string
	}{
		{
			name:   "depends-on label",
			labels: map[string]string{dependsOnLabel: "db, cache"},
			want:   []string{"db", "cache"},
		},
		{
			name: "compose depends_on label",
			labels: map[string]string{
				compose.ComposeDependsOnLabel: "db:service_started:false",
				compose.ComposeProjectLabel:   "shop",
			},
			useComposeDependsOn: true,
			want:                []string{"shop-db"},
		},
		{
			name:                "network mode",
			networkMode:         "container:vpn",
			useComposeDependsOn: true,
			want:                []string{"vpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			log := zerolog.New(&out).Level(zerolog.DebugLevel)
			c := NewContainer(&log, &dockerContainer.InspectResponse{
				Name:       "/app",
				Config:     &dockerContainer.Config{Labels: tt.labels},
				HostConfig: &dockerContainer.HostConfig{NetworkMode: tt.networkMode},
			}, nil)

			// Measure only the Links call, not the container's creation.
			out.Reset()

			assert.Equal(t, tt.want, c.Links(tt.useComposeDependsOn))
			assert.Empty(t, out.String())
		})
	}
}

// TestContainer_Links_HostConfigAlwaysCounts verifies that network mode and
// legacy link targets are dependencies even when a depends-on label or a
// Compose depends_on label names other dependencies, as volumes-from targets
// are. Label dependencies come first and duplicates are dropped.
func TestContainer_Links_HostConfigAlwaysCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		labels              map[string]string
		networkMode         dockerContainer.NetworkMode
		links               []string
		volumesFrom         []string
		useComposeDependsOn bool
		want                []string
	}{
		{
			name:        "depends-on label and network mode",
			labels:      map[string]string{dependsOnLabel: "db"},
			networkMode: "container:vpn",
			want:        []string{"db", "vpn"},
		},
		{
			name:   "depends-on label and legacy link",
			labels: map[string]string{dependsOnLabel: "db"},
			links:  []string{"/cache:/app/cache"},
			want:   []string{"db", "cache"},
		},
		{
			name:        "depends-on label, network mode, legacy link, and volumes-from",
			labels:      map[string]string{dependsOnLabel: "db"},
			networkMode: "container:vpn",
			links:       []string{"/cache:/app/cache"},
			volumesFrom: []string{"data:ro"},
			want:        []string{"db", "cache", "vpn", "data"},
		},
		{
			name: "compose depends_on label and network mode",
			labels: map[string]string{
				compose.ComposeDependsOnLabel: "db:service_started:false",
				compose.ComposeProjectLabel:   "shop",
			},
			networkMode:         "container:vpn",
			useComposeDependsOn: true,
			want:                []string{"shop-db", "vpn"},
		},
		{
			name:        "network mode naming a label dependency",
			labels:      map[string]string{dependsOnLabel: "vpn"},
			networkMode: "container:vpn",
			want:        []string{"vpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := NewContainer(nil, &dockerContainer.InspectResponse{
				Name:   "/app",
				Config: &dockerContainer.Config{Labels: tt.labels},
				HostConfig: &dockerContainer.HostConfig{
					NetworkMode: tt.networkMode,
					Links:       tt.links,
					VolumesFrom: tt.volumesFrom,
				},
			}, nil)

			assert.Equal(t, tt.want, c.Links(tt.useComposeDependsOn))
		})
	}
}
