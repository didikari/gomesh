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
- `User`: Akun pengguna web console (ID, Username, PasswordHash, Role, CreatedAt).
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

## 12. Recommended Folder Structure
```text
.
├── cmd/
│   ├── gomesh/             # Main entry point Server
│   │   └── main.go
│   └── gomesh-agent/       # Main entry point Agent
│       └── main.go
├── internal/
│   ├── agent/              # Logika Agent
│   │   ├── client.go
│   │   ├── desktop/
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
│       ├── css/
│       └── js/
├── docs/                   # Dokumentasi Foundation Pack
├── go.mod
└── go.sum
```

## 13. Cross-Cutting Concerns
- Logging: Structured logging menggunakan Go standard `log/slog`.
- Error Handling: Sentralisasi error responses pada API dan pemulihan graceful (*recover*) pada WebSocket goroutines.
- Graceful Shutdown: Menangkap sinyal `SIGINT` / `SIGTERM` untuk menutup koneksi WebSocket dan database cleanly.

## 14. Risks
- OS-specific PTY on Windows:
  - ConPTY membutuhkan Windows 10/Server 2019 ke atas.
  - Mitigation: Gunakan fallback ke standard pipe I/O jika ConPTY tidak tersedia di Windows legacy.
- Screen capture permission on macOS / Wayland:
  - Mitigation: Dokumentasikan requirement permission dan sediakan deteksi status environment di agent.

## 15. Open Decisions
- [OPEN DECISION] Format frame desktop: Pengiriman diff bounding-box JPEG vs full frame JPEG. (Rekomendasi awal: Full frame JPEG dengan adaptive framerate 15 FPS untuk fase 1).
