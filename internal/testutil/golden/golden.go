// Package golden compares test output against golden files stored under
// testdata/.
package golden

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateEnv is the environment variable that switches Assert into update mode.
const updateEnv = "UPDATE_GOLDEN"

// goldenFileMode is the permission used when writing golden files.
const goldenFileMode = 0o644

// goldenDirMode is the permission used when creating the testdata directory.
const goldenDirMode = 0o755

// Assert compares got with testdata/<name>.golden in the calling package.
//
// When UPDATE_GOLDEN=1, the golden file is written from got instead, unless the
// test has already failed.
//
// Parameters:
//   - t: Test handle.
//   - name: Golden file name without the .golden extension.
//   - got: Output under test.
func Assert(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")

	if os.Getenv(updateEnv) == "1" {
		// A test that already failed may have produced partial output, so it
		// must not overwrite the golden file.
		if t.Failed() {
			t.Fatalf("not updating %s because the test has already failed", path)
		}

		require.NoError(t, os.MkdirAll(filepath.Dir(path), goldenDirMode))
		require.NoError(t, os.WriteFile(path, got, goldenFileMode))

		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file; run with %s=1 to create it", updateEnv)

	assert.Equal(t, string(want), string(got),
		"output differs from %s; if the change is intended, run with %s=1 and review the diff",
		path, updateEnv)
}
