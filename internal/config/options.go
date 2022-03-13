package config

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"time"
)

// OptBuilder is a helper for registering options to both pflags, viper and env
type OptBuilder struct {
	Flags *pflag.FlagSet
	Hide  bool
}

// NewOptBuilder returns a new OptBuilder with the supplied flags
func NewOptBuilder(flags *pflag.FlagSet) *OptBuilder {
	return &OptBuilder{
		Flags: flags,
	}
}

func (ob *OptBuilder) register(key string, env string) {
	_ = viper.BindEnv(key, env)
	if ob.Hide {
		_ = ob.Flags.MarkHidden(key)
	}
}

// StringP registers a string option with a shorthand
func (ob *OptBuilder) StringP(key stringConfKey, short string, defaultValue string, usage string, env string) {
	ob.Flags.StringP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

// BoolP registers a bool option with a shorthand
func (ob *OptBuilder) BoolP(key boolConfKey, short string, defaultValue bool, usage string, env string) {
	ob.Flags.BoolP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

// IntP registers an int option with a shorthand
func (ob *OptBuilder) IntP(key intConfKey, short string, defaultValue int, usage string, env string) {
	ob.Flags.IntP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

// DurationP registers a duration option with a shorthand
func (ob *OptBuilder) DurationP(key durationConfKey, short string, defaultValue time.Duration, usage string, env string) {
	ob.Flags.DurationP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

// String registers a string option
func (ob *OptBuilder) String(key stringConfKey, defaultValue string, usage string, env string) {
	ob.StringP(key, "", defaultValue, usage, env)
}

// Bool registers a bool option
func (ob *OptBuilder) Bool(key boolConfKey, defaultValue bool, usage string, env string) {
	ob.BoolP(key, "", defaultValue, usage, env)
}

// Int registers a int option
func (ob *OptBuilder) Int(key intConfKey, defaultValue int, usage string, env string) {
	ob.IntP(key, "", defaultValue, usage, env)
}

// StringArray registers a string array option
func (ob *OptBuilder) StringArray(key sliceConfKey, defaultValue []string, usage string, env string) {
	ob.Flags.StringArray(string(key), defaultValue, usage)
	ob.register(string(key), env)
}

// StringSliceP registers a string slice option with a shorthand
func (ob *OptBuilder) StringSliceP(key sliceConfKey, short string, defaultValue []string, usage string, env string) {
	ob.Flags.StringSliceP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

// GetString returns the string option value for the given key
func GetString(key stringConfKey) string {
	return viper.GetString(string(key))
}

// GetBool returns the bool option value for the given key
func GetBool(key boolConfKey) bool {
	return viper.GetBool(string(key))
}

// GetInt returns the int option value for the given key
func GetInt(key intConfKey) int {
	return viper.GetInt(string(key))
}

// GetDuration returns the duration option value for the given key
func GetDuration(key durationConfKey) time.Duration {
	return viper.GetDuration(string(key))
}

// GetSlice  returns the slice option value for the given key
func GetSlice(key sliceConfKey) []string {
	return viper.GetStringSlice(string(key))
}
