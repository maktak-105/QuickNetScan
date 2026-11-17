#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
QuickNetScan Full Native Build Script
Builds:
1. Go WASM binary embedded into JS (Zero HTTP Server)
2. Go Concurrent Engine DLL (c-shared)
3. Self-contained bundled HTML
4. Windows Resource (.rc)
5. Native Win32 + WebView2 Desktop Application (QuickNetScan.exe)
6. CLI Executable (QuickNetScan_cli.exe)
"""

import os
import shutil
import subprocess
import sys
from pathlib import Path

# Add scripts directory to path
SCRIPTS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS_DIR))

import bundle_html
import bundle_wasm

ROOT_DIR = SCRIPTS_DIR.parent
DIST_DIR = ROOT_DIR / "dist"
INTER_DIR = ROOT_DIR / "build" / "intermediate"
SRC_DIR = ROOT_DIR / "src"

def find_gxx():
    p = shutil.which("g++") or shutil.which("clang++")
    if p:
        return p
    candidates = [
        r"C:\tools\llvm-mingw\bin\g++.exe",
        r"C:\tools\llvm-mingw\bin\clang++.exe",
        r"C:\msys64\ucrt64\bin\g++.exe",
        r"C:\msys64\mingw64\bin\g++.exe",
        r"C:\ProgramData\chocolatey\bin\g++.exe",
    ]
    for c in candidates:
        if os.path.exists(c):
            return c
    return None

def find_windres(gxx_path):
    if gxx_path:
        d = os.path.dirname(gxx_path)
        for name in ["windres.exe", "llvm-windres.exe"]:
            c = os.path.join(d, name)
            if os.path.exists(c):
                return c
    return shutil.which("windres") or shutil.which("llvm-windres")

def main():
    print("========================================")
    print("      QuickNetScan Native Build         ")
    print("========================================")

    DIST_DIR.mkdir(parents=True, exist_ok=True)
    INTER_DIR.mkdir(parents=True, exist_ok=True)

    go_bin = r"C:\Program Files\Go\bin\go.exe"
    if not os.path.exists(go_bin):
        go_bin = shutil.which("go")
    if not go_bin:
        print("[ERROR] Go compiler not found")
        sys.exit(1)

    # 1. Build and embed Go WASM into JS
    print("\n[1/6] Building & Inlining Go WASM into JS...")
    bundle_wasm.main()

    # 2. Build Go Engine DLL (Goroutines)
    print("\n[2/6] Building Go Concurrent Engine DLL (engine_x64.dll)...")
    out_dll = DIST_DIR / "engine_x64.dll"
    cmd_dll = [go_bin, "build", "-trimpath", "-buildmode=c-shared", "-ldflags", "-s -w", "-o", str(out_dll), "./cmd/engine-dll"]
    res = subprocess.run(cmd_dll, cwd=ROOT_DIR)
    if res.returncode != 0:
        print("[ERROR] Go Engine DLL build failed")
        sys.exit(res.returncode)

    # Copy DLL to core/native as well
    core_native = ROOT_DIR / "core" / "native"
    core_native.mkdir(parents=True, exist_ok=True)
    shutil.copy2(out_dll, core_native / "engine_x64.dll")

    # 3. Bundle HTML into self-contained file
    print("\n[3/6] Bundling HTML/CSS/JS/WASM/Assets into single HTML...")
    bundled_html = bundle_html.bundle(INTER_DIR)

    # 4. Compile Resource (.rc)
    print("\n[4/6] Compiling Windows Resource (.rc)...")
    gxx = find_gxx()
    if not gxx:
        print("[ERROR] C++ compiler (g++) not found")
        sys.exit(1)
    windres = find_windres(gxx)
    if not windres:
        print("[ERROR] windres not found")
        sys.exit(1)

    res_src = SRC_DIR / "app" / "QuickNetScan.rc"
    res_obj = INTER_DIR / "QuickNetScan_res.o"
    cmd_res = [windres, f"-I{SRC_DIR / 'app'}", f"-I{INTER_DIR}", str(res_src), "-O", "coff", "-o", str(res_obj)]
    res = subprocess.run(cmd_res, cwd=SRC_DIR / "app")
    if res.returncode != 0:
        print("[ERROR] Resource compilation failed")
        sys.exit(res.returncode)

    # 5. Compile Native Desktop GUI (Win32 + WebView2)
    print("\n[5/6] Compiling Native Desktop Application (QuickNetScan.exe)...")
    webview_inc = os.environ.get("WEBVIEW2_INCLUDE", r"C:\tools\webview2\build\native\include")
    gui_src = SRC_DIR / "app" / "main_gui.cpp"
    gui_out = DIST_DIR / "QuickNetScan.exe"

    cmd_gui = [
        gxx, "-O3", "-std=c++17", "-mwindows",
        "-static",
        f"-I{webview_inc}",
        f"-I{SRC_DIR / 'app'}",
        str(gui_src), str(res_obj),
        "-lkernel32", "-luser32", "-lgdi32", "-ldwmapi",
        "-lole32", "-loleaut32", "-luuid", "-lcomctl32", "-lshell32",
        "-s", "-o", str(gui_out)
    ]
    res = subprocess.run(cmd_gui, cwd=ROOT_DIR)
    if res.returncode != 0:
        print("[ERROR] Native GUI build failed")
        sys.exit(res.returncode)

    # 6. Compile CLI Executable
    print("\n[6/6] Compiling CLI Executable (QuickNetScan_cli.exe)...")
    cli_out = DIST_DIR / "QuickNetScan_cli.exe"
    cmd_cli = [go_bin, "build", "-trimpath", "-ldflags", "-s -w", "-o", str(cli_out), "./cmd/quicknetscan-cli"]
    res = subprocess.run(cmd_cli, cwd=ROOT_DIR)
    if res.returncode != 0:
        print("[ERROR] CLI build failed")
        sys.exit(res.returncode)

    # 7. Copy web assets to dist/web for Virtual Host Mapping
    print("\n[7/7] Preparing dist/web for Native Virtual Host Mapping...")
    dist_web = DIST_DIR / "web"
    if dist_web.exists():
        shutil.rmtree(dist_web)
    shutil.copytree(ROOT_DIR / "web" / "static", dist_web)
    # Ensure app.wasm is copied to dist/web
    wasm_inter = INTER_DIR / "app.wasm"
    if wasm_inter.exists():
        shutil.copy2(wasm_inter, dist_web / "app.wasm")

    print("\n========================================")
    print("      BUILD SUCCESSFUL (ZERO HTTP)      ")
    print("========================================")
    print(f"GUI: {gui_out}")
    print(f"DLL: {out_dll}")
    print(f"CLI: {cli_out}")

if __name__ == "__main__":
    main()
