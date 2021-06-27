package container

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/containrrr/watchtower/pkg/registry"
	"github.com/containrrr/watchtower/pkg/registry/digest"

	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	sdkClient "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const defaultStopSignal = "SIGTERM"

// A Client is the interface through which watchtower interacts with the
// Docker API.
type Client interface {
	ListContainers(t.Filter) ([]Container, error)
	GetContainer(containerID t.ContainerID) (Container, error)
	StopContainer(Container, time.Duration) error
	StartContainer(Container) (t.ContainerID, error)
	RenameContainer(Container, string) error
	IsContainerStale(Container) (stale bool, latestImage t.ImageID, err error)
	ExecuteCommand(containerID t.ContainerID, command string, timeout int) (SkipUpdate bool, err error)
	RemoveImageByID(t.ImageID) error
	WarnOnHeadPullFailed(container Container) bool
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
// The client reads its configuration from the following environment variables:
//  * DOCKER_HOST			the docker-engine host to send api requests to
//  * DOCKER_TLS_VERIFY		whether to verify tls certificates
//  * DOCKER_API_VERSION	the minimum docker api version to work with
func NewClient(pullImages, includeStopped, reviveStopped, removeVolumes, includeRestarting bool, warnOnHeadFailed string) Client {
	cli, err := sdkClient.NewClientWithOpts(sdkClient.FromEnv)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	return dockerClient{
		api:               cli,
		pullImages:        pullImages,
		removeVolumes:     removeVolumes,
		includeStopped:    includeStopped,
		reviveStopped:     reviveStopped,
		includeRestarting: includeRestarting,
		warnOnHeadFailed:  warnOnHeadFailed,
	}
}

type dockerClient struct {
	api               sdkClient.CommonAPIClient
	pullImages        bool
	removeVolumes     bool
	includeStopped    bool
	reviveStopped     bool
	includeRestarting bool
	warnOnHeadFailed  string
}

func (client dockerClient) WarnOnHeadPullFailed(container Container) bool {
	if client.warnOnHeadFailed == "always" {
		return true
	}
	if client.warnOnHeadFailed == "never" {
		return false
	}

	return registry.WarnOnAPIConsumption(container)
}

func (client dockerClient) ListContainers(fn t.Filter) ([]Container, error) {
	cs := []Container{}
	bg := context.Background()

	if client.includeStopped && client.includeRestarting {
		log.Debug("Retrieving running, stopped, restarting and exited containers")
	} else if client.includeStopped {
		log.Debug("Retrieving running, stopped and exited containers")
	} else if client.includeRestarting {
		log.Debug("Retrieving running and restarting containers")
	} else {
		log.Debug("Retrieving running containers")
	}

	filter := client.createListFilter()
	containers, err := client.api.ContainerList(
		bg,
		types.ContainerListOptions{
			Filters: filter,
		})

	if err != nil {
		return nil, err
	}

	for _, runningContainer := range containers {

		c, err := client.GetContainer(t.ContainerID(runningContainer.ID))
		if err != nil {
			return nil, err
		}

		if fn(c) {
			cs = append(cs, c)
		}
	}

	return cs, nil
}

func (client dockerClient) createListFilter() filters.Args {
	filterArgs := filters.NewArgs()
	filterArgs.Add("status", "running")

	if client.includeStopped {
		filterArgs.Add("status", "created")
		filterArgs.Add("status", "exited")
	}

	if client.includeRestarting {
		filterArgs.Add("status", "restarting")
	}

	return filterArgs
}

func (client dockerClient) GetContainer(containerID t.ContainerID) (Container, error) {
	bg := context.Background()

	containerInfo, err := client.api.ContainerInspect(bg, string(containerID))
	if err != nil {
		return Container{}, err
	}

	imageInfo, _, err := client.api.ImageInspectWithRaw(bg, containerInfo.Image)
	if err != nil {
		log.Warnf("Failed to retrieve container image info: %v", err)
		return Container{containerInfo: &containerInfo, imageInfo: nil}, nil
	}

	return Container{containerInfo: &containerInfo, imageInfo: &imageInfo}, nil
}

func (client dockerClient) StopContainer(c Container, timeout time.Duration) error {
	bg := context.Background()
	signal := c.StopSignal()
	if signal == "" {
		signal = defaultStopSignal
	}

	idStr := string(c.ID())
	shortID := c.ID().ShortID()

	if c.IsRunning() {
		log.Infof("Stopping %s (%s) with %s", c.Name(), shortID, signal)
		if err := client.api.ContainerKill(bg, idStr, signal); err != nil {
			return err
		}
	}

	// TODO: This should probably be checked.
	_ = client.waitForStopOrTimeout(c, timeout)

	if c.containerInfo.HostConfig.AutoRemove {
		log.Debugf("AutoRemove container %s, skipping ContainerRemove call.", shortID)
	} else {
		log.Debugf("Removing container %s", shortID)

		if err := client.api.ContainerRemove(bg, idStr, types.ContainerRemoveOptions{Force: true, RemoveVolumes: client.removeVolumes}); err != nil {
			return err
		}
	}

	// Wait for container to be removed. In this case an error is a good thing
	if err := client.waitForStopOrTimeout(c, timeout); err == nil {
		return fmt.Errorf("container %s (%s) could not be removed", c.Name(), shortID)
	}

	return nil
}

func (client dockerClient) StartContainer(c Container) (t.ContainerID, error) {
	bg := context.Background()
	config := c.runtimeConfig()
	hostConfig := c.hostConfig()
	networkConfig := &network.NetworkingConfig{EndpointsConfig: c.containerInfo.NetworkSettings.Networks}
	// simpleNetworkConfig is a networkConfig with only 1 network.
	// see: https://github.com/docker/docker/issues/29265
	simpleNetworkConfig := func() *network.NetworkingConfig {
		oneEndpoint := make(map[string]*network.EndpointSettings)
		for k, v := range networkConfig.EndpointsConfig {
			oneEndpoint[k] = v
			// we only need 1
			break
		}
		return &network.NetworkingConfig{EndpointsConfig: oneEndpoint}
	}()

	name := c.Name()

	log.Infof("Creating %s", name)
	createdContainer, err := client.api.ContainerCreate(bg, config, hostConfig, simpleNetworkConfig, name)
	if err != nil {
		return "", err
	}

	if !(hostConfig.NetworkMode.IsHost()) {

		for k := range simpleNetworkConfig.EndpointsConfig {
			err = client.api.NetworkDisconnect(bg, k, createdContainer.ID, true)
			if err != nil {
				return "", err
			}
		}

		for k, v := range networkConfig.EndpointsConfig {
			err = client.api.NetworkConnect(bg, k, createdContainer.ID, v)
			if err != nil {
				return "", err
			}
		}

	}

	createdContainerID := t.ContainerID(createdContainer.ID)
	if !c.IsRunning() && !client.reviveStopped {
		return createdContainerID, nil
	}

	return createdContainerID, client.doStartContainer(bg, c, createdContainer)

}

func (client dockerClient) doStartContainer(bg context.Context, c Container, creation container.ContainerCreateCreatedBody) error {
	name := c.Name()

	log.Debugf("Starting container %s (%s)", name, t.ContainerID(creation.ID).ShortID())
	err := client.api.ContainerStart(bg, creation.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (client dockerClient) RenameContainer(c Container, newName string) error {
	bg := context.Background()
	log.Debugf("Renaming container %s (%s) to %s", c.Name(), c.ID().ShortID(), newName)
	return client.api.ContainerRename(bg, string(c.ID()), newName)
}

func (client dockerClient) IsContainerStale(container Container) (stale bool, latestImage t.ImageID, err error) {
	ctx := context.Background()

	if !client.pullImages {
		log.Debugf("Skipping image pull.")
	} else if err := client.PullImage(ctx, container); err != nil {
		return false, container.SafeImageID(), err
	}

	return client.HasNewImage(ctx, container)
}

func (client dockerClient) HasNewImage(ctx context.Context, container Container) (hasNew bool, latestImage t.ImageID, err error) {
	currentImageID := t.ImageID(container.containerInfo.ContainerJSONBase.Image)
	imageName := container.ImageName()

	newImageInfo, _, err := client.api.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return false, currentImageID, err
	}

	newImageID := t.ImageID(newImageInfo.ID)
	if newImageID == currentImageID {
		log.Debugf("No new images found for %s", container.Name())
		return false, currentImageID, nil
	}

	log.Infof("Found new %s image (%s)", imageName, newImageID.ShortID())
	return true, newImageID, nil
}

// PullImage pulls the latest image for the supplied container, optionally skipping if it's digest can be confirmed
// to match the one that the registry reports via a HEAD request
func (client dockerClient) PullImage(ctx context.Context, container Container) error {
	containerName := container.Name()
	imageName := container.ImageName()

	fields := log.Fields{
		"image":     imageName,
		"container": containerName,
	}

	log.WithFields(fields).Debugf("Trying to load authentication credentials.")
	opts, err := registry.GetPullOptions(imageName)
	if err != nil {
		log.Debugf("Error loading authentication credentials %s", err)
		return err
	}
	if opts.RegistryAuth != "" {
		log.Debug("Credentials loaded")
	}

	log.WithFields(fields).Debugf("Checking if pull is needed")

	if match, err := digest.CompareDigest(container, opts.RegistryAuth); err != nil {
		headLevel := log.DebugLevel
		if client.WarnOnHeadPullFailed(container) {
			headLevel = log.WarnLevel
		}
		log.WithFields(fields).Logf(headLevel, "Could not do a head request for %q, falling back to regular pull.", imageName)
		log.WithFields(fields).Log(headLevel, "Reason: ", err)
	} else if match {
		log.Debug("No pull needed. Skipping image.")
		return nil
	} else {
		log.Debug("Digests did not match, doing a pull.")
	}

	log.WithFields(fields).Debugf("Pulling image")

	response, err := client.api.ImagePull(ctx, imageName, opts)
	if err != nil {
		log.Debugf("Error pulling image %s, %s", imageName, err)
		return err
	}

	defer response.Close()
	// the pull request will be aborted prematurely unless the response is read
	if _, err = ioutil.ReadAll(response); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (client dockerClient) RemoveImageByID(id t.ImageID) error {
	log.Infof("Removing image %s", id.ShortID())

	_, err := client.api.ImageRemove(
		context.Background(),
		string(id),
		types.ImageRemoveOptions{
			Force: true,
		})

	return err
}

func (client dockerClient) ExecuteCommand(containerID t.ContainerID, command string, timeout int) (SkipUpdate bool, err error) {
	bg := context.Background()

	// Create the exec
	execConfig := types.ExecConfig{
		Tty:    true,
		Detach: false,
		Cmd:    []string{"sh", "-c", command},
	}

	exec, err := client.api.ContainerExecCreate(bg, string(containerID), execConfig)
	if err != nil {
		return false, err
	}

	response, attachErr := client.api.ContainerExecAttach(bg, exec.ID, types.ExecStartCheck{
		Tty:    true,
		Detach: false,
	})
	if attachErr != nil {
		log.Errorf("Failed to extract command exec logs: %v", attachErr)
	}

	// Run the exec
	execStartCheck := types.ExecStartCheck{Detach: false, Tty: true}
	err = client.api.ContainerExecStart(bg, exec.ID, execStartCheck)
	if err != nil {
		return false, err
	}

	var output string
	if attachErr == nil {
		defer response.Close()
		var writer bytes.Buffer
		written, err := writer.ReadFrom(response.Reader)
		if err != nil {
			log.Error(err)
		} else if written > 0 {
			output = strings.TrimSpace(writer.String())
		}
	}

	// Inspect the exec to get the exit code and print a message if the
	// exit code is not success.
	skipUpdate, err := client.waitForExecOrTimeout(bg, exec.ID, output, timeout)
	if err != nil {
		return true, err
	}

	return skipUpdate, nil
}

func (client dockerClient) waitForExecOrTimeout(bg context.Context, ID string, execOutput string, timeout int) (SkipUpdate bool, err error) {
	const ExTempFail = 75
	var ctx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(bg, time.Duration(timeout)*time.Minute)
		defer cancel()
	} else {
		ctx = bg
	}

	for {
		execInspect, err := client.api.ContainerExecInspect(ctx, ID)

		//goland:noinspection GoNilness
		log.WithFields(log.Fields{
			"exit-code": execInspect.ExitCode,
			"exec-id":   execInspect.ExecID,
			"running":   execInspect.Running,
		}).Debug("Awaiting timeout or completion")

		if err != nil {
			return false, err
		}
		if execInspect.Running == true {
			time.Sleep(1 * time.Second)
			continue
		}
		if len(execOutput) > 0 {
			log.Infof("Command output:\n%v", execOutput)
		}

		if execInspect.ExitCode == ExTempFail {
			return true, nil
		}

		if execInspect.ExitCode > 0 {
			return false, fmt.Errorf("Command exited with code %v  %s", execInspect.ExitCode, execOutput)
		}
		break
	}
	return false, nil
}

func (client dockerClient) waitForStopOrTimeout(c Container, waitTime time.Duration) error {
	bg := context.Background()
	timeout := time.After(waitTime)

	for {
		select {
		case <-timeout:
			return nil
		default:
			if ci, err := client.api.ContainerInspect(bg, string(c.ID())); err != nil {
				return err
			} else if !ci.State.Running {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}
