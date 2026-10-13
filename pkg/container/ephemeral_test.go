package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"
	mock "github.com/stretchr/testify/mock"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func nopLogger() *zerolog.Logger {
	nop := zerolog.Nop()

	return &nop
}

var _ = ginkgo.Describe("Ephemeral Orchestrator", func() {
	ginkgo.Describe("parseEnvVar", func() {
		ginkgo.It("should parse KEY=VALUE correctly", func() {
			key, value, found := parseEnvVar("DOCKER_HOST=tcp://localhost:2375")

			gomega.Expect(found).To(gomega.BeTrue())
			gomega.Expect(key).To(gomega.Equal("DOCKER_HOST"))
			gomega.Expect(value).To(gomega.Equal("tcp://localhost:2375"))
		})

		ginkgo.It("should parse KEY= with empty value", func() {
			key, value, found := parseEnvVar("EMPTY=")

			gomega.Expect(found).To(gomega.BeTrue())
			gomega.Expect(key).To(gomega.Equal("EMPTY"))
			gomega.Expect(value).To(gomega.BeEmpty())
		})

		ginkgo.It("should return not found for KEY without equals sign", func() {
			key, value, found := parseEnvVar("NOEQUALS")

			gomega.Expect(found).To(gomega.BeFalse())
			gomega.Expect(key).To(gomega.BeEmpty())
			gomega.Expect(value).To(gomega.BeEmpty())
		})

		ginkgo.It("should preserve value after second equals sign", func() {
			key, value, found := parseEnvVar("URL=http://host:8080/path")

			gomega.Expect(found).To(gomega.BeTrue())
			gomega.Expect(key).To(gomega.Equal("URL"))
			gomega.Expect(value).To(gomega.Equal("http://host:8080/path"))
		})
	})

	ginkgo.Describe("isLocalDockerHost", func() {
		ginkgo.It("should return true for unix:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "unix:///var/run/docker.sock")).To(gomega.BeTrue())
		})

		ginkgo.It("should return true for npipe:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "npipe:////./pipe/docker_engine")).To(gomega.BeTrue())
		})

		ginkgo.It("should return true for absolute Unix path", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "/path/to/docker.sock")).To(gomega.BeTrue())
		})

		ginkgo.It("should return true for Windows pipe path", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "//./pipe/docker_engine")).To(gomega.BeTrue())
		})

		ginkgo.It("should return false for tcp:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "tcp://host:2375")).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for http:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "http://host:2375")).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for https:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "https://host:2376")).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for ssh:// scheme", func() {
			gomega.Expect(isLocalDockerHost(testLog(), "ssh://user@host")).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("extractSocketPath", func() {
		ginkgo.It("should extract path from unix:// URL", func() {
			gomega.Expect(extractSocketPath("unix:///var/run/docker.sock")).
				To(gomega.Equal("/var/run/docker.sock"))
		})

		ginkgo.It("should extract path from npipe:// URL", func() {
			gomega.Expect(extractSocketPath("npipe:////./pipe/docker_engine")).
				To(gomega.Equal("//./pipe/docker_engine"))
		})

		ginkgo.It("should return bare path as-is", func() {
			gomega.Expect(extractSocketPath("/var/run/docker.sock")).
				To(gomega.Equal("/var/run/docker.sock"))
		})

		ginkgo.It("should return empty for tcp:// URL", func() {
			gomega.Expect(extractSocketPath("tcp://host:2375")).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("appendDockerEnvVars", func() {
		ginkgo.It("should append nothing for empty config", func() {
			env := []string{"EXISTING=val"}
			config := &DockerConnectionConfig{}
			result := appendDockerEnvVars(env, config)

			gomega.Expect(result).To(gomega.Equal([]string{"EXISTING=val"}))
		})

		ginkgo.It("should append Host only when set", func() {
			env := []string{}
			config := &DockerConnectionConfig{Host: "tcp://host:2375"}
			result := appendDockerEnvVars(env, config)

			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_HOST=tcp://host:2375"))
		})

		ginkgo.It("should append all fields when populated", func() {
			env := []string{}
			config := &DockerConnectionConfig{
				Host:       "tcp://host:2376",
				TLSVerify:  "1",
				CertPath:   "/certs",
				APIVersion: "1.41",
			}
			result := appendDockerEnvVars(env, config)

			gomega.Expect(result).To(gomega.HaveLen(4))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_HOST=tcp://host:2376"))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_TLS_VERIFY=1"))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_CERT_PATH=/certs"))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_API_VERSION=1.41"))
		})

		ginkgo.It("should append only populated fields", func() {
			env := []string{"HOME=/root"}
			config := &DockerConnectionConfig{
				Host:      "tcp://host:2375",
				TLSVerify: "1",
			}
			result := appendDockerEnvVars(env, config)

			gomega.Expect(result).To(gomega.HaveLen(3))
			gomega.Expect(result).To(gomega.ContainElement("HOME=/root"))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_HOST=tcp://host:2375"))
			gomega.Expect(result).To(gomega.ContainElement("DOCKER_TLS_VERIFY=1"))
			gomega.Expect(result).NotTo(gomega.ContainElement(gomega.HavePrefix("DOCKER_CERT_PATH")))
			gomega.Expect(result).NotTo(gomega.ContainElement(gomega.HavePrefix("DOCKER_API_VERSION")))
		})
	})

	ginkgo.Describe("extractDockerConnectionConfig", func() {
		ginkgo.When("no DOCKER_HOST env var is set", func() {
			ginkgo.It("should return default local socket config", func() {
				source := MockContainer(
					WithID("source1"),
					WithName("watchtower"),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config).NotTo(gomega.BeNil())
				gomega.Expect(config.IsLocal).To(gomega.BeTrue())
				gomega.Expect(config.Host).To(gomega.BeEmpty())
				gomega.Expect(config.SocketBind).To(gomega.Equal(
					defaultSocketBind(),
				))
			})
		})

		ginkgo.When("DOCKER_HOST is a unix:// socket", func() {
			ginkgo.It("should detect as local", func() {
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

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeTrue())
				gomega.Expect(config.Host).To(gomega.Equal("unix:///custom/docker.sock"))
				gomega.Expect(config.SocketBind).To(gomega.Equal(
					"/var/run/docker.sock:/custom/docker.sock",
				))
			})
		})

		ginkgo.When("DOCKER_HOST is a npipe://", func() {
			ginkgo.It("should detect as local", func() {
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

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeTrue())
				gomega.Expect(config.Host).To(gomega.Equal("npipe:////./pipe/docker_engine"))
				gomega.Expect(config.SocketBind).To(gomega.Equal(
					"//./pipe/docker_engine://./pipe/docker_engine",
				))
			})
		})

		ginkgo.When("DOCKER_HOST is tcp://", func() {
			ginkgo.It("should detect as remote", func() {
				source := MockContainer(
					WithID("source4"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=tcp://remote-host:2375",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeFalse())
				gomega.Expect(config.Host).To(gomega.Equal("tcp://remote-host:2375"))
				// Remote connections do not use socket bind.
				gomega.Expect(config.SocketBind).To(gomega.BeEmpty())
			})
		})

		ginkgo.When("DOCKER_HOST is https://", func() {
			ginkgo.It("should detect as remote", func() {
				source := MockContainer(
					WithID("source5"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=https://remote-host:2376",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeFalse())
				gomega.Expect(config.Host).To(gomega.Equal("https://remote-host:2376"))
			})
		})

		ginkgo.When("DOCKER_HOST is ssh://", func() {
			ginkgo.It("should detect as remote", func() {
				source := MockContainer(
					WithID("source6"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=ssh://user@remote-host",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeFalse())
				gomega.Expect(config.Host).To(gomega.Equal("ssh://user@remote-host"))
			})
		})

		ginkgo.When("TLS configuration is present", func() {
			ginkgo.It("should pass through TLS settings", func() {
				source := MockContainer(
					WithID("source7"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_HOST=tcp://remote:2376",
						"DOCKER_TLS_VERIFY=1",
						"DOCKER_CERT_PATH=/certs",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.TLSVerify).To(gomega.Equal("1"))
				gomega.Expect(config.CertPath).To(gomega.Equal("/certs"))
				gomega.Expect(config.IsLocal).To(gomega.BeFalse())
			})
		})

		ginkgo.When("API version is set", func() {
			ginkgo.It("should pass through API version", func() {
				source := MockContainer(
					WithID("source8"),
					WithName("watchtower"),
					WithEnv([]string{
						"DOCKER_API_VERSION=1.41",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.APIVersion).To(gomega.Equal("1.41"))
			})
		})

		ginkgo.When("source has socket bind in host config", func() {
			ginkgo.It("should extract socket bind from source", func() {
				source := MockContainer(
					WithID("source9"),
					WithName("watchtower"),
					WithBinds([]string{
						"/custom/docker.sock:/var/run/docker.sock",
						"/data:/data",
					}),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config.IsLocal).To(gomega.BeTrue())
				gomega.Expect(config.SocketBind).To(gomega.Equal(
					"/custom/docker.sock:/var/run/docker.sock",
				))
			})
		})

		ginkgo.When("container has no config or env", func() {
			ginkgo.It("should return default config gracefully", func() {
				source := MockContainer(
					WithID("source10"),
					WithName("watchtower"),
				)

				config := extractDockerConnectionConfig(source, nopLogger())

				gomega.Expect(config).NotTo(gomega.BeNil())
				gomega.Expect(config.IsLocal).To(gomega.BeTrue())
				gomega.Expect(config.Host).To(gomega.BeEmpty())
			})
		})
	})

	ginkgo.Describe("buildOrchestratorConfig", func() {
		ginkgo.When("given a source container, new image, container chain, and connConfig", func() {
			ginkgo.It("should return a config with correct image, command, environment, and labels", func() {
				source := MockContainer(
					WithID("abc123def456"),
					WithName("watchtower-test"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &DockerConnectionConfig{
					IsLocal:    true,
					SocketBind: "/var/run/docker.sock:/var/run/docker.sock",
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "old1,old2", connConfig, false)

				gomega.Expect(config).NotTo(gomega.BeNil())
				gomega.Expect(config.Image).To(gomega.Equal("watchtower:v2"))
				gomega.Expect(config.Cmd).To(gomega.HaveLen(1))
				gomega.Expect(config.Cmd).To(gomega.ContainElement("--self-update-orchestrator"))

				// Verify environment variables are correctly set.
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_OLD_ID=abc123def456",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_NEW_IMAGE=watchtower:v2",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_ORIGINAL_NAME=watchtower-test",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_CONTAINER_CHAIN=old1,old2",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_CLEANUP=false",
				))

				// Verify the orchestrator label is set but the watchtower label is NOT set.
				gomega.Expect(config.Labels).To(gomega.HaveKeyWithValue(OrchestratorLabel, "true"))
				gomega.Expect(config.Labels).NotTo(gomega.HaveKey("com.centurylinklabs.watchtower"))
			})
		})

		ginkgo.When("cleanup is enabled", func() {
			ginkgo.It("should set WT_ORCHESTRATOR_CLEANUP to true", func() {
				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)

				config := buildOrchestratorConfig(source, "watchtower:v2", "", nil, true)

				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_CLEANUP=true",
				))
			})
		})

		ginkgo.When("the container chain is empty", func() {
			ginkgo.It("should include an empty container chain env var", func() {
				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &DockerConnectionConfig{
					IsLocal:    true,
					SocketBind: "/var/run/docker.sock:/var/run/docker.sock",
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "", connConfig, false)

				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_CONTAINER_CHAIN=",
				))
			})
		})

		ginkgo.When("connConfig has Docker env vars", func() {
			ginkgo.It("should forward Docker connection environment variables", func() {
				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)
				connConfig := &DockerConnectionConfig{
					Host:       "tcp://remote:2375",
					TLSVerify:  "1",
					CertPath:   "/certs",
					APIVersion: "1.41",
					IsLocal:    false,
				}

				config := buildOrchestratorConfig(source, "watchtower:v2", "chain1", connConfig, false)

				gomega.Expect(config.Env).To(gomega.ContainElement(
					"DOCKER_HOST=tcp://remote:2375",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"DOCKER_TLS_VERIFY=1",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"DOCKER_CERT_PATH=/certs",
				))
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"DOCKER_API_VERSION=1.41",
				))
			})
		})

		ginkgo.When("connConfig is nil", func() {
			ginkgo.It("should not include Docker env vars", func() {
				source := MockContainer(
					WithID("abc123"),
					WithName("watchtower"),
					WithImageName("watchtower:latest"),
				)

				config := buildOrchestratorConfig(source, "watchtower:v2", "chain1", nil, false)

				// Should still have orchestrator env vars but no Docker env vars.
				gomega.Expect(config.Env).To(gomega.ContainElement(
					"WT_ORCHESTRATOR_OLD_ID=abc123",
				))
				gomega.Expect(config.Env).NotTo(gomega.ContainElement(
					gomega.HavePrefix("DOCKER_HOST"),
				))
			})
		})
	})

	ginkgo.Describe("buildOrchestratorHostConfig", func() {
		ginkgo.It("should return a host config with AutoRemove", func() {
			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			gomega.Expect(hostConfig).NotTo(gomega.BeNil())
			gomega.Expect(hostConfig.AutoRemove).To(gomega.BeTrue())
		})

		ginkgo.It("should not set port bindings", func() {
			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			gomega.Expect(hostConfig.PortBindings).To(gomega.BeNil())
		})

		ginkgo.It("should not set a restart policy", func() {
			source := MockContainer()
			hostConfig := buildOrchestratorHostConfig(nil, source)

			gomega.Expect(hostConfig.RestartPolicy.Name).To(gomega.BeEmpty())
		})

		ginkgo.When("connConfig is nil", func() {
			ginkgo.It("should return empty binds", func() {
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(nil, source)

				gomega.Expect(hostConfig.Binds).To(gomega.BeEmpty())
			})
		})

		ginkgo.When("connection is remote", func() {
			ginkgo.It("should have empty binds", func() {
				connConfig := &DockerConnectionConfig{
					Host:    "tcp://remote:2375",
					IsLocal: false,
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				gomega.Expect(hostConfig.Binds).To(gomega.BeEmpty())
				gomega.Expect(hostConfig.AutoRemove).To(gomega.BeTrue())
			})
		})

		ginkgo.When("connection is local", func() {
			ginkgo.It("should include socket bind", func() {
				connConfig := &DockerConnectionConfig{
					Host:       "unix:///custom/docker.sock",
					IsLocal:    true,
					SocketBind: "/custom/docker.sock:/var/run/docker.sock",
				}
				source := MockContainer()
				hostConfig := buildOrchestratorHostConfig(connConfig, source)

				gomega.Expect(hostConfig.Binds).To(gomega.Equal(
					[]string{"/custom/docker.sock:/var/run/docker.sock"},
				))
			})
		})

		ginkgo.When("TLS cert binds are present", func() {
			ginkgo.It("should include TLS cert binds alongside socket bind", func() {
				connConfig := &DockerConnectionConfig{
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

				gomega.Expect(hostConfig.Binds).To(gomega.HaveLen(3))
				gomega.Expect(hostConfig.Binds).To(gomega.ContainElement(
					"/var/run/docker.sock:/var/run/docker.sock",
				))
				gomega.Expect(hostConfig.Binds).To(gomega.ContainElement(
					"/host/certs:/certs/ca.pem",
				))
				gomega.Expect(hostConfig.Binds).To(gomega.ContainElement(
					"/host/certs:/certs/cert.pem",
				))
			})
		})

		ginkgo.When("TLS cert binds are present on remote connection", func() {
			ginkgo.It("should include cert binds without socket bind", func() {
				connConfig := &DockerConnectionConfig{
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

				gomega.Expect(hostConfig.Binds).To(gomega.Equal(
					[]string{"/host/certs:/certs"},
				))
				gomega.Expect(hostConfig.AutoRemove).To(gomega.BeTrue())
			})
		})

		ginkgo.When("source container has a network mode", func() {
			ginkgo.It("should inherit the source NetworkMode", func() {
				source := MockContainer(WithNetworkMode("host"))
				hostConfig := buildOrchestratorHostConfig(nil, source)

				gomega.Expect(hostConfig.NetworkMode).To(
					gomega.Equal(dockerContainer.NetworkMode("host")),
				)
			})
		})
	})

	ginkgo.Describe("CreateEphemeralOrchestrator", func() {
		var (
			mockServer *ghttp.Server
			dockerAPI  dockerClient.APIClient
			testClient *client
			source     types.Container
			ctx        context.Context
		)

		ginkgo.BeforeEach(func() {
			ctx = context.Background()
			mockServer = ghttp.NewServer()
			docker, err := dockerClient.New(
				dockerClient.WithHost(mockServer.URL()),
				dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			mockServer.AppendHandlers(APIVersionPingHandler())

			dockerAPI = docker
			testClient = &client{log: testLog(), api: dockerAPI}
			source = MockContainer(
				WithID("source123"),
				WithName("watchtower-source"),
				WithImageName("watchtower:latest"),
			)
		})

		ginkgo.AfterEach(func() {
			mockServer.Close()
		})

		ginkgo.When("creation and start succeed", func() {
			ginkgo.BeforeEach(func() {
				mockServer.AppendHandlers(
					// ContainerCreate handler.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", gomega.HaveSuffix("/containers/create")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.Header().Set("Content-Type", "application/json")

							var buf bytes.Buffer

							err := json.NewEncoder(&buf).Encode(dockerContainer.CreateResponse{
								ID: "orchestrator-id-123",
							})
							if err != nil {
								ginkgo.GinkgoWriter.Printf(
									"failed to encode CreateResponse: %v\n", err,
								)
								gomega.Expect(err).NotTo(gomega.HaveOccurred())

								return
							}

							w.WriteHeader(http.StatusCreated)
							_, _ = w.Write(buf.Bytes())
						},
					),
					// ContainerStart handler.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", gomega.HaveSuffix("/containers/orchestrator-id-123/start")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusNoContent)
						},
					),
				)
			})

			ginkgo.It("should return the orchestrator container ID", func() {
				orchestratorID, err := testClient.CreateEphemeralOrchestrator(
					ctx, source, "watchtower:v2", "chain1", false,
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(orchestratorID).To(gomega.Equal(types.ContainerID("orchestrator-id-123")))
			})
		})

		ginkgo.When("ContainerCreate fails", func() {
			ginkgo.BeforeEach(func() {
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", gomega.HaveSuffix("/containers/create")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusInternalServerError)
						},
					),
				)
			})

			ginkgo.It("should return an error wrapping ErrEphemeralCreateFailed", func() {
				orchestratorID, err := testClient.CreateEphemeralOrchestrator(
					ctx, source, "watchtower:v2", "chain1", false,
				)

				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(
					ErrEphemeralCreateFailed.Error(),
				)))
				gomega.Expect(orchestratorID).To(gomega.BeEmpty())
			})
		})

		ginkgo.When("ContainerStart fails", func() {
			ginkgo.BeforeEach(func() {
				mockServer.AppendHandlers(
					// ContainerCreate succeeds.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", gomega.HaveSuffix("/containers/create")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.Header().Set("Content-Type", "application/json")

							var buf bytes.Buffer

							err := json.NewEncoder(&buf).Encode(dockerContainer.CreateResponse{
								ID: "orchestrator-fail-start",
							})
							if err != nil {
								ginkgo.GinkgoWriter.Printf(
									"failed to encode CreateResponse: %v\n", err,
								)
								gomega.Expect(err).NotTo(gomega.HaveOccurred())

								return
							}

							w.WriteHeader(http.StatusCreated)
							_, _ = w.Write(buf.Bytes())
						},
					),
					// ContainerStart fails.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", gomega.HaveSuffix("/containers/orchestrator-fail-start/start")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusInternalServerError)
						},
					),
					// ContainerRemove for cleanup.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", gomega.HaveSuffix("/containers/orchestrator-fail-start")),
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusNoContent)
						},
					),
				)
			})

			ginkgo.It("should attempt cleanup and return ErrEphemeralStartFailed", func() {
				orchestratorID, err := testClient.CreateEphemeralOrchestrator(
					ctx, source, "watchtower:v2", "chain1", false,
				)

				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(
					ErrEphemeralStartFailed.Error(),
				)))
				gomega.Expect(orchestratorID).To(gomega.BeEmpty())
				// Verify all handlers were called (including cleanup remove).
				gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(4))
			})
		})
	})

	ginkgo.Describe("RemoveOrphanedOrchestrators", func() {
		var ctx context.Context

		// orphaned marks an orchestrator as running long past any self-update.
		orphaned := WithContainerState(dockerContainer.State{
			Running:   true,
			Status:    "running",
			StartedAt: "2000-01-01T00:00:00Z",
		})

		ginkgo.BeforeEach(func() {
			ctx = context.Background()
		})

		ginkgo.When("there are no containers", func() {
			ginkgo.It("should return 0 removed and no error", func() {
				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{}, nil)

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(0))
			})
		})

		ginkgo.When("there are containers but none are orchestrators", func() {
			ginkgo.It("should return 0 removed and no error", func() {
				regularContainer := MockContainer(
					WithID("regular123"),
					WithName("regular-container"),
					WithLabels(map[string]string{
						"com.centurylinklabs.watchtower": "true",
					}),
				)

				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{regularContainer}, nil)

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(0))
			})
		})

		ginkgo.When("there is an orphaned orchestrator", func() {
			ginkgo.It("should remove the orchestrator and return 1", func() {
				orchestrator := MockContainer(
					WithID("orch123"),
					WithName("watchtower-orchestrator-abc"),
					WithLabels(map[string]string{
						OrchestratorLabel: "true",
					}),
					orphaned,
				)

				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
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

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(1))
			})
		})

		ginkgo.When("there are multiple orphaned orchestrators", func() {
			ginkgo.It("should remove all orchestrators and return the count", func() {
				orch1 := MockContainer(
					WithID("orch001"),
					WithName("watchtower-orchestrator-001"),
					WithLabels(map[string]string{
						OrchestratorLabel: "true",
					}),
					orphaned,
				)
				orch2 := MockContainer(
					WithID("orch002"),
					WithName("watchtower-orchestrator-002"),
					WithLabels(map[string]string{
						OrchestratorLabel: "true",
					}),
					orphaned,
				)
				regular := MockContainer(
					WithID("regular1"),
					WithName("app-container"),
				)

				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
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

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(2))
			})
		})

		ginkgo.When("ListContainers fails", func() {
			ginkgo.It("should return an error", func() {
				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return(nil, errors.New("docker daemon unavailable"))

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(
					"failed to list containers",
				)))
				gomega.Expect(count).To(gomega.Equal(0))
			})
		})

		ginkgo.When("StopAndRemoveContainer fails for one orchestrator", func() {
			ginkgo.It("should continue and remove others, returning partial count", func() {
				orch1 := MockContainer(
					WithID("orch001"),
					WithName("watchtower-orchestrator-001"),
					WithLabels(map[string]string{
						OrchestratorLabel: "true",
					}),
					orphaned,
				)
				orch2 := MockContainer(
					WithID("orch002"),
					WithName("watchtower-orchestrator-002"),
					WithLabels(map[string]string{
						OrchestratorLabel: "true",
					}),
					orphaned,
				)

				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
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

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(1))
			})
		})

		ginkgo.When("a container has no orchestrator label", func() {
			ginkgo.It("should skip the container without error", func() {
				noLabelContainer := MockContainer(
					WithID("no-label"),
					WithName("no-label-container"),
				)

				mockAPIClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
				mockAPIClient.EXPECT().
					ListContainers(ctx).
					Return([]types.Container{noLabelContainer}, nil)

				count, err := RemoveOrphanedOrchestrators(testLog(), ctx, mockAPIClient)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(count).To(gomega.Equal(0))
			})
		})
	})
})

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

			config := extractDockerConnectionConfig(source, nopLogger())

			assert.Equal(t, tt.wantLocal, config.IsLocal)
			assert.Equal(t, tt.wantBind, config.SocketBind)
		})
	}
}
