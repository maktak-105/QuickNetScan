#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
QuickNetScan Build Script
Compiles WASM frontend and Go executables.
"""

import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
DIST_DIR = ROOT_DIR / "dist"
WEB_DIR = ROOT_DIR / "web"
WASM_SRC = WEB_DIR / "cmd" / "wasm"
WASM_OUT = WEB_DIR / "static" / "app.wasm"

def run_command(cmd, env=None, cwd=None):
    print(f"[RUN] {' '.join(cmd)}")
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    res = subprocess.run(cmd, env=merged_env, cwd=cwd or ROOT_DIR)
    if res.returncode != 0:
        print(f"[ERROR] Command failed with code {res.returncode}: {' '.join(cmd)}")
        sys.exit(res.returncode)

def main():
    print("=== Building QuickNetScan ===")
    DIST_DIR.mkdir(parents=True, exist_ok=True)

    # 1. Check Go
    go_bin = shutil.which("go")
    if not go_bin:
        default_paths = [
            Path(r"C:\Program Files\Go\bin\go.exe"),
            Path(os.path.expandvars(r"%LOCALAPPDATA%\Programs\Go\bin\go.exe")),
        ]
        for p in default_paths:
            if p.exists():
                go_bin = str(p)
                break

    if not go_bin:
        print("[ERROR] Go compiler ('go') not found in PATH or standard install paths.")
        sys.exit(1)

    # 2. Compile WebAssembly Frontend
    print("\n--- Compiling WASM UI ---")
    wasm_env = {
        "GOOS": "js",
        "GOARCH": "wasm"
    }
    run_command([go_bin, "build", "-trimpath", "-ldflags", "-s -w", "-o", str(WASM_OUT), "./web/cmd/wasm"], env=wasm_env)

    # 3. Compile GUI Executable
    print("\n--- Compiling GUI Executable ---")
    gui_out = DIST_DIR / "QuickNetScan.exe"
    run_command([
        go_bin, "build",
        "-trimpath",
        "-ldflags", "-H=windowsgui -s -w",
        "-o", str(gui_out),
        "./cmd/quicknetscan"
    ])

    # 4. Compile CLI Executable
    print("\n--- Compiling CLI Executable ---")
    cli_out = DIST_DIR / "QuickNetScan_cli.exe"
    run_command([
        go_bin, "build",
        "-trimpath",
        "-ldflags", "-s -w",
        "-o", str(cli_out),
        "./cmd/quicknetscan-cli"
    ])

    print("\n=== Build Complete ===")
    print(f"GUI: {gui_out}")
    print(f"CLI: {cli_out}")

if __name__ == "__main__":
    main()
