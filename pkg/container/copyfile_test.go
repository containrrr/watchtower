package container

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerMount "github.com/moby/moby/api/types/mount"
	dockerSystem "github.com/moby/moby/api/types/system"
	dockerClient "github.com/moby/moby/client"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestParseCopyFileLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		give    string
		want    []string
		wantErr error
	}{
		{
			name: "empty",
			give: "",
			want: nil,
		},
		{
			name: "whitespace",
			give: "   ",
			want: nil,
		},
		{
			name: "single path",
			give: "/app/config.yaml",
			want: []string{"/app/config.yaml"},
		},
		{
			name: "multiple paths",
			give: "/app/config.yaml,/app/extra.conf",
			want: []string{"/app/config.yaml", "/app/extra.conf"},
		},
		{
			name: "trims spaces and skips empty",
			give: " /a.conf , , /b.conf ",
			want: []string{"/a.conf", "/b.conf"},
		},
		{
			name: "deduplicates in label order",
			give: "/a.conf,/b.conf,/a.conf",
			want: []string{"/a.conf", "/b.conf"},
		},
		{
			name:    "rejects relative path",
			give:    "app/config.yaml",
			wantErr: errCopyFileInvalidPath,
		},
		{
			name:    "rejects parent directory",
			give:    "/app/../config.yaml",
			wantErr: errCopyFileInvalidPath,
		},
		{
			name:    "rejects root",
			give:    "/",
			wantErr: errCopyFileInvalidPath,
		},
		{
			name:    "rejects trailing slash",
			give:    "/app/",
			wantErr: errCopyFileInvalidPath,
		},
		{
			name:    "rejects backslash",
			give:    "/app\\config.yaml",
			wantErr: errCopyFileInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCopyFileLabel(tt.give)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCopyFilePathsFromContainer(t *testing.T) {
	t.Parallel()

	t.Run("missing label", func(t *testing.T) {
		t.Parallel()

		got, err := copyFilePaths(MockContainer())
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("reads label", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		got, err := copyFilePaths(c)
		require.NoError(t, err)
		assert.Equal(t, []string{"/app/config.yaml"}, got)
	})

	t.Run("invalid label", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "../secret",
		}))
		got, err := copyFilePaths(c)
		require.ErrorIs(t, err, errCopyFileInvalidPath)
		assert.Nil(t, got)
	})

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		got, err := copyFilePaths(nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("empty label value", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "",
		}))
		got, err := copyFilePaths(c)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestCopyFileSkipForPath(t *testing.T) {
	t.Parallel()

	nilHostConfig := func() *Container {
		c := MockContainer()
		c.containerInfo.HostConfig = nil

		return c
	}()

	nilHostConfigWithMount := func() *Container {
		c := MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
			{Destination: "/etc/searxng", RW: false},
		}))
		c.containerInfo.HostConfig = nil

		return c
	}()

	tests := []struct {
		name string
		c    *Container
		path string
		want copyFileSkipReason
	}{
		{
			name: "no mounts",
			c:    MockContainer(),
			path: "/app/config.yaml",
			want: copyFileSkipNone,
		},
		{
			name: "exact inspect mount",
			c: MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/app/config.yaml", RW: true},
			})),
			path: "/app/config.yaml",
			want: copyFileSkipMountedAtPath,
		},
		{
			name: "writable inspect mount above path",
			c: MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/etc/searxng", RW: true},
			})),
			path: "/etc/searxng/settings.yml",
			want: copyFileSkipNone,
		},
		{
			name: "read-only inspect mount above path",
			c: MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/etc/searxng", RW: false},
			})),
			path: "/etc/searxng/settings.yml",
			want: copyFileSkipReadOnlyMount,
		},
		{
			name: "host config mount target",
			c: MockContainer(WithMounts([]dockerMount.Mount{
				{Target: "/run/secrets"},
			})),
			path: "/run/secrets/token",
			want: copyFileSkipNone,
		},
		{
			name: "read-only host config mount above path",
			c: MockContainer(WithMounts([]dockerMount.Mount{
				{Target: "/etc/conf", ReadOnly: true},
			})),
			path: "/etc/conf/app.conf",
			want: copyFileSkipReadOnlyMount,
		},
		{
			name: "bind destination at path",
			c:    MockContainer(WithBinds([]string{"/host/config.yaml:/app/config.yaml:ro"})),
			path: "/app/config.yaml",
			want: copyFileSkipMountedAtPath,
		},
		{
			name: "writable bind above path",
			c:    MockContainer(WithBinds([]string{"/host/data:/data"})),
			path: "/data/app.db",
			want: copyFileSkipNone,
		},
		{
			name: "read-only bind above path",
			c:    MockContainer(WithBinds([]string{"/host/data:/data:ro,z"})),
			path: "/data/app.db",
			want: copyFileSkipReadOnlyMount,
		},
		{
			name: "deeper writable mount shadows read-only mount",
			c: MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/data", RW: false},
				{Destination: "/data/inner", RW: true},
			})),
			path: "/data/inner/app.db",
			want: copyFileSkipNone,
		},
		{
			name: "deeper mount wins regardless of order",
			c: MockContainer(WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/data/inner", RW: true},
				{Destination: "/data", RW: false},
			})),
			path: "/data/inner/app.db",
			want: copyFileSkipNone,
		},
		{
			name: "unrelated bind",
			c:    MockContainer(WithBinds([]string{"/host/data:/data"})),
			path: "/app/config.yaml",
			want: copyFileSkipNone,
		},
		{
			name: "malformed bind",
			c:    MockContainer(WithBinds([]string{"nocolon"})),
			path: "/app/config.yaml",
			want: copyFileSkipNone,
		},
		{
			name: "read-only rootfs without mount",
			c:    MockContainer(WithReadonlyRootfs(true)),
			path: "/app/config.yaml",
			want: copyFileSkipReadOnlyRootfs,
		},
		{
			name: "read-only rootfs with volume above path",
			c: MockContainer(
				WithReadonlyRootfs(true),
				WithInspectMounts([]dockerContainer.MountPoint{
					{Destination: "/etc/searxng", RW: true},
				}),
			),
			path: "/etc/searxng/settings.yml",
			want: copyFileSkipReadOnlyRootfs,
		},
		{
			name: "read-only rootfs with mount at path",
			c: MockContainer(
				WithReadonlyRootfs(true),
				WithInspectMounts([]dockerContainer.MountPoint{
					{Destination: "/app/config.yaml", RW: true},
				}),
			),
			path: "/app/config.yaml",
			want: copyFileSkipReadOnlyRootfs,
		},
		{
			name: "nil inspect",
			c:    &Container{},
			path: "/app/config.yaml",
			want: copyFileSkipNone,
		},
		{
			name: "nil host config",
			c:    nilHostConfig,
			path: "/app/config.yaml",
			want: copyFileSkipNone,
		},
		{
			name: "nil host config with read-only inspect mount",
			c:    nilHostConfigWithMount,
			path: "/etc/searxng/settings.yml",
			want: copyFileSkipReadOnlyMount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, copyFileSkipForPath(tt.c, tt.path))
		})
	}
}

func TestSnapshotCopyFiles(t *testing.T) {
	t.Parallel()

	t.Run("no label returns empty snapshot", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, MockContainer())
		require.NoError(t, err)
		assert.True(t, snap.empty())
	})

	t.Run("copies labeled file", func(t *testing.T) {
		t.Parallel()

		tarBytes := mustFileTar(t, "config.yaml", []byte("setting=1"))
		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(tarBytes)),
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Size: int64(len("setting=1")),
					Mode: 0o444,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		require.Len(t, snap.files, 1)
		assert.Equal(t, "/app/config.yaml", snap.files[0].target)

		name, body, mode := tarRegularFile(t, snap.files[0].archive)
		assert.Equal(t, "/app/config.yaml", name)
		assert.Equal(t, []byte("setting=1"), body)
		assert.Equal(t, int64(0o444), mode)
	})

	t.Run("skips missing path", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/etc/missing.conf")).
			Return(dockerClient.CopyFromContainerResult{}, cerrdefs.ErrNotFound)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/etc/missing.conf",
		}))

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		assert.True(t, snap.empty())
	})

	t.Run("skips path a mount provides", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		c := MockContainer(
			WithLabels(map[string]string{
				copyFileLabel: "/app/config.yaml",
			}),
			WithBinds([]string{"/host/config.yaml:/app/config.yaml:ro"}),
		)

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		assert.True(t, snap.empty())
	})

	t.Run("skips path under read-only mount", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		c := MockContainer(
			WithLabels(map[string]string{
				copyFileLabel: "/etc/searxng/settings.yml",
			}),
			WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/etc/searxng", RW: false},
			}),
		)

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		assert.True(t, snap.empty())
	})

	t.Run("skips path when root filesystem is read-only", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		c := MockContainer(
			WithLabels(map[string]string{
				copyFileLabel: "/etc/searxng/settings.yml",
			}),
			WithReadonlyRootfs(true),
			WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/etc/searxng", RW: true},
			}),
		)

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		assert.True(t, snap.empty())
	})

	t.Run("copies path under a directory mount", func(t *testing.T) {
		t.Parallel()

		tarBytes := mustFileTar(t, "settings.yml", []byte("use_default_settings: false"))
		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/etc/searxng/settings.yml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(tarBytes)),
				Stat: dockerContainer.PathStat{
					Name: "settings.yml",
					Size: int64(len(tarBytes)),
					Mode: 0o644,
				},
			}, nil)

		c := MockContainer(
			WithLabels(map[string]string{
				copyFileLabel: "/etc/searxng/settings.yml",
			}),
			WithInspectMounts([]dockerContainer.MountPoint{
				{Destination: "/etc/searxng", RW: true},
			}),
		)

		snap, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.NoError(t, err)
		require.Len(t, snap.files, 1)

		name, body, _ := tarRegularFile(t, snap.files[0].archive)
		assert.Equal(t, "/etc/searxng/settings.yml", name)
		assert.Equal(t, []byte("use_default_settings: false"), body)
	})

	t.Run("rejects directory", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/data")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(nil)),
				Stat: dockerContainer.PathStat{
					Name: "data",
					Mode: os.ModeDir | 0o755,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/data",
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileDirectory)
	})

	t.Run("rejects oversized file", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/etc/huge.conf")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(nil)),
				Stat: dockerContainer.PathStat{
					Name: "huge.conf",
					Size: maxCopyFileBytes + 1,
					Mode: 0o644,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/etc/huge.conf",
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileTooLarge)
	})

	t.Run("rejects symlink", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(nil)),
				Stat: dockerContainer.PathStat{
					Name:       "config.yaml",
					LinkTarget: "/app/other.yaml",
					Mode:       os.ModeSymlink | 0o777,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileSymlink)
	})

	t.Run("rejects nil archive stream", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Size: 1,
					Mode: 0o644,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileSnapshotFailed)
	})

	t.Run("invalid label fails before copy", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "relative.conf",
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileInvalidPath)
	})

	t.Run("rejects when labeled files exceed store cap", func(t *testing.T) {
		t.Parallel()

		const fileCount = 17

		body := bytes.Repeat([]byte("x"), maxCopyFileBytes)
		paths := make([]string, fileCount)
		api := mockContainer.NewMockAPIClient(t)

		for i := range fileCount {
			name := strconv.Itoa(i)
			paths[i] = "/app/" + name
			tarBytes := mustFileTar(t, name, body)

			call := api.EXPECT().
				CopyFromContainer(mock.Anything, "container_id", copyFromPath(paths[i])).
				Return(dockerClient.CopyFromContainerResult{
					Content: io.NopCloser(bytes.NewReader(tarBytes)),
					Stat: dockerContainer.PathStat{
						Name: name,
						Size: int64(len(body)),
						Mode: 0o444,
					},
				}, nil)
			if i == fileCount-1 {
				call.Maybe()
			}
		}

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: strings.Join(paths, ","),
		}))

		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileStoreFull)
	})
}

func TestInjectCopyFiles(t *testing.T) {
	t.Parallel()

	t.Run("empty snapshot is a no-op", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		err := injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{},
		)
		require.NoError(t, err)
	})

	t.Run("copies archive at the container root", func(t *testing.T) {
		t.Parallel()

		tarBytes := mustFileTar(t, "config.yaml", []byte("setting=1"))
		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api, "name=seccomp,profile=builtin", "name=rootless", "name=cgroupns")
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Run(func(_ context.Context, _ string, options dockerClient.CopyToContainerOptions) {
				payload, readErr := io.ReadAll(options.Content)
				require.NoError(t, readErr)
				assert.Equal(t, injectRootPath, options.DestinationPath)
				assert.Equal(t, tarBytes, payload)
				assert.False(t, options.CopyUIDGID)
			}).
			Return(dockerClient.CopyToContainerResult{}, nil)

		err := injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{files: []copyFileEntry{{
				target:  "/app/config.yaml",
				archive: tarBytes,
			}}},
		)
		require.NoError(t, err)
	})

	t.Run("member name carries a missing parent directory", func(t *testing.T) {
		t.Parallel()

		archive, err := sanitizeCopyFileArchive(
			mustFileTar(t, "config.yml", []byte("setting=1")),
			"/config/config.yml",
		)
		require.NoError(t, err)

		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api)
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Run(func(_ context.Context, _ string, options dockerClient.CopyToContainerOptions) {
				payload, readErr := io.ReadAll(options.Content)
				require.NoError(t, readErr)
				assert.Equal(t, injectRootPath, options.DestinationPath)

				name, body, _ := tarRegularFile(t, payload)
				assert.Equal(t, "/config/config.yml", name)
				assert.Equal(t, []byte("setting=1"), body)
			}).
			Return(dockerClient.CopyToContainerResult{}, nil)

		err = injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{files: []copyFileEntry{{
				target:  "/config/config.yml",
				archive: archive,
			}}},
		)
		require.NoError(t, err)
	})

	t.Run("copy failure is returned", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api)
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Return(dockerClient.CopyToContainerResult{}, errors.New("denied"))

		err := injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{files: []copyFileEntry{{
				target:  "/app/config.yaml",
				archive: []byte("tar"),
			}}},
		)
		require.ErrorIs(t, err, errCopyFileInjectFailed)
	})

	// A userns-remap daemon rejects captured non-root owners on upload, so every file in
	// the snapshot takes Config.User ownership, and the daemon is queried only once.
	t.Run("userns daemon applies container user ownership", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api, "name=seccomp,profile=builtin", "name=userns", "name=cgroupns")
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Run(func(_ context.Context, _ string, options dockerClient.CopyToContainerOptions) {
				assert.True(t, options.CopyUIDGID)
			}).
			Return(dockerClient.CopyToContainerResult{}, nil).
			Twice()

		err := injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{files: []copyFileEntry{
				{target: "/app/config.yaml", archive: mustFileTar(t, "config.yaml", []byte("a"))},
				{target: "/run/secrets/token", archive: mustFileTar(t, "token", []byte("b"))},
			}},
		)
		require.NoError(t, err)
	})

	t.Run("daemon info failure keeps captured ownership", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			Info(mock.Anything, dockerClient.InfoOptions{}).
			Return(dockerClient.SystemInfoResult{}, errors.New("unavailable")).
			Once()
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Run(func(_ context.Context, _ string, options dockerClient.CopyToContainerOptions) {
				assert.False(t, options.CopyUIDGID)
			}).
			Return(dockerClient.CopyToContainerResult{}, nil)

		err := injectCopyFiles(
			newTestLogger(t),
			t.Context(),
			api,
			"new-id",
			MockContainer(),
			copyFileSnapshot{files: []copyFileEntry{{
				target:  "/app/config.yaml",
				archive: mustFileTar(t, "config.yaml", []byte("a")),
			}}},
		)
		require.NoError(t, err)
	})
}

// TestDaemonRemapsUserNamespaces covers parsing of Info.SecurityOptions entries, which
// are comma-separated key=value lists such as "name=seccomp,profile=builtin".
func TestDaemonRemapsUserNamespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options []string
		want    bool
	}{
		{name: "no options", options: nil, want: false},
		{name: "default docker", options: []string{"name=seccomp,profile=builtin", "name=cgroupns"}, want: false},
		{name: "rootless", options: []string{"name=seccomp,profile=builtin", "name=rootless"}, want: false},
		{name: "userns remap", options: []string{"name=seccomp,profile=builtin", "name=userns"}, want: true},
		{name: "userns among fields", options: []string{"name=userns,profile=custom"}, want: true},
		{name: "similar option name", options: []string{"name=usernsx", "profile=userns"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, daemonRemapsUserNamespaces(tt.options))
		})
	}
}

func TestDiscardStoredCopyFiles(t *testing.T) {
	t.Parallel()

	sourceID := types.ContainerID("source-id")
	otherID := types.ContainerID("other-id")
	cli := &client{
		copyFiles: map[types.ContainerID]copyFileSnapshot{
			sourceID: {files: []copyFileEntry{{target: "/app/config.yaml"}}},
			otherID:  {files: []copyFileEntry{{target: "/app/extra.conf"}}},
		},
	}

	cli.DiscardCopyFiles(sourceID)
	assert.NotContains(t, cli.copyFiles, sourceID)
	require.Contains(t, cli.copyFiles, otherID)
	assert.Equal(t, "/app/extra.conf", cli.copyFiles[otherID].files[0].target)

	cli.DiscardCopyFiles(sourceID)
}

func TestSanitizeCopyFileArchive(t *testing.T) {
	t.Parallel()

	t.Run("rewrites to the full destination path and strips setuid", func(t *testing.T) {
		t.Parallel()

		raw := mustFileTarWithMode(t, "config.yaml", []byte("setting=1"), 0o4755)
		got, err := sanitizeCopyFileArchive(raw, "/app/config.yaml")
		require.NoError(t, err)

		name, body, mode := tarRegularFile(t, got)
		assert.Equal(t, "/app/config.yaml", name)
		assert.Equal(t, []byte("setting=1"), body)
		assert.Equal(t, int64(0o755), mode)
	})

	t.Run("rejects invalid destination path", func(t *testing.T) {
		t.Parallel()

		raw := mustFileTar(t, "config.yaml", []byte("setting=1"))
		_, err := sanitizeCopyFileArchive(raw, "../config.yaml")
		require.ErrorIs(t, err, errCopyFileInvalidPath)
	})

	t.Run("rejects parent traversal", func(t *testing.T) {
		t.Parallel()

		raw := mustFileTar(t, "../config.yaml", []byte("setting=1"))
		_, err := sanitizeCopyFileArchive(raw, "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileUnsafeArchive)
	})

	t.Run("rejects extra members", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		writer := tar.NewWriter(&buf)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: "config.yaml",
			Size: 1,
			Mode: 0o644,
		}))
		_, err := writer.Write([]byte("a"))
		require.NoError(t, err)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: "extra.yaml",
			Size: 1,
			Mode: 0o644,
		}))
		_, err = writer.Write([]byte("b"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		_, err = sanitizeCopyFileArchive(buf.Bytes(), "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileUnsafeArchive)
	})

	t.Run("rejects symlink member", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		writer := tar.NewWriter(&buf)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     "config.yaml",
			Linkname: "/tmp/other",
			Mode:     0o777,
		}))
		require.NoError(t, writer.Close())

		_, err := sanitizeCopyFileArchive(buf.Bytes(), "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileSymlink)
	})

	t.Run("rejects empty archive", func(t *testing.T) {
		t.Parallel()

		_, err := sanitizeCopyFileArchive(nil, "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileUnsafeArchive)
	})

	t.Run("rejects truncated archive", func(t *testing.T) {
		t.Parallel()

		raw := mustFileTar(t, "config.yaml", []byte("setting=1"))
		_, err := sanitizeCopyFileArchive(raw[:20], "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileUnsafeArchive)
	})

	t.Run("rejects oversized member", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		body := bytes.Repeat([]byte("x"), int(maxCopyFileBytes)+1)
		writer := tar.NewWriter(&buf)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "config.yaml",
			Size:     int64(len(body)),
			Mode:     0o644,
		}))
		_, err := writer.Write(body)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		_, err = sanitizeCopyFileArchive(buf.Bytes(), "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileTooLarge)
	})

	t.Run("rejects directory member", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		writer := tar.NewWriter(&buf)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     "config.yaml",
			Mode:     0o755,
		}))
		require.NoError(t, writer.Close())

		_, err := sanitizeCopyFileArchive(buf.Bytes(), "/app/config.yaml")
		require.ErrorIs(t, err, errCopyFileUnsafeArchive)
	})

	t.Run("skips pax headers", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer

		writer := tar.NewWriter(&buf)
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Typeflag: tar.TypeXGlobalHeader,
			Name:     "pax_global_header",
			PAXRecords: map[string]string{
				"comment": "meta",
			},
		}))
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "config.yaml",
			Size:     1,
			Mode:     0o644,
		}))
		_, err := writer.Write([]byte("a"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		got, err := sanitizeCopyFileArchive(buf.Bytes(), "/app/config.yaml")
		require.NoError(t, err)

		name, body, _ := tarRegularFile(t, got)
		assert.Equal(t, "/app/config.yaml", name)
		assert.Equal(t, []byte("a"), body)
	})
}

func TestStoreCopyFileSnapshotCap(t *testing.T) {
	t.Parallel()

	t.Run("rejects when store is full", func(t *testing.T) {
		t.Parallel()

		cli := &client{
			copyFiles:     make(map[types.ContainerID]copyFileSnapshot),
			copyFileBytes: maxCopyFileStoreBytes,
		}
		err := cli.storeCopyFileSnapshot("source-id", copyFileSnapshot{
			files: []copyFileEntry{{
				target:  "/app/config.yaml",
				archive: []byte("tar"),
			}},
		})
		require.ErrorIs(t, err, errCopyFileStoreFull)
		assert.Empty(t, cli.copyFiles)
		assert.Equal(t, int64(maxCopyFileStoreBytes), cli.copyFileBytes)
	})

	t.Run("stores and overwrites same id", func(t *testing.T) {
		t.Parallel()

		cli := &client{}
		first := copyFileSnapshot{files: []copyFileEntry{{
			target:  "/app/config.yaml",
			archive: []byte("first-archive"),
		}}}
		second := copyFileSnapshot{files: []copyFileEntry{{
			target:  "/app/config.yaml",
			archive: []byte("next"),
		}}}

		require.NoError(t, cli.storeCopyFileSnapshot("source-id", first))
		assert.Equal(t, first.size(), cli.copyFileBytes)
		require.NoError(t, cli.storeCopyFileSnapshot("source-id", second))
		assert.Equal(t, second.size(), cli.copyFileBytes)
		assert.Equal(t, []byte("next"), cli.copyFiles["source-id"].files[0].archive)
	})

	t.Run("restores existing snapshot when overwrite exceeds cap", func(t *testing.T) {
		t.Parallel()

		existing := copyFileSnapshot{files: []copyFileEntry{{
			target:  "/app/config.yaml",
			archive: []byte("keep"),
		}}}
		cli := &client{
			copyFiles: map[types.ContainerID]copyFileSnapshot{
				"source-id": existing,
			},
			copyFileBytes: existing.size(),
		}
		tooLarge := copyFileSnapshot{files: []copyFileEntry{{
			target:  "/app/config.yaml",
			archive: bytes.Repeat([]byte("x"), int(maxCopyFileStoreBytes)+1),
		}}}

		err := cli.storeCopyFileSnapshot("source-id", tooLarge)
		require.ErrorIs(t, err, errCopyFileStoreFull)
		assert.Equal(t, existing.size(), cli.copyFileBytes)
		assert.Equal(t, []byte("keep"), cli.copyFiles["source-id"].files[0].archive)
	})
}

func TestPathIsUnderMount(t *testing.T) {
	t.Parallel()

	assert.False(t, pathIsUnderMount("/app/config.yaml", ""))
	assert.False(t, pathIsUnderMount("/app/config.yaml", "."))
	assert.True(t, pathIsUnderMount("/app/config.yaml", "/app/config.yaml"))
}

func TestPathHasMount(t *testing.T) {
	t.Parallel()

	assert.False(t, pathHasMount("/app/config.yaml", ""))
	assert.False(t, pathHasMount("/app/config.yaml", "."))
	assert.False(t, pathHasMount("/app/config.yaml", "/app"))
	assert.True(t, pathHasMount("/app/config.yaml", "/app/config.yaml"))
	assert.True(t, pathHasMount("/app/./config.yaml", "/app/config.yaml"))
}

func TestBindIsReadOnly(t *testing.T) {
	t.Parallel()

	assert.False(t, bindIsReadOnly("nocolon"))
	assert.False(t, bindIsReadOnly("/host/config.yaml:/app/config.yaml"))
	assert.False(t, bindIsReadOnly("/host/data:/data:rw,z"))
	assert.True(t, bindIsReadOnly("/host/data:/data:ro"))
	assert.True(t, bindIsReadOnly("/host/data:/data:z,ro"))
}

func TestSnapshotCopyFileErrors(t *testing.T) {
	t.Parallel()

	t.Run("copy from failure", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{}, errors.New("denied"))

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileSnapshotFailed)
	})

	t.Run("symlink mode without link target", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(nil)),
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Mode: os.ModeSymlink | 0o777,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileSymlink)
	})

	t.Run("archive stream read error", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(errReader{err: errors.New("read failed")}),
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Size: 1,
					Mode: 0o644,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileSnapshotFailed)
	})

	t.Run("archive stream exceeds tar cap", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), int(maxCopyFileTarBytes)+1))),
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Size: 1,
					Mode: 0o644,
				},
			}, nil)

		c := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		_, err := snapshotCopyFiles(newTestLogger(t), t.Context(), api, c)
		require.ErrorIs(t, err, errCopyFileTooLarge)
	})
}

func TestClientCopyFileStore(t *testing.T) {
	t.Parallel()

	t.Run("snapshot empty label is a no-op", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		cli := &client{log: newTestLogger(t), api: api}
		require.NoError(t, cli.SnapshotCopyFiles(t.Context(), MockContainer()))
		assert.Empty(t, cli.copyFiles)
	})

	t.Run("snapshot stores sanitized file", func(t *testing.T) {
		t.Parallel()

		tarBytes := mustFileTar(t, "config.yaml", []byte("setting=1"))
		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().
			CopyFromContainer(mock.Anything, "container_id", copyFromPath("/app/config.yaml")).
			Return(dockerClient.CopyFromContainerResult{
				Content: io.NopCloser(bytes.NewReader(tarBytes)),
				Stat: dockerContainer.PathStat{
					Name: "config.yaml",
					Size: int64(len("setting=1")),
					Mode: 0o444,
				},
			}, nil)

		cli := &client{log: newTestLogger(t), api: api}
		source := MockContainer(WithLabels(map[string]string{
			copyFileLabel: "/app/config.yaml",
		}))
		require.NoError(t, cli.SnapshotCopyFiles(t.Context(), source))
		require.Contains(t, cli.copyFiles, source.ID())
		assert.Positive(t, cli.copyFileBytes)
	})

	t.Run("snapshot invalid label fails", func(t *testing.T) {
		t.Parallel()

		cli := &client{log: newTestLogger(t), api: mockContainer.NewMockAPIClient(t)}
		err := cli.SnapshotCopyFiles(t.Context(), MockContainer(WithLabels(map[string]string{
			copyFileLabel: "relative.conf",
		})))
		require.ErrorIs(t, err, errCopyFileInvalidPath)
	})

	t.Run("inject missing snapshot is a no-op", func(t *testing.T) {
		t.Parallel()

		cli := &client{log: newTestLogger(t), api: mockContainer.NewMockAPIClient(t)}
		require.NoError(t, cli.injectStoredCopyFiles(t.Context(), MockContainer(), "new-id"))
	})

	t.Run("inject writes then drops snapshot", func(t *testing.T) {
		t.Parallel()

		tarBytes := mustFileTar(t, "config.yaml", []byte("setting=1"))
		source := MockContainer()
		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api)
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Return(dockerClient.CopyToContainerResult{}, nil)

		cli := &client{
			log: newTestLogger(t),
			api: api,
			copyFiles: map[types.ContainerID]copyFileSnapshot{
				source.ID(): {files: []copyFileEntry{{
					target:  "/app/config.yaml",
					archive: tarBytes,
				}}},
			},
			copyFileBytes: int64(len(tarBytes)),
		}
		require.NoError(t, cli.injectStoredCopyFiles(t.Context(), source, "new-id"))
		assert.Empty(t, cli.copyFiles)
		assert.Equal(t, int64(0), cli.copyFileBytes)
	})

	t.Run("inject failure still drops snapshot", func(t *testing.T) {
		t.Parallel()

		source := MockContainer()
		api := mockContainer.NewMockAPIClient(t)
		expectDaemonInfo(api)
		api.EXPECT().
			CopyToContainer(mock.Anything, "new-id", mock.Anything).
			Return(dockerClient.CopyToContainerResult{}, errors.New("denied"))

		cli := &client{
			log: newTestLogger(t),
			api: api,
			copyFiles: map[types.ContainerID]copyFileSnapshot{
				source.ID(): {files: []copyFileEntry{{
					target:  "/app/config.yaml",
					archive: []byte("tar"),
				}}},
			},
			copyFileBytes: 3,
		}
		err := cli.injectStoredCopyFiles(t.Context(), source, "new-id")
		require.ErrorIs(t, err, errCopyFileInjectFailed)
		assert.Empty(t, cli.copyFiles)
		assert.Equal(t, int64(0), cli.copyFileBytes)
	})

	t.Run("store clamps negative byte count", func(t *testing.T) {
		t.Parallel()

		cli := &client{copyFileBytes: -8}
		require.NoError(t, cli.storeCopyFileSnapshot("source-id", copyFileSnapshot{
			files: []copyFileEntry{{archive: []byte("tar")}},
		}))
		assert.Equal(t, int64(3), cli.copyFileBytes)
	})

	t.Run("discard clamps negative byte count", func(t *testing.T) {
		t.Parallel()

		cli := &client{
			copyFiles: map[types.ContainerID]copyFileSnapshot{
				"source-id": {files: []copyFileEntry{{archive: []byte("tar")}}},
			},
		}
		cli.DiscardCopyFiles("source-id")
		assert.Equal(t, int64(0), cli.copyFileBytes)
		assert.Empty(t, cli.copyFiles)
	})
}

type errReader struct {
	err error
}

func (e errReader) Read(_ []byte) (int, error) {
	return 0, e.err
}

// expectDaemonInfo stubs the single daemon info call made by one injection.
func expectDaemonInfo(api *mockContainer.MockAPIClient, securityOptions ...string) {
	api.EXPECT().
		Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(dockerClient.SystemInfoResult{
			Info: dockerSystem.Info{SecurityOptions: securityOptions},
		}, nil).
		Once()
}

func copyFromPath(filePath string) any {
	return mock.MatchedBy(func(options dockerClient.CopyFromContainerOptions) bool {
		return options.SourcePath == filePath
	})
}

func mustFileTar(t *testing.T, name string, body []byte) []byte {
	t.Helper()

	return mustFileTarWithMode(t, name, body, 0o444)
}

func mustFileTarWithMode(t *testing.T, name string, body []byte, mode int64) []byte {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     int64(len(body)),
		Mode:     mode,
		ModTime:  time.Unix(0, 0),
	}))

	_, err := tw.Write(body)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	return buf.Bytes()
}

func tarRegularFile(t *testing.T, archive []byte) (string, []byte, int64) {
	t.Helper()

	reader := tar.NewReader(bytes.NewReader(archive))
	header, err := reader.Next()
	require.NoError(t, err)
	require.Equal(t, byte(tar.TypeReg), header.Typeflag)

	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	_, err = reader.Next()
	require.ErrorIs(t, err, io.EOF)

	return header.Name, body, header.Mode
}
