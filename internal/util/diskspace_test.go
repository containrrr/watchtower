package util

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiskSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty string", input: "", want: 0},
		{name: "zero string", input: "0", want: 0},
		{name: "whitespace only", input: "   ", want: 0},
		{name: "bare bytes", input: "1024", want: 1024},
		{name: "explicit bytes", input: "1024B", want: 1024},
		{name: "lowercase b", input: "1024b", want: 1024},
		{name: "kilobyte short", input: "1K", want: 1000},
		{name: "kilobyte", input: "1KB", want: 1000},
		{name: "megabyte", input: "2MB", want: 2_000_000},
		{name: "megabyte short", input: "2M", want: 2_000_000},
		{name: "gigabyte", input: "40GB", want: 40_000_000_000},
		{name: "gigabyte short", input: "40G", want: 40_000_000_000},
		{name: "terabyte", input: "1TB", want: 1_000_000_000_000},
		{name: "petabyte", input: "1PB", want: 1_000_000_000_000_000},
		{name: "kibibyte", input: "1KiB", want: 1024},
		{name: "mebibyte", input: "1MiB", want: 1024 * 1024},
		{name: "gibibyte", input: "1GiB", want: 1024 * 1024 * 1024},
		{name: "tebibyte", input: "1TiB", want: 1024 * 1024 * 1024 * 1024},
		{name: "pebibyte", input: "1PiB", want: 1024 * 1024 * 1024 * 1024 * 1024},
		{name: "decimal gigabytes", input: "1.5GB", want: 1_500_000_000},
		{name: "exact thousandth of a kilobyte", input: "0.001KB", want: 1},
		{name: "max int64 bytes", input: "9223372036854775807", want: 9223372036854775807},
		{name: "overflow int64 bytes", input: "9223372036854775808", wantErr: true},
		{name: "overflow pebibytes", input: "8192PiB", wantErr: true},
		{name: "lowercase unit", input: "10mb", want: 10_000_000},
		{name: "mixed case unit", input: "10Mb", want: 10_000_000},
		{name: "space before unit", input: "40 GB", want: 40_000_000_000},
		{name: "leading space", input: "  8GiB", want: 8 * 1024 * 1024 * 1024},
		{name: "negative", input: "-1GB", wantErr: true},
		{name: "invalid number", input: "abc", wantErr: true},
		{name: "invalid unit", input: "5x", wantErr: true},
		{name: "percent not accepted", input: "80%", wantErr: true},
		{name: "empty number", input: "GB", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDiskSpace(tc.input)
			if tc.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormatDiskSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero", bytes: 0, want: "0 B"},
		{name: "bytes", bytes: 512, want: "512 B"},
		{name: "999 bytes", bytes: 999, want: "999 B"},
		{name: "exact kilobyte", bytes: 1000, want: "1 KB"},
		{name: "fractional kilobyte", bytes: 1536, want: "1.54 KB"},
		{name: "exact megabyte", bytes: 2_000_000, want: "2 MB"},
		{name: "exact gigabyte", bytes: 10_000_000_000, want: "10 GB"},
		{name: "warning gigabytes", bytes: 8_000_000_000, want: "8 GB"},
		{name: "decimal gigabyte", bytes: 1_500_000_000, want: "1.5 GB"},
		{name: "exact terabyte", bytes: 1_000_000_000_000, want: "1 TB"},
		{name: "exact petabyte", bytes: 1_000_000_000_000_000, want: "1 PB"},
		{name: "negative gigabyte", bytes: -10_000_000_000, want: "-10 GB"},
		{name: "min int64", bytes: math.MinInt64, want: "-9223.37 PB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, FormatDiskSpace(tc.bytes))
		})
	}
}
