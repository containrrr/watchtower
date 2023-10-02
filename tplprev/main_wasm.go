//go:build wasm

package main

import (
	"fmt"

	"github.com/containrrr/watchtower/internal/meta"

	"syscall/js"
)

func main() {
	fmt.Println("watchtower/tplprev v" + meta.Version)

	js.Global().Set("WATCHTOWER", js.ValueOf(map[string]any{
		"tplprev": js.FuncOf(jsTplPrev),
	}))
	<-make(chan bool)

}

func jsTplPrev(this js.Value, args []js.Value) any {

	if len(args) < 3 {
		return "Requires 3 arguments passed"
	}

	input := args[0].String()

	statesArg := args[1]
	var states []State

	if statesArg.Type() == js.TypeString {
		states = StatesFromString(statesArg.String())
	} else {
		for i := 0; i < statesArg.Length(); i++ {
			state := State(statesArg.Index(i).String())
			states = append(states, state)
		}
	}

	levelsArg := args[2]
	var levels []LogLevel

	if levelsArg.Type() == js.TypeString {
		levels = LevelsFromString(statesArg.String())
	} else {
		for i := 0; i < levelsArg.Length(); i++ {
			level := LogLevel(levelsArg.Index(i).String())
			levels = append(levels, level)
		}
	}

	result, err := TplPrev(input, states, levels)
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}
