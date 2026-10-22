package preview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatesFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want []State
	}{
		{give: "c", want: []State{ScannedState}},
		{give: "u", want: []State{UpdatedState}},
		{give: "e", want: []State{FailedState}},
		{give: "k", want: []State{SkippedState}},
		{give: "r", want: []State{RestartedState}},
		{give: "t", want: []State{StaleState}},
		{give: "f", want: []State{FreshState}},
		{give: "cue", want: []State{ScannedState, UpdatedState, FailedState}},
		{give: "x", want: []State{}},
		{give: "", want: []State{}},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, StatesFromString(tt.give))
		})
	}
}

func TestLevelsFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want []LogLevel
	}{
		{give: "p", want: []LogLevel{PanicLevel}},
		{give: "f", want: []LogLevel{FatalLevel}},
		{give: "e", want: []LogLevel{ErrorLevel}},
		{give: "w", want: []LogLevel{WarnLevel}},
		{give: "i", want: []LogLevel{InfoLevel}},
		{give: "d", want: []LogLevel{DebugLevel}},
		{give: "t", want: []LogLevel{TraceLevel}},
		{give: "ewi", want: []LogLevel{ErrorLevel, WarnLevel, InfoLevel}},
		{give: "x", want: []LogLevel{}},
		{give: "", want: []LogLevel{}},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, LevelsFromString(tt.give))
		})
	}
}
