#!/bin/bash
set -e

# ==============================================================================
# GoMesh Central Server - 1-Line Automated Installer for Linux
# Zero Dependencies: No Go compiler, No Docker, No GCC required.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/didikari/gomesh/main/scripts/install-server.sh | sudo bash
#   curl -fsSL https://raw.githubusercontent.com/didikari/gomesh/main/scripts/install-server.sh | sudo bash -s -- -p 9000
# ==============================================================================

PORT=8080
INSTALL_DIR="/opt/gomesh"
VERSION="latest"
REPO="didikari/gomesh"

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -p|--port) PORT="$2"; shift ;;
        -d|--dir) INSTALL_DIR="$2"; shift ;;
        -v|--version) VERSION="$2"; shift ;;
        -r|--repo) REPO="$2"; shift ;;
        *) echo "Parameter tidak dikenal: $1"; exit 1 ;;
    esac
    shift
done

# Pastikan dijalankan sebagai root / sudo
if [ "$(id -u)" -ne 0 ]; then
    echo "[!] Error: Script instalasi ini harus dijalankan sebagai root (gunakan sudo)."
    exit 1
fi

echo "=========================================================="
echo "    🚀 GoMesh Central Server 1-Line Installer             "
echo "=========================================================="
echo "Direktori Instalasi : $INSTALL_DIR"
echo "Port Layanan        : $PORT"
echo "Versi Rilis         : $VERSION"
echo "=========================================================="

# 1. Deteksi Arsitektur CPU
ARCH_RAW=$(uname -m)
ARCH="amd64"
if [ "$ARCH_RAW" = "aarch64" ] || [ "$ARCH_RAW" = "arm64" ]; then
    ARCH="arm64"
elif [ "$ARCH_RAW" = "x86_64" ]; then
    ARCH="amd64"
else
    echo "[*] Arsitektur $ARCH_RAW tidak dikenal, mencoba default amd64..."
    ARCH="amd64"
fi
echo "[+] Terdeteksi arsitektur CPU: linux/$ARCH"

# 2. Siapkan Folder
mkdir -p "$INSTALL_DIR/data"

# 3. Download Binary Pre-compiled
BINARY_NAME="gomesh-linux-${ARCH}"
RELEASE_URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY_NAME}"
if [ "$VERSION" = "latest" ]; then
    RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
fi

echo "[*] Mengunduh binary standalone GoMesh Server..."
DOWNLOAD_SUCCESS=false

if curl -sSLf "$RELEASE_URL" -o "$INSTALL_DIR/gomesh" 2>/dev/null; then
    DOWNLOAD_SUCCESS=true
    echo "[+] Berhasil mengunduh binary rilis dari GitHub Releases."
elif [ -f "./bin/gomesh-linux" ] && [ "$ARCH" = "amd64" ]; then
    echo "[*] Menggunakan binary lokal ./bin/gomesh-linux..."
    cp "./bin/gomesh-linux" "$INSTALL_DIR/gomesh"
    DOWNLOAD_SUCCESS=true
elif [ -f "./bin/gomesh" ]; then
    echo "[*] Menggunakan binary lokal ./bin/gomesh..."
    cp "./bin/gomesh" "$INSTALL_DIR/gomesh"
    DOWNLOAD_SUCCESS=true
fi

if [ "$DOWNLOAD_SUCCESS" = false ]; then
    echo "[!] Binary rilis belum tersedia di GitHub Releases ($RELEASE_URL)."
    echo "[*] Menyiapkan GoMesh Server secara otomatis di latar belakang..."

    TEMP_BUILD_DIR="/tmp/gomesh-bootstrap-$$"
    mkdir -p "$TEMP_BUILD_DIR"

    # Periksa apakah go sudah terpasang di host
    if command -v go >/dev/null 2>&1; then
        echo "[*] Menggunakan Go compiler yang terpasang di sistem..."
        if curl -sSL "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" | tar -C "$TEMP_BUILD_DIR" --strip-components=1 -xz 2>/dev/null; then
            (cd "$TEMP_BUILD_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$INSTALL_DIR/gomesh" ./cmd/gomesh)
            DOWNLOAD_SUCCESS=true
        fi
    else
        # Jika belum ada Go, gunakan compiler portabel sementara di /tmp (hanya untuk build, langsung dihapus)
        echo "[*] Mengunduh runtime compiler portabel sementara ke /tmp..."
        GO_TARBALL="go1.23.2.linux-${ARCH}.tar.gz"
        GO_DL_URL="https://go.dev/dl/${GO_TARBALL}"

        if curl -sSLf "$GO_DL_URL" -o "$TEMP_BUILD_DIR/go.tar.gz" 2>/dev/null; then
            echo "[*] Mengekstrak compiler portabel..."
            tar -C "$TEMP_BUILD_DIR" -xzf "$TEMP_BUILD_DIR/go.tar.gz"
            echo "[*] Mengunduh source code GoMesh..."
            mkdir -p "$TEMP_BUILD_DIR/src"
            if curl -sSL "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" | tar -C "$TEMP_BUILD_DIR/src" --strip-components=1 -xz 2>/dev/null; then
                echo "[*] Mengompilasi GoMesh Server (Pure Go static binary)..."
                (cd "$TEMP_BUILD_DIR/src" && CGO_ENABLED=0 "$TEMP_BUILD_DIR/go/bin/go" build -trimpath -ldflags="-s -w" -o "$INSTALL_DIR/gomesh" ./cmd/gomesh)
                if [ -f "$INSTALL_DIR/gomesh" ]; then
                    DOWNLOAD_SUCCESS=true
                    echo "[+] Kompilasi berhasil! Binary siap digunakan."
                fi
            fi
        fi
    fi

    # Bersihkan direktori sementara
    rm -rf "$TEMP_BUILD_DIR"
fi

if [ "$DOWNLOAD_SUCCESS" = false ]; then
    echo "[X] Fatal: Tidak dapat menyiapkan binary server GoMesh."
    echo "    Silakan periksa koneksi internet server Anda atau rilis di https://github.com/${REPO}/releases"
    exit 1
fi

chmod +x "$INSTALL_DIR/gomesh"

# 4. Daftarkan Service systemd
echo "[*] Mengonfigurasi systemd service (/etc/systemd/system/gomesh.service)..."
cat <<EOF > /etc/systemd/system/gomesh.service
[Unit]
Description=GoMesh Central Remote Management Server
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/gomesh -port ${PORT} -db ${INSTALL_DIR}/data/gomesh.db
Restart=always
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# 5. Jalankan Service
systemctl daemon-reload
systemctl enable gomesh >/dev/null 2>&1 || true
systemctl restart gomesh

# Tunggu 1 detik agar server inisialisasi
sleep 1

# 6. Dapatkan IP Server
IP_LOCAL=$(hostname -I 2>/dev/null | awk '{print $1}' || ip -4 addr show scope global 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -n1 || echo "127.0.0.1")

echo ""
echo "=========================================================="
echo " 🎉 GoMesh Server Berhasil Terpasang & Aktif!"
echo "=========================================================="
echo " Dashboard Web : http://${IP_LOCAL}:${PORT}"
echo " Akun Default  : Username: admin"
echo "                 Password: admin123"
echo "----------------------------------------------------------"
echo " Perintah Berguna:"
echo "   Status Service : sudo systemctl status gomesh"
echo "   Lihat Log      : sudo journalctl -u gomesh -f"
echo "   Restart Server : sudo systemctl restart gomesh"
echo "   Hentikan Server: sudo systemctl stop gomesh"
echo "=========================================================="
echo " Catatan: Segera login dan aktifkan Two-Factor Authentication"
echo " (TOTP 2FA) di Dashboard untuk keamanan maksimal."
echo "=========================================================="
