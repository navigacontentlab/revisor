//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
	"time"

	doc "github.com/navigacontentlab/navigadoc/doc"
	"github.com/navigacontentlab/revisor"
)

var (
	buf                []byte
	validator          *revisor.Validator
	uint8Array         js.Value
	promiseConstructor js.Value
	errorConstructor   js.Value
)

func main() {
	buf = make([]byte, 10*1024)
	uint8Array = js.Global().Get("Uint8Array")
	promiseConstructor = js.Global().Get("Promise")
	errorConstructor = js.Global().Get("Error")

	js.Global().Set("revisor_LoadConstraints", js.FuncOf(loadConstraints))
	js.Global().Set("revisor_ValidateDocument", js.FuncOf(validateDocument))

	for {
		time.Sleep(1 * time.Second)
	}
}

func copyAllBytes(v js.Value) int {
	bufSize := len(buf)
	size := v.Get("byteLength").Int()

	for bufSize < size {
		bufSize *= 2

		if bufSize >= size {
			buf = make([]byte, bufSize)
			break
		}
	}

	return js.CopyBytesToGo(buf, v)
}

func promise(fn func() (any, error)) js.Value {
	handler := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]

		defer func() {
			v := recover()
			if v != nil {
				reject.Invoke(errorConstructor.New(
					fmt.Sprintf("panic: %v", v),
				))
			}
		}()

		data, err := fn()
		if err != nil {
			reject.Invoke(errorConstructor.New(err.Error()))

			return nil
		}

		resolve.Invoke(data)

		return nil
	})

	return promiseConstructor.New(handler)
}

func loadConstraints(_ js.Value, args []js.Value) any {
	return promise(func() (any, error) {
		var sets []revisor.ConstraintSet

		for i, dataValue := range args {
			println(i)
			if !dataValue.InstanceOf(uint8Array) {
				return nil, fmt.Errorf(
					"constraint set %d is not an Uint8Array", i)
			}

			n := copyAllBytes(dataValue)
			data := buf[0:n]

			var set revisor.ConstraintSet

			err := json.Unmarshal(data, &set)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid constraint set %d: %w", i, err)
			}

			sets = append(sets, set)
		}

		v, err := revisor.NewValidator(sets...)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create validator: %w", err)
		}

		validator = v

		return js.Null(), nil
	})
}

func validateDocument(_ js.Value, args []js.Value) any {
	return promise(func() (any, error) {
		if validator == nil {
			return nil, errors.New("load constraints first")
		}

		if len(args) == 0 {
			return nil, errors.New("no document passed")
		}

		if !args[0].InstanceOf(uint8Array) {
			return nil, errors.New(
				"the document is not an Uint8Array")
		}

		n := copyAllBytes(args[0])
		data := buf[0:n]

		var d doc.Document

		err := json.Unmarshal(data, &d)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid document: %w", err)
		}

		result := validator.ValidateDocument(&d)

		returnData, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to marshal result data: %w", err)
		}

		v := js.Global().Get("Uint8Array").New(len(returnData))
		_ = js.CopyBytesToJS(v, returnData)

		return v, nil
	})
}
