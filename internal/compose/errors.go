package compose

import "errors"

// Errors returned to callers outside this package.
var (
	// ErrProjectDir indicates compose-dir or --compose-project pointed at an unreadable path.
	ErrProjectDir = errors.New("compose project directory is not readable")
	// errConfigFile indicates a compose config_files entry leaves the project directory.
	errConfigFile = errors.New("compose config file is outside the project directory")
)

// Errors for apply and load inside this package.
var (
	// errNilProject indicates injectLabels was called with a nil project.
	errNilProject = errors.New("compose project is nil")
	// errEmptyProjectDir indicates Apply was called without a project directory.
	errEmptyProjectDir = errors.New("compose project directory is empty")
	// errNoComposeServices indicates Apply was called with no service names.
	errNoComposeServices = errors.New("no compose services to apply")
	// errEmptyProjectName indicates the loaded project has no name, so compose start cannot find its containers.
	errEmptyProjectName = errors.New("compose project name is empty")
)
