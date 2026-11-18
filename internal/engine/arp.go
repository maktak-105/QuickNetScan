package engine

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modIphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	procSendARP = modIphlpapi.NewProc("SendARP")

	localNetOnce sync.Once
	localMACMap  map[string]string
	localIPNets  []*net.IPNet
)

func initLocalNetworks() {
	localMACMap = make(map[string]string)
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := ""
		if len(iface.HardwareAddr) >= 6 {
			mac = strings.ToUpper(iface.HardwareAddr.String())
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
			ip4 := ipNet.IP.To4()
			if mac != "" {
				localMACMap[ip4.String()] = mac
			}
			if ip4[0] == 169 && ip4[1] == 254 {
				continue
			}
			localIPNets = append(localIPNets, ipNet)
		}
	}
}

// IsLocalSubnetIP checks whether the given IPv4 address belongs to a directly connected local subnet.
func IsLocalSubnetIP(ip net.IP) bool {
	localNetOnce.Do(initLocalNetworks)
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	for _, ipNet := range localIPNets {
		if ipNet.Contains(ip4) {
			return true
		}
	}
	return false
}

// SendARPWindows resolves the MAC address of the target IP on Windows using SendARP.
func SendARPWindows(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return "", fmt.Errorf("invalid IPv4: %s", ipStr)
	}

	localNetOnce.Do(initLocalNetworks)
	if mac, ok := localMACMap[ip.String()]; ok && mac != "" {
		return mac, nil
	}

	// Do not send ARP for off-link (routed) IPs to avoid Proxy-ARP false positives from routers.
	if !IsLocalSubnetIP(ip) {
		return "", fmt.Errorf("off-link IP: %s", ipStr)
	}

	destIP := *(*uint32)(unsafe.Pointer(&ip[0]))
	// Win32 SendARP requires at least 2 ULONGs (8 bytes) for pMacAddr buffer.
	var macAddr [8]byte
	macLen := uint32(6)

	ret, _, err := procSendARP.Call(
		uintptr(destIP),
		0,
		uintptr(unsafe.Pointer(&macAddr[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)

	if ret != 0 {
		return "", fmt.Errorf("SendARP failed with error code: %d (%v)", ret, err)
	}
	if macLen < 6 {
		return "", fmt.Errorf("SendARP returned short MAC length: %d", macLen)
	}

	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		macAddr[0], macAddr[1], macAddr[2], macAddr[3], macAddr[4], macAddr[5]), nil
}
