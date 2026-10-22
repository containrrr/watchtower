//go:build !wasm

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nicholas-fedor/tplprev/internal/metadata"
	"github.com/nicholas-fedor/tplprev/internal/preview"
	"github.com/nicholas-fedor/tplprev/internal/templates"
)

func main() {
	fmt.Fprintf(os.Stderr, "tplprev %s\n\n", metadata.String())

	var states string

	var entries string

	flag.StringVar(
		&states,
		"states",
		"cccuuueeekkktttfff",
		"sCanned, Updated, failEd, sKipped, restaRted, sTale, Fresh",
	)
	flag.StringVar(&entries, "entries", "ewwiiidddd", "Panic,Fatal,Error,Warn,Info,Debug,Trace")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tplprev [flags] TEMPLATE\n\n")
		fmt.Fprintf(os.Stderr, "TEMPLATE is a file path or a builtin name:\n")

		for _, name := range templates.Names() {
			fmt.Fprintf(os.Stderr, "  %s\n", name)
		}

		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Missing required argument TEMPLATE")
		flag.Usage()
		os.Exit(1)

		return
	}

	input, err := resolveTemplate(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read template %q: %v\n", flag.Arg(0), err)
		os.Exit(1)

		return
	}

	result, err := preview.Render(
		input,
		preview.StatesFromString(states),
		preview.LevelsFromString(entries),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render template %q: %v\n", flag.Arg(0), err)
		os.Exit(1)

		return
	}

	//nolint:forbidigo // fmt.Println is appropriate for tplprev output.
	fmt.Println(result)
}

// resolveTemplate returns builtin template source or the contents of a file.
//
// Parameters:
//   - arg: Builtin template name or file path.
//
// Returns:
//   - string: Template source.
//   - error: Non-nil if the file cannot be read.
func resolveTemplate(arg string) (string, error) {
	if tpl, found := templates.Lookup(arg); found {
		return tpl, nil
	}

	contents, err := os.ReadFile(arg)
	if err != nil {
		return "", fmt.Errorf("read template file: %w", err)
	}

	return string(contents), nil
}
