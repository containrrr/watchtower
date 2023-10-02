//go:build !wasm

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/containrrr/watchtower/internal/meta"
	"github.com/containrrr/watchtower/pkg/notifications/preview"
	"github.com/containrrr/watchtower/pkg/notifications/preview/data"
)

func main() {
	fmt.Fprintf(os.Stderr, "watchtower/tplprev %v\n\n", meta.Version)

	var states string
	var entries string

	flag.StringVar(&states, "states", "cccuuueeekkktttfff", "sCanned, Updated, failEd, sKipped, sTale, Fresh")
	flag.StringVar(&entries, "entries", "ewwiiidddd", "Fatal,Error,Warn,Info,Debug,Trace")

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Missing required argument TEMPLATE")
		flag.Usage()
		os.Exit(1)
		return
	}

	input, err := os.ReadFile(flag.Arg(0))
	if err != nil {

		fmt.Fprintf(os.Stderr, "Failed to read template file %q: %v\n", flag.Arg(0), err)
		os.Exit(1)
		return
	}

	result, err := preview.Render(string(input), data.StatesFromString(states), data.LevelsFromString(entries))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read template file %q: %v\n", flag.Arg(0), err)
		os.Exit(1)
		return
	}

	fmt.Println(result)
}
