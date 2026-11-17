package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"quicknetscan/internal/engine"
	"quicknetscan/web"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppServer handles API requests and serves embedded WASM/Web assets.
type AppServer struct {
	server           *http.Server
	listener         net.Listener
	activeScanCancel context.CancelFunc
	mu               sync.Mutex
	progressSubs     []chan engine.ProgressUpdate
}

// NewAppServer initializes the application server.
func NewAppServer() (*AppServer, error) {
	var ln net.Listener
	var err error

	// Try standard port 28080, fallback to other ports if busy
	for port := 28080; port < 28100; port++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if ln == nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
	}

	app := &AppServer{
		listener: ln,
	}

	mux := http.NewServeMux()

	// Serve static files from web.StaticFS
	staticSub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(staticSub))
	mux.Handle("/", fileServer)

	// API routes
	mux.HandleFunc("/api/local-subnets", app.handleLocalSubnets)
	mux.HandleFunc("/api/scan/start", app.handleScanStart)
	mux.HandleFunc("/api/scan/stop", app.handleScanStop)
	mux.HandleFunc("/api/scan/events", app.handleScanEvents)

	app.server = &http.Server{
		Handler: mux,
	}

	return app, nil
}

// URL returns the local HTTP URL.
func (s *AppServer) URL() string {
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}

// Start begins serving in background.
func (s *AppServer) Start() error {
	return s.server.Serve(s.listener)
}

// Close terminates the server.
func (s *AppServer) Close() error {
	s.mu.Lock()
	if s.activeScanCancel != nil {
		s.activeScanCancel()
	}
	s.mu.Unlock()
	return s.server.Close()
}

type SubnetInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default"`
	Priority  int    `json:"priority"`
}

func (s *AppServer) handleLocalSubnets(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

			// Deprioritize APIPA and single-host /32
			if ip[0] == 169 && ip[1] == 254 {
				priority = 5
			} else if ones == 32 {
				priority = 10
			} else if strings.Contains(nameLower, "tailscale") || strings.Contains(nameLower, "vpn") {
				priority = 20
			} else if strings.Contains(nameLower, "wsl") || strings.Contains(nameLower, "veth") || strings.Contains(nameLower, "hyper-v") {
				priority = 30
			} else if strings.Contains(nameLower, "wi-fi") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "イーサネット") || strings.Contains(nameLower, "ethernet") {
				// Prioritize real LAN adapters
				priority = 100
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

	// Sort by priority descending
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"subnets": subnets,
	})
}

func (s *AppServer) handleScanStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TargetCIDR  string `json:"target_cidr"`
		Ports       string `json:"ports"`
		Concurrency int    `json:"concurrency"`
		ScanPorts   bool   `json:"scan_ports"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var portList []int
	if req.ScanPorts && req.Ports != "" {
		for _, pStr := range strings.Split(req.Ports, ",") {
			p, err := strconv.Atoi(strings.TrimSpace(pStr))
			if err == nil && p > 0 && p <= 65535 {
				portList = append(portList, p)
			}
		}
	}

	ips, err := engine.GenerateIPsFromCIDR(req.TargetCIDR)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid CIDR: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.activeScanCancel != nil {
		s.activeScanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.activeScanCancel = cancel
	s.mu.Unlock()

	opts := engine.ScanOptions{
		SubnetCIDR:   req.TargetCIDR,
		Ports:        portList,
		Concurrency:  req.Concurrency,
		ScanPorts:    req.ScanPorts,
		Timeout:      1200 * time.Millisecond,
		PingTimeout:  400 * time.Millisecond,
		ResolveNames: false,
	}

	scanner := engine.NewScanner(opts)
	progressChan := make(chan engine.ProgressUpdate, 100)

	go func() {
		defer close(progressChan)
		scanner.ScanRange(ctx, ips, progressChan)
	}()

	go s.broadcastProgress(progressChan)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "started",
		"total_targets": len(ips),
	})
}

func (s *AppServer) handleScanStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.activeScanCancel != nil {
		s.activeScanCancel()
		s.activeScanCancel = nil
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *AppServer) handleScanEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan engine.ProgressUpdate, 50)
	s.mu.Lock()
	s.progressSubs = append(s.progressSubs, ch)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		for i, sub := range s.progressSubs {
			if sub == ch {
				s.progressSubs = append(s.progressSubs[:i], s.progressSubs[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		close(ch)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(update)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *AppServer) broadcastProgress(ch <-chan engine.ProgressUpdate) {
	for update := range ch {
		s.mu.Lock()
		for _, sub := range s.progressSubs {
			select {
			case sub <- update:
			default:
			}
		}
		s.mu.Unlock()
	}
}
