package config

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"time"
)

type optBuilder struct {
	Flags *pflag.FlagSet
	Hide  bool
}

func OptBuilder(flags *pflag.FlagSet) *optBuilder {
	return &optBuilder{
		Flags: flags,
	}
}

func (ob *optBuilder) register(key string, env string) {
	_ = viper.BindEnv(key, env)
	if ob.Hide {
		_ = ob.Flags.MarkHidden(key)
	}
}

func (ob *optBuilder) StringP(key stringConfKey, short string, defaultValue string, usage string, env string) {
	ob.Flags.StringP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

func (ob *optBuilder) BoolP(key boolConfKey, short string, defaultValue bool, usage string, env string) {
	ob.Flags.BoolP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

func (ob *optBuilder) IntP(key intConfKey, short string, defaultValue int, usage string, env string) {
	ob.Flags.IntP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

func (ob *optBuilder) DurationP(key durationConfKey, short string, defaultValue time.Duration, usage string, env string) {
	ob.Flags.DurationP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

func (ob *optBuilder) String(key stringConfKey, defaultValue string, usage string, env string) {
	ob.StringP(key, "", defaultValue, usage, env)
}

func (ob *optBuilder) Bool(key boolConfKey, defaultValue bool, usage string, env string) {
	ob.BoolP(key, "", defaultValue, usage, env)
}

func (ob *optBuilder) Int(key intConfKey, defaultValue int, usage string, env string) {
	ob.IntP(key, "", defaultValue, usage, env)
}

func (ob *optBuilder) StringArray(key sliceConfKey, defaultValue []string, usage string, env string) {
	ob.Flags.StringArray(string(key), defaultValue, usage)
	ob.register(string(key), env)
}

func (ob *optBuilder) StringSliceP(key sliceConfKey, short string, defaultValue []string, usage string, env string) {
	ob.Flags.StringSliceP(string(key), short, defaultValue, usage)
	ob.register(string(key), env)
}

func GetString(key stringConfKey) string {
	return viper.GetString(string(key))
}

func GetBool(key boolConfKey) bool {
	return viper.GetBool(string(key))
}

func GetInt(key intConfKey) int {
	return viper.GetInt(string(key))
}

func GetDuration(key durationConfKey) time.Duration {
	return viper.GetDuration(string(key))
}

func GetSlice(key sliceConfKey) []string {
	return viper.GetStringSlice(string(key))
}
