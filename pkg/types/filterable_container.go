package types

// FilterableContainer defines an interface for container filtering.
type FilterableContainer interface {
	Name() string                       // Container name.
	IsWatchtower() bool                 // Check if Watchtower instance.
	Enabled() (bool, bool)              // Enabled status and presence.
	Scope() (string, bool)              // Scope value and presence.
	ImageName() string                  // Image name with tag.
	GetLabel(key string) (string, bool) // Arbitrary label value lookup.
}
