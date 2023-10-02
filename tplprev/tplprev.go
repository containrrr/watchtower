package main

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/containrrr/watchtower/pkg/notifications/templates"
)

func TplPrev(input string, states []State, loglevels []LogLevel) (string, error) {

	rb := ReportBuilder()

	tpl, err := template.New("").Funcs(templates.Funcs).Parse(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %e", err)
	}

	for _, state := range states {
		rb.AddFromState(state)
	}

	var entries []*LogEntry
	for _, level := range loglevels {
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
		return "", fmt.Errorf("failed to execute template: %e", err)
	}

	return buf.String(), nil
}
