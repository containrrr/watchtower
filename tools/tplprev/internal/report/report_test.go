package report

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainerReportMatchesWatchtower checks that templates can call the same
// container report methods in the preview as in Watchtower. The expected list
// is generated from Watchtower's report type by `task tplprev:gen`.
func TestContainerReportMatchesWatchtower(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/container-report-methods.txt")
	require.NoError(t, err)

	var want []string

	for line := range strings.Lines(string(contents)) {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			want = append(want, line)
		}
	}

	reportType := reflect.TypeFor[ContainerReport]()

	got := make([]string, 0, reportType.NumMethod())
	for method := range reportType.Methods() {
		got = append(got, method.Name)
	}

	slices.Sort(got)

	assert.Equal(t, want, got)
}
