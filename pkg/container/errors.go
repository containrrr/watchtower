package container

import "errors"

var errorNoImageInfo = errors.New("no available image info")
var errorNoContainerInfo = errors.New("no available container info")
var errorInvalidConfig = errors.New("container configuration missing or invalid")
var errorLabelNotFound = errors.New("label was not found in container")

// ErrorLifecycleSkip is returned by a lifecycle hook when the exit code of the command indicated that it ought to be skipped
var ErrorLifecycleSkip = errors.New("skipping container as the pre-update command returned exit code 75 (EX_TEMPFAIL)")
