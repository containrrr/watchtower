package sorter

// cycleFinder finds the strongly connected components of a dependency graph
// with Tarjan's algorithm. Every component with more than one node is a set of
// containers that depend on each other in a cycle. The graph has no self-loops,
// since a container's link to itself is not added as an edge.
type cycleFinder struct {
	// adjacency lists the outgoing edges of each node.
	adjacency map[string][]string
	// index records the order in which each node was first visited.
	index map[string]int
	// lowLink records the smallest index reachable from each node's subtree.
	lowLink map[string]int
	// onStack reports whether a node is on the stack of the current search.
	onStack map[string]bool
	// stack holds the nodes visited but not yet assigned to a component.
	stack []string
	// inCycle records the nodes found on a cycle.
	inCycle map[string]bool
}

// visit searches the graph depth-first from node and records the members of
// each multi-node component whose root it finishes.
//
// Parameters:
//   - node: The unvisited node to search from.
func (f *cycleFinder) visit(node string) {
	f.index[node] = len(f.index)
	f.lowLink[node] = f.index[node]
	f.stack = append(f.stack, node)
	f.onStack[node] = true

	for _, next := range f.adjacency[node] {
		if _, visited := f.index[next]; !visited {
			f.visit(next)
			f.lowLink[node] = min(f.lowLink[node], f.lowLink[next])
		} else if f.onStack[next] {
			f.lowLink[node] = min(f.lowLink[node], f.index[next])
		}
	}

	// Only the root of a component pops it off the stack.
	if f.lowLink[node] != f.index[node] {
		return
	}

	var component []string

	for {
		top := f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
		f.onStack[top] = false
		component = append(component, top)

		if top == node {
			break
		}
	}

	if len(component) > 1 {
		for _, member := range component {
			f.inCycle[member] = true
		}
	}
}
