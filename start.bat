@echo off
title IPMS — Intelligent Profile Monitoring System
echo.
echo  ╔══════════════════════════════════════════════════════╗
echo  ║   IPMS — Intelligent Profile Monitoring System       ║
echo  ║   Muhammad Abdullah Mujahid ^| 2022-AG-6620 ^| UAF     ║
echo  ╚══════════════════════════════════════════════════════╝
echo.

where go  >nul 2>&1 || (echo [ERROR] Go not found. Download: https://golang.org/dl/ & pause & exit /b 1)
where gcc >nul 2>&1 || (echo [ERROR] GCC not found. Get TDM-GCC: https://jmeubank.github.io/tdm-gcc/ & pause & exit /b 1)

echo [INFO] Starting IPMS on http://localhost:5000 ...
echo [INFO] Frontend + API served together — no npm needed
echo.

cd /d "%~dp0go-backend"
go mod tidy
go run ./cmd/main.go
