package engine

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ScanPort attempts to connect to a specific TCP port and returns whether it is open and the duration.
func ScanPort(ip string, port int, timeout time.Duration) (bool, time.Duration) {
	target := fmt.Sprintf("%s:%d", ip, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, time.Since(start)
}

// ScanPortsConcurrent scans multiple ports concurrently using Goroutines.
func ScanPortsConcurrent(ip string, ports []int, timeout time.Duration, concurrency int) []int {
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

	for _, p := range ports {
		wg.Add(1)
		sem <- struct{}{}
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()

			open, _ := ScanPort(ip, port, timeout)
			if open {
				mu.Lock()
				openPorts = append(openPorts, port)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()
	return openPorts
}
