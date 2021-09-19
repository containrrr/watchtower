package notifications_test

import (
	"github.com/onsi/gomega/format"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNotifications(t *testing.T) {
	RegisterFailHandler(Fail)
	format.CharactersAroundMismatchToInclude = 20
	RunSpecs(t, "Notifications Suite")
}
