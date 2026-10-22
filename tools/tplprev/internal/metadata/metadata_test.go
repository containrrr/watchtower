package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "dev (commit none, built unknown)", String())
}
