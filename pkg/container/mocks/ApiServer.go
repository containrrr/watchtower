package mocks

import (
	"encoding/json"
	"fmt"
	"github.com/onsi/ginkgo"
	"net/http"
	"net/url"
	"os"
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
	buf, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("mock JSON file %q not found: %e", absPath, err)
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
func GetContainerHandlers(containerRefs ...*ContainerRef) []http.HandlerFunc {
	handlers := make([]http.HandlerFunc, 0, len(containerRefs)*3)
	for _, containerRef := range containerRefs {
		handlers = append(handlers, getContainerFileHandler(containerRef))

		// Also append any containers that the container references, if any
		for _, ref := range containerRef.references {
			handlers = append(handlers, getContainerFileHandler(ref))
		}

		// Also append the image request since that will be called for every container
		handlers = append(handlers, getImageHandler(containerRef.image.id,
			RespondWithJSONFile(containerRef.image.getFileName(), http.StatusOK),
		))
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

var defaultImage = imageRef{
	// watchtower
	id:   t.ImageID("sha256:4dbc5f9c07028a985e14d1393e849ea07f68804c4293050d5a641b138db72daa"),
	file: "default",
}

var Watchtower = ContainerRef{
	name:  "watchtower",
	id:    "3d88e0e3543281c747d88b27e246578b65ae8964ba86c7cd7522cf84e0978134",
	image: &defaultImage,
}
var Stopped = ContainerRef{
	name:  "stopped",
	id:    "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b65",
	image: &defaultImage,
}
var Running = ContainerRef{
	name: "running",
	id:   "b978af0b858aa8855cce46b628817d4ed58e58f2c4f66c9b9c5449134ed4c008",
	image: &imageRef{
		// portainer
		id:   t.ImageID("sha256:19d07168491a3f9e2798a9bed96544e34d57ddc4757a4ac5bb199dea896c87fd"),
		file: "running",
	},
}
var Restarting = ContainerRef{
	name:  "restarting",
	id:    "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b67",
	image: &defaultImage,
}

var netSupplierOK = ContainerRef{
	id:   "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2",
	name: "net_supplier",
	image: &imageRef{
		// gluetun
		id:   t.ImageID("sha256:c22b543d33bfdcb9992cbef23961677133cdf09da71d782468ae2517138bad51"),
		file: "net_producer",
	},
}
var netSupplierNotFound = ContainerRef{
	id:        NetSupplierNotFoundID,
	name:      netSupplierOK.name,
	isMissing: true,
}

// NetConsumerOK is used for testing `container` networking mode
// returns a container that consumes an existing supplier container
var NetConsumerOK = ContainerRef{
	id:   "1f6b79d2aff23244382026c76f4995851322bed5f9c50631620162f6f9aafbd6",
	name: "net_consumer",
	image: &imageRef{
		id:   t.ImageID("sha256:904b8cb13b932e23230836850610fa45dce9eb0650d5618c2b1487c2a4f577b8"), // nginx
		file: "net_consumer",
	},
	references: []*ContainerRef{&netSupplierOK},
}

// NetConsumerInvalidSupplier is used for testing `container` networking mode
// returns a container that references a supplying container that does not exist
var NetConsumerInvalidSupplier = ContainerRef{
	id:         NetConsumerOK.id,
	name:       "net_consumer-missing_supplier",
	image:      NetConsumerOK.image,
	references: []*ContainerRef{&netSupplierNotFound},
}

const NetSupplierNotFoundID = "badc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc"
const NetSupplierContainerName = "/wt-contnet-producer-1"

func getContainerFileHandler(cr *ContainerRef) http.HandlerFunc {

	if cr.isMissing {
		return containerNotFoundResponse(string(cr.id))
	}

	containerFile, err := cr.getContainerFile()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Failed to get container mock file: %v", err))
	}

	return getContainerHandler(
		string(cr.id),
		RespondWithJSONFile(containerFile, http.StatusOK),
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
	return getImageHandler(t.ImageID(imageInfo.ID), ghttp.RespondWithJSONEncoded(http.StatusOK, imageInfo))
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

func getImageHandler(imageId t.ImageID, responseHandler http.HandlerFunc) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("/images/%s/json", imageId)),
		responseHandler,
	)
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
	return ghttp.RespondWithJSONEncoded(http.StatusNotFound, struct{ message string }{message: "No such container: " + string(containerID)})
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
