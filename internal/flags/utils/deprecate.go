package utils

import "github.com/spf13/pflag"

// MarkFlagDeprecated marks a pflag as deprecated with a migration hint.
//
// Parameters:
//   - flagSet: Flag set containing the flag.
//   - name: Flag name.
//   - hint: Migration hint message.
//
// TODO: Remove MarkFlagDeprecated and all legacy flags in the v2 release.
//
//nolint:godox
func MarkFlagDeprecated(flagSet *pflag.FlagSet, name, hint string) {
	if flag := flagSet.Lookup(name); flag != nil {
		flag.Deprecated = hint
		flag.Hidden = false // Keep visible so users see the hint in --help.
	}
}
