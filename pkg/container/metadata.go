package container

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Watchtower-specific labels identify containers managed by Watchtower and their configurations.
const (
	// watchtowerLabel marks a container as the Watchtower instance itself when set to "true".
	watchtowerLabel = "com.centurylinklabs.watchtower"
	// signalLabel specifies a custom stop signal for the container (e.g., "SIGTERM").
	signalLabel = "com.centurylinklabs.watchtower.stop-signal"
	// enableLabel indicates whether Watchtower should manage this container (true/false).
	enableLabel = "com.centurylinklabs.watchtower.enable"
	// monitorOnlyLabel flags the container for monitoring only, without updates (true/false).
	monitorOnlyLabel = "com.centurylinklabs.watchtower.monitor-only"
	// noPullLabel prevents Watchtower from pulling a new image for this container (true/false).
	noPullLabel = "com.centurylinklabs.watchtower.no-pull"
	// dependsOnLabel lists container names this container depends on, comma-separated.
	dependsOnLabel = "com.centurylinklabs.watchtower.depends-on"
	// ContainerChainLabel accumulates container IDs across Watchtower self-updates, comma-separated.
	ContainerChainLabel = "com.centurylinklabs.watchtower.container-chain"
	// zodiacLabel stores the original image name for Zodiac compatibility.
	zodiacLabel = "com.centurylinklabs.zodiac.original-image"
	// scope defines a unique monitoring scope for this Watchtower instance.
	scope = "com.centurylinklabs.watchtower.scope"
	// OrchestratorLabel identifies ephemeral orchestrator containers used during self-update.
	OrchestratorLabel = "com.centurylinklabs.watchtower.ephemeral-orchestrator"
	// cooldownDelayLabel sets the minimum image age before updating this container.
	// Accepts duration strings (e.g., "24h", "3d", "1w", "0" to disable).
	cooldownDelayLabel = "com.centurylinklabs.watchtower.cooldown-delay"
	// copyFileLabel lists in-container file paths to copy during recreation, comma-separated.
	copyFileLabel = "com.centurylinklabs.watchtower.copy-file"
)

// Lifecycle hook labels configure commands executed during container update phases.
const (
	// preCheckLabel specifies a command to run before checking for updates.
	preCheckLabel = "com.centurylinklabs.watchtower.lifecycle.pre-check"
	// postCheckLabel specifies a command to run after checking for updates.
	postCheckLabel = "com.centurylinklabs.watchtower.lifecycle.post-check"
	// preUpdateLabel specifies a command to run before updating the container.
	preUpdateLabel = "com.centurylinklabs.watchtower.lifecycle.pre-update"
	// postUpdateLabel specifies a command to run after updating the container.
	postUpdateLabel = "com.centurylinklabs.watchtower.lifecycle.post-update"
	// preUpdateTimeoutLabel sets the timeout (in minutes) for the pre-update command.
	preUpdateTimeoutLabel = "com.centurylinklabs.watchtower.lifecycle.pre-update-timeout"
	// postUpdateTimeoutLabel sets the timeout (in minutes) for the post-update command.
	postUpdateTimeoutLabel = "com.centurylinklabs.watchtower.lifecycle.post-update-timeout"
	// lifecycleUIDLabel specifies the UID to run lifecycle hooks as.
	lifecycleUIDLabel = "com.centurylinklabs.watchtower.lifecycle.uid"
	// lifecycleGIDLabel specifies the GID to run lifecycle hooks as.
	lifecycleGIDLabel = "com.centurylinklabs.watchtower.lifecycle.gid"
)

// GetLifecyclePreCheckCommand returns the pre-check command from labels.
//
// Returns:
//   - string: Pre-check command or empty if unset.
func (c *Container) GetLifecyclePreCheckCommand() string {
	return c.getLabelValueOrEmpty(preCheckLabel)
}

// GetLifecyclePostCheckCommand returns the post-check command from labels.
//
// Returns:
//   - string: Post-check command or empty if unset.
func (c *Container) GetLifecyclePostCheckCommand() string {
	return c.getLabelValueOrEmpty(postCheckLabel)
}

// GetLifecyclePreUpdateCommand returns the pre-update command from labels.
//
// Returns:
//   - string: Pre-update command or empty if unset.
func (c *Container) GetLifecyclePreUpdateCommand() string {
	return c.getLabelValueOrEmpty(preUpdateLabel)
}

// GetLifecyclePostUpdateCommand returns the post-update command from labels.
//
// Returns:
//   - string: Post-update command or empty if unset.
func (c *Container) GetLifecyclePostUpdateCommand() string {
	return c.getLabelValueOrEmpty(postUpdateLabel)
}

// PreUpdateTimeout returns the pre-update command timeout in minutes.
//
// It defaults to 1 minute if unset or invalid.
// 0 allows indefinite execution.
//
// Returns:
//   - int: Timeout in minutes.
func (c *Container) PreUpdateTimeout() int {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	val := c.getLabelValueOrEmpty(preUpdateTimeoutLabel)

	// Use default if label is unset.
	if val == "" {
		clog.Debug().
			Str("label", preUpdateTimeoutLabel).
			Msg("Pre-update timeout not set, using default")

		return 1
	}

	// Parse timeout value.
	minutes, err := strconv.Atoi(val)
	if err != nil {
		clog.Warn().
			Err(err).
			Str("label", preUpdateTimeoutLabel).
			Str("value", val).
			Msg("Invalid pre-update timeout value, using default")

		return 1
	}

	clog.Debug().
		Str("label", preUpdateTimeoutLabel).
		Int("minutes", minutes).
		Msg("Retrieved pre-update timeout")

	return minutes
}

// PostUpdateTimeout returns the post-update command timeout in minutes.
//
// It defaults to 1 minute if unset or invalid.
// 0 allows indefinite execution.
//
// Returns:
//   - int: Timeout in minutes.
func (c *Container) PostUpdateTimeout() int {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	val := c.getLabelValueOrEmpty(postUpdateTimeoutLabel)

	// Use default if label is unset.
	if val == "" {
		clog.Debug().
			Str("label", postUpdateTimeoutLabel).
			Msg("Post-update timeout not set, using default")

		return 1
	}

	// Parse timeout value.
	minutes, err := strconv.Atoi(val)
	if err != nil {
		clog.Warn().
			Err(err).
			Str("label", postUpdateTimeoutLabel).
			Str("value", val).
			Msg("Invalid post-update timeout value, using default")

		return 1
	}

	clog.Debug().
		Str("label", postUpdateTimeoutLabel).
		Int("minutes", minutes).
		Msg("Retrieved post-update timeout")

	return minutes
}

// getLifecycleID parses and validates a lifecycle ID (UID or GID) from labels.
//
// Parameters:
//   - label: The label key to retrieve the value from.
//   - idType: The type of ID ("UID" or "GID") for logging purposes.
//
// Returns:
//   - int: ID value if set and valid.
//   - bool: True if label is present and valid, false otherwise.
func (c *Container) getLifecycleID(label, idType string) (int, bool) {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	rawString, ok := c.getLabelValue(label)

	if !ok {
		clog.Debug().
			Str("label", label).
			Msg(fmt.Sprintf("Lifecycle %s label not set", idType))

		return 0, false
	}

	// Parse ID value.
	parsedID, err := strconv.Atoi(rawString)
	if err != nil {
		clog.Warn().
			Err(err).
			Str("label", label).
			Str("value", rawString).
			Msg(fmt.Sprintf("Invalid lifecycle %s value: not a valid integer", idType))

		return 0, false
	}

	// Validate ID range (must be non-negative and within reasonable bounds).
	if parsedID < 0 {
		clog.Warn().
			Str("label", label).
			Str("value", rawString).
			Str("id_type", idType).
			Int("id", parsedID).
			Msg(fmt.Sprintf("Invalid lifecycle %s value: must be non-negative", idType))

		return 0, false
	}

	// Check for unreasonably large ID values (greater than 2^31-1).
	const maxReasonableID = 2147483647 // 2^31-1
	if parsedID > maxReasonableID {
		clog.Warn().
			Str("label", label).
			Str("value", rawString).
			Str("id_type", idType).
			Int("id", parsedID).
			Int("max", maxReasonableID).
			Msg(fmt.Sprintf("Invalid lifecycle %s value: exceeds maximum reasonable value", idType))

		return 0, false
	}

	clog.Debug().
		Str("label", label).
		Str("id_type", idType).
		Int("id", parsedID).
		Msg("Retrieved lifecycle " + idType)

	return parsedID, true
}

// GetLifecycleUID returns the UID for lifecycle hooks from labels.
//
// Returns:
//   - int: UID value if set and valid.
//   - bool: True if label is present and valid, false otherwise.
func (c *Container) GetLifecycleUID() (int, bool) {
	return c.getLifecycleID(lifecycleUIDLabel, "UID")
}

// GetLifecycleGID returns the GID for lifecycle hooks from labels.
//
// Returns:
//   - int: GID value if set and valid.
//   - bool: True if label is present and valid, false otherwise.
func (c *Container) GetLifecycleGID() (int, bool) {
	return c.getLifecycleID(lifecycleGIDLabel, "GID")
}

// Enabled checks if Watchtower should manage the container.
//
// Returns:
//   - bool: True if enabled, false otherwise.
//   - bool: True if label is set and valid, false if absent or invalid.
func (c *Container) Enabled() (bool, bool) {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	rawBool, ok := c.getLabelValue(enableLabel)

	// Label not set, return default.
	if !ok {
		clog.Debug().
			Str("label", enableLabel).
			Msg("Enable label not set")

		return false, false
	}

	// Parse enable label value.
	parsedBool, err := strconv.ParseBool(rawBool)
	if err != nil {
		clog.Warn().
			Err(err).
			Str("label", enableLabel).
			Str("value", rawBool).
			Msg("Invalid enable label value")

		return false, false
	}

	clog.Debug().
		Str("label", enableLabel).
		Bool("value", parsedBool).
		Msg("Retrieved enable status")

	return parsedBool, true
}

// GetLabel retrieves the value of an arbitrary container label.
//
// It returns the label value and true if the label is present, or an empty
// string and false if the label is absent.
//
// Parameters:
//   - label: Docker label key to look up.
//
// Returns:
//   - string: Label value if present.
//   - bool: True if label exists, false otherwise.
func (c *Container) GetLabel(label string) (string, bool) {
	return c.getLabelValue(label)
}

// IsMonitorOnly determines if the container is monitor-only.
//
// It uses UpdateParams.MonitorOnly and label precedence.
//
// Parameters:
//   - params: Update parameters from types.UpdateParams.
//
// Returns:
//   - bool: True if monitor-only, false otherwise.
func (c *Container) IsMonitorOnly(params types.UpdateParams) bool {
	return c.getContainerOrGlobalBool(params.MonitorOnly, monitorOnlyLabel, params.LabelPrecedence)
}

// IsNoPull determines if image pulls should be skipped.
//
// It uses UpdateParams.NoPull and label precedence.
//
// Parameters:
//   - params: Update parameters from types.UpdateParams.
//
// Returns:
//   - bool: True if no-pull, false otherwise.
func (c *Container) IsNoPull(params types.UpdateParams) bool {
	return c.getContainerOrGlobalBool(params.NoPull, noPullLabel, params.LabelPrecedence)
}

// IsChangelogEnabled reports whether the changelog notification line and the
// versioned release link are enabled for this container.
//
// It uses UpdateParams.EnableChangelog and label precedence, with the same two
// sources no-pull uses. A changelog-url label is a value, not a switch, so it
// never enables the feature on its own.
// A user who authorizes the extra
// registry request must say so explicitly.
//
// Parameters:
//   - params: Update parameters from types.UpdateParams.
//
// Returns:
//   - bool: True when the changelog feature is enabled.
func (c *Container) IsChangelogEnabled(params types.UpdateParams) bool {
	return c.getContainerOrGlobalBool(
		params.EnableChangelog,
		git.EnableChangelogLabel,
		params.LabelPrecedence,
	)
}

// IsGitWatch reports whether the Git watcher is on for this container.
//
// A present git-watch label overrides --git-enable. When the label is absent
// the process-wide --git-enable default is used.
//
// Parameters:
//   - params: Update parameters from types.UpdateParams.
//
// Returns:
//   - bool: True when the watcher should run for an associated container.
func (c *Container) IsGitWatch(params types.UpdateParams) bool {
	return git.WatchEnabled(c, params)
}

// CooldownDelay returns the effective cooldown delay for this container.
//
// If the container has the cooldown-delay label set, its value is used (parsed
// as a duration string). Otherwise, the global CooldownDelay from UpdateParams
// is used. Setting the label to "0" disables cooldown for this container.
//
// Parameters:
//   - params: Update parameters from types.UpdateParams.
//
// Returns:
//   - time.Duration: Effective cooldown delay for this container.
func (c *Container) CooldownDelay(params types.UpdateParams) time.Duration {
	labelVal, ok := c.getLabelValue(cooldownDelayLabel)
	if !ok {
		// No label set — use global value.
		return params.CooldownDelay
	}

	if labelVal == "" {
		// Label present but empty — use global value.
		return params.CooldownDelay
	}

	parsed, err := util.ParseDuration(labelVal)
	if err != nil {
		c.logger().Warn().
			Err(err).
			Str("container", c.Name()).
			Str("label", cooldownDelayLabel).
			Str("value", labelVal).
			Msg("Failed to parse cooldown-delay label, using global value")

		return params.CooldownDelay
	}

	c.logger().Debug().
		Str("container", c.Name()).
		Str("label", cooldownDelayLabel).
		Str("value", labelVal).
		Dur("duration", parsed).
		Msg("Parsed cooldown-delay label")

	return parsed
}

// Scope retrieves the monitoring scope from labels.
//
// Returns:
//   - string: Scope value if set, empty otherwise.
//   - bool: True if label is set, false if absent.
func (c *Container) Scope() (string, bool) {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	rawString, ok := c.getLabelValue(scope)

	if !ok {
		clog.Debug().
			Str("label", scope).
			Msg("Scope label not set")

		return "", false
	}

	clog.Debug().
		Str("label", scope).
		Str("value", rawString).
		Msg("Retrieved scope")

	return rawString, true
}

// GetEffectiveScope determines the effective operational scope by prioritizing explicit scope
// over scope derived from the container's label. This is crucial for self-update scenarios where
// a new Watchtower instance needs to inherit the same scope as the instance being replaced
// to maintain proper isolation and prevent cross-scope interference during the update process.
//
// Parameters:
//   - container: The current Watchtower container.
//   - currentScope: The explicit scope value (takes priority if non-empty).
//
// Returns:
//   - string: The effective scope.
//   - error: Non-nil if derivation fails.
func GetEffectiveScope(container types.Container, currentScope string) (string, error) {
	// Skip derivation if scope is already explicitly set.
	if currentScope != "" {
		return currentScope, nil
	}

	// Container not available.
	if container == nil {
		return "", fmt.Errorf("%w", errCurrentContainerNotCached)
	}

	// Extract the scope label from the container.
	derivedScope, ok := container.Scope()
	if ok && derivedScope != "" {
		nopLog().Debug().
			Str("derived_scope", derivedScope).
			Msg("Derived operational scope from current container's scope label")

		return derivedScope, nil
	}

	return currentScope, nil
}

// GetContainerChain returns the container chain label value.
//
// Returns:
//   - string: The label value or empty if absent.
//   - bool: True if the label is present, false otherwise.
func (c *Container) GetContainerChain() (string, bool) {
	return c.getLabelValue(ContainerChainLabel)
}

// IsWatchtower identifies if this is the Watchtower container.
//
// Returns:
//   - bool: True if watchtower label is "true", false otherwise.
func (c *Container) IsWatchtower() bool {
	info := c.ContainerInfo()
	if info == nil || info.Config == nil {
		return false
	}

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	isWatchtower := ContainsWatchtowerLabel(info.Config.Labels)
	clog.Debug().
		Bool("is_watchtower", isWatchtower).
		Msg("Checked if container is Watchtower")

	return isWatchtower
}

// StopSignal returns the custom stop signal from labels or HostConfig.
//
// Returns:
//   - string: Signal value, defaulting to "SIGTERM" if unset.
func (c *Container) StopSignal() string {
	info := c.ContainerInfo()

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal

	// Check label first
	signal := c.getLabelValueOrEmpty(signalLabel)
	if signal != "" {
		clog.Debug().
			Str("label", signalLabel).
			Str("signal", signal).
			Msg("Retrieved stop signal from label")

		return signal
	}

	// Check Config
	if info != nil && info.Config != nil &&
		info.Config.StopSignal != "" {
		signal = info.Config.StopSignal
		clog.Debug().
			Str("signal", signal).
			Msg("Retrieved stop signal from Config")

		return signal
	}

	// Default to SIGTERM
	clog.Debug().Msg("Stop signal not set, using default SIGTERM")

	return "SIGTERM"
}

// StopTimeout returns the container's configured stop timeout in seconds.
//
// Returns:
//   - *int: Timeout in seconds if set, nil if unset.
func (c *Container) StopTimeout() *int {
	info := c.ContainerInfo()

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal

	// Check Config
	if info != nil && info.Config != nil &&
		info.Config.StopTimeout != nil {
		timeout := *info.Config.StopTimeout
		clog.Debug().
			Int("timeout", timeout).
			Msg("Retrieved stop timeout from Config")

		return &timeout
	}

	clog.Debug().Msg("Stop timeout not set in container config")

	return nil
}

// ContainsWatchtowerLabel checks if the container is Watchtower.
//
// Parameters:
//   - labels: Label map to check.
//
// Returns:
//   - bool: True if watchtower label is "true", false otherwise.
func ContainsWatchtowerLabel(labels map[string]string) bool {
	if labels == nil {
		return false
	}

	val, ok := labels[watchtowerLabel]
	nopLog().Debug().
		Str("label", watchtowerLabel).
		Str("val", val).
		Bool("ok", ok).
		Msg("Checking watchtower label")

	return ok && val == "true"
}

// getRawLabelValue retrieves a raw label value.
//
// Parameters:
//   - label: The label key to retrieve the value from.
//
// Returns:
//   - string: Label value if present.
//   - bool: True if label exists and is accessible, false otherwise.
func (c *Container) getRawLabelValue(label string) (string, bool) {
	return labelValue(c.ContainerInfo(), label)
}

// labelValue retrieves a raw label value from inspect data.
//
// Parameters:
//   - info: Inspect data, or nil.
//   - label: The label key to retrieve the value from.
//
// Returns:
//   - string: Label value if present.
//   - bool: True if label exists and is accessible, false otherwise.
func labelValue(info *dockerContainer.InspectResponse, label string) (string, bool) {
	if info == nil || info.Config == nil || info.Config.Labels == nil {
		return "", false
	}

	val, ok := info.Config.Labels[label]

	return val, ok
}

// getLabelValueOrEmpty retrieves a label's value or empty string.
//
// Returns:
//   - string: Label value or empty if absent.
func (c *Container) getLabelValueOrEmpty(label string) string {
	val, ok := c.getRawLabelValue(label)
	if !ok {
		return ""
	}

	return val
}

// getLabelValue fetches a label's value and presence.
//
// Returns:
//   - string: Label value if present.
//   - bool: True if label exists, false otherwise.
func (c *Container) getLabelValue(label string) (string, bool) {
	val, ok := c.getRawLabelValue(label)
	if !ok {
		return "", false
	}

	return val, true
}

// getBoolLabelValue parses a label as a boolean.
//
// Returns:
//   - bool: Parsed value if valid.
//   - error: Non-nil if parsing fails or label is absent, nil on success.
func (c *Container) getBoolLabelValue(label string) (bool, error) {
	strVal, ok := c.getRawLabelValue(label)
	if !ok {
		return false, errLabelNotFound
	}

	// Treat empty string as false to handle cases where label is explicitly set to empty
	if strVal == "" {
		return false, nil
	}

	value, err := strconv.ParseBool(strVal)
	if err != nil {
		c.logger().Warn().
			Err(err).
			Str("container", c.Name()).
			Str("label", label).
			Str("value", strVal).
			Msg("Failed to parse boolean label value")

		return false, fmt.Errorf("%w: %s=%q", err, label, strVal)
	}

	return value, nil
}

// getContainerOrGlobalBool resolves a boolean from label or global setting.
//
// It respects label precedence if set.
//
// Parameters:
//   - globalVal: Global boolean value.
//   - label: Label to check.
//   - contPrecedence: Whether container label takes precedence.
//
// Returns:
//   - bool: Resolved boolean value.
func (c *Container) getContainerOrGlobalBool(
	globalVal bool,
	label string,
	contPrecedence bool,
) bool {
	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal

	// Fetch container-specific value.
	contVal, err := c.getBoolLabelValue(label)
	if err != nil {
		if !errors.Is(err, errLabelNotFound) {
			clog.Warn().
				Err(err).
				Str("label", label).
				Msg("Failed to parse label value")
		}

		clog.Debug().
			Str("label", label).
			Bool("global_val", globalVal).
			Msg("Using global value due to label absence or error")

		return globalVal
	}

	// Apply container precedence if set.
	if contPrecedence {
		clog.Debug().
			Str("label", label).
			Bool("cont_val", contVal).
			Str("precedence", "container").
			Msg("Using container label value with precedence")

		return contVal
	}

	// Combine values if no precedence.
	result := contVal || globalVal
	clog.Debug().
		Str("label", label).
		Bool("cont_val", contVal).
		Bool("global_val", globalVal).
		Bool("result", result).
		Msg("Combined container and global values")

	return result
}
