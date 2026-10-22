package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
	"github.com/nicholas-fedor/watchtower/internal/flags/utils"
)

// durationValue reads a duration from Viper with bare-second env support.
//
// When the flag was not changed on the CLI and the bound env value is a pure
// number, it is treated as seconds (legacy WATCHTOWER_TIMEOUT behavior).
//
// Parameters:
//   - vCfg: Bound Viper instance.
//   - flagSet: Parsed flag set (for Changed checks).
//   - name: Flag name.
//   - envKeys: Environment keys bound to this flag.
//
// Returns:
//   - time.Duration: Resolved duration.
func durationValue(
	vCfg *viper.Viper,
	flagSet *pflag.FlagSet,
	name string,
	envKeys []string,
) time.Duration {
	if flagSet.Changed(name) {
		d, err := flagSet.GetDuration(name)
		if err == nil {
			return d
		}
	}

	envSeen := false

	for _, envKey := range envKeys {
		raw := strings.TrimSpace(os.Getenv(envKey))
		if raw == "" {
			continue
		}

		envSeen = true

		if utils.IsPureNumeric(raw) {
			d, err := bareSeconds(raw)
			if err == nil {
				return d
			}

			// Unparseable bare number (for example out-of-range float): same as
			// other invalid env values — fall back to the pflag default below.
			break
		}

		d, err := time.ParseDuration(raw)
		if err == nil {
			return d
		}
	}

	// Invalid env values fall back to the static pflag default, not zero.
	if envSeen {
		d, err := flagSet.GetDuration(name)
		if err == nil {
			return d
		}
	}

	return vCfg.GetDuration(name)
}

// bareSeconds parses a pure numeric string as seconds.
//
// Overflow is clamped via utils.DurationFromSeconds. Invalid input returns an error
// so callers can fall back to the static default.
//
// Parameters:
//   - raw: Pure numeric string (no duration unit suffix).
//
// Returns:
//   - time.Duration: Parsed duration in seconds when successful.
//   - error: Non-nil when raw cannot be parsed as a float.
func bareSeconds(raw string) (time.Duration, error) {
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse bare seconds: %w", err)
	}

	return utils.DurationFromSeconds(val), nil
}

// stringSliceValue reads a string list using the FlagSpec ListParse strategy.
//
// Parameters:
//   - vCfg: Bound Viper instance.
//   - flagSet: Parsed flag set.
//   - name: Flag name.
//   - envKeys: Environment keys bound to this flag.
//   - parse: List parse strategy.
//
// Returns:
//   - []string: Resolved list (never nil).
func stringSliceValue(
	vCfg *viper.Viper,
	flagSet *pflag.FlagSet,
	name string,
	envKeys []string,
	parse spec.ListParseKind,
) []string {
	if flagSet.Changed(name) {
		switch parse {
		case spec.ListNotificationURLs, spec.ListNewline:
			vals, err := flagSet.GetStringArray(name)
			if err == nil {
				return vals
			}
		case spec.ListNone, spec.ListCommaOrSpace, spec.ListCommaOnly, spec.ListNative:
			vals, err := flagSet.GetStringSlice(name)
			if err == nil {
				return vals
			}
		}
	}

	for _, envKey := range envKeys {
		raw := os.Getenv(envKey)
		if raw == "" {
			continue
		}

		return spec.ParseList(raw, parse)
	}

	vals := vCfg.GetStringSlice(name)
	if vals == nil {
		return []string{}
	}

	return vals
}
