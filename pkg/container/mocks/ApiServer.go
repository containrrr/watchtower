package mocks

import (
	"encoding/json"
	"fmt"
	"github.com/onsi/ginkgo"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
)

// NewMockAPIServer returns a mocked docker api server that responds to some fixed requests
// used in the test suite.
func NewMockAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logrus.Debug("Mock server has received a HTTP call on ", r.URL)
			var response = ""

			if isRequestFor("filters=", r) {

				Filters := r.URL.Query().Get("filters")
				var result map[string]interface{}
				_ = json.Unmarshal([]byte(Filters), &result)
				status := result["status"].(map[string]interface{})

				response = getMockJSONFromDisk("./mocks/data/containers.json")
				var x2 []types.Container
				var containers []types.Container
				_ = json.Unmarshal([]byte(response), &containers)
				for _, v := range containers {
					for key := range status {
						if v.State == key {
							x2 = append(x2, v)
						}
					}
				}

				b, _ := json.Marshal(x2)
				response = string(b)

			} else if isRequestFor("containers/json?limit=0", r) {
				response = getMockJSONFromDisk("./mocks/data/containers.json")
			} else if isRequestFor("ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b65", r) {
				response = getMockJSONFromDisk("./mocks/data/container_stopped.json")
			} else if isRequestFor("b978af0b858aa8855cce46b628817d4ed58e58f2c4f66c9b9c5449134ed4c008", r) {
				response = getMockJSONFromDisk("./mocks/data/container_running.json")
			} else if isRequestFor("ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b67", r) {
				response = getMockJSONFromDisk("./mocks/data/container_restarting.json")
			} else if isRequestFor("sha256:19d07168491a3f9e2798a9bed96544e34d57ddc4757a4ac5bb199dea896c87fd", r) {
				response = getMockJSONFromDisk("./mocks/data/image01.json")
			} else if isRequestFor("sha256:4dbc5f9c07028a985e14d1393e849ea07f68804c4293050d5a641b138db72daa", r) {
				response = getMockJSONFromDisk("./mocks/data/image02.json")
			} else if isRequestFor("containers/ex-cont-id/exec", r) {
				response = `{"Id": "ex-exec-id"}`
			} else if isRequestFor("exec/ex-exec-id/start", r) {
				response = `{"Id": "ex-exec-id"}`
			} else if isRequestFor("exec/ex-exec-id/json", r) {
				response = `{
    				"ExecID": "ex-exec-id",
					"ContainerID": "ex-cont-id",
					"Running": false,
					"ExitCode": 0,
					"Pid": 0
				}`
			} else {
				// Allow ginkgo to correctly capture the failed assertion, even though this is called from a go func
				defer ginkgo.GinkgoRecover()
				ginkgo.Fail(fmt.Sprintf("mock API server endpoint not supported: %q", r.URL.String()))
			}
			_, _ = fmt.Fprintln(w, response)
		},
	))
}

func isRequestFor(urlPart string, r *http.Request) bool {
	return strings.Contains(r.URL.String(), urlPart)
}

func getMockJSONFromDisk(relPath string) string {
	absPath, _ := filepath.Abs(relPath)
	buf, err := ioutil.ReadFile(absPath)
	if err != nil {
		logrus.WithError(err).WithField("file", absPath).Error(err)
		return ""
	}
	return string(buf)
}
