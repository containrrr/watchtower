package flags

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
	"github.com/nicholas-fedor/watchtower/internal/flags/utils"
)

// ApplyEnvToFlags copies bound environment values onto flags that were not set on the CLI.
//
// Call after Cobra parse and before ProcessFlagAliases / SetupLogging so those
// helpers still see env-sourced values via flag Gets. Load continues to resolve
// through Viper with flag > env > default precedence. Static pflag defaults are
// never env-baked at registration time.
//
// Parameters:
//   - flagSet: Parsed persistent flag set.
//   - specs: Aggregated FlagSpec rows.
//
// Returns:
//   - error: Non-nil when a flag cannot be set from env.
func ApplyEnvToFlags(flagSet *pflag.FlagSet, specs []spec.FlagSpec) error {
	for _, flagSpec := range specs {
		if flagSet.Lookup(flagSpec.Name) == nil {
			continue
		}

		if flagSet.Changed(flagSpec.Name) {
			continue
		}

		raw, ok := firstEnv(flagSpec.EnvKeys)
		if !ok {
			continue
		}

		err := applyEnvValue(flagSet, flagSpec, raw)
		if err != nil {
			return fmt.Errorf("%s: %w", flagSpec.Name, err)
		}
	}

	return nil
}

// applyEnvValue sets one flag from a raw environment string without marking it Changed.
//
// Callers must only invoke this when the flag was not already Changed on the CLI.
func applyEnvValue(flagSet *pflag.FlagSet, flagSpec spec.FlagSpec, raw string) error {
	flag := flagSet.Lookup(flagSpec.Name)
	if flag == nil {
		return fmt.Errorf("%w: %q", ErrFlagNotRegistered, flagSpec.Name)
	}

	switch flagSpec.Kind {
	case spec.KindStringSlice, spec.KindStringArray:
		parts := spec.ParseList(raw, flagSpec.ListParse)

		sliceValue, ok := flag.Value.(pflag.SliceValue)
		if !ok {
			return fmt.Errorf("%w: %s is not a slice", ErrUnsupportedFlagKind, flagSpec.Name)
		}

		err := sliceValue.Replace(parts)
		if err != nil {
			return fmt.Errorf("replace %s: %w", flagSpec.Name, err)
		}

		// Replace must not count as a CLI change for *Changed consumers.
		flag.Changed = false

		return nil
	case spec.KindBool, spec.KindString, spec.KindInt, spec.KindDuration:
		value, err := formatEnvForFlag(flagSpec, raw)
		if err != nil {
			return err
		}

		// flagSet.Set marks Changed. Clear it so env bridging stays CLI-neutral.
		err = flagSet.Set(flagSpec.Name, value)
		if err != nil {
			return fmt.Errorf("set %s: %w", flagSpec.Name, err)
		}

		flag.Changed = false

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFlagKind, flagSpec.Name)
	}
}

// presenceEnvKeys are env keys where any presence enables the bound bool flag
// (https://no-color.org/). Empty, "0", and "false" still mean enabled. Other
// keys treat empty as unset and parse non-empty bools with strconv.ParseBool.
var presenceEnvKeys = map[string]struct{}{
	"NO_COLOR": {},
}

// IsPresenceEnvKey reports whether envKey uses presence-means-true semantics.
func IsPresenceEnvKey(envKey string) bool {
	_, ok := presenceEnvKeys[envKey]

	return ok
}

// hasPresenceEnvKey reports whether any of flagSpec's env keys is presence-based.
func hasPresenceEnvKey(flagSpec spec.FlagSpec) bool {
	return slices.ContainsFunc(flagSpec.EnvKeys, IsPresenceEnvKey)
}

// firstEnv returns the first usable environment value for the given keys.
//
// Empty values are skipped unless the key is in presenceEnvKeys (NO_COLOR),
// where presence alone is meaningful (including empty, "0", and "false").
//
// Parameters:
//   - envKeys: Candidate environment variable names in priority order.
//
// Returns:
//   - string: Raw env value (may be empty only for presenceEnvKeys).
//   - bool: True when a usable entry was found.
func firstEnv(envKeys []string) (string, bool) {
	for _, key := range envKeys {
		raw, ok := os.LookupEnv(key)
		if !ok {
			continue
		}

		if raw == "" && !IsPresenceEnvKey(key) {
			continue
		}

		return raw, true
	}

	return "", false
}

// formatEnvForFlag converts a raw env string into a pflag.Set value string.
//
// Presence-based bool flags (NO_COLOR) always become true when firstEnv found
// the key. Other bools parse non-empty values with strconv.ParseBool. Empty is
// not passed for those keys.
func formatEnvForFlag(flagSpec spec.FlagSpec, raw string) (string, error) {
	switch flagSpec.Kind {
	case spec.KindBool:
		// Presence means true for NO_COLOR (empty, "0", "false", "1", ...).
		if hasPresenceEnvKey(flagSpec) {
			return "true", nil
		}

		b, err := strconv.ParseBool(raw)
		if err != nil {
			return "", fmt.Errorf("parse bool: %w", err)
		}

		return strconv.FormatBool(b), nil
	case spec.KindString:
		return raw, nil
	case spec.KindInt:
		// Allow bare integers only. pflag Set accepts decimal strings.
		trimmed := strings.TrimSpace(raw)

		_, err := strconv.Atoi(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse int: %w", err)
		}

		return trimmed, nil
	case spec.KindDuration:
		trimmed := strings.TrimSpace(raw)
		if utils.IsPureNumeric(trimmed) {
			val, err := strconv.ParseFloat(trimmed, 64)
			if err != nil {
				return "", fmt.Errorf("parse duration seconds: %w", err)
			}

			// Clamp so huge bare-second values do not overflow.
			return utils.DurationFromSeconds(val).String(), nil
		}

		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse duration: %w", err)
		}

		return d.String(), nil
	case spec.KindStringSlice, spec.KindStringArray:
		return "", fmt.Errorf("%w: %s handled via Replace", ErrUnsupportedFlagKind, flagSpec.Name)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedFlagKind, flagSpec.Name)
	}
}
