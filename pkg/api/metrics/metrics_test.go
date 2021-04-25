package metrics_test

import (
	"fmt"
	"github.com/containrrr/watchtower/pkg/metrics"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/containrrr/watchtower/pkg/api"
	metricsAPI "github.com/containrrr/watchtower/pkg/api/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const Token = "123123123"

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

func runTestServer(m *metricsAPI.Handler) {
	http.Handle(m.Path, m.Handle)
	go func() {
		http.ListenAndServe(":8080", nil)
	}()
}

func getWithToken(c http.Client, url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", Token))
	return c.Do(req)
}

var _ = Describe("the metrics", func() {
	httpAPI := api.New(Token)
	m := metricsAPI.New()

	httpAPI.RegisterHandler(m.Path, m.Handle)
	httpAPI.Start(false)

	It("should serve metrics", func() {
		metric := &metrics.Metric{
			Scanned: 4,
			Updated: 3,
			Failed:  1,
		}
		metrics.RegisterScan(metric)
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		c := http.Client{}

		res, err := getWithToken(c, "http://localhost:8080/v1/metrics")
		Expect(err).ToNot(HaveOccurred())

		contents, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_updated 3"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_failed 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_scanned 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 0"))

		for i := 0; i < 3; i++ {
			metrics.RegisterScan(nil)
		}
		Eventually(metrics.Default().QueueIsEmpty).Should(BeTrue())

		res, err = getWithToken(c, "http://localhost:8080/v1/metrics")
		Expect(err).ToNot(HaveOccurred())

		contents, err = ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 3"))
	})
})
