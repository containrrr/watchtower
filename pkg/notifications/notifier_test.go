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
	Describe("the slack notifier", func() {
		builderFn := notifications.NewSlackNotifier

		When("passing a discord url to the slack notifier", func() {
			channel := "123456789"
			token := "abvsihdbau"
			expected := fmt.Sprintf("discord://%s@%s", token, channel)
			buildArgs := func(url string) []string {
				return []string{
					"--notifications",
					"slack",
					"--notification-slack-hook-url",
					url,
				}
			}

			It("should return a discord url when using a hook url with the domain discord.com", func() {
				hookURL := fmt.Sprintf("https://%s/api/webhooks/%s/%s/slack", "discord.com", channel, token)
				testURL(builderFn, buildArgs(hookURL), expected)
			})
			It("should return a discord url when using a hook url with the domain discordapp.com", func() {
				hookURL := fmt.Sprintf("https://%s/api/webhooks/%s/%s/slack", "discordapp.com", channel, token)
				testURL(builderFn, buildArgs(hookURL), expected)
			})
		})
		When("converting a slack service config into a shoutrrr url", func() {

			It("should return the expected URL", func() {

				username := "containrrrbot"
				tokenA := "aaa"
				tokenB := "bbb"
				tokenC := "ccc"

				hookURL := fmt.Sprintf("https://hooks.slack.com/services/%s/%s/%s", tokenA, tokenB, tokenC)
				expectedOutput := fmt.Sprintf("slack://%s@%s/%s/%s", username, tokenA, tokenB, tokenC)

				args := []string{
					"--notification-slack-hook-url",
					hookURL,
					"--notification-slack-identifier",
					username,
				}

				testURL(builderFn, args, expectedOutput)
			})
		})
	})

	Describe("the gotify notifier", func() {
		When("converting a gotify service config into a shoutrrr url", func() {
			builderFn := notifications.NewGotifyNotifier

			It("should return the expected URL", func() {
				token := "aaa"
				host := "shoutrrr.local"

				expectedOutput := fmt.Sprintf("gotify://%s/%s", host, token)

				args := []string{
					"--notification-gotify-url",
					fmt.Sprintf("https://%s", host),
					"--notification-gotify-token",
					token,
				}

				testURL(builderFn, args, expectedOutput)
			})
		})
	})

	Describe("the teams notifier", func() {
		When("converting a teams service config into a shoutrrr url", func() {
			builderFn := notifications.NewMsTeamsNotifier

			It("should return the expected URL", func() {

				tokenA := "aaa"
				tokenB := "bbb"
				tokenC := "ccc"

				hookURL := fmt.Sprintf("https://outlook.office.com/webhook/%s/IncomingWebhook/%s/%s", tokenA, tokenB, tokenC)
				expectedOutput := fmt.Sprintf("teams://%s:%s@%s", tokenA, tokenB, tokenC)

				args := []string{
					"--notification-msteams-hook",
					hookURL,
				}

				testURL(builderFn, args, expectedOutput)
			})
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

				fromAddress := "sender@example.com"
				toAddress := "receiver@example.com"
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

	var template = "smtp://%s:%s@%s:%d/?auth=%s&encryption=None&fromaddress=%s&fromname=Watchtower&starttls=Yes&subject=%s&toaddresses=%s&usehtml=No"
	return fmt.Sprintf(template, username, password, host, port, auth, from, subject, to)
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
