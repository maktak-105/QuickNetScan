package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"quicknetscan/internal/server"
	"runtime"
	"syscall"
	"time"
)

func main() {
	srv, err := server.NewAppServer()
	if err != nil {
		log.Fatalf("Failed to initialize internal server: %v", err)
	}

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	url := srv.URL()
	log.Printf("QuickNetScan running at %s", url)

	// Launch Edge/Chrome in app mode, or open default browser
	go func() {
		time.Sleep(200 * time.Millisecond)
		openGUI(url)
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down QuickNetScan...")
	_ = srv.Close()
}

func openGUI(url string) {
	if runtime.GOOS == "windows" {
		// Try Microsoft Edge in standalone app window mode
		msedgePaths := []string{
			os.ExpandEnv(`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`),
			os.ExpandEnv(`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`),
		}
		for _, path := range msedgePaths {
			if _, err := os.Stat(path); err == nil {
				cmd := exec.Command(path, fmt.Sprintf("--app=%s", url), "--window-size=1024,720")
				if err := cmd.Start(); err == nil {
					return
				}
			}
		}
		// Fallback to default browser
		_ = exec.Command("cmd", "/c", "start", url).Start()
	}
}
