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
	/*
	 * TODO:
	 * This part only confirms that it still works in the same way as it did
	 * with the old version of the docker api client sdk. I'd say that
	 * ParseServerAddress likely needs to be elaborated a bit to default to
	 * dockerhub in case no server address was provided.
	 *
	 * ++ @simskij, 2019-04-04
	 */
	It("parse server address_ should return error if passed empty string", func() {

		_, err := ParseServerAddress("")
		Expect(err).To(HaveOccurred())
	})
	It("parse server address_ should return the organization part if passed an image name missing server name", func() {

		val, _ := ParseServerAddress("containrrr/config")
		Expect(val).To(Equal("containrrr"))
	})
	It("parse server address_ should return the server name if passed a fully qualified image name", func() {

		val, _ := ParseServerAddress("github.com/containrrrr/config")
		Expect(val).To(Equal("github.com"))
	})
})
