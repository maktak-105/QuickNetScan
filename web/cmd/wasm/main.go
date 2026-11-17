//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

var document js.Value

func main() {
	c := make(chan struct{}, 0)
	document = js.Global().Get("document")

	fmt.Println("QuickNetScan: Go WASM GUI Engine Initialized")

	// Register Go functions to JavaScript global scope
	js.Global().Set("wasmInitGUI", js.FuncOf(initGUI))
	js.Global().Set("wasmAddHostRow", js.FuncOf(addHostRow))
	js.Global().Set("wasmUpdateProgress", js.FuncOf(updateProgress))
	js.Global().Set("wasmFormatRTT", js.FuncOf(formatRTT))

	// Signal to JS that WASM is ready
	js.Global().Call("onWasmReady")

	<-c
}

func initGUI(this js.Value, args []js.Value) interface{} {
	fmt.Println("Go WASM: GUI Initialized")
	return true
}

func formatRTT(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return "0.0 ms"
	}
	rttMs := args[0].Float()
	if rttMs < 1.0 {
		return fmt.Sprintf("%.2f ms", rttMs)
	}
	return fmt.Sprintf("%.1f ms", rttMs)
}

func addHostRow(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return nil
	}
	hostObj := args[0]
	ip := hostObj.Get("ip").String()
	mac := hostObj.Get("mac").String()
	rttMs := hostObj.Get("rtt_ms").Float()

	tbody := document.Call("getElementById", "resultsBody")
	if tbody.IsNull() || tbody.IsUndefined() {
		return nil
	}

	tr := document.Call("createElement", "tr")

	// Status cell
	tdStatus := document.Call("createElement", "td")
	spanStatus := document.Call("createElement", "span")
	spanStatus.Set("className", "status-badge")
	spanStatus.Set("textContent", "ONLINE")
	tdStatus.Call("appendChild", spanStatus)
	tr.Call("appendChild", tdStatus)

	// IP cell
	tdIP := document.Call("createElement", "td")
	strongIP := document.Call("createElement", "strong")
	strongIP.Set("textContent", ip)
	tdIP.Call("appendChild", strongIP)
	tr.Call("appendChild", tdIP)

	// MAC cell
	tdMAC := document.Call("createElement", "td")
	codeMAC := document.Call("createElement", "code")
	if mac == "" {
		mac = "-"
	}
	codeMAC.Set("textContent", mac)
	tdMAC.Call("appendChild", codeMAC)
	tr.Call("appendChild", tdMAC)

	// Ports cell
	tdPorts := document.Call("createElement", "td")
	portsVal := hostObj.Get("open_ports")
	if !portsVal.IsNull() && !portsVal.IsUndefined() && portsVal.Length() > 0 {
		for i := 0; i < portsVal.Length(); i++ {
			p := portsVal.Index(i).Int()
			spanP := document.Call("createElement", "span")
			spanP.Set("className", "port-tag")
			spanP.Set("textContent", fmt.Sprintf("%d", p))
			tdPorts.Call("appendChild", spanP)
		}
	} else {
		spanNone := document.Call("createElement", "span")
		spanNone.Set("textContent", "-")
		spanNone.Get("style").Set("color", "#666")
		tdPorts.Call("appendChild", spanNone)
	}
	tr.Call("appendChild", tdPorts)

	// Latency cell
	tdRTT := document.Call("createElement", "td")
	tdRTT.Set("textContent", fmt.Sprintf("%.1f ms", rttMs))
	tr.Call("appendChild", tdRTT)

	tbody.Call("appendChild", tr)
	return true
}

func updateProgress(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return nil
	}
	scanned := args[0].Int()
	total := args[1].Int()

	progressBar := document.Call("getElementById", "progressBar")
	progressCount := document.Call("getElementById", "progressCount")

	if total > 0 {
		pct := int(float64(scanned) / float64(total) * 100.0)
		if pct > 100 {
			pct = 100
		}
		if !progressBar.IsNull() {
			progressBar.Get("style").Set("width", fmt.Sprintf("%d%%", pct))
		}
		if !progressCount.IsNull() {
			progressCount.Set("textContent", fmt.Sprintf("%d / %d", scanned, total))
		}
	}
	return true
}
