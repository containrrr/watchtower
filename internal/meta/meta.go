package meta

var (
	// Version is the compile-time set version of Watchtower
	Version = "v0.0.0-unknown"

	// UserAgent is the http client identifier derived from Version
	UserAgent string
)

func init() {
	UserAgent = "Watchtower/" + Version
}
