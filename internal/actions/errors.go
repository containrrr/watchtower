package actions

import "errors"

// Errors for sanity and instance checks.
var (
	// errRollingRestartDependency flags incompatible dependencies for rolling restarts.
	errRollingRestartDependency = errors.New(
		"container has dependencies incompatible with rolling restarts",
	)
	// errListContainersFailed flags failures in listing containers.
	errListContainersFailed = errors.New("failed to list containers")
	// errImageRemovalFailed flags failures in image cleanup.
	errImageRemovalFailed = errors.New("errors occurred during image cleanup")
)

// Errors for update operations.
var (
	// errSortDependenciesFailed indicates a failure to sort containers by dependencies.
	errSortDependenciesFailed = errors.New("failed to sort containers by dependencies")
	// errVerifyConfigFailed indicates a failure to verify container configuration for recreation.
	errVerifyConfigFailed = errors.New("failed to verify container configuration")
	// errPreUpdateFailed indicates a failure in the pre-update lifecycle command execution.
	errPreUpdateFailed = errors.New("pre-update command failed")
	// errSkipUpdate signals that a container update should be skipped due to a specific condition (e.g., EX_TEMPFAIL).
	errSkipUpdate = errors.New(
		"skipping container due to pre-update command exit code 75 (EX_TEMPFAIL)",
	)
	// errStopContainerFailed indicates a failure to stop a container during the update process.
	errStopContainerFailed = errors.New("failed to stop container")
	// errContainerGone indicates a container no longer existed when the update tried to stop it.
	errContainerGone = errors.New("container no longer exists")
	// errStartContainerFailed indicates a failure to start a container after an update.
	errStartContainerFailed = errors.New("failed to start container")
	// errCreateContainerFailed indicates a failure to create a container during the update process.
	errCreateContainerFailed = errors.New("failed to create container")
	// errParseImageReference indicates a failure to parse a container's image reference.
	errParseImageReference = errors.New("failed to parse image reference")
	// errInvalidImageReference indicates an invalid image reference that cannot be processed.
	errInvalidImageReference = errors.New("invalid image reference")
	// errCircularDependency indicates a circular dependency between containers.
	errCircularDependency = errors.New("circular dependency detected")
	// errSelfDependency indicates a container has a self-dependency.
	errSelfDependency = errors.New("container has self-dependency")
	// errGitNotAssociated indicates a Git rebuild was requested without an association.
	errGitNotAssociated = errors.New("container is not associated with a Git repository")
	// errGitComposeService indicates a Compose project was resolved without a service name.
	errGitComposeService = errors.New("compose project has no com.docker.compose.service label")
	// errGitComposeInstance indicates Compose apply did not return the requested service.
	errGitComposeInstance = errors.New("compose apply did not recreate the service")
	// errGitComposeCommit indicates stale services in one project resolved different commits.
	errGitComposeCommit = errors.New("compose project services resolved different commits")
)

// Errors for Watchtower self-update operations.
var (
	// errRenameWatchtowerFailed indicates a failure to rename the Watchtower container before restarting.
	errRenameWatchtowerFailed = errors.New("failed to rename Watchtower container")
	// errStopWatchtowerFailed flags failures in stopping excess Watchtower containers.
	errStopWatchtowerFailed = errors.New("errors occurred while stopping watchtower containers")
	// errOldSelfDetected indicates the current container is an old
	// Watchtower container that should not be running.
	errOldSelfDetected = errors.New("current container is an old Watchtower container")
)

// Errors for Docker image-usage budget checks.
var (
	// errImageDiskUsageFailed indicates GET /system/df for images failed.
	errImageDiskUsageFailed = errors.New("failed to query Docker image disk usage")
	// errImageDiskSpaceExceeded indicates image usage reached the configured maximum.
	errImageDiskSpaceExceeded = errors.New("docker image usage exceeds configured maximum")
)

// skipError marks the reason a container was deliberately left untouched, such
// as a pre-update hook asking to skip it or an update canceled before reaching
// it, so the container is reported as skipped rather than failed. Its message
// is the reason's message.
type skipError struct {
	// reason explains why the container was skipped.
	reason error
}

// Error returns the reason's message.
func (e skipError) Error() string {
	return e.reason.Error()
}

// Unwrap returns the reason.
func (e skipError) Unwrap() error {
	return e.reason
}

// skipped marks reason as the cause of a deliberate skip.
//
// Parameters:
//   - reason: Why the container was left untouched.
//
// Returns:
//   - error: The reason, marked as a skip.
func skipped(reason error) error {
	return skipError{reason: reason}
}

// isSkip reports whether err marks a deliberate skip rather than a failure.
// Only errors marked by skipped are skips, so a failure that wraps a
// cancellation, such as a recreation canceled after a container was stopped,
// stays a failure.
//
// Parameters:
//   - err: The outcome of a container's stop or restart.
//
// Returns:
//   - bool: True when err marks a skip.
func isSkip(err error) bool {
	_, ok := errors.AsType[skipError](err)

	return ok
}
