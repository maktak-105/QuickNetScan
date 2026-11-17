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
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
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

func (s *AppServer) handleLocalSubnets(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var subnets []string
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
			if ok && ipNet.IP.To4() != nil {
				subnets = append(subnets, ipNet.String())
			}
		}
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
