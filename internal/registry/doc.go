// Package registry resolves the credentials and pull options for an image's
// registry, and reads image creation times from registries for the image
// cooldown.
//
// Its subpackages authenticate registry requests, compare digests, build
// manifest URLs, name the registry hosts Watchtower treats specially, and
// handle registry rate limits.
package registry
