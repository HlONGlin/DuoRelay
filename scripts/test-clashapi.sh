#!/bin/bash
# Test script for Clash API integration
# This script runs both kernels and tests their Clash API endpoints

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=== DuoRelay Clash API Test ==="
echo ""

# Check if kernels exist
if [ ! -f "kernels/sing-box.exe" ] || [ ! -f "kernels/mihomo.exe" ]; then
    echo "Error: Kernels not found. Run scripts/download-kernels.sh first."
    exit 1
fi

# Test sing-box
echo "1. Testing sing-box Clash API..."
./kernels/sing-box.exe run -c testdata/configs/singbox-minimal.json &
SINGBOX_PID=$!
sleep 2

echo "   Running tests..."
go test -v ./internal/clashapi -run TestSingBoxClashAPI

kill $SINGBOX_PID 2>/dev/null || true
sleep 1

echo ""
echo "2. Testing mihomo Clash API..."
./kernels/mihomo.exe -f testdata/configs/mihomo-minimal.yaml &
MIHOMO_PID=$!
sleep 2

echo "   Running tests..."
go test -v ./internal/clashapi -run TestMihomoClashAPI

kill $MIHOMO_PID 2>/dev/null || true

echo ""
echo "=== All tests completed ==="
