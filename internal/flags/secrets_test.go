package flags

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/logging"
)

// secretsFatalHelperEnv selects a GetSecretsFromFiles fatal case when the test
// binary is re-executed as a subprocess. Empty means the parent process.
const secretsFatalHelperEnv = "WATCHTOWER_FLAGS_SECRETS_FATAL_HELPER"

// newSecretsCommand returns a command with every flag registered, as in
// production, parsed from args.
func newSecretsCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()

	cmd := new(cobra.Command)

	SetDefaults()
	RegisterAll(cmd)
	require.NoError(t, cmd.ParseFlags(args))

	return cmd
}

// writeSecretFile writes content to a new file in a temporary directory and
// returns its path.
func writeSecretFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// skipOnWindows skips tests that rely on Unix file system semantics, such as
// stat errors other than "not exist" and reading a directory as a file.
func skipOnWindows(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("relies on Unix file system semantics")
	}
}

// TestGetSecretFromFile_FileErrors verifies that a value that looks like a file
// path but cannot be read returns an error and leaves the flag unchanged.
func TestGetSecretFromFile_FileErrors(t *testing.T) {
	skipOnWindows(t)

	regularFile := writeSecretFile(t, "secret")

	tests := []struct {
		name    string
		flag    string
		value   string
		wantErr error
	}{
		{
			// Stat fails with "not a directory", which is not "not exist", so
			// the value is treated as a file path and opening it fails.
			name:    "list secret below a regular file",
			flag:    "notification-url",
			value:   filepath.Join(regularFile, "child"),
			wantErr: errOpenFileFailed,
		},
		{
			name:    "list secret naming a directory",
			flag:    "notification-url",
			value:   t.TempDir(),
			wantErr: errReadFileFailed,
		},
		{
			name:    "string secret below a regular file",
			flag:    "http-api-token",
			value:   filepath.Join(regularFile, "child"),
			wantErr: errReadFileFailed,
		},
		{
			name:    "string secret naming a directory",
			flag:    "http-api-token",
			value:   t.TempDir(),
			wantErr: errReadFileFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSecretsCommand(t, "--"+tt.flag, tt.value)
			flags := cmd.PersistentFlags()

			err := getSecretFromFile(testLogger(), flags, tt.flag)
			require.ErrorIs(t, err, tt.wantErr)

			flag := flags.Lookup(tt.flag)
			if tt.flag == "notification-url" {
				values, getErr := flags.GetStringArray(tt.flag)
				require.NoError(t, getErr)
				assert.Equal(t, []string{tt.value}, values)
			} else {
				assert.Equal(t, tt.value, flag.Value.String())
			}
		})
	}
}

// isFileOpen reports whether the test process holds a descriptor for path.
func isFileOpen(t *testing.T, path string) bool {
	t.Helper()

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	for _, entry := range entries {
		target, linkErr := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if linkErr == nil && target == path {
			return true
		}
	}

	return false
}

// TestGetSecretFromFile_ClosesEachListFile verifies that each file of a list
// secret is closed before the next one is opened. The second file is a FIFO,
// so the read waits for the test to write to it, and the test checks the
// open descriptors while it waits.
func TestGetSecretFromFile_ClosesEachListFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads open descriptors from /proc/self/fd")
	}

	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("needs mkfifo")
	}

	first := writeSecretFile(t, "logger://\n")
	fifo := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, exec.CommandContext(t.Context(), mkfifo, fifo).Run())

	flags := newSecretsCommand(t, "--notification-url", first, "--notification-url", fifo).PersistentFlags()

	done := make(chan error, 1)

	go func() {
		done <- getSecretFromFile(testLogger(), flags, "notification-url")
	}()

	// A non-blocking open for writing succeeds only once the FIFO is opened
	// for reading, which happens after the first file has been read.
	var writer *os.File

	require.Eventually(t, func() bool {
		var openErr error

		writer, openErr = os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)

		return openErr == nil
	}, 10*time.Second, 10*time.Millisecond, "the FIFO is opened for reading")

	assert.False(t, isFileOpen(t, first), "the first file is closed before the next one is opened")

	_, err = writer.WriteString("mock://\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, <-done)

	values, err := flags.GetStringArray("notification-url")
	require.NoError(t, err)
	assert.Equal(t, []string{"logger://", "mock://"}, values)
}

// TestGetSecretFromFile_NotificationURLRules verifies how each line of a
// notification URL file is validated. A file is rejected as a whole when any
// of its lines is not an accepted URL.
func TestGetSecretFromFile_NotificationURLRules(t *testing.T) {
	tests := []struct {
		line string
		// content is the file content when it is more than the line itself.
		content string
		wantErr bool
	}{
		{line: "discord://token@webhookid"},
		{line: "generic+https://example.com/hook"},
		{line: "smtp://user:pass@host:25/?to=a@example.com"},
		{line: "logger://"},
		{line: "mock://"},
		{line: "not-a-url", wantErr: true},
		{line: "discord:token@webhookid", wantErr: true},
		{line: "://missing-scheme", wantErr: true},
		{line: "discord://", wantErr: true},
		{
			line:    "valid line then invalid line",
			content: "discord://token@webhookid\nnot-a-url\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			content := tt.content
			if content == "" {
				content = tt.line + "\n"
			}

			path := writeSecretFile(t, content)
			cmd := newSecretsCommand(t, "--notification-url", path)

			err := getSecretFromFile(testLogger(), cmd.PersistentFlags(), "notification-url")

			urls, getErr := cmd.PersistentFlags().GetStringArray("notification-url")
			require.NoError(t, getErr)

			if tt.wantErr {
				require.ErrorIs(t, err, errInvalidSecretURL)
				assert.Equal(t, []string{path}, urls, "a rejected file must leave the flag unchanged")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, []string{tt.line}, urls)
		})
	}
}

// TestGetSecretFromFile_StringSecrets verifies that string secrets take the
// whole file, trimmed, without URL validation, and that values which are not
// existing files are kept as literals.
func TestGetSecretFromFile_StringSecrets(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		content string
		literal string
		want    string
	}{
		{
			name:    "surrounding whitespace is trimmed",
			flag:    "http-api-token",
			content: "\n  token-value  \n\n",
			want:    "token-value",
		},
		{
			name:    "inner lines are kept",
			flag:    "git-password",
			content: "line one\nline two\n",
			want:    "line one\nline two",
		},
		{
			name:    "comment lines are kept",
			flag:    "git-auth-token",
			content: "# not a comment for string secrets\n",
			want:    "# not a comment for string secrets",
		},
		{
			name:    "content is not validated as a URL",
			flag:    "http-api-events-token",
			content: "not-a-url",
			want:    "not-a-url",
		},
		{
			name:    "missing file is a literal value",
			flag:    "http-api-token",
			literal: "missing-secret-file",
			want:    "missing-secret-file",
		},
		{
			name:    "value with a scheme is a literal value",
			flag:    "notification-slack-hook-url",
			literal: "https://hooks.example.com/services/abc",
			want:    "https://hooks.example.com/services/abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.literal
			if value == "" {
				value = writeSecretFile(t, tt.content)
			}

			cmd := newSecretsCommand(t, "--"+tt.flag, value)
			flags := cmd.PersistentFlags()

			require.NoError(t, getSecretFromFile(testLogger(), flags, tt.flag))
			assert.Equal(t, tt.want, flags.Lookup(tt.flag).Value.String())
		})
	}
}

// TestGetSecretFromFile_ChangedState records which flags are marked as set
// after secret expansion. List secrets are always marked as set, even when no
// value was given and nothing was read. String secrets keep the state the
// command line gave them unless they are read from a file.
func TestGetSecretFromFile_ChangedState(t *testing.T) {
	tests := []struct {
		name        string
		flag        string
		args        []string
		wantChanged bool
	}{
		{
			name:        "unset list secret",
			flag:        "notification-url",
			wantChanged: true,
		},
		{
			name:        "list secret with a literal URL",
			flag:        "notification-url",
			args:        []string{"--notification-url", "discord://token@webhookid"},
			wantChanged: true,
		},
		{
			name:        "unset string secret",
			flag:        "http-api-token",
			wantChanged: false,
		},
		{
			name:        "string secret with a literal value",
			flag:        "http-api-token",
			args:        []string{"--http-api-token", "literal-token"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSecretsCommand(t, tt.args...)
			flags := cmd.PersistentFlags()

			require.NoError(t, getSecretFromFile(testLogger(), flags, tt.flag))
			assert.Equal(t, tt.wantChanged, flags.Lookup(tt.flag).Changed)
		})
	}
}

// TestGetSecretsFromFiles_ReadsEverySecret verifies which flags are read from
// files. Every registered string and list flag points at its own file, and the
// flags whose values were replaced by file contents must be exactly the
// expected secrets, so a secret added to or removed from the list is noticed.
func TestGetSecretsFromFiles_ReadsEverySecret(t *testing.T) {
	want := []string{
		"git-auth-token",
		"git-password",
		"http-api-events-token",
		"http-api-token",
		"notification-email-server-password",
		"notification-gotify-token",
		"notification-msteams-hook",
		"notification-slack-hook-url",
		"notification-url",
	}

	cmd := newSecretsCommand(t)
	flags := cmd.PersistentFlags()

	// Each flag's file holds a value naming the flag. List flags get a URL so
	// the notification URL rules accept it.
	contents := map[string]string{}

	flags.VisitAll(func(flag *pflag.Flag) {
		var content string

		switch flag.Value.Type() {
		case "string":
			content = "content-of-" + flag.Name
		case "stringArray", "stringSlice":
			content = "generic://example.com/" + flag.Name
		default:
			return
		}

		contents[flag.Name] = content
		require.NoError(t, flags.Set(flag.Name, writeSecretFile(t, content+"\n")))
	})

	GetSecretsFromFiles(testLogger(), cmd)

	var read []string

	for name, content := range contents {
		flag := flags.Lookup(name)

		value := flag.Value.String()
		if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
			value = strings.Join(sliceValue.GetSlice(), ",")
		}

		if value == content {
			read = append(read, name)
		}
	}

	slices.Sort(read)
	assert.Equal(t, want, read)
}

// TestGetSecretsFromFiles_FatalOnInvalidSecret verifies that a secret file that
// fails to load ends the process with a non-zero exit code and names the flag.
func TestGetSecretsFromFiles_FatalOnInvalidSecret(t *testing.T) {
	if path := os.Getenv(secretsFatalHelperEnv); path != "" {
		cmd := newSecretsCommand(t, "--notification-url", path)
		GetSecretsFromFiles(testLoggerAt(os.Stderr, logging.InfoLevel), cmd)

		// GetSecretsFromFiles must exit. Reaching here means it did not.
		os.Exit(0)
	}

	path := writeSecretFile(t, "not-a-url\n")

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGetSecretsFromFiles_FatalOnInvalidSecret$", "-test.v=false")

	cmd.Env = append(os.Environ(), secretsFatalHelperEnv+"="+path)

	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "GetSecretsFromFiles must exit; output:\n%s", out)
	assert.NotEqual(t, 0, exitErr.ExitCode())
	assert.Contains(t, string(out), "Failed to load secret from file")
	assert.Contains(t, string(out), "notification-url")
}
