package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

const testHostname = "test-hostname"

// writeProcFile writes content to path in fsys, failing the test on error.
func writeProcFile(t *testing.T, fsys afero.Fs, path, content string) {
	t.Helper()

	require.NoError(t, afero.WriteFile(fsys, path, []byte(content), 0o600))
}

// cgroupFile is the cgroup file self container ID detection reads for this process.
func cgroupFile() string {
	return fmt.Sprintf("/proc/%d/cgroup", os.Getpid())
}

// TestGetContainerIDFromCgroupFile verifies that the container ID is read from
// this process's cgroup file, and that a missing file or one without an ID fails.
func TestGetContainerIDFromCgroupFile(t *testing.T) {
	t.Parallel()

	t.Run("should return the correct container ID", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			setup   func(t *testing.T, fsys afero.Fs)
			want    types.ContainerID
			wantErr error
		}{
			{name: "SuccessWithValidID", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, cgroupFile(),
					"11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				)
			}, want: types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			), wantErr: nil},
			{name: "FileNotReadable", setup: nil, want: types.ContainerID(""), wantErr: errReadCgroupFile},
			{name: "NoValidID", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, cgroupFile(), "11:perf_event:/user.slice\n10:cpu:/system.slice")
			}, want: types.ContainerID(""), wantErr: errExtractContainerID},
			{name: "EmptyFileContent", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, cgroupFile(), "")
			}, want: types.ContainerID(""), wantErr: errExtractContainerID},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				setup, want, wantErr := tt.setup, tt.want, tt.wantErr

				fsys := afero.NewMemMapFs()
				if setup != nil {
					setup(t, fsys)
				}

				got, err := GetContainerIDFromCgroupFile(newTestLogger(t), fsys)
				if wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, wantErr)
				}

				assert.Equal(t, want, got)
			})
		}
	})
}

// Test_parseContainerIDFromCgroupString verifies that the container ID is found
// in single-line and multi-line cgroup content, and not in content without a
// Docker path or a full ID.
func Test_parseContainerIDFromCgroupString(t *testing.T) {
	t.Parallel()

	t.Run("should extract container ID from string", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			s       string
			want    types.ContainerID
			wantErr error
		}{
			{name: "ValidDockerContainerIDSingleLine", s: "11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", want: types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			), wantErr: nil},
			{
				name: "ValidDockerContainerIDMultiLine",
				s:    "12:memory:/user.slice\n11:perf_event:/docker/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800\n10:cpu:/system.slice",
				want: types.ContainerID(
					"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
				),
				wantErr: nil,
			},
			{name: "NoDockerPatternMultiLine", s: "11:perf_event:/user.slice\n10:cpu:/system.slice", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{name: "EmptyString", s: "", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{name: "InvalidIDLength", s: "11:perf_event:/docker/12345678", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{name: "NonHexID", s: "11:perf_event:/docker/gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{name: "ValidIDWithExtraLines", s: "11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef\n10:cpu:/system.slice", want: types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			), wantErr: nil},
			{name: "NoDockerPatternSingleLine", s: "11:perf_event:/user.slice", want: types.ContainerID(""), wantErr: errNoValidContainerID},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, want, wantErr := tt.s, tt.want, tt.wantErr

				got, err := parseContainerIDFromCgroupString(newTestLogger(t), s)
				if wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, wantErr)
				}

				assert.Equal(t, want, got)
			})
		}
	})
}

// TestGetContainerIDFromMountinfo verifies that the container ID is read from
// the root or mount point of /proc/self/mountinfo, and that a missing file or
// one without an ID fails.
func TestGetContainerIDFromMountinfo(t *testing.T) {
	t.Parallel()

	t.Run("should return the correct container ID", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			setup   func(t *testing.T, fsys afero.Fs)
			want    types.ContainerID
			wantErr error
		}{
			{name: "SuccessWithValidIDInRoot", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, "/proc/self/mountinfo",
					"36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf /etc/resolv.conf rw,noatime - btrfs /dev/sda1 rw",
				)
			}, want: types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			), wantErr: nil},
			{name: "SuccessWithValidIDInMountPoint", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, "/proc/self/mountinfo",
					"36 35 98:0 /var/lib/docker/containers/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800/hostname /etc/hostname rw,noatime - btrfs /dev/sda1 rw",
				)
			}, want: types.ContainerID(
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
			), wantErr: nil},
			{name: "MountinfoFileNotReadable", setup: nil, want: types.ContainerID(""), wantErr: errReadMountinfoFile},
			{name: "NoValidIDInMountinfo", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, "/proc/self/mountinfo", "36 35 98:0 / rw,relatime shared:1 - overlay ")
			}, want: types.ContainerID(""), wantErr: errExtractContainerIDFromMountinfo},
			{name: "EmptyMountinfoFileContent", setup: func(t *testing.T, fsys afero.Fs) {
				t.Helper()

				writeProcFile(t, fsys, "/proc/self/mountinfo", "")
			}, want: types.ContainerID(""), wantErr: errExtractContainerIDFromMountinfo},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				setup, want, wantErr := tt.setup, tt.want, tt.wantErr

				fsys := afero.NewMemMapFs()
				if setup != nil {
					setup(t, fsys)
				}

				got, err := GetContainerIDFromMountinfo(newTestLogger(t), fsys)
				if wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, wantErr)
				}

				assert.Equal(t, want, got)
			})
		}
	})
}

// Test_parseContainerIDFromMountinfo verifies that the container ID is found in
// the root or mount point fields of mountinfo content, and not in malformed
// lines.
func Test_parseContainerIDFromMountinfo(t *testing.T) {
	t.Parallel()

	t.Run("should extract container ID from mountinfo string", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			s       string
			want    types.ContainerID
			wantErr error
		}{
			{
				name: "ValidDockerContainerIDInRoot",
				s:    "36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf /etc/resolv.conf rw,noatime - btrfs /dev/sda1 rw",
				want: types.ContainerID(
					"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				),
				wantErr: nil,
			},
			{
				name: "ValidDockerContainerIDInMountPoint",
				s:    "36 35 98:0 /var/lib/docker/containers/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800/hostname /etc/hostname rw,noatime - btrfs /dev/sda1 rw",
				want: types.ContainerID(
					"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
				),
				wantErr: nil,
			},
			{
				name:    "NoValidMountinfoLine",
				s:       "36 35 98:0 / rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...",
				want:    types.ContainerID(""),
				wantErr: errNoValidContainerID,
			},
			{name: "EmptyString", s: "", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{name: "InvalidMountinfoLine", s: "invalid line format", want: types.ContainerID(""), wantErr: errNoValidContainerID},
			{
				name: "ValidIDWithExtraLines",
				s:    "36 35 98:0 /tmp rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...\n36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/hosts /etc/hosts rw,noatime - btrfs /dev/sda1 rw\n36 35 98:0 /var rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...",
				want: types.ContainerID(
					"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				),
				wantErr: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, want, wantErr := tt.s, tt.want, tt.wantErr

				got, err := parseContainerIDFromMountinfo(newTestLogger(t), s)
				if wantErr == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, wantErr)
				}

				assert.Equal(t, want, got)
			})
		}
	})
}

// Test_extractContainerIDFromPath verifies that a full hexadecimal container ID
// is taken from a Docker cgroup path, and that other paths yield no ID.
func Test_extractContainerIDFromPath(t *testing.T) {
	t.Parallel()

	t.Run("should extract container ID from path", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			path string
			want types.ContainerID
		}{
			{
				name: "ValidDockerPath",
				path: "/var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf",
				want: types.ContainerID("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
			},
			{name: "NoDockerPath", path: "/sys/fs/cgroup", want: types.ContainerID("")},
			{name: "InvalidLength", path: "/docker/12345678", want: types.ContainerID("")},
			{name: "NonHexID", path: "/docker/gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg", want: types.ContainerID("")},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				path, want := tt.path, tt.want

				got := extractContainerIDFromPath(path)
				assert.Equal(t, want, got)
			})
		}
	})
}

// TestGetContainerIDFromHostname verifies that the container ID is resolved from
// the Watchtower container whose hostname matches HOSTNAME, preferring one that
// is not an old instance, and that listing failures and missing matches fail.
//
// The test sets environment variables, so it does not run in parallel.
func TestGetContainerIDFromHostname(t *testing.T) {
	t.Run("should return the correct container ID or error", func(t *testing.T) {
		tests := []struct {
			name           string
			setup          func(t *testing.T, mockClient *mockContainer.MockClient)
			want           types.ContainerID
			wantErr        bool
			errorSubstring string
		}{
			{name: "when a container with matching hostname is found", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				expectedID := types.ContainerID("hostname-container-id")
				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create a mock container with matching hostname
				mockContainer := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = string(expectedID)
					},
				)

				mockClient.EXPECT().ListContainers(context.Background()).Return([]types.Container{mockContainer}, nil)
			}, want: types.ContainerID("hostname-container-id"), wantErr: false, errorSubstring: ""},
			{name: "when no container with matching hostname is found", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname
				differentHostname := "different-hostname"

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create a mock container with different hostname
				mockContainer := MockContainer(
					WithHostname(differentHostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "different-container-id"
					},
				)

				mockClient.EXPECT().ListContainers(context.Background()).Return([]types.Container{mockContainer}, nil)
			}, want: types.ContainerID(""), wantErr: true, errorSubstring: "no container found with matching hostname"},
			{name: "when HOSTNAME environment variable is not set", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				// HOSTNAME is not set.
			}, want: types.ContainerID(""), wantErr: true, errorSubstring: "HOSTNAME environment variable is not set"},
			{name: "when container listing fails", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				expectedError := errors.New("failed to list containers")

				mockClient.EXPECT().ListContainers(context.Background()).Return(nil, expectedError)
			}, want: types.ContainerID(""), wantErr: true, errorSubstring: "failed to list all containers"},
			{name: "when container info is nil", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create containers: one with nil info, one with matching hostname
				mockContainerNilInfo := mockTypes.NewMockContainer(t)
				mockContainerNilInfo.EXPECT().ContainerInfo().Return(nil)

				expectedID := types.ContainerID("matching-container-id")
				mockContainerMatching := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = string(expectedID)
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{mockContainerNilInfo, mockContainerMatching}, nil)
			}, want: types.ContainerID("matching-container-id"), wantErr: false, errorSubstring: ""},
			{name: "when container config is nil", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create containers: one with nil config, one with matching hostname
				mockContainerNilConfig := MockContainer(
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.Config = nil
					},
				)

				expectedID := types.ContainerID("matching-container-id")
				mockContainerMatching := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = string(expectedID)
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{mockContainerNilConfig, mockContainerMatching}, nil)
			}, want: types.ContainerID("matching-container-id"), wantErr: false, errorSubstring: ""},
			{name: "when multiple containers exist but only one matches hostname", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname
				expectedID := types.ContainerID("matching-container-id")

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create multiple containers: some with different hostnames, one matching
				mockContainer1 := MockContainer(
					WithHostname("different-hostname-1"),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "container-1-id"
					},
				)

				mockContainer2 := MockContainer(
					WithHostname("different-hostname-2"),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "container-2-id"
					},
				)

				mockContainerMatching := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = string(expectedID)
					},
				)

				mockContainer3 := MockContainer(
					WithHostname("different-hostname-3"),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "container-3-id"
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{mockContainer1, mockContainer2, mockContainerMatching, mockContainer3}, nil)
			}, want: types.ContainerID("matching-container-id"), wantErr: false, errorSubstring: ""},
			{name: "when multiple containers share hostname but only one is Watchtower", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Create a non-Watchtower container with matching hostname (first match)
				nonWatchtowerContainer := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "non-watchtower-id"
					},
				)

				// Create a Watchtower container with matching hostname (second match)
				watchtowerContainer := MockContainer(
					WithHostname(hostname),
					WithLabels(map[string]string{
						"com.centurylinklabs.watchtower": "true",
					}),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "watchtower-container-id"
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{nonWatchtowerContainer, watchtowerContainer}, nil)
			}, want: types.ContainerID("watchtower-container-id"), wantErr: false, errorSubstring: ""},
			{name: "when multiple Watchtower containers share hostname, prefer the non-old one", setup: func(t *testing.T, mockClient *mockContainer.MockClient) {
				t.Helper()

				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// Old first in list (simulates arbitrary daemon order)
				oldContainer := MockContainer(
					WithHostname(hostname),
					WithName("watchtower-old-6bfdffce0700"),
					WithLabels(map[string]string{
						"com.centurylinklabs.watchtower": "true",
					}),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "old-watchtower-id"
					},
				)

				// Main (non-old) Watchtower
				mainNamed := MockContainer(
					WithHostname(hostname),
					WithName("watchtower"),
					WithLabels(map[string]string{
						"com.centurylinklabs.watchtower": "true",
					}),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = "main-watchtower-id"
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{oldContainer, mainNamed}, nil)
			}, want: types.ContainerID("main-watchtower-id"), wantErr: false, errorSubstring: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Each case starts without HOSTNAME, and t.Setenv restores it.
				t.Setenv("HOSTNAME", "")
				require.NoError(t, os.Unsetenv("HOSTNAME"))

				mockClient := mockContainer.NewMockClient(t)

				setup, want, wantErr, errorSubstring := tt.setup, tt.want, tt.wantErr, tt.errorSubstring

				if setup != nil {
					setup(t, mockClient)
				}

				got, err := GetContainerIDFromHostname(newTestLogger(t), context.Background(), mockClient)
				if wantErr {
					require.Error(t, err)
					assert.Empty(t, got)

					if errorSubstring != "" {
						assert.Contains(t, err.Error(), errorSubstring)
					}
				} else {
					require.NoError(t, err)
					assert.Equal(t, want, got)
				}
			})
		}
	})
}

// TestGetCurrentContainerID verifies that self container ID detection tries the
// mountinfo file, then the cgroup file, then the hostname, and fails when all
// three fail.
//
// The test sets environment variables, so it does not run in parallel.
func TestGetCurrentContainerID(t *testing.T) {
	t.Run("should return the correct container ID or error", func(t *testing.T) {
		tests := []struct {
			name           string
			setup          func(t *testing.T, fsys afero.Fs, mockClient *mockContainer.MockClient)
			expectedID     types.ContainerID
			expectError    bool
			errorSubstring string
		}{
			{name: "when cgroup v2 detection succeeds", setup: func(t *testing.T, fsys afero.Fs, mockClient *mockContainer.MockClient) {
				t.Helper()

				expectedID := types.ContainerID(
					"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				)

				// cgroup v2 detection succeeds.
				writeProcFile(t, fsys, "/proc/self/mountinfo",
					"36 35 98:0 /var/lib/docker/containers/"+string(
						expectedID,
					)+"/resolv.conf /etc/resolv.conf rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...,\n",
				)
			}, expectedID: types.ContainerID("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), expectError: false, errorSubstring: ""},
			{name: "when cgroup v2 detection fails but cgroup v1 detection succeeds", setup: func(t *testing.T, fsys afero.Fs, mockClient *mockContainer.MockClient) {
				t.Helper()

				expectedID := types.ContainerID(
					"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				)

				// No mountinfo file, so cgroup v2 detection fails and cgroup v1 detection succeeds.
				writeProcFile(t, fsys, cgroupFile(), "11:perf_event:/docker/"+string(expectedID)+"\n")
			}, expectedID: types.ContainerID(
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			), expectError: false, errorSubstring: ""},
			{name: "when both cgroup v2 and cgroup v1 detection fail but hostname detection succeeds", setup: func(t *testing.T, fsys afero.Fs, mockClient *mockContainer.MockClient) {
				t.Helper()

				expectedID := types.ContainerID("hostname-container-id")
				hostname := testHostname

				// Set HOSTNAME environment variable
				t.Setenv("HOSTNAME", hostname)

				// No mountinfo or cgroup file, so both cgroup detections fail.

				// Create a mock container with matching hostname
				mockContainer := MockContainer(
					WithHostname(hostname),
					func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
						c.ID = string(expectedID)
					},
				)

				mockClient.EXPECT().
					ListContainers(context.Background()).
					Return([]types.Container{mockContainer}, nil).
					Times(1)
			}, expectedID: types.ContainerID("hostname-container-id"), expectError: false, errorSubstring: ""},
			{name: "when all detection methods fail", setup: func(t *testing.T, fsys afero.Fs, mockClient *mockContainer.MockClient) {
				t.Helper()

				// No mountinfo or cgroup file, and HOSTNAME is not set, so hostname
				// detection fails without calling the client.
			}, expectedID: types.ContainerID(""), expectError: true, errorSubstring: "failed to detect current container ID"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Each case starts without HOSTNAME, and t.Setenv restores it.
				t.Setenv("HOSTNAME", "")
				require.NoError(t, os.Unsetenv("HOSTNAME"))

				mockClient := mockContainer.NewMockClient(t)

				setup, expectedID, expectError, errorSubstring := tt.setup, tt.expectedID, tt.expectError, tt.errorSubstring

				fsys := afero.NewMemMapFs()
				if setup != nil {
					setup(t, fsys, mockClient)
				}

				id, err := GetCurrentContainerID(newTestLogger(t), context.Background(), fsys, mockClient)
				if expectError {
					require.Error(t, err)
					assert.Empty(t, id)

					if errorSubstring != "" {
						assert.Contains(t, err.Error(), errorSubstring)
					}
				} else {
					require.NoError(t, err)
					assert.Equal(t, expectedID, id)
				}
			})
		}
	})
}

// statErrorFs is a filesystem whose Stat fails with err for every path.
type statErrorFs struct {
	afero.Fs

	// err is returned by every Stat call.
	err error
}

// Stat returns the configured error.
func (f statErrorFs) Stat(string) (os.FileInfo, error) {
	return nil, f.err
}

// TestInContainer verifies that the container marker files decide whether
// Watchtower runs in a container, and that an unreadable marker counts as
// present.
func TestInContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// files are created before the check.
		files []string
		// statErr, when set, fails every Stat call.
		statErr error
		want    bool
	}{
		{name: "no marker", want: false},
		{name: "docker marker", files: []string{"/.dockerenv"}, want: true},
		{name: "podman marker", files: []string{"/run/.containerenv"}, want: true},
		{name: "marker cannot be checked", statErr: fs.ErrPermission, want: true},
		{name: "markers do not exist", statErr: fs.ErrNotExist, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fsys := afero.NewMemMapFs()
			for _, name := range tt.files {
				require.NoError(t, afero.WriteFile(fsys, name, nil, 0o600))
			}

			if tt.statErr != nil {
				fsys = statErrorFs{Fs: fsys, err: tt.statErr}
			}

			assert.Equal(t, tt.want, InContainer(fsys))
		})
	}
}

// TestInContainer_WrappedNotExist verifies that a wrapped not-exist error
// still counts as a missing marker.
func TestInContainer_WrappedNotExist(t *testing.T) {
	t.Parallel()

	fsys := statErrorFs{Fs: afero.NewMemMapFs(), err: errors.Join(errors.New("stat"), fs.ErrNotExist)}

	assert.False(t, InContainer(fsys))
}

// TestGetCurrentContainerID_FailureLogsAtDebug verifies that failing to find
// the container ID is not logged as an error. A Watchtower binary on the host
// has no container ID, and callers report a missing ID themselves. It sets
// environment variables, so it runs sequentially.
func TestGetCurrentContainerID_FailureLogsAtDebug(t *testing.T) {
	t.Setenv("HOSTNAME", "")

	var out bytes.Buffer

	log := zerolog.New(&out).Level(zerolog.TraceLevel)

	id, err := GetCurrentContainerID(&log, t.Context(), afero.NewMemMapFs(), nil)
	require.Error(t, err)
	assert.Empty(t, id)
	assert.NotContains(t, out.String(), `"level":"error"`)
	assert.NotContains(t, out.String(), `"level":"warn"`)

	// Find the summary record and check its level.
	var levels []string

	for line := range strings.Lines(out.String()) {
		var record map[string]any

		require.NoError(t, json.Unmarshal([]byte(line), &record))

		if record["message"] == "All container ID detection methods failed" {
			levels = append(levels, record["level"].(string))
		}
	}

	assert.Equal(t, []string{"debug"}, levels)
}

// TestParseContainerIDFromMountinfo_Layouts verifies that the container ID is
// read from the mount roots Docker and Podman use, and that paths which only
// resemble them are ignored.
func TestParseContainerIDFromMountinfo_Layouts(t *testing.T) {
	t.Parallel()

	const id = "867e908db86764ddc4898108f1ffc69c7cdaa903079cfe71d27d3f78278be03c"

	tests := []struct {
		name string
		root string
		want types.ContainerID
	}{
		{name: "Docker", root: "/var/lib/docker/containers/" + id + "/hostname", want: id},
		{name: "rootless Podman", root: "/containers/overlay-containers/" + id + "/userdata/hostname", want: id},
		{
			name: "rootful Podman",
			root: "/var/lib/containers/storage/overlay-containers/" + id + "/userdata/hostname",
			want: id,
		},
		{name: "a directory that only ends in containers", root: "/srv/notcontainers/" + id + "/hostname"},
		{name: "a prefixed overlay directory", root: "/srv/my-overlay-containers/" + id + "/hostname"},
		{name: "a longer hexadecimal directory name", root: "/containers/overlay-containers/" + id + "a/userdata/hostname"},
		{name: "an ID that ends the path", root: "/var/lib/docker/containers/" + id, want: id},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mountinfo := "1234 24 0:52 " + tt.root + " /etc/hostname rw,relatime - ext4 /dev/sda1 rw\n"

			got, err := parseContainerIDFromMountinfo(newTestLogger(t), mountinfo)
			if tt.want == "" {
				require.ErrorIs(t, err, errNoValidContainerID)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseContainerIDFromCgroupString_Layouts verifies the container ID is
// read from cgroup paths under both the cgroupfs and the systemd cgroup
// drivers, for Docker and Podman, and not from paths without a full ID.
func TestParseContainerIDFromCgroupString_Layouts(t *testing.T) {
	t.Parallel()

	const id = "3f4e2b1a9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f"

	tests := []struct {
		name   string
		cgroup string
		want   types.ContainerID
	}{
		{name: "cgroupfs driver", cgroup: "12:memory:/docker/" + id, want: id},
		{name: "systemd driver on cgroup v1", cgroup: "1:name=systemd:/system.slice/docker-" + id + ".scope", want: id},
		{name: "systemd driver on cgroup v2", cgroup: "0::/system.slice/docker-" + id + ".scope", want: id},
		{name: "Podman under systemd", cgroup: "0::/machine.slice/libpod-" + id + ".scope", want: id},
		{
			name:   "Podman under systemd with a nested container cgroup",
			cgroup: "0::/machine.slice/libpod-" + id + ".scope/container",
			want:   id,
		},
		{name: "namespaced cgroup v2", cgroup: "0::/"},
		{name: "scope without a full ID", cgroup: "0::/system.slice/docker-compose-app.scope"},
		{name: "short ID in a scope", cgroup: "0::/system.slice/docker-" + id[:12] + ".scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseContainerIDFromCgroupString(newTestLogger(t), tt.cgroup)
			if tt.want == "" {
				require.ErrorIs(t, err, errNoValidContainerID)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
