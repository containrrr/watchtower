package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestParseImageMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		image   string
		mapping types.GitImage
		wantErr error
	}{
		{
			name:  "repo only uses defaults",
			raw:   "myapp:latest=https://github.com/org/app.git",
			image: "myapp:latest",
			mapping: types.GitImage{
				Repo:   "https://github.com/org/app.git",
				Ref:    DefaultRef,
				Policy: DefaultPolicy,
			},
		},
		{
			name:  "ref and policy",
			raw:   "myapp:latest=https://github.com/org/app.git#release@minor",
			image: "myapp:latest",
			mapping: types.GitImage{
				Repo:   "https://github.com/org/app.git",
				Ref:    "release",
				Policy: types.GitPolicyMinor,
			},
		},
		{
			name:  "empty hash ref falls back to default",
			raw:   "app:v1=https://git.example.com/org/app.git#",
			image: "app:v1",
			mapping: types.GitImage{
				Repo:   "https://git.example.com/org/app.git",
				Ref:    DefaultRef,
				Policy: DefaultPolicy,
			},
		},
		{
			name:  "ssh userinfo is not a policy",
			raw:   "app:latest=git@github.com:org/app.git",
			image: "app:latest",
			mapping: types.GitImage{
				Repo:   "git@github.com:org/app.git",
				Ref:    DefaultRef,
				Policy: DefaultPolicy,
			},
		},
		{
			name:  "ssh repo with ref and policy",
			raw:   "app:latest=git@github.com:org/app.git#main@patch",
			image: "app:latest",
			mapping: types.GitImage{
				Repo:   "git@github.com:org/app.git",
				Ref:    "main",
				Policy: types.GitPolicyPatch,
			},
		},
		{
			name:    "empty",
			raw:     "  ",
			wantErr: ErrInvalidGitImage,
		},
		{
			name:    "missing equals",
			raw:     "myapp:latest",
			wantErr: ErrInvalidGitImage,
		},
		{
			name:    "empty image",
			raw:     "=https://github.com/org/app.git",
			wantErr: ErrInvalidGitImage,
		},
		{
			name:    "empty repo",
			raw:     "myapp:latest=",
			wantErr: ErrInvalidGitImage,
		},
		{
			name:    "empty repo after stripping ref",
			raw:     "myapp:latest=#main",
			wantErr: ErrInvalidGitImage,
		},
		{
			name:    "unknown policy token",
			raw:     "myapp:latest=https://github.com/org/app.git@latest",
			wantErr: errInvalidGitPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			image, mapping, err := parseImageMapping(tt.raw)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.image, image)
			assert.Equal(t, tt.mapping, mapping)
		})
	}
}

func TestParseImageMappings(t *testing.T) {
	t.Parallel()

	t.Run("nil and empty", func(t *testing.T) {
		t.Parallel()

		got, err := ParseImageMappings(nil)
		require.NoError(t, err)
		assert.Empty(t, got)

		got, err = ParseImageMappings([]string{})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("skips blanks and last mapping wins", func(t *testing.T) {
		t.Parallel()

		got, err := ParseImageMappings([]string{
			"",
			" myapp:latest=https://github.com/org/app.git#main ",
			"other:v1=https://gitlab.com/org/other.git@major",
			"myapp:latest=https://github.com/org/app.git#dev@patch",
		})
		require.NoError(t, err)
		assert.Equal(t, types.GitImage{
			Repo:   "https://github.com/org/app.git",
			Ref:    "dev",
			Policy: types.GitPolicyPatch,
		}, got["myapp:latest"])
		assert.Equal(t, types.GitImage{
			Repo:   "https://gitlab.com/org/other.git",
			Ref:    DefaultRef,
			Policy: types.GitPolicyMajor,
		}, got["other:v1"])
	})

	t.Run("invalid value", func(t *testing.T) {
		t.Parallel()

		_, err := ParseImageMappings([]string{"myapp:latest=https://github.com/org/app.git", "bad"})
		require.ErrorIs(t, err, ErrInvalidGitImage)
	})
}

func TestIsPolicyToken(t *testing.T) {
	t.Parallel()

	assert.False(t, isPolicyToken(""))
	assert.True(t, isPolicyToken("latest"))
	assert.True(t, isPolicyToken("minor"))
	assert.True(t, isPolicyToken("github.com"))
	assert.False(t, isPolicyToken("org/app.git"))
	assert.False(t, isPolicyToken("user:pass"))
	assert.False(t, isPolicyToken("git@host"))
}

func TestParseComposeProjects(t *testing.T) {
	t.Parallel()

	t.Run("empty and blanks", func(t *testing.T) {
		t.Parallel()

		got, err := ParseComposeProjects(nil)
		require.NoError(t, err)
		assert.Empty(t, got)

		got, err = ParseComposeProjects([]string{"", "  "})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("parses name equals path", func(t *testing.T) {
		t.Parallel()

		got, err := ParseComposeProjects([]string{
			"webstack=/srv/webstack",
			" other = /data/other ",
		})
		require.NoError(t, err)
		assert.Equal(t, "/srv/webstack", got["webstack"])
		assert.Equal(t, "/data/other", got["other"])
	})

	t.Run("last value wins", func(t *testing.T) {
		t.Parallel()

		got, err := ParseComposeProjects([]string{
			"webstack=/first",
			"webstack=/second",
		})
		require.NoError(t, err)
		assert.Equal(t, "/second", got["webstack"])
	})

	t.Run("invalid values", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{"nopair", "= /path", "name=", "name = "} {
			_, err := ParseComposeProjects([]string{raw})
			require.ErrorIs(t, err, ErrInvalidComposeProject, raw)
		}
	})
}
