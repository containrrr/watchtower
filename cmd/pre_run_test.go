package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// preRunExitHelperEnv selects a preRun exit case when the test binary is
// re-executed as a subprocess. Empty means the parent process.
const preRunExitHelperEnv = "WATCHTOWER_CMD_PRE_RUN_EXIT_HELPER"

// selfID is the container ID the preRun tests detect for Watchtower itself.
var selfID = types.ContainerID(strings.Repeat("c", 64))

// selfDetection is what the self container ID detection can read: the
// mountinfo and cgroup files, and the HOSTNAME variable. Empty means absent.
type selfDetection struct {
	mountinfo string
	cgroup    string
	hostname  string
	// marker creates the Docker container marker file.
	marker bool
}

// mountinfoWithSelf is a mountinfo file that names the Watchtower container.
func mountinfoWithSelf() string {
	return "1234 24 0:52 /var/lib/docker/containers/" + string(selfID) +
		"/hostname /etc/hostname rw,relatime - ext4 /dev/sda1 rw\n"
}

// cgroupWithSelf is a cgroup file that names the Watchtower container.
func cgroupWithSelf() string {
	return "12:memory:/docker/" + string(selfID) + "\n"
}

// setPreRunState prepares the process state preRun reads and writes, and
// restores it when the test ends. newClient returns client, self container ID
// detection reads only detection, and the Docker variables and registry TLS
// settings preRun sets are restored.
func setPreRunState(t *testing.T, client container.Client, detection selfDetection) {
	t.Helper()

	setRunState(t, runTestState{})
	clearRegistryTLSOverrides(t)

	savedNewClient := newClient
	savedMountinfo := container.ReadMountinfoFunc
	savedCgroup := container.ReadCgroupFunc
	savedProcessFs := processFs

	t.Cleanup(func() {
		newClient = savedNewClient
		container.ReadMountinfoFunc = savedMountinfo
		container.ReadCgroupFunc = savedCgroup
		processFs = savedProcessFs
	})

	processFs = afero.NewMemMapFs()
	if detection.marker {
		require.NoError(t, afero.WriteFile(processFs, "/.dockerenv", nil, 0o600))
	}

	newClient = func(*zerolog.Logger, container.ClientOptions) container.Client { return client }
	container.ReadMountinfoFunc = readFixture(detection.mountinfo)
	container.ReadCgroupFunc = readFixture(detection.cgroup)

	for _, key := range []string{"HOSTNAME", "DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_API_VERSION", "DOCKER_CERT_PATH"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}

	if detection.hostname != "" {
		t.Setenv("HOSTNAME", detection.hostname)
	}

	// preRun replaces the notifier with one hooked to its logger.
	t.Cleanup(func() { notifier.Close() })
}

// readFixture returns a file reader that returns content, or a not-found
// error when content is empty.
func readFixture(content string) func(string) ([]byte, error) {
	return func(string) ([]byte, error) {
		if content == "" {
			return nil, os.ErrNotExist
		}

		return []byte(content), nil
	}
}

// selfContainer returns the Watchtower container preRun detects, with the
// given name and container hostname.
func selfContainer(name, hostname string) types.Container {
	return container.NewContainer(nil,
		&dockerContainer.InspectResponse{
			ID:      string(selfID),
			Name:    name,
			Image:   "sha256:" + string(selfID),
			Created: "2026-10-08T12:00:00Z",
			State:   &dockerContainer.State{Running: true},
			Config: &dockerContainer.Config{
				Hostname: hostname,
				Image:    "nickfedor/watchtower:latest",
				Labels:   map[string]string{watchtowerLabelKey: "true"},
			},
			HostConfig: &dockerContainer.HostConfig{},
		},
		&dockerImage.InspectResponse{ID: "sha256:" + string(selfID)},
	)
}

// TestPreRun_SelfDetection records how preRun finds the Watchtower container
// it runs in, and what it keeps when detection or the lookup fails. A process
// in a container whose ID is not known is marked unknown, unlike one outside a
// container.
func TestPreRun_SelfDetection(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		detection selfDetection
		setup     func(client *mockContainer.MockClient)
		wantID    types.ContainerID
		wantName  string
		wantCalls []string
		// wantUnknown is whether the own container is marked unknown.
		wantUnknown bool
	}{
		{
			name:      "outside a container",
			wantCalls: []string{},
		},
		{
			name:        "detection fails in a container",
			detection:   selfDetection{marker: true},
			wantCalls:   []string{},
			wantUnknown: true,
		},
		{
			name:      "found from mountinfo",
			detection: selfDetection{mountinfo: mountinfoWithSelf()},
			setup: func(client *mockContainer.MockClient) {
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).
					Return(selfContainer("/watchtower", ""), nil)
			},
			wantID:    selfID,
			wantName:  "watchtower",
			wantCalls: []string{"GetCurrentWatchtowerContainer cccccccccccc"},
		},
		{
			name:      "found from cgroup",
			detection: selfDetection{cgroup: cgroupWithSelf()},
			setup: func(client *mockContainer.MockClient) {
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).
					Return(selfContainer("/watchtower", ""), nil)
			},
			wantID:    selfID,
			wantName:  "watchtower",
			wantCalls: []string{"GetCurrentWatchtowerContainer cccccccccccc"},
		},
		{
			name:      "found from hostname",
			detection: selfDetection{hostname: "wt-host"},
			setup: func(client *mockContainer.MockClient) {
				self := selfContainer("/watchtower", "wt-host")
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{self}, nil)
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).Return(self, nil)
			},
			wantID:    selfID,
			wantName:  "watchtower",
			wantCalls: []string{"ListContainers all", "GetCurrentWatchtowerContainer cccccccccccc"},
		},
		{
			name:      "lookup fails and the ID is kept",
			detection: selfDetection{mountinfo: mountinfoWithSelf()},
			setup: func(client *mockContainer.MockClient) {
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).Return(nil, errRunTestDocker)
			},
			wantID:    selfID,
			wantCalls: []string{"GetCurrentWatchtowerContainer cccccccccccc"},
		},
		{
			name:      "lookup times out and the ID is cleared",
			detection: selfDetection{mountinfo: mountinfoWithSelf(), marker: true},
			setup: func(client *mockContainer.MockClient) {
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).
					Return(nil, fmt.Errorf("inspect: %w", context.DeadlineExceeded))
			},
			wantCalls:   []string{"GetCurrentWatchtowerContainer cccccccccccc"},
			wantUnknown: true,
		},
		{
			name:      "old instance continues in run-once mode",
			args:      []string{"--run-once"},
			detection: selfDetection{mountinfo: mountinfoWithSelf()},
			setup: func(client *mockContainer.MockClient) {
				client.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).
					Return(selfContainer("/watchtower-old-cccccccccccc", ""), nil)
			},
			wantID:    selfID,
			wantName:  "watchtower-old-cccccccccccc",
			wantCalls: []string{"GetCurrentWatchtowerContainer cccccccccccc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockContainer.NewMockClient(t)
			if tt.setup != nil {
				tt.setup(mockClient)
			}

			setPreRunState(t, mockClient, tt.detection)

			fixtureNotifier := notifier

			newRunProcess().preRun(newTestRootCommand(t, tt.args...), nil)

			assert.Equal(t, tt.wantID, currentWatchtowerContainerID)
			assert.Equal(t, tt.wantUnknown, currentWatchtowerContainerUnknown)
			assert.Equal(t, tt.wantName, containerName(currentWatchtowerContainer))
			assert.Equal(t, tt.wantCalls, runCalls(mockClient))
			assert.Same(t, mockClient, client, "preRun uses the client from newClient")
			assert.NotSame(t, fixtureNotifier, notifier, "preRun creates its own notifier")
		})
	}
}

// containerName names a container, or returns "" when it is nil.
func containerName(c types.Container) string {
	if c == nil {
		return ""
	}

	return strings.TrimPrefix(c.Name(), "/")
}

// TestPreRun_Configuration verifies that preRun loads the configuration from
// the flags and exports the Docker connection settings to the environment,
// where the Docker client reads them.
func TestPreRun_Configuration(t *testing.T) {
	setPreRunState(t, mockContainer.NewMockClient(t), selfDetection{})

	newRunProcess().preRun(newTestRootCommand(t,
		"--scope", "prod",
		"--host", "tcp://docker.example.com:2376",
		"--tlsverify",
		"--api-version", "1.47",
	), nil)

	assert.Equal(t, "prod", appCfg.Filter.Scope)
	// With TLS verification enabled, a tcp:// host is exported as https://.
	assert.Equal(t, "https://docker.example.com:2376", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "1", os.Getenv("DOCKER_TLS_VERIFY"))
	assert.Equal(t, "1.47", os.Getenv("DOCKER_API_VERSION"))
}

// TestPreRun_RegistryTLSSettings verifies that the registry TLS settings the
// registry client reads follow the registry-tls-skip and
// registry-tls-min-version configuration options, whether set by flag or by
// environment variable, with a flag taking precedence over its environment
// variable.
func TestPreRun_RegistryTLSSettings(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		wantSkip       bool
		wantMinVersion string
	}{
		{
			// The minimum version flag defaults to TLS1.2, the same minimum the
			// registry client applies when the setting is empty.
			name:           "neither set",
			wantMinVersion: "TLS1.2",
		},
		{
			name:           "flags",
			args:           []string{"--registry-tls-skip", "--registry-tls-min-version", "TLS1.3"},
			wantSkip:       true,
			wantMinVersion: "TLS1.3",
		},
		{
			name: "environment variables",
			env: map[string]string{
				"WATCHTOWER_REGISTRY_TLS_SKIP":        "true",
				"WATCHTOWER_REGISTRY_TLS_MIN_VERSION": "TLS1.3",
			},
			wantSkip:       true,
			wantMinVersion: "TLS1.3",
		},
		{
			name: "flags override environment variables",
			args: []string{"--registry-tls-skip=false", "--registry-tls-min-version", "TLS1.3"},
			env: map[string]string{
				"WATCHTOWER_REGISTRY_TLS_SKIP":        "true",
				"WATCHTOWER_REGISTRY_TLS_MIN_VERSION": "TLS1.2",
			},
			wantSkip:       false,
			wantMinVersion: "TLS1.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPreRunState(t, mockContainer.NewMockClient(t), selfDetection{})

			cmd := newTestRootCommand(t, tt.args...)

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			newRunProcess().preRun(cmd, nil)

			assert.Equal(t, tt.wantSkip, viper.GetBool("WATCHTOWER_REGISTRY_TLS_SKIP"))
			assert.Equal(t, tt.wantMinVersion, viper.GetString("WATCHTOWER_REGISTRY_TLS_MIN_VERSION"))
		})
	}
}

// TestPreRun_PorcelainKeepsExplicitSettings verifies that the porcelain
// configuration option only fills in the notification template, report, and
// log-stdout settings that were not set, and treats a setting from an
// environment variable the same as one from a flag.
func TestPreRun_PorcelainKeepsExplicitSettings(t *testing.T) {
	const custom = "{{range .}}{{.Message}}{{end}}"

	tests := []struct {
		name          string
		args          []string
		env           map[string]string
		wantTemplate  string
		wantReport    bool
		wantLogStdout bool
	}{
		{
			name:          "porcelain alone",
			env:           map[string]string{"WATCHTOWER_PORCELAIN": "v1"},
			wantTemplate:  "porcelain.v1.summary-no-log",
			wantReport:    true,
			wantLogStdout: true,
		},
		{
			name:          "settings from flags",
			args:          []string{"--porcelain", "v1", "--notification-template", custom, "--notification-report=false", "--notification-log-stdout=false"},
			wantTemplate:  custom,
			wantReport:    false,
			wantLogStdout: false,
		},
		{
			name: "settings from environment variables",
			env: map[string]string{
				"WATCHTOWER_PORCELAIN":               "v1",
				"WATCHTOWER_NOTIFICATION_TEMPLATE":   custom,
				"WATCHTOWER_NOTIFICATION_REPORT":     "false",
				"WATCHTOWER_NOTIFICATION_LOG_STDOUT": "false",
			},
			wantTemplate:  custom,
			wantReport:    false,
			wantLogStdout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPreRunState(t, mockContainer.NewMockClient(t), selfDetection{})

			cmd := newTestRootCommand(t, tt.args...)

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			newRunProcess().preRun(cmd, nil)

			assert.Equal(t, tt.wantTemplate, appCfg.Notify.Template)
			assert.Equal(t, tt.wantReport, appCfg.Notify.Report)
			assert.Equal(t, tt.wantLogStdout, appCfg.Notify.LogStdout)
		})
	}
}

// clearRegistryTLSOverrides clears the registry TLS settings preRun sets on
// the process-wide Viper instance when the test ends. Only preRun sets them,
// so clearing them, rather than setting their earlier values, leaves Viper
// reading them from the environment as before.
func clearRegistryTLSOverrides(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		viper.Set("WATCHTOWER_REGISTRY_TLS_SKIP", nil)
		viper.Set("WATCHTOWER_REGISTRY_TLS_MIN_VERSION", nil)
	})
}

// TestPreRun_Exit verifies how preRun ends the process when it cannot or
// must not continue.
func TestPreRun_Exit(t *testing.T) {
	if helperCase := os.Getenv(preRunExitHelperEnv); helperCase != "" {
		preRunExitHelper(t, helperCase)

		// Every helper case must end the process. Reaching here means it did not.
		os.Exit(2)
	}

	tests := []struct {
		helperCase   string
		wantExitCode int
		wantOutput   string
	}{
		{helperCase: "invalid-log-format", wantExitCode: 1, wantOutput: "Failed to initialize logging"},
		{helperCase: "invalid-configuration", wantExitCode: 1, wantOutput: "Failed to load configuration"},
		{
			// RunOrchestrator always ends the process, so the restart policy
			// fallback after it in preRun does not run.
			helperCase:   "orchestrator-without-settings",
			wantExitCode: 1,
			wantOutput:   "Failed to read orchestrator environment variables",
		},
		{
			helperCase:   "invalid-restart-of-old-instance",
			wantExitCode: 0,
			wantOutput:   "SetRestartPolicy watchtower-old-cccccccccccc no",
		},
		{
			helperCase:   "invalid-restart-of-chain-parent",
			wantExitCode: 0,
			wantOutput:   "SetRestartPolicy watchtower no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.helperCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPreRun_Exit$", "-test.v=false")

			cmd.Env = append(os.Environ(), preRunExitHelperEnv+"="+tt.helperCase)

			out, err := cmd.CombinedOutput()

			exitCode := 0

			var exitErr *exec.ExitError

			switch {
			case errors.As(err, &exitErr):
				exitCode = exitErr.ExitCode()
			default:
				require.NoError(t, err, "output:\n%s", out)
			}

			assert.Equal(t, tt.wantExitCode, exitCode, "output:\n%s", out)
			assert.Contains(t, string(out), tt.wantOutput)
			assert.NotContains(t, string(out), "PASS", "the helper must end the process")
		})
	}
}

// preRunExitHelper runs the named exit case in the re-executed test binary.
// The Docker client prints each restart policy change so the parent can
// check it.
func preRunExitHelper(t *testing.T, helperCase string) {
	t.Helper()

	mockClient := mockContainer.NewMockClient(t)
	mockClient.EXPECT().SetRestartPolicy(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, c types.Container, policy dockerContainer.RestartPolicy) {
			fmt.Fprintf(os.Stderr, "SetRestartPolicy %s %s\n", containerLabel(c), policy.Name)
		}).Maybe()

	var (
		args      []string
		detection selfDetection
	)

	switch helperCase {
	case "invalid-log-format":
		args = []string{"--log-format", "bogus"}
	case "invalid-configuration":
		args = []string{"--rolling-restart", "--monitor-only"}
	case "orchestrator-without-settings":
		args = []string{"--self-update-orchestrator"}
	case "invalid-restart-of-old-instance":
		old := selfContainer("/watchtower-old-cccccccccccc", "")
		detection = selfDetection{mountinfo: mountinfoWithSelf()}

		mockClient.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).Return(old, nil)
		mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{old}, nil).Maybe()
	case "invalid-restart-of-chain-parent":
		// The container's chain lists its own ID, which marks it as the parent
		// of a newer instance.
		parent := selfContainer("/watchtower", "")
		parent.ContainerInfo().Config.Labels[container.ContainerChainLabel] = strings.Repeat("a", 64) + "," + string(selfID)
		detection = selfDetection{mountinfo: mountinfoWithSelf()}

		mockClient.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, selfID).Return(parent, nil)
		mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{parent}, nil).Maybe()
	default:
		panic("unknown preRun exit helper case " + helperCase)
	}

	setPreRunState(t, mockClient, detection)

	for _, key := range []string{"WT_ORCHESTRATOR_OLD_ID", "WT_ORCHESTRATOR_NEW_IMAGE", "WT_ORCHESTRATOR_ORIGINAL_NAME"} {
		t.Setenv(key, "")
	}

	proc := &process{log: logging.New(os.Stderr, logging.InfoLevel)}
	proc.preRun(newTestRootCommand(t, args...), nil)
}
