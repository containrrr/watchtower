package types

type ImageMeta interface {
	Authors() string
	Created() string
	Description() string
	Documentation() string
	Licenses() string
	Revision() string
	Source() string
	Title() string
	Url() string
	Version() string
}
