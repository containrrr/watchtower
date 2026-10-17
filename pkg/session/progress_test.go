package session

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	testifyMock "github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func testLog() *zerolog.Logger {
	n := zerolog.Nop()

	return &n
}

func TestUpdateFromContainer(t *testing.T) {
	type args struct {
		container types.Container
		newImage  types.ImageID
		state     State
		params    types.UpdateParams
	}

	tests := []struct {
		name string
		args args
		want *ContainerStatus
	}{
		{
			name: "basic container update",
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))
					mock.EXPECT().ImageID().Return(types.ImageID("img1"))
					mock.EXPECT().Name().Return("container1")
					mock.EXPECT().ImageName().Return("image1:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				newImage: "img2",
				state:    ScannedState,
				params:   types.UpdateParams{},
			},
			want: &ContainerStatus{
				containerID:    "cont1",
				oldImage:       "img1",
				newImage:       "img2",
				containerName:  "container1",
				imageName:      "image1:latest",
				containerError: nil,
				state:          ScannedState,
				monitorOnly:    false,
			},
		},
		{
			name: "empty container fields",
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID(""))
					mock.EXPECT().ImageID().Return(types.ImageID(""))
					mock.EXPECT().Name().Return("")
					mock.EXPECT().ImageName().Return("")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				newImage: "",
				state:    UnknownState,
				params:   types.UpdateParams{},
			},
			want: &ContainerStatus{
				containerID:    "",
				oldImage:       "",
				newImage:       "",
				containerName:  "",
				imageName:      "",
				containerError: nil,
				state:          UnknownState,
				monitorOnly:    false,
			},
		},
		{
			name: "monitor-only container",
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont3"))
					mock.EXPECT().ImageID().Return(types.ImageID("img3"))
					mock.EXPECT().Name().Return("container3")
					mock.EXPECT().ImageName().Return("image3:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(true)

					return mock
				}(),
				newImage: "img4",
				state:    ScannedState,
				params:   types.UpdateParams{},
			},
			want: &ContainerStatus{
				containerID:    "cont3",
				oldImage:       "img3",
				newImage:       "img4",
				containerName:  "container3",
				imageName:      "image3:latest",
				containerError: nil,
				state:          ScannedState,
				monitorOnly:    true,
			},
		},
		{
			name: "empty monitor-only container",
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID(""))
					mock.EXPECT().ImageID().Return(types.ImageID(""))
					mock.EXPECT().Name().Return("")
					mock.EXPECT().ImageName().Return("")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(true)

					return mock
				}(),
				newImage: "",
				state:    UnknownState,
				params:   types.UpdateParams{},
			},
			want: &ContainerStatus{
				containerID:    "",
				oldImage:       "",
				newImage:       "",
				containerName:  "",
				imageName:      "",
				containerError: nil,
				state:          UnknownState,
				monitorOnly:    true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updateFromContainer(testLog(),
				tt.args.container,
				tt.args.newImage,
				tt.args.state,
				tt.args.params,
			)
			if got.containerID != tt.want.containerID ||
				got.oldImage != tt.want.oldImage ||
				got.newImage != tt.want.newImage ||
				got.containerName != tt.want.containerName ||
				got.imageName != tt.want.imageName ||
				got.state != tt.want.state ||
				got.monitorOnly != tt.want.monitorOnly {
				t.Errorf("updateFromContainer(testLog(), ) = %+v, want %+v", got, tt.want)
			}
			// Handle error field separately
			if (got.containerError == nil) != (tt.want.containerError == nil) {
				t.Errorf(
					"updateFromContainer(testLog(), ) error = %v, want %v",
					got.containerError,
					tt.want.containerError,
				)
			} else if got.containerError != nil && !errors.Is(got.containerError, tt.want.containerError) {
				t.Errorf(
					"updateFromContainer(testLog(), ) error message = %v, want %v",
					got.containerError,
					tt.want.containerError,
				)
			}
		})
	}
}

func TestProgress_AddSkipped(t *testing.T) {
	type args struct {
		container types.Container
		err       error
		params    types.UpdateParams
	}

	tests := []struct {
		name string
		m    Progress
		args args
		want Progress
	}{
		{
			name: "add skipped with error",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))
					mock.EXPECT().ImageID().Return(types.ImageID("img1"))
					mock.EXPECT().Name().Return("container1")
					mock.EXPECT().ImageName().Return("image1:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				err:    errors.New("skipped due to policy"),
				params: types.UpdateParams{},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:    "cont1",
					oldImage:       "img1",
					newImage:       "img1",
					containerName:  "container1",
					imageName:      "image1:latest",
					containerError: errors.New("skipped due to policy"),
					state:          SkippedState,
					monitorOnly:    false,
				},
			},
		},
		{
			name: "add skipped without error",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2"))
					mock.EXPECT().ImageID().Return(types.ImageID("img2"))
					mock.EXPECT().Name().Return("container2")
					mock.EXPECT().ImageName().Return("image2:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				err:    nil,
				params: types.UpdateParams{},
			},
			want: Progress{
				"cont2": &ContainerStatus{
					containerID:    "cont2",
					oldImage:       "img2",
					newImage:       "img2",
					containerName:  "container2",
					imageName:      "image2:latest",
					containerError: nil,
					state:          SkippedState,
					monitorOnly:    false,
				},
			},
		},
		{
			name: "add skipped monitor-only with error",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont3"))
					mock.EXPECT().ImageID().Return(types.ImageID("img3"))
					mock.EXPECT().Name().Return("container3")
					mock.EXPECT().ImageName().Return("image3:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(true)

					return mock
				}(),
				err:    errors.New("monitor-only skipped"),
				params: types.UpdateParams{},
			},
			want: Progress{
				"cont3": &ContainerStatus{
					containerID:    "cont3",
					oldImage:       "img3",
					newImage:       "img3",
					containerName:  "container3",
					imageName:      "image3:latest",
					containerError: errors.New("monitor-only skipped"),
					state:          SkippedState,
					monitorOnly:    true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.AddSkipped(testLog(), tt.args.container, tt.args.err, tt.args.params)

			if len(tt.m) != len(tt.want) {
				t.Errorf("Progress.AddSkipped(testLog(), ) map length = %d, want %d", len(tt.m), len(tt.want))

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state ||
					gotStatus.monitorOnly != wantStatus.monitorOnly {
					t.Errorf(
						"Progress.AddSkipped(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if gotStatus.Error() != wantStatus.Error() {
					t.Errorf(
						"Progress.AddSkipped(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.Error(),
						wantStatus.Error(),
					)
				}
			}
		})
	}
}

func TestProgress_AddFailed(t *testing.T) {
	mock := mockTypes.NewMockContainer(t)
	mock.EXPECT().ID().Return(types.ContainerID("cont1"))
	mock.EXPECT().ImageID().Return(types.ImageID("img1"))
	mock.EXPECT().Name().Return("container1")
	mock.EXPECT().ImageName().Return("image1:latest")
	mock.EXPECT().
		IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
		Return(false)

	progress := Progress{}
	failErr := errors.New("registry rate limited")
	progress.AddFailed(testLog(), mock, failErr, types.UpdateParams{})

	got := progress["cont1"]
	if got == nil {
		t.Fatal("Progress.AddFailed did not store the container")
	}

	if got.state != FailedState {
		t.Errorf("Progress.AddFailed state = %v, want %v", got.state, FailedState)
	}

	if got.Error() != failErr.Error() {
		t.Errorf("Progress.AddFailed error = %v, want %v", got.Error(), failErr.Error())
	}
}

func TestProgress_AddScanned(t *testing.T) {
	type args struct {
		container types.Container
		newImage  types.ImageID
		params    types.UpdateParams
	}

	tests := []struct {
		name string
		m    Progress
		args args
		want Progress
	}{
		{
			name: "add scanned with new image",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))
					mock.EXPECT().ImageID().Return(types.ImageID("img1"))
					mock.EXPECT().Name().Return("container1")
					mock.EXPECT().ImageName().Return("image1:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				newImage: "img2",
				params:   types.UpdateParams{},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:    "cont1",
					oldImage:       "img1",
					newImage:       "img2",
					containerName:  "container1",
					imageName:      "image1:latest",
					containerError: nil,
					state:          ScannedState,
					monitorOnly:    false,
				},
			},
		},
		{
			name: "add scanned with same image",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2"))
					mock.EXPECT().ImageID().Return(types.ImageID("img2"))
					mock.EXPECT().Name().Return("container2")
					mock.EXPECT().ImageName().Return("image2:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(false)

					return mock
				}(),
				newImage: "img2",
				params:   types.UpdateParams{},
			},
			want: Progress{
				"cont2": &ContainerStatus{
					containerID:    "cont2",
					oldImage:       "img2",
					newImage:       "img2",
					containerName:  "container2",
					imageName:      "image2:latest",
					containerError: nil,
					state:          ScannedState,
					monitorOnly:    false,
				},
			},
		},
		{
			name: "add scanned monitor-only with new image",
			m:    Progress{},
			args: args{
				container: func() types.Container {
					mock := mockTypes.NewMockContainer(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont3"))
					mock.EXPECT().ImageID().Return(types.ImageID("img3"))
					mock.EXPECT().Name().Return("container3")
					mock.EXPECT().ImageName().Return("image3:latest")
					mock.EXPECT().
						IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
						Return(true)

					return mock
				}(),
				newImage: "img4",
				params:   types.UpdateParams{},
			},
			want: Progress{
				"cont3": &ContainerStatus{
					containerID:    "cont3",
					oldImage:       "img3",
					newImage:       "img4",
					containerName:  "container3",
					imageName:      "image3:latest",
					containerError: nil,
					state:          ScannedState,
					monitorOnly:    true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.AddScanned(testLog(), tt.args.container, tt.args.newImage, tt.args.params)

			if len(tt.m) != len(tt.want) {
				t.Errorf("Progress.AddScanned(testLog(), ) map length = %d, want %d", len(tt.m), len(tt.want))

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state ||
					gotStatus.monitorOnly != wantStatus.monitorOnly {
					t.Errorf(
						"Progress.AddScanned(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if (gotStatus.containerError == nil) != (wantStatus.containerError == nil) {
					t.Errorf(
						"Progress.AddScanned(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				} else if gotStatus.containerError != nil && !errors.Is(gotStatus.containerError, wantStatus.containerError) {
					t.Errorf(
						"Progress.AddScanned(testLog(), ) error message for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				}
			}
		})
	}
}

func TestProgress_UpdateFailed(t *testing.T) {
	type args struct {
		failures map[types.ContainerID]error
	}

	tests := []struct {
		name string
		m    Progress
		args args
		want Progress
	}{
		{
			name: "update single failed container",
			m: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
			},
			args: args{
				failures: map[types.ContainerID]error{
					"cont1": errors.New("update failed"),
				},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					state:          FailedState,
					containerID:    "cont1",
					containerError: errors.New("update failed"),
				},
			},
		},
		{
			name: "update multiple failed containers",
			m: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
				"cont2": &ContainerStatus{state: UpdatedState, containerID: "cont2"},
			},
			args: args{
				failures: map[types.ContainerID]error{
					"cont1": errors.New("timeout"),
					"cont2": errors.New("permission denied"),
				},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					state:          FailedState,
					containerID:    "cont1",
					containerError: errors.New("timeout"),
				},
				"cont2": &ContainerStatus{
					state:          FailedState,
					containerID:    "cont2",
					containerError: errors.New("permission denied"),
				},
			},
		},
		{
			name: "no failures",
			m: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
			},
			args: args{
				failures: map[types.ContainerID]error{},
			},
			want: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
			},
		},
		{
			name: "failure container not in progress map",
			m: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
			},
			args: args{
				failures: map[types.ContainerID]error{
					"cont2": errors.New("container not found"),
				},
			},
			want: Progress{
				"cont1": &ContainerStatus{state: ScannedState, containerID: "cont1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.UpdateFailed(testLog(), tt.args.failures)

			if len(tt.m) != len(tt.want) {
				t.Errorf(
					"Progress.UpdateFailed(testLog(), ) map length = %d, want %d",
					len(tt.m),
					len(tt.want),
				)

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state {
					t.Errorf(
						"Progress.UpdateFailed(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if gotStatus.Error() != wantStatus.Error() {
					t.Errorf(
						"Progress.UpdateFailed(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.Error(),
						wantStatus.Error(),
					)
				}
			}
		})
	}
}

func TestProgress_Add(t *testing.T) {
	type args struct {
		update *ContainerStatus
	}

	tests := []struct {
		name string
		m    Progress
		args args
		want Progress
	}{
		{
			name: "add new container",
			m:    Progress{},
			args: args{
				update: &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
		},
		{
			name: "overwrite existing container",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: UnknownState},
			},
			args: args{
				update: &ContainerStatus{containerID: "cont1", state: UpdatedState},
			},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.Add(testLog(), tt.args.update)

			if len(tt.m) != len(tt.want) {
				t.Errorf("Progress.Add(testLog(), ) map length = %d, want %d", len(tt.m), len(tt.want))

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state {
					t.Errorf(
						"Progress.Add(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if (gotStatus.containerError == nil) != (wantStatus.containerError == nil) {
					t.Errorf(
						"Progress.Add(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				} else if gotStatus.containerError != nil && !errors.Is(gotStatus.containerError, wantStatus.containerError) {
					t.Errorf(
						"Progress.Add(testLog(), ) error message for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				}
			}
		})
	}
}

func TestProgress_MarkForUpdate(t *testing.T) {
	type args struct {
		containerID types.ContainerID
	}

	tests := []struct {
		name        string
		m           Progress
		args        args
		want        Progress
		expectPanic bool
	}{
		{
			name: "mark existing container",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
			},
			expectPanic: false,
		},
		{
			name:        "mark non-existent container",
			m:           Progress{},
			args:        args{containerID: "cont1"},
			want:        Progress{},
			expectPanic: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.expectPanic && r == nil {
					t.Errorf("expected panic, got none")
				}

				if !tt.expectPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			tt.m.MarkForUpdate(testLog(), tt.args.containerID)

			if len(tt.m) != len(tt.want) {
				t.Errorf(
					"Progress.MarkForUpdate(testLog(), ) map length = %d, want %d",
					len(tt.m),
					len(tt.want),
				)

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state {
					t.Errorf(
						"Progress.MarkForUpdate(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if (gotStatus.containerError == nil) != (wantStatus.containerError == nil) {
					t.Errorf(
						"Progress.MarkForUpdate(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				} else if gotStatus.containerError != nil && !errors.Is(gotStatus.containerError, wantStatus.containerError) {
					t.Errorf(
						"Progress.MarkForUpdate(testLog(), ) error message for %v = %v, want %v",
						id,
						gotStatus.containerError,
						wantStatus.containerError,
					)
				}
			}
		})
	}
}

func TestProgress_Report(t *testing.T) {
	tests := []struct {
		name string
		m    Progress
		want types.Report
	}{
		{
			name: "empty progress",
			m:    Progress{},
			want: &report{
				scanned:   []types.ContainerReport{},
				updated:   []types.ContainerReport{},
				failed:    []types.ContainerReport{},
				skipped:   []types.ContainerReport{},
				stale:     []types.ContainerReport{},
				fresh:     []types.ContainerReport{},
				restarted: []types.ContainerReport{},
			},
		},
		{
			name: "single scanned container",
			m: Progress{
				"cont1": &ContainerStatus{
					containerID:   "cont1",
					oldImage:      "img1",
					newImage:      "img2",
					containerName: "container1",
					imageName:     "image1:latest",
					state:         ScannedState,
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						containerID:   "cont1",
						oldImage:      "img1",
						newImage:      "img2",
						containerName: "container1",
						imageName:     "image1:latest",
						state:         StaleState, // Scanned with differing images becomes Stale
					},
				},
				updated: []types.ContainerReport{},
				failed:  []types.ContainerReport{},
				skipped: []types.ContainerReport{},
				stale: []types.ContainerReport{
					&ContainerStatus{
						containerID:   "cont1",
						oldImage:      "img1",
						newImage:      "img2",
						containerName: "container1",
						imageName:     "image1:latest",
						state:         StaleState,
					},
				},
				fresh:     []types.ContainerReport{},
				restarted: []types.ContainerReport{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.Report(testLog()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Progress.Report(testLog(), ) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgress_MarkRestarted(t *testing.T) {
	type args struct {
		containerID types.ContainerID
	}

	tests := []struct {
		name        string
		m           Progress
		args        args
		want        Progress
		expectPanic bool
	}{
		{
			name: "mark existing container as restarted from updated state",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: RestartedState},
			},
			expectPanic: false,
		},
		{
			name: "mark existing container as restarted from scanned state",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: RestartedState},
			},
			expectPanic: false,
		},
		{
			name: "mark existing container as restarted from failed state",
			m: Progress{
				"cont1": &ContainerStatus{
					containerID:    "cont1",
					state:          FailedState,
					containerError: errors.New("fail"),
				},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:    "cont1",
					state:          RestartedState,
					containerError: errors.New("fail"),
				},
			},
			expectPanic: false,
		},
		{
			name: "mark existing container as restarted from skipped state",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: SkippedState},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: RestartedState},
			},
			expectPanic: false,
		},
		{
			name: "mark already restarted container",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: RestartedState},
			},
			args: args{containerID: "cont1"},
			want: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: RestartedState},
			},
			expectPanic: false,
		},
		{
			name:        "mark non-existent container as restarted",
			m:           Progress{},
			args:        args{containerID: "cont1"},
			want:        Progress{},
			expectPanic: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.expectPanic && r == nil {
					t.Errorf("expected panic, got none")
				}

				if !tt.expectPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			tt.m.MarkRestarted(testLog(), tt.args.containerID)

			if len(tt.m) != len(tt.want) {
				t.Errorf(
					"Progress.MarkRestarted(testLog(), ) map length = %d, want %d",
					len(tt.m),
					len(tt.want),
				)

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.oldImage != wantStatus.oldImage ||
					gotStatus.newImage != wantStatus.newImage ||
					gotStatus.containerName != wantStatus.containerName ||
					gotStatus.imageName != wantStatus.imageName ||
					gotStatus.state != wantStatus.state {
					t.Errorf(
						"Progress.MarkRestarted(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if gotStatus.Error() != wantStatus.Error() {
					t.Errorf(
						"Progress.MarkRestarted(testLog(), ) error for %v = %v, want %v",
						id,
						gotStatus.Error(),
						wantStatus.Error(),
					)
				}
			}
		})
	}
}

func TestProgress_MarkRestarted_UpdateFailed_Integration(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
	}

	// Mark as restarted
	m.MarkRestarted(testLog(), "cont1")

	if m["cont1"].state != RestartedState {
		t.Errorf("Expected state RestartedState, got %v", m["cont1"].state)
	}

	// Then mark as failed
	failures := map[types.ContainerID]error{
		"cont1": errors.New("restart failed"),
	}
	m.UpdateFailed(testLog(), failures)

	if m["cont1"].state != FailedState {
		t.Errorf("Expected state FailedState after UpdateFailed, got %v", m["cont1"].state)
	}

	if m["cont1"].containerError == nil || m["cont1"].containerError.Error() != "restart failed" {
		t.Errorf("Expected error 'restart failed', got %v", m["cont1"].containerError)
	}
}

func TestProgress_MarkRestarted_AddSkipped_Integration(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
	}

	// Mark as restarted
	m.MarkRestarted(testLog(), "cont1")

	if m["cont1"].state != RestartedState {
		t.Errorf("Expected state RestartedState, got %v", m["cont1"].state)
	}

	// Then add as skipped (this should overwrite)
	mock := mockTypes.NewMockContainer(t)
	mock.EXPECT().ID().Return(types.ContainerID("cont1"))
	mock.EXPECT().ImageID().Return(types.ImageID("img1"))
	mock.EXPECT().Name().Return("container1")
	mock.EXPECT().ImageName().Return("image1:latest")
	mock.EXPECT().
		IsMonitorOnly(testifyMock.MatchedBy(func(_ types.UpdateParams) bool { return true })).
		Return(false)

	m.AddSkipped(testLog(), mock, errors.New("skipped after restart"), types.UpdateParams{})

	if m["cont1"].state != SkippedState {
		t.Errorf("Expected state SkippedState after AddSkipped, got %v", m["cont1"].state)
	}

	if m["cont1"].containerError == nil ||
		m["cont1"].containerError.Error() != "skipped after restart" {
		t.Errorf("Expected error 'skipped after restart', got %v", m["cont1"].containerError)
	}
}

func TestProgress_Restarted_With_Error(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{
			containerID:    "cont1",
			state:          FailedState,
			containerError: errors.New("initial error"),
		},
	}

	// Mark as restarted, error should persist
	m.MarkRestarted(testLog(), "cont1")

	if m["cont1"].state != RestartedState {
		t.Errorf("Expected state RestartedState, got %v", m["cont1"].state)
	}

	if m["cont1"].containerError == nil || m["cont1"].containerError.Error() != "initial error" {
		t.Errorf("Expected error 'initial error' to persist, got %v", m["cont1"].containerError)
	}
}

func TestProgress_Report_With_Restarted_In_All(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{
			containerID:   "cont1",
			state:         RestartedState,
			containerName: "container1",
		},
		"cont2": &ContainerStatus{
			containerID:   "cont2",
			state:         FailedState,
			containerName: "container2",
		},
	}

	report := m.Report(testLog())

	all := report.All()

	// Check that restarted container is included in All()
	foundRestarted := false
	foundFailed := false

	for _, c := range all {
		if c.ID() == "cont1" {
			foundRestarted = true
		}

		if c.ID() == "cont2" {
			foundFailed = true
		}
	}

	if !foundRestarted {
		t.Errorf("Restarted container not found in All()")
	}

	if !foundFailed {
		t.Errorf("Failed container not found in All()")
	}

	// Check restarted list
	restarted := report.Restarted()
	if len(restarted) != 1 || restarted[0].ID() != "cont1" {
		t.Errorf("Expected one restarted container cont1, got %v", restarted)
	}
}

func TestProgress_MarkRestarted_Concurrent(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{containerID: "cont1", state: UpdatedState},
		"cont2": &ContainerStatus{containerID: "cont2", state: ScannedState},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		m.MarkRestarted(testLog(), "cont1")
	}()

	go func() {
		defer wg.Done()

		m.MarkRestarted(testLog(), "cont2")
	}()

	wg.Wait()

	if m["cont1"].state != RestartedState {
		t.Errorf("cont1 not marked as restarted")
	}

	if m["cont2"].state != RestartedState {
		t.Errorf("cont2 not marked as restarted")
	}
}

func TestProgress_SetCooldownInfo(t *testing.T) {
	type args struct {
		containerID types.ContainerID
		age         string
		delay       string
		remaining   string
		eligibleAt  time.Time
	}

	tests := []struct {
		name string
		m    Progress
		args args
		want Progress
	}{
		{
			name: "set cooldown info on existing container - passed",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
			args: args{
				containerID: "cont1",
				age:         "47 days, 11 hours",
				delay:       "24 hours",
				remaining:   "",
				eligibleAt:  time.Time{},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:        "cont1",
					state:              ScannedState,
					cooldownAge:        "47 days, 11 hours",
					cooldownDelay:      "24 hours",
					cooldownRemaining:  "",
					cooldownEligibleAt: time.Time{},
				},
			},
		},
		{
			name: "set cooldown info on existing container - not passed",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: SkippedState},
			},
			args: args{
				containerID: "cont1",
				age:         "2 hours",
				delay:       "24 hours",
				remaining:   "22 hours",
				eligibleAt:  time.Date(2026, 5, 26, 0, 45, 0, 0, time.UTC),
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:        "cont1",
					state:              SkippedState,
					cooldownAge:        "2 hours",
					cooldownDelay:      "24 hours",
					cooldownRemaining:  "22 hours",
					cooldownEligibleAt: time.Date(2026, 5, 26, 0, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "set cooldown info on non-existent container - no panic",
			m:    Progress{},
			args: args{
				containerID: "cont1",
				age:         "1 day",
				delay:       "24 hours",
				remaining:   "",
				eligibleAt:  time.Time{},
			},
			want: Progress{},
		},
		{
			name: "overwrite existing cooldown info",
			m: Progress{
				"cont1": &ContainerStatus{
					containerID:       "cont1",
					state:             ScannedState,
					cooldownAge:       "old age",
					cooldownDelay:     "old delay",
					cooldownRemaining: "old remaining",
				},
			},
			args: args{
				containerID: "cont1",
				age:         "new age",
				delay:       "new delay",
				remaining:   "new remaining",
				eligibleAt:  time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:        "cont1",
					state:              ScannedState,
					cooldownAge:        "new age",
					cooldownDelay:      "new delay",
					cooldownRemaining:  "new remaining",
					cooldownEligibleAt: time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "set cooldown with empty values",
			m: Progress{
				"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
			},
			args: args{
				containerID: "cont1",
				age:         "",
				delay:       "",
				remaining:   "",
				eligibleAt:  time.Time{},
			},
			want: Progress{
				"cont1": &ContainerStatus{
					containerID:       "cont1",
					state:             ScannedState,
					cooldownAge:       "",
					cooldownDelay:     "",
					cooldownRemaining: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.SetCooldownInfo(testLog(), tt.args.containerID, tt.args.age, tt.args.delay, tt.args.remaining, tt.args.eligibleAt)

			if len(tt.m) != len(tt.want) {
				t.Errorf(
					"Progress.SetCooldownInfo(testLog(), ) map length = %d, want %d",
					len(tt.m),
					len(tt.want),
				)

				return
			}

			for id, gotStatus := range tt.m {
				wantStatus := tt.want[id]
				if gotStatus.containerID != wantStatus.containerID ||
					gotStatus.state != wantStatus.state {
					t.Errorf(
						"Progress.SetCooldownInfo(testLog(), ) status for %v = %+v, want %+v",
						id,
						gotStatus,
						wantStatus,
					)
				}

				if gotStatus.CooldownAge() != wantStatus.cooldownAge {
					t.Errorf(
						"Progress.SetCooldownInfo(testLog(), ) CooldownAge for %v = %v, want %v",
						id,
						gotStatus.CooldownAge(),
						wantStatus.cooldownAge,
					)
				}

				if gotStatus.CooldownDelay() != wantStatus.cooldownDelay {
					t.Errorf(
						"Progress.SetCooldownInfo(testLog(), ) CooldownDelay for %v = %v, want %v",
						id,
						gotStatus.CooldownDelay(),
						wantStatus.cooldownDelay,
					)
				}

				if gotStatus.CooldownRemaining() != wantStatus.cooldownRemaining {
					t.Errorf(
						"Progress.SetCooldownInfo(testLog(), ) CooldownRemaining for %v = %v, want %v",
						id,
						gotStatus.CooldownRemaining(),
						wantStatus.cooldownRemaining,
					)
				}

				if gotStatus.CooldownEligibleAt() != wantStatus.cooldownEligibleAt {
					t.Errorf(
						"Progress.SetCooldownInfo(testLog(), ) CooldownEligibleAt for %v = %v, want %v",
						id,
						gotStatus.CooldownEligibleAt(),
						wantStatus.cooldownEligibleAt,
					)
				}
			}
		})
	}
}

func TestProgress_SetCooldownInfo_Concurrent(t *testing.T) {
	m := Progress{
		"cont1": &ContainerStatus{containerID: "cont1", state: ScannedState},
		"cont2": &ContainerStatus{containerID: "cont2", state: ScannedState},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		m.SetCooldownInfo(testLog(), "cont1", "1 day", "24 hours", "", time.Time{})
	}()

	go func() {
		defer wg.Done()

		m.SetCooldownInfo(testLog(), "cont2", "2 hours", "24 hours", "22 hours", time.Date(2026, 5, 26, 0, 45, 0, 0, time.UTC))
	}()

	wg.Wait()

	if m["cont1"].CooldownAge() != "1 day" {
		t.Errorf("cont1 CooldownAge = %v, want '1 day'", m["cont1"].CooldownAge())
	}

	if m["cont2"].CooldownRemaining() != "22 hours" {
		t.Errorf("cont2 CooldownRemaining = %v, want '22 hours'", m["cont2"].CooldownRemaining())
	}
}

func TestApplyReportMeta(t *testing.T) {
	t.Parallel()

	t.Run("nil status", func(t *testing.T) {
		t.Parallel()

		applyReportMeta(nil, reportTestContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://github.com/org/app.git",
		}), types.UpdateParams{})
	})

	t.Run("skips mock containers", func(t *testing.T) {
		t.Parallel()

		mock := mockTypes.NewMockContainer(t)
		status := &ContainerStatus{}
		applyReportMeta(status, mock, types.UpdateParams{})
		assert.Empty(t, status.GitRepo())
	})

	t.Run("copies git and oci fields from a concrete container", func(t *testing.T) {
		t.Parallel()

		c := reportTestContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://github.com/org/app.git",
			gitPkg.RefLabel:  "main",
		})
		got := updateFromContainer(testLog(), c, "img2", ScannedState, types.UpdateParams{})
		assert.Equal(t, "https://github.com/org/app.git", got.GitRepo())
		assert.Equal(t, "main", got.GitRef())
		assert.Equal(t, "https://github.com/org/app/releases", got.Changelog())
	})
}

func TestProgressSetLatestImage(t *testing.T) {
	t.Parallel()

	id := types.ContainerID("cont1")
	progress := Progress{
		id: &ContainerStatus{containerID: id, newImage: "old"},
	}

	progress.SetLatestImage(testLog(), id, "sha256:gitbuilt")
	assert.Equal(t, types.ImageID("sha256:gitbuilt"), progress[id].LatestImageID())

	progress.SetLatestImage(testLog(), "missing", "ignored")
	assert.NotContains(t, progress, types.ContainerID("missing"))
}

func TestProgressSetLatestImageMeta(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		meta := Progress{}.SetLatestImageMeta(
			testLog(),
			nil,
			types.UpdateParams{},
			container.ChangelogVars{},
			oci.Annotations{},
		)
		assert.Equal(t, container.ReportMeta{}, meta)
	})

	t.Run("unknown container", func(t *testing.T) {
		t.Parallel()

		c := reportTestContainer(t, map[string]string{
			gitPkg.ChangelogURLLabel: "https://example.com/{tag}",
		})
		meta := Progress{}.SetLatestImageMeta(
			testLog(),
			c,
			types.UpdateParams{},
			container.ChangelogVars{Tag: "v1.0.0"},
			oci.Annotations{},
		)
		assert.Equal(t, container.ReportMeta{}, meta)
	})

	t.Run("substitutes new tag", func(t *testing.T) {
		t.Parallel()

		c := reportTestContainer(t, map[string]string{
			gitPkg.RepoLabel:         "https://github.com/org/app.git",
			gitPkg.ChangelogURLLabel: "https://example.com/notes/{tag}",
		})
		status := updateFromContainer(testLog(), c, "img", ScannedState, types.UpdateParams{})
		progress := Progress{c.ID(): status}

		progress.SetLatestImageMeta(
			testLog(),
			c,
			types.UpdateParams{},
			container.ChangelogVars{Tag: "v1.2.3", Commit: "deadbeef"},
			oci.Annotations{},
		)
		assert.Equal(t, "https://example.com/notes/v1.2.3", status.Changelog())
	})

	t.Run("derives a versioned releases url from a probed tag", func(t *testing.T) {
		t.Parallel()

		c := reportTestContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://github.com/org/app.git",
		})
		status := updateFromContainer(testLog(), c, "img", ScannedState, types.UpdateParams{})
		progress := Progress{c.ID(): status}

		meta := progress.SetLatestImageMeta(
			testLog(),
			c,
			types.UpdateParams{},
			container.ChangelogVars{Tag: "v1.2.3"},
			oci.Annotations{Version: "1.2.3", Revision: "newrev"},
		)

		assert.Equal(t, "https://github.com/org/app/releases/tag/v1.2.3", meta.Changelog)
		assert.Equal(t, "https://github.com/org/app/releases/tag/v1.2.3", status.Changelog())
		assert.Equal(t, "1.2.3", status.LatestImageVersion())
		assert.Equal(t, "newrev", status.LatestImageRevision())
		assert.Empty(t, status.CurrentImageVersion())
	})

	t.Run("a git rebuild leaves the latest version empty", func(t *testing.T) {
		t.Parallel()

		c := reportTestContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://github.com/org/app.git",
		})
		status := updateFromContainer(testLog(), c, "img", ScannedState, types.UpdateParams{})
		progress := Progress{c.ID(): status}

		progress.SetLatestImageMeta(
			testLog(),
			c,
			types.UpdateParams{},
			container.ChangelogVars{Tag: "v9.9.9", Commit: "deadbeef"},
			oci.Annotations{},
		)

		assert.Equal(t, "https://github.com/org/app/releases/tag/v9.9.9", status.Changelog())
		assert.Empty(t, status.LatestImageVersion())
	})
}

// reportTestContainer builds a concrete container used to exercise applyReportMeta.
//
// Parameters:
//   - t: Test handle.
//   - labels: Container config labels.
//
// Returns:
//   - *container.Container: Container with inspect metadata.
func reportTestContainer(t *testing.T, labels map[string]string) *container.Container {
	t.Helper()

	cfg := &dockerspec.DockerOCIImageConfig{}
	cfg.Labels = map[string]string{}

	return container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name: "/app",
		Config: &dockerContainer.Config{
			Image:  "myapp:latest",
			Labels: labels,
		},
	}, &dockerImage.InspectResponse{
		ID:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Config:   cfg,
		RepoTags: []string{"myapp:latest"},
	})
}
