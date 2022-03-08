package notifications_test

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/containrrr/watchtower/cmd"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/pkg/notifications"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("notifications", func() {
	Describe("the notifier", func() {
		When("only empty notifier types are provided", func() {

			command := cmd.NewRootCommand()
			flags.RegisterNotificationFlags(command)

			err := command.ParseFlags([]string{
				"--notifications",
				"shoutrrr",
			})
			Expect(err).NotTo(HaveOccurred())
			notif := notifications.NewNotifier(command)

			Expect(notif.GetNames()).To(BeEmpty())
		})
		When("title is overriden in flag", func() {
			It("should use the specified hostname in the title", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				err := command.ParseFlags([]string{
					"--notifications-hostname",
					"test.host",
				})
				Expect(err).NotTo(HaveOccurred())
				hostname := notifications.GetHostname(command)
				title := notifications.GetTitle(hostname)
				Expect(title).To(Equal("Watchtower updates on test.host"))
			})
		})
		When("no hostname can be resolved", func() {
			It("should use the default simple title", func() {
				title := notifications.GetTitle("")
				Expect(title).To(Equal("Watchtower updates"))
			})
		})
		When("no delay is defined", func() {
			It("should use the default delay", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				delay := notifications.GetDelay(command, time.Duration(0))
				Expect(delay).To(Equal(time.Duration(0)))
			})
		})
		When("delay is defined", func() {
			It("should use the specified delay", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				err := command.ParseFlags([]string{
					"--notifications-delay",
					"5",
				})
				Expect(err).NotTo(HaveOccurred())
				delay := notifications.GetDelay(command, time.Duration(0))
				Expect(delay).To(Equal(time.Duration(5) * time.Second))
			})
		})
		When("legacy delay is defined", func() {
			It("should use the specified legacy delay", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)
				delay := notifications.GetDelay(command, time.Duration(5)*time.Second)
				Expect(delay).To(Equal(time.Duration(5) * time.Second))
			})
		})
		When("legacy delay and delay is defined", func() {
			It("should use the specified legacy delay", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				err := command.ParseFlags([]string{
					"--notifications-delay",
					"5",
				})
				Expect(err).NotTo(HaveOccurred())
				delay := notifications.GetDelay(command, time.Duration(7)*time.Second)
				Expect(delay).To(Equal(time.Duration(7) * time.Second))
			})
		})
	})
	Describe("the slack notifier", func() {
		// builderFn := notifications.NewSlackNotifier

		When("passing a discord url to the slack notifier", func() {
			command := cmd.NewRootCommand()
			flags.RegisterNotificationFlags(command)

			channel := "123456789"
			token := "abvsihdbau"
			color := notifications.ColorInt
			hostname := notifications.GetHostname(command)
			title := url.QueryEscape(notifications.GetTitle(hostname))
			expected := fmt.Sprintf("discord://%s@%s?color=0x%x&colordebug=0x0&colorerror=0x0&colorinfo=0x0&colorwarn=0x0&title=%s&username=watchtower", token, channel, color, title)
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
				testURL(buildArgs(hookURL), expected, time.Duration(0))
			})
			It("should return a discord url when using a hook url with the domain discordapp.com", func() {
				hookURL := fmt.Sprintf("https://%s/api/webhooks/%s/%s/slack", "discordapp.com", channel, token)
				testURL(buildArgs(hookURL), expected, time.Duration(0))
			})
		})
		When("converting a slack service config into a shoutrrr url", func() {
			command := cmd.NewRootCommand()
			flags.RegisterNotificationFlags(command)
			username := "containrrrbot"
			tokenA := "AAAAAAAAA"
			tokenB := "BBBBBBBBB"
			tokenC := "123456789123456789123456"
			color := url.QueryEscape(notifications.ColorHex)
			hostname := notifications.GetHostname(command)
			title := url.QueryEscape(notifications.GetTitle(hostname))
			iconURL := "https://containrrr.dev/watchtower-sq180.png"
			iconEmoji := "whale"

			When("icon URL is specified", func() {
				It("should return the expected URL", func() {

					hookURL := fmt.Sprintf("https://hooks.slack.com/services/%s/%s/%s", tokenA, tokenB, tokenC)
					expectedOutput := fmt.Sprintf("slack://hook:%s-%s-%s@webhook?botname=%s&color=%s&icon=%s&title=%s", tokenA, tokenB, tokenC, username, color, url.QueryEscape(iconURL), title)
					expectedDelay := time.Duration(7) * time.Second

					args := []string{
						"--notifications",
						"slack",
						"--notification-slack-hook-url",
						hookURL,
						"--notification-slack-identifier",
						username,
						"--notification-slack-icon-url",
						iconURL,
						"--notifications-delay",
						fmt.Sprint(expectedDelay.Seconds()),
					}

					testURL(args, expectedOutput, expectedDelay)
				})
			})

			When("icon emoji is specified", func() {
				It("should return the expected URL", func() {
					hookURL := fmt.Sprintf("https://hooks.slack.com/services/%s/%s/%s", tokenA, tokenB, tokenC)
					expectedOutput := fmt.Sprintf("slack://hook:%s-%s-%s@webhook?botname=%s&color=%s&icon=%s&title=%s", tokenA, tokenB, tokenC, username, color, iconEmoji, title)

					args := []string{
						"--notifications",
						"slack",
						"--notification-slack-hook-url",
						hookURL,
						"--notification-slack-identifier",
						username,
						"--notification-slack-icon-emoji",
						iconEmoji,
					}

					testURL(args, expectedOutput, time.Duration(0))
				})
			})
		})
	})

	Describe("the gotify notifier", func() {
		When("converting a gotify service config into a shoutrrr url", func() {
			It("should return the expected URL", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				token := "aaa"
				host := "shoutrrr.local"
				hostname := notifications.GetHostname(command)
				title := url.QueryEscape(notifications.GetTitle(hostname))

				expectedOutput := fmt.Sprintf("gotify://%s/%s?title=%s", host, token, title)

				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					fmt.Sprintf("https://%s", host),
					"--notification-gotify-token",
					token,
				}

				testURL(args, expectedOutput, time.Duration(0))
			})
		})
	})

	Describe("the teams notifier", func() {
		When("converting a teams service config into a shoutrrr url", func() {
			It("should return the expected URL", func() {
				command := cmd.NewRootCommand()
				flags.RegisterNotificationFlags(command)

				tokenA := "11111111-4444-4444-8444-cccccccccccc@22222222-4444-4444-8444-cccccccccccc"
				tokenB := "33333333012222222222333333333344"
				tokenC := "44444444-4444-4444-8444-cccccccccccc"
				color := url.QueryEscape(notifications.ColorHex)
				hostname := notifications.GetHostname(command)
				title := url.QueryEscape(notifications.GetTitle(hostname))

				hookURL := fmt.Sprintf("https://outlook.office.com/webhook/%s/IncomingWebhook/%s/%s", tokenA, tokenB, tokenC)
				expectedOutput := fmt.Sprintf("teams://%s/%s/%s?color=%s&title=%s", tokenA, tokenB, tokenC, color, title)

				args := []string{
					"--notifications",
					"msteams",
					"--notification-msteams-hook",
					hookURL,
				}

				testURL(args, expectedOutput, time.Duration(0))
			})
		})
	})

	Describe("the email notifier", func() {
		When("converting an email service config into a shoutrrr url", func() {
			It("should set the from address in the URL", func() {
				fromAddress := "lala@example.com"
				expectedOutput := buildExpectedURL("containrrrbot", "secret-password", "mail.containrrr.dev", 25, fromAddress, "mail@example.com", "Plain")
				expectedDelay := time.Duration(7) * time.Second

				args := []string{
					"--notifications",
					"email",
					"--notification-email-from",
					fromAddress,
					"--notification-email-to",
					"mail@example.com",
					"--notification-email-server-user",
					"containrrrbot",
					"--notification-email-server-password",
					"secret-password",
					"--notification-email-server",
					"mail.containrrr.dev",
					"--notifications-delay",
					fmt.Sprint(expectedDelay.Seconds()),
				}
				testURL(args, expectedOutput, expectedDelay)
			})

			It("should return the expected URL", func() {

				fromAddress := "sender@example.com"
				toAddress := "receiver@example.com"
				expectedOutput := buildExpectedURL("containrrrbot", "secret-password", "mail.containrrr.dev", 25, fromAddress, toAddress, "Plain")
				expectedDelay := time.Duration(7) * time.Second

				args := []string{
					"--notifications",
					"email",
					"--notification-email-from",
					fromAddress,
					"--notification-email-to",
					toAddress,
					"--notification-email-server-user",
					"containrrrbot",
					"--notification-email-server-password",
					"secret-password",
					"--notification-email-server",
					"mail.containrrr.dev",
					"--notification-email-delay",
					fmt.Sprint(expectedDelay.Seconds()),
				}

				testURL(args, expectedOutput, expectedDelay)
			})
		})
	})
})

func buildExpectedURL(username string, password string, host string, port int, from string, to string, auth string) string {
	hostname, err := os.Hostname()
	Expect(err).NotTo(HaveOccurred())

	subject := fmt.Sprintf("Watchtower updates on %s", hostname)

	var template = "smtp://%s:%s@%s:%d/?auth=%s&fromaddress=%s&fromname=Watchtower&subject=%s&toaddresses=%s"
	return fmt.Sprintf(template,
		url.QueryEscape(username),
		url.QueryEscape(password),
		host, port, auth,
		url.QueryEscape(from),
		url.QueryEscape(subject),
		url.QueryEscape(to))
}

func testURL(args []string, expectedURL string, expectedDelay time.Duration) {
	defer GinkgoRecover()

	command := cmd.NewRootCommand()
	flags.RegisterNotificationFlags(command)

	err := command.ParseFlags(args)
	Expect(err).NotTo(HaveOccurred())

	hostname := notifications.GetHostname(command)
	title := notifications.GetTitle(hostname)
	urls, delay := notifications.AppendLegacyUrls([]string{}, command, title)

	Expect(err).NotTo(HaveOccurred())

	Expect(urls).To(ContainElement(expectedURL))
	Expect(delay).To(Equal(expectedDelay))
}
