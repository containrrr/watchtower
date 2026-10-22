package cmd

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
)

// envKeyPattern matches environment variable names that form part of the
// user-facing configuration surface.
var envKeyPattern = regexp.MustCompile(`^((WATCHTOWER|WT_ORCHESTRATOR|DOCKER)_[A-Z0-9_]+|REPO_USER|REPO_PASS|NO_COLOR)$`)

// labelPattern matches Watchtower container label keys.
var labelPattern = regexp.MustCompile(`^com\.centurylinklabs\.watchtower\.[a-z0-9.-]+$`)

// renderHelp returns the help output of cmd as users see it from --help.
//
// It reproduces what Cobra and Execute set up before running: the default help
// and completion subcommands, the --help flag, and a Run handler on the root
// command, which makes Cobra print the "[flags]" usage line.
func renderHelp(t *testing.T, root, cmd *cobra.Command) []byte {
	t.Helper()

	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	cmd.InitDefaultHelpFlag()

	if root.Run == nil {
		root.Run = func(*cobra.Command, []string) {}

		t.Cleanup(func() { root.Run = nil })
	}

	var out bytes.Buffer

	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })

	require.NoError(t, cmd.Help())

	return out.Bytes()
}

// TestRootHelp snapshots the root command's --help output.
func TestRootHelp(t *testing.T) {
	golden.Assert(t, "help-root", renderHelp(t, rootCmd, rootCmd))
}

// TestNotifyUpgradeHelp snapshots the notify-upgrade subcommand's --help output.
func TestNotifyUpgradeHelp(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"notify-upgrade"})
	require.NoError(t, err)

	golden.Assert(t, "help-notify-upgrade", renderHelp(t, rootCmd, sub))
}

// TestConfigurationSurfaceInventory snapshots every environment variable name and
// container label key that appears as a string literal in production code.
//
// It complements the flag manifest by catching settings read directly from the
// environment, such as keys with no corresponding flag.
func TestConfigurationSurfaceInventory(t *testing.T) {
	envKeys := map[string]struct{}{}
	labels := map[string]struct{}{}

	moduleRoot := ".."

	for _, dir := range []string{"cmd", "internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join(moduleRoot, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				if entry.Name() == "mocks" || entry.Name() == "testdata" || entry.Name() == "testutil" {
					return filepath.SkipDir
				}

				return nil
			}

			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			collectLiterals(t, path, envKeys, labels)

			return nil
		})
		require.NoError(t, err)
	}

	collectLiterals(t, filepath.Join(moduleRoot, "main.go"), envKeys, labels)

	var builder strings.Builder

	builder.WriteString("environment variables:\n")

	for _, key := range sortedKeys(envKeys) {
		builder.WriteString("  " + key + "\n")
	}

	builder.WriteString("container labels:\n")

	for _, label := range sortedKeys(labels) {
		builder.WriteString("  " + label + "\n")
	}

	golden.Assert(t, "configuration-surface", []byte(builder.String()))
}

// collectLiterals records the env keys and labels found in the string literals of one file.
func collectLiterals(t *testing.T, path string, envKeys, labels map[string]struct{}) {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	require.NoError(t, err)

	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}

		switch {
		case envKeyPattern.MatchString(value):
			envKeys[value] = struct{}{}
		case labelPattern.MatchString(value):
			labels[value] = struct{}{}
		}

		return true
	})
}

// sortedKeys returns the keys of set in ascending order.
func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}
