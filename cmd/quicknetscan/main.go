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

	// Launch dedicated standalone app window
	exitChan := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		openGUI(url, exitChan)
	}()

	// Wait for window close or OS interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
	case <-exitChan:
	}

	log.Println("Shutting down QuickNetScan...")
	_ = srv.Close()
}

func openGUI(url string, exitChan chan struct{}) {
	if runtime.GOOS == "windows" {
		candidates := []string{
			os.ExpandEnv(`%ProgramFiles%\Google\Chrome\Application\chrome.exe`),
			os.ExpandEnv(`%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`),
			os.ExpandEnv(`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`),
			os.ExpandEnv(`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`),
		}

		appArgs := []string{
			fmt.Sprintf("--app=%s", url),
			"--window-size=1080,750",
			"--user-data-dir=" + os.TempDir() + "\\quicknetscan_profile",
		}

		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				cmd := exec.Command(path, appArgs...)
				if err := cmd.Start(); err == nil {
					_ = cmd.Wait()
					close(exitChan)
					return
				}
			}
		}

		// Fallback
		_ = exec.Command("cmd", "/c", "start", url).Start()
	}
}
