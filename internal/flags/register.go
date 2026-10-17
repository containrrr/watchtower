package flags

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/api"
	"github.com/nicholas-fedor/watchtower/internal/flags/client"
	"github.com/nicholas-fedor/watchtower/internal/flags/compat"
	"github.com/nicholas-fedor/watchtower/internal/flags/docker"
	"github.com/nicholas-fedor/watchtower/internal/flags/filter"
	"github.com/nicholas-fedor/watchtower/internal/flags/git"
	"github.com/nicholas-fedor/watchtower/internal/flags/lifecycle"
	"github.com/nicholas-fedor/watchtower/internal/flags/logging"
	"github.com/nicholas-fedor/watchtower/internal/flags/mode"
	"github.com/nicholas-fedor/watchtower/internal/flags/notify"
	"github.com/nicholas-fedor/watchtower/internal/flags/registry"
	"github.com/nicholas-fedor/watchtower/internal/flags/schedule"
	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
	"github.com/nicholas-fedor/watchtower/internal/flags/update"
)

// RegisterAll registers every domain's flags on the root command.
//
// Domain packages match the config taxonomy: docker, client, schedule, mode,
// update, lifecycle, filter, registry, compat, api, notify, logging, git.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func RegisterAll(rootCmd *cobra.Command) {
	docker.Register(rootCmd)
	client.Register(rootCmd)
	schedule.Register(rootCmd)
	mode.Register(rootCmd)
	update.Register(rootCmd)
	lifecycle.Register(rootCmd)
	filter.Register(rootCmd)
	registry.Register(rootCmd)
	compat.Register(rootCmd)
	api.Register(rootCmd)
	notify.Register(rootCmd)
	logging.Register(rootCmd)
	git.Register(rootCmd)
}

// AllSpecs returns aggregated FlagSpec rows from every domain.
//
// Returns:
//   - []spec.FlagSpec: Flag metadata for BindAll.
func AllSpecs() []spec.FlagSpec {
	return collectSpecs(
		docker.Specs(),
		client.Specs(),
		schedule.Specs(),
		mode.Specs(),
		update.Specs(),
		lifecycle.Specs(),
		filter.Specs(),
		registry.Specs(),
		compat.Specs(),
		api.Specs(),
		notify.Specs(),
		logging.Specs(),
		git.Specs(),
	)
}
