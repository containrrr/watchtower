// Package containers provides the /v1/containers HTTP API endpoint, exposing the
// current image identity (name, local ID, and registry manifest digest) of each
// container Watchtower watches.
//
// This lets an external orchestrator compare what each container is actually
// running against a registry's current digest without pulling any image layers.
package containers
