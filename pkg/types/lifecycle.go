package types

import "fmt"

type LifecyclePhase int

const (
	PreCheck LifecyclePhase = iota
	PreUpdate
	PostUpdate
	PostCheck
)

func (p LifecyclePhase) String() string {
	switch p {
	case PreCheck:
		return "pre-check"
	case PreUpdate:
		return "pre-update"
	case PostUpdate:
		return "post-update"
	case PostCheck:
		return "post-check"
	default:
		return fmt.Sprintf("invalid(%d)", p)
	}
}
