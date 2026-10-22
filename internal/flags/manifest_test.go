package flags_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
)

// flagKindNames maps each flag kind to a stable name for the manifest.
var flagKindNames = map[spec.FlagKind]string{
	spec.KindBool:        "bool",
	spec.KindString:      "string",
	spec.KindInt:         "int",
	spec.KindDuration:    "duration",
	spec.KindStringSlice: "string-slice",
	spec.KindStringArray: "string-array",
}

// listParseNames maps each list parse mode to a stable name for the manifest.
var listParseNames = map[spec.ListParseKind]string{
	spec.ListNone:             "none",
	spec.ListCommaOrSpace:     "comma-or-space",
	spec.ListCommaOnly:        "comma-only",
	spec.ListNotificationURLs: "notification-urls",
	spec.ListNative:           "native",
	spec.ListNewline:          "newline",
}

// TestFlagManifest snapshots every registered flag's user-facing contract.
//
// The manifest covers fields --help does not show, such as env keys, list parsing,
// and hidden flags, so any change to the CLI or environment surface fails here
// until the golden file is deliberately updated.
func TestFlagManifest(t *testing.T) {
	var builder strings.Builder

	for _, flagSpec := range flags.AllSpecs() {
		kind, ok := flagKindNames[flagSpec.Kind]
		if !ok {
			t.Fatalf("flag %q has unknown kind %d; add it to flagKindNames", flagSpec.Name, flagSpec.Kind)
		}

		listParse, ok := listParseNames[flagSpec.ListParse]
		if !ok {
			t.Fatalf("flag %q has unknown list parse %d; add it to listParseNames", flagSpec.Name, flagSpec.ListParse)
		}

		envKeys := "(none)"
		if len(flagSpec.EnvKeys) > 0 {
			envKeys = strings.Join(flagSpec.EnvKeys, ", ")
		}

		fmt.Fprintf(&builder, "--%s\n", flagSpec.Name)
		fmt.Fprintf(&builder, "  shorthand: %q\n", flagSpec.Shorthand)
		fmt.Fprintf(&builder, "  kind: %s\n", kind)
		fmt.Fprintf(&builder, "  default: %s\n", formatDefault(flagSpec.Default))
		fmt.Fprintf(&builder, "  env: %s\n", envKeys)
		fmt.Fprintf(&builder, "  list-parse: %s\n", listParse)
		fmt.Fprintf(&builder, "  hidden: %t\n", flagSpec.Hidden)
		fmt.Fprintf(&builder, "  deprecated: %q\n", flagSpec.Deprecated)
	}

	golden.Assert(t, "flag-manifest", []byte(builder.String()))
}

// formatDefault renders a flag default in Go syntax, with durations shown in
// their readable form instead of nanoseconds.
func formatDefault(value any) string {
	if duration, ok := value.(time.Duration); ok {
		return fmt.Sprintf("time.Duration(%s)", duration)
	}

	return fmt.Sprintf("%#v", value)
}
