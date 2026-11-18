package engine

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// ScanPort attempts to connect to a specific TCP port and returns whether it is open and the duration.
func ScanPort(ip string, port int, timeout time.Duration) (bool, time.Duration) {
	return ScanPortContext(context.Background(), ip, port, timeout)
}

// ScanPortContext attempts to connect to a specific TCP port with context cancellation support.
func ScanPortContext(ctx context.Context, ip string, port int, timeout time.Duration) (bool, time.Duration) {
	target := fmt.Sprintf("%s:%d", ip, port)
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, time.Since(start)
}

// ScanPortsConcurrent scans multiple ports concurrently using Goroutines.
func ScanPortsConcurrent(ip string, ports []int, timeout time.Duration, concurrency int) []int {
	return ScanPortsConcurrentContext(context.Background(), ip, ports, timeout, concurrency)
}

// ScanPortsConcurrentContext scans multiple ports concurrently with context cancellation and sorted output.
func ScanPortsConcurrentContext(ctx context.Context, ip string, ports []int, timeout time.Duration, concurrency int) []int {
	if len(ports) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 50
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var openPorts []int
	var wg sync.WaitGroup

portLoop:
	for _, p := range ports {
		select {
		case <-ctx.Done():
			break portLoop
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			open, _ := ScanPortContext(ctx, ip, port, timeout)
			if open {
				mu.Lock()
				openPorts = append(openPorts, port)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()
	sort.Ints(openPorts)
	return openPorts
}
