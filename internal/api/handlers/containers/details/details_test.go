package details

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

func TestGetContainerDetails(t *testing.T) {
	tests := []struct {
		name    string
		client  func(t *testing.T) *mockContainer.MockClient
		filter  types.Filter
		params  types.UpdateParams
		wantErr bool
		wantLen int
		want    func(t *testing.T, details []ContainerDetails)
	}{
		{
			name: "successful list with labeled enabled container",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("test-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().IsRunning().Return(true)
				container.EXPECT().IsWatchtower().Return(false)
				container.EXPECT().IsMonitorOnly(mock.Anything).Return(false)
				container.EXPECT().IsNoPull(mock.Anything).Return(false)
				container.EXPECT().Enabled().Return(true, true)
				container.EXPECT().IsStale().Return(false)
				container.EXPECT().Scope().Return("", true)
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			params:  types.UpdateParams{},
			wantErr: false,
			wantLen: 1,
			want: func(t *testing.T, details []ContainerDetails) {
				t.Helper()
				assert.True(t, details[0].Enabled)
			},
		},
		{
			name: "unlabeled container is enabled when label-enable is false",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("unlabeled-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().IsRunning().Return(true)
				container.EXPECT().IsWatchtower().Return(false)
				container.EXPECT().IsMonitorOnly(mock.Anything).Return(false)
				container.EXPECT().IsNoPull(mock.Anything).Return(false)
				container.EXPECT().Enabled().Return(false, false)
				container.EXPECT().IsStale().Return(false)
				container.EXPECT().Scope().Return("", true)
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			params:  types.UpdateParams{LabelEnable: false},
			wantErr: false,
			wantLen: 1,
			want: func(t *testing.T, details []ContainerDetails) {
				t.Helper()
				assert.True(t, details[0].Enabled)
			},
		},
		{
			name: "unlabeled container is disabled when label-enable is true",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("unlabeled-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().IsRunning().Return(true)
				container.EXPECT().IsWatchtower().Return(false)
				container.EXPECT().IsMonitorOnly(mock.Anything).Return(false)
				container.EXPECT().IsNoPull(mock.Anything).Return(false)
				container.EXPECT().Enabled().Return(false, false)
				container.EXPECT().IsStale().Return(false)
				container.EXPECT().Scope().Return("", true)
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			params:  types.UpdateParams{LabelEnable: true},
			wantErr: false,
			wantLen: 1,
			want: func(t *testing.T, details []ContainerDetails) {
				t.Helper()
				assert.False(t, details[0].Enabled)
			},
		},
		{
			name: "labeled false container is disabled when label-enable is false",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("disabled-container")
				container.EXPECT().ImageName().Return("nginx:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().IsRunning().Return(true)
				container.EXPECT().IsWatchtower().Return(false)
				container.EXPECT().IsMonitorOnly(mock.Anything).Return(false)
				container.EXPECT().IsNoPull(mock.Anything).Return(false)
				container.EXPECT().Enabled().Return(false, true)
				container.EXPECT().IsStale().Return(false)
				container.EXPECT().Scope().Return("", true)
				container.EXPECT().ImageInfo().Return(nil)
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			params:  types.UpdateParams{LabelEnable: false},
			wantErr: false,
			wantLen: 1,
			want: func(t *testing.T, details []ContainerDetails) {
				t.Helper()
				assert.False(t, details[0].Enabled)
			},
		},
		{
			name: "digest from the container's repository when the image has several",
			client: func(t *testing.T) *mockContainer.MockClient {
				t.Helper()
				c := mockContainer.NewMockClient(t)
				container := mockTypes.NewMockContainer(t)
				container.EXPECT().Name().Return("app")
				container.EXPECT().ImageName().Return("ghcr.io/org/app:latest")
				container.EXPECT().ImageID().Return(types.ImageID("sha256:abc123"))
				container.EXPECT().IsRunning().Return(true)
				container.EXPECT().IsWatchtower().Return(false)
				container.EXPECT().IsMonitorOnly(mock.Anything).Return(false)
				container.EXPECT().IsNoPull(mock.Anything).Return(false)
				container.EXPECT().Enabled().Return(false, false)
				container.EXPECT().IsStale().Return(false)
				container.EXPECT().Scope().Return("", true)
				container.EXPECT().ImageInfo().Return(&image.InspectResponse{
					RepoDigests: []string{"docker.io/org/app@sha256:hub", "ghcr.io/org/app@sha256:ghcr"},
				})
				c.EXPECT().ListContainers(mock.Anything).Return([]types.Container{container}, nil)

				return c
			},
			params:  types.UpdateParams{},
			wantErr: false,
			wantLen: 1,
			want: func(t *testing.T, details []ContainerDetails) {
				t.Helper()
				assert.Equal(t, "sha256:ghcr", details[0].Digest)
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
			params:  types.UpdateParams{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client(t)
			details, err := GetContainerDetails(t.Context(), client, tt.filter, "", "", tt.params)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, details, tt.wantLen)

				if tt.want != nil {
					tt.want(t, details)
				}
			}
		})
	}
}
