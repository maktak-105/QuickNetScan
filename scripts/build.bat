@echo off
setlocal
cd /d "%~dp0\.."

python scripts\build.py
if errorlevel 1 (
    echo [ERROR] Build failed.
    exit /b 1
)

echo [SUCCESS] Build completed.
exit /b 0
