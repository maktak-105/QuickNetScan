package engine

import (
	"context"
	"encoding/binary"
	"net"
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
		opts.Timeout = 1200 * time.Millisecond
	}
	if opts.PingTimeout <= 0 {
		opts.PingTimeout = 400 * time.Millisecond
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

	for _, ip := range ipList {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			res := s.probeHost(ctx, targetIP)
			atomic.AddInt64(&scannedCount, 1)

			if res != nil && res.IsAlive {
				atomic.AddInt64(&foundCount, 1)
				resultsMu.Lock()
				results = append(results, res)
				resultsMu.Unlock()
			}

			if progressChan != nil {
				select {
				case progressChan <- ProgressUpdate{
					ScannedCount: int(atomic.LoadInt64(&scannedCount)),
					TotalTargets: total,
					FoundCount:   int(atomic.LoadInt64(&foundCount)),
					LatestHost:   res,
					IsComplete:   false,
				}:
				default:
				}
			}
		}(ip)
	}

	wg.Wait()

	if progressChan != nil {
		progressChan <- ProgressUpdate{
			ScannedCount: total,
			TotalTargets: total,
			FoundCount:   int(atomic.LoadInt64(&foundCount)),
			IsComplete:   true,
		}
	}

	return results
}

// probeHost tests host availability using ARP, TCP connect, and port scanning.
func (s *Scanner) probeHost(ctx context.Context, ip string) *HostResult {
	start := time.Now()
	mac, err := SendARPWindows(ip)
	rtt := time.Since(start)

	isAlive := false
	if err == nil && mac != "" && mac != "00:00:00:00:00:00" {
		isAlive = true
	} else {
		// Fallback TCP probe on common quick ports
		quickPorts := []int{80, 445, 135, 443, 22}
		for _, qp := range quickPorts {
			start = time.Now()
			open, _ := ScanPort(ip, qp, s.Options.PingTimeout)
			if open {
				isAlive = true
				rtt = time.Since(start)
				break
			}
		}
	}

	if !isAlive {
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
	if s.Options.ScanPorts && len(s.Options.Ports) > 0 {
		res.OpenPorts = ScanPortsConcurrent(ip, s.Options.Ports, s.Options.Timeout, 20)
	}

	// Hostname resolution if enabled
	if s.Options.ResolveNames {
		names, err := net.LookupAddr(ip)
		if err == nil && len(names) > 0 {
			res.Hostname = names[0]
		}
	}

	return res
}

// GenerateIPsFromCIDR parses a CIDR string and returns all host IPv4 addresses.
func GenerateIPsFromCIDR(cidrStr string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, err
	}

	var ips []string
	mask := binary.BigEndian.Uint32(ipNet.Mask)
	start := binary.BigEndian.Uint32(ipNet.IP)

	// network address to broadcast address
	finish := (start & mask) | (mask ^ 0xffffffff)

	for i := start + 1; i < finish; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		ips = append(ips, ip.String())
	}

	return ips, nil
}
