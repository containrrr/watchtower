package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceEqual_True(t *testing.T) {
	s1 := []string{"a", "b", "c"}
	s2 := []string{"a", "b", "c"}

	result := sliceEqual(s1, s2)

	assert.True(t, result)
}

func TestSliceEqual_DifferentLengths(t *testing.T) {
	s1 := []string{"a", "b", "c"}
	s2 := []string{"a", "b", "c", "d"}

	result := sliceEqual(s1, s2)

	assert.False(t, result)
}

func TestSliceEqual_DifferentContents(t *testing.T) {
	s1 := []string{"a", "b", "c"}
	s2 := []string{"a", "b", "d"}

	result := sliceEqual(s1, s2)

	assert.False(t, result)
}

func TestSliceSubtract(t *testing.T) {
	a1 := []string{"a", "b", "c"}
	a2 := []string{"a", "c"}

	result := sliceSubtract(a1, a2)
	assert.Equal(t, []string{"b"}, result)
	assert.Equal(t, []string{"a", "b", "c"}, a1)
	assert.Equal(t, []string{"a", "c"}, a2)
}

func TestStringMapSubtract(t *testing.T) {
	m1 := map[string]string{"a": "a", "b": "b", "c": "sea"}
	m2 := map[string]string{"a": "a", "c": "c"}

	result := stringMapSubtract(m1, m2)
	assert.Equal(t, map[string]string{"b": "b", "c": "sea"}, result)
	assert.Equal(t, map[string]string{"a": "a", "b": "b", "c": "sea"}, m1)
	assert.Equal(t, map[string]string{"a": "a", "c": "c"}, m2)
}

func TestStructMapSubtract(t *testing.T) {
	x := struct{}{}
	m1 := map[string]struct{}{"a": x, "b": x, "c": x}
	m2 := map[string]struct{}{"a": x, "c": x}

	result := structMapSubtract(m1, m2)
	assert.Equal(t, map[string]struct{}{"b": x}, result)
	assert.Equal(t, map[string]struct{}{"a": x, "b": x, "c": x}, m1)
	assert.Equal(t, map[string]struct{}{"a": x, "c": x}, m2)
}
