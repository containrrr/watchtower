package mocks

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	O "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
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
		handlers = append(handlers, getContainerHandler(file))

		// Also append the image request since that will be called for every container
		if file == "running" {
			// The "running" container is the only one using image02
			handlers = append(handlers, getImageHandler(1))
		} else {
			handlers = append(handlers, getImageHandler(0))
		}
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

var containerFileIds = map[string]string{
	"stopped":    "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b65",
	"watchtower": "3d88e0e3543281c747d88b27e246578b65ae8964ba86c7cd7522cf84e0978134",
	"running":    "b978af0b858aa8855cce46b628817d4ed58e58f2c4f66c9b9c5449134ed4c008",
	"restarting": "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b67",
}

var imageIds = []string{
	"sha256:4dbc5f9c07028a985e14d1393e849ea07f68804c4293050d5a641b138db72daa",
	"sha256:19d07168491a3f9e2798a9bed96544e34d57ddc4757a4ac5bb199dea896c87fd",
}

func getContainerHandler(file string) http.HandlerFunc {
	id, ok := containerFileIds[file]
	failTestUnless(ok)
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("/containers/%v/json", id)),
		RespondWithJSONFile(fmt.Sprintf("./mocks/data/container_%v.json", file), http.StatusOK),
	)
}

// ListContainersHandler mocks the GET containers/json endpoint, filtering the returned containers based on statuses
func ListContainersHandler(statuses ...string) http.HandlerFunc {
	filterArgs := createFilterArgs(statuses)
	bytes, err := filterArgs.MarshalJSON()
	O.ExpectWithOffset(1, err).ShouldNot(O.HaveOccurred())
	query := url.Values{
		"limit":   []string{"0"},
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

func getImageHandler(index int) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", O.HaveSuffix("/images/%v/json", imageIds[index])),
		RespondWithJSONFile(fmt.Sprintf("./mocks/data/image%02d.json", index+1), http.StatusOK),
	)
}

func failTestUnless(ok bool) {
	O.ExpectWithOffset(2, ok).To(O.BeTrue(), "test setup failed")
}
