package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
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

// Static error for test.
var (
	ErrMockedFileRead          = errors.New("mocked file read error")
	ErrMockedMountinfoFileRead = errors.New("mocked mountinfo file read error")
)

var _ = ginkgo.Describe("GetContainerIDFromCgroupFile", func() {
	var originalReadFileFunc func(string) ([]byte, error)

	ginkgo.BeforeEach(func() {
		originalReadFileFunc = ReadCgroupFunc
	})

	ginkgo.AfterEach(func() {
		ReadCgroupFunc = originalReadFileFunc
	})

	ginkgo.DescribeTable("should return the correct container ID",
		func(setup func(), want types.ContainerID, wantErr error) {
			if setup != nil {
				setup()
			}

			got, err := GetContainerIDFromCgroupFile(testLog())
			if wantErr == nil {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(errors.Is(err, wantErr)).To(gomega.BeTrue())
			}

			gomega.Expect(got).To(gomega.Equal(want))
		},
		ginkgo.Entry("SuccessWithValidID", func() {
			ReadCgroupFunc = func(string) ([]byte, error) {
				return []byte(
					"11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				), nil
			}
		}, types.ContainerID(
			"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		), nil),
		ginkgo.Entry("FileNotReadable", func() {
			ReadCgroupFunc = func(string) ([]byte, error) {
				return nil, ErrMockedFileRead
			}
		}, types.ContainerID(""), errReadCgroupFile),
		ginkgo.Entry("NoValidID", func() {
			ReadCgroupFunc = func(string) ([]byte, error) {
				return []byte("11:perf_event:/user.slice\n10:cpu:/system.slice"), nil
			}
		}, types.ContainerID(""), errExtractContainerID),
		ginkgo.Entry("EmptyFileContent", func() {
			ReadCgroupFunc = func(string) ([]byte, error) {
				return []byte(""), nil
			}
		}, types.ContainerID(""), errExtractContainerID),
	)
})

var _ = ginkgo.Describe("ParseContainerIDFromCgroupString", func() {
	ginkgo.DescribeTable("should extract container ID from string",
		func(s string, want types.ContainerID, wantErr error) {
			got, err := ParseContainerIDFromCgroupString(testLog(), s)
			if wantErr == nil {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(errors.Is(err, wantErr)).To(gomega.BeTrue())
			}

			gomega.Expect(got).To(gomega.Equal(want))
		},
		ginkgo.Entry(
			"ValidDockerContainerIDSingleLine",
			"11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			),
			nil,
		),
		ginkgo.Entry(
			"ValidDockerContainerIDMultiLine",
			"12:memory:/user.slice\n11:perf_event:/docker/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800\n10:cpu:/system.slice",
			types.ContainerID(
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
			),
			nil,
		),
		ginkgo.Entry("NoDockerPatternMultiLine",
			"11:perf_event:/user.slice\n10:cpu:/system.slice",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry("EmptyString",
			"",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry("InvalidIDLength",
			"11:perf_event:/docker/12345678",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry(
			"NonHexID",
			"11:perf_event:/docker/gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry(
			"ValidIDWithExtraLines",
			"11:perf_event:/docker/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef\n10:cpu:/system.slice",
			types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			),
			nil,
		),
		ginkgo.Entry("NoDockerPatternSingleLine",
			"11:perf_event:/user.slice",
			types.ContainerID(""),
			errNoValidContainerID,
		),
	)
})

var _ = ginkgo.Describe("GetContainerIDFromMountinfo", func() {
	var originalReadMountinfoFunc func(string) ([]byte, error)

	ginkgo.BeforeEach(func() {
		originalReadMountinfoFunc = ReadMountinfoFunc
	})

	ginkgo.AfterEach(func() {
		ReadMountinfoFunc = originalReadMountinfoFunc
	})

	ginkgo.DescribeTable("should return the correct container ID",
		func(setup func(), want types.ContainerID, wantErr error) {
			if setup != nil {
				setup()
			}

			got, err := GetContainerIDFromMountinfo(testLog())
			if wantErr == nil {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(errors.Is(err, wantErr)).To(gomega.BeTrue())
			}

			gomega.Expect(got).To(gomega.Equal(want))
		},
		ginkgo.Entry("SuccessWithValidIDInRoot", func() {
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return []byte(
					"36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf /etc/resolv.conf rw,noatime - btrfs /dev/sda1 rw",
				), nil
			}
		}, types.ContainerID(
			"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		), nil),
		ginkgo.Entry("SuccessWithValidIDInMountPoint", func() {
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return []byte(
					"36 35 98:0 /var/lib/docker/containers/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800/hostname /etc/hostname rw,noatime - btrfs /dev/sda1 rw",
				), nil
			}
		}, types.ContainerID(
			"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
		), nil),
		ginkgo.Entry("MountinfoFileNotReadable", func() {
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return nil, ErrMockedMountinfoFileRead
			}
		}, types.ContainerID(""), errReadMountinfoFile),
		ginkgo.Entry("NoValidIDInMountinfo", func() {
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return []byte("36 35 98:0 / rw,relatime shared:1 - overlay "), nil
			}
		}, types.ContainerID(""), errExtractContainerIDFromMountinfo),
		ginkgo.Entry("EmptyMountinfoFileContent", func() {
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return []byte(""), nil
			}
		}, types.ContainerID(""), errExtractContainerIDFromMountinfo),
	)
})

var _ = ginkgo.Describe("ParseContainerIDFromMountinfo", func() {
	ginkgo.DescribeTable("should extract container ID from mountinfo string",
		func(s string, want types.ContainerID, wantErr error) {
			got, err := ParseContainerIDFromMountinfo(testLog(), s)
			if wantErr == nil {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			} else {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(errors.Is(err, wantErr)).To(gomega.BeTrue())
			}

			gomega.Expect(got).To(gomega.Equal(want))
		},
		ginkgo.Entry(
			"ValidDockerContainerIDInRoot",
			"36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf /etc/resolv.conf rw,noatime - btrfs /dev/sda1 rw",
			types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			),
			nil,
		),
		ginkgo.Entry(
			"ValidDockerContainerIDInMountPoint",
			"36 35 98:0 /var/lib/docker/containers/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800/hostname /etc/hostname rw,noatime - btrfs /dev/sda1 rw",
			types.ContainerID(
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567800",
			),
			nil,
		),
		ginkgo.Entry(
			"NoValidMountinfoLine",
			"36 35 98:0 / rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry("EmptyString",
			"",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry("InvalidMountinfoLine",
			"invalid line format",
			types.ContainerID(""),
			errNoValidContainerID,
		),
		ginkgo.Entry(
			"ValidIDWithExtraLines",
			"36 35 98:0 /tmp rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...\n36 35 98:0 /var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/hosts /etc/hosts rw,noatime - btrfs /dev/sda1 rw\n36 35 98:0 /var rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...",
			types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			),
			nil,
		),
	)
})

var _ = ginkgo.Describe("ExtractContainerIDFromPath", func() {
	ginkgo.DescribeTable("should extract container ID from path",
		func(path string, want types.ContainerID) {
			got := ExtractContainerIDFromPath(path)
			gomega.Expect(got).To(gomega.Equal(want))
		},
		ginkgo.Entry(
			"ValidDockerPath",
			"/var/lib/docker/containers/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/resolv.conf",
			types.ContainerID("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		),
		ginkgo.Entry("NoDockerPath",
			"/sys/fs/cgroup",
			types.ContainerID(""),
		),
		ginkgo.Entry("InvalidLength",
			"/docker/12345678",
			types.ContainerID(""),
		),
		ginkgo.Entry("NonHexID",
			"/docker/gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			types.ContainerID(""),
		),
	)
})

var _ = ginkgo.Describe("GetContainerIDFromHostname", func() {
	var mockClient *mockContainer.MockClient

	ginkgo.BeforeEach(func() {
		mockClient = mockContainer.NewMockClient(ginkgo.GinkgoT())
	})

	ginkgo.AfterEach(func() {
		// Reset HOSTNAME env var
		os.Unsetenv("HOSTNAME")
	})

	ginkgo.DescribeTable("should return the correct container ID or error",
		func(setup func(), want types.ContainerID, wantErr bool, errorSubstring string) {
			if setup != nil {
				setup()
			}

			got, err := GetContainerIDFromHostname(testLog(), context.Background(), mockClient)
			if wantErr {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(got).To(gomega.BeEmpty())

				if errorSubstring != "" {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring(errorSubstring))
				}
			} else {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(got).To(gomega.Equal(want))
			}
		},
		ginkgo.Entry("when a container with matching hostname is found", func() {
			expectedID := types.ContainerID("hostname-container-id")
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

			// Create a mock container with matching hostname
			mockContainer := MockContainer(
				WithHostname(hostname),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					c.ID = string(expectedID)
				},
			)

			mockClient.EXPECT().ListContainers(context.Background()).Return([]types.Container{mockContainer}, nil)
		}, types.ContainerID("hostname-container-id"), false, ""),
		ginkgo.Entry("when no container with matching hostname is found", func() {
			hostname := testHostname
			differentHostname := "different-hostname"

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

			// Create a mock container with different hostname
			mockContainer := MockContainer(
				WithHostname(differentHostname),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					c.ID = "different-container-id"
				},
			)

			mockClient.EXPECT().ListContainers(context.Background()).Return([]types.Container{mockContainer}, nil)
		}, types.ContainerID(""), true, "no container found with matching hostname"),
		ginkgo.Entry("when HOSTNAME environment variable is not set", func() {
			// HOSTNAME is not set (already unset in AfterEach)
		}, types.ContainerID(""), true, "HOSTNAME environment variable is not set"),
		ginkgo.Entry("when container listing fails", func() {
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

			expectedError := errors.New("failed to list containers")

			mockClient.EXPECT().ListContainers(context.Background()).Return(nil, expectedError)
		}, types.ContainerID(""), true, "failed to list all containers"),
		ginkgo.Entry("when container info is nil", func() {
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

			// Create containers: one with nil info, one with matching hostname
			mockContainerNilInfo := mockTypes.NewMockContainer(ginkgo.GinkgoT())
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
		}, types.ContainerID("matching-container-id"), false, ""),
		ginkgo.Entry("when container config is nil", func() {
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

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
		}, types.ContainerID("matching-container-id"), false, ""),
		ginkgo.Entry("when multiple containers exist but only one matches hostname", func() {
			hostname := testHostname
			expectedID := types.ContainerID("matching-container-id")

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

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
		}, types.ContainerID("matching-container-id"), false, ""),
		ginkgo.Entry("when multiple containers share hostname but only one is Watchtower", func() {
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

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
		}, types.ContainerID("watchtower-container-id"), false, ""),

		ginkgo.Entry("when multiple Watchtower containers share hostname, prefer the non-old one", func() {
			hostname := testHostname

			// Set HOSTNAME environment variable
			os.Setenv("HOSTNAME", hostname)

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
		}, types.ContainerID("main-watchtower-id"), false, ""),
	)
})

var _ = ginkgo.Describe("GetCurrentContainerID", func() {
	var mockClient *mockContainer.MockClient

	ginkgo.BeforeEach(func() {
		mockClient = mockContainer.NewMockClient(ginkgo.GinkgoT())
	})

	ginkgo.AfterEach(func() {
		// Reset ReadCgroupFunc to original
		ReadCgroupFunc = os.ReadFile
		// Reset ReadMountinfoFunc to original
		ReadMountinfoFunc = os.ReadFile
		// Reset HOSTNAME env var
		os.Unsetenv("HOSTNAME")
	})

	ginkgo.DescribeTable(
		"should return the correct container ID or error",
		func(setup func(), expectedID types.ContainerID, expectError bool, errorSubstring string) {
			if setup != nil {
				setup()
			}

			id, err := GetCurrentContainerID(testLog(), context.Background(), mockClient)
			if expectError {
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(id).To(gomega.BeEmpty())

				if errorSubstring != "" {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring(errorSubstring))
				}
			} else {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(id).To(gomega.Equal(expectedID))
			}
		},
		ginkgo.Entry("when cgroup v2 detection succeeds", func() {
			expectedID := types.ContainerID(
				"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			)

			// Mock cgroup v2 detection to succeed
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return []byte(
					"36 35 98:0 /var/lib/docker/containers/" + string(
						expectedID,
					) + "/resolv.conf /etc/resolv.conf rw,relatime shared:1 - overlay rw,lowerdir=/var/lib/docker/overlay2/l/...,upperdir=/var/lib/docker/overlay2/...,workdir=/var/lib/docker/overlay2/...,\n",
				), nil
			}
		}, types.ContainerID("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), false, ""),
		ginkgo.Entry("when cgroup v2 detection fails but cgroup v1 detection succeeds", func() {
			expectedID := types.ContainerID(
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			)

			// Mock cgroup v2 detection to fail
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return nil, errors.New("mountinfo read failed")
			}

			// Mock cgroup v1 detection to succeed
			ReadCgroupFunc = func(string) ([]byte, error) {
				return []byte("11:perf_event:/docker/" + string(expectedID) + "\n"), nil
			}
		}, types.ContainerID(
			"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		), false, ""),
		ginkgo.Entry(
			"when both cgroup v2 and cgroup v1 detection fail but hostname detection succeeds",
			func() {
				expectedID := types.ContainerID("hostname-container-id")
				hostname := testHostname

				// Set HOSTNAME environment variable
				os.Setenv("HOSTNAME", hostname)

				// Mock cgroup v2 detection to fail
				ReadMountinfoFunc = func(string) ([]byte, error) {
					return nil, errors.New("mountinfo read failed")
				}

				// Mock cgroup v1 detection to fail
				ReadCgroupFunc = func(string) ([]byte, error) {
					return nil, errors.New("cgroup file not found")
				}

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
			},
			types.ContainerID("hostname-container-id"),
			false,
			"",
		),
		ginkgo.Entry("when all detection methods fail", func() {
			// Mock cgroup v2 detection to fail
			ReadMountinfoFunc = func(string) ([]byte, error) {
				return nil, errors.New("mountinfo read failed")
			}

			// Mock cgroup v1 detection to fail
			ReadCgroupFunc = func(string) ([]byte, error) {
				return nil, errors.New("cgroup file not found")
			}

			// HOSTNAME not set, so hostname detection will fail without calling client
		}, types.ContainerID(""), true, "failed to detect current container ID"),
	)
})

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
// has no container ID, and callers report a missing ID themselves. It replaces
// the file readers and sets environment variables, so it runs sequentially.
func TestGetCurrentContainerID_FailureLogsAtDebug(t *testing.T) {
	t.Setenv("HOSTNAME", "")

	savedMountinfo, savedCgroup := ReadMountinfoFunc, ReadCgroupFunc

	t.Cleanup(func() { ReadMountinfoFunc, ReadCgroupFunc = savedMountinfo, savedCgroup })

	notFound := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	ReadMountinfoFunc, ReadCgroupFunc = notFound, notFound

	var out bytes.Buffer

	log := zerolog.New(&out).Level(zerolog.TraceLevel)

	id, err := GetCurrentContainerID(&log, t.Context(), nil)
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

			got, err := ParseContainerIDFromMountinfo(testLog(), mountinfo)
			if tt.want == "" {
				require.ErrorIs(t, err, errNoValidContainerID)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
