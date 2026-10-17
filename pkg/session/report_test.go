package session

import (
	"errors"
	"testing"

	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func Test_report_Scanned(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{scanned: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single scanned container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont1"))
				// Only expect ID() since that's all we check
				return &report{scanned: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Scanned()
			if len(got) != len(tt.want) {
				t.Errorf("report.Scanned() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Scanned()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Updated(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{updated: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single updated container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont2"))

				return &report{updated: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Updated()
			if len(got) != len(tt.want) {
				t.Errorf("report.Updated() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Updated()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Failed(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{failed: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single failed container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont3"))

				return &report{failed: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont3"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Failed()
			if len(got) != len(tt.want) {
				t.Errorf("report.Failed() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Failed()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Skipped(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{skipped: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single skipped container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont4"))

				return &report{skipped: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont4"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Skipped()
			if len(got) != len(tt.want) {
				t.Errorf("report.Skipped() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Skipped()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Stale(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{stale: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single stale container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont5"))

				return &report{stale: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont5"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Stale()
			if len(got) != len(tt.want) {
				t.Errorf("report.Stale() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Stale()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Fresh(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{fresh: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single fresh container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont6"))

				return &report{fresh: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont6"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Fresh()
			if len(got) != len(tt.want) {
				t.Errorf("report.Fresh() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Fresh()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_Restarted(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []types.ContainerReport
	}{
		{
			name: "empty report",
			r:    &report{restarted: []types.ContainerReport{}},
			want: []types.ContainerReport{},
		},
		{
			name: "single restarted container",
			r: func() *report {
				mock := mockTypes.NewMockContainerReport(t)
				mock.EXPECT().ID().Return(types.ContainerID("cont7"))

				return &report{restarted: []types.ContainerReport{mock}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont7"))

					return mock
				}(),
			},
		},
		{
			name: "multiple restarted containers",
			r: func() *report {
				mock1 := mockTypes.NewMockContainerReport(t)
				mock1.EXPECT().ID().Return(types.ContainerID("cont8"))

				mock2 := mockTypes.NewMockContainerReport(t)
				mock2.EXPECT().ID().Return(types.ContainerID("cont9"))

				return &report{restarted: []types.ContainerReport{mock1, mock2}}
			}(),
			want: []types.ContainerReport{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont8"))

					return mock
				}(),
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont9"))

					return mock
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Restarted()
			if len(got) != len(tt.want) {
				t.Errorf("report.Restarted() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i].ID() != tt.want[i].ID() {
					t.Errorf(
						"report.Restarted()[%d].ID() = %v, want %v",
						i,
						got[i].ID(),
						tt.want[i].ID(),
					)
				}
			}
		})
	}
}

func Test_report_All(t *testing.T) {
	tests := []struct {
		name string
		r    *report
		want []string
	}{
		{
			name: "empty report",
			r:    &report{},
			want: []string{},
		},
		{
			name: "mixed containers with deduplication",
			r: func() *report {
				mock1 := mockTypes.NewMockContainerReport(t)
				// No strict expectation. Allow calls without failing if unmet
				mock1.EXPECT().ID().Return(types.ContainerID("cont1")).Times(0)

				mock2 := mockTypes.NewMockContainerReport(t)
				mock2.EXPECT().ID().Return(types.ContainerID("cont2")).Times(0)

				return &report{
					updated: []types.ContainerReport{mock1},
					scanned: []types.ContainerReport{mock1, mock2},
				}
			}(),
			want: []string{"cont1", "cont2"},
		},
		{
			name: "containers with restarted priority",
			r: func() *report {
				mock1 := mockTypes.NewMockContainerReport(t)
				mock1.EXPECT().ID().Return(types.ContainerID("cont1")).Times(0)

				mock2 := mockTypes.NewMockContainerReport(t)
				mock2.EXPECT().ID().Return(types.ContainerID("cont2")).Times(0)

				mock3 := mockTypes.NewMockContainerReport(t)
				mock3.EXPECT().ID().Return(types.ContainerID("cont3")).Times(0)

				return &report{
					updated:   []types.ContainerReport{mock1},
					restarted: []types.ContainerReport{mock2},
					failed:    []types.ContainerReport{mock3},
					scanned:   []types.ContainerReport{mock1, mock2, mock3},
				}
			}(),
			want: []string{"cont1", "cont2", "cont3"}, // updated first, then restarted, then failed
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.All()
			if len(got) != len(tt.want) {
				t.Errorf("report.All() length = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				t.Logf("Calling ID() on got[%d]", i)

				gotID := got[i].ID()
				if gotID != types.ContainerID(tt.want[i]) {
					t.Errorf("report.All()[%d].ID() = %v, want %v", i, gotID, tt.want[i])
				}
			}
		})
	}
}

func TestNewReport(t *testing.T) {
	type args struct {
		progress Progress
	}

	tests := []struct {
		name string
		args args
		want types.Report
	}{
		{
			name: "empty progress",
			args: args{progress: Progress{}},
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
			name: "mixed states",
			args: args{
				progress: Progress{
					"cont1": &ContainerStatus{
						state:         SkippedState,
						containerID:   "cont1",
						containerName: "container1",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image1:latest",
					},
					"cont2": &ContainerStatus{
						state:         UpdatedState,
						containerID:   "cont2",
						containerName: "container2",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image2:latest",
					},
					"cont3": &ContainerStatus{
						state:          FailedState,
						containerID:    "cont3",
						containerName:  "container3",
						oldImage:       "img1",
						newImage:       "img2",
						imageName:      "image3:latest",
						containerError: errors.New("failed"),
					},
					"cont4": &ContainerStatus{
						state:         ScannedState,
						containerID:   "cont4",
						containerName: "container4",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image4:latest",
					},
					"cont5": &ContainerStatus{
						state:         ScannedState,
						containerID:   "cont5",
						containerName: "container5",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image5:latest",
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:         UpdatedState,
						containerID:   "cont2",
						containerName: "container2",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image2:latest",
					},
					&ContainerStatus{
						state:          FailedState,
						containerID:    "cont3",
						containerName:  "container3",
						oldImage:       "img1",
						newImage:       "img2",
						imageName:      "image3:latest",
						containerError: errors.New("failed"),
					},
					&ContainerStatus{
						state:         FreshState,
						containerID:   "cont4",
						containerName: "container4",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image4:latest",
					},
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont5",
						containerName: "container5",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image5:latest",
					},
				},
				updated: []types.ContainerReport{
					&ContainerStatus{
						state:         UpdatedState,
						containerID:   "cont2",
						containerName: "container2",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image2:latest",
					},
				},
				failed: []types.ContainerReport{
					&ContainerStatus{
						state:          FailedState,
						containerID:    "cont3",
						containerName:  "container3",
						oldImage:       "img1",
						newImage:       "img2",
						imageName:      "image3:latest",
						containerError: errors.New("failed"),
					},
				},
				skipped: []types.ContainerReport{
					&ContainerStatus{
						state:         SkippedState,
						containerID:   "cont1",
						containerName: "container1",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image1:latest",
					},
				},
				stale: []types.ContainerReport{
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont5",
						containerName: "container5",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image5:latest",
					},
				},
				fresh: []types.ContainerReport{
					&ContainerStatus{
						state:         FreshState,
						containerID:   "cont4",
						containerName: "container4",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image4:latest",
					},
				},
				restarted: []types.ContainerReport{},
			},
		},
		{
			name: "stale container with unknown state",
			args: args{
				progress: Progress{
					"cont6": &ContainerStatus{
						state:         UnknownState,
						containerID:   "cont6",
						containerName: "container6",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image6:latest",
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont6",
						containerName: "container6",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image6:latest",
					},
				},
				updated: []types.ContainerReport{},
				failed:  []types.ContainerReport{},
				skipped: []types.ContainerReport{},
				stale: []types.ContainerReport{
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont6",
						containerName: "container6",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image6:latest",
					},
				},
				fresh:     []types.ContainerReport{},
				restarted: []types.ContainerReport{},
			},
		},
		{
			name: "stale container with initial stale state",
			args: args{
				progress: Progress{
					"cont7": &ContainerStatus{
						state:         StaleState,
						containerID:   "cont7",
						containerName: "container7",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image7:latest",
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont7",
						containerName: "container7",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image7:latest",
					},
				},
				updated: []types.ContainerReport{},
				failed:  []types.ContainerReport{},
				skipped: []types.ContainerReport{},
				stale: []types.ContainerReport{
					&ContainerStatus{
						state:         StaleState,
						containerID:   "cont7",
						containerName: "container7",
						oldImage:      "img1",
						newImage:      "img2",
						imageName:     "image7:latest",
					},
				},
				fresh:     []types.ContainerReport{},
				restarted: []types.ContainerReport{},
			},
		},
		{
			name: "restarted container",
			args: args{
				progress: Progress{
					"cont8": &ContainerStatus{
						state:         RestartedState,
						containerID:   "cont8",
						containerName: "container8",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image8:latest",
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont8",
						containerName: "container8",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image8:latest",
					},
				},
				updated: []types.ContainerReport{},
				failed:  []types.ContainerReport{},
				skipped: []types.ContainerReport{},
				stale:   []types.ContainerReport{},
				fresh:   []types.ContainerReport{},
				restarted: []types.ContainerReport{
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont8",
						containerName: "container8",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image8:latest",
					},
				},
			},
		},
		{
			name: "multiple restarted containers",
			args: args{
				progress: Progress{
					"cont9": &ContainerStatus{
						state:         RestartedState,
						containerID:   "cont9",
						containerName: "container9",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image9:latest",
					},
					"cont10": &ContainerStatus{
						state:         RestartedState,
						containerID:   "cont10",
						containerName: "container10",
						oldImage:      "img2",
						newImage:      "img2",
						imageName:     "image10:latest",
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont10",
						containerName: "container10",
						oldImage:      "img2",
						newImage:      "img2",
						imageName:     "image10:latest",
					},
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont9",
						containerName: "container9",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image9:latest",
					},
				},
				updated: []types.ContainerReport{},
				failed:  []types.ContainerReport{},
				skipped: []types.ContainerReport{},
				stale:   []types.ContainerReport{},
				fresh:   []types.ContainerReport{},
				restarted: []types.ContainerReport{
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont10",
						containerName: "container10",
						oldImage:      "img2",
						newImage:      "img2",
						imageName:     "image10:latest",
					},
					&ContainerStatus{
						state:         RestartedState,
						containerID:   "cont9",
						containerName: "container9",
						oldImage:      "img1",
						newImage:      "img1",
						imageName:     "image9:latest",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newReport(testLog(), tt.args.progress)
			if !compareReports(got.(*report), tt.want.(*report)) {
				t.Errorf("newReport(testLog(), ) = %+v, want %+v", got, tt.want)
			}
			// Explicitly exercise Stale() to ensure coverage
			stale := got.Stale()
			wantStale := tt.want.Stale()

			if len(stale) != len(wantStale) {
				t.Errorf("newReport(testLog(), ).Stale() length = %d, want %d", len(stale), len(wantStale))
			}

			for i := range stale {
				if stale[i].ID() != wantStale[i].ID() {
					t.Errorf(
						"newReport(testLog(), ).Stale()[%d].ID() = %v, want %v",
						i,
						stale[i].ID(),
						wantStale[i].ID(),
					)
				}
			}
		})
	}
}

// compareReports compares two report structs by their fields, ignoring pointer addresses.
func compareReports(got, want *report) bool {
	compareSlice := func(gotSlice, wantSlice []types.ContainerReport) bool {
		if len(gotSlice) != len(wantSlice) {
			return false
		}

		for i := range gotSlice {
			g := gotSlice[i].(*ContainerStatus)

			w := wantSlice[i].(*ContainerStatus)
			if g.containerID != w.containerID ||
				g.state != w.state ||
				g.oldImage != w.oldImage ||
				g.newImage != w.newImage ||
				g.containerName != w.containerName ||
				g.imageName != w.imageName ||
				g.Error() != w.Error() { // Compare error strings
				return false
			}
		}

		return true
	}

	return compareSlice(got.scanned, want.scanned) &&
		compareSlice(got.updated, want.updated) &&
		compareSlice(got.failed, want.failed) &&
		compareSlice(got.skipped, want.skipped) &&
		compareSlice(got.stale, want.stale) &&
		compareSlice(got.fresh, want.fresh) &&
		compareSlice(got.restarted, want.restarted)
}

func Test_categorizeContainer(t *testing.T) {
	type args struct {
		r      *report
		update *ContainerStatus
	}

	tests := []struct {
		name string
		args args
		want *report
	}{
		{
			name: "skipped container",
			args: args{
				r:      &report{},
				update: &ContainerStatus{state: SkippedState, containerID: "cont1"},
			},
			want: &report{
				skipped: []types.ContainerReport{
					&ContainerStatus{state: SkippedState, containerID: "cont1"},
				},
			},
		},
		{
			name: "fresh container",
			args: args{
				r: &report{},
				update: &ContainerStatus{
					state:       ScannedState,
					oldImage:    "img1",
					newImage:    "img1",
					containerID: "cont2",
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:       FreshState,
						oldImage:    "img1",
						newImage:    "img1",
						containerID: "cont2",
					},
				},
				fresh: []types.ContainerReport{
					&ContainerStatus{
						state:       FreshState,
						oldImage:    "img1",
						newImage:    "img1",
						containerID: "cont2",
					},
				},
			},
		},
		{
			name: "updated container",
			args: args{
				r: &report{},
				update: &ContainerStatus{
					state:       UpdatedState,
					oldImage:    "img1",
					newImage:    "img2",
					containerID: "cont3",
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:       UpdatedState,
						oldImage:    "img1",
						newImage:    "img2",
						containerID: "cont3",
					},
				},
				updated: []types.ContainerReport{
					&ContainerStatus{
						state:       UpdatedState,
						oldImage:    "img1",
						newImage:    "img2",
						containerID: "cont3",
					},
				},
			},
		},
		{
			name: "restarted container",
			args: args{
				r: &report{},
				update: &ContainerStatus{
					state:       RestartedState,
					oldImage:    "img1",
					newImage:    "img1",
					containerID: "cont4",
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:       RestartedState,
						oldImage:    "img1",
						newImage:    "img1",
						containerID: "cont4",
					},
				},
				restarted: []types.ContainerReport{
					&ContainerStatus{
						state:       RestartedState,
						oldImage:    "img1",
						newImage:    "img1",
						containerID: "cont4",
					},
				},
			},
		},
		{
			name: "restarted container with different images",
			args: args{
				r: &report{},
				update: &ContainerStatus{
					state:       RestartedState,
					oldImage:    "img1",
					newImage:    "img2",
					containerID: "cont5",
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{
						state:       RestartedState,
						oldImage:    "img1",
						newImage:    "img2",
						containerID: "cont5",
					},
				},
				restarted: []types.ContainerReport{
					&ContainerStatus{
						state:       RestartedState,
						oldImage:    "img1",
						newImage:    "img2",
						containerID: "cont5",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categorizeContainer(testLog(), tt.args.r, tt.args.update)

			if !compareReports(tt.args.r, tt.want) {
				t.Errorf("categorizeContainer(testLog(), ) resulted in %v, want %v", tt.args.r, tt.want)
			}
		})
	}
}

func Test_sortCategories(t *testing.T) {
	type args struct {
		r *report
	}

	tests := []struct {
		name string
		args args
		want *report
	}{
		{
			name: "unsorted scanned",
			args: args{
				r: &report{
					scanned: []types.ContainerReport{
						&ContainerStatus{containerID: "cont2"},
						&ContainerStatus{containerID: "cont1"},
					},
				},
			},
			want: &report{
				scanned: []types.ContainerReport{
					&ContainerStatus{containerID: "cont1"},
					&ContainerStatus{containerID: "cont2"},
				},
			},
		},
		{
			name: "mixed unsorted",
			args: args{
				r: &report{
					updated: []types.ContainerReport{
						&ContainerStatus{containerID: "cont3"},
						&ContainerStatus{containerID: "cont1"},
					},
					failed: []types.ContainerReport{
						&ContainerStatus{containerID: "cont4"},
						&ContainerStatus{containerID: "cont2"},
					},
				},
			},
			want: &report{
				updated: []types.ContainerReport{
					&ContainerStatus{containerID: "cont1"},
					&ContainerStatus{containerID: "cont3"},
				},
				failed: []types.ContainerReport{
					&ContainerStatus{containerID: "cont2"},
					&ContainerStatus{containerID: "cont4"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortCategories(tt.args.r)

			if !compareReports(tt.args.r, tt.want) {
				t.Errorf("sortCategories() resulted in %v, want %v", tt.args.r, tt.want)
			}
		})
	}
}

func Test_SortableContainers_Len(t *testing.T) {
	tests := []struct {
		name string
		s    sortableContainers
		want int
	}{
		{
			name: "empty slice",
			s:    sortableContainers{},
			want: 0,
		},
		{
			name: "two elements",
			s: sortableContainers{
				mockTypes.NewMockContainerReport(t),
				mockTypes.NewMockContainerReport(t),
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Len(); got != tt.want {
				t.Errorf("sortableContainers.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SortableContainers_Less(t *testing.T) {
	type args struct {
		i int
		j int
	}

	tests := []struct {
		name string
		s    sortableContainers
		args args
		want bool
	}{
		{
			name: "lower ID first",
			s: sortableContainers{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))

					return mock
				}(),
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2"))

					return mock
				}(),
			},
			args: args{i: 0, j: 1},
			want: true,
		},
		{
			name: "higher ID first",
			s: sortableContainers{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2"))

					return mock
				}(),
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont1"))

					return mock
				}(),
			},
			args: args{i: 0, j: 1},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Less(tt.args.i, tt.args.j); got != tt.want {
				t.Errorf("sortableContainers.Less() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SortableContainers_Swap(t *testing.T) {
	type args struct {
		i int
		j int
	}

	tests := []struct {
		name string
		s    sortableContainers
		args args
		want []string // Use IDs directly instead of mocks for want
	}{
		{
			name: "swap first and second",
			s: sortableContainers{
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					// Expect 1 call in the comparison loop after swap
					mock.EXPECT().ID().Return(types.ContainerID("cont1")).Times(1)

					return mock
				}(),
				func() types.ContainerReport {
					mock := mockTypes.NewMockContainerReport(t)
					mock.EXPECT().ID().Return(types.ContainerID("cont2")).Times(1)

					return mock
				}(),
			},
			args: args{i: 0, j: 1},
			want: []string{"cont2", "cont1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.s.Swap(tt.args.i, tt.args.j)

			if len(tt.s) != len(tt.want) {
				t.Errorf("sortableContainers.Swap() length = %d, want %d", len(tt.s), len(tt.want))

				return
			}

			for i := range tt.s {
				t.Logf("Calling ID() on s[%d]", i)

				gotID := tt.s[i].ID()
				if gotID != types.ContainerID(tt.want[i]) {
					t.Errorf(
						"sortableContainers.Swap()[%d].ID() = %v, want %v",
						i,
						gotID,
						tt.want[i],
					)
				}
			}
		})
	}
}
