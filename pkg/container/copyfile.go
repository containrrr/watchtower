package container

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// maxCopyFileBytes is the maximum size of a single labeled file to snapshot.
	maxCopyFileBytes = 1 << 20
	// maxCopyFileTarBytes bounds the tar stream read for a labeled file.
	maxCopyFileTarBytes = maxCopyFileBytes + 32<<10
	// maxCopyFileStoreBytes caps in-flight snapshot archives across all containers.
	maxCopyFileStoreBytes = 16 << 20
	// unixFilePermMask keeps owner/group/other bits and drops setuid, setgid, and sticky.
	unixFilePermMask = 0o777
	// bindStringMinParts is source and destination in a HostConfig.Binds entry.
	bindStringMinParts = 2
	// injectRootPath is the extraction point for injected files. The daemon requires
	// it to name an existing directory, and the archive extractor creates any absent
	// parent of the archived member, so injecting at the root matches Docker Compose.
	injectRootPath = "/"
	// bindReadOnlyOption is the HostConfig.Binds option that marks a bind read-only.
	bindReadOnlyOption = "ro"
	// userNSSecurityOption is the daemon security option reported under --userns-remap.
	userNSSecurityOption = "name=userns"
)

// copyFileSkipReason explains why a labeled path is not copied into a replacement.
type copyFileSkipReason uint8

const (
	// copyFileSkipNone means the path should be snapshotted and injected.
	copyFileSkipNone copyFileSkipReason = iota
	// copyFileSkipMountedAtPath means a mount destination is the path itself.
	copyFileSkipMountedAtPath
	// copyFileSkipReadOnlyMount means a read-only mount above the path provides the file.
	copyFileSkipReadOnlyMount
	// copyFileSkipReadOnlyRootfs means no mount covers the path and the root filesystem is read-only.
	copyFileSkipReadOnlyRootfs
)

// CopyFileStore captures labeled files around recreate and discards leftovers.
type CopyFileStore interface {
	// SnapshotCopyFiles captures labeled files before the source is removed.
	SnapshotCopyFiles(ctx context.Context, container types.Container) error
	// DiscardCopyFiles drops a leftover snapshot for the source container.
	DiscardCopyFiles(containerID types.ContainerID)
}

var _ CopyFileStore = (*client)(nil)

// copyFileEntry is one labeled file captured from a container.
type copyFileEntry struct {
	target  string
	archive []byte
}

// copyFileSnapshot holds tar archives to inject into a replacement container.
type copyFileSnapshot struct {
	files []copyFileEntry
}

// empty reports whether the snapshot contains no files.
//
// Returns:
//   - bool: True when no files were captured.
func (s copyFileSnapshot) empty() bool {
	return len(s.files) == 0
}

// size returns the total archive bytes in the snapshot.
//
// Returns:
//   - int64: Sum of stored tar archive lengths.
func (s copyFileSnapshot) size() int64 {
	var total int64

	for _, file := range s.files {
		total += int64(len(file.archive))
	}

	return total
}

// parseCopyFileLabel splits and validates comma-separated in-container paths.
//
// Parameters:
//   - value: Raw copy-file label value.
//
// Returns:
//   - []string: Clean absolute paths in label order, with duplicates removed.
//   - error: Non-nil if any path is invalid.
func parseCopyFileLabel(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		filePath := strings.TrimSpace(part)
		if filePath == "" {
			continue
		}

		err := validateCopyFilePath(filePath)
		if err != nil {
			return nil, err
		}

		if _, ok := seen[filePath]; ok {
			continue
		}

		seen[filePath] = struct{}{}
		out = append(out, filePath)
	}

	return out, nil
}

// validateCopyFilePath rejects relative, root, unclean, or parent-traversal paths.
//
// Parameters:
//   - filePath: Candidate in-container path.
//
// Returns:
//   - error: Non-nil if the path is not a clean absolute file path.
func validateCopyFilePath(filePath string) error {
	if filePath == "" || filePath == "/" {
		return fmt.Errorf("%w: %q", errCopyFileInvalidPath, filePath)
	}

	if !path.IsAbs(filePath) || strings.Contains(filePath, "\\") {
		return fmt.Errorf("%w: %q", errCopyFileInvalidPath, filePath)
	}

	if path.Clean(filePath) != filePath {
		return fmt.Errorf("%w: %q", errCopyFileInvalidPath, filePath)
	}

	for elem := range strings.SplitSeq(filePath, "/") {
		if elem == ".." {
			return fmt.Errorf("%w: %q", errCopyFileInvalidPath, filePath)
		}
	}

	return nil
}

// copyFilePaths reads and validates the copy-file label on a container.
//
// Parameters:
//   - c: Container whose labels should be read.
//
// Returns:
//   - []string: Validated in-container paths, or nil when the label is absent.
//   - error: Non-nil if the label contains an invalid path.
func copyFilePaths(c types.Container) ([]string, error) {
	if c == nil {
		return nil, nil
	}

	val, ok := c.GetLabel(copyFileLabel)
	if !ok || val == "" {
		return nil, nil
	}

	return parseCopyFileLabel(val)
}

// copyFileSkipForPath reports why a labeled path is not copied, if it is skipped.
//
// A read-only root filesystem skips every path because the archive is extracted at the
// container root and the Engine refuses a write there.
//
// A mount that provides the file skips the copy too, because that mount is preserved on
// recreation. That covers a mount whose destination is the path itself and a read-only
// mount above it. Anonymous volumes are always writable, so a writable mount above the
// path does not skip it and the file is copied into the recreated volume.
//
// Parameters:
//   - c: Container whose inspect mounts should be checked.
//   - filePath: Absolute in-container path.
//
// Returns:
//   - copyFileSkipReason: Reason the path must be skipped, or copyFileSkipNone.
func copyFileSkipForPath(c types.Container, filePath string) copyFileSkipReason {
	info := c.ContainerInfo()
	if info == nil {
		return copyFileSkipNone
	}

	if info.HostConfig != nil && info.HostConfig.ReadonlyRootfs {
		return copyFileSkipReadOnlyRootfs
	}

	dest, readOnly, covered := mostSpecificMount(info, filePath)
	if !covered {
		return copyFileSkipNone
	}

	if pathHasMount(filePath, dest) {
		return copyFileSkipMountedAtPath
	}

	if readOnly {
		return copyFileSkipReadOnlyMount
	}

	return copyFileSkipNone
}

// mostSpecificMount returns the deepest mount covering filePath.
//
// Docker resolves a path against the deepest mount whose destination contains it, so a
// deeper writable mount shadows a shallower read-only one. Using the deepest entry
// keeps a path copyable whenever the write would actually succeed.
//
// Parameters:
//   - info: Container inspect response.
//   - filePath: Absolute in-container path.
//
// Returns:
//   - string: Cleaned destination of the deepest covering mount.
//   - bool: Whether that mount is read-only.
//   - bool: True when any mount covers filePath.
func mostSpecificMount(info *dockerContainer.InspectResponse, filePath string) (string, bool, bool) {
	var (
		bestDest string
		bestRO   bool
		found    bool
	)

	consider := func(dest string, readOnly bool) {
		if !pathIsUnderMount(filePath, dest) {
			return
		}

		clean := path.Clean(dest)
		if found && len(clean) <= len(bestDest) {
			return
		}

		bestDest, bestRO, found = clean, readOnly, true
	}

	for _, mount := range info.Mounts {
		consider(mount.Destination, !mount.RW)
	}

	if info.HostConfig == nil {
		return bestDest, bestRO, found
	}

	for _, mount := range info.HostConfig.Mounts {
		consider(mount.Target, mount.ReadOnly)
	}

	for _, bind := range info.HostConfig.Binds {
		if dest := bindDestination(bind); dest != "" {
			consider(dest, bindIsReadOnly(bind))
		}
	}

	return bestDest, bestRO, found
}

// shouldSkipCopyFile logs and reports whether a labeled path cannot be copied.
//
// Parameters:
//   - clog: Logger with container fields.
//   - source: Source container being snapshotted.
//   - filePath: Absolute in-container path.
//
// Returns:
//   - bool: True when the path must be skipped.
func shouldSkipCopyFile(clog *zerolog.Logger, source types.Container, filePath string) bool {
	switch copyFileSkipForPath(source, filePath) {
	case copyFileSkipMountedAtPath:
		clog.Debug().
			Str("path", filePath).
			Msg("Skipping copy-file path because a mount provides it")

		return true
	case copyFileSkipReadOnlyMount:
		clog.Debug().
			Str("path", filePath).
			Msg("Skipping copy-file path because a read-only mount provides it")

		return true
	case copyFileSkipReadOnlyRootfs:
		clog.Debug().
			Str("path", filePath).
			Msg("Skipping copy-file path because the root filesystem is read-only")

		return true
	case copyFileSkipNone:
	}

	return false
}

// pathHasMount reports whether filePath is exactly a mount destination.
//
// Parameters:
//   - filePath: Absolute in-container file path.
//   - mountDest: Absolute mount destination.
//
// Returns:
//   - bool: True when filePath is the mount destination itself.
func pathHasMount(filePath, mountDest string) bool {
	mountDest = path.Clean(mountDest)
	if mountDest == "" || mountDest == "." {
		return false
	}

	return path.Clean(filePath) == mountDest
}

// pathIsUnderMount reports whether filePath is mountDest or a descendant.
//
// Parameters:
//   - filePath: Absolute in-container file path.
//   - mountDest: Absolute mount destination.
//
// Returns:
//   - bool: True if filePath is at or under mountDest.
func pathIsUnderMount(filePath, mountDest string) bool {
	if pathHasMount(filePath, mountDest) {
		return true
	}

	return strings.HasPrefix(path.Clean(filePath), path.Clean(mountDest)+"/")
}

// bindDestination returns the container path from a Unix bind string.
//
// Parameters:
//   - bind: HostConfig.Binds entry in source:destination[:options] form.
//
// Returns:
//   - string: Container destination, or empty if the bind is malformed.
func bindDestination(bind string) string {
	parts := strings.Split(bind, ":")
	if len(parts) < bindStringMinParts {
		return ""
	}

	return parts[1]
}

// bindIsReadOnly reports whether a Unix bind string marks the mount read-only.
//
// Parameters:
//   - bind: HostConfig.Binds entry in source:destination[:options] form.
//
// Returns:
//   - bool: True when the options include ro.
func bindIsReadOnly(bind string) bool {
	parts := strings.Split(bind, ":")
	if len(parts) < bindStringMinParts+1 {
		return false
	}

	return slices.Contains(strings.Split(parts[bindStringMinParts], ","), bindReadOnlyOption)
}

// snapshotCopyFiles reads labeled files from the source container into memory.
//
// Missing paths are skipped. Invalid labels, directories, oversized files, and
// unexpected copy errors fail the snapshot so the source container is not removed.
//
// Parameters:
//   - log: Logger for debug messages.
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker copy API.
//   - source: Source container to copy from.
//
// Returns:
//   - copyFileSnapshot: Captured tar archives keyed by in-container path.
//   - error: Non-nil if a labeled path cannot be snapshotted.
func snapshotCopyFiles(
	log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	source types.Container,
) (copyFileSnapshot, error) {
	paths, err := copyFilePaths(source)
	if err != nil {
		return copyFileSnapshot{}, err
	}

	if len(paths) == 0 {
		return copyFileSnapshot{}, nil
	}

	clogVal := log.With().
		Str("container", source.Name()).
		Str("id", source.ID().ShortID()).
		Logger()
	clog := &clogVal
	files := make([]copyFileEntry, 0, len(paths))

	var total int64

	for _, filePath := range paths {
		if shouldSkipCopyFile(clog, source, filePath) {
			continue
		}

		entry, skip, snapErr := snapshotCopyFile(ctx, clog, api, string(source.ID()), filePath)
		if snapErr != nil {
			return copyFileSnapshot{}, snapErr
		}

		if skip {
			continue
		}

		next := total + int64(len(entry.archive))
		if next > maxCopyFileStoreBytes {
			return copyFileSnapshot{}, fmt.Errorf("%w: %d bytes", errCopyFileStoreFull, next)
		}

		files = append(files, entry)
		total = next
	}

	return copyFileSnapshot{files: files}, nil
}

// snapshotCopyFile copies one labeled path from a container into a tar archive.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - clog: Logger with container fields.
//   - api: Docker copy API.
//   - containerID: Source container ID.
//   - filePath: Absolute in-container path.
//
// Returns:
//   - copyFileEntry: Captured archive when the path exists and is a file.
//   - bool: True when the path should be skipped.
//   - error: Non-nil if the copy fails for a reason other than a missing path.
func snapshotCopyFile(
	ctx context.Context,
	clog *zerolog.Logger,
	api dockerClient.APIClient,
	containerID string,
	filePath string,
) (copyFileEntry, bool, error) {
	result, err := api.CopyFromContainer(ctx, containerID, dockerClient.CopyFromContainerOptions{
		SourcePath: filePath,
	})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			clog.Debug().
				Str("path", filePath).
				Msg("Skipping missing copy-file path")

			return copyFileEntry{}, true, nil
		}

		return copyFileEntry{}, false, fmt.Errorf("%w: %s: %w", errCopyFileSnapshotFailed, filePath, err)
	}

	if result.Content != nil {
		defer result.Content.Close()
	}

	if result.Stat.Mode.IsDir() {
		return copyFileEntry{}, false, fmt.Errorf("%w: %q", errCopyFileDirectory, filePath)
	}

	if result.Stat.LinkTarget != "" || result.Stat.Mode.Type() == os.ModeSymlink {
		return copyFileEntry{}, false, fmt.Errorf("%w: %q", errCopyFileSymlink, filePath)
	}

	if result.Stat.Size > maxCopyFileBytes {
		return copyFileEntry{}, false, fmt.Errorf("%w: %q", errCopyFileTooLarge, filePath)
	}

	if result.Content == nil {
		return copyFileEntry{}, false, fmt.Errorf("%w: %s: empty archive stream", errCopyFileSnapshotFailed, filePath)
	}

	limited := io.LimitReader(result.Content, maxCopyFileTarBytes+1)

	archive, err := io.ReadAll(limited)
	if err != nil {
		return copyFileEntry{}, false, fmt.Errorf("%w: %s: %w", errCopyFileSnapshotFailed, filePath, err)
	}

	if int64(len(archive)) > maxCopyFileTarBytes {
		return copyFileEntry{}, false, fmt.Errorf("%w: %q", errCopyFileTooLarge, filePath)
	}

	sanitized, err := sanitizeCopyFileArchive(archive, filePath)
	if err != nil {
		return copyFileEntry{}, false, err
	}

	return copyFileEntry{target: filePath, archive: sanitized}, false, nil
}

// sanitizeCopyFileArchive rewrites a Docker copy archive to a single regular file.
//
// The output tar holds one regular file named with the full destination path, permission
// bits without setuid, setgid, or sticky, and the original uid, gid, and file contents.
// Naming the member with the whole path lets the daemon create any absent parent, which
// is the same mechanism Docker Compose uses to write content-based configs.
//
// Parameters:
//   - raw: Tar stream returned by CopyFromContainer.
//   - destPath: Absolute in-container destination path.
//
// Returns:
//   - []byte: Sanitized tar archive.
//   - error: Non-nil if the archive is not a single regular file at destPath.
func sanitizeCopyFileArchive(raw []byte, destPath string) ([]byte, error) {
	err := validateCopyFilePath(destPath)
	if err != nil {
		return nil, err
	}

	base := path.Base(destPath)
	reader := tar.NewReader(bytes.NewReader(raw))

	var (
		body    []byte
		header  *tar.Header
		found   bool
		readErr error
	)

	for {
		next, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("%w: %w", errCopyFileUnsafeArchive, err)
		}

		switch next.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			continue
		case tar.TypeReg:
			if found {
				return nil, fmt.Errorf("%w: extra member %q", errCopyFileUnsafeArchive, next.Name)
			}

			if !copyFileTarNameOK(next.Name, base) {
				return nil, fmt.Errorf("%w: %q", errCopyFileUnsafeArchive, next.Name)
			}

			if next.Size > maxCopyFileBytes {
				return nil, fmt.Errorf("%w: %q", errCopyFileTooLarge, destPath)
			}

			body, readErr = io.ReadAll(io.LimitReader(reader, maxCopyFileBytes+1))
			if readErr != nil {
				return nil, fmt.Errorf("%w: %s: %w", errCopyFileSnapshotFailed, destPath, readErr)
			}

			if int64(len(body)) > maxCopyFileBytes {
				return nil, fmt.Errorf("%w: %q", errCopyFileTooLarge, destPath)
			}

			header = next
			found = true
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("%w: %q", errCopyFileSymlink, destPath)
		default:
			return nil, fmt.Errorf("%w: %q", errCopyFileUnsafeArchive, next.Name)
		}
	}

	if !found {
		return nil, fmt.Errorf("%w: empty archive", errCopyFileUnsafeArchive)
	}

	return writeSanitizedCopyFileTar(destPath, body, header)
}

// copyFileTarNameOK reports whether a tar member name is the destination basename.
//
// Parameters:
//   - name: Tar header name.
//   - base: Expected file basename.
//
// Returns:
//   - bool: True when name has no parent traversal and its base name matches.
func copyFileTarNameOK(name, base string) bool {
	if name == "" || strings.Contains(name, "..") {
		return false
	}

	return path.Base(path.Clean(name)) == base
}

// writeSanitizedCopyFileTar builds a one-member regular-file tar.
//
// Parameters:
//   - destPath: Absolute in-container path written into the tar header.
//   - body: File contents.
//   - src: Original tar header for uid, gid, mode, and modtime.
//
// Returns:
//   - []byte: Tar archive.
//   - error: Non-nil if the tar cannot be written.
func writeSanitizedCopyFileTar(destPath string, body []byte, src *tar.Header) ([]byte, error) {
	var buf bytes.Buffer

	writer := tar.NewWriter(&buf)

	err := writer.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     destPath,
		Size:     int64(len(body)),
		Mode:     src.Mode & unixFilePermMask,
		Uid:      src.Uid,
		Gid:      src.Gid,
		Uname:    src.Uname,
		Gname:    src.Gname,
		ModTime:  src.ModTime,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCopyFileUnsafeArchive, err)
	}

	_, err = writer.Write(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCopyFileUnsafeArchive, err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCopyFileUnsafeArchive, err)
	}

	return buf.Bytes(), nil
}

// injectCopyFiles writes a snapshot into a newly created container before start.
//
// The archive is extracted at the container root with the destination as the member name,
// so a parent directory missing from the new image is created rather than failing the copy.
// Files keep the uid and gid captured in the snapshot unless the daemon remaps user
// namespaces, as described on copyFileUIDGID.
//
// Parameters:
//   - log: Logger for debug messages.
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker copy API.
//   - containerID: Replacement container ID.
//   - source: Source container whose name is attached to log records.
//   - snapshot: Files captured from the source container.
//
// Returns:
//   - error: Non-nil if a file cannot be written into the new container.
func injectCopyFiles(
	log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	containerID string,
	source types.Container,
	snapshot copyFileSnapshot,
) error {
	if snapshot.empty() {
		return nil
	}

	clogVal := log.With().
		Str("container", source.Name()).
		Str("new_id", containerID).
		Logger()
	clog := &clogVal
	copyUIDGID := copyFileUIDGID(ctx, clog, api)

	for _, file := range snapshot.files {
		_, err := api.CopyToContainer(ctx, containerID, dockerClient.CopyToContainerOptions{
			DestinationPath: injectRootPath,
			Content:         bytes.NewReader(file.archive),
			CopyUIDGID:      copyUIDGID,
		})
		if err != nil {
			return fmt.Errorf("%w: %s: %w", errCopyFileInjectFailed, file.target, err)
		}

		clog.Debug().
			Str("path", file.target).
			Msg("Copied file into new container")
	}

	return nil
}

// copyFileUIDGID reports whether injection asks the daemon to apply Config.User ownership.
//
// With CopyUIDGID false the daemon keeps the uid and gid stored in the archive, which
// preserves the ownership captured from the source container. A daemon that remaps user
// namespaces returns host IDs when it archives a file but treats archive IDs as container
// IDs when it extracts one, so it rejects any captured owner other than root. On those
// daemons CopyUIDGID true chowns each file to Config.User instead, which the daemon maps
// itself. When the daemon info is unavailable, the captured ownership is kept.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - clog: Logger with container fields.
//   - api: Docker API used to read the daemon security options.
//
// Returns:
//   - bool: True when the daemon remaps user namespaces.
func copyFileUIDGID(ctx context.Context, clog *zerolog.Logger, api dockerClient.APIClient) bool {
	result, err := api.Info(ctx, dockerClient.InfoOptions{})
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to get daemon info, keeping copied file ownership")

		return false
	}

	if !daemonRemapsUserNamespaces(result.Info.SecurityOptions) {
		return false
	}

	clog.Debug().Msg("Daemon remaps user namespaces, copied files take the container user ownership")

	return true
}

// daemonRemapsUserNamespaces reports whether daemon security options include userns.
//
// Parameters:
//   - securityOptions: Info.SecurityOptions entries, each a comma-separated key=value list.
//
// Returns:
//   - bool: True when any entry names the userns option.
func daemonRemapsUserNamespaces(securityOptions []string) bool {
	for _, option := range securityOptions {
		for field := range strings.SplitSeq(option, ",") {
			if field == userNSSecurityOption {
				return true
			}
		}
	}

	return false
}

// SnapshotCopyFiles captures labeled files before the source is removed.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - source: Source container about to be stopped and removed.
//
// Returns:
//   - error: Non-nil if a labeled path cannot be snapshotted.
func (c *client) SnapshotCopyFiles(ctx context.Context, source types.Container) error {
	snapshot, err := snapshotCopyFiles(c.logger(), ctx, c.api, source)
	if err != nil {
		return err
	}

	if snapshot.empty() {
		return nil
	}

	return c.storeCopyFileSnapshot(source.ID(), snapshot)
}

// storeCopyFileSnapshot records a snapshot if it fits in the in-flight byte cap.
//
// Parameters:
//   - containerID: Source container ID.
//   - snapshot: Sanitized archives to hold until inject.
//
// Returns:
//   - error: Non-nil if storing the snapshot would exceed maxCopyFileStoreBytes.
func (c *client) storeCopyFileSnapshot(
	containerID types.ContainerID,
	snapshot copyFileSnapshot,
) error {
	c.copyFileMu.Lock()
	defer c.copyFileMu.Unlock()

	if c.copyFiles == nil {
		c.copyFiles = make(map[types.ContainerID]copyFileSnapshot)
	}

	existing, ok := c.copyFiles[containerID]
	if ok {
		c.copyFileBytes -= existing.size()
	}

	if c.copyFileBytes < 0 {
		c.copyFileBytes = 0
	}

	next := c.copyFileBytes + snapshot.size()
	if next > maxCopyFileStoreBytes {
		if ok {
			c.copyFileBytes += existing.size()
		}

		return fmt.Errorf("%w: %d bytes", errCopyFileStoreFull, next)
	}

	c.copyFiles[containerID] = snapshot
	c.copyFileBytes = next

	return nil
}

// injectStoredCopyFiles writes a stored snapshot into a newly created container.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - source: Source container whose snapshot should be injected.
//   - newID: ID of the replacement container.
//
// Returns:
//   - error: Non-nil if a stored file cannot be written into the new container.
func (c *client) injectStoredCopyFiles(
	ctx context.Context,
	source types.Container,
	newID types.ContainerID,
) error {
	c.copyFileMu.Lock()
	snapshot, ok := c.dropCopyFileLocked(source.ID())
	c.copyFileMu.Unlock()

	if !ok {
		return nil
	}

	return injectCopyFiles(c.logger(), ctx, c.api, string(newID), source, snapshot)
}

// DiscardCopyFiles drops a leftover snapshot for the source container.
//
// Parameters:
//   - containerID: Source container ID whose snapshot should be discarded.
func (c *client) DiscardCopyFiles(containerID types.ContainerID) {
	c.copyFileMu.Lock()
	defer c.copyFileMu.Unlock()

	c.dropCopyFileLocked(containerID)
}

// dropCopyFileLocked removes a stored snapshot and subtracts its size.
//
// Parameters:
//   - containerID: Source container ID whose snapshot should be dropped.
//
// Returns:
//   - copyFileSnapshot: The removed snapshot when present.
//   - bool: True when a snapshot was stored for containerID.
func (c *client) dropCopyFileLocked(
	containerID types.ContainerID,
) (copyFileSnapshot, bool) {
	snapshot, ok := c.copyFiles[containerID]
	if !ok {
		return copyFileSnapshot{}, false
	}

	delete(c.copyFiles, containerID)

	c.copyFileBytes -= snapshot.size()
	if c.copyFileBytes < 0 {
		c.copyFileBytes = 0
	}

	return snapshot, true
}
