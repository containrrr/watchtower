package container

import "errors"

var errorNoImageInfo = errors.New("no available image info")
var errorNoExposedPorts = errors.New("exposed port configuration missing")
var errorInvalidConfig = errors.New("container configuration missing or invalid")
