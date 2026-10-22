package report

import "strings"

// ImageID is a hash string for a container image.
type ImageID string

// ContainerID is a hash string for a container instance.
type ContainerID string

const shortIDLength = 12

// ShortID returns the 12-character short version of an image ID.
//
// Returns:
//   - string: Shortened ID without "sha256:" prefix.
func (id ImageID) ShortID() string {
	return shortID(string(id))
}

// ShortID returns the 12-character short version of a container ID.
//
// Returns:
//   - string: Shortened ID without "sha256:" prefix.
func (id ContainerID) ShortID() string {
	return shortID(string(id))
}

// shortID shortens a hash string to 12 characters.
//
// Parameters:
//   - longID: Full hash string.
//
// Returns:
//   - string: Shortened ID, adjusted for "sha256:" prefix.
func shortID(longID string) string {
	prefixSep := strings.IndexRune(longID, ':')
	offset := 0
	length := shortIDLength

	if prefixSep >= 0 {
		if longID[0:prefixSep] == "sha256" {
			offset = prefixSep + 1
		} else {
			length += prefixSep + 1
		}
	}

	if len(longID) >= offset+length {
		return longID[offset : offset+length]
	}

	return longID[offset:]
}
