package mocks

import (
	"fmt"
	"os"

	t "github.com/containrrr/watchtower/pkg/types"
)

type imageRef struct {
	id   t.ImageID
	file string
}

func (ir *imageRef) getFileName() string {
	return fmt.Sprintf("./mocks/data/image_%v.json", ir.file)
}

type ContainerRef struct {
	name       string
	id         t.ContainerID
	image      *imageRef
	file       string
	references []*ContainerRef
	isMissing  bool
}

func (cr *ContainerRef) getContainerFile() (containerFile string, err error) {
	file := cr.file
	if file == "" {
		file = cr.name
	}

	containerFile = fmt.Sprintf("./mocks/data/container_%v.json", file)
	_, err = os.Stat(containerFile)

	return containerFile, err
}

func (cr *ContainerRef) ContainerID() t.ContainerID {
	return cr.id
}
