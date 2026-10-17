package sorter

import (
	"errors"
	"strings"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// errCircularReference indicates a circular dependency between containers.
var errCircularReference = errors.New("circular reference detected")

// errIdentifierCollision indicates an identifier collision between containers.
var errIdentifierCollision = errors.New("identifier collision detected")

// errNotInGraph indicates a container that is not part of a dependency graph.
var errNotInGraph = errors.New("container is not in the dependency graph")

// circularReferenceError represents a circular dependency error with the container name and cycle path.
type circularReferenceError struct {
	ContainerName string
	CyclePath     []string
}

// Error implements the error interface.
func (e circularReferenceError) Error() string {
	if len(e.CyclePath) > 0 {
		var pathStrSb20 strings.Builder

		for i, name := range e.CyclePath {
			if i > 0 {
				pathStrSb20.WriteString(" -> ")
			}

			pathStrSb20.WriteString(name)
		}

		return "circular reference detected: " + pathStrSb20.String()
	}

	return "circular reference detected: " + e.ContainerName
}

// Unwrap returns the underlying error for errors.Is compatibility.
func (e circularReferenceError) Unwrap() error {
	return errCircularReference
}

// identifierCollisionError represents an error when multiple containers have the same normalized identifier.
type identifierCollisionError struct {
	DuplicateIdentifier string
	AffectedContainers  []types.Container
}

// Error implements the error interface.
func (e identifierCollisionError) Error() string {
	containerDetails := make([]string, 0, len(e.AffectedContainers))
	for _, c := range e.AffectedContainers {
		containerDetails = append(containerDetails, c.Name()+" ("+c.ID().ShortID()+")")
	}

	return "identifier collision detected: '" + e.DuplicateIdentifier + "' used by containers: " + strings.Join(
		containerDetails,
		", ",
	)
}

// Unwrap returns the underlying error for errors.Is compatibility.
func (e identifierCollisionError) Unwrap() error {
	return errIdentifierCollision
}
