package util

import (
	"bytes"
	"crypto/rand"
	"fmt"
)

// GenerateRandomSHA256 generates a random 64 character SHA 256 hash string
func GenerateRandomSHA256() string {
	return GenerateRandomPrefixedSHA256()[7:]
}

// GenerateRandomPrefixedSHA256 generates a random 64 character SHA 256 hash string, prefixed with `sha256:`
func GenerateRandomPrefixedSHA256() string {
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	sb := bytes.NewBufferString("sha256:")
	sb.Grow(64)
	for _, h := range hash {
		_, _ = fmt.Fprintf(sb, "%02x", h)
	}
	return sb.String()
}
