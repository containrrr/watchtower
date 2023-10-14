package metrics_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/containrrr/watchtower/pkg/api"
	metricsAPI "github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/metrics"
)

const (
	token  = "123123123"
	getURL = "http://localhost:8080/v1/metrics"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

func getWithToken(handler http.Handler) map[string]string {
	metricMap := map[string]string{}
	respWriter := httptest.NewRecorder()

	req := httptest.NewRequest("GET", getURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	handler.ServeHTTP(respWriter, req)

	res := respWriter.Result()
	body, _ := io.ReadAll(res.Body)

	for _, line := range strings.Split(string(body), "\n") {
		if len(line) < 1 || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, " ")
		metricMap[parts[0]] = parts[1]
	}

	return metricMap
}

var _ = Describe("the metrics API", func() {
	httpAPI := api.New(token)
	m := metricsAPI.New()

	handleReq := httpAPI.RequireToken(m.Handle)
	tryGetMetrics := func() map[string]string { return getWithToken(handleReq) }

	It("should serve metrics", func() {

		Expect(tryGetMetrics()).To(HaveKeyWithValue("watchtower_containers_updated", "0"))

		metric := &metrics.Metric{
			Scanned: 4,
			Updated: 3,
			Failed:  1,
		}

		metrics.RegisterScan(metric)
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		Eventually(tryGetMetrics).Should(SatisfyAll(
			HaveKeyWithValue("watchtower_containers_updated", "3"),
			HaveKeyWithValue("watchtower_containers_failed", "1"),
			HaveKeyWithValue("watchtower_containers_scanned", "4"),
			HaveKeyWithValue("watchtower_scans_total", "1"),
			HaveKeyWithValue("watchtower_scans_skipped", "0"),
		))

		for i := 0; i < 3; i++ {
			metrics.RegisterScan(nil)
		}
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		Eventually(tryGetMetrics).Should(SatisfyAll(
			HaveKeyWithValue("watchtower_scans_total", "4"),
			HaveKeyWithValue("watchtower_scans_skipped", "3"),
		))
	})
})
