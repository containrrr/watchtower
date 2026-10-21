package container

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// hasPrefixedElement reports whether any element of values starts with prefix.
//
// Parameters:
//   - values: Values to search, such as environment entries.
//   - prefix: Prefix to look for.
//
// Returns:
//   - bool: True when an element starts with prefix.
func hasPrefixedElement(values []string, prefix string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.HasPrefix(value, prefix)
	})
}

// TestEphemeralOrchestrator verifies the Docker connection settings the
// self-update orchestrator inherits from the source container, its container
// and host configuration, its creation and start, and the removal of orphaned
// orchestrators.
func TestEphemeralOrchestrator(t *testing.T) {
	t.Parallel()

	t.Run("parseEnvVar", func(t *testing.T) {
		t.Parallel()

		t.Run("should parse KEY=VALUE correctly", func(t *testing.T) {
			t.Parallel()

			key, value, found := parseEnvVar("DOCKER_HOST=tcp://localhost:2375")

			assert.True(t, found)
			assert.Equal(t, "DOCKER_HOST", key)
			assert.Equal(t, "tcp://localhost:2375", value)
		})

		t.Run("should parse KEY= with empty value", func(t *testing.T) {
			t.Parallel()

			key, value, found := parseEnvVar("EMPTY=")

			assert.True(t, found)
			assert.Equal(t, "EMPTY", key)
			assert.Empty(t, value)
		})

		t.Run("should return not found for KEY without equals sign", func(t *testing.T) {
			t.Parallel()

			key, value, found := parseEnvVar("NOEQUALS")

			assert.False(t, found)
			assert.Empty(t, key)
			assert.Empty(t, value)
		})

		t.Run("should preserve value after second equals sign", func(t *testing.T) {
			t.Parallel()

			key, value, found := parseEnvVar("URL=http://host:8080/path")

			assert.True(t, found)
			assert.Equal(t, "URL", key)
			assert.Equal(t, "http://host:8080/path", value)
		})
	})

	t.Run("isLocalDockerHost", func(t *testing.T) {
		t.Parallel()

		t.Run("should return true for unix:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.True(t, isLocalDockerHost(newTestLogger(t), "unix:///var/run/docker.sock"))
		})

		t.Run("should return true for npipe:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.True(t, isLocalDockerHost(newTestLogger(t), "npipe:////./pipe/docker_engine"))
		})

		t.Run("should return true for absolute Unix path", func(t *testing.T) {
			t.Parallel()

			assert.True(t, isLocalDockerHost(newTestLogger(t), "/path/to/docker.sock"))
		})

		t.Run("should return true for Windows pipe path", func(t *testing.T) {
			t.Parallel()

			assert.True(t, isLocalDockerHost(newTestLogger(t), "//./pipe/docker_engine"))
		})

		t.Run("should return false for tcp:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.False(t, isLocalDockerHost(newTestLogger(t), "tcp://host:2375"))
		})

		t.Run("should return false for http:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.False(t, isLocalDockerHost(newTestLogger(t), "http://host:2375"))
		})

		t.Run("should return false for https:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.False(t, isLocalDockerHost(newTestLogger(t), "https://host:2376"))
		})

		t.Run("should return false for ssh:// scheme", func(t *testing.T) {
			t.Parallel()

			assert.False(t, isLocalDockerHost(newTestLogger(t), "ssh://user@host"))
		})
	})

	t.Run("extractSocketPath", func(t *testing.T) {
		t.Parallel()

		t.Run("should extract path from unix:// URL", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "/var/run/docker.sock", extractSocketPath("unix:///var/run/docker.sock"))
		})

		t.Run("should extract path from npipe:// URL", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "//./pipe/docker_engine", extractSocketPath("npipe:////./pipe/docker_engine"))
		})

		t.Run("should return bare path as-is", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "/var/run/docker.sock", extractSocketPath("/var/run/docker.sock"))
		})

		t.Run("should return empty for tcp:// URL", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, extractSocketPath("tcp://host:2375"))
		})
	})

	t.Run("appendDockerEnvVars", func(t *testing.T) {
		t.Parallel()

		t.Run("should append nothing for empty config", func(t *testing.T) {
			t.Parallel()

			env := []string{"EXISTING=val"}
			config := &dockerConnectionConfig{}
			result := appendDockerEnvVars(env, config)

			assert.Equal(t, []string{"EXISTING=val"}, result)
		})

		t.Run("should append Host only when set", func(t *testing.T) {
			t.Parallel()

			env := []string{}
			config := &dockerConnectionConfig{Host: "tcp://host:2375"}
			result := appendDockerEnvVars(env, config)

			require.Len(t, result, 1)
			assert.Contains(t, result, "DOCKER_HOST=tcp://host:2375")
		})

		t.Run("should append all fields when populated", func(t *testing.T) {
			t.Parallel()

			env := []string{}
			config := &dockerConnectionConfig{
				Host:       "tcp://host:2376",
				TLSVerify:  "1",
				CertPath:   "/certs",
				APIVersion: "1.41",
			}
			result := appendDockerEnvVars(env, config)

			require.Len(t, result, 4)
			assert.Contains(t, result, "DOCKER_HOST=tcp://host:2376")
			assert.Contains(t, result, "DOCKER_TLS_VERIFY=1")
			assert.Contains(t, result, "DOCKER_CERT_PATH=/certs")
			assert.Contains(t, result, "DOCKER_API_VERSION=1.41")
		})

		t.Run("should append only populated fields", func(t *testing.T) {
			t.Parallel()

			env := []string{"HOME=/root"}
			config := &dockerConnectionConfig{
				Host:      "tcp://host:2375",
				TLSVerify: "1",
			}
			result := appendDockerEnvVars(env, config)

			require.Len(t, result, 3)
			assert.Contains(t, result, "HOME=/root")
			assert.Contains(t, result, "DOCKER_HOST=tcp://host:2375")
			assert.Contains(t, result, "DOCKER_TLS_VERIFY=1")
			assert.False(t, hasPrefixedElement(result, "DOCKER_CERT_PATH"), "%v", result)
			assert.False(t, hasPrefixedElement(result, "DOCKER_API_VERSION"), "%v", result)
		})
	})

	t.Run("extractDockerConnectionConfig", func(t *testing.T) {
		t.Parallel()

		t.Run("no DOCKER_HOST env var is set", func(t *testing.T) {
			t.Parallel()

			t.Run("should return default local socket config", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source1"),
					WithName("watchtower"),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				require.NotNil(t, config)
				assert.True(t, config.IsLocal)
				assert.Empty(t, config.Host)
				assert.Equal(t, defaultSocketBind(), config.SocketBind)
			})
		})

		t.Run("DOCKER_HOST is a unix:// socket", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect as local", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source2"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=unix:///custom/docker.sock",
					}),
					WithBinds([]string{
						"/var/run/docker.sock:/custom/docker.sock",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.True(t, config.IsLocal)
				assert.Equal(t, "unix:///custom/docker.sock", config.Host)
				assert.Equal(t, "/var/run/docker.sock:/custom/docker.sock", config.SocketBind)
			})
		})

		t.Run("DOCKER_HOST is a npipe://", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect as local", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source3"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=npipe:////./pipe/docker_engine",
					}),
					WithBinds([]string{
						"//./pipe/docker_engine://./pipe/docker_engine",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.True(t, config.IsLocal)
				assert.Equal(t, "npipe:////./pipe/docker_engine", config.Host)
				assert.Equal(t, "//./pipe/docker_engine://./pipe/docker_engine", config.SocketBind)
			})
		})

		t.Run("DOCKER_HOST is tcp://", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect as remote", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source4"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=tcp://remote-host:2375",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.False(t, config.IsLocal)
				assert.Equal(t, "tcp://remote-host:2375", config.Host)
				// Remote connections do not use socket bind.
				assert.Empty(t, config.SocketBind)
			})
		})

		t.Run("DOCKER_HOST is https://", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect as remote", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source5"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=https://remote-host:2376",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.False(t, config.IsLocal)
				assert.Equal(t, "https://remote-host:2376", config.Host)
			})
		})

		t.Run("DOCKER_HOST is ssh://", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect as remote", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source6"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=ssh://user@remote-host",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.False(t, config.IsLocal)
				assert.Equal(t, "ssh://user@remote-host", config.Host)
			})
		})

		t.Run("TLS configuration is present", func(t *testing.T) {
			t.Parallel()

			t.Run("should pass through TLS settings", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source7"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=tcp://remote:2376",
						"DOCKER_TLS_VERIFY=1",
						"DOCKER_CERT_PATH=/certs",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.Equal(t, "1", config.TLSVerify)
				assert.Equal(t, "/certs", config.CertPath)
				assert.False(t, config.IsLocal)
			})
		})

		t.Run("API version is set", func(t *testing.T) {
			t.Parallel()

			t.Run("should pass through API version", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source8"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_API_VERSION=1.41",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.Equal(t, "1.41", config.APIVersion)
			})
		})

		t.Run("source has socket bind in host config", func(t *testing.T) {
			t.Parallel()

			t.Run("should extract socket bind from source", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source9"),
					WithName("watchtower"),
					WithBinds([]string{
						"/custom/docker.sock:/var/run/docker.sock",
						"/data:/data",
					}),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				assert.True(t, config.IsLocal)
				assert.Equal(t, "/custom/docker.sock:/var/run/docker.sock", config.SocketBind)
			})
		})

		t.Run("container has no config or env", func(t *testing.T) {
			t.Parallel()

			t.Run("should return default config gracefully", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("source10"),
					WithName("watchtower"),
				)

				config := extractDockerConnectionConfig(source, newTestLogger(t))

				require.NotNil(t, config)
				assert.True(t, config.IsLocal)
				assert.Empty(t, config.Host)
			})
		})
	})

	t.Run("buildOrchestratorConfig", func(t *testing.T) {
		t.Parallel()

		t.Run("given a source container, new image, container chain, and connConfig", func(t *testing.T) {
			t.Parallel()

			t.Run("should return a config with correct image, command, environment, and labels", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("abc123def456"),
					WithName("watchtower-test"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &dockerConnectionConfig{
					IsLocal:    true,
					SocketBind: "/var/run/docker.sock:/var/run/docker.sock",
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "old1,old2", connConfig, false)

				require.NotNil(t, config)
				assert.Equal(t, "watchtower:v2", config.Image)
				require.Len(t, config.Cmd, 1)
				assert.Contains(t, config.Cmd, "--self-update-orchestrator")

				// Verify environment variables are correctly set.
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_OLD_ID=abc123def456")
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_NEW_IMAGE=watchtower:v2")
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_ORIGINAL_NAME=watchtower-test")
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_CONTAINER_CHAIN=old1,old2")
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_CLEANUP=false")

				// Verify the orchestrator label is set but the watchtower label is NOT set.
				assert.Equal(t, "true", config.Labels[orchestratorLabel])
				assert.NotContains(t, config.Labels, "com.centurylinklabs.watchtower")
			})
		})

		t.Run("cleanup is enabled", func(t *testing.T) {
			t.Parallel()

			t.Run("should set WT_ORCHESTRATOR_CLEANUP to true", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)

				config := buildOrchestratorConfig(source, "watchtower:v2", "", nil, true)

				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_CLEANUP=true")
			})
		})

		t.Run("the container chain is empty", func(t *testing.T) {
			t.Parallel()

			t.Run("should include an empty container chain env var", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &dockerConnectionConfig{
					IsLocal:    true,
					SocketBind: "/var/run/docker.sock:/var/run/docker.sock",
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "", connConfig, false)

				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_CONTAINER_CHAIN=")
			})
		})

		t.Run("connConfig has Docker env vars", func(t *testing.T) {
			t.Parallel()

			t.Run("should forward Docker connection environment variables", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &dockerConnectionConfig{
					Host:       "tcp://remote:2375",
					TLSVerify:  "1",
					CertPath:   "/certs",
					APIVersion: "1.41",
					IsLocal:    false,
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "chain1", connConfig, false)

				assert.Contains(t, config.Env, "DOCKER_HOST=tcp://remote:2375")
				assert.Contains(t, config.Env, "DOCKER_TLS_VERIFY=1")
				assert.Contains(t, config.Env, "DOCKER_CERT_PATH=/certs")
				assert.Contains(t, config.Env, "DOCKER_API_VERSION=1.41")
			})
		})

		t.Run("connConfig is nil", func(t *testing.T) {
			t.Parallel()

			t.Run("should not include Docker env vars", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)

				config := buildOrchestratorConfig(source, "watchtower:v2", "chain1", nil, false)

				// Should still have orchestrator env vars but no Docker env vars.
				assert.Contains(t, config.Env, "WT_ORCHESTRATOR_OLD_ID=abc123")
				assert.False(t, hasPrefixedElement(config.Env, "DOCKER_HOST"), "%v", config.Env)
			})
		})
	})

	t.Run("buildOrchestratorHostConfig", func(t *testing.T) {
		t.Parallel()

		t.Run("should return a host config with AutoRemove", func(t *testing.T) {
			t.Parallel()

			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			require.NotNil(t, hostConfig)
			assert.True(t, hostConfig.AutoRemove)
		})

		t.Run("should not set port bindings", func(t *testing.T) {
			t.Parallel()

			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			assert.Nil(t, hostConfig.PortBindings)
		})

		t.Run("should not set a restart policy", func(t *testing.T) {
			t.Parallel()

			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			assert.Empty(t, hostConfig.RestartPolicy.Name)
		})

		t.Run("connConfig is nil", func(t *testing.T) {
			t.Parallel()

			t.Run("should return empty binds", func(t *testing.T) {
				t.Parallel()

				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(nil, source)

				assert.Empty(t, hostConfig.Binds)
			})
		})

		t.Run("connection is remote", func(t *testing.T) {
			t.Parallel()

			t.Run("should have empty binds", func(t *testing.T) {
				t.Parallel()

				connConfig := &dockerConnectionConfig{
					Host:    "tcp://remote:2375",
					IsLocal: false,
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				assert.Empty(t, hostConfig.Binds)
				assert.True(t, hostConfig.AutoRemove)
			})
		})

		t.Run("connection is local", func(t *testing.T) {
			t.Parallel()

			t.Run("should include socket bind", func(t *testing.T) {
				t.Parallel()

				connConfig := &dockerConnectionConfig{
					Host:       "unix:///custom/docker.sock",
					IsLocal:    true,
					SocketBind: "/custom/docker.sock:/var/run/docker.sock",
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				assert.Equal(t, []string{"/custom/docker.sock:/var/run/docker.sock"}, hostConfig.Binds)
			})
		})

		t.Run("TLS cert binds are present", func(t *testing.T) {
			t.Parallel()

			t.Run("should include TLS cert binds alongside socket bind", func(t *testing.T) {
				t.Parallel()

				connConfig := &dockerConnectionConfig{
					Host:       "unix:///var/run/docker.sock",
					IsLocal:    true,
					SocketBind: "/var/run/docker.sock:/var/run/docker.sock",
					CertBinds: []string{
						"/host/certs:/certs/ca.pem",
						"/host/certs:/certs/cert.pem",
					},
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				require.Len(t, hostConfig.Binds, 3)
				assert.Contains(t, hostConfig.Binds, "/var/run/docker.sock:/var/run/docker.sock")
				assert.Contains(t, hostConfig.Binds, "/host/certs:/certs/ca.pem")
				assert.Contains(t, hostConfig.Binds, "/host/certs:/certs/cert.pem")
			})
		})

		t.Run("TLS cert binds are present on remote connection", func(t *testing.T) {
			t.Parallel()

			t.Run("should include cert binds without socket bind", func(t *testing.T) {
				t.Parallel()

				connConfig := &dockerConnectionConfig{
					Host:      "tcp://remote:2376",
					IsLocal:   false,
					TLSVerify: "1",
					CertPath:  "/certs",
					CertBinds: []string{
						"/host/certs:/certs",
					},
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				assert.Equal(t, []string{"/host/certs:/certs"}, hostConfig.Binds)
				assert.True(t, hostConfig.AutoRemove)
			})
		})

		t.Run("source container has a network mode", func(t *testing.T) {
			t.Parallel()

			t.Run("should inherit the source NetworkMode", func(t *testing.T) {
				t.Parallel()

				source := MockContainer(WithNetworkMode("host"))
				hostConfig := buildOrchestratorHostConfig(nil, source)

				assert.Equal(t, dockerContainer.NetworkMode("host"), hostConfig.NetworkMode)
			})
		})
	})

	t.Run("CreateEphemeralOrchestrator", func(t *testing.T) {
		t.Parallel()

		const orchestratorName = "watchtower-orchestrator-source123"

		// createOptions matches the create options of the orchestrator for source.
		createOptions := mock.MatchedBy(func(options dockerClient.ContainerCreateOptions) bool {
			return options.Name == orchestratorName && options.Config != nil && options.Config.Image == "watchtower:v2"
		})

		// newSource returns the Watchtower container being replaced.
		newSource := func() types.Container {
			return MockContainer(
				WithID("source123"),
				WithName("watchtower-source"),
				WithImageName("watchtower:latest"),
			)
		}

		serverError := errors.New("Error response from daemon: internal server error")

		t.Run("creation and start succeed", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the orchestrator container ID", func(t *testing.T) {
				t.Parallel()

				api := mockContainer.NewMockEphemeralAPI(t)
				api.EXPECT().ContainerCreate(mock.Anything, createOptions).
					Return(dockerClient.ContainerCreateResult{ID: "orchestrator-id-123"}, nil).Once()
				api.EXPECT().ContainerStart(mock.Anything, "orchestrator-id-123", dockerClient.ContainerStartOptions{}).
					Return(dockerClient.ContainerStartResult{}, nil).Once()

				orchestratorID, err := createEphemeralOrchestrator(
					newTestLogger(t), t.Context(), api, newSource(), "watchtower:v2", "chain1", false,
				)

				require.NoError(t, err)
				assert.Equal(t, types.ContainerID("orchestrator-id-123"), orchestratorID)
			})
		})

		t.Run("ContainerCreate fails", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error wrapping errEphemeralCreateFailed", func(t *testing.T) {
				t.Parallel()

				api := mockContainer.NewMockEphemeralAPI(t)
				api.EXPECT().ContainerCreate(mock.Anything, createOptions).
					Return(dockerClient.ContainerCreateResult{}, serverError).Once()

				orchestratorID, err := createEphemeralOrchestrator(
					newTestLogger(t), t.Context(), api, newSource(), "watchtower:v2", "chain1", false,
				)

				require.ErrorIs(t, err, errEphemeralCreateFailed)
				assert.Empty(t, orchestratorID)
			})
		})

		t.Run("ContainerStart fails", func(t *testing.T) {
			t.Parallel()

			t.Run("should attempt cleanup and return errEphemeralStartFailed", func(t *testing.T) {
				t.Parallel()

				api := mockContainer.NewMockEphemeralAPI(t)
				api.EXPECT().ContainerCreate(mock.Anything, createOptions).
					Return(dockerClient.ContainerCreateResult{ID: "orchestrator-fail-start"}, nil).Once()
				api.EXPECT().ContainerStart(mock.Anything, "orchestrator-fail-start", dockerClient.ContainerStartOptions{}).
					Return(dockerClient.ContainerStartResult{}, serverError).Once()
				// The created container is removed, with a context of its own so
				// cleanup survives a canceled parent.
				api.EXPECT().ContainerRemove(mock.Anything, "orchestrator-fail-start", dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, nil).Once()

				orchestratorID, err := createEphemeralOrchestrator(
					newTestLogger(t), t.Context(), api, newSource(), "watchtower:v2", "chain1", false,
				)

				require.ErrorIs(t, err, errEphemeralStartFailed)
				assert.Empty(t, orchestratorID)
			})
		})
	})

	t.Run("RemoveOrphanedOrchestrators", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		// orphaned marks an orchestrator as running long past any self-update.
		orphaned := WithContainerState(dockerContainer.State{
			Running:   true,
			Status:    "running",
			StartedAt: "2000-01-01T00:00:00Z",
		})

		t.Run("there are no containers", func(t *testing.T) {
			t.Parallel()

			t.Run("should return 0 removed and no error", func(t *testing.T) {
				t.Parallel()

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{}, nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 0, count)
			})
		})

		t.Run("there are containers but none are orchestrators", func(t *testing.T) {
			t.Parallel()

			t.Run("should return 0 removed and no error", func(t *testing.T) {
				t.Parallel()

				regularContainer := MockContainer(
					WithID("regular123"),
					WithName("regular-container"),
					WithLabels(map[string]string{
						"com.centurylinklabs.watchtower": "true",
					}),
				)

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{regularContainer}, nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 0, count)
			})
		})

		t.Run("there is an orphaned orchestrator", func(t *testing.T) {
			t.Parallel()

			t.Run("should remove the orchestrator and return 1", func(t *testing.T) {
				t.Parallel()

				orchestrator := MockContainer(
					WithID("orch123"),
					WithName("watchtower-orchestrator-abc"),
					WithLabels(map[string]string{
						orchestratorLabel: "true",
					}),
					orphaned,
				)

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{orchestrator}, nil)
				mockAPIClient.EXPECT().
					StopAndRemoveContainer(
						ctx,
						orchestrator,
						mock.AnythingOfType("time.Duration"),
					).
					Return(nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 1, count)
			})
		})

		t.Run("there are multiple orphaned orchestrators", func(t *testing.T) {
			t.Parallel()

			t.Run("should remove all orchestrators and return the count", func(t *testing.T) {
				t.Parallel()

				orch1 := MockContainer(
					WithID("orch001"),
					WithName("watchtower-orchestrator-001"),
					WithLabels(map[string]string{
						orchestratorLabel: "true",
					}),
					orphaned,
				)
				orch2 := MockContainer(
					WithID("orch002"),
					WithName("watchtower-orchestrator-002"),
					WithLabels(map[string]string{
						orchestratorLabel: "true",
					}),
					orphaned,
				)
				regular := MockContainer(
					WithID("regular1"),
					WithName("app-container"),
				)

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{orch1, regular, orch2}, nil)
				mockAPIClient.EXPECT().
					StopAndRemoveContainer(
						ctx,
						orch1,
						mock.AnythingOfType("time.Duration"),
					).
					Return(nil)
				mockAPIClient.EXPECT().
					StopAndRemoveContainer(
						ctx,
						orch2,
						mock.AnythingOfType("time.Duration"),
					).
					Return(nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 2, count)
			})
		})

		t.Run("ListContainers fails", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error", func(t *testing.T) {
				t.Parallel()

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return(nil, errors.New("docker daemon unavailable"))

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.ErrorContains(t, err, "failed to list containers")
				assert.Equal(t, 0, count)
			})
		})

		t.Run("StopAndRemoveContainer fails for one orchestrator", func(t *testing.T) {
			t.Parallel()

			t.Run("should continue and remove others, returning partial count", func(t *testing.T) {
				t.Parallel()

				orch1 := MockContainer(
					WithID("orch001"),
					WithName("watchtower-orchestrator-001"),
					WithLabels(map[string]string{
						orchestratorLabel: "true",
					}),
					orphaned,
				)
				orch2 := MockContainer(
					WithID("orch002"),
					WithName("watchtower-orchestrator-002"),
					WithLabels(map[string]string{
						orchestratorLabel: "true",
					}),
					orphaned,
				)

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{orch1, orch2}, nil)
				mockAPIClient.EXPECT().
					StopAndRemoveContainer(
						ctx,
						orch1,
						mock.AnythingOfType("time.Duration"),
					).
					Return(errors.New("removal failed"))
				mockAPIClient.EXPECT().
					StopAndRemoveContainer(
						ctx,
						orch2,
						mock.AnythingOfType("time.Duration"),
					).
					Return(nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 1, count)
			})
		})

		t.Run("a container has no orchestrator label", func(t *testing.T) {
			t.Parallel()

			t.Run("should skip the container without error", func(t *testing.T) {
				t.Parallel()

				noLabelContainer := MockContainer(
					WithID("no-label"),
					WithName("no-label-container"),
				)

				mockAPIClient := mockContainer.NewMockClient(t)
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{noLabelContainer}, nil)

				count, err := RemoveOrphanedOrchestrators(newTestLogger(t), ctx, mockAPIClient)

				require.NoError(t, err)
				assert.Equal(t, 0, count)
			})
		})
	})
}

// TestExtractDockerConnectionConfig_DockerHostSocket verifies that the
// orchestrator mounts the socket the source container's DOCKER_HOST names, so
// the forwarded DOCKER_HOST reaches the daemon, and that a remote DOCKER_HOST
// mounts no socket whatever the case of its scheme.
func TestExtractDockerConnectionConfig_DockerHostSocket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      string
		binds     []string
		wantLocal bool
		wantBind  string
	}{
		{
			name:      "socket proxy mounted at the DOCKER_HOST path",
			host:      "unix:///run/proxy/proxy.sock",
			binds:     []string{"/srv/proxy/proxy.sock:/run/proxy/proxy.sock", "/var/run/docker.sock:/var/run/docker.sock"},
			wantLocal: true,
			wantBind:  "/srv/proxy/proxy.sock:/run/proxy/proxy.sock",
		},
		{
			name:      "Docker socket mounted at a custom DOCKER_HOST path",
			host:      "unix:///var/run/host-docker.sock",
			binds:     []string{"/var/run/docker.sock:/var/run/host-docker.sock:ro"},
			wantLocal: true,
			wantBind:  "/var/run/docker.sock:/var/run/host-docker.sock:ro",
		},
		{
			name:      "DOCKER_HOST socket without a mount keeps its own path",
			host:      "unix:///run/custom.sock",
			binds:     []string{"/data:/data"},
			wantLocal: true,
			wantBind:  "/run/custom.sock:/run/custom.sock",
		},
		{
			name:      "DOCKER_HOST path that another mount only names on the host side",
			host:      "unix:///custom/docker.sock",
			binds:     []string{"/custom/docker.sock:/var/run/docker.sock"},
			wantLocal: true,
			wantBind:  "/custom/docker.sock:/custom/docker.sock",
		},
		{
			name:      "socket mounted through its directory",
			host:      "unix:///run/proxy/proxy.sock",
			binds:     []string{"/srv/proxy:/run/proxy"},
			wantLocal: true,
			wantBind:  "/srv/proxy:/run/proxy",
		},
		{
			name:      "socket mounted through a named volume",
			host:      "unix:///run/proxy/proxy.sock",
			binds:     []string{"proxyvol:/run/proxy:ro"},
			wantLocal: true,
			wantBind:  "proxyvol:/run/proxy:ro",
		},
		{
			name:      "the most specific mount wins",
			host:      "unix:///run/proxy/proxy.sock",
			binds:     []string{"/srv:/run", "/srv/proxy/proxy.sock:/run/proxy/proxy.sock", "/srv/proxy:/run/proxy"},
			wantLocal: true,
			wantBind:  "/srv/proxy/proxy.sock:/run/proxy/proxy.sock",
		},
		{
			name:      "a sibling path with the same prefix is not a parent",
			host:      "unix:///run/proxy/proxy.sock",
			binds:     []string{"/srv/prox:/run/prox"},
			wantLocal: true,
			wantBind:  "/run/proxy/proxy.sock:/run/proxy/proxy.sock",
		},
		{
			name:      "uppercase unix scheme",
			host:      "UNIX:///run/custom.sock",
			wantLocal: true,
			wantBind:  "/run/custom.sock:/run/custom.sock",
		},
		{
			name:      "redundant path elements in DOCKER_HOST",
			host:      "unix:///run//proxy/./proxy.sock",
			binds:     []string{"/srv/proxy:/run/proxy"},
			wantLocal: true,
			wantBind:  "/srv/proxy:/run/proxy",
		},
		{
			name:      "mixed-case npipe scheme",
			host:      "NPipe:////./pipe/docker_engine",
			binds:     []string{"//./pipe/docker_engine://./pipe/docker_engine"},
			wantLocal: true,
			wantBind:  "//./pipe/docker_engine://./pipe/docker_engine",
		},
		{
			name:      "uppercase tcp scheme is remote",
			host:      "TCP://docker.example.com:2375",
			binds:     []string{"/var/run/docker.sock:/var/run/docker.sock"},
			wantLocal: false,
			wantBind:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := MockContainer(
				WithID("source"),
				WithName("watchtower"),
				WithEnv([]string{"DOCKER_HOST=" + tt.host}),
				WithBinds(tt.binds),
			)

			config := extractDockerConnectionConfig(source, newTestLogger(t))

			assert.Equal(t, tt.wantLocal, config.IsLocal)
			assert.Equal(t, tt.wantBind, config.SocketBind)
		})
	}
}
