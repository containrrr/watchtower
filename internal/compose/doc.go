// Package compose reads Docker Compose labels and projects, and applies updated
// services after a Git checkout.
//
// Compose labels supply the service identities used for dependency ordering. A
// project is resolved only when a container's compose-dir label or the
// compose-project configuration option names one, and it is parsed with
// compose-go from its config files, or from the default Compose files in its
// directory. Services are built and started through the Docker Compose SDK
// rather than a compose binary, and only the stale associated services are
// applied.
package compose
