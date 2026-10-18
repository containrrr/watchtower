package notifications_test

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/cmd"
	"github.com/nicholas-fedor/watchtower/internal/config"
	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/pkg/notifications"
)

var testLog = func() *zerolog.Logger {
	l := zerolog.Nop()

	return &l
}()

var _ = ginkgo.Describe("notifications", func() {
	ginkgo.Describe("the notifier", func() {
		ginkgo.When("only empty notifier types are provided", func() {
			ginkgo.It("should configure no services", func() {
				notifier := notifications.NewNotifier(testLog, loadNotify([]string{
					"--notifications",
					"shoutrrr",
				}))

				gomega.Expect(notifier.GetNames()).To(gomega.BeEmpty())
			})
		})
		ginkgo.When("title is overridden in flag", func() {
			ginkgo.It("should use the specified hostname in the title", func() {
				data := templateData([]string{
					"--notifications-hostname",
					"test.host",
				})
				title := data.Title
				gomega.Expect(title).To(gomega.Equal("Watchtower updates on test.host"))
			})
		})
		ginkgo.When("no hostname can be resolved", func() {
			ginkgo.It("should use the default simple title", func() {
				title := notifications.GetTitle(testLog, "", "")
				gomega.Expect(title).To(gomega.Equal("Watchtower updates"))
			})
		})
		ginkgo.When("title tag is set", func() {
			ginkgo.It("should use the prefix in the title", func() {
				data := templateData([]string{
					"--notification-title-tag",
					"PREFIX",
				})
				gomega.Expect(data.Title).To(gomega.HavePrefix("[PREFIX]"))
			})
		})
		ginkgo.When("legacy email tag is set", func() {
			//nolint:godox
			// TODO: Remove legacy email subjecttag test when legacy notification types are removed.
			ginkgo.It("should use the prefix in the title", func() {
				data := templateData([]string{
					"--notification-email-subjecttag",
					"PREFIX",
				})
				gomega.Expect(data.Title).To(gomega.HavePrefix("[PREFIX]"))
			})
		})
		ginkgo.When("the skip title flag is set", func() {
			ginkgo.It("should return an empty title", func() {
				data := templateData([]string{
					"--notification-skip-title",
				})
				gomega.Expect(data.Title).To(gomega.BeEmpty())
			})
		})
		ginkgo.When("no delay is defined", func() {
			ginkgo.It("should use the default delay", func() {
				delay := notifications.GetDelay(testLog, 0, time.Duration(0))
				gomega.Expect(delay).To(gomega.Equal(time.Duration(0)))
			})
		})
		ginkgo.When("delay is defined", func() {
			ginkgo.It("should use the specified delay", func() {
				delay := notifications.GetDelay(testLog, 5, time.Duration(0))
				gomega.Expect(delay).To(gomega.Equal(5 * time.Second))
			})
		})
		ginkgo.When("legacy delay is defined", func() {
			//nolint:godox
			// TODO: Remove legacy delay tests when legacy notification types are removed.
			ginkgo.It("should use the specified legacy delay", func() {
				delay := notifications.GetDelay(testLog, 0, 5*time.Second)
				gomega.Expect(delay).To(gomega.Equal(5 * time.Second))
			})
		})
		ginkgo.When("legacy delay and delay is defined", func() {
			//nolint:godox
			// TODO: Remove legacy delay tests when legacy notification types are removed.
			ginkgo.It(
				"should use the specified legacy delay and ignore the specified delay",
				func() {
					delay := notifications.GetDelay(testLog, 5, 7*time.Second)
					gomega.Expect(delay).To(gomega.Equal(7 * time.Second))
				},
			)
		})
		ginkgo.When("notification template file is specified", func() {
			ginkgo.It("should load template from file", func() {
				content := "{{.Data.Host}} updated"
				tmpFile, err := os.CreateTemp("", "template")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				defer os.Remove(tmpFile.Name())

				_, err = tmpFile.WriteString(content)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				tmpFile.Close()

				notifier := notifications.NewNotifier(testLog, loadNotify([]string{
					"--notification-url",
					"logger://",
					"--notification-template-file",
					tmpFile.Name(),
				}))
				gomega.Expect(notifier).NotTo(gomega.BeNil())
				gomega.Expect(notifier.GetNames()).To(gomega.ContainElement("logger"))
			})
		})
	})
	//nolint:godox
	// TODO: Remove legacy slack notifier tests when legacy notification types are removed.
	ginkgo.Describe("the slack notifier", func() {
		ginkgo.When("passing a discord url to the slack notifier", func() {
			channel := "123456789"
			token := "abvsihdbau"
			color := notifications.ColorInt
			username := "containrrrbot"
			iconURL := "https://github.com/nicholas-fedor/watchtower/blob/main/watchtower-sq180.png"
			expected := fmt.Sprintf(
				"discord://%s@%s?color=0x%x&colordebug=0x0&colorerror=0x0&colorinfo=0x0&colorwarn=0x0&username=watchtower",
				token,
				channel,
				color,
			)
			buildArgs := func(url string) []string {
				return []string{
					"--notifications",
					"slack",
					"--notification-slack-hook-url",
					url,
				}
			}

			ginkgo.It(
				"should return a discord url ginkgo.when using a hook url with the domain discord.com",
				func() {
					hookURL := fmt.Sprintf(
						"https://%s/api/webhooks/%s/%s/slack",
						"discord.com",
						channel,
						token,
					)
					testURL(buildArgs(hookURL), expected, time.Duration(0))
				},
			)
			ginkgo.It(
				"should return a discord url ginkgo.when using a hook url with the domain discordapp.com",
				func() {
					hookURL := fmt.Sprintf(
						"https://%s/api/webhooks/%s/%s/slack",
						"discordapp.com",
						channel,
						token,
					)
					testURL(buildArgs(hookURL), expected, time.Duration(0))
				},
			)
			ginkgo.When("icon URL and username are specified", func() {
				ginkgo.It("should return the expected URL", func() {
					hookURL := fmt.Sprintf(
						"https://%s/api/webhooks/%s/%s/slack",
						"discord.com",
						channel,
						token,
					)
					expectedOutput := fmt.Sprintf(
						"discord://%s@%s?avatar=%s&color=0x%x&colordebug=0x0&colorerror=0x0&colorinfo=0x0&colorwarn=0x0&username=%s",
						token,
						channel,
						url.QueryEscape(iconURL),
						color,
						username,
					)
					expectedDelay := 7 * time.Second
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
		})
		ginkgo.When("converting a slack service config into a shoutrrr url", func() {
			username := "containrrrbot"
			tokenA := "AAAAAAAAA"
			tokenB := "BBBBBBBBB"
			tokenC := "123456789123456789123456"
			color := url.QueryEscape(notifications.ColorHex)
			iconURL := "https://github.com/nicholas-fedor/watchtower/blob/main/watchtower-sq180.png"
			iconEmoji := "whale"

			ginkgo.When("icon URL is specified", func() {
				ginkgo.It("should return the expected URL", func() {
					hookURL := fmt.Sprintf(
						"https://hooks.slack.com/services/%s/%s/%s",
						tokenA,
						tokenB,
						tokenC,
					)
					expectedOutput := fmt.Sprintf(
						"slack://hook:%s-%s-%s@webhook?botname=%s&color=%s&icon=%s",
						tokenA,
						tokenB,
						tokenC,
						username,
						color,
						url.QueryEscape(iconURL),
					)
					expectedDelay := 7 * time.Second

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

			ginkgo.When("icon emoji is specified", func() {
				ginkgo.It("should return the expected URL", func() {
					hookURL := fmt.Sprintf(
						"https://hooks.slack.com/services/%s/%s/%s",
						tokenA,
						tokenB,
						tokenC,
					)
					expectedOutput := fmt.Sprintf(
						"slack://hook:%s-%s-%s@webhook?botname=%s&color=%s&icon=%s",
						tokenA,
						tokenB,
						tokenC,
						username,
						color,
						iconEmoji,
					)

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
		ginkgo.When("the hook URL is empty", func() {
			ginkgo.It("should fatal with a clear missing argument message", func() {
				expectNewNotifierFatal([]string{
					"--notifications",
					"slack",
				}, "Slack hook URL is empty.")
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy gotify notifier tests when legacy notification types are removed.
	ginkgo.Describe("the gotify notifier", func() {
		ginkgo.When("converting a gotify service config into a shoutrrr url", func() {
			ginkgo.It("should return the expected URL", func() {
				token := "aaa"
				host := "shoutrrr.local"

				expectedOutput := fmt.Sprintf("gotify://%s/%s?title=", host, token)

				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					"https://" + host,
					"--notification-gotify-token",
					token,
				}

				testURL(args, expectedOutput, time.Duration(0))
			})
		})
		ginkgo.When("initializing a gotify notifier via NewNotifier", func() {
			ginkgo.BeforeEach(func() {
			})

			ginkgo.It("should configure with valid flags", func() {
				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					"https://gotify.example.com",
					"--notification-gotify-token",
					"test-token",
					"--notification-gotify-tls-skip-verify",
				}
				notifier := notifications.NewNotifier(testLog, loadNotify(args))
				names := notifier.GetNames()
				gomega.Expect(names).To(gomega.ContainElement("gotify"))

				urls := notifier.GetURLs()
				gomega.Expect(urls).
					To(gomega.ContainElement(gomega.ContainSubstring("gotify://gotify.example.com/test-token")))
			})

			ginkgo.It("should log token at trace level", func() {
				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					"https://gotify.example.com",
					"--notification-gotify-token",
					"test-token",
				}
				notifier := notifications.NewNotifier(testLog, loadNotify(args))
				names := notifier.GetNames()
				gomega.Expect(names).To(gomega.ContainElement("gotify"))

				urls := notifier.GetURLs()
				gomega.Expect(urls).
					To(gomega.ContainElement(gomega.ContainSubstring("gotify://gotify.example.com/test-token")))
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy msteams notifier tests when legacy notification types are removed.
	ginkgo.Describe("the teams notifier", func() {
		ginkgo.BeforeEach(func() {
		})
		ginkgo.When("converting a teams service config into a shoutrrr url", func() {
			ginkgo.It("should return the expected URL", func() {
				color := url.QueryEscape(notifications.ColorHex)

				// Power Automate workflow incoming webhook URL.
				hookURL := "https://default.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123/triggers/manual/paths/invoke?api-version=1&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=XXXXXXXX"
				expectedOutput := fmt.Sprintf(
					"teams:?color=%s&host=%s",
					color,
					url.QueryEscape(hookURL),
				)

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

	//nolint:godox
	// TODO: Remove legacy email notifier tests when legacy notification types are removed.
	ginkgo.Describe("the email notifier", func() {
		ginkgo.When("converting an email service config into a shoutrrr url", func() {
			ginkgo.It("should set the from address in the URL", func() {
				fromAddress := "lala@example.com"
				expectedOutput := buildExpectedURL(
					"containrrrbot",
					"secret-password",
					"mail.watchtower.dev",
					25,
					fromAddress,
					"mail@example.com",
					"Plain",
				)
				expectedDelay := 7 * time.Second

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
					"mail.watchtower.dev",
					"--notifications-delay",
					fmt.Sprint(expectedDelay.Seconds()),
				}
				testURL(args, expectedOutput, expectedDelay)
			})

			ginkgo.It("should return the expected URL", func() {
				fromAddress := "sender@example.com"
				toAddress := "receiver@example.com"
				expectedOutput := buildExpectedURL(
					"containrrrbot",
					"secret-password",
					"mail.watchtower.dev",
					25,
					fromAddress,
					toAddress,
					"Plain",
				)
				expectedDelay := 7 * time.Second

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
					"mail.watchtower.dev",
					"--notification-email-delay",
					fmt.Sprint(expectedDelay.Seconds()),
				}

				testURL(args, expectedOutput, expectedDelay)
			})
		})
		ginkgo.When("a required field is empty", func() {
			ginkgo.It("should fatal when the from address is empty", func() {
				expectNewNotifierFatal([]string{
					"--notifications",
					"email",
					"--notification-email-to",
					"to@example.com",
					"--notification-email-server",
					"mail.example.com",
				}, "Email from address is empty.")
			})

			ginkgo.It("should fatal when the to address is empty", func() {
				expectNewNotifierFatal([]string{
					"--notifications",
					"email",
					"--notification-email-from",
					"from@example.com",
					"--notification-email-server",
					"mail.example.com",
				}, "Email to address is empty.")
			})

			ginkgo.It("should fatal when the server is empty", func() {
				expectNewNotifierFatal([]string{
					"--notifications",
					"email",
					"--notification-email-from",
					"from@example.com",
					"--notification-email-to",
					"to@example.com",
				}, "Email server is empty.")
			})
		})
	})
})

// expectNewNotifierFatal runs NewNotifier under a temporary FatalExitFunc
// that panics instead of os.Exit, captures buffer-backed fatal log output, and
// restores the previous exit callback.
//
// Parameters:
//   - args: CLI flags to load before constructing the notifier
//   - wantMsg: exact fatal message text expected in the log output
func expectNewNotifierFatal(args []string, wantMsg string) {
	ginkgo.GinkgoHelper()

	oldFatalExit := zerolog.FatalExitFunc
	defer func() { zerolog.FatalExitFunc = oldFatalExit }()

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.FatalLevel)

	zerolog.FatalExitFunc = func() {
		panic("fatal exit")
	}

	cfg := loadNotify(args)

	gomega.Expect(func() {
		notifications.NewNotifier(&log, cfg)
	}).To(gomega.PanicWith("fatal exit"))

	out := buf.String()
	gomega.Expect(out).To(gomega.ContainSubstring(`"message":"`+wantMsg+`"`),
		"expected exact fatal message %q in output:\n%s", wantMsg, out)
}

// TODO: Remove buildExpectedURL helper when legacy notification tests are removed.
//
//nolint:godox
func buildExpectedURL(
	username string,
	password string,
	host string,
	port int,
	from string,
	destAddress string,
	auth string,
) string {
	template := "smtp://%s:%s@%s:%d/?auth=%s&clienthost=localhost&encryption=Auto&fromaddress=%s&fromname=Watchtower&subject=&toaddresses=%s&usehtml=No&usestarttls=Yes&timeout=10s"

	return fmt.Sprintf(template,
		url.QueryEscape(username),
		url.QueryEscape(password),
		host, port, auth,
		url.QueryEscape(from),
		url.QueryEscape(destAddress))
}

// TODO: Remove testURL helper when legacy notification tests are removed.
//
//nolint:godox
func testURL(args []string, expectedURL string, expectedDelay time.Duration) {
	defer ginkgo.GinkgoRecover()

	cfg := loadNotify(args)

	urls, legacyDelay := notifications.AppendLegacyURLs(testLog, []string{}, cfg.LegacyTypes, cfg.Legacy)
	delay := notifications.GetDelay(testLog, cfg.DelaySeconds, legacyDelay)

	gomega.Expect(urls).To(gomega.ContainElement(expectedURL))
	gomega.Expect(delay).To(gomega.Equal(expectedDelay))
}

// loadNotify loads the notification settings from args the way production
// does, by parsing them into the root command's flags and running config.Load.
//
// Parameters:
//   - args: CLI flags to parse.
//
// Returns:
//   - notifyConfig.Notify: The resolved notification settings.
func loadNotify(args []string) notifyConfig.Notify {
	ginkgo.GinkgoHelper()

	command := cmd.NewRootCommand()
	flags.RegisterAll(command)
	gomega.Expect(command.ParseFlags(args)).To(gomega.Succeed())

	cfg, err := config.Load(testLog, command, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return cfg.Notify
}

// templateData returns the static notification data for the settings loaded
// from args.
//
// Parameters:
//   - args: CLI flags to parse.
//
// Returns:
//   - notifications.StaticData: Host and title for notification templates.
func templateData(args []string) notifications.StaticData {
	ginkgo.GinkgoHelper()

	cfg := loadNotify(args)

	return notifications.TemplateData(testLog, cfg.Hostname, cfg.TitleTag, cfg.EmailSubjectTag, cfg.SkipTitle)
}
