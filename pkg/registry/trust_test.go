package registry

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("encoded env auth_ should return an error if repo envs are unset", func() {
		_ = os.Unsetenv("REPO_USER")
		_ = os.Unsetenv("REPO_PASS")

		_, err := EncodedEnvAuth("")
		Expect(err).To(HaveOccurred())
	})
	It("encoded env auth_ should return auth hash if repo envs are set", func() {
		var err error
		expectedHash := "eyJ1c2VybmFtZSI6ImNvbnRhaW5ycnItdXNlciIsInBhc3N3b3JkIjoiY29udGFpbnJyci1wYXNzIn0="

		err = os.Setenv("REPO_USER", "containrrr-user")
		Expect(err).NotTo(HaveOccurred())

		err = os.Setenv("REPO_PASS", "containrrr-pass")
		Expect(err).NotTo(HaveOccurred())

		config, err := EncodedEnvAuth("")
		Expect(config).To(Equal(expectedHash))
		Expect(err).NotTo(HaveOccurred())
	})
	It("encoded config auth_ should return an error if file is not present", func() {
		var err error

		err = os.Setenv("DOCKER_CONFIG", "/dev/null/should-fail")
		Expect(err).NotTo(HaveOccurred())

		_, err = EncodedConfigAuth("")
		Expect(err).To(HaveOccurred())
	})
	})
})
