package container

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Constants for container ID detection.
const (
	minMatchGroups     = 2
	minMountinfoParts  = 2
	minMountinfoFields = 4
)

// Regex patterns for container ID extraction.
var (
	dockerContainerPattern = regexp.MustCompile(`[0-9]+:.*:/docker/([a-f0-9]{64})`)
	containerIDPattern     = regexp.MustCompile(`/containers/([a-f0-9]{64})`)
)

// File reading functions for testing mocks.
var (
	ReadMountinfoFunc = os.ReadFile
	ReadCgroupFunc    = os.ReadFile
)

// containerMarkers are files that container runtimes create inside a container.
var containerMarkers = []string{
	"/run/.containerenv", // Podman
	"/.dockerenv",        // Docker
}

// IsOldContainer reports whether the container's runtime name (from Name()
// or raw inspect) indicates a predecessor renamed during Watchtower self-update.
// It trims any leading '/' (as Docker names have) and checks for the
// WatchtowerOldPrefix convention. Used for disambiguation in hostname
// fallback, forcing cleanup of old containers, skipping redundant renames,
// and suppressing notifs for self-cleanup of prior incarnations.
//
// Parameters:
//   - name: The container name (may be empty, with or without leading '/').
//
// Returns:
//   - bool: true if the (normalized) name has the old- prefix.
func IsOldContainer(name string) bool {
	if name == "" {
		return false
	}

	n := strings.TrimLeft(name, "/")

	return strings.HasPrefix(n, types.WatchtowerOldPrefix)
}

// GetCurrentContainerID retrieves the current container ID using a fallback strategy.
// It attempts multiple detection methods in order of preference and reliability.
//
// The detection methods are tried in the following order:
// 1. Mountinfo-based detection - cgroup v2 compatible
// 2. Cgroup file parsing - cgroup v1 compatible
// 3. Hostname matching - fallback using Docker API
//
// Parameters:
//   - client: Docker client interface for container operations
//
// Returns:
//   - types.ContainerID: The detected container ID if successful
//   - error: Non-nil if all detection methods fail, containing the last error encountered
func GetCurrentContainerID(log *zerolog.Logger, ctx context.Context, client Client) (types.ContainerID, error) {
	// Collect errors from failed detection attempts for final error reporting
	var errs []error

	// First attempt: Mountinfo-based detection - most reliable when available
	log.Debug().Msg("Attempting to get current container ID using mountinfo detection")

	containerID, err := GetContainerIDFromMountinfo(log)
	if err == nil {
		log.Debug().
			Str("container_id", string(containerID)).
			Msg("Successfully detected container ID using mountinfo")

		return containerID, nil
	}

	log.Debug().
		Err(err).
		Msg("Mountinfo detection failed")
	errs = append(errs, err)

	// Second attempt: Cgroup file parsing - works in most containerized environments
	log.Debug().Msg("Attempting to get current container ID using cgroup file parsing")

	containerID, err = GetContainerIDFromCgroupFile(log)
	if err == nil {
		log.Debug().
			Str("container_id", string(containerID)).
			Msg("Successfully detected container ID using cgroup file")

		return containerID, nil
	}

	log.Debug().
		Err(err).
		Msg("Cgroup file parsing failed")
	errs = append(errs, err)

	// Third attempt: Hostname matching - fallback using Docker API
	log.Debug().Msg("Attempting to get current container ID using hostname matching")

	containerID, err = GetContainerIDFromHostname(log, ctx, client)
	if err == nil {
		log.Debug().
			Str("container_id", string(containerID)).
			Msg("Successfully detected container ID using hostname matching")

		return containerID, nil
	}

	log.Debug().
		Err(err).
		Msg("Hostname matching failed")
	errs = append(errs, err)

	// All methods failed - return the last error with context. A binary on
	// the host has no container ID, so callers decide whether this matters.
	lastErr := errs[len(errs)-1]
	log.Debug().
		Err(lastErr).
		Msg("All container ID detection methods failed")

	return "", fmt.Errorf("failed to detect current container ID: %w", lastErr)
}

// InContainer reports whether the process runs in a container, based on the
// marker files that Docker and Podman create.
//
// A marker that cannot be checked for a reason other than not existing counts
// as present, so callers that protect other Watchtower containers fail closed.
//
// Parameters:
//   - fsys: The filesystem to check.
//
// Returns:
//   - bool: True if a container marker exists or cannot be checked.
func InContainer(fsys afero.Fs) bool {
	for _, marker := range containerMarkers {
		_, err := fsys.Stat(marker)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return true
		}
	}

	return false
}

// GetContainerIDFromMountinfo retrieves the container ID from /proc/self/mountinfo.
// Uses the mountinfo file to find container paths containing /containers/<id>.
func GetContainerIDFromMountinfo(log *zerolog.Logger) (types.ContainerID, error) {
	file, err := ReadMountinfoFunc("/proc/self/mountinfo")
	if err != nil {
		log.Debug().
			Err(err).
			Str("file", "/proc/self/mountinfo").
			Msg("Failed to read mountinfo file")

		return "", errReadMountinfoFile
	}

	log.Debug().
		Str("file", "/proc/self/mountinfo").
		Msg("Read mountinfo file successfully")

	containerID, err := ParseContainerIDFromMountinfo(log, string(file))
	if err != nil {
		log.Debug().
			Err(err).
			Str("file", "/proc/self/mountinfo").
			Msg("Failed to extract container ID from mountinfo")

		return "", errExtractContainerIDFromMountinfo
	}

	return containerID, nil
}

// ParseContainerIDFromMountinfo parses the mountinfo string to extract container ID.
func ParseContainerIDFromMountinfo(log *zerolog.Logger, mountinfoString string) (types.ContainerID, error) {
	lines := strings.SplitSeq(strings.TrimSpace(mountinfoString), "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, " - ")
		if len(parts) < minMountinfoParts {
			continue
		}

		firstPart := parts[0]

		fields := strings.Split(firstPart, " ")
		if len(fields) >= minMountinfoFields {
			root := fields[3]
			log.Debug().
				Str("root", root).
				Msg("Processing mountinfo root")

			containerID := ExtractContainerIDFromPath(root)
			if containerID != "" {
				log.Debug().
					Str("id", string(containerID)).
					Msg("Extracted container ID from mountinfo root")

				return containerID, nil
			}
		}
	}

	return "", errNoValidContainerID
}

// ExtractContainerIDFromPath extracts container ID from a path containing /containers/<id>.
func ExtractContainerIDFromPath(path string) types.ContainerID {
	matches := containerIDPattern.FindStringSubmatch(path)
	if len(matches) >= minMatchGroups {
		return types.ContainerID(matches[1])
	}

	return ""
}

// GetContainerIDFromCgroupFile retrieves the container ID from /proc/<pid>/cgroup.
// Uses the cgroup file to find Docker container paths.
func GetContainerIDFromCgroupFile(log *zerolog.Logger) (types.ContainerID, error) {
	filePath := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())

	file, err := ReadCgroupFunc(filePath)
	if err != nil {
		log.Debug().
			Err(err).
			Str("file", filePath).
			Msg("Failed to read cgroup file")

		return "", errReadCgroupFile
	}

	log.Debug().
		Str("file", filePath).
		Msg("Read cgroup file successfully")

	containerID, err := ParseContainerIDFromCgroupString(log, string(file))
	if err != nil {
		log.Debug().
			Err(err).
			Str("file", filePath).
			Msg("Failed to extract container ID from cgroup")

		return "", errExtractContainerID
	}

	return containerID, nil
}

// ParseContainerIDFromCgroupString parses the cgroup string to extract container ID.
func ParseContainerIDFromCgroupString(log *zerolog.Logger, cgroupString string) (types.ContainerID, error) {
	var lines iter.Seq[string]
	if strings.Contains(cgroupString, "\n") {
		lines = strings.Lines(cgroupString)
	} else {
		lines = func(yield func(string) bool) {
			yield(cgroupString)
		}
	}

	for line := range lines {
		trimmedLine := strings.TrimRight(line, "\n")

		matches := dockerContainerPattern.FindStringSubmatch(trimmedLine)

		log.Debug().
			Str("line", trimmedLine).
			Strs("matches", matches).
			Str("pattern", dockerContainerPattern.String()).
			Msg("Processed cgroup line for container ID")

		if len(matches) >= minMatchGroups {
			id := types.ContainerID(matches[1])
			log.Debug().
				Str("id", string(id)).
				Msg("Extracted container ID from cgroup")

			return id, nil
		}
	}

	return "", fmt.Errorf("%w: %q", errNoValidContainerID, cgroupString)
}

// GetContainerIDFromHostname retrieves the container ID by matching the HOSTNAME env var.
//
// It uses the Docker API to list containers and find a matching hostname.
// When multiple containers share the same hostname (e.g., in Docker Compose setups),
// it prefers the container with the Watchtower label.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - client: Docker client interface for container operations.
//
// Returns:
//   - types.ContainerID: The detected container ID if a match is found.
//   - error: Non-nil if the HOSTNAME env var is missing, the Docker client's
//     ListContainers call fails (wrapped and propagated), or no matching container is found.
func GetContainerIDFromHostname(log *zerolog.Logger, ctx context.Context, client Client) (types.ContainerID, error) {
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		return "", ErrContainerIDNotFound
	}

	containers, err := client.ListContainers(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list all containers: %w", err)
	}

	// Collect all containers matching the hostname.
	var (
		firstMatchID    types.ContainerID
		watchtowerMatch types.ContainerID
	)

	for _, c := range containers {
		containerInfo := c.ContainerInfo()
		if containerInfo == nil {
			log.Debug().Msg("Container info is nil, skipping hostname check")

			continue
		}

		if containerInfo.Config == nil {
			log.Debug().Msg("Container config is nil, skipping hostname check")

			continue
		}

		if containerInfo.Config.Hostname == hostname {
			// Record only the first match ID instead of accumulating full containers.
			if firstMatchID == "" {
				firstMatchID = c.ID()
			}

			// Prefer non-old WT to avoid selecting a lingering predecessor
			// as "current" for the running (successor) process.
			if c.IsWatchtower() {
				contName := c.Name()
				if contName == "" {
					contName = containerInfo.Name
				}

				isOld := IsOldContainer(contName)

				if watchtowerMatch == "" {
					watchtowerMatch = c.ID()
				} else if !isOld {
					watchtowerMatch = c.ID()
				}
			}
		}
	}

	// Return the Watchtower container if found among matches.
	if watchtowerMatch != "" {
		log.Debug().
			Str("container_id", string(watchtowerMatch)).
			Msg("Found Watchtower container by hostname with Watchtower label")

		return watchtowerMatch, nil
	}

	// Fall back to the first match if no Watchtower container was found.
	if firstMatchID != "" {
		log.Debug().
			Str("container_id", string(firstMatchID)).
			Msg("Found container by hostname (first match, no Watchtower label)")

		return firstMatchID, nil
	}

	return "", errNoContainerWithHostname
}
