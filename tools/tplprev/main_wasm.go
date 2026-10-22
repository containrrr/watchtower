//go:build wasm

package main

import (
	"errors"
	"fmt"
	"syscall/js"

	"github.com/nicholas-fedor/tplprev/internal/metadata"
	"github.com/nicholas-fedor/tplprev/internal/preview"
)

// errInvalidJSArg is returned when a WASM argument is not a string or collection.
var errInvalidJSArg = errors.New("invalid states or levels argument")

func main() {
	fmt.Println("tplprev " + metadata.String())

	js.Global().Set("WATCHTOWER", js.ValueOf(map[string]any{
		"tplprev": js.FuncOf(jsTplPrev),
	}))
	<-make(chan bool)
}

func jsTplPrev(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return "Requires 3 arguments passed"
	}

	input := args[0].String()

	states, err := statesFromJS(args[1])
	if err != nil {
		return "Error: " + err.Error()
	}

	levels, err := levelsFromJS(args[2])
	if err != nil {
		return "Error: " + err.Error()
	}

	result, err := preview.Render(input, states, levels)
	if err != nil {
		return "Error: " + err.Error()
	}

	return result
}

func statesFromJS(arg js.Value) ([]preview.State, error) {
	isArray, isTypedArray := jsCollectionFlags(arg)
	switch classifyJSType(arg.Type().String(), isArray, isTypedArray) {
	case jsKindString:
		return preview.StatesFromString(arg.String()), nil
	case jsKindCollection:
		states := make([]preview.State, 0, arg.Length())
		for i := range arg.Length() {
			states = append(states, preview.State(arg.Index(i).String()))
		}

		return states, nil
	default:
		return nil, errInvalidJSArg
	}
}

func levelsFromJS(arg js.Value) ([]preview.LogLevel, error) {
	isArray, isTypedArray := jsCollectionFlags(arg)
	switch classifyJSType(arg.Type().String(), isArray, isTypedArray) {
	case jsKindString:
		return preview.LevelsFromString(arg.String()), nil
	case jsKindCollection:
		levels := make([]preview.LogLevel, 0, arg.Length())
		for i := range arg.Length() {
			levels = append(levels, preview.LogLevel(arg.Index(i).String()))
		}

		return levels, nil
	default:
		return nil, errInvalidJSArg
	}
}

func jsCollectionFlags(arg js.Value) (isArray, isTypedArray bool) {
	if arg.Type() != js.TypeObject {
		return false, false
	}

	array := js.Global().Get("Array")
	if array.Truthy() {
		isArray = array.Call("isArray", arg).Bool()
	}

	arrayBuffer := js.Global().Get("ArrayBuffer")
	if arrayBuffer.Truthy() {
		isView := arrayBuffer.Get("isView")
		if isView.Truthy() {
			isTypedArray = arrayBuffer.Call("isView", arg).Bool()
		}
	}

	return isArray, isTypedArray
}
