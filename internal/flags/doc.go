// Package flags declares Watchtower's command-line flags and binds them to
// their environment variables.
//
// Each settings domain declares its flags in a subpackage, and values resolve
// as flag, then environment variable, then default. Before the configuration
// loads, this package also applies flag aliases, reads secrets from files,
// configures logging, and exports the Docker client settings to the
// environment.
package flags
