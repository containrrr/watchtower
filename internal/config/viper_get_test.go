package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// TestStringSliceValue_ChangedFlagWinsOverEnv verifies that stringSliceValue
// reads the flag when Changed=true instead of falling back to os.Getenv.
func TestStringSliceValue_ChangedFlagWinsOverEnv(t *testing.T) {
	vCfg := viper.New()
	flagSet := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)

	flagSet.StringArray("test-flag", []string{}, "")
	require.NoError(t, flagSet.Set("test-flag", "gotify://host/token"))
	flag := flagSet.Lookup("test-flag")
	require.NotNil(t, flag)
	flag.Changed = true

	// Env points to a different file path - if stringSliceValue re-reads env instead
	// of the flag, it will return the file path instead of the URL.
	t.Setenv("TEST_NOTIFICATION_URL", "/run/secrets/WATCHTOWER_NOTIFICATION_URL")

	result := stringSliceValue(
		vCfg,
		flagSet,
		"test-flag",
		[]string{"TEST_NOTIFICATION_URL"},
		spec.ListNotificationURLs,
	)

	assert.Equal(t, []string{"gotify://host/token"}, result,
		"stringSliceValue must prefer the Changed=true flag over raw env")
}

// TestStringSliceValue_UnchangedFlagFallsBackToEnv verifies that stringSliceValue
// falls back to os.Getenv when the flag value is unchanged.
func TestStringSliceValue_UnchangedFlagFallsBackToEnv(t *testing.T) {
	vCfg := viper.New()
	flagSet := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)

	flagSet.StringArray("test-flag", []string{}, "")
	require.NoError(t, flagSet.Set("test-flag", "/run/secrets/WATCHTOWER_NOTIFICATION_URL"))
	flag := flagSet.Lookup("test-flag")
	require.NotNil(t, flag)
	flag.Changed = false

	t.Setenv("TEST_NOTIFICATION_URL", "/run/secrets/WATCHTOWER_NOTIFICATION_URL")

	result := stringSliceValue(
		vCfg,
		flagSet,
		"test-flag",
		[]string{"TEST_NOTIFICATION_URL"},
		spec.ListNotificationURLs,
	)

	assert.Equal(t, []string{"/run/secrets/WATCHTOWER_NOTIFICATION_URL"}, result,
		"stringSliceValue must fall back to env when Changed=false")
}

// TestStringSliceValue_ViperGetStringSliceIsFinalFallback verifies that stringSliceValue
// falls back to Viper when neither the flag nor env provide a value.
func TestStringSliceValue_ViperGetStringSliceIsFinalFallback(t *testing.T) {
	vCfg := viper.New()
	flagSet := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)

	flagSet.StringArray("test-flag", []string{}, "")
	require.NoError(t, flagSet.Set("test-flag", ""))
	flag := flagSet.Lookup("test-flag")
	require.NotNil(t, flag)
	flag.Changed = false

	// Ensure env key is unset.
	t.Setenv("TEST_NOTIFICATION_URL", "")

	// Set value via viper (simulating a config file).
	vCfg.Set("test-flag", []string{"smtp://user:pass@host:25"})

	result := stringSliceValue(
		vCfg,
		flagSet,
		"test-flag",
		[]string{"TEST_NOTIFICATION_URL"},
		spec.ListNotificationURLs,
	)

	assert.Equal(t, []string{"smtp://user:pass@host:25"}, result,
		"stringSliceValue must fall back to viper when flag and env are unset")
}
