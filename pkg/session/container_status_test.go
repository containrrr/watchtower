package session

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestContainerStatus_ID(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want types.ContainerID
	}{
		{
			name: "valid container ID",
			u:    &ContainerStatus{containerID: "cont1"},
			want: "cont1",
		},
		{
			name: "empty container ID",
			u:    &ContainerStatus{containerID: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.ID(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ContainerStatus.ID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_Name(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want string
	}{
		{
			name: "valid container name",
			u:    &ContainerStatus{containerName: "my-container"},
			want: "my-container",
		},
		{
			name: "empty container name",
			u:    &ContainerStatus{containerName: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.Name(); got != tt.want {
				t.Errorf("ContainerStatus.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_CurrentImageID(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want types.ImageID
	}{
		{
			name: "valid current image ID",
			u:    &ContainerStatus{oldImage: "img123"},
			want: "img123",
		},
		{
			name: "empty current image ID",
			u:    &ContainerStatus{oldImage: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.CurrentImageID(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ContainerStatus.CurrentImageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_LatestImageID(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want types.ImageID
	}{
		{
			name: "valid latest image ID",
			u:    &ContainerStatus{newImage: "img456"},
			want: "img456",
		},
		{
			name: "empty latest image ID",
			u:    &ContainerStatus{newImage: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.LatestImageID(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ContainerStatus.LatestImageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_ImageName(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want string
	}{
		{
			name: "valid image name",
			u:    &ContainerStatus{imageName: "myimage:latest"},
			want: "myimage:latest",
		},
		{
			name: "empty image name",
			u:    &ContainerStatus{imageName: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.ImageName(); got != tt.want {
				t.Errorf("ContainerStatus.ImageName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_Error(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want string
	}{
		{
			name: "no error",
			u:    &ContainerStatus{containerError: nil},
			want: "",
		},
		{
			name: "with error",
			u:    &ContainerStatus{containerError: errors.New("update failed")},
			want: "update failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.Error(); got != tt.want {
				t.Errorf("ContainerStatus.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_State(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want string
	}{
		{
			name: "unknown state",
			u:    &ContainerStatus{state: UnknownState},
			want: "Unknown",
		},
		{
			name: "skipped state",
			u:    &ContainerStatus{state: SkippedState},
			want: "Skipped",
		},
		{
			name: "scanned state",
			u:    &ContainerStatus{state: ScannedState},
			want: "Scanned",
		},
		{
			name: "updated state",
			u:    &ContainerStatus{state: UpdatedState},
			want: "Updated",
		},
		{
			name: "failed state",
			u:    &ContainerStatus{state: FailedState},
			want: "Failed",
		},
		{
			name: "fresh state",
			u:    &ContainerStatus{state: FreshState},
			want: "Fresh",
		},
		{
			name: "stale state",
			u:    &ContainerStatus{state: StaleState},
			want: "Stale",
		},
		{
			name: "restarted state",
			u:    &ContainerStatus{state: RestartedState},
			want: RestartedStateString,
		},
		{
			name: "invalid state",
			u:    &ContainerStatus{state: State(999)}, // Beyond defined states
			want: "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.State(); got != tt.want {
				t.Errorf("ContainerStatus.State() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerStatus_RestartedStateBehavior(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
	}{
		{
			name: "restarted state with all fields",
			u: &ContainerStatus{
				containerID:    "cont1",
				oldImage:       "img123",
				newImage:       "img456",
				containerName:  "my-container",
				imageName:      "nginx:latest",
				containerError: nil,
				state:          RestartedState,
				monitorOnly:    false,
				newContainerID: "newcont1",
			},
		},
		{
			name: "restarted state with minimal fields",
			u: &ContainerStatus{
				containerID: "cont1",
				state:       RestartedState,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify state is Restarted
			if got := tt.u.State(); got != RestartedStateString {
				t.Errorf("ContainerStatus.State() = %v, want %v", got, RestartedStateString)
			}
			// Verify other methods return expected values
			if got := tt.u.ID(); got != tt.u.containerID {
				t.Errorf("ContainerStatus.ID() = %v, want %v", got, tt.u.containerID)
			}

			if got := tt.u.Name(); got != tt.u.containerName {
				t.Errorf("ContainerStatus.Name() = %v, want %v", got, tt.u.containerName)
			}

			if got := tt.u.CurrentImageID(); got != tt.u.oldImage {
				t.Errorf("ContainerStatus.CurrentImageID() = %v, want %v", got, tt.u.oldImage)
			}

			if got := tt.u.LatestImageID(); got != tt.u.newImage {
				t.Errorf("ContainerStatus.LatestImageID() = %v, want %v", got, tt.u.newImage)
			}

			if got := tt.u.ImageName(); got != tt.u.imageName {
				t.Errorf("ContainerStatus.ImageName() = %v, want %v", got, tt.u.imageName)
			}

			if got := tt.u.IsMonitorOnly(); got != tt.u.monitorOnly {
				t.Errorf("ContainerStatus.IsMonitorOnly() = %v, want %v", got, tt.u.monitorOnly)
			}

			if got := tt.u.NewContainerID(); got != tt.u.newContainerID {
				t.Errorf("ContainerStatus.NewContainerID() = %v, want %v", got, tt.u.newContainerID)
			}
		})
	}
}

func TestContainerStatus_RestartedStateErrorHandling(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
		want string
	}{
		{
			name: "restarted state with no error",
			u: &ContainerStatus{
				state:          RestartedState,
				containerError: nil,
			},
			want: "",
		},
		{
			name: "restarted state with error",
			u: &ContainerStatus{
				state:          RestartedState,
				containerError: errors.New("restart failed"),
			},
			want: "restart failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.u.Error(); got != tt.want {
				t.Errorf("ContainerStatus.Error() = %v, want %v", got, tt.want)
			}
			// Ensure state is still Restarted
			if got := tt.u.State(); got != RestartedStateString {
				t.Errorf("ContainerStatus.State() = %v, want %v", got, RestartedStateString)
			}
		})
	}
}

func TestContainerStatus_RestartedStateWithMissingData(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
	}{
		{
			name: "restarted state with empty container ID",
			u: &ContainerStatus{
				containerID: "",
				state:       RestartedState,
			},
		},
		{
			name: "restarted state with empty images",
			u: &ContainerStatus{
				containerID: "cont1",
				oldImage:    "",
				newImage:    "",
				state:       RestartedState,
			},
		},
		{
			name: "restarted state with empty name and image name",
			u: &ContainerStatus{
				containerID:   "cont1",
				containerName: "",
				imageName:     "",
				state:         RestartedState,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify state is Restarted
			if got := tt.u.State(); got != RestartedStateString {
				t.Errorf("ContainerStatus.State() = %v, want %v", got, RestartedStateString)
			}
			// Verify methods return empty values appropriately
			if tt.u.ID() != tt.u.containerID {
				t.Errorf("ID() mismatch")
			}

			if tt.u.Name() != tt.u.containerName {
				t.Errorf("Name() mismatch")
			}

			if tt.u.CurrentImageID() != tt.u.oldImage {
				t.Errorf("CurrentImageID() mismatch")
			}

			if tt.u.LatestImageID() != tt.u.newImage {
				t.Errorf("LatestImageID() mismatch")
			}

			if tt.u.ImageName() != tt.u.imageName {
				t.Errorf("ImageName() mismatch")
			}
		})
	}
}

func TestNewContainerStatus(t *testing.T) {
	got := NewContainerStatus("app", "org/app:latest")
	if got.Name() != "app" {
		t.Errorf("Name() = %q, want app", got.Name())
	}

	if got.ImageName() != "org/app:latest" {
		t.Errorf("ImageName() = %q, want org/app:latest", got.ImageName())
	}

	if got.State() != UpdatedStateString {
		t.Errorf("State() = %q, want %q", got.State(), UpdatedStateString)
	}
}

func TestContainerStatus_SetGitMetadata(t *testing.T) {
	u := &ContainerStatus{}
	u.SetGitMetadata(container.ReportMeta{
		GitRepo:         "https://github.com/org/app.git",
		GitRef:          "main",
		Changelog:       "https://github.com/org/app/releases/tag/v1.2.3",
		Source:          "https://github.com/org/app",
		ImageURL:        "https://example.com/image",
		Documentation:   "https://example.com/docs",
		CurrentVersion:  "1.2.2",
		LatestVersion:   "1.2.3",
		CurrentRevision: "olddeadbeef",
		LatestRevision:  "deadbeef",
	})

	if u.GitRepo() != "https://github.com/org/app.git" {
		t.Errorf("GitRepo() = %q", u.GitRepo())
	}

	if u.GitRef() != "main" {
		t.Errorf("GitRef() = %q", u.GitRef())
	}

	if u.Changelog() != "https://github.com/org/app/releases/tag/v1.2.3" {
		t.Errorf("Changelog() = %q", u.Changelog())
	}

	if u.Source() != "https://github.com/org/app" {
		t.Errorf("Source() = %q", u.Source())
	}

	if u.ImageURL() != "https://example.com/image" {
		t.Errorf("ImageURL() = %q", u.ImageURL())
	}

	if u.Documentation() != "https://example.com/docs" {
		t.Errorf("Documentation() = %q", u.Documentation())
	}

	if u.CurrentImageVersion() != "1.2.2" {
		t.Errorf("CurrentImageVersion() = %q", u.CurrentImageVersion())
	}

	if u.LatestImageVersion() != "1.2.3" {
		t.Errorf("LatestImageVersion() = %q", u.LatestImageVersion())
	}

	if u.CurrentImageRevision() != "olddeadbeef" {
		t.Errorf("CurrentImageRevision() = %q", u.CurrentImageRevision())
	}

	if u.LatestImageRevision() != "deadbeef" {
		t.Errorf("LatestImageRevision() = %q", u.LatestImageRevision())
	}
}

func TestContainerStatus_ImageVersionDefaultsToEmpty(t *testing.T) {
	u := &ContainerStatus{}

	if got := u.CurrentImageVersion(); got != "" {
		t.Errorf("CurrentImageVersion() = %q, want empty", got)
	}

	if got := u.LatestImageVersion(); got != "" {
		t.Errorf("LatestImageVersion() = %q, want empty", got)
	}

	if got := u.CurrentImageRevision(); got != "" {
		t.Errorf("CurrentImageRevision() = %q, want empty", got)
	}

	if got := u.LatestImageRevision(); got != "" {
		t.Errorf("LatestImageRevision() = %q, want empty", got)
	}
}

func TestContainerStatus_SetCooldownInfo(t *testing.T) {
	tests := []struct {
		name       string
		u          *ContainerStatus
		age        string
		delay      string
		remaining  string
		eligibleAt time.Time
	}{
		{
			name:       "cooldown passed with age",
			u:          &ContainerStatus{containerID: "cont1"},
			age:        "47 days, 11 hours",
			delay:      "24 hours",
			remaining:  "",
			eligibleAt: time.Time{},
		},
		{
			name:       "cooldown not passed with remaining",
			u:          &ContainerStatus{containerID: "cont2"},
			age:        "2 hours",
			delay:      "24 hours",
			remaining:  "22 hours",
			eligibleAt: time.Date(2026, 5, 26, 0, 45, 0, 0, time.UTC),
		},
		{
			name:       "empty values",
			u:          &ContainerStatus{containerID: "cont3"},
			age:        "",
			delay:      "",
			remaining:  "",
			eligibleAt: time.Time{},
		},
		{
			name:       "overwrites previous values",
			u:          &ContainerStatus{containerID: "cont4", cooldownAge: "old age", cooldownDelay: "old delay", cooldownRemaining: "old remaining"},
			age:        "new age",
			delay:      "new delay",
			remaining:  "new remaining",
			eligibleAt: time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.u.SetCooldownInfo(tt.age, tt.delay, tt.remaining, tt.eligibleAt)

			if got := tt.u.CooldownAge(); got != tt.age {
				t.Errorf("CooldownAge() = %v, want %v", got, tt.age)
			}

			if got := tt.u.CooldownDelay(); got != tt.delay {
				t.Errorf("CooldownDelay() = %v, want %v", got, tt.delay)
			}

			if got := tt.u.CooldownRemaining(); got != tt.remaining {
				t.Errorf("CooldownRemaining() = %v, want %v", got, tt.remaining)
			}

			if got := tt.u.CooldownEligibleAt(); got != tt.eligibleAt {
				t.Errorf("CooldownEligibleAt() = %v, want %v", got, tt.eligibleAt)
			}
		})
	}
}

func TestContainerStatus_CooldownDefaults(t *testing.T) {
	u := &ContainerStatus{containerID: "cont1"}

	if got := u.CooldownAge(); got != "" {
		t.Errorf("CooldownAge() default = %v, want empty", got)
	}

	if got := u.CooldownDelay(); got != "" {
		t.Errorf("CooldownDelay() default = %v, want empty", got)
	}

	if got := u.CooldownRemaining(); got != "" {
		t.Errorf("CooldownRemaining() default = %v, want empty", got)
	}

	if !u.CooldownEligibleAt().IsZero() {
		t.Errorf("CooldownEligibleAt() default = %v, want zero", u.CooldownEligibleAt())
	}
}

func TestContainerStatus_CooldownGettersReturnDirectValues(t *testing.T) {
	eligibleAt := time.Date(2026, 5, 26, 0, 45, 0, 0, time.UTC)
	u := &ContainerStatus{
		containerID:        "cont1",
		cooldownAge:        "3 days",
		cooldownDelay:      "48 hours",
		cooldownRemaining:  "12 hours",
		cooldownEligibleAt: eligibleAt,
	}

	if got := u.CooldownAge(); got != "3 days" {
		t.Errorf("CooldownAge() = %v, want '3 days'", got)
	}

	if got := u.CooldownDelay(); got != "48 hours" {
		t.Errorf("CooldownDelay() = %v, want '48 hours'", got)
	}

	if got := u.CooldownRemaining(); got != "12 hours" {
		t.Errorf("CooldownRemaining() = %v, want '12 hours'", got)
	}

	if got := u.CooldownEligibleAt(); got != eligibleAt {
		t.Errorf("CooldownEligibleAt() = %v, want %v", got, eligibleAt)
	}
}

func TestContainerStatus_RestartedStateIntegration(t *testing.T) {
	tests := []struct {
		name string
		u    *ContainerStatus
	}{
		{
			name: "restarted state with monitor only",
			u: &ContainerStatus{
				containerID:   "cont1",
				containerName: "my-container",
				state:         RestartedState,
				monitorOnly:   true,
			},
		},
		{
			name: "restarted state with new container ID",
			u: &ContainerStatus{
				containerID:    "cont1",
				newContainerID: "newcont1",
				state:          RestartedState,
			},
		},
		{
			name: "restarted state with all fields and error",
			u: &ContainerStatus{
				containerID:    "cont1",
				oldImage:       "img123",
				newImage:       "img456",
				containerName:  "my-container",
				imageName:      "nginx:latest",
				containerError: errors.New("some error"),
				state:          RestartedState,
				monitorOnly:    true,
				newContainerID: "newcont1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify state is Restarted
			if got := tt.u.State(); got != RestartedStateString {
				t.Errorf("ContainerStatus.State() = %v, want %v", got, RestartedStateString)
			}
			// Verify integration of fields
			if tt.u.ID() != tt.u.containerID {
				t.Errorf("ID integration failed")
			}

			if tt.u.IsMonitorOnly() != tt.u.monitorOnly {
				t.Errorf("IsMonitorOnly integration failed")
			}

			if tt.u.NewContainerID() != tt.u.newContainerID {
				t.Errorf("NewContainerID integration failed")
			}
			// Test SetNewContainerID
			tt.u.SetNewContainerID("updatedID")

			if tt.u.NewContainerID() != "updatedID" {
				t.Errorf("SetNewContainerID failed")
			}
		})
	}
}
