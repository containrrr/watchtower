package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImageID_ShortID verifies that an image ID is shortened to 12 characters
// without its sha256 prefix, that a short ID is returned as it is, and that an
// ID with another prefix keeps it.
func TestImageID_ShortID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   ImageID
		want string
	}{
		{
			name: "normal ID with a sha256 prefix is shortened",
			id:   "sha256:0123456789abcd00000000001111111111222222222233333333334444444444",
			want: "0123456789ab",
		},
		{
			name: "normal ID without a prefix is shortened",
			id:   "0123456789abcd00000000001111111111222222222233333333334444444444",
			want: "0123456789ab",
		},
		{name: "short ID without a prefix is returned as it is", id: "0123456789ab", want: "0123456789ab"},
		{name: "short ID with a sha256 prefix loses the prefix", id: "sha256:0123456789ab", want: "0123456789ab"},
		{name: "unknown prefix is kept", id: "md5:0123456789ab", want: "md5:0123456789ab"},
		{name: "unknown prefix is kept when the ID is shortened", id: "md5:0123456789abcdefg", want: "md5:0123456789ab"},
		{name: "unknown prefix is kept on a very short ID", id: "md5:01", want: "md5:01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.id.ShortID())
		})
	}
}
