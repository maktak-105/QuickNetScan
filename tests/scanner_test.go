package tests

import (
	"context"
	"net"
	"quicknetscan/internal/engine"
	"strconv"
	"testing"
	"time"
)

func TestGenerateIPsFromCIDR(t *testing.T) {
	ips, err := engine.GenerateIPsFromCIDR("192.168.1.0/29")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// /29 has 8 addresses, minus network and broadcast = 6 hosts
	if len(ips) != 6 {
		t.Errorf("expected 6 IPs, got %d", len(ips))
	}

	if ips[0] != "192.168.1.1" {
		t.Errorf("expected first IP to be 192.168.1.1, got %s", ips[0])
	}
	if ips[len(ips)-1] != "192.168.1.6" {
		t.Errorf("expected last IP to be 192.168.1.6, got %s", ips[len(ips)-1])
	}
}

func TestGenerateIPsFromRangeAndSingle(t *testing.T) {
	// Full dash range
	ips, err := engine.GenerateIPsFromCIDR("10.0.0.10 - 10.0.0.15")
	if err != nil {
		t.Fatalf("unexpected error on full range: %v", err)
	}
	if len(ips) != 6 || ips[0] != "10.0.0.10" || ips[5] != "10.0.0.15" {
		t.Errorf("unexpected range result: %v", ips)
	}

	// Shorthand dash range
	ips2, err := engine.GenerateIPsFromCIDR("192.168.0.100-102")
	if err != nil {
		t.Fatalf("unexpected error on shorthand range: %v", err)
	}
	if len(ips2) != 3 || ips2[0] != "192.168.0.100" || ips2[2] != "192.168.0.102" {
		t.Errorf("unexpected shorthand range result: %v", ips2)
	}

	// Single IP
	ips3, err := engine.GenerateIPsFromCIDR("127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error on single IP: %v", err)
	}
	if len(ips3) != 1 || ips3[0] != "127.0.0.1" {
		t.Errorf("unexpected single IP result: %v", ips3)
	}
}

func TestScanPortsConcurrentSortingAndCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	openPort, _ := strconv.Atoi(portStr)

	ports := []int{65001, openPort, 65002}
	res := engine.ScanPortsConcurrent("127.0.0.1", ports, 200*time.Millisecond, 10)
	if len(res) != 1 || res[0] != openPort {
		t.Errorf("expected open port [%d], got %v", openPort, res)
	}

	// Cancelled context should return immediately with no results
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resCanceled := engine.ScanPortsConcurrentContext(ctx, "127.0.0.1", ports, 200*time.Millisecond, 10)
	if len(resCanceled) != 0 {
		t.Errorf("expected empty result on canceled context, got %v", resCanceled)
	}
}
