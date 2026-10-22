package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerFilter(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		inputName string
		wantMatch bool
	}{
		{
			name:      "exact match",
			patterns:  []string{"mycontainer"},
			inputName: "mycontainer",
			wantMatch: true,
		},
		{
			name:      "exact mismatch",
			patterns:  []string{"othercontainer"},
			inputName: "mycontainer",
			wantMatch: false,
		},
		{
			name:      "regex match with prefix",
			patterns:  []string{"^web-.*"},
			inputName: "web-server",
			wantMatch: true,
		},
		{
			name:      "regex match with suffix",
			patterns:  []string{".*-prod$"},
			inputName: "api-prod",
			wantMatch: true,
		},
		{
			name:      "regex no match",
			patterns:  []string{"^web-.*"},
			inputName: "api-server",
			wantMatch: false,
		},
		{
			name:      "multiple patterns first matches",
			patterns:  []string{"^web-.*", "^api-.*"},
			inputName: "web-server",
			wantMatch: true,
		},
		{
			name:      "multiple patterns second matches",
			patterns:  []string{"^web-.*", "^api-.*"},
			inputName: "api-server",
			wantMatch: true,
		},
		{
			name:      "multiple patterns none match",
			patterns:  []string{"^web-.*", "^api-.*"},
			inputName: "db-server",
			wantMatch: false,
		},
		{
			name:      "empty patterns matches all",
			patterns:  nil,
			inputName: "anything",
			wantMatch: true,
		},
		{
			name:      "empty slice matches all",
			patterns:  []string{},
			inputName: "anything",
			wantMatch: true,
		},
		{
			name:      "invalid regex falls back to exact",
			patterns:  []string{"[invalid"},
			inputName: "[invalid",
			wantMatch: true,
		},
		{
			name:      "invalid regex no match",
			patterns:  []string{"[invalid"},
			inputName: "other",
			wantMatch: false,
		},
		{
			name:      "leading slash on pattern exact match",
			patterns:  []string{"/mycontainer"},
			inputName: "mycontainer",
			wantMatch: true,
		},
		{
			name:      "leading slash on name exact match",
			patterns:  []string{"mycontainer"},
			inputName: "/mycontainer",
			wantMatch: true,
		},
		{
			name:      "leading slash on regex pattern",
			patterns:  []string{"/web-.*"},
			inputName: "web-server",
			wantMatch: true,
		},
		{
			name:      "leading slash on both pattern and name",
			patterns:  []string{"/web-.*"},
			inputName: "/web-server",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := ContainerFilter(tt.patterns)
			got := filter(tt.inputName, true)
			assert.Equal(t, tt.wantMatch, got)
		})
	}
}
