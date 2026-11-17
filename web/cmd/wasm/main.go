//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

func main() {
	c := make(chan struct{}, 0)
	fmt.Println("QuickNetScan WASM frontend initialized")

	js.Global().Set("wasmFormatRTT", js.FuncOf(formatRTT))
	js.Global().Set("wasmFilterHosts", js.FuncOf(filterHosts))

	<-c
}

func formatRTT(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return "0 ms"
	}
	rttMs := args[0].Float()
	if rttMs < 1.0 {
		return fmt.Sprintf("%.2f ms", rttMs)
	}
	return fmt.Sprintf("%.1f ms", rttMs)
}

func filterHosts(this js.Value, args []js.Value) interface{} {
	// Utility hook for WASM-side filtering
	return true
}
