package container

import (
	"fmt"
	"os"
	"regexp"

	"github.com/containrrr/watchtower/pkg/types"
)

var dockerContainerPattern = regexp.MustCompile(`[0-9]+:.*:/docker/([a-f|0-9]{64})`)

// GetRunningContainerID tries to resolve the current container ID from the current process cgroup information
func GetRunningContainerID() (cid types.ContainerID, err error) {
	file, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", os.Getpid()))
	if err != nil {
		return
	}

	return getRunningContainerIDFromString(string(file)), nil
}

func getRunningContainerIDFromString(s string) types.ContainerID {
	matches := dockerContainerPattern.FindStringSubmatch(s)
	if len(matches) < 2 {
		return ""
	}
	return types.ContainerID(matches[1])
}
