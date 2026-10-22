package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give      string
		wantFound bool
	}{
		{give: "default", wantFound: true},
		{give: "default-legacy", wantFound: true},
		{give: "json.v1", wantFound: true},
		{give: "porcelain.v1.summary-no-log", wantFound: true},
		{give: "porcelain.json", wantFound: true},
		{give: "missing", wantFound: false},
		{give: "", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			got, found := Lookup(tt.give)
			require.Equal(t, tt.wantFound, found)

			if tt.wantFound {
				assert.NotEmpty(t, got)
				assert.Equal(t, Templates[tt.give], got)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{
		"default",
		"default-legacy",
		"json.v1",
		"porcelain.json",
		"porcelain.v1.summary-no-log",
	}, Names())
}
