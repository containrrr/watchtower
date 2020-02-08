package metrics_test

import (
	"fmt"
	"github.com/containrrr/watchtower/pkg/metrics"
	"io/ioutil"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = Describe("the metrics", func() {
	m := metrics.NewMetrics("9091")

	// We should likely split this into multiple tests, but as prometheus requires a restart of the binary
	// to reset the metrics and gauges, we'll just do it all at once.

	It("should serve metrics", func() {
		m.RegisterScan(4, 3, 1)
		c := http.Client{}
		res, err := c.Get("http://localhost:9091/metrics")
		Expect(err).NotTo(HaveOccurred())
		contents, err := ioutil.ReadAll(res.Body)
		fmt.Printf("%s\n", string(contents))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_updated 3"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_failed 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_containers_scanned 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 1"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 0"))

		for i := 0; i < 3; i++ {
			m.RegisterSkipped()
		}

		res, err = c.Get("http://localhost:9091/metrics")
		Expect(err).NotTo(HaveOccurred())
		contents, err = ioutil.ReadAll(res.Body)
		fmt.Printf("%s\n", string(contents))

		Expect(string(contents)).To(ContainSubstring("watchtower_scans_total 4"))
		Expect(string(contents)).To(ContainSubstring("watchtower_scans_skipped 3"))

	})
})
