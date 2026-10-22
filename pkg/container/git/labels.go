package git

// Watchtower Git labels use the same com.centurylinklabs.watchtower. prefix as other container labels.
const (
	RepoLabel            = "com.centurylinklabs.watchtower.git-repo"
	RefLabel             = "com.centurylinklabs.watchtower.git-ref"
	branchLabel          = "com.centurylinklabs.watchtower.git-branch"
	HostLabel            = "com.centurylinklabs.watchtower.git-host"
	SemverPolicyLabel    = "com.centurylinklabs.watchtower.git-semver-policy"
	WatchLabel           = "com.centurylinklabs.watchtower.git-watch"
	DockerfileLabel      = "com.centurylinklabs.watchtower.git-dockerfile"
	ContextLabel         = "com.centurylinklabs.watchtower.git-context"
	EnableChangelogLabel = "com.centurylinklabs.watchtower.enable-changelog"
	ChangelogURLLabel    = "com.centurylinklabs.watchtower.changelog-url"
	LastCommitLabel      = "com.centurylinklabs.watchtower.git-last-commit"
	LastTagLabel         = "com.centurylinklabs.watchtower.git-last-tag"
	ComposeDirLabel      = "com.centurylinklabs.watchtower.compose-dir"
)
