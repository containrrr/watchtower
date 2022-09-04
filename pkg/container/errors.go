package container

import "errors"

var errorNoImageInfo = errors.New("no available image info")
var errorNoContainerInfo = errors.New("no available container info")
var errorInvalidConfig = errors.New("container configuration missing or invalid")
