package meta

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pkgMarker is a type declared in this package, so its PkgPath is the
// package's import path.
type pkgMarker struct{}

// ldflagsTarget matches the target of an -X linker flag, such as
// "example.com/pkg.Var" in "-X example.com/pkg.Var=value".
var ldflagsTarget = regexp.MustCompile(`-X\s+([^\s=]+)=`)

// TestVersionLdflagsTargets verifies that every -X linker flag in the release
// and image build configurations sets this package's Version variable.
//
// The linker ignores an -X flag whose target does not exist, so a moved
// package, a renamed variable, or a typo would otherwise ship binaries that
// report the fallback version. Every configuration that builds the binary
// must also inject the version, so a build that drops the flag fails too.
// Dockerfiles that only package a binary built elsewhere are not checked.
func TestVersionLdflagsTargets(t *testing.T) {
	t.Parallel()

	// Referencing Version stops this test from compiling if it is renamed.
	_ = Version

	want := reflect.TypeFor[pkgMarker]().PkgPath() + ".Version"
	root := moduleRoot(t)

	var configs []string

	for _, pattern := range []string{"build/goreleaser/*.yaml", "build/docker/Dockerfile*"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		require.NoError(t, err)
		require.NotEmpty(t, matches, "no build configuration matches %s", pattern)

		configs = append(configs, matches...)
	}

	for _, config := range configs {
		name, err := filepath.Rel(root, config)
		require.NoError(t, err)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(config)
			require.NoError(t, err)

			matches := ldflagsTarget.FindAllStringSubmatch(string(content), -1)
			if !buildsBinary(string(content)) {
				assert.Empty(t, matches, "a configuration that does not build the binary sets linker flags")

				return
			}

			require.NotEmpty(t, matches, "the configuration builds the binary without injecting the version")

			for _, match := range matches {
				assert.Equal(t, want, match[1], "the linker ignores -X flags for variables that do not exist")
			}
		})
	}
}

// buildsBinary reports whether a build configuration compiles the binary,
// rather than packaging one built elsewhere.
//
// Parameters:
//   - content: The configuration file's content.
//
// Returns:
//   - bool: True for a goreleaser configuration or a Dockerfile that runs go build.
func buildsBinary(content string) bool {
	return strings.Contains(content, "go build") || strings.Contains(content, "\nbuilds:")
}

// moduleRoot returns the directory holding the module's go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found above the test directory")

		dir = parent
	}
}
