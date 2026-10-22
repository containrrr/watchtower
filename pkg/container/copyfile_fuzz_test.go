package container

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// FuzzParseCopyFileLabel verifies label parsing never panics and that accepted
// paths are unique, non-empty, and pass validateCopyFilePath.
func FuzzParseCopyFileLabel(f *testing.F) {
	f.Add("")
	f.Add("   ")
	f.Add("/app/config.yaml")
	f.Add("/app/config.yaml,/app/extra.conf")
	f.Add(" /a.conf , , /b.conf ")
	f.Add("/a.conf,/b.conf,/a.conf")
	f.Add("app/config.yaml")
	f.Add("/app/../config.yaml")
	f.Add("/")
	f.Add("/app/")
	f.Add("\\windows\\path")
	f.Add("/app/config.yaml\x00")
	f.Add(strings.Repeat("/a", 4096))
	f.Add(",,,,")
	f.Add("/a,/b,/c,/d,/e")

	f.Fuzz(func(t *testing.T, value string) {
		paths, err := parseCopyFileLabel(value)
		if err != nil {
			if !errors.Is(err, errCopyFileInvalidPath) {
				t.Fatalf("unexpected error %v for %q", err, value)
			}

			return
		}

		seen := make(map[string]struct{}, len(paths))

		for _, filePath := range paths {
			if filePath == "" {
				t.Fatalf("accepted empty path from %q", value)
			}

			if validateCopyFilePath(filePath) != nil {
				t.Fatalf("accepted invalid path %q from %q", filePath, value)
			}

			if _, ok := seen[filePath]; ok {
				t.Fatalf("duplicate path %q from %q", filePath, value)
			}

			seen[filePath] = struct{}{}
		}
	})
}

// FuzzValidateCopyFilePath verifies path validation never panics and accepted
// paths are clean absolute file paths without parent traversal.
func FuzzValidateCopyFilePath(f *testing.F) {
	f.Add("/app/config.yaml")
	f.Add("")
	f.Add("/")
	f.Add("/app/")
	f.Add("app/config.yaml")
	f.Add("/app/../config.yaml")
	f.Add("/app/./config.yaml")
	f.Add("/app//config.yaml")
	f.Add("C:\\app\\config.yaml")
	f.Add("/tmp/../etc/passwd")
	f.Add("/app/config.yaml\x00")
	f.Add(strings.Repeat("a", 8192))

	f.Fuzz(func(t *testing.T, filePath string) {
		err := validateCopyFilePath(filePath)
		if err != nil {
			if !errors.Is(err, errCopyFileInvalidPath) {
				t.Fatalf("unexpected error %v for %q", err, filePath)
			}

			return
		}

		if filePath == "" || filePath == "/" {
			t.Fatalf("accepted empty or root path %q", filePath)
		}

		if !path.IsAbs(filePath) || strings.Contains(filePath, "\\") {
			t.Fatalf("accepted non-absolute path %q", filePath)
		}

		if path.Clean(filePath) != filePath {
			t.Fatalf("accepted unclean path %q", filePath)
		}

		for elem := range strings.SplitSeq(filePath, "/") {
			if elem == ".." {
				t.Fatalf("accepted parent traversal in %q", filePath)
			}
		}
	})
}

// FuzzSanitizeCopyFileArchive verifies tar sanitization never panics and that
// accepted archives are a single regular file named as the full destination path
// with setuid, setgid, and sticky bits cleared.
func FuzzSanitizeCopyFileArchive(f *testing.F) {
	f.Add(seedCopyFileTar("config.yaml", []byte("setting=1"), 0o444), "/app/config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte("setting=1"), 0o4755), "/app/config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte("setting=1"), 0o644), "/config/config.yml")
	f.Add(seedCopyFileTar("../config.yaml", []byte("x"), 0o644), "/app/config.yaml")
	f.Add(seedCopyFileTar("extra.yaml", []byte("x"), 0o644), "/app/config.yaml")
	f.Add(seedCopyFileSymlink("config.yaml", "/tmp/other"), "/app/config.yaml")
	f.Add([]byte(""), "/app/config.yaml")
	f.Add([]byte("not a tar"), "/app/config.yaml")
	f.Add([]byte{0x1f, 0x8b}, "/app/config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte(""), 0o644), "/app/config.yaml")
	f.Add(seedCopyFileTar("./config.yaml", []byte("ok"), 0o600), "/app/config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte("x"), 0o644), "../config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte("x"), 0o644), "config.yaml")
	f.Add(seedCopyFileTar("config.yaml", []byte("x"), 0o644), "/")

	f.Fuzz(func(t *testing.T, raw []byte, destPath string) {
		if len(raw) > maxCopyFileTarBytes*2 {
			raw = raw[:maxCopyFileTarBytes*2]
		}

		got, err := sanitizeCopyFileArchive(raw, destPath)
		if err != nil {
			switch {
			case errors.Is(err, errCopyFileUnsafeArchive),
				errors.Is(err, errCopyFileSymlink),
				errors.Is(err, errCopyFileTooLarge),
				errors.Is(err, errCopyFileInvalidPath),
				errors.Is(err, errCopyFileSnapshotFailed):
				return
			default:
				t.Fatalf("unexpected error %v", err)
			}
		}

		name, body, mode := mustReadSingleTarFile(t, got)

		if name != destPath {
			t.Fatalf("sanitized name %q want %q", name, destPath)
		}

		if strings.Contains(name, "..") {
			t.Fatalf("sanitized name contains parent traversal: %q", name)
		}

		if mode&^unixFilePermMask != 0 {
			t.Fatalf("sanitized mode %#o still has special bits", mode)
		}

		if int64(len(body)) > maxCopyFileBytes {
			t.Fatalf("sanitized body length %d exceeds cap", len(body))
		}
	})
}

// FuzzCopyFileTarNameOK verifies tar member name matching never panics and that
// a true result has no parent traversal and a matching basename.
func FuzzCopyFileTarNameOK(f *testing.F) {
	f.Add("config.yaml", "config.yaml")
	f.Add("./config.yaml", "config.yaml")
	f.Add("../config.yaml", "config.yaml")
	f.Add("/app/config.yaml", "config.yaml")
	f.Add("", "config.yaml")
	f.Add("config.yaml", "")
	f.Add("extra.yaml", "config.yaml")
	f.Add("..", "config.yaml")

	f.Fuzz(func(t *testing.T, name, base string) {
		ok := copyFileTarNameOK(name, base)
		if !ok {
			return
		}

		if name == "" || strings.Contains(name, "..") {
			t.Fatalf("accepted empty or traversal name %q", name)
		}

		if path.Base(path.Clean(name)) != base {
			t.Fatalf("accepted name %q for base %q", name, base)
		}
	})
}

// FuzzPathIsUnderMount verifies mount prefix checks never panic and that a true
// result means the cleaned file path is the mount or a descendant.
func FuzzPathIsUnderMount(f *testing.F) {
	f.Add("/app/config.yaml", "/app")
	f.Add("/app/config.yaml", "/app/config.yaml")
	f.Add("/app/config.yaml", "/data")
	f.Add("/app/config.yaml", "")
	f.Add("/app/config.yaml", ".")
	f.Add("/app-other/file", "/app")
	f.Add("/", "/")
	f.Add("/app/config.yaml", "/")

	f.Fuzz(func(t *testing.T, filePath, mountDest string) {
		if !pathIsUnderMount(filePath, mountDest) {
			return
		}

		cleanedFile := path.Clean(filePath)
		cleanedMount := path.Clean(mountDest)

		if cleanedMount == "" || cleanedMount == "." {
			t.Fatalf("matched empty mount %q for %q", mountDest, filePath)
		}

		if cleanedFile == cleanedMount {
			return
		}

		if !strings.HasPrefix(cleanedFile, cleanedMount+"/") {
			t.Fatalf("matched %q under %q without prefix", filePath, mountDest)
		}
	})
}

// FuzzPathHasMount verifies mount equality checks never panic and that a true
// result means the cleaned file path and mount destination are identical.
func FuzzPathHasMount(f *testing.F) {
	f.Add("/app/config.yaml", "/app/config.yaml")
	f.Add("/app/config.yaml", "/app")
	f.Add("/app/./config.yaml", "/app/config.yaml")
	f.Add("/app/config.yaml", "")
	f.Add("/app/config.yaml", ".")
	f.Add("/", "/")
	f.Add("/app/config.yaml", "/app/config.yaml/")

	f.Fuzz(func(t *testing.T, filePath, mountDest string) {
		if !pathHasMount(filePath, mountDest) {
			return
		}

		cleanedFile := path.Clean(filePath)
		cleanedMount := path.Clean(mountDest)

		if cleanedMount == "" || cleanedMount == "." {
			t.Fatalf("matched empty mount %q for %q", mountDest, filePath)
		}

		if cleanedFile != cleanedMount {
			t.Fatalf("matched %q against %q without equality", filePath, mountDest)
		}
	})
}

// FuzzBindDestination verifies bind parsing never panics.
func FuzzBindDestination(f *testing.F) {
	f.Add("/host/config.yaml:/app/config.yaml:ro")
	f.Add("/host/data:/data")
	f.Add("nocolon")
	f.Add("")
	f.Add("::")
	f.Add("/src:/dst:rw,z")

	f.Fuzz(func(t *testing.T, bind string) {
		dest := bindDestination(bind)
		if dest == "" {
			return
		}

		if !strings.Contains(bind, ":") {
			t.Fatalf("non-empty dest %q from bind without colon %q", dest, bind)
		}
	})
}

// FuzzBindIsReadOnly verifies read-only bind option detection never panics and
// that a true result means the bind options contain the ro flag.
func FuzzBindIsReadOnly(f *testing.F) {
	f.Add("/host/config.yaml:/app/config.yaml:ro")
	f.Add("/host/data:/data")
	f.Add("/host/data:/data:rw,z")
	f.Add("/host/data:/data:z,ro")
	f.Add("nocolon")
	f.Add("::")
	f.Add("")

	f.Fuzz(func(t *testing.T, bind string) {
		if !bindIsReadOnly(bind) {
			return
		}

		parts := strings.Split(bind, ":")
		if len(parts) < bindStringMinParts+1 {
			t.Fatalf("read-only bind %q has no options field", bind)
		}

		if !slices.Contains(strings.Split(parts[bindStringMinParts], ","), bindReadOnlyOption) {
			t.Fatalf("bind %q reported read-only without ro option", bind)
		}
	})
}

// FuzzStoreCopyFileSnapshot verifies the in-flight byte cap is never exceeded
// and that discard returns the store to empty for that id.
func FuzzStoreCopyFileSnapshot(f *testing.F) {
	f.Add("source-id", []byte("tar"))
	f.Add("", []byte{})
	f.Add("id", bytes.Repeat([]byte("a"), 64))

	f.Fuzz(func(t *testing.T, id string, archive []byte) {
		if len(archive) > maxCopyFileStoreBytes+1024 {
			archive = archive[:maxCopyFileStoreBytes+1024]
		}

		cli := &client{
			copyFiles: make(map[types.ContainerID]copyFileSnapshot),
		}
		containerID := types.ContainerID(id)
		snapshot := copyFileSnapshot{files: []copyFileEntry{{
			target:  "/app/config.yaml",
			archive: archive,
		}}}

		err := cli.storeCopyFileSnapshot(containerID, snapshot)
		if err != nil {
			if !errors.Is(err, errCopyFileStoreFull) {
				t.Fatalf("unexpected error %v", err)
			}

			if cli.copyFileBytes > maxCopyFileStoreBytes {
				t.Fatalf("store bytes %d exceed cap after error", cli.copyFileBytes)
			}

			return
		}

		if cli.copyFileBytes > maxCopyFileStoreBytes {
			t.Fatalf("store bytes %d exceed cap", cli.copyFileBytes)
		}

		if cli.copyFileBytes != snapshot.size() {
			t.Fatalf("store bytes %d want %d", cli.copyFileBytes, snapshot.size())
		}

		cli.DiscardCopyFiles(containerID)

		if cli.copyFileBytes != 0 {
			t.Fatalf("store bytes %d after discard", cli.copyFileBytes)
		}

		if _, ok := cli.copyFiles[containerID]; ok {
			t.Fatalf("snapshot still stored for %q", id)
		}
	})
}

func seedCopyFileTar(name string, body []byte, mode int64) []byte {
	var buf bytes.Buffer

	writer := tar.NewWriter(&buf)
	_ = writer.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     int64(len(body)),
		Mode:     mode,
	})
	_, _ = writer.Write(body)
	_ = writer.Close()

	return buf.Bytes()
}

func seedCopyFileSymlink(name, target string) []byte {
	var buf bytes.Buffer

	writer := tar.NewWriter(&buf)
	_ = writer.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     name,
		Linkname: target,
		Mode:     0o777,
	})
	_ = writer.Close()

	return buf.Bytes()
}

func mustReadSingleTarFile(t *testing.T, archive []byte) (string, []byte, int64) {
	t.Helper()

	reader := tar.NewReader(bytes.NewReader(archive))

	header, err := reader.Next()
	if err != nil {
		t.Fatalf("read sanitized tar header: %v", err)
	}

	if header.Typeflag != tar.TypeReg {
		t.Fatalf("sanitized type %q want regular file", header.Typeflag)
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read sanitized tar body: %v", err)
	}

	_, err = reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("sanitized tar has extra members: %v", err)
	}

	return header.Name, body, header.Mode
}
