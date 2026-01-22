package digest_test

import (
    "github.com/containrrr/watchtower/pkg/registry"
    "github.com/containrrr/watchtower/pkg/registry/digest"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

var _ = Describe("Digest transport configuration", func() {
    AfterEach(func() {
        // Reset to default after each test
        registry.InsecureSkipVerify = false
    })

    It("should have nil TLSClientConfig by default", func() {
        registry.InsecureSkipVerify = false
        tr := digest.NewTransportForTest()
        Expect(tr.TLSClientConfig).To(BeNil())
    })

    It("should set TLSClientConfig when insecure flag is true", func() {
        registry.InsecureSkipVerify = true
        tr := digest.NewTransportForTest()
        Expect(tr.TLSClientConfig).ToNot(BeNil())
    })
})
