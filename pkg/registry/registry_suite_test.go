package registry_test

import (
	"testing"

	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	logrus.SetOutput(GinkgoWriter)
	RunSpecs(t, "Registry Suite")
}
