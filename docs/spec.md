# QuickNetScan Specifications

## 1. Application Overview

- **Name**: QuickNetScan
- **Purpose**: A single-binary Windows utility that instantly scans local IP ranges (home or office LAN) using highly concurrent goroutines across hundreds of targets, listing active devices, IP/MAC addresses, open ports, and latency in real time.
- **Target OS**: Windows 10 / 11 (64-bit)
- **Implementation**: Go (Goroutine concurrent scanning engine) + WebAssembly (WASM) / HTML / CSS
- **Distribution Format**: Flat ZIP archive on GitHub Releases (single binary)
- **Version**: v1.0.0

## 2. Architecture

```text
[WASM / HTML / CSS]  ←WebSocket / WebMessage / Local API→  [Go Host & Goroutine Engine]
```

- `internal/engine/`: Core concurrent scanning engine. ARP resolution, ICMP Ping / TCP connect concurrent scanning, port probing, RTT measurement. GUI-independent, shared with CLI.
- `cmd/quicknetscan/`: GUI application entry point. Manages embedded assets (Go `embed`), local web server / WebView2 host.
- `cmd/quicknetscan-cli/`: CLI application entry point.
- `web/cmd/wasm/`: WebAssembly Go logic. Reactive state management and UI formatting.
- `web/static/`: Quick series standard dark UI (HTML/CSS).

## 3. UI Layout

| Region | Content |
| --- | --- |
| Header | Title (QuickNetScan), Language switch pill button (JA/EN), About modal trigger, Export button |
| Control Panel | IP range input (Auto-detect / CIDR / Range), Port scan presets / custom, Concurrency slider (Goroutine count), Start/Stop scan button |
| Main Table | Detected devices table (Status, IP address, MAC address, Vendor, Hostname, Open ports, Latency / RTT) |
| Status Bar | Scan progress bar, detected device count, total target count, elapsed time |

## 4. Feature List

1. **Subnet Auto-detection & Scan**
   - Automatically detects current network adapter subnet (CIDR)
   - Supports arbitrary IP ranges (e.g. 192.168.1.1 - 192.168.1.254)
2. **Ultra-concurrent Goroutine Scanning**
   - Spawns hundreds of concurrent goroutines to scan hundreds or thousands of targets within seconds
3. **Device Information Gathering**
   - Host liveness detection via ARP cache and fast pinging
   - MAC address and OUI vendor resolution
   - High-speed TCP port scanning on standard ports (80, 443, 22, 445, 3389, etc.)
4. **Data Export**
   - Export scan results to CSV or JSON
5. **Quick Series Dark UI & Bilingual Support**
   - One-click toggle between Japanese and English
   - Standard About modal with version, tech stack, and author badge
