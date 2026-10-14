package images

import (
	"errors"
	"testing"

	"github.com/moby/moby/api/types/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func TestListImageStatuses(t *testing.T) {
	tests := []struct {
		name    string
		client  func(t *testing.T) *mockContainer.MockClient
		filter  types.Filter
		wantErr bool
		want    []ImageStatus
	}{
		{
			name: "successful list with single image",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "nginx:latest", ImageID: "sha256:abc123", Containers: 1},
			},
		},
		{
			name: "multiple containers same image",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)

				container1 := mockTypes.NewMockContainer(t)
				container1.EXPECT().ImageName().Return("nginx:latest")
				container1.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container1.EXPECT().ImageInfo().Return(nil)

				container2 := mockTypes.NewMockContainer(t)
				container2.EXPECT().ImageName().Return("nginx:latest")
				container2.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))

				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container1, container2}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "nginx:latest", ImageID: "sha256:abc123", Containers: 2},
			},
		},
		{
			name: "digest extracted from canonical repo digest",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"docker.io/library/nginx@sha256:digest123"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "nginx:latest", ImageID: "sha256:abc123", Digest: "sha256:digest123", Containers: 1},
			},
		},
		{
			name: "digest extracted from short name repo digest",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("myapp:1.0")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:def456"))
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"myapp@sha256:shortname"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "myapp:1.0", ImageID: "sha256:def456", Digest: "sha256:shortname", Containers: 1},
			},
		},
		{
			name: "digest extracted from private registry",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("registry.example.com/team/app:v2")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:ghi789"))
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"registry.example.com/team/app@sha256:privreg"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "registry.example.com/team/app:v2", ImageID: "sha256:ghi789", Digest: "sha256:privreg", Containers: 1},
			},
		},
		{
			name: "multiple digests uses first",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"docker.io/library/nginx@sha256:first", "nginx@sha256:second"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "nginx:latest", ImageID: "sha256:abc123", Digest: "sha256:first", Containers: 1},
			},
		},
		{
			name: "multiple repositories uses the container's repository",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().ImageName().Return("ghcr.io/org/app:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"docker.io/org/app@sha256:hub", "ghcr.io/org/app@sha256:ghcr"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter: nil,
			want: []ImageStatus{
				{Name: "ghcr.io/org/app:latest", ImageID: "sha256:abc123", Digest: "sha256:ghcr", Containers: 1},
			},
		},
		{
			name: "client error returns wrapped error",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				c.EXPECT().ListContainers(mock.Anything).Return(nil, errors.New("connection refused"))

				return c
			},
			filter:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client(t)
			statuses, err := ListImageStatuses(t.Context(), client, tt.filter)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, statuses, len(tt.want))

				for i, want := range tt.want {
					assert.Equal(t, want.Name, statuses[i].Name)
					assert.Equal(t, want.ImageID, statuses[i].ImageID)
					assert.Equal(t, want.Digest, statuses[i].Digest)
					assert.Equal(t, want.Containers, statuses[i].Containers)
				}
			}
		})
	}
}

// TestListImageStatuses_SortsByNameThenImageID verifies that images are
// returned in a fixed order, by name and then by image ID for one name that
// points at several local images.
func TestListImageStatuses_SortsByNameThenImageID(t *testing.T) {
	t.Parallel()

	newContainer := func(name, imageID string) types.Container {
		c := mockTypes.NewMockContainer(t)
		c.EXPECT().ImageName().Return(name)
		c.EXPECT().ImageID().Return(types.ImageID(imageID))
		c.EXPECT().ImageInfo().Return(nil)

		return c
	}

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{
		newContainer("redis:7", "sha256:bbb"),
		newContainer("nginx:1.27", "sha256:zzz"),
		newContainer("ghcr.io/org/app:latest", "sha256:ccc"),
		newContainer("nginx:1.27", "sha256:aaa"),
	}, nil)

	statuses, err := ListImageStatuses(t.Context(), client, nil)
	require.NoError(t, err)

	got := make([]string, 0, len(statuses))
	for _, status := range statuses {
		got = append(got, status.Name+"@"+status.ImageID)
	}

	assert.Equal(t, []string{
		"ghcr.io/org/app:latest@sha256:ccc",
		"nginx:1.27@sha256:aaa",
		"nginx:1.27@sha256:zzz",
		"redis:7@sha256:bbb",
	}, got)
}

// TestListImageStatuses_NameForSeveralTags verifies that containers running one
// image under several tags are reported under the same name in either list
// order.
func TestListImageStatuses_NameForSeveralTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		names []string
	}{
		{name: "latest listed first", names: []string{"nginx:latest", "nginx:1.27"}},
		{name: "version listed first", names: []string{"nginx:1.27", "nginx:latest"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			containers := make([]types.Container, 0, len(tt.names))

			for _, name := range tt.names {
				c := mockTypes.NewMockContainer(t)
				c.EXPECT().ImageName().Return(name)
				c.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				c.EXPECT().ImageInfo().Return(nil).Maybe()

				containers = append(containers, c)
			}

			client := mockContainer.NewMockClient(t)
			client.EXPECT().ListContainers(mock.Anything).Return(containers, nil)

			statuses, err := ListImageStatuses(t.Context(), client, nil)
			require.NoError(t, err)
			require.Len(t, statuses, 1)
			assert.Equal(t, "nginx:1.27", statuses[0].Name)
			assert.Equal(t, 2, statuses[0].Containers)
		})
	}
}

func Test_filterImages(t *testing.T) {
	statuses := []ImageStatus{
		{Name: "nginx:latest", ImageID: "sha256:abc"},
		{Name: "redis:latest", ImageID: "sha256:def"},
	}

	tests := []struct {
		name    string
		nameF   string
		idF     string
		wantLen int
	}{
		{name: "no filter", nameF: "", idF: "", wantLen: 2},
		{name: "filter by name", nameF: "nginx:latest", idF: "", wantLen: 1},
		{name: "filter by id", nameF: "", idF: "sha256:abc", wantLen: 1},
		{name: "no match", nameF: "postgres:latest", idF: "", wantLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterImages(statuses, tt.nameF, tt.idF)
			assert.Len(t, result, tt.wantLen)
		})
	}
}
