package sorter

import (
	"fmt"
	"time"

	"github.com/containrrr/watchtower/pkg/types"
)

// ByCreated allows a list of Container structs to be sorted by the container's
// created date.
type ByCreated []types.Container

func (c ByCreated) Len() int      { return len(c) }
func (c ByCreated) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Less will compare two elements (identified by index) in the Container
// list by created-date.
func (c ByCreated) Less(i, j int) bool {
	t1, err := time.Parse(time.RFC3339Nano, c[i].ContainerInfo().Created)
	if err != nil {
		t1 = time.Now()
	}

	t2, _ := time.Parse(time.RFC3339Nano, c[j].ContainerInfo().Created)
	if err != nil {
		t1 = time.Now()
	}

	return t1.Before(t2)
}

// SortByDependencies will sort the list of containers taking into account any
// links between containers. Container with no outgoing links will be sorted to
// the front of the list while containers with links will be sorted after all
// of their dependencies. This sort order ensures that linked containers can
// be started in the correct order.
func SortByDependencies(containers []types.Container) ([]types.Container, error) {
	sorter := dependencySorter{}
	return sorter.Sort(containers)
}

type dependencySorter struct {
	unvisited []types.Container
	marked    map[string]bool
	sorted    []types.Container
}

func (ds *dependencySorter) Sort(containers []types.Container) ([]types.Container, error) {
	ds.unvisited = containers
	ds.marked = map[string]bool{}

	for len(ds.unvisited) > 0 {
		if err := ds.visit(ds.unvisited[0]); err != nil {
			return nil, err
		}
	}

	return ds.sorted, nil
}

func (ds *dependencySorter) visit(c types.Container) error {

	if _, ok := ds.marked[c.Name()]; ok {
		return fmt.Errorf("circular reference to %s", c.Name())
	}

	// Mark any visited node so that circular references can be detected
	ds.marked[c.Name()] = true
	defer delete(ds.marked, c.Name())

	// Recursively visit links
	for _, linkName := range c.Links() {
		if linkedContainer := ds.findUnvisited(linkName); linkedContainer != nil {
			if err := ds.visit(*linkedContainer); err != nil {
				return err
			}
		}
	}

	// Move container from unvisited to sorted
	ds.removeUnvisited(c)
	ds.sorted = append(ds.sorted, c)

	return nil
}

func (ds *dependencySorter) findUnvisited(name string) *types.Container {
	for _, c := range ds.unvisited {
		if c.Name() == name {
			return &c
		}
	}

	return nil
}

func (ds *dependencySorter) removeUnvisited(c types.Container) {
	var idx int
	for i := range ds.unvisited {
		if ds.unvisited[i].Name() == c.Name() {
			idx = i
			break
		}
	}

	ds.unvisited = append(ds.unvisited[0:idx], ds.unvisited[idx+1:]...)
}
