# Implementation Roadmap — GoMesh

## 1. Roadmap Summary
- Product/project: GoMesh
- Roadmap owner: Didik Ariyanto / Solution Architect
- Planning horizon: 4-6 Phases (Iterative Slices)
- Delivery style: Phased Milestone-Based Delivery

## 2. Delivery Principles
- Deliver thin but production-usable slices: Setiap fase menghasilkan binary yang dapat dijalankan dan diuji.
- Protocol-first development: Definisikan protokol pesan WebSocket terlebih dahulu sebelum menulis logika agent dan server.
- Reverse-connection core early: Validasi arsitektur koneksi agen-ke-server di fase paling awal.

## 3. Recommended Phase Structure

### Phase 0 — Project Initialization & Protocol Definition
Goal: Inisialisasi struktur repo Go, embedded web assets dasar, dan kontrak protokol WebSocket.
- Setup `go.mod` dan folder layout (`cmd/`, `internal/`, `web/`).
- Definisi tipe pesan framing di `internal/protocol/messages.go`.
- Server HTTP skeleton dengan embedded static HTML dasar (`web/embed.go`).
Success criteria:
- Server berhasil dijalankan dan menyajikan halaman landing status di browser.
- Protokol framing memiliki unit test validasi encoder/decoder.

### Phase 1 — Agent Reverse Connection & Device Presence
Goal: Hubungan dua arah antara Agent dan Server dengan heartbeat realtime.
- Implementasi `internal/server/hub` (WebSocket handler untuk agen).
- Implementasi `internal/agent/client.go` (Agent reverse WSS dialer dengan auto-reconnect).
- SQLite store untuk persistence metadata device (hostname, IP, OS, status).
- Web dashboard menampilkan tabel device online/offline realtime.
Success criteria:
- Agent binary berhasil terkoneksi ke Server dan status di web dashboard berubah menjadi "Online" dalam < 2 detik.
- Saat agen dihentikan, server mendeteksi status "Offline" via timeout heartbeat.

### Phase 2 — Web Terminal (Interactive PTY Relay)
Goal: Akses shell interaktif dari browser ke komputer target.
- Integrasi `xterm.js` di web dashboard.
- Implementasi PTY spawner di agen (`creack/pty` untuk Linux/Mac, ConPTY untuk Windows).
- Relay data PTY dua arah di server hub.
- Penanganan resize window terminal (cols/rows).
Success criteria:
- Teknisi dapat mengetik perintah shell di browser dan melihat output secara interaktif dan real-time.

### Phase 3 — Remote File Explorer & Transfer
Goal: Penjelajahan direktori dan transfer file jarak jauh.
- Endpoint protokol RPC file: ListDir, DownloadChunk, UploadChunk, DeleteFile.
- UI file browser di web console (pohon folder dan file list).
- Binary chunk streaming aman via WebSocket.
Success criteria:
- Dapat melihat struktur file remote, men-download file log, dan mengunggah file uji coba tanpa korupsi stream.

### Phase 4 — Remote Desktop (Screen View & Input Injection)
Goal: Menampilkan layar desktop komputer target dan kendali mouse/keyboard.
- Screen capture loop di agen (kompresi frame JPEG).
- HTML5 Canvas renderer di web dashboard.
- Input event capture (mouse move, click, keystroke) dari browser diteruskan ke agen.
Success criteria:
- Layar remote tampil di canvas browser dengan update frame rate stabil (> 10-15 FPS di LAN).
- Klik dan ketikan dari browser terefleksi di desktop target.

### Phase 5 — Hardening, Auth RBAC & Production Readiness
Goal: Pengamanan sistem dan deployment readiness.
- Autentikasi user web (bcrypt password, JWT/session cookie).
- Manajemen Enrollment Token untuk pendaftaran agen baru.
- Service installer script untuk Linux (`systemd`) dan Windows (`sc.exe`).
- Audit logging tersimpan di database.
Success criteria:
- Multi-user RBAC aktif.
- Agent dapat diinstal dengan single command script.

## 4. Module Priority Table
| Module | Priority | Why now | Dependencies |
|---|---|---|---|
| Protocol & Core Hub | P0 | Fondasi seluruh komunikasi WebSocket | Go stdlib, websocket |
| Agent Presence & DB | P0 | Memvalidasi reverse connection & device registry | Protocol, SQLite |
| Web Terminal (PTY) | P0 | Fitur utama paling bernilai dan tercepat untuk remote ops | Hub, Agent Presence |
| File Explorer | P1 | Esensial untuk troubleshooting teknisi | Hub, Agent I/O |
| Remote Desktop | P1 | Fitur visual krusial untuk desktop support | Protocol, Display API |
| Auth & Token RBAC | P1 | Keamanan sebelum produksi | SQLite, JWT |

## 5. Milestones
### Milestone 1 (v0.1.0-alpha) — Foundation & Presence
- Repository diinisialisasi, server & agent terkoneksi via reverse WebSocket.
- Dashboard menampilkan status live device.

### Milestone 2 (v0.2.0-beta) — Interactive Terminal & Files
- Web terminal xterm.js berfungsi penuh di Linux & Windows.
- File manager dasar siap digunakan.

### Milestone 3 (v0.3.0-rc) — Remote Desktop & Complete Auth
- Remote desktop canvas streaming & input control aktif.
- User management dan token enrollment aktif.
