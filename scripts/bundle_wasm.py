#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import base64
import os
import subprocess
import sys
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
WASM_SRC = ROOT_DIR / "web" / "cmd" / "wasm"
INTER_DIR = ROOT_DIR / "build" / "intermediate"
WASM_BIN = INTER_DIR / "app.wasm"
OUT_JS = ROOT_DIR / "web" / "static" / "js" / "wasm_data.js"

def main():
    INTER_DIR.mkdir(parents=True, exist_ok=True)
    go_bin = r"C:\Program Files\Go\bin\go.exe"
    if not os.path.exists(go_bin):
        go_bin = "go"

    print("[WASM] Compiling Go to WebAssembly...")
    env = os.environ.copy()
    env["GOOS"] = "js"
    env["GOARCH"] = "wasm"

    res = subprocess.run([go_bin, "build", "-trimpath", "-ldflags", "-s -w", "-o", str(WASM_BIN), "./web/cmd/wasm"], env=env, cwd=ROOT_DIR)
    if res.returncode != 0:
        print("[ERROR] WASM build failed")
        sys.exit(res.returncode)

    print(f"[WASM] Encoding {WASM_BIN} to Base64 in {OUT_JS}...")
    with open(WASM_BIN, "rb") as f:
        wasm_bytes = f.read()

    b64_str = base64.b64encode(wasm_bytes).decode("ascii")
    with open(OUT_JS, "w", encoding="utf-8") as f:
        f.write("// Auto-generated: Embedded Go WebAssembly binary (Zero HTTP Server required)\n")
        f.write(f'const GO_WASM_BASE64 = "{b64_str}";\n')

    print("[WASM] Embedding complete! Total size: {:.2f} MB".format(len(b64_str) / (1024 * 1024)))

if __name__ == "__main__":
    main()
