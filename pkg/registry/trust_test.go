package registry

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry credential helpers", func() {
	Describe("EncodedAuth", func() {
		It("should return repo credentials from env when set", func() {
			var err error
			expected := "eyJ1c2VybmFtZSI6ImNvbnRhaW5ycnItdXNlciIsInBhc3N3b3JkIjoiY29udGFpbnJyci1wYXNzIn0="

			err = os.Setenv("REPO_USER", "containrrr-user")
			Expect(err).NotTo(HaveOccurred())

			err = os.Setenv("REPO_PASS", "containrrr-pass")
			Expect(err).NotTo(HaveOccurred())

			config, err := EncodedEnvAuth()
			Expect(config).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("EncodedEnvAuth", func() {
		It("should return an error if repo envs are unset", func() {
			_ = os.Unsetenv("REPO_USER")
			_ = os.Unsetenv("REPO_PASS")

			_, err := EncodedEnvAuth()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("EncodedConfigAuth", func() {
		It("should return an error if file is not present", func() {
			var err error

			err = os.Setenv("DOCKER_CONFIG", "/dev/null/should-fail")
			Expect(err).NotTo(HaveOccurred())

			_, err = EncodedConfigAuth("")
			Expect(err).To(HaveOccurred())
		})
	})
})
