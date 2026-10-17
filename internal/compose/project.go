package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectRef is a Compose project Watchtower can apply.
type ProjectRef struct {
	Name        string
	Dir         string
	ConfigFiles []string
}

// ResolveProjectDir returns the Compose project the user opted into for this container.
//
// Local Compose apply is explicit:
//  1. com.centurylinklabs.watchtower.compose-dir
//  2. --compose-project / WATCHTOWER_COMPOSE_PROJECT keyed by Compose project name
//
// Compose's own working_dir label is not used. An explicit path that is missing
// or has no compose file is an error. Watchtower does not fall back to a Git URL context.
//
// Parameters:
//   - labels: Container labels.
//   - projectDirs: Map of Compose project name to a path inside Watchtower.
//
// Returns:
//   - ProjectRef: Directory when the user opted in. Dir is empty when they did not.
//   - error: Non-nil when an opted-in path is not a readable Compose project.
func ResolveProjectDir(labels, projectDirs map[string]string) (ProjectRef, error) {
	name := GetProjectName(labels)

	if dir := labelValue(labels, WatchtowerComposeDirLabel); dir != "" {
		return projectRefOrErr(name, dir, labels)
	}

	if name != "" {
		if dir := strings.TrimSpace(projectDirs[name]); dir != "" {
			return projectRefOrErr(name, dir, labels)
		}
	}

	return ProjectRef{}, nil
}

// projectRefOrErr returns a project ref when dir is a readable Compose project.
//
// Parameters:
//   - name: Compose project name.
//   - dir: Path inside Watchtower.
//   - labels: Container labels for optional compose file list.
//
// Returns:
//   - ProjectRef: Directory and optional compose files.
//   - error: ErrProjectDir when dir is missing or has no compose file.
func projectRefOrErr(name, dir string, labels map[string]string) (ProjectRef, error) {
	if !readableProjectDir(dir) {
		return ProjectRef{}, fmt.Errorf("%w: %s", ErrProjectDir, dir)
	}

	files, err := configFiles(labels, dir)
	if err != nil {
		return ProjectRef{}, err
	}

	return ProjectRef{
		Name:        name,
		Dir:         dir,
		ConfigFiles: files,
	}, nil
}

// labelValue returns a trimmed label, or empty when labels is nil.
//
// Parameters:
//   - labels: Container labels.
//   - key: Label key.
//
// Returns:
//   - string: Label value, or empty.
func labelValue(labels map[string]string, key string) string {
	if labels == nil {
		return ""
	}

	return strings.TrimSpace(labels[key])
}

// configFiles splits Compose's config_files label into paths inside dir.
// Absolute host paths under Compose's working_dir are rebased onto dir.
//
// Parameters:
//   - labels: Container labels.
//   - dir: Project directory used to resolve relative paths.
//
// Returns:
//   - []string: Compose file paths, or nil when the label is absent.
//   - error: errConfigFile when an entry leaves dir.
func configFiles(labels map[string]string, dir string) ([]string, error) {
	raw := labelValue(labels, ComposeConfigFilesLabel)
	if raw == "" {
		return nil, nil
	}

	var out []string

	workingDir := labelValue(labels, ComposeWorkingDirLabel)

	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var (
			cleaned string
			err     error
		)
		if workingDir == "" {
			cleaned, err = containedConfigFile(dir, part)
		} else {
			cleaned, err = resolveConfigFile(dir, workingDir, part)
		}

		if err != nil {
			return nil, err
		}

		out = append(out, cleaned)
	}

	return out, nil
}

// containedConfigFile resolves part and rejects paths outside dir.
//
// Parameters:
//   - dir: Project directory.
//   - part: One config_files entry.
//
// Returns:
//   - string: Absolute or joined path inside dir.
//   - error: errConfigFile when part escapes dir.
func containedConfigFile(dir, part string) (string, error) {
	return resolveConfigFile(dir, "", part)
}

// resolveConfigFile resolves a config path within the mounted project or maps
// an absolute host path from workingDir into the mounted project.
//
// Parameters:
//   - dir: Project directory inside Watchtower.
//   - workingDir: Optional Compose working directory on the host.
//   - part: One config_files entry.
//
// Returns:
//   - string: Resolved path inside dir.
//   - error: errConfigFile when the entry is invalid or escapes the project.
func resolveConfigFile(dir, workingDir, part string) (string, error) {
	mountDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return "", fmt.Errorf("%w: %s", errConfigFile, dir)
	}

	if !filepath.IsAbs(part) {
		if !filepath.IsLocal(part) {
			return "", fmt.Errorf("%w: %s", errConfigFile, part)
		}

		cleaned, err := filepath.Abs(filepath.Join(mountDir, part))
		if err != nil {
			return "", fmt.Errorf("%w: %s", errConfigFile, part)
		}

		return checkedConfigFile(mountDir, cleaned)
	}

	hostPath, err := filepath.Abs(filepath.Clean(part))
	if err != nil {
		return "", fmt.Errorf("%w: %s", errConfigFile, part)
	}

	if pathWithin(mountDir, hostPath) {
		return checkedConfigFile(mountDir, hostPath)
	}

	if workingDir == "" || !filepath.IsAbs(workingDir) {
		return "", fmt.Errorf("%w: %s", errConfigFile, part)
	}

	hostRoot, err := filepath.Abs(filepath.Clean(workingDir))
	if err != nil {
		return "", fmt.Errorf("%w: %s", errConfigFile, workingDir)
	}

	rel, err := filepath.Rel(hostRoot, hostPath)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%w: %s", errConfigFile, part)
	}

	mountPath := filepath.Join(mountDir, rel)

	return checkedConfigFile(mountDir, mountPath)
}

// checkedConfigFile verifies that path remains inside dir after symlink resolution.
//
// Parameters:
//   - dir: Project directory.
//   - path: Candidate compose file.
//
// Returns:
//   - string: The verified path.
//   - error: errConfigFile when the path resolves outside dir.
func checkedConfigFile(dir, path string) (string, error) {
	err := symlinkEscapes(dir, path)
	if err != nil {
		return "", err
	}

	return path, nil
}

// pathWithin reports whether path is within dir according to lexical path comparison.
//
// Parameters:
//   - dir: Container project directory.
//   - path: Path to compare.
//
// Returns:
//   - bool: True when path is local to dir.
func pathWithin(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)

	return err == nil && filepath.IsLocal(rel)
}

// symlinkEscapes reports whether path exists and resolves outside dir.
//
// A missing path is not an escape. Compose reports that later.
//
// Parameters:
//   - dir: Project directory.
//   - path: Candidate compose file.
//
// Returns:
//   - error: errConfigFile when the resolved path leaves dir.
func symlinkEscapes(dir, path string) error {
	_, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("%w: %s", errConfigFile, path)
	}

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("%w: %s", errConfigFile, dir)
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("%w: %s", errConfigFile, path)
	}

	rel, err := filepath.Rel(realDir, realPath)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("%w: %s", errConfigFile, path)
	}

	return nil
}

// readableProjectDir reports whether dir exists and contains a compose file.
//
// Parameters:
//   - dir: Candidate project directory.
//
// Returns:
//   - bool: True when a compose.yaml or docker-compose.yaml is present.
func readableProjectDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}

	return false
}

// fileExists reports whether path is a regular file.
//
// Parameters:
//   - path: Filesystem path.
//
// Returns:
//   - bool: True when the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}
