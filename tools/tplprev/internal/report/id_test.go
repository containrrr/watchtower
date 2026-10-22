package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageIDShortID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want string
	}{
		{give: "sha256:0123456789abcdef0123456789abcdef", want: "0123456789ab"},
		{give: "0123456789abcdef0123456789abcdef", want: "0123456789ab"},
		{give: "md5:0123456789abcdef", want: "md5:0123456789ab"},
		{give: "sha256:short", want: "short"},
		{give: "sha256:", want: ""},
		{give: "short", want: "short"},
		{give: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ImageID(tt.give).ShortID())
			assert.Equal(t, tt.want, ContainerID(tt.give).ShortID())
		})
	}
}
