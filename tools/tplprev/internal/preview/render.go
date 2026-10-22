package preview

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/nicholas-fedor/tplprev/internal/notify"
	"github.com/nicholas-fedor/tplprev/internal/templates"
)

// Render generates a preview string from a template, states, and log levels.
//
// When states is non-empty, the template executes against a notify.Data
// value (report mode). When states is empty, it executes against the Entries
// slice (legacy mode).
//
// Parameters:
//   - input: Template string to render.
//   - states: List of container states to include.
//   - logLevels: List of log levels to include.
//
// Returns:
//   - string: Rendered preview string.
//   - error: Non-nil if parsing or execution fails, nil on success.
func Render(input string, states []State, logLevels []LogLevel) (string, error) {
	generator := New()

	for _, state := range states {
		generator.AddFromState(state)
	}

	for _, level := range logLevels {
		generator.AddLogEntry(level)
	}

	return Execute(input, generator.NotificationData(), len(states) > 0)
}

// Execute renders a template against notification data.
//
// When reportMode is true, the template executes against payload. When it is
// false, the template first executes against payload.Entries. If that fails
// because the template expects a Data root (for example .Report), execution
// is retried against payload with a nil Report.
//
// Parameters:
//   - input: Template string to render.
//   - payload: Notification data to execute against.
//   - reportMode: When true, execute against payload. When false, prefer payload.Entries.
//
// Returns:
//   - string: Rendered preview string.
//   - error: Non-nil if parsing or execution fails, nil on success.
func Execute(input string, payload notify.Data, reportMode bool) (string, error) {
	tpl, err := template.New("").Funcs(templates.Funcs).Parse(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	root := any(payload)
	if !reportMode {
		root = payload.Entries
	}

	var buf strings.Builder

	err = tpl.Execute(&buf, root)
	if err != nil && !reportMode {
		buf.Reset()

		err = tpl.Execute(&buf, payload)
	}

	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
