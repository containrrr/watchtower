// Package metadata holds compile-time build information for tplprev.
package metadata

// These values are populated via ldflags from scripts/build-tplprev.sh
// and scripts/build-tplprev.ps1:
//
//	-X github.com/nicholas-fedor/tplprev/internal/metadata.Version=<version>
//	-X github.com/nicholas-fedor/tplprev/internal/metadata.Commit=<commit>
//	-X github.com/nicholas-fedor/tplprev/internal/metadata.Date=<date>
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the formatted build metadata string.
//
// Returns:
//   - string: Version, commit, and build date.
func String() string {
	return Version + " (commit " + Commit + ", built " + Date + ")"
}
