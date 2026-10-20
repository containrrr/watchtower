package compose

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzConfigFiles verifies that a Compose config_files label yields one path
// inside the project directory for each non-blank entry, and that a rejected
// label is a config file error.
//
// The seeds cover relative and nested paths, blank entries, an empty label, a
// single file, an absolute path, and a parent path.
func FuzzConfigFiles(f *testing.F) {
	f.Add("compose.yaml,nested/app.yml")
	f.Add(",  ,compose.yaml,")
	f.Add("")
	f.Add("compose.yaml")
	f.Add("/abs/only.yml")
	f.Add("../secret.yml")

	f.Fuzz(func(t *testing.T, raw string) {
		root, err := filepath.Abs("/proj")
		require.NoError(t, err)

		got, err := configFiles(map[string]string{ComposeConfigFilesLabel: raw}, "/proj")
		if err != nil {
			assert.ErrorIs(t, err, errConfigFile)

			return
		}

		entries := 0

		for part := range strings.SplitSeq(raw, ",") {
			if strings.TrimSpace(part) != "" {
				entries++
			}
		}

		assert.Len(t, got, entries)

		for _, path := range got {
			assert.NotEmpty(t, path)
			rel, relErr := filepath.Rel(root, path)
			require.NoError(t, relErr)
			assert.True(t, filepath.IsLocal(rel), path)
		}
	})
}
