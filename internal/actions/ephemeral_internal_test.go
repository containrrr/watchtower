package actions

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// firstOperationIndex returns the index of the first occurrence of name in ops,
// or -1 when name is absent.
//
// Parameters:
//   - ops: Ordered list of operation names.
//   - name: Operation name to locate.
//
// Returns:
//   - int: Zero-based index, or -1 if not found.
func firstOperationIndex(ops []string, name string) int {
	for i, op := range ops {
		if op == name {
			return i
		}
	}

	return -1
}

// createEphemeralHandoffContainer builds a running Watchtower mock used by
// orchestrateSelfUpdate handoff tests.
//
// Parameters:
//   - id: Container ID.
//   - portBindings: Optional host port bindings. Nil yields empty bindings.
//
// Returns:
//   - types.Container: Configured mock container.
func createEphemeralHandoffContainer(
	id string,
	portBindings dockerNetwork.PortMap,
) types.Container {
	if portBindings == nil {
		portBindings = dockerNetwork.PortMap{}
	}

	const (
		name  = "/watchtower"
		image = "watchtower:v1"
	)

	ctr := mockActions.CreateMockContainerWithConfig(
		id,
		name,
		image,
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: image,
			Labels: map[string]string{
				"com.centurylinklabs.watchtower": "true",
			},
		},
	)

	info := ctr.ContainerInfo()
	if info != nil && info.HostConfig != nil {
		info.HostConfig.PortBindings = portBindings
	}

	return ctr
}

var _ = ginkgo.Describe("orchestrateSelfUpdate handoff", func() {
	// Stop frees host ports. Rename keeps the stopped predecessor for recovery.
	// Remove runs only after the replacement is verified running.
	ginkgo.It("stops, renames, creates, then removes the old container on success", func() {
		old := createEphemeralHandoffContainer("wt-old-id-ok", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-id-ok",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.BeNumerically(">=", int32(1)))
		gomega.Expect(client.TestData.StopContainerCount.Load()).To(gomega.BeNumerically(">=", int32(1)))
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(1)))
		gomega.Expect(client.TestData.RemoveContainerCount.Load()).To(gomega.BeNumerically(">=", int32(1)))
		gomega.Expect(client.TestData.RenameTargets).To(gomega.HaveLen(1))
		gomega.Expect(client.TestData.RenameTargets[0]).To(gomega.HavePrefix(types.WatchtowerOldPrefix))

		stopIdx := firstOperationIndex(client.TestData.OperationOrder, "StopContainer")
		renameIdx := firstOperationIndex(client.TestData.OperationOrder, "RenameContainer")
		startIdx := firstOperationIndex(client.TestData.OperationOrder, "StartContainer")
		removeIdx := firstOperationIndex(client.TestData.OperationOrder, "RemoveContainer")

		gomega.Expect(stopIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(renameIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(startIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(removeIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(stopIdx).To(gomega.BeNumerically("<", renameIdx))
		gomega.Expect(renameIdx).To(gomega.BeNumerically("<", startIdx))
		gomega.Expect(startIdx).To(gomega.BeNumerically("<", removeIdx))
	})

	ginkgo.It("stops the old container before create when host ports are published", func() {
		old := createEphemeralHandoffContainer(
			"wt-old-ports",
			dockerNetwork.PortMap{
				dockerNetwork.MustParsePort("8080/tcp"): {
					{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "8080"},
				},
			},
		)

		gomega.Expect(old.HasExposedPorts()).To(gomega.BeTrue())

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-ports",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		stopIdx := firstOperationIndex(client.TestData.OperationOrder, "StopContainer")
		startIdx := firstOperationIndex(client.TestData.OperationOrder, "StartContainer")

		gomega.Expect(stopIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(startIdx).To(gomega.BeNumerically(">=", 0))
		gomega.Expect(stopIdx).To(gomega.BeNumerically("<", startIdx))
	})

	ginkgo.It("does not create a new container when stopping the old container fails", func() {
		old := createEphemeralHandoffContainer("wt-old-stop-fail", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers:             []types.Container{old},
				StopContainerError:     errors.New("simulated stop failure"),
				StopContainerFailCount: 1,
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-stop-fail",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to stop old container"))
		gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
	})

	ginkgo.It("restores and restarts the old container when create fails after rename", func() {
		old := createEphemeralHandoffContainer("wt-old-create-fail", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
				// Fail only the replacement create/start path. Leave recovery
				// StartContainerByID successful so restart can be asserted.
				StartContainerError: errors.New("simulated create failure"),
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-create-fail",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to create new container"))

		gomega.Expect(client.TestData.StopContainerCount.Load()).To(gomega.BeNumerically(">=", int32(1)))
		// Rename off original name, then rename back on failure.
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(2)))
		gomega.Expect(client.TestData.RenameTargets).To(gomega.HaveLen(2))
		gomega.Expect(client.TestData.RenameTargets[0]).To(gomega.HavePrefix(types.WatchtowerOldPrefix))
		gomega.Expect(client.TestData.RenameTargets[1]).To(gomega.Equal("watchtower"))
		// Predecessor must not be deleted when handoff fails.
		gomega.Expect(client.TestData.RemoveContainerCount.Load()).To(gomega.Equal(int32(0)))
		// Recovery restart succeeded (StartContainerByIDError is unset).
		gomega.Expect(client.Stopped[string(old.ID())]).To(gomega.BeFalse())

		stopIdx := firstOperationIndex(client.TestData.OperationOrder, "StopContainer")
		renameIdx := firstOperationIndex(client.TestData.OperationOrder, "RenameContainer")
		startIdx := firstOperationIndex(client.TestData.OperationOrder, "StartContainer")
		restartIdx := firstOperationIndex(client.TestData.OperationOrder, "StartContainerByID")

		gomega.Expect(stopIdx).To(gomega.BeNumerically("<", renameIdx))
		gomega.Expect(renameIdx).To(gomega.BeNumerically("<", startIdx))
		gomega.Expect(restartIdx).To(gomega.BeNumerically(">", startIdx))
	})

	ginkgo.It("pins Config.Image to the new image reference before create", func() {
		old := createEphemeralHandoffContainer("wt-old-pin", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-pin",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.LastStartedContainer).NotTo(gomega.BeNil())
		gomega.Expect(client.TestData.LastStartedContainer.ImageName()).To(gomega.Equal("watchtower:v2"))
	})

	ginkgo.It("propagates the container chain label onto the source config used for create", func() {
		old := createEphemeralHandoffContainer("wt-old-chain", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		chain := "prev-id,wt-old-chain"
		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-chain",
			"watchtower:v2",
			"watchtower",
			chain,
			false,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.LastStartedContainer).NotTo(gomega.BeNil())

		gotChain, present := client.TestData.LastStartedContainer.GetContainerChain()
		gomega.Expect(present).To(gomega.BeTrue())
		gomega.Expect(gotChain).To(gomega.Equal(chain))
	})

	ginkgo.It("removes the old image after a successful handoff when cleanup is enabled", func() {
		old := createEphemeralHandoffContainer("wt-old-cleanup", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-cleanup",
			"watchtower:v2",
			"watchtower",
			"",
			true,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).To(gomega.Equal(int32(1)))
		gomega.Expect(client.TestData.RemoveContainerCount.Load()).To(gomega.BeNumerically(">=", int32(1)))
	})

	ginkgo.It("does not remove the old image when cleanup is disabled", func() {
		old := createEphemeralHandoffContainer("wt-old-no-cleanup", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{old},
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-no-cleanup",
			"watchtower:v2",
			"watchtower",
			"",
			false,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).To(gomega.Equal(int32(0)))
	})

	ginkgo.It("treats an in-use old image as a non-fatal skip", func() {
		old := createEphemeralHandoffContainer("wt-old-in-use", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers:       []types.Container{old},
				FailedImageIDs:   []types.ImageID{old.ImageID()},
				RemoveImageError: container.ErrImageInUse,
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-in-use",
			"watchtower:v2",
			"watchtower",
			"",
			true,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).To(gomega.Equal(int32(1)))
	})

	ginkgo.It("treats a missing old image as a non-fatal skip", func() {
		old := createEphemeralHandoffContainer("wt-old-missing-image", nil)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers:       []types.Container{old},
				FailedImageIDs:   []types.ImageID{old.ImageID()},
				RemoveImageError: cerrdefs.ErrNotFound,
			},
			false,
			false,
		)

		err := orchestrateSelfUpdate(newNopLogger(),
			context.Background(),
			client,
			"wt-old-missing-image",
			"watchtower:v2",
			"watchtower",
			"",
			true,
		)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).To(gomega.Equal(int32(1)))
	})
})
