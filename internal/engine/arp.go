package engine

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

var (
	modIphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	procSendARP = modIphlpapi.NewProc("SendARP")
)

// SendARPWindows resolves the MAC address of the target IP on Windows using SendARP.
func SendARPWindows(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return "", fmt.Errorf("invalid IPv4: %s", ipStr)
	}

	destIP := *(*uint32)(unsafe.Pointer(&ip[0]))
	macAddr := make([]byte, 6)
	macLen := uint32(len(macAddr))

	ret, _, err := procSendARP.Call(
		uintptr(destIP),
		0,
		uintptr(unsafe.Pointer(&macAddr[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)

	if ret != 0 {
		return "", fmt.Errorf("SendARP failed with error code: %d (%v)", ret, err)
	}

	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		macAddr[0], macAddr[1], macAddr[2], macAddr[3], macAddr[4], macAddr[5]), nil
}
