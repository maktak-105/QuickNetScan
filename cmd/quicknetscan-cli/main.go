package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"quicknetscan/internal/engine"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	rangeFlag := flag.String("range", "", "Target CIDR (e.g. 192.168.1.0/24)")
	portsFlag := flag.String("ports", "22,80,443,445,3389", "Comma-separated port list to probe")
	concurrencyFlag := flag.Int("concurrency", 256, "Number of concurrent goroutines")
	scanPortsFlag := flag.Bool("scan-ports", true, "Enable TCP port scanning")
	timeoutMsFlag := flag.Int("timeout", 1000, "Port connection timeout in ms")
	flag.Parse()

	if *rangeFlag == "" {
		fmt.Println("QuickNetScan CLI - High-speed LAN Scanner")
		fmt.Println("Usage: QuickNetScan_cli.exe -range <CIDR> [-ports <port_list>] [-concurrency <int>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ips, err := engine.GenerateIPsFromCIDR(*rangeFlag)
	if err != nil {
		fmt.Printf("Error: invalid CIDR '%s': %v\n", *rangeFlag, err)
		os.Exit(1)
	}

	var ports []int
	for _, pStr := range strings.Split(*portsFlag, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(pStr))
		if err == nil && p > 0 && p <= 65535 {
			ports = append(ports, p)
		}
	}

	opts := engine.ScanOptions{
		SubnetCIDR:  *rangeFlag,
		Ports:       ports,
		Concurrency: *concurrencyFlag,
		ScanPorts:   *scanPortsFlag,
		Timeout:     time.Duration(*timeoutMsFlag) * time.Millisecond,
		PingTimeout: 400 * time.Millisecond,
	}

	scanner := engine.NewScanner(opts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nScan canceled by user.")
		cancel()
	}()

	fmt.Printf("Starting scan on %s (%d hosts) with %d concurrent goroutines...\n", *rangeFlag, len(ips), *concurrencyFlag)
	startTime := time.Now()

	progressChan := make(chan engine.ProgressUpdate, 50)
	go func() {
		defer close(progressChan)
		scanner.ScanRange(ctx, ips, progressChan)
	}()

	fmt.Printf("%-18s %-20s %-10s %s\n", "IP Address", "MAC Address", "Latency", "Open Ports")
	fmt.Println(strings.Repeat("-", 65))

	for update := range progressChan {
		if update.LatestHost != nil && update.LatestHost.IsAlive {
			h := update.LatestHost
			var portStrs []string
			for _, p := range h.OpenPorts {
				portStrs = append(portStrs, strconv.Itoa(p))
			}
			portsDisplay := strings.Join(portStrs, ", ")
			if portsDisplay == "" {
				portsDisplay = "-"
			}
			fmt.Printf("%-18s %-20s %-10.1fms %s\n", h.IP, h.MAC, h.RTTMs, portsDisplay)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Println(strings.Repeat("-", 65))
	fmt.Printf("Scan completed in %.2fs.\n", elapsed.Seconds())
}
