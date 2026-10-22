package spec

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/nicholas-fedor/watchtower/internal/flags/utils"
)

// errUnsupportedFlagKind indicates a FlagSpec kind is not supported for registration.
var errUnsupportedFlagKind = errors.New("unsupported flag kind")

// register registers pflags from FlagSpec rows using static defaults only.
//
// Environment values are not baked into pflag defaults.
// ApplyEnvToFlags and Viper bind resolve env after parse.
//
// Parameters:
//   - flagSet: Target flag set that receives the registered flags.
//   - specs: Domain flag specifications to register in order.
//
// Returns:
//   - error: Non-nil when a kind is unsupported or a flag cannot be marked hidden.
func register(flagSet *pflag.FlagSet, specs []FlagSpec) error {
	for _, flagSpec := range specs {
		err := registerOne(flagSet, flagSpec)
		if err != nil {
			return err
		}
	}

	return nil
}

// MustRegister registers FlagSpec rows and panics when registration fails.
//
// Parameters:
//   - flagSet: Target flag set that receives the registered flags.
//   - specs: Domain flag specifications to register in order.
func MustRegister(flagSet *pflag.FlagSet, specs []FlagSpec) {
	err := register(flagSet, specs)
	if err != nil {
		panic(err)
	}
}

// registerOne registers a single FlagSpec onto flagSet.
//
// It dispatches by kind, then applies deprecation and hidden metadata when set.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: Flag specification to register.
//
// Returns:
//   - error: Non-nil when the kind is unsupported or MarkHidden fails.
func registerOne(flagSet *pflag.FlagSet, flagSpec FlagSpec) error {
	switch flagSpec.Kind {
	case KindBool:
		registerBool(flagSet, flagSpec)
	case KindString:
		registerString(flagSet, flagSpec)
	case KindInt:
		registerInt(flagSet, flagSpec)
	case KindDuration:
		registerDuration(flagSet, flagSpec)
	case KindStringSlice:
		registerStringSlice(flagSet, flagSpec)
	case KindStringArray:
		registerStringArray(flagSet, flagSpec)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedFlagKind, flagSpec.Name)
	}

	if flagSpec.Deprecated != "" {
		utils.MarkFlagDeprecated(flagSet, flagSpec.Name, flagSpec.Deprecated)
	}

	if flagSpec.Hidden {
		err := flagSet.MarkHidden(flagSpec.Name)
		if err != nil {
			return fmt.Errorf("hide %s: %w", flagSpec.Name, err)
		}
	}

	return nil
}

// stringSliceDefault returns def, or an empty non-nil slice when def is nil.
//
// Parameters:
//   - def: Slice default from FlagSpec, which may be nil.
//
// Returns:
//   - []string: A non-nil slice suitable for pflag StringSlice/StringArray defaults.
func stringSliceDefault(def []string) []string {
	if def == nil {
		return []string{}
	}

	return def
}

// registerBool registers a boolean flag from flagSpec.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: Boolean flag specification (Default must be bool).
func registerBool(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.(bool)
	if flagSpec.Shorthand != "" {
		flagSet.BoolP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.Bool(flagSpec.Name, def, flagSpec.Help)
}

// registerString registers a string flag from flagSpec.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: String flag specification (Default must be string).
func registerString(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.(string)
	if flagSpec.Shorthand != "" {
		flagSet.StringP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.String(flagSpec.Name, def, flagSpec.Help)
}

// registerInt registers an int flag from flagSpec.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: Int flag specification (Default must be int).
func registerInt(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.(int)
	if flagSpec.Shorthand != "" {
		flagSet.IntP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.Int(flagSpec.Name, def, flagSpec.Help)
}

// registerDuration registers a duration flag from flagSpec.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: Duration flag specification (Default must be time.Duration).
func registerDuration(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.(time.Duration)
	if flagSpec.Shorthand != "" {
		flagSet.DurationP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.Duration(flagSpec.Name, def, flagSpec.Help)
}

// registerStringSlice registers a string-slice flag from flagSpec.
//
// A nil Default becomes an empty non-nil slice so pflag receives a stable default.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: String-slice flag specification (Default must be []string or nil).
func registerStringSlice(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.([]string)
	def = stringSliceDefault(def)

	if flagSpec.Shorthand != "" {
		flagSet.StringSliceP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.StringSlice(flagSpec.Name, def, flagSpec.Help)
}

// registerStringArray registers a string-array flag from flagSpec.
//
// A nil Default becomes an empty non-nil slice so pflag receives a stable default.
//
// Parameters:
//   - flagSet: Target flag set.
//   - flagSpec: String-array flag specification (Default must be []string or nil).
func registerStringArray(flagSet *pflag.FlagSet, flagSpec FlagSpec) {
	def, _ := flagSpec.Default.([]string)
	def = stringSliceDefault(def)

	if flagSpec.Shorthand != "" {
		flagSet.StringArrayP(flagSpec.Name, flagSpec.Shorthand, def, flagSpec.Help)

		return
	}

	flagSet.StringArray(flagSpec.Name, def, flagSpec.Help)
}
