# QuickNetScan Changelog

This file records the major changes in each public version.

## Versioning rules

- First digit (for example, `1.0.0` to `2.0.0`): new features
- Second digit (for example, `1.0.0` to `1.1.0`): bug fixes
- Third digit (for example, `1.1.0` to `1.1.1`): other changes, such as documentation updates

## 1.0.0 (2026-11-17)

### Initial Release

- High-concurrency network scanning engine using Go goroutines
- Real-time detection of active hosts, IP, MAC addresses, open ports, and ping latency
- Self-contained single binary with embedded WebAssembly (WASM) user interface
- Quick series common dark UI with Japanese / English bilingual support
- Command-line interface (`QuickNetScan_cli.exe`)
