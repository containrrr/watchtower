package compose

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzConfigFiles verifies Compose config_files labels never panic and that
// accepted paths stay inside the project directory.
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

		for _, path := range got {
			assert.NotEmpty(t, path)
			rel, relErr := filepath.Rel(root, path)
			require.NoError(t, relErr)
			assert.True(t, filepath.IsLocal(rel), path)
		}
	})
}
