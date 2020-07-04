package notifications_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/containrrr/watchtower/cmd"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/pkg/notifications"
	"github.com/containrrr/watchtower/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func TestActions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Notifier Suite")
}

var _ = Describe("notifications", func() {
	When("getting notifiers from a types array", func() {
		It("should return the same amount of notifiers a string entries", func() {
			notifier := &notifications.Notifier{}
			notifiers := notifier.GetNotificationTypes(&cobra.Command{}, []log.Level{}, []string{"slack", "email"})
			Expect(len(notifiers)).To(Equal(2))
		})
	})
	Describe("the email notifier", func() {

		builderFn := notifications.NewEmailNotifier

		When("converting an email service config into a shoutrrr url", func() {
			It("should set the from address in the URL", func() {
				fromAddress := "lala@example.com"
				expectedOutput := buildExpectedURL("", "", "", 25, fromAddress, "", "None")

				args := []string{
					"--notification-email-from",
					fromAddress,
				}

				testURL(builderFn, args, expectedOutput)
			})

			It("should return the expected URL", func() {

				fromAddress := "lala@example.com"
				toAddress := "papa@example.com"
				expectedOutput := buildExpectedURL("", "", "", 25, fromAddress, toAddress, "None")

				args := []string{
					"--notification-email-from",
					fromAddress,
					"--notification-email-to",
					toAddress,
				}

				testURL(builderFn, args, expectedOutput)

			})
		})
	})
})

func buildExpectedURL(username string, password string, host string, port int, from string, to string, auth string) string {
	hostname, err := os.Hostname()
	Expect(err).NotTo(HaveOccurred())

	subject := fmt.Sprintf("Watchtower updates on %s", hostname)

	var template = "smtp://%s:%s@%s:%d/?fromAddress=%s&fromName=Watchtower&toAddresses=%s&auth=%s&subject=%s&startTls=No&useHTML=No"
	return fmt.Sprintf(template, username, password, host, port, from, to, auth, subject)
}

type builderFn = func(c *cobra.Command, acceptedLogLevels []log.Level) types.ConvertableNotifier

func testURL(builder builderFn, args []string, expectedURL string) {
	command := cmd.NewRootCommand()
	flags.RegisterNotificationFlags(command)

	command.ParseFlags(args)

	notifier := builder(command, []log.Level{})
	actualURL := notifier.GetURL()

	Expect(actualURL).To(Equal(expectedURL))
}
