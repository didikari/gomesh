#!/bin/bash
set -e

# GoMesh Agent Linux Installer & systemd service setup
# Usage:
#   curl -sSL http://<server-host>:8080/scripts/install-linux.sh | sudo bash -s -- -s ws://<server-host>:8080/ws/agent -t <token>

SERVER_URL="ws://localhost:8080/ws/agent"
TOKEN="default-token"
INSTALL_DIR="/opt/gomesh"

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -s|--server) SERVER_URL="$2"; shift ;;
        -t|--token) TOKEN="$2"; shift ;;
        -d|--dir) INSTALL_DIR="$2"; shift ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

if [ "$(id -u)" -ne 0 ]; then
    echo "[!] Error: This installation script must be run as root (or via sudo)."
    exit 1
fi

echo "================================================="
echo "   GoMesh Linux Agent Service Installer         "
echo "================================================="
echo "Target Server : $SERVER_URL"
echo "Install Dir   : $INSTALL_DIR"
echo "================================================="

# Create installation directory
mkdir -p "$INSTALL_DIR"

# Detect Architecture
ARCH=$(uname -m)
BIN_NAME="gomesh-agent-linux"
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    echo "[*] Detected ARM64 architecture"
    BIN_NAME="gomesh-agent-linux-arm64"
elif [ "$ARCH" = "x86_64" ]; then
    echo "[*] Detected x86_64 (amd64) architecture"
    BIN_NAME="gomesh-agent-linux"
else
    echo "[*] Architecture $ARCH, defaulting to $BIN_NAME"
fi

HTTP_BASE=$(echo "$SERVER_URL" | sed -e 's/^ws:\/\//http:\/\//' -e 's/^wss:\/\//https:\/\//' -e 's/\/ws\/agent$//')

echo "[*] Fetching agent binary from server ($HTTP_BASE/bin/$BIN_NAME)..."
if curl -sSLf "$HTTP_BASE/bin/$BIN_NAME" -o "$INSTALL_DIR/gomesh-agent" 2>/dev/null; then
    echo "[+] Agent binary downloaded successfully"
elif [ -f "./bin/$BIN_NAME" ]; then
    echo "[*] Using local binary ./bin/$BIN_NAME"
    cp "./bin/$BIN_NAME" "$INSTALL_DIR/gomesh-agent"
elif [ -f "./bin/gomesh-agent" ]; then
    echo "[*] Using local binary ./bin/gomesh-agent"
    cp "./bin/gomesh-agent" "$INSTALL_DIR/gomesh-agent"
else
    echo "[!] Warning: Could not download binary from $HTTP_BASE/bin/$BIN_NAME. Please ensure agent binary exists at $INSTALL_DIR/gomesh-agent"
fi

if [ -f "$INSTALL_DIR/gomesh-agent" ]; then
    chmod 755 "$INSTALL_DIR/gomesh-agent"
fi

# Write systemd service file
SERVICE_PATH="/etc/systemd/system/gomesh-agent.service"
echo "[*] Creating systemd service at $SERVICE_PATH..."

cat <<EOF > "$SERVICE_PATH"
[Unit]
Description=GoMesh Remote Management Agent
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/gomesh-agent -server $SERVER_URL -token $TOKEN -config $INSTALL_DIR
Restart=always
RestartSec=5s
LimitNOFILE=65536
KillMode=process

[Install]
WantedBy=multi-user.target
EOF

# Reload and enable service
echo "[*] Enabling and starting gomesh-agent service..."
systemctl daemon-reload
systemctl enable gomesh-agent.service
systemctl restart gomesh-agent.service

echo ""
echo "[+] GoMesh Agent service installed and active!"
echo "    Check status: systemctl status gomesh-agent"
echo "    View logs   : journalctl -u gomesh-agent -f"
