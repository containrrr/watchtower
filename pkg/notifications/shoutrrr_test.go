package notifications

import (
	"time"

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

var allButTrace = logrus.AllLevels[0:logrus.TraceLevel]

var legacyMockData = Data{
	Entries: []*logrus.Entry{
		{
			Level:   logrus.InfoLevel,
			Message: "foo Bar",
		},
	},
}

var mockDataMultipleEntries = Data{
	Entries: []*logrus.Entry{
		{
			Level:   logrus.InfoLevel,
			Message: "The situation is under control",
		},
		{
			Level:   logrus.WarnLevel,
			Message: "All the smoke might be covering up some problems",
		},
		{
			Level:   logrus.ErrorLevel,
			Message: "Turns out everything is on fire",
		},
	},
}

var mockDataAllFresh = Data{
	Entries: []*logrus.Entry{},
	Report:  mocks.CreateMockProgressReport(s.FreshState),
}

func mockDataFromStates(states ...s.State) Data {
	hostname := "Mock"
	prefix := ""
	return Data{
		Entries: legacyMockData.Entries,
		Report:  mocks.CreateMockProgressReport(states...),
		StaticData: StaticData{
			Title: GetTitle(hostname, prefix),
			Host:  hostname,
		},
	}
}

var _ = Describe("Shoutrrr", func() {
	var logBuffer *gbytes.Buffer

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		logrus.SetOutput(logBuffer)
		logrus.SetLevel(logrus.TraceLevel)
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

				shoutrrr := createNotifier([]string{}, logrus.AllLevels, "", true, StaticData{})

				entries := []*logrus.Entry{
					{
						Message: "foo bar",
					},
				}

				s, err := shoutrrr.buildMessage(Data{Entries: entries})
				Expect(err).NotTo(HaveOccurred())

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

				s, err := shoutrrr.buildMessage(Data{Entries: entries})
				Expect(err).NotTo(HaveOccurred())

				Expect(s).To(Equal("info: foo bar\n"))
			})
		})

		Describe("the default template", func() {
			When("all containers are fresh", func() {
				It("should return an empty string", func() {
					Expect(getTemplatedResult(``, true, mockDataAllFresh)).To(Equal(""))
				})
			})
		})

		When("given an invalid custom template", func() {
			It("should format the messages using the default template", func() {
				invNotif, err := createNotifierWithTemplate(`{{ intentionalSyntaxError`, true)
				Expect(err).To(HaveOccurred())
				invMsg, err := invNotif.buildMessage(legacyMockData)
				Expect(err).NotTo(HaveOccurred())

				defNotif, err := createNotifierWithTemplate(``, true)
				Expect(err).ToNot(HaveOccurred())
				defMsg, err := defNotif.buildMessage(legacyMockData)
				Expect(err).ToNot(HaveOccurred())

				Expect(invMsg).To(Equal(defMsg))
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
- fail1 (mock/fail1:latest): Failed: accidentally the whole container`
				data := mockDataFromStates(s.UpdatedState, s.FreshState, s.FailedState, s.SkippedState, s.UpdatedState)
				Expect(getTemplatedResult(``, false, data)).To(Equal(expected))
			})

		})

		When("using a template referencing Title", func() {
			It("should contain the title in the output", func() {
				expected := `Watchtower updates on Mock`
				data := mockDataFromStates(s.UpdatedState)
				Expect(getTemplatedResult(`{{ .Title }}`, false, data)).To(Equal(expected))
			})
		})

		When("using a template referencing Host", func() {
			It("should contain the hostname in the output", func() {
				expected := `Mock`
				data := mockDataFromStates(s.UpdatedState)
				Expect(getTemplatedResult(`{{ .Host }}`, false, data)).To(Equal(expected))
			})
		})

		Describe("the default template", func() {
			When("all containers are fresh", func() {
				It("should return an empty string", func() {
					Expect(getTemplatedResult(``, false, mockDataAllFresh)).To(Equal(""))
				})
			})
			When("at least one container was updated", func() {
				It("should send a report", func() {
					expected := `1 Scanned, 1 Updated, 0 Failed
- updt1 (mock/updt1:latest): 01d110000000 updated to d0a110000000`
					data := mockDataFromStates(s.UpdatedState)
					Expect(getTemplatedResult(``, false, data)).To(Equal(expected))
				})
			})
			When("at least one container failed to update", func() {
				It("should send a report", func() {
					expected := `1 Scanned, 0 Updated, 1 Failed
- fail1 (mock/fail1:latest): Failed: accidentally the whole container`
					data := mockDataFromStates(s.FailedState)
					Expect(getTemplatedResult(``, false, data)).To(Equal(expected))
				})
			})
			When("the report is nil", func() {
				It("should return the logged entries", func() {
					expected := `The situation is under control
All the smoke might be covering up some problems
Turns out everything is on fire
`
					Expect(getTemplatedResult(``, false, mockDataMultipleEntries)).To(Equal(expected))
				})
			})
		})
	})

	When("batching notifications", func() {
		When("no messages are queued", func() {
			It("should not send any notification", func() {
				shoutrrr := newShoutrrrNotifier("", allButTrace, true, StaticData{}, time.Duration(0), "logger://")
				shoutrrr.StartNotification()
				shoutrrr.SendNotification(nil)
				Consistently(logBuffer).ShouldNot(gbytes.Say(`Shoutrrr:`))
			})
		})
		When("at least one message is queued", func() {
			It("should send a notification", func() {
				shoutrrr := newShoutrrrNotifier("", allButTrace, true, StaticData{}, time.Duration(0), "logger://")
				shoutrrr.StartNotification()
				logrus.Info("This log message is sponsored by ContainrrrVPN")
				shoutrrr.SendNotification(nil)
				Eventually(logBuffer).Should(gbytes.Say(`Shoutrrr: This log message is sponsored by ContainrrrVPN`))
			})
		})
	})

	When("the title data field is empty", func() {
		It("should not have set the title param", func() {
			shoutrrr := createNotifier([]string{"logger://"}, allButTrace, "", true, StaticData{
				Host:  "test.host",
				Title: "",
			})
			_, found := shoutrrr.params.Title()
			Expect(found).ToNot(BeTrue())
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
		params:         &types.Params{},
	}

	entry := &logrus.Entry{
		Message: "foo bar",
	}

	go sendNotifications(shoutrrr, time.Duration(0))

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

func getTemplatedResult(tplString string, legacy bool, data Data) (msg string) {
	notifier, err := createNotifierWithTemplate(tplString, legacy)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	msg, err = notifier.buildMessage(data)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return msg
}
