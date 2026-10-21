package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/compose"
	mockCompose "github.com/nicholas-fedor/watchtower/internal/compose/mocks"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/git/project"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// writeComposeProjectDir writes a Compose project with two built services to a
// temporary directory.
//
// Parameters:
//   - t: Test the directory belongs to.
//
// Returns:
//   - string: The project directory.
func writeComposeProjectDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services:\n  api:\n    build: .\n  worker:\n    build: ./worker\n"), 0o600))

	return dir
}

// gitTestClient returns a Git client with a one-second timeout that logs to t.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *git.Client: The client.
func gitTestClient(t *testing.T) *git.Client {
	t.Helper()

	return git.New(newTestLogger(t), git.Options{Timeout: time.Second})
}

// expectGitBuild expects one remote Git build and answers it with an image ID
// derived from the first tag, or with the context's error once it is done.
//
// Parameters:
//   - client: Mocked container client.
func expectGitBuild(client *mockContainer.MockClient) {
	client.EXPECT().BuildRemoteImage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _, _ string, tags []string) (types.ImageID, error) {
			err := ctx.Err()
			if err != nil {
				return "", err
			}

			if len(tags) > 0 {
				return types.ImageID("sha256:gitbuild" + tags[0]), nil
			}

			return "sha256:gitbuild", nil
		},
	).Once()
}

// gitAssociatedContainer returns a running container associated with the
// Git repository https://github.com/org/app.git on its main branch.
//
// Parameters:
//   - id: Container ID.
//   - name: Container name.
//   - image: Image reference the container runs.
//   - extra: Labels added to the Git association labels.
//
// Returns:
//   - types.Container: The configured container.
func gitAssociatedContainer(id, name, image string, extra map[string]string) types.Container {
	labels := map[string]string{
		gitPkg.RepoLabel: "https://github.com/org/app.git",
		gitPkg.RefLabel:  "main",
	}

	maps.Copy(labels, extra)

	return mockActions.CreateMockContainerWithConfig(
		id,
		name,
		image,
		true,
		false,
		time.Now().Add(-time.Hour),
		&dockerContainer.Config{
			Image:  image,
			Labels: labels,
		},
	)
}

// Test_gitSession verifies the Git monitoring session of one update: which
// containers it checks and watches, how it stores results, how it rebuilds
// stale containers as remote Git builds or Compose applies, which containers
// it excludes from recreation and restarts, and how it tags rebuilt images.
func Test_gitSession(t *testing.T) {
	t.Parallel()

	t.Run("newGitSession", func(t *testing.T) {
		t.Parallel()

		t.Run("returns empty result and built maps", func(t *testing.T) {
			t.Parallel()

			client := gitTestClient(t)
			sess := newGitSession(client)

			require.NotNil(t, sess)
			assert.Equal(t, client, sess.client)
			assert.Empty(t, sess.results)
			assert.Empty(t, sess.built)
		})
	})

	t.Run("store and result", func(t *testing.T) {
		t.Parallel()

		t.Run("does nothing when the session is nil", func(t *testing.T) {
			t.Parallel()

			var sess *gitSession

			sess.store("abc", git.CheckResult{Stale: true, Commit: "deadbeef"})
			got, ok := sess.result("abc")
			assert.False(t, ok)
			assert.Equal(t, git.CheckResult{}, got)
		})

		t.Run("stores and retrieves a check result", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			want := git.CheckResult{Stale: true, Commit: "abc123", Tag: "v1.2.3", Kind: "tag", Ref: "v1.2.3"}

			sess.store("c1", want)
			got, ok := sess.result("c1")
			assert.True(t, ok)
			assert.Equal(t, want, got)
		})

		t.Run("returns false for an unknown container", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			_, ok := sess.result("missing")
			assert.False(t, ok)
		})
	})

	t.Run("watching", func(t *testing.T) {
		t.Parallel()

		t.Run("is false when the session is nil", func(t *testing.T) {
			t.Parallel()

			var sess *gitSession

			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)

			assert.False(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})

		t.Run("is false when the client is nil", func(t *testing.T) {
			t.Parallel()

			sess := &gitSession{}
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)

			assert.False(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})

		t.Run("is true for an associated container when monitoring is enabled", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)

			assert.True(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})

		t.Run("is false when git-watch is false", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", map[string]string{
				gitPkg.WatchLabel: "false",
			})

			assert.False(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})

		t.Run("is false for an unassociated container", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := mockActions.CreateMockContainer("id", "/app", "myapp:latest", time.Now())

			assert.False(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})

		t.Run("is false for Watchtower itself", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := mockActions.CreateMockContainerWithConfig(
				"wt",
				"/watchtower",
				"nickfedor/watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "nickfedor/watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower": "true",
						gitPkg.RepoLabel:                 "https://github.com/nicholas-fedor/watchtower.git",
						gitPkg.WatchLabel:                "true",
					},
				},
			)

			assert.False(t, sess.watching(newTestLogger(t), c, types.UpdateParams{EnableGitMonitoring: true}))
		})
	})

	t.Run("check", func(t *testing.T) {
		t.Parallel()

		t.Run("treats an unassociated container as fresh and stores the empty result", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := mockActions.CreateMockContainer("plain", "/plain", "nginx:latest", time.Now())

			stale, newest, err := sess.check(t.Context(), c, types.UpdateParams{})
			require.NoError(t, err)
			assert.False(t, stale)
			assert.Equal(t, c.ImageID(), newest)

			stored, ok := sess.result(c.ID())
			assert.True(t, ok)
			assert.False(t, stored.Stale)
		})

		t.Run("returns a wrapped error when the remote cannot be listed", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(git.New(newTestLogger(t), git.Options{Timeout: 50 * time.Millisecond}))
			c := gitAssociatedContainer("bad", "/bad", "myapp:latest", map[string]string{
				gitPkg.RepoLabel: "https://127.0.0.1:1/org/app.git",
			})

			ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()

			stale, newest, err := sess.check(ctx, c, types.UpdateParams{
				EnableGitMonitoring: true,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "git check:")
			assert.False(t, stale)
			assert.Empty(t, newest)
		})
	})

	t.Run("prepareRebuilds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns immediately when the session or client is nil", func(t *testing.T) {
			t.Parallel()

			failed := map[types.ContainerID]error{}
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			c.SetStale(true)

			var sess *gitSession
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				mockContainer.NewMockClient(t),
				[]types.Container{c},
				types.UpdateParams{},
				nil,
				failed,
			)
			assert.Empty(t, failed)

			sess = &gitSession{results: map[types.ContainerID]git.CheckResult{}}
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				mockContainer.NewMockClient(t),
				[]types.Container{c},
				types.UpdateParams{},
				nil,
				failed,
			)
			assert.Empty(t, failed)
		})

		t.Run("skips containers that are not stale or have no stored stale result", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			fresh := gitAssociatedContainer("fresh", "/fresh", "myapp:latest", nil)
			staleNoResult := gitAssociatedContainer("none", "/none", "myapp:latest", nil)
			staleFreshResult := gitAssociatedContainer("stored", "/stored", "myapp:latest", nil)

			staleNoResult.SetStale(true)
			staleFreshResult.SetStale(true)
			sess.store(staleFreshResult.ID(), git.CheckResult{Stale: false})

			failed := map[types.ContainerID]error{}
			// The mock has no expectations, so a build or image removal fails
			// the test.
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{fresh, staleNoResult, staleFreshResult}, types.UpdateParams{}, nil, failed)

			assert.Empty(t, failed)
		})

		t.Run("skips monitor-only and no-pull containers", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			monitor := gitAssociatedContainer("mon", "/mon", "myapp:latest", map[string]string{
				"com.centurylinklabs.watchtower.monitor-only": "true",
			})
			noPull := gitAssociatedContainer("np", "/np", "myapp:latest", map[string]string{
				"com.centurylinklabs.watchtower.no-pull": "true",
			})

			monitor.SetStale(true)
			noPull.SetStale(true)
			sess.store(monitor.ID(), git.CheckResult{Stale: true, Commit: "abc123abc123"})
			sess.store(noPull.ID(), git.CheckResult{Stale: true, Commit: "abc123abc123"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{monitor, noPull}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Empty(t, sess.built)
		})

		t.Run("records a failed build and unmarks the container stale", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			c := gitAssociatedContainer("fail", "/fail", "myapp:latest", nil)
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: ""})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Contains(t, failed, c.ID())
			assert.False(t, c.IsStale())
		})

		t.Run("builds a stale associated container", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			c := gitAssociatedContainer("ok", "/ok", "myapp:latest", nil)
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "0123456789abcdef0123456789abcdef01234567", Tag: "v1.0.0"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			expectGitBuild(docker)

			progress := &session.Progress{}
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, progress, failed)

			assert.Empty(t, failed)
			assert.Contains(t, sess.built, c.ID())
			assert.True(t, strings.HasPrefix(c.ImageName(), "docker.io/library/myapp:git-"))
			commit, tag := gitPkg.LastStamp(c)
			assert.Equal(t, "0123456789abcdef0123456789abcdef01234567", commit)
			assert.Equal(t, "v1.0.0", tag)
		})
	})

	t.Run("buildOne", func(t *testing.T) {
		t.Parallel()

		t.Run("returns errGitNotAssociated when the container has no repo", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := mockActions.CreateMockContainer("plain", "/plain", "nginx:latest", time.Now())
			docker := mockContainer.NewMockClient(t)

			err := sess.buildOne(newTestLogger(t), t.Context(), docker, c, git.CheckResult{Commit: "abc"}, types.UpdateParams{}, nil)
			require.ErrorIs(t, err, errGitNotAssociated)
		})

		t.Run("returns an error when the commit is empty", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			docker := mockContainer.NewMockClient(t)

			err := sess.buildOne(newTestLogger(t), t.Context(), docker, c, git.CheckResult{Stale: true}, types.UpdateParams{EnableGitMonitoring: true}, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "git remote context:")
		})

		t.Run("returns an error when the build context is cancelled", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			docker := mockContainer.NewMockClient(t)
			expectGitBuild(docker)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := sess.buildOne(newTestLogger(t), ctx, docker, c, git.CheckResult{Stale: true, Commit: "abc123abc123"}, types.UpdateParams{EnableGitMonitoring: true}, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "build git image:")
		})

		t.Run("keeps a nested dockerfile and context on the rebuilt container", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", map[string]string{
				gitPkg.DockerfileLabel: "build/docker/Dockerfile",
				gitPkg.ContextLabel:    ".",
			})
			docker := mockContainer.NewMockClient(t)
			expectGitBuild(docker)

			err := sess.buildOne(
				newTestLogger(t),
				t.Context(),
				docker,
				c,
				git.CheckResult{Stale: true, Commit: "ffffffffffffffffffffffffffffffffffffffff"},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
			)
			require.NoError(t, err)

			assoc, ok := gitPkg.ResolveAssociation(c, types.UpdateParams{EnableGitMonitoring: true})
			assert.True(t, ok)
			assert.Equal(t, "build/docker/Dockerfile", assoc.Dockerfile)
			assert.Equal(t, ".", assoc.Context)

			_, watchSet := c.GetLabel(gitPkg.WatchLabel)
			assert.False(t, watchSet)
		})

		t.Run("persists git-watch only when the source label enabled it", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", map[string]string{
				gitPkg.WatchLabel: "yes",
			})
			docker := mockContainer.NewMockClient(t)
			expectGitBuild(docker)

			err := sess.buildOne(
				newTestLogger(t),
				t.Context(),
				docker,
				c,
				git.CheckResult{Stale: true, Commit: "ffffffffffffffffffffffffffffffffffffffff"},
				types.UpdateParams{},
				nil,
			)
			require.NoError(t, err)

			got, ok := c.GetLabel(gitPkg.WatchLabel)
			assert.True(t, ok)
			assert.Equal(t, "true", got)
		})
		t.Run("does not pass a digest-pinned image as a build tag", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			image := "ghcr.io/org/app@sha256:" + strings.Repeat("a", 64)
			commit := "0123456789abcdef0123456789abcdef01234567"
			c := gitAssociatedContainer("digest", "/digest", image, nil)

			var tags []string

			docker := mockContainer.NewMockClient(t)
			docker.EXPECT().
				BuildRemoteImage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(_ context.Context, _, _ string, got []string) {
					tags = append([]string(nil), got...)
				}).
				Return(types.ImageID("sha256:built"), nil)

			err := sess.buildOne(
				newTestLogger(t),
				t.Context(),
				docker,
				c,
				git.CheckResult{Stale: true, Commit: commit},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
			)
			require.NoError(t, err)
			require.Len(t, tags, 1)
			assert.Equal(t, "ghcr.io/org/app:git-0123456789ab", tags[0])
			assert.NotContains(t, tags[0], "@")
		})

		t.Run("applies the Git timeout to the URL-context build", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(git.New(newTestLogger(t), git.Options{Timeout: 50 * time.Millisecond}))
			commit := "0123456789abcdef0123456789abcdef01234567"
			c := gitAssociatedContainer("timeout", "/timeout", "myapp:latest", nil)

			docker := mockContainer.NewMockClient(t)
			docker.EXPECT().
				BuildRemoteImage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(ctx context.Context, _, _ string, _ []string) {
					deadline, ok := ctx.Deadline()
					assert.True(t, ok)
					assert.LessOrEqual(t, time.Until(deadline), 50*time.Millisecond)
				}).
				Return(types.ImageID("sha256:built"), nil)

			err := sess.buildOne(
				newTestLogger(t),
				t.Context(),
				docker,
				c,
				git.CheckResult{Stale: true, Commit: commit},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
			)
			require.NoError(t, err)
		})

		t.Run("keeps a build that returns nil after its timeout elapses", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(git.New(newTestLogger(t), git.Options{Timeout: 10 * time.Millisecond}))
			c := gitAssociatedContainer("timeout", "/timeout", "myapp:latest", nil)
			commit := "0123456789abcdef0123456789abcdef01234567"

			docker := mockContainer.NewMockClient(t)
			docker.EXPECT().
				BuildRemoteImage(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(ctx context.Context, _, _ string, _ []string) (types.ImageID, error) {
					<-ctx.Done()

					return "sha256:built", nil
				}).
				Once()

			err := sess.buildOne(
				newTestLogger(t),
				t.Context(),
				docker,
				c,
				git.CheckResult{Stale: true, Commit: commit},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, types.ImageID("sha256:built"), sess.built[c.ID()])
		})
	})

	t.Run("skipRecreate", func(t *testing.T) {
		t.Parallel()

		t.Run("is false when the session is nil", func(t *testing.T) {
			t.Parallel()

			var sess *gitSession

			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)

			assert.False(t, sess.skipRecreate(c, types.UpdateParams{NoRestart: true}))
		})

		t.Run("is false when no-restart is off", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			sess.built[c.ID()] = "sha256:git"

			assert.False(t, sess.skipRecreate(c, types.UpdateParams{}))
		})

		t.Run("is false for Watchtower even when an image was built", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := mockActions.CreateMockContainerWithConfig(
				"wt",
				"/watchtower",
				"nickfedor/watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "nickfedor/watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				},
			)
			sess.built[c.ID()] = "sha256:git"

			assert.False(t, sess.skipRecreate(c, types.UpdateParams{NoRestart: true}))
		})

		t.Run("is true when no-restart is on and this session built the image", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			sess.built[c.ID()] = "sha256:git"

			assert.True(t, sess.skipRecreate(c, types.UpdateParams{NoRestart: true}))
		})
	})

	t.Run("excludeNoRestart", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the original list when the session is nil or no-restart is off", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			list := []types.Container{c}

			var sess *gitSession
			assert.Equal(t, list, sess.excludeNoRestart(list, types.UpdateParams{NoRestart: true}))

			sess = newGitSession(gitTestClient(t))
			assert.Equal(t, list, sess.excludeNoRestart(list, types.UpdateParams{}))
		})

		t.Run("drops containers this session built when no-restart is on", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			built := gitAssociatedContainer("built", "/built", "myapp:latest", nil)
			kept := gitAssociatedContainer("kept", "/kept", "other:latest", nil)
			sess.built[built.ID()] = "sha256:git"

			got := sess.excludeNoRestart([]types.Container{built, kept}, types.UpdateParams{NoRestart: true})
			require.Len(t, got, 1)
			assert.Equal(t, kept.ID(), got[0].ID())
		})
	})

	t.Run("excludeApplied", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the original list when the session is nil or nothing was applied", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)
			list := []types.Container{c}

			var sess *gitSession
			assert.Equal(t, list, sess.excludeApplied(list))

			sess = newGitSession(gitTestClient(t))
			assert.Equal(t, list, sess.excludeApplied(list))
		})

		t.Run("drops compose-applied containers", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			applied := gitAssociatedContainer("applied", "/applied", "myapp:latest", nil)
			kept := gitAssociatedContainer("kept", "/kept", "other:latest", nil)
			sess.applied[applied.ID()] = struct{}{}

			got := sess.excludeApplied([]types.Container{applied, kept})
			require.Len(t, got, 1)
			assert.Equal(t, kept.ID(), got[0].ID())
		})
	})

	t.Run("compose apply", func(t *testing.T) {
		t.Parallel()

		t.Run("checkouts once and applies stale services that share a project dir", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var got compose.Request

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Run(func(ctx context.Context, req compose.Request) {
					deadline, ok := ctx.Deadline()
					assert.True(t, ok)
					assert.LessOrEqual(t, time.Until(deadline), time.Second)

					got = req
				}).
				Return([]compose.Container{
					{Service: "api", ID: "new-api", ImageID: "sha256:api"},
					{Service: "worker", ID: "new-worker", ImageID: "sha256:worker"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier

			var checkouts int

			sess.checkout = func(ctx context.Context, gotDir, commit string, opts project.CheckoutOptions) error {
				deadline, ok := ctx.Deadline()
				assert.True(t, ok)
				assert.LessOrEqual(t, time.Until(deadline), time.Second)

				checkouts++

				assert.Equal(t, dir, gotDir)
				assert.Equal(t, "0123456789abcdef0123456789abcdef01234567", commit)
				assert.True(t, opts.Stash)
				require.NotNil(t, opts.AuthFor)

				return nil
			}

			api := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "webstack",
				compose.ComposeServiceLabel: "api",
			})
			worker := gitAssociatedContainer("worker", "/worker", "webstack-worker:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "webstack",
				compose.ComposeServiceLabel: "worker",
			})

			api.SetStale(true)
			worker.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "0123456789abcdef0123456789abcdef01234567", Tag: "v1.2.3"}
			sess.store(api.ID(), result)
			sess.store(worker.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				docker,
				[]types.Container{api, worker},
				types.UpdateParams{EnableGitMonitoring: true, GitComposeStash: true},
				nil,
				failed,
			)

			assert.Empty(t, failed)
			assert.Equal(t, 1, checkouts)
			assert.Equal(t, dir, got.Ref.Dir)
			assert.ElementsMatch(t, []string{"api", "worker"}, got.Services)
			assert.False(t, got.BuildOnly)
			assert.Equal(t, result.Commit, got.Labels["api"][gitPkg.LastCommitLabel])
			assert.Contains(t, sess.applied, api.ID())
			assert.Contains(t, sess.applied, worker.ID())
			assert.True(t, sess.skipRecreate(api, types.UpdateParams{}))
		})

		t.Run("separates projects that share a directory and config identity", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			override := filepath.Join(dir, "override.yml")
			require.NoError(t, os.WriteFile(override, []byte("services: {}\n"), 0o600))

			applier := mockCompose.NewMockApplier(t)

			var requests []compose.Request

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, req compose.Request) ([]compose.Container, error) {
					requests = append(requests, req)

					return []compose.Container{{
						Service: req.Services[0],
						Name:    "/" + req.Ref.Name + "-api",
						ID:      types.ContainerID("new-" + req.Ref.Name),
						ImageID: types.ImageID("sha256:" + req.Ref.Name),
					}}, nil
				}).
				Twice()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier

			var checkouts int

			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				checkouts++

				return nil
			}

			alpha := gitAssociatedContainer("alpha", "alpha-api", "alpha:latest", map[string]string{
				gitPkg.ComposeDirLabel:          dir,
				compose.ComposeProjectLabel:     "alpha",
				compose.ComposeServiceLabel:     "api",
				compose.ComposeConfigFilesLabel: "compose.yaml",
			})
			beta := gitAssociatedContainer("beta", "beta-api", "beta:latest", map[string]string{
				gitPkg.ComposeDirLabel:          dir,
				compose.ComposeProjectLabel:     "beta",
				compose.ComposeServiceLabel:     "api",
				compose.ComposeConfigFilesLabel: "override.yml",
			})

			alpha.SetStale(true)
			beta.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(alpha.ID(), result)
			sess.store(beta.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{alpha, beta}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Equal(t, 2, checkouts)
			require.Len(t, requests, 2)
			assert.NotEqual(t, requests[1].Ref.Name, requests[0].Ref.Name)
			assert.NotEqual(t, requests[1].Ref.ConfigFiles, requests[0].Ref.ConfigFiles)
			assert.Contains(t, sess.applied, alpha.ID())
			assert.Contains(t, sess.applied, beta.ID())
		})

		t.Run("applies dependent Compose projects after their dependencies", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var order []string

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, req compose.Request) ([]compose.Container, error) {
					order = append(order, req.Ref.Name)

					return []compose.Container{{
						Service: req.Services[0],
						Name:    "/" + req.Ref.Name + "-api",
						ID:      types.ContainerID("new-" + req.Ref.Name),
						ImageID: types.ImageID("sha256:" + req.Ref.Name),
					}}, nil
				}).
				Twice()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			provider := gitAssociatedContainer("provider", "alpha-api", "alpha:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "alpha",
				compose.ComposeServiceLabel: "api",
			})
			dependent := gitAssociatedContainer("dependent", "beta-api", "beta:latest", map[string]string{
				gitPkg.ComposeDirLabel:                      dir,
				compose.ComposeProjectLabel:                 "beta",
				compose.ComposeServiceLabel:                 "api",
				"com.centurylinklabs.watchtower.depends-on": "alpha-api",
			})

			provider.SetStale(true)
			dependent.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(provider.ID(), result)
			sess.store(dependent.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{dependent, provider}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Equal(t, []string{"alpha", "beta"}, order)
		})

		t.Run("applies a dependency project before a project whose first service is independent", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var order []string

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, req compose.Request) ([]compose.Container, error) {
					order = append(order, req.Ref.Name)

					items := make([]compose.Container, 0, len(req.Services))
					for _, service := range req.Services {
						items = append(items, compose.Container{
							Service: service,
							Name:    "/" + req.Ref.Name + "-" + service,
							ID:      types.ContainerID("new-" + req.Ref.Name + "-" + service),
							ImageID: types.ImageID("sha256:" + req.Ref.Name),
						})
					}

					return items, nil
				}).
				Twice()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			metrics := gitAssociatedContainer("metrics", "beta-metrics", "beta:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "beta",
				compose.ComposeServiceLabel: "metrics",
			})
			dependent := gitAssociatedContainer("dependent", "beta-api", "beta:latest", map[string]string{
				gitPkg.ComposeDirLabel:                      dir,
				compose.ComposeProjectLabel:                 "beta",
				compose.ComposeServiceLabel:                 "api",
				"com.centurylinklabs.watchtower.depends-on": "alpha-api",
			})
			provider := gitAssociatedContainer("provider", "alpha-api", "alpha:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "alpha",
				compose.ComposeServiceLabel: "api",
			})

			metrics.SetStale(true)
			dependent.SetStale(true)
			provider.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(metrics.ID(), result)
			sess.store(dependent.ID(), result)
			sess.store(provider.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				docker,
				[]types.Container{metrics, dependent, provider},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
				failed,
			)

			assert.Empty(t, failed)
			assert.Equal(t, []string{"alpha", "beta"}, order)
		})

		t.Run("keeps a Compose apply that returns success after its timeout elapses", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				RunAndReturn(func(context.Context, compose.Request) ([]compose.Container, error) {
					time.Sleep(30 * time.Millisecond)

					return []compose.Container{{
						Service: "api",
						Name:    "/web-api",
						ID:      "new-api",
						ImageID: "sha256:api",
					}}, nil
				}).
				Once()

			sess := newGitSession(git.New(newTestLogger(t), git.Options{Timeout: 5 * time.Millisecond}))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			c := gitAssociatedContainer("api", "web-api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "webstack",
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				docker,
				[]types.Container{c},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
				failed,
			)

			assert.Empty(t, failed)
			assert.Contains(t, sess.applied, c.ID())
		})

		t.Run("does not apply after the parent context is canceled during checkout", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			parent, cancel := context.WithCancel(t.Context())

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(_ context.Context, _, _ string, opts project.CheckoutOptions) error {
				if opts.Prior != nil {
					*opts.Prior = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}

				cancel()

				return nil
			}

			c := gitAssociatedContainer("api", "web-api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "webstack",
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(
				newTestLogger(t),
				parent,
				docker,
				[]types.Container{c},
				types.UpdateParams{EnableGitMonitoring: true},
				nil,
				failed,
			)

			require.ErrorIs(t, failed[c.ID()], context.Canceled)
			assert.Empty(t, sess.applied)
		})

		t.Run("reports a normal Compose apply as updated", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{{Service: "api", ID: "new-api", ImageID: "sha256:api"}}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			c := gitAssociatedContainer("api", "web-1", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeProjectLabel: "webstack",
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "0123456789abcdef0123456789abcdef01234567", Tag: "v1.2.3"}
			sess.store(c.ID(), result)

			params := types.UpdateParams{EnableGitMonitoring: true}
			progress := &session.Progress{}
			progress.AddScanned(newTestLogger(t), c, types.ImageID("git:"+result.Commit), params)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, params, progress, failed)

			report := progress.Report(newTestLogger(t))

			assert.Empty(t, failed)
			require.Len(t, report.Updated(), 1)
			assert.Equal(t, types.ContainerID("new-api"), report.Updated()[0].NewContainerID())
			assert.Empty(t, report.Stale())
		})

		t.Run("uses a remote Git build when compose-dir is unset", func(t *testing.T) {
			t.Parallel()

			sess := newGitSession(gitTestClient(t))
			applier := mockCompose.NewMockApplier(t)
			sess.apply = applier
			c := gitAssociatedContainer("ok", "/ok", "myapp:latest", nil)
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "0123456789abcdef0123456789abcdef01234567"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			expectGitBuild(docker)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Contains(t, sess.built, c.ID())
			assert.Empty(t, sess.applied)
		})

		t.Run("skips when compose-dir is set but the path is not a compose project", func(t *testing.T) {
			t.Parallel()

			missing := filepath.Join(t.TempDir(), "missing")
			applier := mockCompose.NewMockApplier(t)
			sess := newGitSession(gitTestClient(t))
			sess.apply = applier

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      missing,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.NotContains(t, failed, c.ID())
			assert.False(t, c.IsStale())
			assert.Empty(t, sess.built)
			assert.Empty(t, sess.applied)
		})

		t.Run("skips a scanned container when compose-dir is set but the path is not a compose project", func(t *testing.T) {
			t.Parallel()

			missing := filepath.Join(t.TempDir(), "missing")
			applier := mockCompose.NewMockApplier(t)
			sess := newGitSession(gitTestClient(t))
			sess.apply = applier

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      missing,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			params := types.UpdateParams{EnableGitMonitoring: true}
			log := newTestLogger(t)
			progress := &session.Progress{}
			progress.AddScanned(log, c, types.ImageID("git:abc"), params)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(log, t.Context(), docker, []types.Container{c}, params, progress, failed)
			progress.UpdateFailed(log, failed)

			report := progress.Report(log)

			assert.NotContains(t, failed, c.ID())
			assert.False(t, c.IsStale())
			assert.Empty(t, sess.built)
			assert.Empty(t, sess.applied)
			require.Len(t, report.Skipped(), 1)
			assert.Equal(t, c.ID(), report.Skipped()[0].ID())
			assert.Contains(t, report.Skipped()[0].Error(), compose.ErrProjectDir.Error())
			assert.Empty(t, report.Failed())
			assert.Empty(t, report.Scanned())
		})

		t.Run("fails a compose container that has no service label", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel: dir,
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			require.ErrorIs(t, failed[c.ID()], errGitComposeService)
			assert.False(t, c.IsStale())
		})

		t.Run("aborts the project when checkout fails", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return project.ErrDirty
			}

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			require.ErrorIs(t, failed[c.ID()], project.ErrDirty)
			assert.False(t, c.IsStale())
		})

		t.Run("applies after checkout returns nil when only its timeout elapsed", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{{Service: "api", Name: "/api", ID: "new-api", ImageID: "sha256:api"}}, nil).
				Once()

			sess := newGitSession(git.New(newTestLogger(t), git.Options{Timeout: 10 * time.Millisecond}))
			sess.apply = applier
			sess.checkout = func(ctx context.Context, _, _ string, _ project.CheckoutOptions) error {
				<-ctx.Done()

				return nil
			}

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{c}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Contains(t, sess.applied, c.ID())
		})

		t.Run("unmarks stale containers when compose apply fails", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{
					{Service: "api", Name: "/api", ID: "new-api", ImageID: "sha256:api"},
					{Service: "api", Name: "/fresh", ID: "new-fresh", ImageID: "sha256:api"},
				}, context.Canceled).
				Once()

			var logs bytes.Buffer

			log := zerolog.New(&logs)
			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			fresh := gitAssociatedContainer("fresh", "/fresh", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})

			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			params := types.UpdateParams{EnableGitMonitoring: true}
			progress := &session.Progress{}
			progress.AddScanned(&log, c, types.ImageID("git:new"), params)
			progress.AddScanned(&log, fresh, fresh.ImageID(), params)
			sess.prepareRebuilds(&log, t.Context(), docker, []types.Container{c}, params, progress, failed)
			progress.UpdateFailed(&log, failed)

			report := progress.Report(&log)

			require.ErrorIs(t, failed[c.ID()], context.Canceled)
			assert.Contains(t, failed[c.ID()].Error(), "partially recreated services")
			require.ErrorIs(t, failed[fresh.ID()], context.Canceled)
			assert.False(t, c.IsStale())
			assert.Empty(t, sess.applied)
			assert.Empty(t, report.Updated())
			require.Len(t, report.Failed(), 2)
			assert.Contains(t, logs.String(), "may have partially recreated services")
			assert.NotContains(t, logs.String(), "Leaving running container untouched")
		})

		t.Run("fails a service omitted from the compose apply result", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{{Service: "api", ID: "new-api", ImageID: "sha256:api"}}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			api := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			worker := gitAssociatedContainer("worker", "/worker", "webstack-worker:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "worker",
			})

			api.SetStale(true)
			worker.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(api.ID(), result)
			sess.store(worker.ID(), result)

			var logs bytes.Buffer

			log := zerolog.New(&logs)
			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			params := types.UpdateParams{EnableGitMonitoring: true}
			progress := &session.Progress{}
			progress.AddScanned(&log, api, types.ImageID("git:abc"), params)
			progress.AddScanned(&log, worker, types.ImageID("git:abc"), params)
			sess.prepareRebuilds(&log, t.Context(), docker, []types.Container{api, worker}, params, progress, failed)
			progress.UpdateFailed(&log, failed)

			report := progress.Report(&log)

			assert.NotContains(t, failed, api.ID())
			require.ErrorIs(t, failed[worker.ID()], errGitComposeInstance)
			assert.False(t, worker.IsStale())
			assert.Contains(t, sess.applied, api.ID())
			assert.NotContains(t, sess.applied, worker.ID())
			require.Len(t, report.Updated(), 1)
			require.Len(t, report.Failed(), 1)
			assert.Contains(t, logs.String(), "Runtime state is unknown")
			assert.NotContains(t, logs.String(), "Leaving running container untouched")
		})

		t.Run("builds only when no-restart is set", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var got compose.Request

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Run(func(_ context.Context, req compose.Request) {
					got = req
				}).
				Return([]compose.Container{{Service: "api", ImageID: "sha256:built"}}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			params := types.UpdateParams{EnableGitMonitoring: true, NoRestart: true}
			progress := &session.Progress{}
			progress.AddScanned(newTestLogger(t), c, types.ImageID("git:abc"), params)
			sess.prepareRebuilds(
				newTestLogger(t),
				t.Context(),
				docker,
				[]types.Container{c},
				params,
				progress,
				failed,
			)

			report := progress.Report(newTestLogger(t))

			assert.Empty(t, failed)
			assert.True(t, got.BuildOnly)
			assert.Empty(t, sess.applied)
			assert.Equal(t, types.ImageID("sha256:built"), sess.built[c.ID()])
			assert.True(t, sess.skipRecreate(c, params))
			assert.Empty(t, report.Updated())
			require.Len(t, report.Stale(), 1)
		})

		t.Run("reports Compose build failures without partial runtime claims", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return(nil, context.Canceled).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			c.SetStale(true)
			sess.store(c.ID(), git.CheckResult{Stale: true, Commit: "abc"})

			var logs bytes.Buffer

			log := zerolog.New(&logs)
			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			params := types.UpdateParams{EnableGitMonitoring: true, NoRestart: true}
			sess.prepareRebuilds(&log, t.Context(), docker, []types.Container{c}, params, nil, failed)

			require.ErrorIs(t, failed[c.ID()], context.Canceled)
			assert.Contains(t, failed[c.ID()].Error(), "compose build failed")
			assert.False(t, c.IsStale())
			assert.Empty(t, sess.built)
			assert.Empty(t, sess.applied)
			assert.NotContains(t, logs.String(), "partially recreated services")
		})

		t.Run("fails the project when services resolve different commits", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			checkedOut := false
			sess := newGitSession(gitTestClient(t))
			sess.apply = mockCompose.NewMockApplier(t)
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				checkedOut = true

				return nil
			}

			api := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			worker := gitAssociatedContainer("worker", "/worker", "webstack-worker:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "worker",
			})

			api.SetStale(true)
			worker.SetStale(true)
			sess.store(api.ID(), git.CheckResult{Stale: true, Commit: "aaa"})
			sess.store(worker.ID(), git.CheckResult{Stale: true, Commit: "bbb"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{api, worker}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.False(t, checkedOut)
			require.ErrorIs(t, failed[api.ID()], errGitComposeCommit)
			require.ErrorIs(t, failed[worker.ID()], errGitComposeCommit)
			assert.False(t, api.IsStale())
			assert.False(t, worker.IsStale())
		})

		t.Run("stamps each service from its own tag", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var got compose.Request

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Run(func(_ context.Context, req compose.Request) {
					got = req
				}).
				Return([]compose.Container{
					{Service: "api", ID: "new-api", ImageID: "sha256:api"},
					{Service: "worker", ID: "new-worker", ImageID: "sha256:worker"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			api := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			worker := gitAssociatedContainer("worker", "/worker", "webstack-worker:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "worker",
			})

			api.SetStale(true)
			worker.SetStale(true)
			sess.store(api.ID(), git.CheckResult{Stale: true, Commit: "abc", Tag: "v1.2.3"})
			sess.store(worker.ID(), git.CheckResult{Stale: true, Commit: "abc", Tag: "v1.2.4"})

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{api, worker}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Equal(t, "v1.2.3", got.Labels["api"][gitPkg.LastTagLabel])
			assert.Equal(t, "v1.2.4", got.Labels["worker"][gitPkg.LastTagLabel])
			assert.Contains(t, sess.applied, api.ID())
			assert.Contains(t, sess.applied, worker.ID())
		})

		t.Run("marks every replica of a service applied", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)

			var got compose.Request

			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Run(func(_ context.Context, req compose.Request) {
					got = req
				}).
				Return([]compose.Container{
					{Service: "api", Name: "/web-1", ID: "new-1", ImageID: "sha256:api"},
					{Service: "api", Name: "/web-2", ID: "new-2", ImageID: "sha256:api"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			one := gitAssociatedContainer("api-1", "web-1", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			two := gitAssociatedContainer("api-2", "web-2", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})

			one.SetStale(true)
			two.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(one.ID(), result)
			sess.store(two.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{one, two}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.Empty(t, failed)
			assert.Equal(t, []string{"api"}, got.Services)
			assert.Contains(t, sess.applied, one.ID())
			assert.Contains(t, sess.applied, two.ID())
		})

		t.Run("tracks every recreated replica in a mixed stale and fresh service", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{
					{Service: "api", Name: "/web-1", ID: "new-1", ImageID: "sha256:api"},
					{Service: "api", Name: "/web-2", ID: "new-2", ImageID: "sha256:api"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			stale := gitAssociatedContainer("api-1", "web-1", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			fresh := gitAssociatedContainer("api-2", "web-2", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})

			stale.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc", Tag: "v1.2.3"}
			sess.store(stale.ID(), result)

			params := types.UpdateParams{EnableGitMonitoring: true}
			progress := &session.Progress{}
			progress.AddScanned(newTestLogger(t), stale, types.ImageID("git:"+result.Commit), params)
			progress.AddScanned(newTestLogger(t), fresh, fresh.ImageID(), params)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{stale}, params, progress, failed)

			report := progress.Report(newTestLogger(t))

			assert.Empty(t, failed)
			assert.Contains(t, sess.applied, stale.ID())
			assert.Contains(t, sess.applied, fresh.ID())
			require.Len(t, report.Updated(), 2)
			assert.Empty(t, report.Stale())
			assert.NotEmpty(t, report.Updated()[0].NewContainerID())
			assert.NotEmpty(t, report.Updated()[1].NewContainerID())
			assert.Empty(t, sess.excludeApplied([]types.Container{stale, fresh}))
		})

		t.Run("accepts additional Compose replicas absent from filtered progress", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{
					{Service: "api", Name: "/web-1", ID: "new-1", ImageID: "sha256:api"},
					{Service: "api", Name: "/web-2", ID: "new-2", ImageID: "sha256:api"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			stale := gitAssociatedContainer("api-1", "web-1", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			fresh := gitAssociatedContainer("api-2", "web-2", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})

			stale.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc", Tag: "v1.2.3"}
			sess.store(stale.ID(), result)

			params := types.UpdateParams{EnableGitMonitoring: true}
			progress := &session.Progress{}
			progress.AddScanned(newTestLogger(t), stale, types.ImageID("git:"+result.Commit), params)

			var logs bytes.Buffer

			log := zerolog.New(&logs)
			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(&log, t.Context(), docker, []types.Container{stale}, params, progress, failed)

			report := progress.Report(&log)

			assert.Empty(t, failed)
			assert.Contains(t, sess.applied, stale.ID())
			assert.NotContains(t, sess.applied, fresh.ID())
			require.Len(t, report.Updated(), 1)
			assert.Contains(t, logs.String(), "replica absent from filtered progress")
		})

		t.Run("does not apply a replica compose did not identify", func(t *testing.T) {
			t.Parallel()

			dir := writeComposeProjectDir(t)
			applier := mockCompose.NewMockApplier(t)
			applier.EXPECT().
				Apply(mock.Anything, mock.Anything).
				Return([]compose.Container{
					{Service: "api", Name: "web-1", ID: "new-1", ImageID: "sha256:api"},
				}, nil).
				Once()

			sess := newGitSession(gitTestClient(t))
			sess.apply = applier
			sess.checkout = func(context.Context, string, string, project.CheckoutOptions) error {
				return nil
			}

			one := gitAssociatedContainer("api-1", "web-1", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})
			two := gitAssociatedContainer("api-2", "web-2", "webstack-api:latest", map[string]string{
				gitPkg.ComposeDirLabel:      dir,
				compose.ComposeServiceLabel: "api",
			})

			one.SetStale(true)
			two.SetStale(true)

			result := git.CheckResult{Stale: true, Commit: "abc"}
			sess.store(one.ID(), result)
			sess.store(two.ID(), result)

			failed := map[types.ContainerID]error{}
			docker := mockContainer.NewMockClient(t)
			sess.prepareRebuilds(newTestLogger(t), t.Context(), docker, []types.Container{one, two}, types.UpdateParams{EnableGitMonitoring: true}, nil, failed)

			assert.NotContains(t, failed, one.ID())
			require.ErrorIs(t, failed[two.ID()], errGitComposeInstance)
			assert.False(t, two.IsStale())
			assert.Contains(t, sess.applied, one.ID())
			assert.NotContains(t, sess.applied, two.ID())
		})
	})

	t.Run("composeServiceLabels", func(t *testing.T) {
		t.Parallel()

		t.Run("omits git-watch when only process-wide enable is set", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", nil)
			got := composeServiceLabels(c, types.UpdateParams{EnableGitMonitoring: true}, git.CheckResult{Commit: "abc"})
			assert.NotContains(t, got, gitPkg.WatchLabel)
			assert.Equal(t, "https://github.com/org/app.git", got[gitPkg.RepoLabel])
		})

		t.Run("redacts credentials from the Compose repository label", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.RepoLabel: "https://user:secret@example.com/org/app.git?token=secret",
			})
			got := composeServiceLabels(c, types.UpdateParams{EnableGitMonitoring: true}, git.CheckResult{Commit: "abc"})
			assert.Equal(t, "https://example.com/org/app.git", got[gitPkg.RepoLabel])
			assert.NotContains(t, got[gitPkg.RepoLabel], "secret")
		})

		t.Run("redacts API origin credentials while preserving path and port", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.HostLabel: "https://user:secret@git.example.com:8443/git/api?token=secret",
			})
			got := composeServiceLabels(c, types.UpdateParams{EnableGitMonitoring: true}, git.CheckResult{Commit: "abc"})
			assert.Equal(t, "https://git.example.com:8443/git/api", got[gitPkg.HostLabel])
			assert.NotContains(t, got[gitPkg.HostLabel], "secret")
		})

		t.Run("omits malformed API origins", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.HostLabel: "https://",
			})
			got := composeServiceLabels(c, types.UpdateParams{EnableGitMonitoring: true}, git.CheckResult{Commit: "abc"})
			assert.NotContains(t, got, gitPkg.HostLabel)
		})

		t.Run("writes git-watch when the source label enabled it", func(t *testing.T) {
			t.Parallel()

			c := gitAssociatedContainer("api", "/api", "webstack-api:latest", map[string]string{
				gitPkg.WatchLabel: "true",
			})
			got := composeServiceLabels(c, types.UpdateParams{}, git.CheckResult{Commit: "abc"})
			assert.Equal(t, "true", got[gitPkg.WatchLabel])
		})
	})

	t.Run("reconcileImplicitRestarts", func(t *testing.T) {
		t.Parallel()

		t.Run("drops dependents when the anchor apply failed", func(t *testing.T) {
			t.Parallel()

			log := newTestLogger(t)
			app := mockActions.CreateMockContainerWithLinks(
				"app",
				"/app",
				"app:latest",
				time.Now(),
				nil,
				nil,
			)
			db := mockActions.CreateMockContainerWithLinks(
				"db",
				"/db",
				"db:latest",
				time.Now(),
				[]string{"/app:app"},
				nil,
			)

			app.SetStale(true)
			containers := []types.Container{app, db}
			updateImplicitRestart(log, containers, containers, false)
			assert.True(t, db.IsLinkedToRestarting())

			app.SetStale(false)

			got := reconcileImplicitRestarts(log, containers, containers, types.UpdateParams{})

			assert.False(t, db.IsLinkedToRestarting())
			assert.Empty(t, got)
		})

		t.Run("keeps dependents when the anchor is still stale", func(t *testing.T) {
			t.Parallel()

			log := newTestLogger(t)
			app := mockActions.CreateMockContainerWithLinks(
				"app",
				"/app",
				"app:latest",
				time.Now(),
				nil,
				nil,
			)
			db := mockActions.CreateMockContainerWithLinks(
				"db",
				"/db",
				"db:latest",
				time.Now(),
				[]string{"/app:app"},
				nil,
			)

			app.SetStale(true)
			containers := []types.Container{app, db}
			got := reconcileImplicitRestarts(log, containers, containers, types.UpdateParams{})

			assert.True(t, db.IsLinkedToRestarting())
			assert.ElementsMatch(t, []types.Container{app, db}, got)
		})

		t.Run("does not propagate restarts from a no-restart build anchor", func(t *testing.T) {
			t.Parallel()

			log := newTestLogger(t)
			app := mockActions.CreateMockContainerWithLinks(
				"app",
				"/app",
				"app:latest",
				time.Now(),
				nil,
				nil,
			)
			dependent := mockActions.CreateMockContainerWithLinks(
				"dependent",
				"/dependent",
				"dependent:latest",
				time.Now(),
				[]string{"/app:app"},
				nil,
			)

			app.SetStale(true)
			containers := []types.Container{app, dependent}
			sess := newGitSession(gitTestClient(t))
			sess.built[app.ID()] = "sha256:git"

			graph, err := sorter.NewDependencyGraph(log, containers, nil, false)
			require.NoError(t, err)

			got := reconcileImplicitRestartsExcluding(
				log,
				graph,
				containers,
				containers,
				types.UpdateParams{NoRestart: true},
				sess.noRestartBuiltIDs(types.UpdateParams{NoRestart: true}),
			)

			assert.False(t, dependent.IsLinkedToRestarting())
			assert.Empty(t, got)
		})
		t.Run("does not fall through from an excluded exact link to a service-name match", func(t *testing.T) {
			t.Parallel()

			log := newTestLogger(t)
			excluded := mockActions.CreateMockContainerWithLinks(
				"app",
				"/app",
				"app:latest",
				time.Now(),
				nil,
				nil,
			)
			serviceMatch := mockActions.CreateMockContainerWithLinks(
				"app-1",
				"/app-1",
				"app:latest",
				time.Now(),
				nil,
				nil,
			)
			dependent := mockActions.CreateMockContainerWithLinks(
				"dependent",
				"/dependent",
				"dependent:latest",
				time.Now(),
				[]string{"/app:app"},
				nil,
			)

			excluded.SetStale(true)
			serviceMatch.SetStale(true)
			containers := []types.Container{excluded, serviceMatch, dependent}

			graph, err := sorter.NewDependencyGraph(log, containers, nil, false)
			require.NoError(t, err)

			got := reconcileImplicitRestartsExcluding(
				log,
				graph,
				containers,
				containers,
				types.UpdateParams{},
				map[types.ContainerID]struct{}{excluded.ID(): {}},
			)

			assert.False(t, dependent.IsLinkedToRestarting())
			assert.ElementsMatch(t, []types.Container{serviceMatch}, got)
		})
	})

	t.Run("gitImageTag", func(t *testing.T) {
		t.Parallel()

		t.Run("formats name:git-<shortsha>", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name   string
				image  string
				commit string
				want   string
			}{
				{name: "docker official image", image: "nginx:latest", commit: "0123456789abcdef0123456789abcdef01234567", want: "docker.io/library/nginx:git-0123456789ab"},
				{name: "already normalized name", image: "docker.io/library/myapp:latest", commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: "docker.io/library/myapp:git-aaaaaaaaaaaa"},
				{name: "short commit is kept", image: "myapp:latest", commit: "abc", want: "docker.io/library/myapp:git-abc"},
				{name: "empty commit", image: "myapp:latest", commit: "", want: "docker.io/library/myapp:git-"},
				{name: "unparseable name strips a tag suffix", image: "not a ref:dev", commit: "0123456789abcdef", want: "not a ref:git-0123456789ab"},
				{name: "registry with port is not stripped at the port colon", image: "localhost:5000/app", commit: "0123456789abcdef0123", want: "localhost:5000/app:git-0123456789ab"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					image, commit, want := tt.image, tt.commit, tt.want

					assert.Equal(t, want, gitImageTag(image, commit))
				})
			}
		})

		t.Run("validDockerTag", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name  string
				image string
				want  bool
			}{
				{name: "tagged image", image: "myapp:latest", want: true},
				{name: "bare image", image: "myapp", want: true},
				{name: "digest image", image: "myapp@sha256:" + strings.Repeat("a", 64), want: false},
				{name: "tagged digest image", image: "myapp:latest@sha256:" + strings.Repeat("a", 64), want: false},
				{name: "invalid image", image: "not a ref:dev", want: false},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					image, want := tt.image, tt.want

					assert.Equal(t, want, validDockerTag(image))
				})
			}
		})

		t.Run("notifies invalid git config without the raw host", func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			log := zerolog.New(&buf).Level(zerolog.WarnLevel)
			warnGitConfigSkip(
				&log,
				"app",
				"app:latest",
				fmt.Errorf("%w: %w", git.ErrInvalidHost, errors.New("https://user:supersecret@evil.example")),
			)
			warnGitConfigSkip(&log, "app", "app:latest", fmt.Errorf("%w: nightly", git.ErrInvalidPolicy))

			text := buf.String()
			assert.Contains(t, text, "Skipped container with an invalid git-host")
			assert.Contains(t, text, "Skipped container with an invalid git semver policy")
			assert.NotContains(t, text, "supersecret")
			assert.Equal(t, "7ab67c773bf4", shortGitSHA("7ab67c773bf4bf670a97bc7b03a032a7fbd80892"))
			assert.Equal(t, "git.example.com/org/watchtower-git-support-test", gitNoticePath("https://git.example.com/org/watchtower-git-support-test.git"))
			assert.Equal(t, "git.example.com/org/watchtower-git-support-test", gitNoticePath("git@git.example.com:org/watchtower-git-support-test.git"))
			assert.Equal(t, "wtgit-semver:git-7ab67c773bf4", familiarImage("docker.io/library/wtgit-semver:git-7ab67c773bf4"))
		})
	})
}

// Test_withGitMeta verifies that a log event a disabled level discards, which
// zerolog represents as nil, stays nil instead of gaining Git fields.
func Test_withGitMeta(t *testing.T) {
	t.Parallel()

	c := gitAssociatedContainer("id", "/app", "myapp:latest", nil)

	assert.Nil(t, withGitMeta(nil, c, types.UpdateParams{EnableGitMonitoring: true}, "v1.0.0", "0123456789abcdef"))
}
