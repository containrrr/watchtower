package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestParseAPIEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{
			name:  "empty",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty strings filtered",
			input: []string{"", " ", "  "},
			want:  nil,
		},
		{
			name:  "single",
			input: []string{"metrics"},
			want:  []string{endpointMetrics},
		},
		{
			name:  "multiple pre-split",
			input: []string{"Health", "UPDATE", "metrics"},
			want:  []string{endpointHealth, endpointUpdate, endpointMetrics},
		},
		{
			name:  "comma-separated single element",
			input: []string{"health,update,metrics"},
			want:  []string{endpointHealth, endpointUpdate, endpointMetrics},
		},
		{
			name:  "space-separated single element",
			input: []string{"health update metrics"},
			want:  []string{endpointHealth, endpointUpdate, endpointMetrics},
		},
		{
			name:  "mixed separators in one element",
			input: []string{"health, update metrics"},
			want:  []string{endpointHealth, endpointUpdate, endpointMetrics},
		},
		{
			name:  "duplicates ignored",
			input: []string{"metrics", "metrics", "health"},
			want:  []string{endpointHealth, endpointMetrics},
		},
		{
			name:  "all alone",
			input: []string{"all"},
			want:  allEndpointNames,
		},
		{
			name:  "all with spaces",
			input: []string{" ALL "},
			want:  allEndpointNames,
		},
		{
			name:    "all with other names",
			input:   []string{"all", "metrics"},
			wantErr: errAllMustBeAlone,
		},
		{
			name:    "unknown",
			input:   []string{"metrics", "nope"},
			wantErr: errUnknownEndpoint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAPIEndpoints(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)

			if tt.want == nil {
				assert.True(t, got.Empty())

				return
			}

			for _, name := range tt.want {
				assert.True(t, got.Contains(name), "expected %s", name)
			}

			assert.Len(t, got, len(tt.want))
		})
	}
}

func TestEndpointsFromLegacyMainFlags(t *testing.T) {
	t.Parallel()

	endpointMap := parseLegacyOptions(true, true, false)
	assert.True(t, endpointMap.Contains(endpointUpdate))
	assert.True(t, endpointMap.Contains(endpointMetrics))
	assert.False(t, endpointMap.Contains(endpointContainers))
	assert.False(t, endpointMap.Contains(endpointHealth))
}

func TestFormatEndpoints(t *testing.T) {
	t.Parallel()

	assert.Empty(t, formatEndpoints(nil))
	assert.Empty(t, formatEndpoints(enabledEndpoints{}))

	endpointMap := enabledEndpoints{
		endpointMetrics: {},
		endpointUpdate:  {},
		endpointHealth:  {},
	}
	// Order follows allEndpointNames.
	assert.Equal(t, "health,update,metrics", formatEndpoints(endpointMap))
}

func TestResolveEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("endpoints only", func(t *testing.T) {
		t.Parallel()

		endpointMap, err := ResolveEndpoints([]string{"health", "metrics"}, false, false, false)
		require.NoError(t, err)
		assert.True(t, endpointMap.Contains(endpointHealth))
		assert.True(t, endpointMap.Contains(endpointMetrics))
		assert.False(t, endpointMap.Contains(endpointUpdate))
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()

		endpointMap, err := ResolveEndpoints([]string{"all"}, false, false, false)
		require.NoError(t, err)
		assert.Len(t, endpointMap, len(allEndpointNames))
	})

	t.Run("legacy only", func(t *testing.T) {
		t.Parallel()

		endpointMap, err := ResolveEndpoints(nil, true, false, true)
		require.NoError(t, err)
		assert.True(t, endpointMap.Contains(endpointUpdate))
		assert.True(t, endpointMap.Contains(endpointContainers))
		assert.False(t, endpointMap.Contains(endpointMetrics))
	})

	t.Run("neither", func(t *testing.T) {
		t.Parallel()

		endpointMap, err := ResolveEndpoints(nil, false, false, false)
		require.NoError(t, err)
		assert.True(t, endpointMap.Empty())
	})

	t.Run("union allowlist and legacy with dedupe", func(t *testing.T) {
		t.Parallel()

		// metrics from allowlist + update from legacy → both; metrics not duplicated.
		endpointMap, err := ResolveEndpoints([]string{"metrics", "health"}, true, true, false)
		require.NoError(t, err)
		assert.True(t, endpointMap.Contains(endpointMetrics))
		assert.True(t, endpointMap.Contains(endpointHealth))
		assert.True(t, endpointMap.Contains(endpointUpdate))
		assert.False(t, endpointMap.Contains(endpointContainers))
		assert.Len(t, endpointMap, 3)
	})

	t.Run("invalid endpoints", func(t *testing.T) {
		t.Parallel()

		_, err := ResolveEndpoints([]string{"bogus"}, false, false, false)
		require.ErrorIs(t, err, errUnknownEndpoint)
	})
}

func TestPopulateRouteFlags(t *testing.T) {
	t.Parallel()

	endpointMap := allEndpoints()

	var cfg types.RunConfig
	SetEndpointConfig(endpointMap, &cfg)
	assert.True(t, cfg.EnableHealthAPI)
	assert.True(t, cfg.EnableUpdateAPI)
	assert.True(t, cfg.EnableMetricsAPI)
	assert.True(t, cfg.EnableContainersAPI)
	assert.True(t, cfg.EnableCheckAPI)
	assert.True(t, cfg.EnableHistoryAPI)
	assert.True(t, cfg.EnableImagesAPI)
	assert.True(t, cfg.EnableConfigAPI)
	assert.True(t, cfg.EnableEventsAPI)
	assert.True(t, cfg.EnableSwaggerAPI)

	var empty types.RunConfig
	SetEndpointConfig(enabledEndpoints{}, &empty)
	assert.False(t, empty.EnableHealthAPI)
	assert.False(t, empty.EnableUpdateAPI)
	assert.False(t, empty.EnableMetricsAPI)
}
