package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyJSType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		give         string
		isArray      bool
		isTypedArray bool
		want         jsValueKind
	}{
		{name: "string", give: "string", want: jsKindString},
		{name: "array", give: "object", isArray: true, want: jsKindCollection},
		{name: "typed-array", give: "object", isTypedArray: true, want: jsKindCollection},
		{name: "plain-object", give: "object", want: jsKindInvalid},
		{name: "null", give: "null", want: jsKindInvalid},
		{name: "undefined", give: "undefined", want: jsKindInvalid},
		{name: "number", give: "number", want: jsKindInvalid},
		{name: "boolean", give: "boolean", want: jsKindInvalid},
		{name: "symbol", give: "symbol", want: jsKindInvalid},
		{name: "function", give: "function", want: jsKindInvalid},
		{name: "empty", give: "", want: jsKindInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, classifyJSType(tt.give, tt.isArray, tt.isTypedArray))
		})
	}
}
