package container

import "strings"

// ShortID returns the 12-character (hex) short version of an image ID hash, removing any "sha256:" prefix if present
func ShortID(imageID string) (short string) {
	prefixSep := strings.IndexRune(imageID, ':')
	offset := 0
	length := 12
	if prefixSep >= 0 {
		if imageID[0:prefixSep] == "sha256" {
			offset = prefixSep + 1
		} else {
			length += prefixSep + 1
		}
	}

	if len(imageID) >= offset+length {
		return imageID[offset : offset+length]
	}

	return imageID
}
