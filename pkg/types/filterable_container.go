package types

// A FilterableContainer is the interface which is used to filter
// containers.
type FilterableContainer interface {
	Name() string
	IsWatchtower() bool
	Enabled() (bool, bool)
	Scope() (string, bool)
	ImageName() string
}
