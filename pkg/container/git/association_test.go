package git

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
	typemocks "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func TestResolveAssociation(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		got, ok := ResolveAssociation(nil, types.UpdateParams{})
		assert.False(t, ok)
		assert.Equal(t, Association{}, got)
	})

	t.Run("repo label wins over mapping", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			RepoLabel:       "https://github.com/org/from-label.git",
			RefLabel:        "release",
			DockerfileLabel: "build/Dockerfile",
			ContextLabel:    "src",
		}, "myapp:latest")

		got, ok := ResolveAssociation(c, types.UpdateParams{
			GitDefaultRef:   "main",
			GitSemverPolicy: types.GitPolicyPatch,
			GitDockerfile:   "Dockerfile",
			GitContext:      ".",
			GitImages: map[string]types.GitImage{
				"myapp:latest": {Repo: "https://github.com/org/from-map.git", Ref: "mapped"},
			},
		})
		require.True(t, ok)
		assert.Equal(t, Association{
			Repo:              "https://github.com/org/from-label.git",
			Ref:               "release",
			Policy:            types.GitPolicyPatch,
			Dockerfile:        "build/Dockerfile",
			Context:           "src",
			PolicyFromDefault: true,
		}, got)
	})

	t.Run("git-host label is kept on the association", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			RepoLabel: "git@git.example.com:org/app.git",
			HostLabel: "https://git.example.com:3000",
		}, "myapp:latest")

		got, ok := ResolveAssociation(c, types.UpdateParams{})
		require.True(t, ok)
		assert.Equal(t, "https://git.example.com:3000", got.Host)
	})

	t.Run("label defaults for ref policy and paths", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			RepoLabel: "https://github.com/org/app.git",
		}, "myapp:latest")

		got, ok := ResolveAssociation(c, types.UpdateParams{})
		require.True(t, ok)
		assert.Equal(t, "main", got.Ref)
		assert.Equal(t, types.GitPolicyNone, got.Policy)
		assert.Empty(t, got.Dockerfile)
		assert.Empty(t, got.Context)
	})

	t.Run("git-branch alias", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			RepoLabel:   "https://github.com/org/app.git",
			branchLabel: "develop",
		}, "myapp:latest")

		got, ok := ResolveAssociation(c, types.UpdateParams{})
		require.True(t, ok)
		assert.Equal(t, "develop", got.Ref)
	})

	t.Run("image mapping", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			DockerfileLabel: "label.Dockerfile",
		}, "myapp:latest")

		got, ok := ResolveAssociation(c, types.UpdateParams{
			GitImages: map[string]types.GitImage{
				"myapp:latest": {
					Repo:       "https://github.com/org/mapped.git",
					Ref:        "v1",
					Policy:     types.GitPolicyMinor,
					Dockerfile: "map.Dockerfile",
					Context:    "app",
				},
			},
		})
		require.True(t, ok)
		assert.Equal(t, Association{
			Repo:       "https://github.com/org/mapped.git",
			Ref:        "v1",
			Policy:     types.GitPolicyMinor,
			Dockerfile: "label.Dockerfile",
			Context:    "app",
		}, got)
	})

	t.Run("mapping falls back to process defaults", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, nil, "myapp:latest")
		got, ok := ResolveAssociation(c, types.UpdateParams{
			GitDefaultRef:   "trunk",
			GitSemverPolicy: types.GitPolicyMajor,
			GitDockerfile:   "ci/Dockerfile",
			GitContext:      "docker",
			GitImages: map[string]types.GitImage{
				"myapp:latest": {Repo: "https://github.com/org/mapped.git"},
			},
		})
		require.True(t, ok)
		assert.Equal(t, Association{
			Repo:                  "https://github.com/org/mapped.git",
			Ref:                   "trunk",
			Policy:                types.GitPolicyMajor,
			Dockerfile:            "ci/Dockerfile",
			Context:               "docker",
			RefFromDefault:        true,
			PolicyFromDefault:     true,
			DockerfileFromDefault: true,
			ContextFromDefault:    true,
		}, got)
	})

	t.Run("unassociated", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, nil, "nginx:latest")
		got, ok := ResolveAssociation(c, types.UpdateParams{})
		assert.False(t, ok)
		assert.Equal(t, Association{}, got)
	})
}

func TestShouldMonitor(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		assert.False(t, ShouldMonitor(nil, nil, types.UpdateParams{EnableGitMonitoring: true}))
	})

	t.Run("associated and process watch", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{RepoLabel: "https://github.com/org/app.git"}, "myapp:latest")
		assert.True(t, ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true}))
	})

	t.Run("associated but watch off", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			RepoLabel:  "https://github.com/org/app.git",
			WatchLabel: "false",
		}, "myapp:latest")
		assert.False(t, ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true}))
	})

	t.Run("watchtower is excluded", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().IsWatchtower().Return(true)
		assert.False(t, ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true}))
	})

	t.Run("watch without association", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{WatchLabel: "true"}, "nginx:latest")
		nop := zerolog.Nop()
		assert.False(t, ShouldMonitor(&nop, c, types.UpdateParams{}))
		assert.False(t, ShouldMonitor(nil, c, types.UpdateParams{}))
	})
}

func TestLabel(t *testing.T) {
	t.Parallel()

	assert.Empty(t, label(nil, RepoLabel))
	assert.Empty(t, label(testContainer(t, nil, "app:latest"), RepoLabel))
	assert.Equal(t, "https://github.com/org/app.git", label(testContainer(t, map[string]string{
		RepoLabel: "  https://github.com/org/app.git  ",
	}, "app:latest"), RepoLabel))
}

func TestRefLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "release", refLabel(testContainer(t, map[string]string{
		RefLabel:    "release",
		branchLabel: "ignored",
	}, "app:latest")))
	assert.Equal(t, "develop", refLabel(testContainer(t, map[string]string{
		branchLabel: "develop",
	}, "app:latest")))
	assert.Empty(t, refLabel(testContainer(t, nil, "app:latest")))
}

func TestPolicyLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.GitPolicyPatch, policyLabel(testContainer(t, map[string]string{
		SemverPolicyLabel: "Patch",
	}, "app:latest")))
	assert.Empty(t, policyLabel(testContainer(t, map[string]string{
		SemverPolicyLabel: "nightly",
	}, "app:latest")))
	assert.Empty(t, policyLabel(testContainer(t, nil, "app:latest")))
}

func TestImageMapping(t *testing.T) {
	t.Parallel()

	images := map[string]types.GitImage{
		"myapp:latest": {Repo: "https://github.com/org/app.git"},
	}

	assert.Equal(t, images["myapp:latest"], imageMapping("myapp:latest", images))
	assert.Equal(t, types.GitImage{}, imageMapping("other:latest", images))
	assert.Equal(t, types.GitImage{}, imageMapping("myapp:latest", nil))
	assert.Equal(t, types.GitImage{}, imageMapping("", images))
}

func TestPersistWatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		label   string
		present bool
		want    bool
	}{
		{name: "absent"},
		{name: "blank", label: "  ", present: true},
		{name: "true", label: "true", present: true, want: true},
		{name: "yes", label: "YES", present: true, want: true},
		{name: "one", label: "1", present: true, want: true},
		{name: "false", label: "false", present: true},
		{name: "unknown", label: "maybe", present: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			labels := map[string]string{}
			if tt.present {
				labels[WatchLabel] = tt.label
			}

			assert.Equal(t, tt.want, PersistWatch(testContainer(t, labels, "app:latest")))
		})
	}

	assert.False(t, PersistWatch(nil))
}

func TestIsWatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		label   string
		present bool
		process bool
		want    bool
	}{
		{name: "missing uses process true", process: true, want: true},
		{name: "missing uses process false"},
		{name: "blank uses process", label: "  ", present: true, process: true, want: true},
		{name: "true", label: "true", present: true, want: true},
		{name: "yes", label: "YES", present: true, want: true},
		{name: "one", label: "1", present: true, want: true},
		{name: "t", label: "t", present: true, want: true},
		{name: "false", label: "false", present: true, process: true},
		{name: "no", label: "no", present: true, process: true},
		{name: "zero", label: "0", present: true, process: true},
		{name: "f", label: "f", present: true, process: true},
		{name: "unknown uses process", label: "maybe", present: true, process: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			labels := map[string]string{}
			if tt.present {
				labels[WatchLabel] = tt.label
			}

			got := isWatch(testContainer(t, labels, "app:latest"), types.UpdateParams{EnableGitMonitoring: tt.process})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultRef(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "main", defaultRef(types.UpdateParams{}))
	assert.Equal(t, "develop", defaultRef(types.UpdateParams{GitDefaultRef: "develop"}))
}

func TestDefaultPolicy(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.GitPolicyNone, defaultPolicy(types.UpdateParams{}))
	assert.Equal(t, types.GitPolicyMinor, defaultPolicy(types.UpdateParams{GitSemverPolicy: types.GitPolicyMinor}))
	assert.Equal(t, types.GitPolicyNone, defaultPolicy(types.UpdateParams{GitSemverPolicy: "nightly"}))
}

// testContainer returns a container whose labels and image name are fixed.
//
// Parameters:
//   - t: Test handle.
//   - labels: Label key to value map. Missing keys are treated as unset.
//   - imageName: ImageName() result.
//
// Returns:
//   - types.Container: Mock container.
func testContainer(t *testing.T, labels map[string]string, imageName string) types.Container {
	t.Helper()

	c := typemocks.NewMockContainer(t)
	c.EXPECT().GetLabel(mock.Anything).RunAndReturn(func(key string) (string, bool) {
		value, ok := labels[key]

		return value, ok
	}).Maybe()
	c.EXPECT().ImageName().Return(imageName).Maybe()
	c.EXPECT().HasImageInfo().Return(false).Maybe()
	c.EXPECT().Name().Return("app").Maybe()
	c.EXPECT().IsWatchtower().Return(false).Maybe()

	return c
}
