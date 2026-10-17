package compose

import (
	"context"
	"fmt"
	"maps"

	composetypes "github.com/compose-spec/compose-go/v2/types"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Request is one Compose apply for a checked-out project directory.
type Request struct {
	// Ref is the resolved project directory and optional compose files.
	Ref ProjectRef
	// Services are Compose service names to build or recreate.
	Services []string
	// Labels are extra service labels merged onto the compose model before apply.
	Labels map[string]map[string]string
	// BuildOnly builds images and does not recreate running containers.
	BuildOnly bool
}

// Container is a running Compose service instance after apply.
type Container struct {
	Service string
	// Name is the container name from compose ps, used to match replicas.
	Name    string
	ID      types.ContainerID
	ImageID types.ImageID
}

// Applier loads a Compose project and applies selected services.
type Applier interface {
	// Apply loads the project, merges Labels, and runs compose up or build.
	//
	// Parameters:
	//   - ctx: Cancellation and timeout.
	//   - req: Project, services, labels, and build-only flag.
	//
	// Returns:
	//   - []Container: Service instances after apply.
	//   - error: Non-nil when load or apply fails.
	Apply(ctx context.Context, req Request) ([]Container, error)
}

// injectLabels returns a copy of project with extra labels merged onto named services.
//
// Parameters:
//   - project: Loaded Compose project.
//   - labels: Service name to label map.
//
// Returns:
//   - *composetypes.Project: Transformed project.
//   - error: Non-nil when a transform fails.
func injectLabels(project *composetypes.Project, labels map[string]map[string]string) (*composetypes.Project, error) {
	if project == nil {
		return nil, errNilProject
	}

	if len(labels) == 0 {
		return project, nil
	}

	project, err := project.WithServicesTransform(func(name string, service composetypes.ServiceConfig) (composetypes.ServiceConfig, error) {
		extra, ok := labels[name]
		if !ok || len(extra) == 0 {
			return service, nil
		}

		if service.Labels == nil {
			service.Labels = composetypes.Labels{}
		}

		maps.Copy(service.Labels, extra)

		return service, nil
	})
	if err != nil {
		return nil, fmt.Errorf("transform compose services: %w", err)
	}

	return project, nil
}
