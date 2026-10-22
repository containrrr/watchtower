// Package release resolves the exact release tag that matches a pulled image.
//
// A release tag and the OCI version annotation of the same release are not
// required to agree on a leading "v", so the spelling cannot be derived from
// the version alone. This package confirms it by comparing the manifest digest
// of each candidate tag against the digest of the image that was actually
// pulled.
//
// It deliberately does not know anything about Git hosts, changelog URLs, or
// notifications. Deciding whether a lookup is worth performing belongs to the
// caller, which is what keeps the extra registry traffic behind an explicit
// opt-in.
package release
