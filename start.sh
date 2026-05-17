#!/bin/bash
set -e
CYAN='\033[0;36m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'; NC='\033[0m'

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   IPMS — Intelligent Profile Monitoring System       ║${NC}"
echo -e "${CYAN}║   Muhammad Abdullah Mujahid | 2022-AG-6620 | UAF     ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v go &>/dev/null; then
  echo -e "${RED}✗ Go not found. Install from: https://golang.org/dl/${NC}"; exit 1
fi
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}') found${NC}"

if ! command -v gcc &>/dev/null; then
  echo -e "${RED}✗ GCC not found.${NC}"
  echo -e "${YELLOW}  Linux:   sudo apt install gcc${NC}"
  echo -e "${YELLOW}  macOS:   xcode-select --install${NC}"; exit 1
fi
echo -e "${GREEN}✓ GCC found${NC}"

echo ""
echo -e "${BOLD}Starting IPMS (frontend + backend on :5000)...${NC}"
echo ""
echo -e "${CYAN}  ➜ Open:  ${BOLD}http://localhost:5000${NC}"
echo -e "${CYAN}  ➜ API:   ${BOLD}http://localhost:5000/api/health${NC}"
echo -e "${CYAN}  ➜ WS:    ${BOLD}ws://localhost:5000/ws${NC}"
echo ""
echo -e "${YELLOW}  Press Ctrl+C to stop${NC}"
echo ""

cd "${SCRIPT_DIR}/go-backend"
go mod tidy -q
go run ./cmd/main.go
