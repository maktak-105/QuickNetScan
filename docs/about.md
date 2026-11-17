# About QuickNetScan

## 1. Application Information

- **Application Name**: QuickNetScan
- **Japanese Name**: クイック・ネットスキャン
- **Latest Version**: v1.0.0
- **Release Date**: 2026-11-17
- **Author**: maktak-105 (GitHub: https://github.com/maktak-105)
- **License**: MIT License

## 2. Technology Stack

| Component | Technology |
| --- | --- |
| Scanning Engine | Go 1.22+ (Goroutines ultra-concurrent socket/ARP probing) |
| Frontend | WebAssembly (Go WASM) + HTML5 / CSS3 (Quick series common dark theme) |
| Binary Packaging | Self-contained single executable (embedded assets via Go `embed`) |
| CI/CD | GitHub Actions (automated builds & integrity verification on Windows runners) |
