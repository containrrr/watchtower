package preview

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/containrrr/watchtower/pkg/notifications/preview/data"
	"github.com/containrrr/watchtower/pkg/notifications/templates"
)

func Render(input string, states []data.State, loglevels []data.LogLevel) (string, error) {

	data := data.New()

	tpl, err := template.New("").Funcs(templates.Funcs).Parse(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse %v", err)
	}

	for _, state := range states {
		data.AddFromState(state)
	}

	for _, level := range loglevels {
		data.AddLogEntry(level)
	}

	var buf strings.Builder
	err = tpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	return buf.String(), nil
}
