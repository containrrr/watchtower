package notifications

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/config"
	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/flags"
)

// fatalExitMu serializes the tests that replace zerolog.FatalExitFunc, which
// is process-wide.
var fatalExitMu sync.Mutex

// TestNotifier verifies the notifier that NewNotifier builds from the
// notification settings: its services, title, delay, and the shoutrrr URLs of
// the legacy notifiers, and that a legacy email notifier without a required
// setting is fatal.
func TestNotifier(t *testing.T) {
	t.Parallel()

	t.Run("the notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("only empty notifier types are provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should configure no services", func(t *testing.T) {
				t.Parallel()

				notifier := NewNotifier(newTestLogger(t), loadNotify(t, []string{
					"--notifications",
					"shoutrrr",
				}))

				assert.Empty(t, notifier.GetNames())
			})
		})
		t.Run("title is overridden in flag", func(t *testing.T) {
			t.Parallel()

			t.Run("should use the specified hostname in the title", func(t *testing.T) {
				t.Parallel()

				data := templateDataFromFlags(t, []string{
					"--notifications-hostname",
					"test.host",
				})
				title := data.Title
				assert.Equal(t, "Watchtower updates on test.host", title)
			})
		})
		t.Run("no hostname can be resolved", func(t *testing.T) {
			t.Parallel()

			t.Run("should use the default simple title", func(t *testing.T) {
				t.Parallel()

				title := getTitle(newTestLogger(t), "", "")
				assert.Equal(t, "Watchtower updates", title)
			})
		})
		t.Run("title tag is set", func(t *testing.T) {
			t.Parallel()

			t.Run("should use the prefix in the title", func(t *testing.T) {
				t.Parallel()

				data := templateDataFromFlags(t, []string{
					"--notification-title-tag",
					"PREFIX",
				})
				assert.True(t, strings.HasPrefix(data.Title, "[PREFIX]"))
			})
		})
		t.Run("legacy email tag is set", func(t *testing.T) {
			t.Parallel()

			//nolint:godox
			// TODO: Remove legacy email subjecttag test when legacy notification types are removed.
			t.Run("should use the prefix in the title", func(t *testing.T) {
				t.Parallel()

				data := templateDataFromFlags(t, []string{
					"--notification-email-subjecttag",
					"PREFIX",
				})
				assert.True(t, strings.HasPrefix(data.Title, "[PREFIX]"))
			})
		})
		t.Run("the skip title flag is set", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an empty title", func(t *testing.T) {
				t.Parallel()

				data := templateDataFromFlags(t, []string{
					"--notification-skip-title",
				})
				assert.Empty(t, data.Title)
			})
		})
		t.Run("no delay is defined", func(t *testing.T) {
			t.Parallel()

			t.Run("should use the default delay", func(t *testing.T) {
				t.Parallel()

				delay := getDelay(newTestLogger(t), 0, time.Duration(0))
				assert.Equal(t, time.Duration(0), delay)
			})
		})
		t.Run("delay is defined", func(t *testing.T) {
			t.Parallel()

			t.Run("should use the specified delay", func(t *testing.T) {
				t.Parallel()

				delay := getDelay(newTestLogger(t), 5, time.Duration(0))
				assert.Equal(t, 5*time.Second, delay)
			})
		})
		t.Run("legacy delay is defined", func(t *testing.T) {
			t.Parallel()

			//nolint:godox
			// TODO: Remove legacy delay tests when legacy notification types are removed.
			t.Run("should use the specified legacy delay", func(t *testing.T) {
				t.Parallel()

				delay := getDelay(newTestLogger(t), 0, 5*time.Second)
				assert.Equal(t, 5*time.Second, delay)
			})
		})
		t.Run("legacy delay and delay is defined", func(t *testing.T) {
			t.Parallel()

			//nolint:godox
			// TODO: Remove legacy delay tests when legacy notification types are removed.
			t.Run("should use the specified legacy delay and ignore the specified delay", func(t *testing.T) {
				t.Parallel()

				delay := getDelay(newTestLogger(t), 5, 7*time.Second)
				assert.Equal(t, 7*time.Second, delay)
			})
		})
		t.Run("notification template file is specified", func(t *testing.T) {
			t.Parallel()

			t.Run("should load template from file", func(t *testing.T) {
				t.Parallel()

				content := "{{.Data.Host}} updated"
				tmpFile, err := os.CreateTemp("", "template")
				require.NoError(t, err)

				defer os.Remove(tmpFile.Name())

				_, err = tmpFile.WriteString(content)
				require.NoError(t, err)
				tmpFile.Close()

				notifier := NewNotifier(newTestLogger(t), loadNotify(t, []string{
					"--notification-url",
					"logger://",
					"--notification-template-file",
					tmpFile.Name(),
				}))
				require.NotNil(t, notifier)
				assert.Contains(t, notifier.GetNames(), "logger")
			})
		})
	})
	//nolint:godox
	// TODO: Remove legacy slack notifier tests when legacy notification types are removed.
	t.Run("the slack notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("passing a discord url to the slack notifier", func(t *testing.T) {
			t.Parallel()

			channel := "123456789"
			token := "abvsihdbau"
			color := colorInt
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

			t.Run("should return a discord url when using a hook url with the domain discord.com", func(t *testing.T) {
				t.Parallel()

				hookURL := fmt.Sprintf(
					"https://%s/api/webhooks/%s/%s/slack",
					"discord.com",
					channel,
					token,
				)
				testURL(t, buildArgs(hookURL), expected, time.Duration(0))
			})
			t.Run("should return a discord url when using a hook url with the domain discordapp.com", func(t *testing.T) {
				t.Parallel()

				hookURL := fmt.Sprintf(
					"https://%s/api/webhooks/%s/%s/slack",
					"discordapp.com",
					channel,
					token,
				)
				testURL(t, buildArgs(hookURL), expected, time.Duration(0))
			})
			t.Run("icon URL and username are specified", func(t *testing.T) {
				t.Parallel()

				t.Run("should return the expected URL", func(t *testing.T) {
					t.Parallel()

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

					testURL(t, args, expectedOutput, expectedDelay)
				})
			})
		})
		t.Run("converting a slack service config into a shoutrrr url", func(t *testing.T) {
			t.Parallel()

			username := "containrrrbot"
			tokenA := "AAAAAAAAA"
			tokenB := "BBBBBBBBB"
			tokenC := "123456789123456789123456"
			color := url.QueryEscape(colorHex)
			iconURL := "https://github.com/nicholas-fedor/watchtower/blob/main/watchtower-sq180.png"
			iconEmoji := "whale"

			t.Run("icon URL is specified", func(t *testing.T) {
				t.Parallel()

				t.Run("should return the expected URL", func(t *testing.T) {
					t.Parallel()

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

					testURL(t, args, expectedOutput, expectedDelay)
				})
			})

			t.Run("icon emoji is specified", func(t *testing.T) {
				t.Parallel()

				t.Run("should return the expected URL", func(t *testing.T) {
					t.Parallel()

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

					testURL(t, args, expectedOutput, time.Duration(0))
				})
			})
		})
		t.Run("the hook URL is empty", func(t *testing.T) {
			t.Parallel()

			t.Run("should fatal with a clear missing argument message", func(t *testing.T) {
				t.Parallel()

				expectNewNotifierFatal(t, []string{
					"--notifications",
					"slack",
				}, "Slack hook URL is empty.")
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy gotify notifier tests when legacy notification types are removed.
	t.Run("the gotify notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("converting a gotify service config into a shoutrrr url", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the expected URL", func(t *testing.T) {
				t.Parallel()

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

				testURL(t, args, expectedOutput, time.Duration(0))
			})
		})
		t.Run("initializing a gotify notifier via NewNotifier", func(t *testing.T) {
			t.Parallel()

			t.Run("should configure with valid flags", func(t *testing.T) {
				t.Parallel()

				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					"https://gotify.example.com",
					"--notification-gotify-token",
					"Atesttoken12345",
					"--notification-gotify-tls-skip-verify",
				}
				notifier := NewNotifier(newTestLogger(t), loadNotify(t, args))
				names := notifier.GetNames()
				assert.Contains(t, names, "gotify")

				urls := notifier.GetURLs()
				assert.True(
					t,
					slices.ContainsFunc(urls, func(u string) bool { return strings.Contains(u, "gotify://gotify.example.com/Atesttoken12345") }),
					"no URL contains %q: %v",
					"gotify://gotify.example.com/Atesttoken12345",
					urls,
				)
			})

			t.Run("should log the token length, not the token, at trace level", func(t *testing.T) {
				t.Parallel()

				args := []string{
					"--notifications",
					"gotify",
					"--notification-gotify-url",
					"https://gotify.example.com",
					"--notification-gotify-token",
					"Atesttoken12345",
				}

				var buf lockedBuffer

				log := zerolog.New(&buf).Level(zerolog.TraceLevel)

				notifier := NewNotifier(&log, loadNotify(t, args))
				assert.Contains(t, notifier.GetNames(), "gotify")

				out := buf.String()
				assert.Contains(t, out, `"token_length":15,"message":"Gotify notifier token loaded"`)
				assert.NotContains(t, out, "Atesttoken12345")
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy msteams notifier tests when legacy notification types are removed.
	t.Run("the teams notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("converting a teams service config into a shoutrrr url", func(t *testing.T) {
			t.Parallel()

			t.Run("should return the expected URL", func(t *testing.T) {
				t.Parallel()

				color := url.QueryEscape(colorHex)

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

				testURL(t, args, expectedOutput, time.Duration(0))
			})
		})
	})

	//nolint:godox
	// TODO: Remove legacy email notifier tests when legacy notification types are removed.
	t.Run("the email notifier", func(t *testing.T) {
		t.Parallel()

		t.Run("converting an email service config into a shoutrrr url", func(t *testing.T) {
			t.Parallel()

			t.Run("should set the from address in the URL", func(t *testing.T) {
				t.Parallel()

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
				testURL(t, args, expectedOutput, expectedDelay)
			})

			t.Run("should return the expected URL", func(t *testing.T) {
				t.Parallel()

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

				testURL(t, args, expectedOutput, expectedDelay)
			})
		})
		t.Run("a required field is empty", func(t *testing.T) {
			t.Parallel()

			t.Run("should fatal when the from address is empty", func(t *testing.T) {
				t.Parallel()

				expectNewNotifierFatal(t, []string{
					"--notifications",
					"email",
					"--notification-email-to",
					"to@example.com",
					"--notification-email-server",
					"mail.example.com",
				}, "Email from address is empty.")
			})

			t.Run("should fatal when the to address is empty", func(t *testing.T) {
				t.Parallel()

				expectNewNotifierFatal(t, []string{
					"--notifications",
					"email",
					"--notification-email-from",
					"from@example.com",
					"--notification-email-server",
					"mail.example.com",
				}, "Email to address is empty.")
			})

			t.Run("should fatal when the server is empty", func(t *testing.T) {
				t.Parallel()

				expectNewNotifierFatal(t, []string{
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
}

// expectNewNotifierFatal runs NewNotifier under a temporary FatalExitFunc
// that panics instead of os.Exit, captures buffer-backed fatal log output, and
// restores the previous exit callback.
//
// Parameters:
//   - t: Test that receives the failures.
//   - args: CLI flags to load before constructing the notifier
//   - wantMsg: exact fatal message text expected in the log output
func expectNewNotifierFatal(t *testing.T, args []string, wantMsg string) {
	t.Helper()

	fatalExitMu.Lock()
	defer fatalExitMu.Unlock()

	oldFatalExit := zerolog.FatalExitFunc
	defer func() { zerolog.FatalExitFunc = oldFatalExit }()

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.FatalLevel)

	zerolog.FatalExitFunc = func() {
		panic("fatal exit")
	}

	cfg := loadNotify(t, args)

	assert.PanicsWithValue(t, "fatal exit", func() {
		NewNotifier(&log, cfg)
	})

	out := buf.String()
	assert.Contains(t, out, `"message":"`+wantMsg+`"`, "expected exact fatal message %q in output:\n%s", wantMsg, out)
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
func testURL(t *testing.T, args []string, expectedURL string, expectedDelay time.Duration) {
	t.Helper()

	cfg := loadNotify(t, args)

	urls, legacyDelay := appendLegacyURLs(newTestLogger(t), []string{}, cfg.LegacyTypes, cfg.Legacy)
	delay := getDelay(newTestLogger(t), cfg.DelaySeconds, legacyDelay)

	assert.Contains(t, urls, expectedURL)
	assert.Equal(t, expectedDelay, delay)
}

// loadNotify loads the notification settings from args the way production
// does, by parsing them into a command with every flag registered and running
// config.Load.
//
// Parameters:
//   - t: Test that receives the failures.
//   - args: CLI flags to parse.
//
// Returns:
//   - notifyConfig.Notify: The resolved notification settings.
func loadNotify(t *testing.T, args []string) notifyConfig.Notify {
	t.Helper()

	command := &cobra.Command{Use: "watchtower"}
	flags.RegisterAll(command)
	require.NoError(t, command.ParseFlags(args))

	cfg, err := config.Load(newTestLogger(t), command, nil)
	require.NoError(t, err)

	return cfg.Notify
}

// templateDataFromFlags returns the static notification data for the settings loaded
// from args.
//
// Parameters:
//   - t: Test that receives the failures.
//   - args: CLI flags to parse.
//
// Returns:
//   - StaticData: Host and title for notification templates.
func templateDataFromFlags(t *testing.T, args []string) StaticData {
	t.Helper()

	cfg := loadNotify(t, args)

	return templateData(newTestLogger(t), cfg.Hostname, cfg.TitleTag, cfg.EmailSubjectTag, cfg.SkipTitle)
}
