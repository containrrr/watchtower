// Package git registers Git association and watcher flags.
package git

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// defaultTimeout is the static default Git operation timeout.
const defaultTimeout = 30 * time.Second

// Specs returns Git domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Git flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:    "git-enable",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_GIT_ENABLE"},
			Help:    "Use Git ref change detection as the staleness signal for associated containers (process-wide watch default)",
		},
		{
			Name:      "git-image",
			Kind:      spec.KindStringArray,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_GIT_IMAGE"},
			ListParse: spec.ListNewline,
			Help:      "Associate an image with a Git repository (repeatable). Format: image=repo[#ref][@policy]",
		},
		{
			Name:    "git-auth-token",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_AUTH_TOKEN"},
			Help:    "Git authentication token (HTTPS). Takes priority over username/password and SSH",
		},
		{
			Name:    "git-username",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_USERNAME"},
			Help:    "Git basic-auth username",
		},
		{
			Name:    "git-password",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_PASSWORD"},
			Help:    "Git basic-auth password",
		},
		{
			Name:    "git-ssh-key-path",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_SSH_KEY_PATH"},
			Help:    "Path to an SSH private key for Git clone, ls-remote, and local path project checkout",
		},
		{
			Name:    "git-ssh-known-hosts",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_SSH_KNOWN_HOSTS"},
			Help:    "Path to an SSH known_hosts file used to verify Git hosts",
		},
		{
			Name:    "git-timeout",
			Kind:    spec.KindDuration,
			Default: defaultTimeout,
			EnvKeys: []string{"WATCHTOWER_GIT_TIMEOUT"},
			Help:    "Timeout for Git network operations (e.g., 30s, 1m)",
		},
		{
			Name:    "git-ca-bundle",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_CA_BUNDLE"},
			Help:    "Path to a PEM CA bundle for Git HTTPS (clone, ls-remote, REST probes, and local path project checkout)",
		},
		{
			Name:    "git-insecure-skip-tls",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_GIT_INSECURE_SKIP_TLS"},
			Help:    "Skip TLS verification for Git HTTPS (clone, ls-remote, REST probes, and local path project checkout)",
		},
		{
			Name:    "git-compose-stash",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_GIT_COMPOSE_STASH"},
			Help:    "Save and restore local Compose project changes (for example an untracked .env) around checkout",
		},
		{
			Name:      "compose-project",
			Kind:      spec.KindStringArray,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_COMPOSE_PROJECT"},
			ListParse: spec.ListNewline,
			Help:      "Map a Compose project name to a project directory inside Watchtower (name=/path, repeatable)",
		},
		{
			Name:    "git-dockerfile",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_DOCKERFILE"},
			Help:    "Default Dockerfile path relative to the build context (empty means Dockerfile)",
		},
		{
			Name:    "git-context",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_GIT_CONTEXT"},
			Help:    "Default subdirectory of a Git URL context used as the Docker build context (empty means the repository root)",
		},
	}
}

// Register adds Git domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
