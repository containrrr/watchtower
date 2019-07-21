package main // import "github.com/containrrr/watchtower"

import (
	"github.com/containrrr/watchtower/cmd"
	log "github.com/sirupsen/logrus"
)

// DockerAPIMinVersion is the version of the docker API, which is minimally required by
// watchtower. Currently we require at least API 1.24 and therefore Docker 1.12 or later.

var version = "master"
var commit = "unknown"
var date = "unknown"

func init() {
	log.SetLevel(log.InfoLevel)
}

func main() {
	cmd.Execute()
}
