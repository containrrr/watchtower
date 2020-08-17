package sorter

import (
	"github.com/containrrr/watchtower/pkg/container"
	"time"
)

// ByCreated allows a list of Container structs to be sorted by the container's
// created date.
type ByCreated []container.Container

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
// the front of their dependency list while containers without links will be
// placed into their own list.This sort order ensures that linked containers can
// be started in the correct order as well as separate independent sets of linked
// containers from each other.
func SortByDependencies(containers []container.Container, undirectedNodes map[string][]string) ([][]container.Container, error) {
	sorter := dependencySorter{}
	return sorter.Sort(containers,undirectedNodes)
}

type dependencySorter struct {
	unvisited []container.Container
	marked    map[string]bool
	sorted    [][]container.Container
}

func (ds *dependencySorter) Sort(containers []container.Container, undirectedNodes map[string][]string) ([][]container.Container, error) {
	ds.unvisited = containers
	ds.marked = map[string]bool{}

	for len(ds.unvisited) > 0 {
		linkedGraph := make([]container.Container,0,0)
		ds.sorted = append(ds.sorted,linkedGraph)
		if err := ds.visit(ds.unvisited[0],undirectedNodes); err != nil {
			return nil, err
		}
	}

	return ds.sorted, nil
}

func (ds *dependencySorter) visit(c container.Container, undirectedNodes map[string][]string) error {

	if _, ok := ds.marked[c.Name()]; ok {
		return nil
	}

	// Mark any visited node so that we don't visit it again
	ds.marked[c.Name()] = true
	defer delete(ds.marked, c.Name())

	// Recursively visit links
	for _, linkName := range undirectedNodes[c.Name()] {
		if linkedContainer := ds.findUnvisited(linkName); linkedContainer != nil {
			if err := ds.visit(*linkedContainer,undirectedNodes); err != nil {
				return err
			}
		}
	}

	// Move container from unvisited to sorted
	ds.removeUnvisited(c)
	ds.sorted[len(ds.sorted)-1] = append(ds.sorted[len(ds.sorted)-1], c)

	return nil
}

func (ds *dependencySorter) findUnvisited(name string) *container.Container {
	for _, c := range ds.unvisited {
		if c.Name() == name {
			return &c
		}
	}

	return nil
}

func (ds *dependencySorter) removeUnvisited(c container.Container) {
	var idx int
	for i := range ds.unvisited {
		if ds.unvisited[i].Name() == c.Name() {
			idx = i
			break
		}
	}

	ds.unvisited = append(ds.unvisited[0:idx], ds.unvisited[idx+1:]...)
}
