package ratelimit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "nil receiver uses the sentinel",
			err:  nil,
			want: ErrRateLimited.Error(),
		},
		{
			name: "includes retry-after and allowed budget",
			err: &Error{
				RetryAfter:    2 * time.Second,
				Allowed:       44000,
				AllowedWindow: time.Minute,
			},
			want: "registry rate limited: retry-after 2s allowed 44000 per 1m0s",
		},
		{
			name: "includes retry-after only",
			err:  &Error{RetryAfter: 5 * time.Second},
			want: "registry rate limited: retry-after 5s",
		},
		{
			name: "includes raw message",
			err:  &Error{Message: "toomanyrequests"},
			want: "registry rate limited: toomanyrequests",
		},
		{
			name: "empty fields use the sentinel",
			err:  &Error{},
			want: ErrRateLimited.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestIsMatchesWrappedRateLimit(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("digest check rate limited: %w", ErrRateLimited)
	require.True(t, Is(wrapped))
	assert.False(t, Is(errors.New("connection refused")))
	assert.False(t, Is(nil))
}

// TestIsTokenBucket covers which Retry-After values count as a bucket refill
// signal rather than a backoff instruction.
func TestIsTokenBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info *Error
		want bool
	}{
		{
			name: "nil info",
			info: nil,
			want: true,
		},
		{
			name: "sub-millisecond with quota",
			info: &Error{RetryAfter: 331 * time.Microsecond, Allowed: 44000, AllowedWindow: time.Minute},
			want: true,
		},
		{
			name: "zero with quota",
			info: &Error{Allowed: 44000, AllowedWindow: time.Minute},
			want: true,
		},
		{
			name: "at the floor",
			info: &Error{RetryAfter: minHonorWait},
			want: false,
		},
		{
			name: "seconds",
			info: &Error{RetryAfter: 5 * time.Second},
			want: false,
		},
		{
			name: "beyond the honor window",
			info: &Error{RetryAfter: 2 * time.Hour},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isTokenBucket(tt.info))
		})
	}
}
