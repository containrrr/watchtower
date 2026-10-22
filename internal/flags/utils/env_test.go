package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsPureNumeric verifies IsPureNumeric behavior with table-driven cases.
func TestIsPureNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid cases
		{name: "zero", input: "0", want: true},
		{name: "positive integer", input: "42", want: true},
		{name: "large integer (legacy common case)", input: "300", want: true},
		{name: "simple float", input: "1.5", want: true},
		{name: "leading dot", input: ".5", want: true},
		{name: "trailing dot", input: "1.", want: true},
		{name: "negative integer", input: "-10", want: true},
		{name: "explicit positive", input: "+3", want: true},
		{name: "negative float", input: "-1.5", want: true},
		{name: "positive leading dot", input: "+.5", want: true},
		{name: "negative leading dot", input: "-.5", want: true},

		// Invalid: no digits
		{name: "empty string", input: "", want: false},
		{name: "just decimal point", input: ".", want: false},
		{name: "just plus sign", input: "+", want: false},
		{name: "just minus sign", input: "-", want: false},
		{name: "plus and dot only", input: "+.", want: false},
		{name: "minus and dot only", input: "-.", want: false},

		// Invalid: multiple dots
		{name: "multiple dots", input: "1.2.3", want: false},
		{name: "double dot", input: "1..2", want: false},
		{name: "three dots", input: "1.2.3.4", want: false},

		// Invalid: misplaced or multiple signs
		{name: "minus after digit", input: "1-2", want: false},
		{name: "plus at end", input: "12+", want: false},
		{name: "plus in middle", input: "1+2", want: false},
		{name: "trailing minus", input: "5-", want: false},
		{name: "multiple signs at start", input: "+-1", want: false},
		{name: "mixed signs", input: "-+5", want: false},

		// Invalid: other characters
		{name: "contains letter", input: "1a", want: false},
		{name: "scientific notation", input: "1e3", want: false},
		{name: "thousands separator", input: "1,000", want: false},
		{name: "duration unit seconds", input: "30s", want: false},
		{name: "duration unit minutes", input: "2m", want: false},
		{name: "embedded space", input: "1 2", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsPureNumeric(tc.input)
			assert.Equal(t, tc.want, got, "IsPureNumeric(%q)", tc.input)
		})
	}
}
