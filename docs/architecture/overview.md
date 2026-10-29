# Architecture Blueprint — GoMesh

## 1. Blueprint Summary
- System name: GoMesh
- Architecture style: Modular Monolith (Central Relay Server) + Distributed Lightweight Agent
- Primary stack: Golang 1.23+, WebSockets (TLS), SQLite3 (`modernc.org/sqlite` pure Go), Embedded Web UI (`embed.FS` + Tailwind + xterm.js)
- Key architectural goals:
  - High performance & low latency for terminal and desktop streaming
  - Zero external runtime dependencies (single binary deployment for both server and agent)
  - Resilient reverse connection architecture (outbound TLS WebSocket from agent to server)

## 2. System Context
- Primary actors:
  - Admin & Operator: Mengakses web browser dashboard untuk mengontrol perangkat.
  - Device Agent: Daemon yang berjalan di perangkat target dan membuka reverse tunnel.
- External systems/integrations:
  - TLS / Reverse Proxy (Opsional: Nginx/Caddy atau Go native ACME Let's Encrypt).
  - Target OS subsystem (PTY subsystem, Screen capture API, OS Filesystem).
- Deployment context:
  - Central Server: Linux/Docker VPS atau server lokal dengan 1 port HTTP/HTTPS terbuka.
  - Agent: Windows Service, Linux systemd, atau macOS LaunchDaemon.

## 3. High-Level Architecture
### Major layers
- Presentation Layer: Web Management Console (HTML5/Tailwind/xterm.js/Canvas) di-bundle ke dalam binary Go server via `embed.FS`.
- Relay & Multiplexing Layer: Go WebSocket Hub yang mengarahkan pesan antara browser session dan agent session.
- Business & Control Layer: Device Presence Manager, Session Controller, Auth/RBAC, Token Validator.
- Data Layer: SQLite database (WAL mode) untuk menyimpan user, devices, token, dan audit log.
- Agent Core Layer: Go agent binary yang mengeksekusi PTY, screen capture, file I/O, dan telemetry collection.

### Boundary rules
- Server hanya bertindak sebagai control plane dan data relay; tidak pernah menyimpan payload data terminal/desktop ke disk.
- Komunikasi Agent selalu bersifat *outbound reverse connection* ke Server; Server tidak pernah mencoba membuka koneksi masuk (*inbound*) ke Agent.
- Seluruh streaming binary (terminal raw byte, desktop frame, file chunk) menggunakan message framing khusus pada WebSocket sub-protocol.

```
+-------------------------------------------------------------------+
|                        Web Browser (Admin)                        |
|  [ Dashboard UI ]    [ xterm.js Terminal ]    [ Screen Canvas ]   |
+-------------------------------------------------------------------+
                                  ▲
                                  │ (WSS / HTTPS)
                                  ▼
+-------------------------------------------------------------------+
|                         GoMesh Central Server                     |
|  +------------------+  +--------------------+  +---------------+  |
|  | Web & Auth API   |  | WS Session Hub     |  | SQLite DB     |  |
|  | (embed.FS UI)    |  | (Relay Router)     |  | (WAL mode)    |  |
|  +------------------+  +--------------------+  +---------------+  |
+-------------------------------------------------------------------+
                                  ▲
                                  │ (Outbound WSS Reverse Tunnel)
                                  ▼
+-------------------------------------------------------------------+
|                       GoMesh Device Agent                         |
|  +------------------+  +--------------------+  +---------------+  |
|  | Heartbeat & Info |  | PTY Terminal       |  | Screen & Input|  |
|  | Telemetry        |  | (creack/pty)       |  | & File I/O    |  |
|  +------------------+  +--------------------+  +---------------+  |
+-------------------------------------------------------------------+
```

## 4. Core Modules
### Module: `server/hub`
- Purpose: Mengelola pool koneksi WebSocket aktif dari agen dan sesi browser, serta merutekan paket data.
- Main responsibilities:
  - Registrasi & deregistrasi koneksi agen.
  - Pair session antara browser dan agen (Terminal session, Desktop session, File session).
  - Multiplexing channel data.
- Depends on: `internal/protocol`, `internal/auth`.

### Module: `server/web`
- Purpose: Menyajikan antarmuka pengguna (UI) dan REST API manajemen.
- Main responsibilities:
  - Serve file statis dari `embed.FS`.
  - Endpoint login, logout, device list, device details.
  - Audit log endpoint.
- Depends on: `internal/store`.

### Module: `agent/pty`
- Purpose: Mengontrol pseudo-terminal pada sistem operasi target.
- Main responsibilities:
  - Spawning shell (`/bin/bash`, `sh`, atau `powershell.exe`).
  - Read/Write stream I/O antara PTY dan WebSocket channel.
  - Menangani sinyal window resize (SIGWINCH / ConPTY size).
- Depends on: OS-specific PTY implementation.

### Module: `agent/desktop`
- Purpose: Menangkap gambar layar dan mensimulasikan input remote.
- Main responsibilities:
  - Screen capture frame-by-frame (JPEG encoder).
  - Mouse event injection (move, click, scroll).
  - Keyboard event injection (keystrokes).
- Depends on: OS display APIs.

### Module: `agent/filemgr`
- Purpose: Melayani perintah penjelajahan dan transfer file.
- Main responsibilities:
  - List directory dengan metadata lengkap.
  - Chunked read file (download) dan chunked write file (upload).
  - Create / delete / rename filesystem nodes.
- Depends on: `os` package standard library.

## 5. Domain Model
### Main entities
- `User`: Akun pengguna web console (ID, Username, PasswordHash, Role, TOTPSecret, TOTPEnabled, CreatedAt).
- `Device`: Perangkat yang terdaftar (ID, HardwareUUID, Hostname, OS, Arch, InternalIP, PublicIP, Status, LastSeenAt).
- `EnrollmentToken`: Token pendaftaran agen (ID, TokenString, CreatedBy, ExpiresAt, UsageCount).
- `SessionAudit`: Catatan audit interaksi (ID, UserID, DeviceID, SessionType, StartedAt, EndedAt).

### Key relationships
- `User` 1 -> N `SessionAudit`
- `Device` 1 -> N `SessionAudit`
- `User` 1 -> N `EnrollmentToken`

### Identifier policy
- UUID v4 / ULID string untuk semua entitas (Device ID, Session ID, Token).
- Integer auto-increment hanya untuk internal database sequence jika dibutuhkan.

## 6. Role & Permission Model
- `Admin`: Hak penuh membaca dan mengubah konfigurasi, menambah user, membuat enrollment token, dan membuka sesi remote.
- `Operator`: Hanya memiliki hak melihat daftar device dan membuka sesi remote (terminal, desktop, file).
- Permission strategy: Simple role-based authorization pada middleware HTTP handler.

## 7. Main Workflows
### Workflow: Device Connection & Heartbeat
1. Agent membaca file konfigurasi lokal (`/etc/gomesh/agent.json` atau `%PROGRAMDATA%\GoMesh\agent.json`).
2. Agent melakukan handshake WSS ke `wss://server/ws/agent` dengan header `Authorization: Bearer <token>`.
3. Server memvalidasi token dan hardware UUID, lalu mendaftarkan koneksi agent ke dalam in-memory Hub.
4. Agent mengirim payload spesifikasi sistem (OS, RAM, CPU, IP, Hostname).
5. Agent mengirim ping heartbeat tiap 10 detik; Server memperbarui timestamp `LastSeenAt`.

### Workflow: Interactive Terminal Session
1. User mengklik "Connect Terminal" di Web UI.
2. Browser membuka WebSocket ke `wss://server/ws/terminal?device_id=<uuid>`.
3. Server Hub membuat Session Relay dan mengirimkan sinyal `START_PTY` ke koneksi Agent terkait.
4. Agent men-spawn shell process dalam PTY dan mengonfirmasi ke Server.
5. Server menjembatani byte raw antara browser (xterm.js) dan agent PTY.
6. Saat browser terputus, Server mengirim sinyal `STOP_PTY` dan Agent mematikan subprocess shell.

## 8. Backend Conventions
- Menggunakan standar Go Project Layout:
  - `cmd/gomesh/`: Entry point binary Central Server.
  - `cmd/gomesh-agent/`: Entry point binary Device Agent.
  - `internal/server/`: Logika privat server (hub, handlers, auth, store).
  - `internal/agent/`: Logika privat agent (heartbeat, pty, desktop, filemgr).
  - `internal/protocol/`: Definisi protokol payload dan framing WebSocket bersama.
  - `web/`: Asset frontend (HTML, Tailwind CSS, JS, xterm.js).
- Error Handling: Wajib memeriksa `if err != nil` dan membungkus dengan konteks `fmt.Errorf("gagal ... : %w", err)`.
- Context Passing: Selalu teruskan `ctx context.Context` untuk menangani graceful shutdown dan cancelation.
- Dependency Injection: Teruskan dependensi melalui constructor struct (`NewServer(cfg, store, hub)`).

## 9. Frontend Conventions
- Single-page application ringan tanpa overhead framework raksasa (Vanilla JS modern dengan modul ES6 atau Preact/Alpine + Tailwind CSS).
- Integrasi `xterm.js` via CDN atau file statis ter-embed untuk terminal emulator.
- Canvas render loop untuk remote desktop stream.
- Zero external assets runtime: semua file CSS/JS dibundel dalam Go binary via `//go:embed`.

## 10. Routing Strategy
- Web routes:
  - `/` — Dashboard & daftar device
  - `/login` — Halaman login
  - `/devices/{id}` — Detail device & panel remote
- API routes:
  - `POST /api/v1/auth/login`
  - `GET /api/v1/devices`
  - `POST /api/v1/tokens`
- WebSocket routes:
  - `GET /ws/agent` — Endpoint koneksi reverse tunnel dari agen
  - `GET /ws/terminal` — Tunnel interaktif terminal teknisi
  - `GET /ws/desktop` — Tunnel interaktif remote screen teknisi
  - `GET /ws/files` — Tunnel RPC file manager

## 11. Data & Persistence Strategy
- Main database: SQLite3 via `modernc.org/sqlite` (Pure Go, tanpa kebutuhan GCC / CGO).
- Mode: WAL (Write-Ahead Logging) dengan busy timeout untuk konkurensi optimal.
- Migrasi: File SQL migrasi tertanam yang dijalankan otomatis saat server startup.
- Data retention: Audit log disimpan dengan opsi vacuum periodik.

## 12. Smart Roaming Dual-Network Architecture (Phase 6)
- **Multi-Endpoint Prioritization:**
  - Agen menerima daftar endpoint terurut: `[Priority 1: LAN kantor, Priority 2+: Cloudflare Tunnel / IP Publik WAN]`.
  - Dial timeout cepat (3 detik per endpoint) agar saat perangkat di luar kantor langsung melompat ke koneksi WAN tanpa freeze.
- **Background LAN Prober:**
  - Saat agen aktif di WAN, goroutine di latar belakang memeriksa `GET /api/health` ke endpoint LAN kantor setiap 30 detik.
  - Begitu perangkat kembali ke kantor (terkoneksi Wi-Fi kantor atau router MikroTik), koneksi WAN otomatis ditutup dan beralih ke LAN (latensi sub-milidetik, hemat bandwidth).
- **Multi-VLAN & Router MikroTik Awareness:**
  - Deteksi IP server pada router bertingkat (misal server di `172.16.10.x` dan client di `192.168.20.x`).
  - Web console menyediakan selektor IP LAN aktif (`GET /api/server/network`) dan badge status real-time `[LAN]` vs `[WAN]`.

## 13. Zero-Click Windows Deployment & Service Integration
- **Zero-Click Launcher:**
  - Agen dikompilasi dengan `-ldflags="-H=windowsgui"` agar tidak memunculkan jendela CMD saat di-double-click.
  - Deteksi otomatis lokasi eksekusi: Jika dijalankan di luar `C:\ProgramData\GoMesh`, agen otomatis meng-copy dirinya ke direktori instalasi, mendaftarkan startup service, memicu notifikasi visual Windows, dan meluncurkan background worker tersembunyi (`CREATE_NO_WINDOW`).
- **Resilient Auto-Start:**
  - Tingkat 1: Windows Service API murni (`golang.org/x/sys/windows/svc`).
  - Tingkat 2: Windows Task Scheduler (`schtasks /create /sc onlogon`).
  - Tingkat 3: Windows Registry Run Key (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).
- **Dynamic Binary Configuration Overlay:**
  - Konfigurasi token, endpoints, dan group disematkan langsung di akhir file executable (`<!--GOMESH_CFG_START-->` JSON) saat diunduh via Web Console, sehingga client tidak memerlukan file konfigurasi terpisah.

## 14. Sat-Set Ultra-Low Latency Remote Desktop Engine
- **Fast Screen Change Detection (Dirty Frame Skipping):**
  - Sampling pixel hashing (FNV-1a 64-bit) berjalan dalam < 0.2ms. Jika layar statis, agen tidak melakukan kompresi JPEG dan tidak mengirimkan data, menurunkan konsumsi CPU ke 0% dan mengosongkan antrean TCP network.
  - Begitu ada perubahan (klik/mengetik), frame langsung dikirim tanpa antrean bufferbloat.
- **Off-Thread GPU Decoding:**
  - Browser merender frame JPEG menggunakan API `createImageBitmap()` langsung pada thread rasterizer GPU, menghindari garbage collection freeze dari DOM `new Image()`.
- **Cross-Platform Native Input Simulation:**
  - Windows: Win32 `SetCursorPos`, `mouse_event` dengan `WHEEL_DELTA` (120) terkalibrasi.
  - Linux: Pure Go X11 / XTest via `github.com/jezek/xgb/xtest` + fallback `xdotool`.
  - macOS: CoreGraphics / `cliclick` + fallback universal `osascript` (System Events).
- **Intelligent Mouse Throttling & Normal Cursor:**
  - Kursor menggunakan `cursor-default` panah standar.
  - Throttling `mousemove` 40ms dengan filter threshold 2-3px untuk mencegah TCP buffer congestion.

## 15. Recommended Folder Structure
```text
.
├── cmd/
│   ├── gomesh/             # Main entry point Server
│   │   └── main.go
│   └── gomesh-agent/       # Main entry point Agent
│       ├── main.go
│       ├── selfinstall_windows.go
│       ├── selfinstall_other.go
│       ├── service_windows.go
│       └── service_other.go
├── internal/
│   ├── agent/              # Logika Agent & Smart Roaming
│   │   ├── client.go
│   │   ├── desktop/
│   │   ├── embeddedcfg/
│   │   ├── filemgr/
│   │   ├── pty/
│   │   └── telemetry/
│   ├── protocol/           # Pesan & Framing WebSocket
│   │   └── messages.go
│   ├── server/             # Logika Server
│   │   ├── auth/
│   │   ├── handlers/
│   │   ├── hub/
│   │   └── store/
│   └── version/
├── web/                    # Static Web UI ter-embed
│   ├── embed.go
│   ├── index.html
│   └── static/
├── scripts/                # Script instalasi & embedder
│   ├── install-server.sh   # 1-Line automated server installer (Linux systemd)
│   ├── install-windows.ps1
│   ├── install-linux.sh
│   ├── install-macos.sh
│   └── build-all.sh        # Cross-compile standalone binaries
├── .github/workflows/
│   └── release.yml         # CI/CD automated release & multi-arch Docker image
├── Dockerfile              # Multi-stage lightweight server container (< 20MB)
├── docker-compose.yml      # Zero-dependency compose setup
├── docs/                   # Dokumentasi Foundation Pack
├── go.mod
└── go.sum
```

## 15. Cross-Cutting Concerns
- Logging: Structured logging menggunakan Go standard `log/slog`.
- Error Handling: Sentralisasi error responses pada API dan pemulihan graceful (*recover*) pada WebSocket goroutines.
- Graceful Shutdown: Menangkap sinyal `SIGINT` / `SIGTERM` untuk menutup koneksi WebSocket dan database cleanly.

## 16. Risks
- OS-specific PTY on Windows:
  - ConPTY membutuhkan Windows 10/Server 2019 ke atas.
  - Mitigation: Fallback ke standard pipe I/O dan powershell/cmd jika ConPTY tidak tersedia.
- Screen capture permission on macOS:
  - Mitigation: Deteksi permission dan sediakan panduan Accessibility / Screen Recording di Web UI.

## 17. Security & Hardening Highlights
- Antivirus Clean: Tanpa runtime packer / crypter pihak ketiga (mencegah false-positive Defender).
- Two-Factor Authentication (TOTP RFC 6238): Pembangkitan QR code dan validasi OTP murni standar Go tanpa dependensi eksternal.
- RBAC: Session cookie HTTP-only dengan token acak 256-bit dan proteksi CSRF.
