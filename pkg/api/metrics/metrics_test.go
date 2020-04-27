package metrics_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/containrrr/watchtower/pkg/api"
	"github.com/containrrr/watchtower/pkg/api/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const Token = "123123123"

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

func runTestServer(m *metrics.MetricsHandle) {
	http.Handle(m.Path, m.Handle)
	go func() {
		http.ListenAndServe(":8080", nil)
	}()
}

func getWithToken(c http.Client, url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Token", Token)
	return c.Do(req)
}

var _ = Describe("the metrics", func() {
	httpAPI := api.New(Token)
	m := metrics.New()
	httpAPI.RegisterHandler(m.Path, m.Handle)
	httpAPI.Start(false)

	// We should likely split this into multiple tests, but as prometheus requires a restart of the binary
	// to reset the metrics and gauges, we'll just do it all at once.

	It("should serve metrics", func() {
		m.Metrics.RegisterScan(4, 3, 1)
		c := http.Client{}
		res, err := getWithToken(c, "http://localhost:8080/v1/metrics")

		Expect(err).NotTo(HaveOccurred())
		contents, err := ioutil.ReadAll(res.Body)
		fmt.Printf("%s\n", string(contents))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_updated 3"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_failed 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_scanned 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 0"))

		for i := 0; i < 3; i++ {
			m.Metrics.RegisterSkipped()
		}

		res, err = getWithToken(c, "http://localhost:8080/v1/metrics")
		Expect(err).NotTo(HaveOccurred())
		contents, err = ioutil.ReadAll(res.Body)
		fmt.Printf("%s\n", string(contents))

		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 3"))
	})
})
