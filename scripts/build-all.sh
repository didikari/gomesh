#!/bin/bash
set -e

# ==============================================================================
# GoMesh Cross-Compilation Script
# Compiles standalone static binaries for Server and Agent across all platforms.
# ==============================================================================

OUTPUT_DIR="dist"
mkdir -p "$OUTPUT_DIR"

LDFLAGS="-s -w"
GUI_LDFLAGS="-s -w -H=windowsgui"

echo "=========================================================="
echo "    📦 Building GoMesh Binaries (Zero CGO / Pure Go)      "
echo "=========================================================="

echo "[1/10] Building Server (Linux amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-linux-amd64" ./cmd/gomesh

echo "[2/10] Building Server (Linux arm64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-linux-arm64" ./cmd/gomesh

echo "[3/10] Building Server (Windows amd64)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-windows-amd64.exe" ./cmd/gomesh

echo "[4/10] Building Server (macOS arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-darwin-arm64" ./cmd/gomesh

echo "[5/10] Building Server (macOS amd64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-darwin-amd64" ./cmd/gomesh

echo "[6/10] Building Agent (Linux amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-agent-linux-amd64" ./cmd/gomesh-agent

echo "[7/10] Building Agent (Linux arm64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-agent-linux-arm64" ./cmd/gomesh-agent

echo "[8/10] Building Agent (Windows amd64 - GUI Mode)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$GUI_LDFLAGS" -o "$OUTPUT_DIR/gomesh-agent-windows-amd64.exe" ./cmd/gomesh-agent

echo "[9/10] Building Agent (macOS arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-agent-darwin-arm64" ./cmd/gomesh-agent

echo "[10/10] Building Agent (macOS amd64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o "$OUTPUT_DIR/gomesh-agent-darwin-amd64" ./cmd/gomesh-agent

echo "[*] Generating Checksums..."
cd "$OUTPUT_DIR"
sha256sum * > checksums.txt 2>/dev/null || shasum -a 256 * > checksums.txt
cd ..

echo "=========================================================="
echo " 🎉 All Binaries Successfully Built in ./$OUTPUT_DIR/"
echo "=========================================================="
ls -lh "$OUTPUT_DIR"
