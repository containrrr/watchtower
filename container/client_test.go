package container

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the client", func() {
	When("creating a new client", func() {
		It("should return a client for the api", func() {
			client := NewClient(false)
			Expect(client).NotTo(BeNil())
		})
	})
})