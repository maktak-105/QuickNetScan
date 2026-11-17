QuickNetScan - High-speed Local Network Scanner
Distribution package  v1.0.0

GitHub repository:
  https://github.com/maktak-105/QuickNetScan
Releases and updates:
  https://github.com/maktak-105/QuickNetScan/releases

--------------------------------------------------------------------------------
System Requirements
--------------------------------------------------------------------------------
- Windows 10 (64-bit) or Windows 11
- Microsoft Edge WebView2 Runtime (installed by default on Windows 10/11)
- Network adapter connected to a local area network (LAN / Wi-Fi)

--------------------------------------------------------------------------------
Getting Started
--------------------------------------------------------------------------------
Extract all files from the ZIP archive to a folder of your choice.
Run QuickNetScan.exe directly. No installation or administrative setup is required.

--------------------------------------------------------------------------------
Basic Usage
--------------------------------------------------------------------------------
[GUI Version]
1. Run QuickNetScan.exe.
2. Enter your subnet or target range (e.g. 192.168.1.0/24).
3. Select port scan options (Web, SSH, RDP, File Share, etc.).
4. Click "Scan" to launch concurrent goroutines across all targets.
5. Live results showing IP, MAC, ports, and ping response times appear immediately.

[CLI Version]
Run QuickNetScan_cli.exe with desired options:
  QuickNetScan_cli.exe -range 192.168.1.0/24 -ports 22,80,443,445,3389

--------------------------------------------------------------------------------
Distribution Files
--------------------------------------------------------------------------------
- QuickNetScan.exe      : GUI executable with embedded WASM user interface
- QuickNetScan_cli.exe  : Command-line interface executable
- readme.txt            : This user guide (English)
- readme_jp.txt         : User guide (Japanese)
- history.txt           : Change log (English)
- history_jp.txt        : Change log (Japanese)
- LICENSE.txt           : MIT License text (English)
- LICENSE_jp.txt        : MIT License Japanese translation

--------------------------------------------------------------------------------
License and Warranty
--------------------------------------------------------------------------------
QuickNetScan is free and open-source software licensed under the MIT License.
See LICENSE.txt for details. The software is provided "AS IS", without warranty
of any kind.
