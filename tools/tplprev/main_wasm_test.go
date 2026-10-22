//go:build wasm

package main

import (
	"syscall/js"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatesFromJSPlainObject(t *testing.T) {
	t.Parallel()

	obj := js.Global().Get("Object").New()

	got, err := statesFromJS(obj)
	require.ErrorIs(t, err, errInvalidJSArg)
	assert.Nil(t, got)
}

func TestLevelsFromJSPlainObject(t *testing.T) {
	t.Parallel()

	obj := js.Global().Get("Object").New()

	got, err := levelsFromJS(obj)
	require.ErrorIs(t, err, errInvalidJSArg)
	assert.Nil(t, got)
}
