package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"

	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/rs/zerolog"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	dockerFlags "github.com/docker/cli/cli/flags"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Client applies a Compose project through docker/compose v5.
type Client struct {
	mu      sync.Mutex
	service api.Compose
}

// NewClient returns an applier that talks to the Docker daemon on first Apply.
//
// Returns:
//   - *Client: Lazy Compose service.
func NewClient() *Client {
	return &Client{}
}

// Apply loads the project and runs compose up, or compose build when BuildOnly is set.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - req: Project, services, labels, and build-only flag.
//
// Returns:
//   - []Container: Service instances after apply.
//   - error: Non-nil when load or apply fails.
func (c *Client) Apply(ctx context.Context, req Request) ([]Container, error) {
	if req.Ref.Dir == "" {
		return nil, errEmptyProjectDir
	}

	if len(req.Services) == 0 {
		return nil, errNoComposeServices
	}

	project, err := loadProject(ctx, req.Ref)
	if err != nil {
		return nil, err
	}

	project, err = scopeComposeProject(project, req.Services)
	if err != nil {
		return nil, fmt.Errorf("select compose services: %w", err)
	}

	project, err = injectLabels(project, req.Labels)
	if err != nil {
		return nil, fmt.Errorf("inject compose labels: %w", err)
	}

	project, err = stampComposeIdentity(project)
	if err != nil {
		return nil, err
	}

	svc, err := c.composeService()
	if err != nil {
		return nil, err
	}

	if req.BuildOnly {
		err = svc.Build(ctx, project, api.BuildOptions{
			Services: req.Services,
			Quiet:    true,
			Progress: "quiet",
		})
		if err != nil {
			return nil, fmt.Errorf("compose build: %w", err)
		}

		return listApplied(ctx, svc, project, req.Services)
	}

	err = svc.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{
			Build: &api.BuildOptions{
				Services: req.Services,
				Quiet:    true,
				Progress: "quiet",
			},
			Services:             req.Services,
			Recreate:             api.RecreateForce,
			RecreateDependencies: api.RecreateNever,
			Inherit:              true,
		},
		Start: api.StartOptions{
			Project: project,
			Wait:    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("compose up: %w", err)
	}

	return listApplied(ctx, svc, project, req.Services)
}

// scopeComposeProject scopes a project to the requested services and their dependencies.
//
// Parameters:
//   - project: Loaded Compose project.
//   - services: Service names to select.
//
// Returns:
//   - *composeTypes.Project: Scoped Compose project.
//   - error: Non-nil when service selection fails.
func scopeComposeProject(project *composeTypes.Project, services []string) (*composeTypes.Project, error) {
	if project == nil {
		return nil, errNilProject
	}

	project, err := project.WithServicesEnabled(services...)
	if err != nil {
		return nil, fmt.Errorf("enable selected compose services: %w", err)
	}

	project, err = project.WithSelectedServices(services, composeTypes.IncludeDependencies)
	if err != nil {
		return nil, fmt.Errorf("select selected compose services: %w", err)
	}

	return project, nil
}

// stampComposeIdentity sets the compose identity CustomLabels that docker/compose
// applies in its unexported postProcessProject before create and start.
//
// svc.Up lists containers with com.docker.compose.oneoff=False. A replacement
// created without that label is left Created, and start then reports that the
// service has no container to start. Existing CustomLabels are kept. Identity
// keys are overwritten. Watchtower git stamps stay on service.Labels.
//
// Parameters:
//   - project: Loaded Compose project about to be built or started.
//
// Returns:
//   - *composeTypes.Project: Project with identity CustomLabels on every service.
//   - error: Non-nil when the project is nil, its name is empty, or a transform fails.
func stampComposeIdentity(project *composeTypes.Project) (*composeTypes.Project, error) {
	if project == nil {
		return nil, errNilProject
	}

	if project.Name == "" {
		return nil, errEmptyProjectName
	}

	projectName := project.Name
	workingDir := project.WorkingDir
	configFiles := strings.Join(project.ComposeFiles, ",")

	project, err := project.WithServicesTransform(func(name string, service composeTypes.ServiceConfig) (composeTypes.ServiceConfig, error) {
		labels := maps.Clone(service.CustomLabels)
		if labels == nil {
			labels = composeTypes.Labels{}
		}

		labels[api.ProjectLabel] = projectName
		labels[api.ServiceLabel] = name
		labels[api.VersionLabel] = api.ComposeVersion
		labels[api.WorkingDirLabel] = workingDir
		labels[api.ConfigFilesLabel] = configFiles
		// Start filters on this exact string. "false" does not match.
		labels[api.OneoffLabel] = "False"

		service.CustomLabels = labels

		return service, nil
	})
	if err != nil {
		return nil, fmt.Errorf("stamp compose identity: %w", err)
	}

	return project, nil
}

// composeService returns the docker/compose service, creating it on first use.
//
// Returns:
//   - api.Compose: Compose service.
//   - error: Non-nil when the Docker CLI cannot be initialized.
func (c *Client) composeService() (api.Compose, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.service != nil {
		return c.service, nil
	}

	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("docker cli: %w", err)
	}

	err = dockerCLI.Initialize(dockerFlags.NewClientOptions())
	if err != nil {
		return nil, fmt.Errorf("docker cli init: %w", err)
	}

	svc, err := compose.NewComposeService(
		dockerCLI,
		compose.WithPrompt(compose.AlwaysOkPrompt()),
		compose.WithStreams(io.Discard, io.Discard, bytes.NewReader(nil)),
	)
	if err != nil {
		return nil, fmt.Errorf("compose service: %w", err)
	}

	c.service = svc

	return svc, nil
}

// listApplied returns running service containers after compose up or build.
//
// Parameters:
//   - ctx: Cancellation and timeout.
//   - svc: Compose service.
//   - project: Loaded project.
//   - services: Service names that were applied.
//
// Returns:
//   - []Container: Service instances.
//   - error: Non-nil when compose ps fails.
func listApplied(
	ctx context.Context,
	svc api.Compose,
	project *composeTypes.Project,
	services []string,
) ([]Container, error) {
	summaries, err := svc.Ps(ctx, project.Name, api.PsOptions{
		Project:  project,
		All:      true,
		Services: services,
	})
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w", err)
	}

	images, err := svc.Images(ctx, project.Name, api.ImagesOptions{Services: services})
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to list Compose images. Image IDs from that call will be omitted.")

		images = nil
	}

	out := make([]Container, 0, len(summaries))

	for _, summary := range summaries {
		out = append(out, Container{
			Service: summary.Service,
			Name:    summary.Name,
			ID:      types.ContainerID(summary.ID),
			ImageID: appliedImageID(summary, images),
		})
	}

	return out, nil
}

// appliedImageID prefers the inspected image ID from compose images over the Ps name.
//
// Parameters:
//   - summary: Compose ps row.
//   - images: compose images keyed by container name.
//
// Returns:
//   - types.ImageID: Inspected ID, a sha256: reference, or empty.
func appliedImageID(summary api.ContainerSummary, images map[string]api.ImageSummary) types.ImageID {
	if images != nil {
		if img, ok := images[summary.Name]; ok && img.ID != "" {
			return types.ImageID(img.ID)
		}
	}

	if strings.HasPrefix(summary.Image, "sha256:") {
		return types.ImageID(summary.Image)
	}

	return ""
}
