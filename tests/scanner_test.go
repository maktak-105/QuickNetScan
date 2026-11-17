package tests

import (
	"quicknetscan/internal/engine"
	"testing"
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
