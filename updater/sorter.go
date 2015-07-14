package updater

import (
	"fmt"

	"github.com/CenturyLinkLabs/watchtower/docker"
)

type ContainerSorter struct {
	unvisited []docker.Container
	marked    map[string]bool
	sorted    []docker.Container
}

func (cs *ContainerSorter) Sort(containers []docker.Container) ([]docker.Container, error) {
	cs.unvisited = containers
	cs.marked = map[string]bool{}

	for len(cs.unvisited) > 0 {
		if err := cs.visit(cs.unvisited[0]); err != nil {
			return nil, err
		}
	}

	return cs.sorted, nil
}

func (cs *ContainerSorter) visit(c docker.Container) error {

	if _, ok := cs.marked[c.Name()]; ok {
		return fmt.Errorf("Circular reference to %s", c.Name())
	}

	// Mark any visited node so that circular references can be detected
	cs.marked[c.Name()] = true
	defer delete(cs.marked, c.Name())

	// Recursively visit links
	for _, linkName := range c.Links() {
		if linkedContainer := cs.findUnvisited(linkName); linkedContainer != nil {
			if err := cs.visit(*linkedContainer); err != nil {
				return err
			}
		}
	}

	// Move container from unvisited to sorted
	cs.removeUnvisited(c)
	cs.sorted = append(cs.sorted, c)

	return nil
}

func (cs *ContainerSorter) findUnvisited(name string) *docker.Container {
	for _, c := range cs.unvisited {
		if c.Name() == name {
			return &c
		}
	}

	return nil
}

func (cs *ContainerSorter) removeUnvisited(c docker.Container) {
	var idx int
	for i := range cs.unvisited {
		if cs.unvisited[i].Name() == c.Name() {
			idx = i
			break
		}
	}

	cs.unvisited = append(cs.unvisited[0:idx], cs.unvisited[idx+1:]...)
}
