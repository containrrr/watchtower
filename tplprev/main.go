package main

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/containrrr/watchtower/internal/meta"
	"github.com/containrrr/watchtower/pkg/notifications/templates"

	"syscall/js"
)

func main() {
	fmt.Println("watchtower/tplprev v" + meta.Version)

	js.Global().Set("WATCHTOWER", js.ValueOf(map[string]any{
		"tplprev": js.FuncOf(tplprev),
	}))
	<-make(chan bool)

}

func tplprev(this js.Value, args []js.Value) any {

	rb := ReportBuilder()

	if len(args) < 2 {
		return "Requires 3 argument passed"
	}

	input := args[0].String()
	tpl, err := template.New("").Funcs(templates.Funcs).Parse(input)
	if err != nil {
		return "Failed to parse template: " + err.Error()
	}

	actionsArg := args[1]

	for i := 0; i < actionsArg.Length(); i++ {
		action := actionsArg.Index(i)
		if action.Length() != 2 {
			return fmt.Sprintf("Invalid size of action tuple, expected 2, got %v", action.Length())
		}
		count := action.Index(0).Int()
		state := State(action.Index(1).String())
		rb.AddNContainers(count, state)
	}

	entriesArg := args[2]
	var entries []*LogEntry
	for i := 0; i < entriesArg.Length(); i++ {
		count := entriesArg.Index(i).Int()
		level := ErrorLevel + LogLevel(i)
		for m := 0; m < count; m++ {
			var msg string
			if level <= WarnLevel {
				msg = rb.randomEntry(logErrors)
			} else {
				msg = rb.randomEntry(logMessages)
			}
			entries = append(entries, &LogEntry{
				Message: msg,
				Data:    map[string]any{},
				Time:    time.Now(),
				Level:   level,
			})
		}
	}

	report := rb.Build()
	data := Data{
		Entries: entries,
		StaticData: StaticData{
			Title: "Title",
			Host:  "Host",
		},
		Report: report,
	}

	var buf strings.Builder
	err = tpl.Execute(&buf, data)
	if err != nil {
		return "Failed to execute template: " + err.Error()
	}

	return buf.String()
}
