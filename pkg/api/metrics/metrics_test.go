package metrics_test

import (
	"fmt"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containrrr/watchtower/pkg/api"
	metricsAPI "github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	token  = "123123123"
	getUrl = "http://localhost:8080/v1/metrics"
)

var log = stdlog.New(GinkgoWriter, "", 0)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

func getWithToken(handler http.Handler) string {
	respWriter := httptest.NewRecorder()

	req := httptest.NewRequest("GET", getUrl, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	handler.ServeHTTP(respWriter, req)

	res := respWriter.Result()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("reading body failed: ", err)
		return ""
	}
	return string(body)
}

var _ = Describe("the metrics API", func() {
	httpAPI := api.New(token)
	m := metricsAPI.New()

	handleReq := httpAPI.RequireToken(m.Handle)

	It("should serve metrics", func() {
		metric := &metrics.Metric{
			Scanned: 4,
			Updated: 3,
			Failed:  1,
		}
		metrics.RegisterScan(metric)
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_containers_updated 3"))
		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_containers_failed 1"))
		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_containers_scanned 4"))
		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_scans_total 1"))
		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_scans_skipped 0"))

		for i := 0; i < 3; i++ {
			metrics.RegisterScan(nil)
		}
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_scans_total 4"))
		Eventually(getWithToken(handleReq)).Should(ContainSubstring("watchtower_scans_skipped 3"))
	})
})
