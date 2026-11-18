package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Scanner coordinates concurrent network scanning.
type Scanner struct {
	Options ScanOptions
}

// NewScanner creates a new scanner instance.
func NewScanner(opts ScanOptions) *Scanner {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 256
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 500 * time.Millisecond
	}
	if opts.PingTimeout <= 0 {
		opts.PingTimeout = 300 * time.Millisecond
	}
	return &Scanner{Options: opts}
}

// ScanRange concurrently scans the provided list of IP strings.
func (s *Scanner) ScanRange(ctx context.Context, ipList []string, progressChan chan<- ProgressUpdate) []*HostResult {
	total := len(ipList)
	if total == 0 {
		if progressChan != nil {
			progressChan <- ProgressUpdate{IsComplete: true}
		}
		return nil
	}

	sem := make(chan struct{}, s.Options.Concurrency)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex
	var results []*HostResult

	var scannedCount int64
	var foundCount int64

scanLoop:
	for _, ip := range ipList {
		select {
		case <-ctx.Done():
			break scanLoop
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			res := s.probeHost(ctx, targetIP)
			if ctx.Err() != nil {
				return
			}

			curScanned := int(atomic.AddInt64(&scannedCount, 1))
			var curFound int
			if res != nil && res.IsAlive {
				curFound = int(atomic.AddInt64(&foundCount, 1))
				resultsMu.Lock()
				results = append(results, res)
				resultsMu.Unlock()
			} else {
				curFound = int(atomic.LoadInt64(&foundCount))
			}

			if progressChan != nil {
				update := ProgressUpdate{
					ScannedCount: curScanned,
					TotalTargets: total,
					FoundCount:   curFound,
					LatestHost:   res,
					IsComplete:   false,
				}
				if res != nil && res.IsAlive {
					// Never drop discovered hosts
					select {
					case progressChan <- update:
					case <-ctx.Done():
					}
				} else {
					// Non-blocking progress tick for non-responding IPs
					select {
					case progressChan <- update:
					default:
					}
				}
			}
		}(ip)
	}

	wg.Wait()

	if progressChan != nil && ctx.Err() == nil {
		progressChan <- ProgressUpdate{
			ScannedCount: total,
			TotalTargets: total,
			FoundCount:   int(atomic.LoadInt64(&foundCount)),
			IsComplete:   true,
		}
	}

	return results
}

// probeHost tests host availability using ARP, concurrent TCP connect fallback, and port scanning.
func (s *Scanner) probeHost(ctx context.Context, ip string) *HostResult {
	if ctx.Err() != nil {
		return nil
	}

	start := time.Now()
	mac, err := SendARPWindows(ip)
	rtt := time.Since(start)

	isAlive := false
	if err == nil && mac != "" && mac != "00:00:00:00:00:00" {
		isAlive = true
	} else if ctx.Err() == nil && rtt < 150*time.Millisecond {
		// SendARP failed fast (e.g. routed subnet or non-ARP interface like VPN/loopback):
		// run concurrent TCP fallback probe on common ports.
		quickPorts := []int{80, 445, 135, 443, 22, 3389}
		probeCtx, cancelProbe := context.WithCancel(ctx)
		var probeWg sync.WaitGroup
		var aliveOnce sync.Once

		for _, qp := range quickPorts {
			probeWg.Add(1)
			go func(port int) {
				defer probeWg.Done()
				t0 := time.Now()
				open, _ := ScanPortContext(probeCtx, ip, port, s.Options.PingTimeout)
				if open {
					aliveOnce.Do(func() {
						isAlive = true
						rtt = time.Since(t0)
						cancelProbe()
					})
				}
			}(qp)
		}
		probeWg.Wait()
		cancelProbe()
	}

	if !isAlive || ctx.Err() != nil {
		return nil
	}

	res := &HostResult{
		IP:      ip,
		MAC:     mac,
		RTT:     rtt,
		RTTMs:   float64(rtt.Microseconds()) / 1000.0,
		IsAlive: true,
	}

	// Port scan if enabled
	if s.Options.ScanPorts && len(s.Options.Ports) > 0 && ctx.Err() == nil {
		res.OpenPorts = ScanPortsConcurrentContext(ctx, ip, s.Options.Ports, s.Options.Timeout, 20)
	}

	// Hostname resolution if enabled
	if s.Options.ResolveNames && ctx.Err() == nil {
		names, err := net.LookupAddr(ip)
		if err == nil && len(names) > 0 {
			res.Hostname = names[0]
		}
	}

	return res
}

// GenerateIPsFromCIDR parses a CIDR string, an IP range (e.g. "192.168.1.1-192.168.1.254" or "192.168.1.1-254"),
// or a single IPv4 address, and returns all target IPv4 addresses.
func GenerateIPsFromCIDR(targetStr string) ([]string, error) {
	targetStr = strings.TrimSpace(targetStr)
	if targetStr == "" {
		return nil, fmt.Errorf("empty target range")
	}

	// Case 1: IP Range with '-'
	if strings.Contains(targetStr, "-") && !strings.Contains(targetStr, "/") {
		parts := strings.SplitN(targetStr, "-", 2)
		startStr := strings.TrimSpace(parts[0])
		endStr := strings.TrimSpace(parts[1])

		startIP := net.ParseIP(startStr).To4()
		if startIP == nil {
			return nil, fmt.Errorf("invalid start IPv4: %s", startStr)
		}

		var endIP net.IP
		if !strings.Contains(endStr, ".") {
			// Shorthand like 192.168.1.1-254
			lastOctet, err := strconv.Atoi(endStr)
			if err != nil || lastOctet < 0 || lastOctet > 255 {
				return nil, fmt.Errorf("invalid end IPv4 octet: %s", endStr)
			}
			endIP = net.IPv4(startIP[0], startIP[1], startIP[2], byte(lastOctet)).To4()
		} else {
			endIP = net.ParseIP(endStr).To4()
		}
		if endIP == nil {
			return nil, fmt.Errorf("invalid end IPv4: %s", endStr)
		}

		startU := binary.BigEndian.Uint32(startIP)
		endU := binary.BigEndian.Uint32(endIP)
		if endU < startU {
			return nil, fmt.Errorf("end IP (%s) is smaller than start IP (%s)", endIP.String(), startIP.String())
		}
		if endU-startU > 65536 {
			return nil, fmt.Errorf("IP range too large (%d hosts, max 65536)", endU-startU+1)
		}

		ips := make([]string, 0, endU-startU+1)
		for i := startU; i <= endU; i++ {
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, i)
			ips = append(ips, ip.String())
		}
		return ips, nil
	}

	// Case 2: Single IPv4 address without '/'
	if !strings.Contains(targetStr, "/") {
		singleIP := net.ParseIP(targetStr).To4()
		if singleIP == nil {
			return nil, fmt.Errorf("invalid IPv4 or CIDR: %s", targetStr)
		}
		return []string{singleIP.String()}, nil
	}

	// Case 3: Standard CIDR notation
	_, ipNet, err := net.ParseCIDR(targetStr)
	if err != nil {
		return nil, err
	}

	ip4 := ipNet.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("only IPv4 is supported: %s", targetStr)
	}

	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("invalid IPv4 mask: %s", targetStr)
	}
	if ones == 32 {
		return []string{ip4.String()}, nil
	}

	mask := binary.BigEndian.Uint32(ipNet.Mask)
	start := binary.BigEndian.Uint32(ip4)
	network := start & mask
	broadcast := network | (^mask)

	if ones == 31 {
		ips := make([]string, 0, 2)
		for i := network; i <= broadcast; i++ {
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, i)
			ips = append(ips, ip.String())
		}
		return ips, nil
	}

	if broadcast-network-1 > 65536 {
		return nil, fmt.Errorf("CIDR range too large (%d hosts, max 65536)", broadcast-network-1)
	}

	ips := make([]string, 0, broadcast-network-1)
	for i := network + 1; i < broadcast; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		ips = append(ips, ip.String())
	}

	return ips, nil
}

// GetLocalSubnets returns active IPv4 subnets sorted by priority (physical LAN first).
func GetLocalSubnets() []SubnetInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []SubnetInfo{}
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
			// Skip disconnected APIPA (169.254.x.x) addresses
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}

			ones, _ := ipNet.Mask.Size()
			networkIP := ip.Mask(ipNet.Mask)
			cidr := fmt.Sprintf("%s/%d", networkIP.String(), ones)

			priority := 50
			nameLower := strings.ToLower(iface.Name)

			if ones == 32 {
				priority = 10
			} else if strings.Contains(nameLower, "tailscale") || strings.Contains(nameLower, "vpn") || strings.Contains(nameLower, "wireguard") {
				priority = 20
			} else if strings.Contains(nameLower, "wsl") || strings.Contains(nameLower, "veth") || strings.Contains(nameLower, "hyper-v") || strings.Contains(nameLower, "docker") || strings.Contains(nameLower, "vmware") || strings.Contains(nameLower, "virtualbox") {
				priority = 30
			} else if strings.Contains(nameLower, "wi-fi") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "イーサネット") || strings.Contains(nameLower, "ethernet") {
				priority = 100
				if ones < 24 {
					c24 := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
					subnets = append(subnets, SubnetInfo{
						Name:     iface.Name + " (/24)",
						IP:       ip.String(),
						CIDR:     c24,
						Label:    fmt.Sprintf("%s 推奨 /24 (%s - %s)", iface.Name, c24, ip.String()),
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

			// Also provide a /24 quick option for non-virtual private LANs with masks wider than /24
			if priority >= 80 && priority < 100 && ones < 24 {
				c24 := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
				subnets = append(subnets, SubnetInfo{
					Name:     iface.Name + " (/24)",
					IP:       ip.String(),
					CIDR:     c24,
					Label:    fmt.Sprintf("%s 推奨 /24 (%s - %s)", iface.Name, c24, ip.String()),
					Priority: priority + 5,
				})
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

	return subnets
}
