#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
WASM_SRC = ROOT_DIR / "web" / "cmd" / "wasm"
INTER_DIR = ROOT_DIR / "build" / "intermediate"
WASM_BIN = INTER_DIR / "app.wasm"
STATIC_WASM = ROOT_DIR / "web" / "static" / "app.wasm"
WASM_EXEC_JS = ROOT_DIR / "web" / "static" / "js" / "wasm_exec.js"

def sync_wasm_exec_js(go_bin):
    try:
        res = subprocess.run([go_bin, "env", "GOROOT"], capture_output=True, text=True, check=True)
        goroot = Path(res.stdout.strip())
        candidates = [
            goroot / "lib" / "wasm" / "wasm_exec.js",
            goroot / "misc" / "wasm" / "wasm_exec.js",
        ]
        for cand in candidates:
            if cand.exists():
                shutil.copy2(cand, WASM_EXEC_JS)
                print(f"[WASM] Synced wasm_exec.js from {cand}")
                return
    except Exception as e:
        print(f"[WASM] Note: Could not sync wasm_exec.js from GOROOT: {e}")

def main():
    INTER_DIR.mkdir(parents=True, exist_ok=True)
    go_bin = r"C:\Program Files\Go\bin\go.exe"
    if not os.path.exists(go_bin):
        go_bin = shutil.which("go") or "go"

    sync_wasm_exec_js(go_bin)

    print("[WASM] Compiling Go to WebAssembly...")
    env = os.environ.copy()
    env["GOOS"] = "js"
    env["GOARCH"] = "wasm"

    res = subprocess.run([go_bin, "build", "-trimpath", "-ldflags", "-s -w", "-o", str(WASM_BIN), "./web/cmd/wasm"], env=env, cwd=ROOT_DIR)
    if res.returncode != 0:
        print("[ERROR] WASM build failed")
        sys.exit(res.returncode)

    shutil.copy2(WASM_BIN, STATIC_WASM)
    print("[WASM] Build complete! Binary size: {:.2f} MB".format(WASM_BIN.stat().st_size / (1024 * 1024)))

if __name__ == "__main__":
    main()
