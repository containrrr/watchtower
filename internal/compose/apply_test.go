package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/compose/v5/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	composetypes "github.com/compose-spec/compose-go/v2/types"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestLoad verifies that a project loads from its directory's default Compose
// file, or from explicit files under a given name, and that an empty directory
// is an error.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("empty dir", func(t *testing.T) {
		t.Parallel()

		_, err := loadProject(t.Context(), ProjectRef{})
		require.ErrorIs(t, err, errEmptyProjectDir)
	})

	t.Run("default compose file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`
name: webstack
services:
  api:
    build: ./api
  db:
    image: postgres:16
`), 0o600))

		project, err := loadProject(t.Context(), ProjectRef{Dir: dir})
		require.NoError(t, err)
		assert.Equal(t, "webstack", project.Name)
		assert.ElementsMatch(t, []string{"api", "db"}, project.ServiceNames())
	})

	t.Run("named project and explicit files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "stack.yml")
		require.NoError(t, os.WriteFile(path, []byte(`
services:
  worker:
    build: ./worker
`), 0o600))

		project, err := loadProject(t.Context(), ProjectRef{
			Name:        "jobs",
			Dir:         dir,
			ConfigFiles: []string{path},
		})
		require.NoError(t, err)
		assert.Equal(t, "jobs", project.Name)
		assert.Equal(t, []string{"worker"}, project.ServiceNames())
	})
}

// TestLoadIgnoresProcessSecrets verifies that a project interpolates variables
// from its .env file and not from Watchtower's own environment, so Watchtower's
// secrets never reach a service.
//
// The test sets an environment variable, so it does not run in parallel.
func TestLoadIgnoresProcessSecrets(t *testing.T) {
	t.Setenv("WATCHTOWER_GIT_AUTH_TOKEN", "supersecret")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_TAG=fromdotenv\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`
services:
  api:
    image: app:${APP_TAG}
    environment:
      TOKEN: ${WATCHTOWER_GIT_AUTH_TOKEN}
`), 0o600))

	project, err := loadProject(t.Context(), ProjectRef{Dir: dir, Name: "p"})
	require.NoError(t, err)

	api, err := project.GetService("api")
	require.NoError(t, err)
	assert.Equal(t, "app:fromdotenv", api.Image)
	assert.NotContains(t, fmt.Sprint(api.Environment), "supersecret")
}

// TestInjectLabels verifies that labels are merged onto the named service only,
// that no labels leave the project unchanged, and that a nil project is an
// error.
func TestInjectLabels(t *testing.T) {
	t.Parallel()

	t.Run("nil project", func(t *testing.T) {
		t.Parallel()

		_, err := injectLabels(nil, map[string]map[string]string{"api": {"k": "v"}})
		require.ErrorIs(t, err, errNilProject)
	})

	t.Run("empty labels returns the same project", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services:\n  api:\n    image: app\n"), 0o600))

		project, err := loadProject(t.Context(), ProjectRef{Dir: dir, Name: "p"})
		require.NoError(t, err)

		got, err := injectLabels(project, nil)
		require.NoError(t, err)
		assert.Equal(t, project, got)
	})

	t.Run("merges labels onto the named service only", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`
services:
  api:
    build: ./api
    labels:
      keep: "yes"
  db:
    image: postgres:16
`), 0o600))

		project, err := loadProject(t.Context(), ProjectRef{Dir: dir, Name: "p"})
		require.NoError(t, err)

		got, err := injectLabels(project, map[string]map[string]string{
			"api": {
				"com.centurylinklabs.watchtower.git-last-commit": "abc",
			},
		})
		require.NoError(t, err)

		api, err := got.GetService("api")
		require.NoError(t, err)
		assert.Equal(t, "yes", api.Labels["keep"])
		assert.Equal(t, "abc", api.Labels["com.centurylinklabs.watchtower.git-last-commit"])

		db, err := got.GetService("db")
		require.NoError(t, err)
		assert.Empty(t, db.Labels["com.centurylinklabs.watchtower.git-last-commit"])
	})
}

// TestAppliedImageID verifies that a container's image ID comes from the
// Compose image summary for the container, or from the container's image when
// that is an ID.
func TestAppliedImageID(t *testing.T) {
	t.Parallel()

	summary := api.ContainerSummary{Name: "webstack-api-1", Image: "webstack-api:latest"}
	assert.Empty(t, appliedImageID(summary, nil))
	assert.Empty(t, appliedImageID(summary, map[string]api.ImageSummary{}))
	assert.Equal(t, types.ImageID("sha256:abc"), appliedImageID(
		summary,
		map[string]api.ImageSummary{"webstack-api-1": {ID: "sha256:abc"}},
	))
	assert.Equal(
		t,
		types.ImageID("sha256:fromps"),
		appliedImageID(api.ContainerSummary{Name: "x", Image: "sha256:fromps"}, nil),
	)
}

// TestClientApplyScopesStartProject verifies that applying a service scopes the project to that service and its dependencies.
func TestClientApplyScopesStartProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`
services:
  app:
    image: app
    depends_on:
      - db
  db:
    image: db
  cache:
    image: cache
`), 0o600))

	var (
		upProject *composetypes.Project
		upOptions api.UpOptions
	)

	service := &composeServiceStub{
		up: func(_ context.Context, project *composetypes.Project, options api.UpOptions) error {
			upProject = project
			upOptions = options

			return nil
		},
		ps: func(context.Context, string, api.PsOptions) ([]api.ContainerSummary, error) {
			return []api.ContainerSummary{{Service: "app", Name: "project-app-1", ID: "app-id"}}, nil
		},
		images: func(context.Context, string, api.ImagesOptions) (map[string]api.ImageSummary, error) {
			return map[string]api.ImageSummary{}, nil
		},
	}

	client := &Client{service: service}
	containers, err := client.Apply(t.Context(), Request{
		Ref:      ProjectRef{Dir: dir, Name: "project"},
		Services: []string{"app"},
	})
	require.NoError(t, err)
	assert.Equal(t, []Container{{
		Service: "app",
		Name:    "project-app-1",
		ID:      types.ContainerID("app-id"),
	}}, containers)

	require.NotNil(t, upProject)
	assert.Same(t, upProject, upOptions.Start.Project)
	assert.ElementsMatch(t, []string{"app", "db"}, upProject.ServiceNames())
	assert.NotContains(t, upProject.ServiceNames(), "cache")
	assert.Equal(t, []string{"app"}, upOptions.Create.Services)
	assert.Equal(t, api.RecreateNever, upOptions.Create.RecreateDependencies)
	assert.Empty(t, upOptions.Start.Services)
}

// TestClientApplyRejectsInvalidRequests verifies that Client.Apply rejects invalid requests before invoking Compose.
func TestClientApplyRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	service := &composeServiceStub{
		up: func(context.Context, *composetypes.Project, api.UpOptions) error {
			t.Fatal("Compose up must not run for invalid requests")

			return nil
		},
		ps: func(context.Context, string, api.PsOptions) ([]api.ContainerSummary, error) {
			t.Fatal("Compose ps must not run for invalid requests")

			return []api.ContainerSummary{}, nil
		},
		images: func(context.Context, string, api.ImagesOptions) (map[string]api.ImageSummary, error) {
			t.Fatal("Compose images must not run for invalid requests")

			return map[string]api.ImageSummary{}, nil
		},
	}
	client := &Client{service: service}

	_, err := client.Apply(t.Context(), Request{
		Ref:      ProjectRef{},
		Services: []string{"app"},
	})
	require.ErrorIs(t, err, errEmptyProjectDir)

	_, err = client.Apply(t.Context(), Request{
		Ref:      ProjectRef{Dir: t.TempDir()},
		Services: nil,
	})
	require.ErrorIs(t, err, errNoComposeServices)
}

// TestClientApplyStampsComposeIdentity verifies Apply sets the labels compose start filters on.
func TestClientApplyStampsComposeIdentity(t *testing.T) {
	t.Parallel()

	dir := writeIdentityComposeDir(t)

	t.Run("up", func(t *testing.T) {
		t.Parallel()

		var (
			upProject *composetypes.Project
			upOptions api.UpOptions
		)

		service := &composeServiceStub{
			up: func(_ context.Context, project *composetypes.Project, options api.UpOptions) error {
				upProject = project
				upOptions = options

				return nil
			},
			ps: func(context.Context, string, api.PsOptions) ([]api.ContainerSummary, error) {
				return []api.ContainerSummary{{Service: "api", Name: "jobs-api-1", ID: "api-id"}}, nil
			},
			images: func(context.Context, string, api.ImagesOptions) (map[string]api.ImageSummary, error) {
				return map[string]api.ImageSummary{}, nil
			},
		}

		client := &Client{service: service}
		_, err := client.Apply(t.Context(), Request{
			Ref:      ProjectRef{Dir: dir, Name: "jobs"},
			Services: []string{"api"},
			Labels: map[string]map[string]string{
				"api": {"com.centurylinklabs.watchtower.git-last-commit": "abc"},
			},
		})
		require.NoError(t, err)

		require.NotNil(t, upProject)
		assert.Same(t, upProject, upOptions.Start.Project)
		assert.Nil(t, upOptions.Start.Attach)
		assert.Equal(t, api.RecreateForce, upOptions.Create.Recreate)
		assertComposeIdentity(t, upProject, dir)

		apiService, err := upProject.GetService("api")
		require.NoError(t, err)
		assert.Equal(t, "yes", apiService.Labels["keep"])
		assert.Equal(t, "from-file", apiService.Labels[api.ProjectLabel])
		assert.Equal(t, "abc", apiService.Labels["com.centurylinklabs.watchtower.git-last-commit"])

		dbService, err := upProject.GetService("db")
		require.NoError(t, err)
		assert.Equal(t, "db", dbService.CustomLabels[api.ServiceLabel])
	})

	t.Run("build", func(t *testing.T) {
		t.Parallel()

		var built *composetypes.Project

		service := &composeServiceStub{
			up: func(context.Context, *composetypes.Project, api.UpOptions) error {
				t.Fatal("compose up must not run for build-only apply")

				return nil
			},
			build: func(_ context.Context, project *composetypes.Project, _ api.BuildOptions) error {
				built = project

				return nil
			},
			ps: func(context.Context, string, api.PsOptions) ([]api.ContainerSummary, error) {
				return []api.ContainerSummary{}, nil
			},
			images: func(context.Context, string, api.ImagesOptions) (map[string]api.ImageSummary, error) {
				return map[string]api.ImageSummary{}, nil
			},
		}

		client := &Client{service: service}
		_, err := client.Apply(t.Context(), Request{
			Ref:       ProjectRef{Dir: dir, Name: "jobs"},
			Services:  []string{"api"},
			BuildOnly: true,
		})
		require.NoError(t, err)
		assertComposeIdentity(t, built, dir)
	})
}

// TestStampComposeIdentity verifies identity labels are merged and an empty name is rejected.
func TestStampComposeIdentity(t *testing.T) {
	t.Parallel()

	t.Run("nil project", func(t *testing.T) {
		t.Parallel()

		_, err := stampComposeIdentity(nil)
		require.ErrorIs(t, err, errNilProject)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		_, err := stampComposeIdentity(&composetypes.Project{
			Services: composetypes.Services{
				"api": {Name: "api", Image: "app"},
			},
		})
		require.ErrorIs(t, err, errEmptyProjectName)
	})

	t.Run("loaded project keeps other custom labels", func(t *testing.T) {
		t.Parallel()

		dir := writeIdentityComposeDir(t)
		project, err := loadProject(t.Context(), ProjectRef{Dir: dir, Name: "jobs"})
		require.NoError(t, err)

		service, err := project.GetService("api")
		require.NoError(t, err)

		service.CustomLabels = composetypes.Labels{
			"keep-custom":   "yes",
			api.OneoffLabel: "True",
		}
		project.Services["api"] = service

		got, err := stampComposeIdentity(project)
		require.NoError(t, err)
		assertComposeIdentity(t, got, dir)

		stamped, err := got.GetService("api")
		require.NoError(t, err)
		assert.Equal(t, "yes", stamped.CustomLabels["keep-custom"])
		assert.Equal(t, "False", stamped.CustomLabels[api.OneoffLabel])

		original, err := project.GetService("api")
		require.NoError(t, err)
		assert.Equal(t, "True", original.CustomLabels[api.OneoffLabel])
		assert.Equal(t, "yes", original.CustomLabels["keep-custom"])
	})
}

// assertComposeIdentity checks the labels compose start uses to find service containers.
//
// Parameters:
//   - t: Test that receives the failures.
//   - project: Project whose services are checked.
//   - dir: Working directory the project must have.
func assertComposeIdentity(t *testing.T, project *composetypes.Project, dir string) {
	t.Helper()

	require.NotNil(t, project)
	require.NotEmpty(t, project.ServiceNames())
	require.Equal(t, dir, project.WorkingDir)
	require.NotEmpty(t, project.Name)

	configFiles := strings.Join(project.ComposeFiles, ",")

	for _, serviceName := range project.ServiceNames() {
		service, err := project.GetService(serviceName)
		require.NoError(t, err)
		assert.Equal(t, "False", service.CustomLabels[api.OneoffLabel])
		assert.Equal(t, project.Name, service.CustomLabels[api.ProjectLabel])
		assert.Equal(t, serviceName, service.CustomLabels[api.ServiceLabel])
		assert.Equal(t, project.WorkingDir, service.CustomLabels[api.WorkingDirLabel])
		assert.Equal(t, api.ComposeVersion, service.CustomLabels[api.VersionLabel])
		assert.Equal(t, configFiles, service.CustomLabels[api.ConfigFilesLabel])
	}
}

// writeIdentityComposeDir writes a project whose file labels are not compose identity labels.
//
// Parameters:
//   - t: Test that owns the directory.
//
// Returns:
//   - string: Project directory path.
func writeIdentityComposeDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(`
name: webstack
services:
  api:
    image: app
    depends_on:
      - db
    labels:
      keep: "yes"
      com.docker.compose.project: from-file
      com.docker.compose.service: from-file
  db:
    image: postgres:16
`), 0o600))

	return dir
}

// TestScopeComposeProjectRejectsMissingService verifies that selecting an unknown service returns an error.
func TestScopeComposeProjectRejectsMissingService(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services:\n  app:\n    image: app\n"), 0o600))

	project, err := loadProject(t.Context(), ProjectRef{Dir: dir, Name: "project"})
	require.NoError(t, err)

	_, err = scopeComposeProject(project, []string{"missing"})
	require.Error(t, err)
	assert.ErrorContains(t, err, `no such service: missing`)
}

// composeServiceStub is a Compose service whose Up, Build, Ps, and Images calls
// run the test's functions. Any other call panics on the nil embedded service.
type composeServiceStub struct {
	api.Compose

	// up answers Up.
	up func(context.Context, *composetypes.Project, api.UpOptions) error
	// build answers Build.
	build func(context.Context, *composetypes.Project, api.BuildOptions) error
	// ps answers Ps.
	ps func(context.Context, string, api.PsOptions) ([]api.ContainerSummary, error)
	// images answers Images.
	images func(context.Context, string, api.ImagesOptions) (map[string]api.ImageSummary, error)
}

// Up delegates the stubbed Compose up call.
func (s *composeServiceStub) Up(ctx context.Context, project *composetypes.Project, options api.UpOptions) error {
	return s.up(ctx, project, options)
}

// Build delegates the stubbed Compose build call.
func (s *composeServiceStub) Build(ctx context.Context, project *composetypes.Project, options api.BuildOptions) error {
	return s.build(ctx, project, options)
}

// Ps delegates the stubbed Compose ps call.
func (s *composeServiceStub) Ps(ctx context.Context, projectName string, options api.PsOptions) ([]api.ContainerSummary, error) {
	return s.ps(ctx, projectName, options)
}

// Images delegates the stubbed Compose images call.
func (s *composeServiceStub) Images(ctx context.Context, projectName string, options api.ImagesOptions) (map[string]api.ImageSummary, error) {
	return s.images(ctx, projectName, options)
}
