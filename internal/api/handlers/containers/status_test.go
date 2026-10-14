package containers

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

func TestListContainerStatuses(t *testing.T) {
	tests := []struct {
		name    string
		client  func(t *testing.T) *mockContainer.MockClient
		filter  types.Filter
		wantErr bool
		errMsg  string
		wantLen int
	}{
		{
			name: "successful list with containers",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("test-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter:  nil,
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "successful list without filter",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("test-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter:  nil,
			wantErr: false,
			wantLen: 1,
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
			errMsg:  "failed to list containers",
		},
		{
			name: "empty container list",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{}, nil)

				return c
			},
			filter:  nil,
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "non-nil filter is passed through to ListContainers",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("filtered-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc"))
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			filter:  func(_ types.FilterableContainer) bool { return true },
			wantErr: false,
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client(t)
			statuses, err := ListContainerStatuses(t.Context(), client, tt.filter)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, statuses, tt.wantLen)
			}
		})
	}
}

func Test_containerToStatus(t *testing.T) {
	tests := []struct {
		name      string
		container func(t *testing.T) *mockTypes.MockContainer
		want      Status
	}{
		{
			name: "container without image info",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				c := mockTypes.NewMockContainer(t)
				c.EXPECT().Name().Return("my-container")
				c.EXPECT().ImageName().Return("nginx:latest")
				c.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				c.EXPECT().ImageInfo().Return(nil)

				return c
			},
			want: Status{
				Name:    "my-container",
				Image:   "nginx:latest",
				ImageID: "sha256:abc123",
				Digest:  "",
			},
		},
		{
			name: "container with image info but no repo digests",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				c := mockTypes.NewMockContainer(t)
				c.EXPECT().Name().Return("my-container")
				c.EXPECT().ImageName().Return("nginx:latest")
				c.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))

				info := &image.InspectResponse{RepoDigests: []string{}}
				c.EXPECT().ImageInfo().Return(info)

				return c
			},
			want: Status{
				Name:    "my-container",
				Image:   "nginx:latest",
				ImageID: "sha256:abc123",
				Digest:  "",
			},
		},
		{
			name: "container with valid repo digest",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				c := mockTypes.NewMockContainer(t)
				c.EXPECT().Name().Return("my-container")
				c.EXPECT().ImageName().Return("nginx:latest")
				c.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))

				info := &image.InspectResponse{RepoDigests: []string{"nginx@sha256:digest123"}}
				c.EXPECT().ImageInfo().Return(info)

				return c
			},
			want: Status{
				Name:    "my-container",
				Image:   "nginx:latest",
				ImageID: "sha256:abc123",
				Digest:  "sha256:digest123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.container(t)
			got := containerToStatus(c)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_filterStatuses(t *testing.T) {
	tests := []struct {
		name        string
		statuses    []Status
		nameFilter  string
		imageFilter string
		wantLen     int
	}{
		{
			name: "no filters returns all",
			statuses: []Status{
				{Name: "app1", Image: "nginx:latest"},
				{Name: "app2", Image: "redis:7"},
			},
			nameFilter:  "",
			imageFilter: "",
			wantLen:     2,
		},
		{
			name: "filter by name",
			statuses: []Status{
				{Name: "app1", Image: "nginx:latest"},
				{Name: "app2", Image: "redis:7"},
			},
			nameFilter:  "app1",
			imageFilter: "",
			wantLen:     1,
		},
		{
			name: "filter by image",
			statuses: []Status{
				{Name: "app1", Image: "nginx:latest"},
				{Name: "app2", Image: "redis:7"},
			},
			nameFilter:  "",
			imageFilter: "redis:7",
			wantLen:     1,
		},
		{
			name: "filter by both name and image",
			statuses: []Status{
				{Name: "app1", Image: "nginx:latest"},
				{Name: "app2", Image: "redis:7"},
			},
			nameFilter:  "app1",
			imageFilter: "nginx:latest",
			wantLen:     1,
		},
		{
			name: "no match returns empty",
			statuses: []Status{
				{Name: "app1", Image: "nginx:latest"},
			},
			nameFilter:  "nonexistent",
			imageFilter: "",
			wantLen:     0,
		},
		{
			name:        "empty statuses returns empty",
			statuses:    []Status{},
			nameFilter:  "",
			imageFilter: "",
			wantLen:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterStatuses(tt.statuses, tt.nameFilter, tt.imageFilter)
			assert.NotNil(t, got, "an empty result encodes as a JSON array, not null")
			assert.Len(t, got, tt.wantLen)
		})
	}
}
