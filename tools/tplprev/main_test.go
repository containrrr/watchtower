//go:build !wasm

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTemplateBuiltin(t *testing.T) {
	t.Parallel()

	got, err := resolveTemplate("default")
	require.NoError(t, err)
	assert.Contains(t, got, ".Report")
}

func TestResolveTemplateFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "custom.tpl")
	require.NoError(t, os.WriteFile(path, []byte("hello {{.Title}}"), 0o600))

	got, err := resolveTemplate(path)
	require.NoError(t, err)
	assert.Equal(t, "hello {{.Title}}", got)
}

func TestResolveTemplateMissingFile(t *testing.T) {
	t.Parallel()

	_, err := resolveTemplate(filepath.Join(t.TempDir(), "missing.tpl"))
	require.Error(t, err)
}
