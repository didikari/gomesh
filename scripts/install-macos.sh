#!/bin/bash
set -e

# GoMesh Agent macOS Installer & LaunchDaemon setup
# Usage:
#   curl -sSL http://<server-host>:8080/scripts/install-macos.sh | sudo bash -s -- -s ws://<server-host>:8080/ws/agent -t <token>

SERVER_URL="ws://localhost:8080/ws/agent"
ENDPOINTS=""
TOKEN="default-token"
GROUP="Default"
INSTALL_DIR="/usr/local/gomesh"
PLIST_PATH="/Library/LaunchDaemons/com.gomesh.agent.plist"

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -s|--server) SERVER_URL="$2"; shift ;;
        -e|--endpoints) ENDPOINTS="$2"; shift ;;
        -t|--token) TOKEN="$2"; shift ;;
        -g|--group) GROUP="$2"; shift ;;
        -d|--dir) INSTALL_DIR="$2"; shift ;;
        -u|--uninstall)
            echo "[*] Unloading and removing LaunchDaemon..."
            launchctl unload -w "$PLIST_PATH" 2>/dev/null || true
            rm -f "$PLIST_PATH"
            echo "[+] GoMesh agent service uninstalled."
            exit 0
            ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

if [ "$(id -u)" -ne 0 ]; then
    echo "[!] Error: Root or sudo privileges required."
    exit 1
fi

echo "================================================="
echo "   GoMesh macOS Agent Service Installer          "
echo "================================================="
echo "Target Server : $SERVER_URL"
echo "Install Dir   : $INSTALL_DIR"
echo "================================================="

mkdir -p "$INSTALL_DIR"

HTTP_BASE=$(echo "$SERVER_URL" | sed -e 's/^ws:\/\//http:\/\//' -e 's/^wss:\/\//https:\/\//' -e 's/\/ws\/agent$//')

DL_PARAMS="token=${TOKEN}&group=${GROUP}"
if [ -n "$ENDPOINTS" ]; then
    DL_PARAMS="${DL_PARAMS}&endpoints=${ENDPOINTS}"
else
    DL_PARAMS="${DL_PARAMS}&server=${SERVER_URL}"
fi

echo "[*] Fetching agent binary ($HTTP_BASE/bin/gomesh-agent)..."
if curl -sSLf "$HTTP_BASE/bin/gomesh-agent?$DL_PARAMS" -o "$INSTALL_DIR/gomesh-agent" 2>/dev/null; then
    echo "[+] Agent binary downloaded successfully"
elif [ -f "./bin/gomesh-agent" ]; then
    echo "[*] Using local binary ./bin/gomesh-agent"
    cp "./bin/gomesh-agent" "$INSTALL_DIR/gomesh-agent"
else
    echo "[!] Warning: Binary not found at $HTTP_BASE/bin/gomesh-agent. Ensure binary exists at $INSTALL_DIR/gomesh-agent"
fi

if [ -f "$INSTALL_DIR/gomesh-agent" ]; then
    chmod 755 "$INSTALL_DIR/gomesh-agent"
fi

# Stop existing daemon if loaded
launchctl unload -w "$PLIST_PATH" 2>/dev/null || true

# Prepare ProgramArguments XML items
PROG_ARGS="        <string>$INSTALL_DIR/gomesh-agent</string>\n        <string>-config</string>\n        <string>$INSTALL_DIR</string>"
if [ -n "$SERVER_URL" ]; then
    PROG_ARGS="${PROG_ARGS}\n        <string>-server</string>\n        <string>$SERVER_URL</string>"
fi
if [ -n "$ENDPOINTS" ]; then
    PROG_ARGS="${PROG_ARGS}\n        <string>-endpoints</string>\n        <string>$ENDPOINTS</string>"
fi
if [ -n "$TOKEN" ]; then
    PROG_ARGS="${PROG_ARGS}\n        <string>-token</string>\n        <string>$TOKEN</string>"
fi
if [ -n "$GROUP" ]; then
    PROG_ARGS="${PROG_ARGS}\n        <string>-group</string>\n        <string>$GROUP</string>"
fi

# Write LaunchDaemon plist
cat <<EOF > "$PLIST_PATH"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gomesh.agent</string>
    <key>ProgramArguments</key>
    <array>
$(printf "$PROG_ARGS")
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>$INSTALL_DIR</string>
    <key>StandardOutPath</key>
    <string>/var/log/gomesh-agent.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/gomesh-agent.err</string>
</dict>
</plist>
EOF

chmod 644 "$PLIST_PATH"
chown root:wheel "$PLIST_PATH"

echo "[*] Loading LaunchDaemon..."
launchctl load -w "$PLIST_PATH"

echo ""
echo "[+] GoMesh Agent LaunchDaemon installed and running!"
echo "    Check logs: tail -f /var/log/gomesh-agent.log"
