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
	"fmt"
	"net"
	"quicknetscan/internal/engine"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	scanCancel context.CancelFunc
	scanMu     sync.Mutex
)

type SubnetInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default"`
	Priority  int    `json:"priority"`
}

//export GetSubnetsJSON
func GetSubnetsJSON() *C.char {
	ifaces, err := net.Interfaces()
	if err != nil {
		return C.CString("[]")
	}

	var subnets []SubnetInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}

			ip := ipNet.IP.To4()
			ones, _ := ipNet.Mask.Size()
			networkIP := ip.Mask(ipNet.Mask)
			cidr := fmt.Sprintf("%s/%d", networkIP.String(), ones)

			priority := 50
			nameLower := strings.ToLower(iface.Name)

			if ip[0] == 169 && ip[1] == 254 {
				priority = 5
			} else if ones == 32 {
				priority = 10
			} else if strings.Contains(nameLower, "tailscale") || strings.Contains(nameLower, "vpn") {
				priority = 20
			} else if strings.Contains(nameLower, "wsl") || strings.Contains(nameLower, "veth") || strings.Contains(nameLower, "hyper-v") {
				priority = 30
			} else if strings.Contains(nameLower, "wi-fi") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "イーサネット") || strings.Contains(nameLower, "ethernet") {
				priority = 100
				if ones < 24 {
					c24 := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
					subnets = append(subnets, SubnetInfo{
						Name:     iface.Name + " (推奨 /24)",
						IP:       ip.String(),
						CIDR:     c24,
						Label:    fmt.Sprintf("%s 推奨 (%s)", iface.Name, c24),
						Priority: 110,
					})
				}
			} else if ip[0] == 192 && ip[1] == 168 {
				priority = 90
			} else if ip[0] == 172 && (ip[1] >= 16 && ip[1] <= 31) {
				priority = 85
			} else if ip[0] == 10 {
				priority = 80
			}

			label := fmt.Sprintf("%s (%s - %s)", iface.Name, cidr, ip.String())
			subnets = append(subnets, SubnetInfo{
				Name:     iface.Name,
				IP:       ip.String(),
				CIDR:     cidr,
				Label:    label,
				Priority: priority,
			})
		}
	}

	for i := 0; i < len(subnets)-1; i++ {
		for j := i + 1; j < len(subnets); j++ {
			if subnets[j].Priority > subnets[i].Priority {
				subnets[i], subnets[j] = subnets[j], subnets[i]
			}
		}
	}

	if len(subnets) > 0 {
		subnets[0].IsDefault = true
	}

	data, _ := json.Marshal(subnets)
	return C.CString(string(data))
}

//export StartScanEngine
func StartScanEngine(cCidr *C.char, cPorts *C.char, concurrency C.int, scanPorts C.int, cb C.ProgressCallback) C.int {
	cidrStr := C.GoString(cCidr)
	portsStr := C.GoString(cPorts)

	ips, err := engine.GenerateIPsFromCIDR(cidrStr)
	if err != nil {
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
		Timeout:     1000 * time.Millisecond,
		PingTimeout: 400 * time.Millisecond,
	}

	scanner := engine.NewScanner(opts)
	progressChan := make(chan engine.ProgressUpdate, 100)

	go func() {
		defer close(progressChan)
		scanner.ScanRange(ctx, ips, progressChan)
	}()

	go func() {
		for update := range progressChan {
			if cb != nil {
				data, _ := json.Marshal(update)
				cStr := C.CString(string(data))
				C.callCallback(cb, cStr)
				C.free(unsafe.Pointer(cStr))
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
