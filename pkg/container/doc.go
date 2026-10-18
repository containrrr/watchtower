// Package container manages the Docker containers that Watchtower watches.
//
// It uses the Docker API to list, inspect, stop, recreate, and start
// containers, pulls and compares their images, runs their lifecycle hook
// commands, and reads the Watchtower labels that control how each container is
// updated.
package container
