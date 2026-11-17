# QuickNetScan Development Environment & Build Guide

## 1. Prerequisites

- **OS**: Windows 10 (64-bit) or Windows 11
- **Go**: 1.22 or later
- **Python**: 3.10 or later (for build scripts, standard library only)

## 2. Setting Up the Environment

### Installing Go
Download and install Go from https://go.dev/dl/ or use Chocolatey / Winget:

```powershell
choco install golang -y
```

Verify installation:
```powershell
go version
```

## 3. Build Instructions

### One-Click Build (WASM + Go Binaries)

Run `scripts/build.bat` from PowerShell:

```powershell
.\scripts\build.bat
```

The script automatically executes:
1. Compiles `web/cmd/wasm/` targeting `GOOS=js GOARCH=wasm` to `web/static/app.wasm`
2. Embeds static web assets (HTML, CSS, WASM) via Go's `embed` package
3. Compiles `cmd/quicknetscan/` for Windows, producing `dist/QuickNetScan.exe`
4. Compiles `cmd/quicknetscan-cli/`, producing `dist/QuickNetScan_cli.exe`

## 4. Running Tests

```powershell
go test ./...
```
