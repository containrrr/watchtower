package container

import (
	"errors"
)

// Errors for client operations in client.go.
var (
	// errCreateExecFailed indicates a failure to create an exec instance in a container.
	errCreateExecFailed = errors.New("failed to create exec instance")
	// errStartExecFailed indicates a failure to start an exec instance in a container.
	errStartExecFailed = errors.New("failed to start exec instance")
	// errAttachExecFailed indicates a failure to attach to an exec instance for output capture.
	errAttachExecFailed = errors.New("failed to attach to exec instance")
	// errReadExecOutputFailed indicates a failure to read output from an exec instance.
	errReadExecOutputFailed = errors.New("failed to read exec output")
	// errInspectExecFailed indicates a failure to inspect an exec instance's status.
	errInspectExecFailed = errors.New("failed to inspect exec instance")
	// errCommandFailed indicates a command executed in a container failed with a non-zero exit code.
	errCommandFailed = errors.New("command execution failed")
)

// Errors for container operations in container_source.go.
var (
	// errListContainersFailed indicates a failure to list containers from the Docker host.
	errListContainersFailed = errors.New("failed to list containers")
	// errInspectContainerFailed indicates a failure to inspect a container's details.
	errInspectContainerFailed = errors.New("failed to inspect container")
	// errStopContainerFailed indicates a failure to stop a container with a signal.
	errStopContainerFailed = errors.New("failed to stop container")
	// errRemoveContainerFailed indicates a failure to remove a container from the host.
	errRemoveContainerFailed = errors.New("failed to remove container")
	// errUnexpectedMacInLegacy indicates a MAC address was found in a legacy API configuration where it should not be.
	errUnexpectedMacInLegacy = errors.New("unexpected MAC address in legacy config")
	// errUnexpectedMacInHost indicates a MAC address was found in a host network configuration where it should not be.
	errUnexpectedMacInHost = errors.New("unexpected MAC address in host network config")
	// errNoMacInNonHost indicates no MAC address was found in a non-host network configuration where one is expected.
	errNoMacInNonHost = errors.New("no MAC address found in non-host network config")
	// errNilSourceEndpoint indicates a nil source endpoint was provided.
	errNilSourceEndpoint = errors.New("nil source endpoint provided")
)

// Errors for container start and rename operations in container_target.go.
var (
	// errCreateContainerFailed indicates a failure to create a new container.
	errCreateContainerFailed = errors.New("failed to create container")
	// errStartContainerFailed indicates a failure to start a newly created container.
	errStartContainerFailed = errors.New("failed to start container")
	// errRenameContainerFailed indicates a failure to rename an existing container.
	errRenameContainerFailed = errors.New("failed to rename container")
)

// Errors for container configuration and metadata operations in container.go.
var (
	// errNoImageInfo indicates the container lacks image metadata required for recreation.
	errNoImageInfo = errors.New("no image info available")
	// errNoContainerInfo indicates the container lacks metadata required for recreation.
	errNoContainerInfo = errors.New("no container info available")
	// errInvalidConfig indicates the container's configuration is invalid for recreation.
	errInvalidConfig = errors.New("invalid container configuration")
	// errCurrentContainerNotCached indicates the current container is not cached for scope derivation.
	errCurrentContainerNotCached = errors.New("current container not cached for scope derivation")
)

// Errors for image operations in image.go.
var (
	// ErrImageInUse indicates that the image is actively used by a container and cannot be removed.
	ErrImageInUse = errors.New("image is actively used by a container")
	// errPinnedImage indicates an attempt to pull an immutable (sha256-pinned) image.
	errPinnedImage = errors.New("image is pinned with sha256, skipping pull")
	// errInspectImageFailed indicates a failure to inspect an image from the Docker daemon.
	errInspectImageFailed = errors.New("failed to inspect image")
	// errPullImageFailed indicates a failure to pull an image from the registry.
	errPullImageFailed = errors.New("failed to pull image")
	// errFailedToLoadPullOptions indicates pull options could not be resolved.
	errFailedToLoadPullOptions = errors.New("failed to load pull options")
	// errPullImageUnauthorized indicates an authentication failure during image pull.
	errPullImageUnauthorized = errors.New("failed to pull image: authentication required")
	// errPullImageNotFound indicates the image was not found in the registry.
	errPullImageNotFound = errors.New("failed to pull image: image not found")
	// errReadPullResponseFailed indicates a failure to read the pull response stream.
	errReadPullResponseFailed = errors.New("failed to read pull response")
	// errRemoveImageFailed indicates a failure to remove an image from the Docker host.
	errRemoveImageFailed = errors.New("failed to remove image")
	// errBuildImageIDMissing indicates a successful-looking build returned no image ID.
	errBuildImageIDMissing = errors.New("image build did not return an image ID")
	// errGitRemoteEmpty indicates no clone URL was associated.
	errGitRemoteEmpty = errors.New("git remote is empty")
	// errGitDockerfileEscape indicates a Dockerfile path left the build context.
	errGitDockerfileEscape = errors.New("git dockerfile path escapes context")
)

// Errors for image cooldown operations in cooldown.go and image.go.
var (
	// ErrImageCooldown indicates the image is within the configured cooldown window and should not be pulled.
	ErrImageCooldown = errors.New("image is within cooldown period")
)

// Errors for label operations in metadata.go.
var (
	// errLabelNotFound indicates a requested label is not present in the container's metadata.
	errLabelNotFound = errors.New("label not found")
)

// Errors for container ID detection operations in container_id.go.
var (
	// errReadMountinfoFile indicates a failure to read the mountinfo file for the current process.
	errReadMountinfoFile = errors.New("failed to read mountinfo file")
	// errExtractContainerIDFromMountinfo indicates a failure to extract a container ID from mountinfo data.
	errExtractContainerIDFromMountinfo = errors.New("failed to extract container ID from mountinfo")
	// errReadCgroupFile indicates a failure to read the cgroup file for the current process.
	errReadCgroupFile = errors.New("failed to read cgroup file")
	// errExtractContainerID indicates a failure to extract a container ID from the cgroup data.
	errExtractContainerID = errors.New("failed to extract container ID")
	// errNoValidContainerID indicates no valid Docker container ID was found in the input data.
	errNoValidContainerID = errors.New("no valid docker container ID found in input")
	// errContainerIDNotFound indicates the container ID could not be found using hostname matching.
	errContainerIDNotFound = errors.New("HOSTNAME environment variable is not set")
	// errNoContainerWithHostname indicates no container was found with the matching hostname.
	errNoContainerWithHostname = errors.New("no container found with matching hostname")
)

// Errors for ephemeral orchestrator operations in ephemeral.go.
var (
	// errEphemeralCreateFailed indicates a failure to create the ephemeral orchestrator container.
	errEphemeralCreateFailed = errors.New("failed to create ephemeral orchestrator container")
	// errEphemeralStartFailed indicates a failure to start the ephemeral orchestrator container.
	errEphemeralStartFailed = errors.New("failed to start ephemeral orchestrator container")
)

// Errors for copy-file label operations in copyfile.go.
var (
	// errCopyFileInvalidPath indicates a copy-file label path is not a clean absolute file path.
	errCopyFileInvalidPath = errors.New("invalid copy-file path")
	// errCopyFileDirectory indicates a copy-file label path refers to a directory.
	errCopyFileDirectory = errors.New("copy-file path is a directory")
	// errCopyFileTooLarge indicates a copy-file label path exceeds the per-file size cap.
	errCopyFileTooLarge = errors.New("copy-file path exceeds size limit")
	// errCopyFileSnapshotFailed indicates CopyFromContainer failed for a labeled path.
	errCopyFileSnapshotFailed = errors.New("failed to snapshot copy-file path")
	// errCopyFileInjectFailed indicates CopyToContainer failed for a labeled path.
	errCopyFileInjectFailed = errors.New("failed to copy file into new container")
	// errCopyFileSymlink indicates a copy-file label path is a symbolic link.
	errCopyFileSymlink = errors.New("copy-file path is a symbolic link")
	// errCopyFileUnsafeArchive indicates the copied archive was not a single regular file.
	errCopyFileUnsafeArchive = errors.New("copy-file archive is not a single regular file")
	// errCopyFileStoreFull indicates in-flight copy-file snapshots exceed the memory cap.
	errCopyFileStoreFull = errors.New("copy-file snapshot store exceeds size limit")
)
