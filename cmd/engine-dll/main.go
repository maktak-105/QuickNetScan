package main

/*
#include <stdlib.h>
typedef void (*ProgressCallback)(const char* jsonUpdate);
static void callCallback(ProgressCallback cb, const char* jsonUpdate) {
    if (cb != NULL) {
        cb(jsonUpdate);
    }
}
*/
import "C"
import (
	"context"
	"encoding/json"
	"quicknetscan/internal/engine"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	scanCancel    context.CancelFunc
	scanMu        sync.Mutex
	cachedSubnets []byte
	subnetsOnce   sync.Once
)

func loadSubnetsBytes() []byte {
	subnetsOnce.Do(func() {
		subnets := engine.GetLocalSubnets()
		data, err := json.Marshal(subnets)
		if err != nil {
			cachedSubnets = []byte("[]")
		} else {
			cachedSubnets = data
		}
	})
	return cachedSubnets
}

func init() {
	go loadSubnetsBytes()
}

//export GetSubnetsJSON
func GetSubnetsJSON() *C.char {
	data := loadSubnetsBytes()
	return C.CString(string(data))
}

//export StartScanEngine
func StartScanEngine(cCidr *C.char, cPorts *C.char, concurrency C.int, scanPorts C.int, cb C.ProgressCallback) C.int {
	cidrStr := C.GoString(cCidr)
	portsStr := C.GoString(cPorts)

	ips, err := engine.GenerateIPsFromCIDR(cidrStr)
	if err != nil || len(ips) == 0 {
		return -1
	}

	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	scanCancel = cancel
	scanMu.Unlock()

	var portList []int
	if scanPorts != 0 && portsStr != "" {
		for _, pStr := range strings.Split(portsStr, ",") {
			p, err := strconv.Atoi(strings.TrimSpace(pStr))
			if err == nil && p > 0 && p <= 65535 {
				portList = append(portList, p)
			}
		}
	}

	opts := engine.ScanOptions{
		SubnetCIDR:  cidrStr,
		Ports:       portList,
		Concurrency: int(concurrency),
		ScanPorts:   scanPorts != 0,
		Timeout:     500 * time.Millisecond,
		PingTimeout: 300 * time.Millisecond,
	}

	scanner := engine.NewScanner(opts)
	bufSize := len(ips) + 32
	if bufSize > 4096 {
		bufSize = 4096
	}
	progressChan := make(chan engine.ProgressUpdate, bufSize)

	go func() {
		defer close(progressChan)
		scanner.ScanRange(ctx, ips, progressChan)
	}()

	go func() {
		for update := range progressChan {
			if cb != nil {
				data, err := json.Marshal(update)
				if err == nil {
					cStr := C.CString(string(data))
					C.callCallback(cb, cStr)
					C.free(unsafe.Pointer(cStr))
				}
			}
		}
	}()

	return C.int(len(ips))
}

//export StopScanEngine
func StopScanEngine() {
	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
		scanCancel = nil
	}
	scanMu.Unlock()
}

//export FreeCString
func FreeCString(str *C.char) {
	C.free(unsafe.Pointer(str))
}

func main() {}
