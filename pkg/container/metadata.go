package container

const (
	watchtowerLabel       = "com.centurylinklabs.watchtower"
	signalLabel           = "com.centurylinklabs.watchtower.stop-signal"
	enableLabel           = "com.centurylinklabs.watchtower.enable"
	monitorOnlyLabel      = "com.centurylinklabs.watchtower.monitor-only"
	dependsOnLabel        = "com.centurylinklabs.watchtower.depends-on"
	zodiacLabel           = "com.centurylinklabs.zodiac.original-image"
	scope                 = "com.centurylinklabs.watchtower.scope"
	preCheckLabel         = "com.centurylinklabs.watchtower.lifecycle.pre-check"
	postCheckLabel        = "com.centurylinklabs.watchtower.lifecycle.post-check"
	preUpdateLabel        = "com.centurylinklabs.watchtower.lifecycle.pre-update"
	postUpdateLabel       = "com.centurylinklabs.watchtower.lifecycle.post-update"
	preCheckUserLabel     = "com.centurylinklabs.watchtower.lifecycle.pre-check.user"
	postCheckUserLabel    = "com.centurylinklabs.watchtower.lifecycle.post-check.user"
	preUpdateTimeoutLabel = "com.centurylinklabs.watchtower.lifecycle.pre-update-timeout"
	preUpdateUserLabel    = "com.centurylinklabs.watchtower.lifecycle.pre-update.user"
	postUpdateUserLabel   = "com.centurylinklabs.watchtower.lifecycle.post-update.user"
)

// GetLifecyclePreCheckCommand returns the pre-check command set in the container metadata or an empty string
func (c Container) GetLifecyclePreCheckCommand() string {
	return c.getLabelValueOrEmpty(preCheckLabel)
}

// GetLifecyclePostCheckCommand returns the post-check command set in the container metadata or an empty string
func (c Container) GetLifecyclePostCheckCommand() string {
	return c.getLabelValueOrEmpty(postCheckLabel)
}

// GetLifecyclePreUpdateCommand returns the pre-update command set in the container metadata or an empty string
func (c Container) GetLifecyclePreUpdateCommand() string {
	return c.getLabelValueOrEmpty(preUpdateLabel)
}

// GetLifecyclePostUpdateCommand returns the post-update command set in the container metadata or an empty string
func (c Container) GetLifecyclePostUpdateCommand() string {
	return c.getLabelValueOrEmpty(postUpdateLabel)
}

// GetLifecyclePreCheckUser returns the pre-check user set in the container metadata or an empty string
func (c Container) GetLifecyclePreCheckUser() string {
	return c.getLabelValueOrEmpty(preCheckUserLabel)
}

// GetLifecyclePostCheckUser returns the post-check user in the container metadata or an empty string
func (c Container) GetLifecyclePostCheckUser() string {
	return c.getLabelValueOrEmpty(postCheckUserLabel)
}

// GetLifecyclePreUpdateUser returns the pre-update user set in the container metadata or an empty string
func (c Container) GetLifecyclePreUpdateUser() string {
	return c.getLabelValueOrEmpty(preUpdateUserLabel)
}

// GetLifecyclePostUpdateUser returns the post-update set in the container metadata or an empty string
func (c Container) GetLifecyclePostUpdateUser() string {
	return c.getLabelValueOrEmpty(postUpdateUserLabel)
}

// ContainsWatchtowerLabel takes a map of labels and values and tells
// the consumer whether it contains a valid watchtower instance label
func ContainsWatchtowerLabel(labels map[string]string) bool {
	val, ok := labels[watchtowerLabel]
	return ok && val == "true"
}

func (c Container) getLabelValueOrEmpty(label string) string {
	if val, ok := c.containerInfo.Config.Labels[label]; ok {
		return val
	}
	return ""
}

func (c Container) getLabelValue(label string) (string, bool) {
	val, ok := c.containerInfo.Config.Labels[label]
	return val, ok
}
