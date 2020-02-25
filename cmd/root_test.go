package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/containrrr/watchtower/internal/flags"
)

func TestComputeMaxMemoryPerContainerInByte_default_2G_applyFlagSet(t *testing.T) {
	cmd := new(cobra.Command)
	defaultMaxMemory := int64(2 * (1024 * 1024 * 1024))

	viper.Set("APPLY_RESOURCE_LIMIT", true)
	initializeCmd(cmd)
	PreRun(cmd, cmd.Flags().Args())

	assert.Equal(t, defaultMaxMemory, maxMemoryPerContainer)
}

func TestComputeMaxMemoryPerContainerInByte_applyFlagNotSet(t *testing.T) {
	cmd := new(cobra.Command)

	initializeCmd(cmd)
	PreRun(cmd, cmd.Flags().Args())

	//	assert.Equal(t, int64(0), maxMemoryPerContainer)
}

/*
func TestComputeMaxMemoryPerContainerInByte_custom_4G(t *testing.T) {
	cmd := new(cobra.Command)
	defaultMaxMemory := int64(4 * (1024 * 1024 * 1024))

	//args := []string{"max-memory-per-container", "4G"}
	cmd.Flags().StringP("--max-memory-per-container", "", "4G", "max-memory-per-container")
	initializeCmd(cmd)
	//rootCmd.SetArgs(args)
	//Execute()
	//Run(rootCmd, args)
	PreRun(cmd, cmd.Flags().Args())

	assert.Equal(t, defaultMaxMemory, maxMemoryPerContainer)
}

/*
func TestComputeMaxMemoryPerContainerInByte_custom_512m(t *testing.T) {
	cmd := new(cobra.Command)
	defaultMaxMemory := int64(512 * (1024 * 1024))

	cmd.SetArgs(strings.Split("max-memory-per-container 512M", " "))
	initializeCmd(cmd)
	//args := []string{"max-memory-per-container", "512M"}
	PreRun(cmd, cmd.Flags().Args())
	//Execute()
	assert.Equal(t, defaultMaxMemory, maxMemoryPerContainer)
}
*/
func initializeCmd(cmd *cobra.Command) {
	flags.SetDefaults()
	flags.RegisterDockerFlags(cmd)
	flags.RegisterSystemFlags(cmd)
	flags.RegisterNotificationFlags(cmd)
	flags.RegisterContainerMemoryFlags(cmd)
}
