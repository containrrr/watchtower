package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/containrrr/watchtower/internal/flags"
)

func TestComputeMaxMemoryPerContainerInByte_shouldBe0(t *testing.T) {
	cmd := new(cobra.Command)
	initializeCmd(cmd)
	PreRun(cmd, cmd.Flags().Args())

	assert.Equal(t, int64(0), maxMemoryPerContainer)
}

func initializeCmd(cmd *cobra.Command) {
	flags.SetDefaults()
	flags.RegisterDockerFlags(cmd)
	flags.RegisterSystemFlags(cmd)
	flags.RegisterNotificationFlags(cmd)
	flags.RegisterContainerMemoryFlags(cmd)
}
