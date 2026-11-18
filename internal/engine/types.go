package engine

import "time"

// HostResult represents the scan result of a single host.
type HostResult struct {
	IP        string        `json:"ip"`
	MAC       string        `json:"mac"`
	Vendor    string        `json:"vendor"`
	Hostname  string        `json:"hostname"`
	OpenPorts []int         `json:"open_ports"`
	RTT       time.Duration `json:"rtt"`
	RTTMs     float64       `json:"rtt_ms"`
	IsAlive   bool          `json:"is_alive"`
}

// ScanOptions configures the concurrent network scan.
type ScanOptions struct {
	SubnetCIDR   string        `json:"subnet_cidr"`
	StartIP      string        `json:"start_ip"`
	EndIP        string        `json:"end_ip"`
	Ports        []int         `json:"ports"`
	Concurrency  int           `json:"concurrency"`
	Timeout      time.Duration `json:"timeout"`
	PingTimeout  time.Duration `json:"ping_timeout"`
	ScanPorts    bool          `json:"scan_ports"`
	ResolveNames bool          `json:"resolve_names"`
}

// ProgressUpdate represents real-time scan progress.
type ProgressUpdate struct {
	ScannedCount int         `json:"scanned_count"`
	TotalTargets int         `json:"total_targets"`
	FoundCount   int         `json:"found_count"`
	LatestHost   *HostResult `json:"latest_host,omitempty"`
	IsComplete   bool        `json:"is_complete"`
}

// SubnetInfo represents a detected local network interface subnet.
type SubnetInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default"`
	Priority  int    `json:"priority"`
}
