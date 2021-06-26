package notifications

import (
	"github.com/containrrr/shoutrrr/pkg/types"
	"github.com/containrrr/watchtower/internal/actions/mocks"
	"github.com/containrrr/watchtower/internal/flags"
	s "github.com/containrrr/watchtower/pkg/session"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var legacyMockData = Data{
	Entries: []*logrus.Entry{
		{
			Level:   logrus.InfoLevel,
			Message: "foo Bar",
		},
	},
}

func mockDataFromStates(states ...s.State) Data {
	return Data{
		Entries: legacyMockData.Entries,
		Report:  mocks.CreateMockProgressReport(states...),
	}
}

var _ = Describe("Shoutrrr", func() {
	var logBuffer *gbytes.Buffer

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		logrus.SetOutput(logBuffer)
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors:    true,
			DisableTimestamp: true,
		})
	})

	When("using legacy templates", func() {

		When("no custom template is provided", func() {
			It("should format the messages using the default template", func() {
				cmd := new(cobra.Command)
				flags.RegisterNotificationFlags(cmd)

				shoutrrr := createNotifier([]string{}, logrus.AllLevels, "", true)

				entries := []*logrus.Entry{
					{
						Message: "foo bar",
					},
				}

				s := shoutrrr.buildMessage(Data{Entries: entries})

				Expect(s).To(Equal("foo bar\n"))
			})
		})
		When("given a valid custom template", func() {
			It("should format the messages using the custom template", func() {

				tplString := `{{range .}}{{.Level}}: {{.Message}}{{println}}{{end}}`
				tpl, err := getShoutrrrTemplate(tplString, true)
				Expect(err).ToNot(HaveOccurred())

				shoutrrr := &shoutrrrTypeNotifier{
					template:       tpl,
					legacyTemplate: true,
				}

				entries := []*logrus.Entry{
					{
						Level:   logrus.InfoLevel,
						Message: "foo bar",
					},
				}

				s := shoutrrr.buildMessage(Data{Entries: entries})

				Expect(s).To(Equal("info: foo bar\n"))
			})
		})

		When("given an invalid custom template", func() {
			It("should format the messages using the default template", func() {
				invNotif, err := createNotifierWithTemplate(`{{ intentionalSyntaxError`, true)
				Expect(err).To(HaveOccurred())

				defNotif, err := createNotifierWithTemplate(``, true)
				Expect(err).ToNot(HaveOccurred())

				Expect(invNotif.buildMessage(legacyMockData)).To(Equal(defNotif.buildMessage(legacyMockData)))
			})
		})

		When("given a template that is using ToUpper function", func() {
			It("should return the text in UPPER CASE", func() {
				tplString := `{{range .}}{{ .Message | ToUpper }}{{end}}`
				Expect(getTemplatedResult(tplString, true, legacyMockData)).To(Equal("FOO BAR"))
			})
		})

		When("given a template that is using ToLower function", func() {
			It("should return the text in lower case", func() {
				tplString := `{{range .}}{{ .Message | ToLower }}{{end}}`
				Expect(getTemplatedResult(tplString, true, legacyMockData)).To(Equal("foo bar"))
			})
		})

		When("given a template that is using Title function", func() {
			It("should return the text in Title Case", func() {
				tplString := `{{range .}}{{ .Message | Title }}{{end}}`
				Expect(getTemplatedResult(tplString, true, legacyMockData)).To(Equal("Foo Bar"))
			})
		})

	})

	When("using report templates", func() {

		When("no custom template is provided", func() {
			It("should format the messages using the default template", func() {
				expected := `4 Scanned, 2 Updated, 1 Failed
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000
- updt2 (mock/updt2:latest): 01d120000000 updated to d0a120000000
- frsh1 (mock/frsh1:latest): Fresh
- skip1 (mock/skip1:latest): Skipped: unpossible
- fail1 (mock/fail1:latest): Failed: accidentally the whole container
`
				data := mockDataFromStates(s.UpdatedState, s.FreshState, s.FailedState, s.SkippedState, s.UpdatedState)
				Expect(getTemplatedResult(``, false, data)).To(Equal(expected))
			})

			It("should format the messages using the default template", func() {
				expected := `1 Scanned, 0 Updated, 0 Failed
- frsh1 (mock/frsh1:latest): Fresh
`
				data := mockDataFromStates(s.FreshState)
				Expect(getTemplatedResult(``, false, data)).To(Equal(expected))
			})
		})
	})

	When("sending notifications", func() {

		It("SlowNotificationNotSent", func() {
			_, blockingRouter := sendNotificationsWithBlockingRouter(true)

			Eventually(blockingRouter.sent).Should(Not(Receive()))

		})

		It("SlowNotificationSent", func() {
			shoutrrr, blockingRouter := sendNotificationsWithBlockingRouter(true)

			blockingRouter.unlock <- true
			shoutrrr.Close()

			Eventually(blockingRouter.sent).Should(Receive(BeTrue()))
		})
	})
})

type blockingRouter struct {
	unlock chan bool
	sent   chan bool
}

func (b blockingRouter) Send(_ string, _ *types.Params) []error {
	_ = <-b.unlock
	b.sent <- true
	return nil
}

func sendNotificationsWithBlockingRouter(legacy bool) (*shoutrrrTypeNotifier, *blockingRouter) {

	router := &blockingRouter{
		unlock: make(chan bool, 1),
		sent:   make(chan bool, 1),
	}

	tpl, err := getShoutrrrTemplate("", legacy)
	Expect(err).NotTo(HaveOccurred())

	shoutrrr := &shoutrrrTypeNotifier{
		template:       tpl,
		messages:       make(chan string, 1),
		done:           make(chan bool),
		Router:         router,
		legacyTemplate: legacy,
	}

	entry := &logrus.Entry{
		Message: "foo bar",
	}

	go sendNotifications(shoutrrr)

	shoutrrr.StartNotification()
	_ = shoutrrr.Fire(entry)

	shoutrrr.SendNotification(nil)

	return shoutrrr, router
}

func createNotifierWithTemplate(tplString string, legacy bool) (*shoutrrrTypeNotifier, error) {
	tpl, err := getShoutrrrTemplate(tplString, legacy)

	return &shoutrrrTypeNotifier{
		template:       tpl,
		legacyTemplate: legacy,
	}, err
}

func getTemplatedResult(tplString string, legacy bool, data Data) (string, error) {
	notifier, err := createNotifierWithTemplate(tplString, legacy)
	if err != nil {
		return "", err
	}
	return notifier.buildMessage(data), err
}
