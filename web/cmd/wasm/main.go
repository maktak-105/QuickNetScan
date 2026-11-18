//go:build js && wasm

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"syscall/js"
)

var document js.Value

func main() {
	c := make(chan struct{})
	document = js.Global().Get("document")

	fmt.Println("QuickNetScan: Go WASM GUI Engine Initialized")

	// Register Go functions to JavaScript global scope
	js.Global().Set("wasmInitGUI", js.FuncOf(initGUI))
	js.Global().Set("wasmAddHostRow", js.FuncOf(addHostRow))
	js.Global().Set("wasmUpdateProgress", js.FuncOf(updateProgress))
	js.Global().Set("wasmFormatRTT", js.FuncOf(formatRTT))

	// Signal to JS that WASM is ready
	if !js.Global().Get("onWasmReady").IsUndefined() {
		js.Global().Call("onWasmReady")
	}

	<-c
}

func initGUI(this js.Value, args []js.Value) interface{} {
	return true
}

func formatRTTString(rttMs float64) string {
	if rttMs < 1.0 {
		return fmt.Sprintf("%.2f ms", rttMs)
	}
	return fmt.Sprintf("%.1f ms", rttMs)
}

func formatRTT(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return "0.0 ms"
	}
	return formatRTTString(args[0].Float())
}

func ipToUint32(ipStr string) uint32 {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
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

	ipNum := ipToUint32(ip)
	tr := document.Call("createElement", "tr")
	tr.Call("setAttribute", "data-ip", ip)
	tr.Call("setAttribute", "data-ip-num", strconv.FormatUint(uint64(ipNum), 10))

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
	tdRTT.Set("textContent", formatRTTString(rttMs))
	tr.Call("appendChild", tdRTT)

	// Insert sorted by IPv4 numerical order
	children := tbody.Get("children")
	length := children.Get("length").Int()
	inserted := false
	for i := 0; i < length; i++ {
		child := children.Call("item", i)
		existingIP := child.Call("getAttribute", "data-ip")
		if !existingIP.IsNull() && existingIP.String() == ip {
			tbody.Call("replaceChild", tr, child)
			inserted = true
			break
		}
		attrVal := child.Call("getAttribute", "data-ip-num")
		if !attrVal.IsNull() && attrVal.String() != "" {
			otherNum, err := strconv.ParseUint(attrVal.String(), 10, 32)
			if err == nil && uint32(otherNum) > ipNum {
				tbody.Call("insertBefore", tr, child)
				inserted = true
				break
			}
		}
	}
	if !inserted {
		tbody.Call("appendChild", tr)
	}
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
		if !progressBar.IsNull() && !progressBar.IsUndefined() {
			progressBar.Get("style").Set("width", fmt.Sprintf("%d%%", pct))
		}
		if !progressCount.IsNull() && !progressCount.IsUndefined() {
			progressCount.Set("textContent", fmt.Sprintf("%d / %d", scanned, total))
		}
	}
	return true
}
