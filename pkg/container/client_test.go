package container

import (
	"testing"

	"time"

	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/docker/docker/api/types"
	container2 "github.com/docker/docker/api/types/container"
	cli "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	docker *cli.Client
	client Client
)

func TestMain(m *testing.M) {
	server := mocks.NewMockAPIServer()
	docker, _ := cli.NewClientWithOpts(
		cli.WithHost(server.URL),
		cli.WithHTTPClient(server.Client()))

	client = dockerClient{
		api:        docker,
		pullImages: false,
	}
}

func TestSetMaxMemoryLimit_shouldHaveReduceMemory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestSetMaxMemoryLimit_positive")
	limit := int64(2147483648)
	c := CreateMockContainer(
		"test-container-01",
		"test-container-01",
		"fake-image:latest",
		time.Now().AddDate(0, 0, -1),
		int64(8589934592), // 8G
	)
	apply, err := client.SetMaxMemoryLimit(c, limit)

	Expect(err).NotTo(HaveOccurred())
	Expect(apply).To(BeTrue())
}

func TestSetMaxMemoryLimit_shouldNotApply(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestSetMaxMemoryLimit sould not apply limit")
	limit := int64(8589934592) // limit 8G
	c := CreateMockContainer(
		"test-container-01",
		"test-container-01",
		"fake-image:latest",
		time.Now().AddDate(0, 0, -1),
		int64(1073741824), // actual using 1G
	)
	apply, err := client.SetMaxMemoryLimit(c, limit)

	Expect(err).NotTo(HaveOccurred())
	Expect(apply).To(BeFalse())
}

func TestSetMaxMemoryLimit_shouldNotApply_on_same_value(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestSetMaxMemoryLimit sould not apply limit")
	limit := int64(8589934592) // limit 8G
	c := CreateMockContainer(
		"test-container-01",
		"test-container-01",
		"fake-image:latest",
		time.Now().AddDate(0, 0, -1),
		int64(8589934592), // actual using 8G
	)
	apply, err := client.SetMaxMemoryLimit(c, limit)

	Expect(err).NotTo(HaveOccurred())
	Expect(apply).To(BeFalse())
}

// CreateMockContainer creates a container substitute valid for testing
func CreateMockContainer(id string, name string, image string, created time.Time, actualMemory int64) Container {
	hostConfig := &container2.HostConfig{}
	hostConfig.Memory = actualMemory
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         id,
			Image:      image,
			Name:       name,
			Created:    created.String(),
			HostConfig: hostConfig,
		},
		Config: &container2.Config{
			Labels: make(map[string]string),
		},
	}
	return *NewContainer(
		&content,
		&types.ImageInspect{
			ID: image,
		},
	)
}
