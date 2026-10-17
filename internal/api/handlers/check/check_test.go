package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gofiber/fiber/v3"
	"github.com/moby/moby/api/types/image"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	goGit "github.com/go-git/go-git/v5"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/git"
	wtcontainer "github.com/nicholas-fedor/watchtower/pkg/container"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// testLogger returns a silent logger for the whole test package.
func testLogger() *zerolog.Logger { return new(zerolog.Nop()) }

func TestCheckForUpdates(t *testing.T) {
	tests := []struct {
		name      string
		client    func(t *testing.T) *mockContainer.MockClient
		filter    types.Filter
		wantErr   bool
		errMsg    string
		wantLen   int
		wantStale []bool
		wantNames []string
	}{
		{
			name: "single container with update available",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("my-app")
				container.EXPECT().ImageName().Return("nginx:1.25")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				c.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
					Return(true, types.ImageID("sha256:def"), "", nil)

				return c
			},
			wantLen:   1,
			wantStale: []bool{true},
		},
		{
			name: "single container no update available",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("my-app")
				container.EXPECT().ImageName().Return("nginx:1.25")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				c.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
					Return(false, types.ImageID("sha256:abc"), "", nil)

				return c
			},
			wantLen:   1,
			wantStale: []bool{false},
		},
		{
			name: "filter by image name excludes non-matching",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container1 := mockTypes.NewMockContainer(t)
				container1.EXPECT().Name().Return("app1").Maybe()
				container1.EXPECT().ImageName().Return("nginx:latest").Maybe()
				container1.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
				container1.EXPECT().ImageInfo().Return(nil).Maybe()

				container2 := mockTypes.NewMockContainer(t)
				container2.EXPECT().Name().Return("app2").Maybe()
				container2.EXPECT().ImageName().Return("redis:latest").Maybe()
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container1, container2}, nil)
				c.EXPECT().CheckContainerUpdate(mock.Anything, container1, mock.Anything).
					Return(false, types.ImageID("sha256:abc"), "", nil)

				return c
			},
			filter: func(c types.FilterableContainer) bool {
				return c.ImageName() == "nginx:latest"
			},
			wantLen:   1,
			wantNames: []string{"app1"},
		},
		{
			name: "filter by container name excludes non-matching",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container1 := mockTypes.NewMockContainer(t)
				container1.EXPECT().Name().Return("app1").Maybe()
				container1.EXPECT().ImageName().Return("nginx:latest").Maybe()
				container1.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
				container1.EXPECT().ImageInfo().Return(nil).Maybe()

				container2 := mockTypes.NewMockContainer(t)
				container2.EXPECT().Name().Return("app2").Maybe()
				container2.EXPECT().ImageName().Return("redis:latest").Maybe()
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container1, container2}, nil)
				c.EXPECT().CheckContainerUpdate(mock.Anything, container1, mock.Anything).
					Return(false, types.ImageID("sha256:abc"), "", nil)

				return c
			},
			filter: func(c types.FilterableContainer) bool {
				return c.Name() == "app1"
			},
			wantLen:   1,
			wantNames: []string{"app1"},
		},
		{
			name: "both filters ANDed require both to match",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container1 := mockTypes.NewMockContainer(t)
				container1.EXPECT().Name().Return("app1").Maybe()
				container1.EXPECT().ImageName().Return("nginx:latest").Maybe()
				container1.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
				container1.EXPECT().ImageInfo().Return(nil).Maybe()

				container2 := mockTypes.NewMockContainer(t)
				container2.EXPECT().Name().Return("app2").Maybe()
				container2.EXPECT().ImageName().Return("nginx:latest").Maybe()
				container2.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
				container2.EXPECT().ImageInfo().Return(nil).Maybe()

				container3 := mockTypes.NewMockContainer(t)
				container3.EXPECT().Name().Return("app3").Maybe()
				container3.EXPECT().ImageName().Return("redis:latest").Maybe()
				container3.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
				container3.EXPECT().ImageInfo().Return(nil).Maybe()
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container1, container2, container3}, nil)
				c.EXPECT().CheckContainerUpdate(mock.Anything, container1, mock.Anything).
					Return(false, types.ImageID("sha256:abc"), "", nil)

				return c
			},
			filter: func(c types.FilterableContainer) bool {
				return c.ImageName() == "nginx:latest" && c.Name() == "app1"
			},
			wantLen:   1,
			wantNames: []string{"app1"},
		},
		{
			name: "empty container list",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil)

				return c
			},
			wantLen: 0,
		},
		{
			name: "list error returns wrapped error",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(nil, errors.New("connection refused"))

				return c
			},
			wantErr: true,
			errMsg:  "failed to list containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client(t)
			results, err := CheckForUpdates(testLogger(), t.Context(), client, tt.filter, types.UpdateParams{}, nil)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, results, tt.wantLen)

				if tt.wantStale != nil {
					for i, wantStale := range tt.wantStale {
						if i < len(results) {
							assert.Equal(t, wantStale, results[i].UpdateAvailable)
						}
					}
				}

				if tt.wantNames != nil {
					for i, wantName := range tt.wantNames {
						if i < len(results) {
							assert.Equal(t, wantName, results[i].Name)
						}
					}
				}
			}
		})
	}
}

func TestCheckForUpdates_DigestExtraction(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return("my-app")
	container.EXPECT().ImageName().Return("nginx:latest")
	container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))

	info := &image.InspectResponse{RepoDigests: []string{"nginx@sha256:digest123"}}
	container.EXPECT().ImageInfo().Return(info)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
	client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
		Return(false, types.ImageID("sha256:abc"), "", nil)

	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, types.UpdateParams{}, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sha256:digest123", results[0].Digest)
}

func TestCheckForUpdates_CheckContainerUpdateError(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return("my-app")
	container.EXPECT().ImageName().Return("nginx:latest")
	container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
	container.EXPECT().ImageInfo().Return(nil)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
	client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
		Return(false, types.ImageID(""), "", errors.New("registry unavailable"))

	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, types.UpdateParams{}, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "registry unavailable", results[0].Error)
	assert.False(t, results[0].UpdateAvailable)
}

func TestCheckForUpdates_ParamsPropagation(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return("my-app")
	container.EXPECT().ImageName().Return("nginx:latest")
	container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
	container.EXPECT().ImageInfo().Return(nil)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
	client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.MatchedBy(func(p types.UpdateParams) bool {
		return p.MonitorOnly && p.NoPull && p.LabelPrecedence && p.CooldownDelay == 5*time.Minute
	})).Return(true, types.ImageID("sha256:new"), "sha256:newdigest", nil)

	params := types.UpdateParams{
		MonitorOnly:     true,
		NoPull:          true,
		LabelPrecedence: true,
		CooldownDelay:   5 * time.Minute,
	}
	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, params, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].UpdateAvailable)
	assert.Equal(t, "sha256:new", results[0].LatestImageID)
	assert.Equal(t, "sha256:newdigest", results[0].LatestDigest)
}

func TestCheckForUpdates_MixedStaleResults(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	container1 := mockTypes.NewMockContainer(t)
	container1.EXPECT().Name().Return("app1")
	container1.EXPECT().ImageName().Return("nginx:latest")
	container1.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
	container1.EXPECT().ImageInfo().Return(nil)

	container2 := mockTypes.NewMockContainer(t)
	container2.EXPECT().Name().Return("app2")
	container2.EXPECT().ImageName().Return("redis:latest")
	container2.EXPECT().ImageID().Return(types.ImageID("sha256:def"))
	container2.EXPECT().ImageInfo().Return(nil)

	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container1, container2}, nil)
	client.EXPECT().CheckContainerUpdate(mock.Anything, container1, mock.Anything).
		Return(true, types.ImageID("sha256:new1"), "sha256:digest1", nil)
	client.EXPECT().CheckContainerUpdate(mock.Anything, container2, mock.Anything).
		Return(false, types.ImageID("sha256:def"), "", nil)

	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, types.UpdateParams{}, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].UpdateAvailable)
	assert.Equal(t, "sha256:new1", results[0].LatestImageID)
	assert.Equal(t, "sha256:digest1", results[0].LatestDigest)
	assert.False(t, results[1].UpdateAvailable)
	assert.Equal(t, "sha256:def", results[1].LatestImageID)
	assert.Empty(t, results[1].LatestDigest)
}

func TestCheckForUpdates_GitNoPullSkipsCheck(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	watched := wtcontainer.NewContainer(testLogger(), &dockerContainer.InspectResponse{
		ID:   "app-id",
		Name: "/app",
		Config: &dockerContainer.Config{
			Image: "app:latest",
			Labels: map[string]string{
				gitPkg.RepoLabel: "https://git.example.com/org/app.git",
			},
		},
	}, nil)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{watched}, nil)

	gitClient := git.New(testLogger(), git.Options{Timeout: time.Millisecond})
	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, types.UpdateParams{
		EnableGitMonitoring: true,
		NoPull:              true,
	}, gitClient)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "git", results[0].UpdateSource)
	assert.False(t, results[0].UpdateAvailable)
	assert.Empty(t, results[0].Error)
	assert.Empty(t, results[0].LatestImageID)
}

// TestCheckForUpdates_GitCommitField verifies that Git results expose the commit separately from the local image ID.
func TestCheckForUpdates_GitCommitField(t *testing.T) {
	repoDir, branch, commit := initCheckGitRepo(t)
	watched := wtcontainer.NewContainer(testLogger(), &dockerContainer.InspectResponse{
		ID:   "app-id",
		Name: "/app",
		Config: &dockerContainer.Config{
			Image: "app:latest",
			Labels: map[string]string{
				gitPkg.RepoLabel: repoDir,
				gitPkg.RefLabel:  branch,
			},
		},
	}, nil)

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{watched}, nil)

	results, err := CheckForUpdates(testLogger(), t.Context(), client, nil, types.UpdateParams{
		EnableGitMonitoring: true,
	}, git.New(testLogger(), git.Options{Timeout: time.Second}))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "git", results[0].UpdateSource)
	assert.Equal(t, commit, results[0].GitCommit)
	assert.Empty(t, results[0].LatestImageID)
}

// initCheckGitRepo creates a temporary Git repository and returns its directory, branch, and commit.
func initCheckGitRepo(t *testing.T) (string, string, string) {
	t.Helper()

	dir := t.TempDir()
	repo, err := goGit.PlainInit(dir, false)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("hello\n"), 0o600))

	_, err = worktree.Add("README")
	require.NoError(t, err)

	hash, err := worktree.Commit("init", &goGit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	return dir, head.Name().Short(), hash.String()
}

// initCheckGitRepoWithTag creates a temporary Git repository with one commit, tags
// that commit, and returns the directory, branch, and commit.
func initCheckGitRepoWithTag(t *testing.T, tag string) (string, string, string) {
	t.Helper()

	dir, branch, commit := initCheckGitRepo(t)

	repo, err := goGit.PlainOpen(dir)
	require.NoError(t, err)

	_, err = repo.CreateTag(tag, plumbing.NewHash(commit), nil)
	require.NoError(t, err)

	return dir, branch, commit
}

func TestExtractFilterParams(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		key       string
		wantEmpty bool
		wantVals  []string
	}{
		{
			name:      "no params",
			query:     "",
			key:       "name",
			wantEmpty: true,
		},
		{
			name:     "single value",
			query:    "?name=my-container",
			key:      "name",
			wantVals: []string{"my-container"},
		},
		{
			name:     "comma-separated values",
			query:    "?name=container1,container2",
			key:      "name",
			wantVals: []string{"container1", "container2"},
		},
		{
			name:     "multiple params",
			query:    "?name=app1&name=app2",
			key:      "name",
			wantVals: []string{"app1", "app2"},
		},
		{
			name:      "empty value filtered out",
			query:     "?name=",
			key:       "name",
			wantEmpty: true,
		},
		{
			name:     "whitespace trimmed",
			query:    "?name=%20app1%20&name=app2",
			key:      "name",
			wantVals: []string{"app1", "app2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{})
			app.Get("/test", func(c fiber.Ctx) error {
				vals := extractFilterParams(c, tt.key)
				if tt.wantEmpty {
					assert.Empty(t, vals)
				} else {
					assert.Equal(t, tt.wantVals, vals)
				}

				return c.SendStatus(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test"+tt.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// stubContainer returns a mock container reporting a fixed identity.
func stubContainer(t *testing.T, info *image.InspectResponse) *mockTypes.MockContainer {
	t.Helper()

	c := mockTypes.NewMockContainer(t)
	c.EXPECT().Name().Return("/app").Maybe()
	c.EXPECT().ImageName().Return("app:latest").Maybe()
	c.EXPECT().ImageID().Return(types.ImageID("sha256:abc")).Maybe()
	c.EXPECT().ImageInfo().Return(info).Maybe()

	return c
}

// gitContainer returns a concrete container associated with a Git repository.
func gitContainer(t *testing.T, repo string, imageLabels map[string]string) *wtcontainer.Container {
	t.Helper()

	imageConfig := &dockerspec.DockerOCIImageConfig{}
	imageConfig.Labels = imageLabels

	return wtcontainer.NewContainer(testLogger(), &dockerContainer.InspectResponse{
		ID:   "app-id",
		Name: "/app",
		Config: &dockerContainer.Config{
			Image: "app:latest",
			Labels: map[string]string{
				gitPkg.RepoLabel: repo,
			},
		},
	}, &image.InspectResponse{
		ID:     "sha256:image",
		Config: imageConfig,
	})
}

func TestNewContainerCheck(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	c := stubContainer(t, nil)

	got := newContainerCheck(c, now)

	assert.Equal(t, "/app", got.Name)
	assert.Equal(t, "app:latest", got.Image)
	assert.Equal(t, "sha256:abc", got.ImageID)
	assert.Equal(t, now, got.Timestamp)
	// Source and availability belong to the check helpers, not the identity.
	assert.Empty(t, got.UpdateSource)
	assert.Empty(t, got.Digest)
	assert.False(t, got.UpdateAvailable)
}

func TestApplyReportMetadata(t *testing.T) {
	t.Parallel()

	t.Run("concrete container resolves labels and OCI annotations", func(t *testing.T) {
		t.Parallel()

		c := gitContainer(t, "https://github.com/org/app.git", map[string]string{
			oci.SourceLabel:   "https://github.com/org/app.git",
			oci.VersionLabel:  "1.2.2",
			oci.RevisionLabel: "abc123",
		})

		var result ContainerCheck
		applyReportMetadata(c, types.UpdateParams{}, &result)

		assert.Equal(t, "https://github.com/org/app.git", result.GitRepo)
		assert.Equal(t, "https://github.com/org/app/releases", result.Changelog)
		assert.Equal(t, "https://github.com/org/app.git", result.OCISource)
		assert.Equal(t, "1.2.2", result.CurrentImageVersion)
		assert.Equal(t, "abc123", result.CurrentImageRevision)
		// The endpoint never pulls, so the latest* fields stay empty.
		assert.Empty(t, result.LatestImageVersion)
		assert.Empty(t, result.LatestImageRevision)
	})

	t.Run("a stubbed container exposes no metadata", func(t *testing.T) {
		t.Parallel()

		c := stubContainer(t, nil)

		result := ContainerCheck{}
		applyReportMetadata(c, types.UpdateParams{}, &result)

		assert.Empty(t, result.GitRepo)
		assert.Empty(t, result.GitRef)
		assert.Empty(t, result.Changelog)
		assert.Empty(t, result.OCISource)
		assert.Empty(t, result.ImageURL)
		assert.Empty(t, result.Documentation)
		assert.Empty(t, result.CurrentImageVersion)
		assert.Empty(t, result.CurrentImageRevision)
	})
}

// The digest selection rules, including the fallback to an unrelated repo
// digest, belong to ExtractImageDigest's own suite. This covers only the
// wrapper's nil guard and its pass-through wiring.
func TestApplyLocalDigest(t *testing.T) {
	t.Parallel()

	t.Run("nil image info leaves the digest empty", func(t *testing.T) {
		t.Parallel()

		c := stubContainer(t, nil)

		var result ContainerCheck
		applyLocalDigest(c, &result)

		assert.Empty(t, result.Digest)
	})

	t.Run("matching repo digest is extracted", func(t *testing.T) {
		t.Parallel()

		c := stubContainer(t, &image.InspectResponse{
			RepoDigests: []string{"app@sha256:1111111111111111111111111111111111111111111111111111111111111111"},
		})

		var result ContainerCheck
		applyLocalDigest(c, &result)

		assert.Equal(t, "sha256:1111111111111111111111111111111111111111111111111111111111111111", result.Digest)
	})

	t.Run("an existing digest is not cleared by a nil image info", func(t *testing.T) {
		t.Parallel()

		c := stubContainer(t, nil)

		result := ContainerCheck{Digest: "sha256:preexisting"}
		applyLocalDigest(c, &result)

		assert.Equal(t, "sha256:preexisting", result.Digest)
	})
}

func TestIsGitWatched(t *testing.T) {
	t.Parallel()

	gitClient := git.New(testLogger(), git.Options{Timeout: time.Second})

	tests := []struct {
		name      string
		container types.Container
		params    types.UpdateParams
		gitClient *git.Client
		want      bool
	}{
		{
			name:      "no git client falls back to the registry",
			container: gitContainer(t, "https://github.com/org/app.git", nil),
			params:    types.UpdateParams{EnableGitMonitoring: true},
			gitClient: nil,
			want:      false,
		},
		{
			name:      "stubbed container has no labels to watch",
			container: stubContainer(t, nil),
			params:    types.UpdateParams{EnableGitMonitoring: true},
			gitClient: gitClient,
			want:      false,
		},
		{
			name:      "git monitoring disabled",
			container: gitContainer(t, "https://github.com/org/app.git", nil),
			params:    types.UpdateParams{},
			gitClient: gitClient,
			want:      false,
		},
		{
			name:      "associated container with monitoring enabled",
			container: gitContainer(t, "https://github.com/org/app.git", nil),
			params:    types.UpdateParams{EnableGitMonitoring: true},
			gitClient: gitClient,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isGitWatched(testLogger(), tt.container, tt.params, tt.gitClient))
		})
	}
}

func TestCheckGitSource(t *testing.T) {
	t.Parallel()

	t.Run("no-pull reports the skip without querying", func(t *testing.T) {
		t.Parallel()

		repoDir, branch, _ := initCheckGitRepo(t)
		c := wtcontainer.NewContainer(testLogger(), &dockerContainer.InspectResponse{
			ID:   "app-id",
			Name: "/app",
			Config: &dockerContainer.Config{
				Image: "app:latest",
				Labels: map[string]string{
					gitPkg.RepoLabel: repoDir,
					gitPkg.RefLabel:  branch,
				},
			},
		}, nil)

		var result ContainerCheck

		checkGitSource(
			t.Context(),
			testLogger(),
			c,
			types.UpdateParams{EnableGitMonitoring: true, NoPull: true},
			git.New(testLogger(), git.Options{Timeout: time.Second}),
			&result,
		)

		assert.Equal(t, "git", result.UpdateSource)
		assert.False(t, result.UpdateAvailable)
		assert.Empty(t, result.Error)
		assert.Empty(t, result.GitCommit)
	})

	t.Run("an unreadable repository surfaces the error", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "absent")
		c := gitContainer(t, missing, nil)

		var result ContainerCheck

		checkGitSource(
			t.Context(),
			testLogger(),
			c,
			types.UpdateParams{EnableGitMonitoring: true},
			git.New(testLogger(), git.Options{Timeout: time.Second}),
			&result,
		)

		assert.Equal(t, "git", result.UpdateSource)
		assert.NotEmpty(t, result.Error)
		assert.False(t, result.UpdateAvailable)
		assert.Empty(t, result.GitCommit)
	})

	t.Run("a fresh commit reports no update", func(t *testing.T) {
		t.Parallel()

		repoDir, branch, commit := initCheckGitRepo(t)
		c := gitContainer(t, repoDir, nil)
		c.SetLabel(gitPkg.RefLabel, branch)

		var result ContainerCheck

		checkGitSource(
			t.Context(),
			testLogger(),
			c,
			types.UpdateParams{EnableGitMonitoring: true},
			git.New(testLogger(), git.Options{Timeout: time.Second}),
			&result,
		)

		assert.Equal(t, "git", result.UpdateSource)
		assert.Empty(t, result.Error)
		assert.Equal(t, commit, result.GitCommit)
	})
	t.Run("a resolved tag refines the changelog to that release", func(t *testing.T) {
		t.Parallel()

		repoDir, branch, commit := initCheckGitRepoWithTag(t, "v1.2.3")
		c := gitContainer(t, repoDir, nil)
		c.SetLabel(gitPkg.RefLabel, branch)
		// A semver policy is what makes the check resolve a tag at all.
		c.SetLabel(gitPkg.SemverPolicyLabel, string(types.GitPolicyPatch))
		// A template makes the tag substitution observable, because a local
		// repository path has no known host to derive a releases URL from.
		c.SetLabel(gitPkg.ChangelogURLLabel, "https://example.com/notes/{tag}")

		var result ContainerCheck

		checkGitSource(
			t.Context(),
			testLogger(),
			c,
			types.UpdateParams{EnableGitMonitoring: true},
			git.New(testLogger(), git.Options{Timeout: time.Second}),
			&result,
		)

		assert.Equal(t, "git", result.UpdateSource)
		assert.Equal(t, commit, result.GitCommit)
		assert.Equal(t, "https://example.com/notes/v1.2.3", result.Changelog)
	})
}

func TestCheckRegistrySource(t *testing.T) {
	t.Parallel()

	t.Run("an error is reported without update fields", func(t *testing.T) {
		t.Parallel()

		client := mockContainer.NewMockClient(t)
		client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
			Return(false, types.ImageID(""), "", errors.New("registry unreachable"))

		c := stubContainer(t, nil)

		var result ContainerCheck
		checkRegistrySource(t.Context(), testLogger(), client, c, types.UpdateParams{}, &result)

		assert.Equal(t, "registry", result.UpdateSource)
		assert.Equal(t, "registry unreachable", result.Error)
		assert.False(t, result.UpdateAvailable)
		assert.Empty(t, result.LatestDigest)
		assert.Empty(t, result.LatestImageID)
	})

	t.Run("a newer image populates the update fields", func(t *testing.T) {
		t.Parallel()

		client := mockContainer.NewMockClient(t)
		client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
			Return(true, types.ImageID("sha256:def"), "sha256:def", nil)

		c := stubContainer(t, nil)

		var result ContainerCheck
		checkRegistrySource(t.Context(), testLogger(), client, c, types.UpdateParams{}, &result)

		assert.Equal(t, "registry", result.UpdateSource)
		assert.True(t, result.UpdateAvailable)
		assert.Equal(t, "sha256:def", result.LatestImageID)
		assert.Equal(t, "sha256:def", result.LatestDigest)
		assert.Empty(t, result.Error)
	})

	t.Run("empty digest and image id are left unset", func(t *testing.T) {
		t.Parallel()

		client := mockContainer.NewMockClient(t)
		client.EXPECT().CheckContainerUpdate(mock.Anything, mock.Anything, mock.Anything).
			Return(false, types.ImageID(""), "", nil)

		c := stubContainer(t, nil)

		var result ContainerCheck
		checkRegistrySource(t.Context(), testLogger(), client, c, types.UpdateParams{}, &result)

		assert.Equal(t, "registry", result.UpdateSource)
		assert.False(t, result.UpdateAvailable)
		assert.Empty(t, result.LatestDigest)
		assert.Empty(t, result.LatestImageID)
	})
}

// logCheckFailure must stay out of the notification stream, because a failing
// container is already reported in the response body.
func TestLogCheckFailure(t *testing.T) {
	t.Parallel()

	c := stubContainer(t, nil)

	var buf bytes.Buffer

	log := zerolog.New(&buf)

	logCheckFailure(&log, c, errors.New("boom"), "Failed to check container for updates")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))

	assert.Equal(t, "debug", entry["level"])
	assert.Equal(t, "no", entry["notify"], "the entry must not raise a notification")
	assert.Equal(t, "/app", entry["container"])
	assert.Equal(t, "app:latest", entry["image"])
	assert.Equal(t, "Failed to check container for updates", entry["message"])
	assert.Equal(t, "boom", entry["error"])
}
