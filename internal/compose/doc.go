// Package compose handles Docker Compose labels, on-disk project resolution,
// and applying selected services after a Git checkout.
//
// Label helpers parse com.docker.compose.* metadata used for dependency
// ordering. ResolveProjectDir returns a project only when the user set
// compose-dir or --compose-project.
// Projects are parsed with compose-go from the project's config files, or from
// the default Compose files in the project directory when none are listed.
// Client talks to the Docker daemon through docker/compose v5 (compose up /
// compose build). Watchtower does not shell out to a compose binary.
//
// Apply is scoped to the stale associated service names. Image-only services
// in the same file are left on the registry path.
package compose
