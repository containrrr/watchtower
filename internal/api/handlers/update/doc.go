// Package update provides the handler for the /v1/update endpoint, which
// updates every watched container or only the containers that use the requested
// images.
//
// An update runs while the request waits or in the background, and only one
// update runs at a time.
package update
