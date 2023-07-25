package mocks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	t "github.com/containrrr/watchtower/pkg/types"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	O "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

func getMockJSONFile(relPath string) ([]byte, error) {
	absPath, _ := filepath.Abs(relPath)
	buf, err := ioutil.ReadFile(absPath)
	if err != nil {
		// logrus.WithError(err).WithField("file", absPath).Error(err)
		return nil, err
	}
	return buf, nil
}

// RespondWithJSONFile handles a request by returning the contents of the supplied file
func RespondWithJSONFile(relPath string, statusCode int, optionalHeader ...http.Header) http.HandlerFunc {
	handler, err := respondWithJSONFile(relPath, statusCode, optionalHeader...)
	O.ExpectWithOffset(1, err).ShouldNot(O.HaveOccurred())
	return handler
}

func respondWithJSONFile(relPath string, statusCode int, optionalHeader ...http.Header) (http.HandlerFunc, error) {
	buf, err := getMockJSONFile(relPath)
	if err != nil {
		return nil, err
	}
	return ghttp.RespondWith(statusCode, buf, optionalHeader...), nil
}

// GetContainerHandlers returns the handlers serving lookups for the supplied container mock files
func GetContainerHandlers(containerFiles ...string) []http.HandlerFunc {
	handlers := make([]http.HandlerFunc, 0, len(containerFiles)*2)
	for _, file := range containerFiles {
		handlers = append(handlers, getContainerFileHandler(file))

		if file == "net_consumer" {
			// Also append the net_producer container, since it's used to reconfigure networking
			handlers = append(handlers, getContainerHandler("net_producer"))
		}

		// Also append the image request since that will be called for every container
		handlers = append(handlers, getImageFileHandler(file))
	}
	return handlers
}

func createFilterArgs(statuses []string) filters.Args {
	args := filters.NewArgs()
	for _, status := range statuses {
		args.Add("status", status)
	}
	return args
}

const NetConsumerID = t.ContainerID("1f6b79d2aff23244382026c76f4995851322bed5f9c50631620162f6f9aafbd6")
const NetProducerID = t.ContainerID("25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2")
const NetProducerContainerName = "/wt-contnet-producer-1"

var containerFileIds = map[string]t.ContainerID{
	"stopped":      t.ContainerID("ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b65"),
	"watchtower":   t.ContainerID("3d88e0e3543281c747d88b27e246578b65ae8964ba86c7cd7522cf84e0978134"),
	"running":      t.ContainerID("b978af0b858aa8855cce46b628817d4ed58e58f2c4f66c9b9c5449134ed4c008"),
	"restarting":   t.ContainerID("ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b67"),
	"net_consumer": NetConsumerID,
	"net_producer": NetProducerID,
}

var imageIds = map[string]t.ImageID{
	"default":      t.ImageID("sha256:4dbc5f9c07028a985e14d1393e849ea07f68804c4293050d5a641b138db72daa"), // watchtower
	"running":      t.ImageID("sha256:19d07168491a3f9e2798a9bed96544e34d57ddc4757a4ac5bb199dea896c87fd"), // portainer
	"net_consumer": t.ImageID("sha256:904b8cb13b932e23230836850610fa45dce9eb0650d5618c2b1487c2a4f577b8"), // nginx
	"net_producer": t.ImageID("sha256:c22b543d33bfdcb9992cbef23961677133cdf09da71d782468ae2517138bad51"), // gluetun
}

func getContainerFileHandler(file string) http.HandlerFunc {
	id, ok := containerFileIds[file]
	failTestUnless(ok)
	return getContainerHandler(
		id,
		RespondWithJSONFile(fmt.Sprintf("./mocks/data/container_%v.json", file), http.StatusOK),
	)
}

func getContainerHandler(containerId string, responseHandler http.HandlerFunc) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("/containers/%v/json", containerId)),
		responseHandler,
	)
}

// GetContainerHandler mocks the GET containers/{id}/json endpoint
func GetContainerHandler(containerID string, containerInfo *types.ContainerJSON) http.HandlerFunc {
	responseHandler := containerNotFoundResponse(containerID)
	if containerInfo != nil {
		responseHandler = ghttp.RespondWithJSONEncoded(http.StatusOK, containerInfo)
	}
	return getContainerHandler(containerID, responseHandler)
}

// GetImageHandler mocks the GET images/{id}/json endpoint
func GetImageHandler(imageInfo *types.ImageInspect) http.HandlerFunc {
	return getImageHandler(imageInfo.ID, ghttp.RespondWithJSONEncoded(http.StatusOK, imageInfo))
}

// ListContainersHandler mocks the GET containers/json endpoint, filtering the returned containers based on statuses
func ListContainersHandler(statuses ...string) http.HandlerFunc {
	filterArgs := createFilterArgs(statuses)
	bytes, err := filterArgs.MarshalJSON()
	O.ExpectWithOffset(1, err).ShouldNot(O.HaveOccurred())
	query := url.Values{
		"filters": []string{string(bytes)},
	}
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("containers/json"), query.Encode()),
		respondWithFilteredContainers(filterArgs),
	)
}

func respondWithFilteredContainers(filters filters.Args) http.HandlerFunc {
	containersJSON, err := getMockJSONFile("./mocks/data/containers.json")
	O.ExpectWithOffset(2, err).ShouldNot(O.HaveOccurred())
	var filteredContainers []types.Container
	var containers []types.Container
	O.ExpectWithOffset(2, json.Unmarshal(containersJSON, &containers)).To(O.Succeed())
	for _, v := range containers {
		for _, key := range filters.Get("status") {
			if v.State == key {
				filteredContainers = append(filteredContainers, v)
			}
		}
	}

	return ghttp.RespondWithJSONEncoded(http.StatusOK, filteredContainers)
}

func getImageHandler(imageId string, responseHandler http.HandlerFunc) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("/images/%s/json", imageId)),
		responseHandler,
	)
}

func getImageFileHandler(key string) http.HandlerFunc {
	if _, found := imageIds[key]; !found {
		// The default image (watchtower) is used for most of the containers
		key = `default`
	}
	return getImageHandler(imageIds[key],
		RespondWithJSONFile(fmt.Sprintf("./mocks/data/image_%v.json", key), http.StatusOK),
	)
}

func failTestUnless(ok bool) {
	O.ExpectWithOffset(2, ok).To(O.BeTrue(), "test setup failed")
}

// KillContainerHandler mocks the POST containers/{id}/kill endpoint
func KillContainerHandler(containerID string, found FoundStatus) http.HandlerFunc {
	responseHandler := noContentStatusResponse
	if !found {
		responseHandler = containerNotFoundResponse(containerID)
	}
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("POST", O.HaveSuffix("containers/%s/kill", containerID)),
		responseHandler,
	)
}

// RemoveContainerHandler mocks the DELETE containers/{id} endpoint
func RemoveContainerHandler(containerID string, found FoundStatus) http.HandlerFunc {
	responseHandler := noContentStatusResponse
	if !found {
		responseHandler = containerNotFoundResponse(containerID)
	}
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("DELETE", O.HaveSuffix("containers/%s", containerID)),
		responseHandler,
	)
}

func containerNotFoundResponse(containerID string) http.HandlerFunc {
	return ghttp.RespondWithJSONEncoded(http.StatusNotFound, struct{ message string }{message: "No such container: " + containerID})
}

var noContentStatusResponse = ghttp.RespondWith(http.StatusNoContent, nil)

type FoundStatus bool

const (
	Found   FoundStatus = true
	Missing FoundStatus = false
)

// RemoveImageHandler mocks the DELETE images/ID endpoint, simulating removal of the given imagesWithParents
func RemoveImageHandler(imagesWithParents map[string][]string) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("DELETE", O.MatchRegexp("/images/.*")),
		func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, `/`)
			image := parts[len(parts)-1]

			if parents, found := imagesWithParents[image]; found {
				items := []types.ImageDeleteResponseItem{
					{Untagged: image},
					{Deleted: image},
				}
				for _, parent := range parents {
					items = append(items, types.ImageDeleteResponseItem{Deleted: parent})
				}
				ghttp.RespondWithJSONEncoded(http.StatusOK, items)(w, r)
			} else {
				ghttp.RespondWithJSONEncoded(http.StatusNotFound, struct{ message string }{
					message: "Something went wrong.",
				})(w, r)
			}
		},
	)
}
