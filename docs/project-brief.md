# Project Brief — GoMesh

## 1. Project Identity
- Project name: GoMesh
- Domain: Remote Device Management & Systems Administration
- Project type: internal-tool / platform (Self-hosted Remote Administration Tool)
- Stage: new build

## 2. Executive Summary
GoMesh adalah platform open-source modern untuk remote computer management & monitoring yang terinspirasi dari MeshCentral. Dibangun sepenuhnya menggunakan Go (Golang) untuk server maupun agent, GoMesh memberikan solusi berkinerja tinggi, hemat sumber daya, dan berbentuk *single binary* tanpa dependensi eksternal. GoMesh memungkinkan SysAdmin dan teknisi IT untuk mengakses remote terminal (PTY), remote desktop, manajemen file, dan telemetri perangkat secara web-based bahkan di balik firewall/NAT tanpa memerlukan konfigurasi router khusus.

## 3. Problem Statement
- Masalah utama saat ini:
  - Solusi proprietary (TeamViewer, AnyDesk) berbiaya langganan tinggi dan membatasi kontrol data perusahaan.
  - MeshCentral klasik berbasis Node.js memiliki footprint dependensi npm yang besar dan pemeliharaannya lebih kompleks untuk arsitektur modern.
  - Mengakses perangkat target di jaringan berbeda (di balik NAT/firewall) biasanya membutuhkan VPN rumit atau port forwarding publik yang berisiko keamanan.
- Dampak bisnis/operasional:
  - Waktu respons insiden IT dan troubleshoot server/klien lambat.
  - Risiko keamanan meningkat jika membuka port terbuka secara sembarangan.
  - Konsumsi resource RAM/CPU tinggi pada agent jika menggunakan runtime berat.

## 4. Goals
- Primary goals:
  - Menyediakan Central Server dan Device Agent berbasis Go dengan binary mandiri (*zero external runtime*).
  - Mengimplementasikan reverse WebSocket (outbound-only dari agent ke server) yang aman dengan TLS dan token autentikasi.
  - Menghadirkan web management dashboard yang terintegrasi (Web Terminal via xterm.js, Remote Screen View, File Explorer, Device Telemetry).
- Secondary goals:
  - Dukungan cross-platform agent (Linux, Windows, macOS).
  - Single binary deployment untuk server dengan web assets ter-embed (`embed.FS`) dan database SQLite lokal.
  - Audit logging lengkap untuk setiap sesi remote dan transfer file.

## 5. Non-Goals / Out of Scope
- Intel AMT / vPro hardware out-of-band management (di luar scope fase awal; MeshCentral memiliki fitur ini namun sangat kompleks).
- Peer-to-peer WebRTC direct mesh data channels (fase awal fokus pada Server-Relayed WebSocket yang stabil dan deterministic).
- Mobile client app (Android/iOS) pada MVP awal (fokus pada desktop/server agent).

## 6. Target Users
### Primary users
- SysAdmin & DevOps Engineers: Mengelola server headless (Linux), mengeksekusi perintah terminal, dan memantau status server.
- IT Support Specialists: Membantu pengguna desktop (Windows/macOS), remote desktop, dan memindahkan file troubleshooting.

### Secondary users
- Security / Compliance Auditors: Memeriksa log aktivitas sesi remote dan riwayat akses perangkat.

## 7. Core Use Cases
- SysAdmin needs to open an interactive terminal on a remote Linux server behind NAT so that they can debug issues directly from the browser without SSH port exposure.
- IT Support needs to view the client screen and send keyboard/mouse inputs so that they can guide users and resolve software issues remotely.
- Support Engineer needs to browse, upload, and download files on the target machine so that log files can be analyzed or patch scripts deployed.
- Operator needs to see realtime device telemetry (CPU, RAM, disk, OS, IP, online status) so that unhealthy devices can be spotted immediately.

## 8. Platform & Delivery Context
- Target platform: Web Application (Browser Dashboard) + Cross-platform Daemon (Agent: Linux x86_64/arm64, Windows amd64, macOS darwin).
- Usage environment: Self-hosted internal network atau VPS publik, outbound NAT traversal.
- Expected scale: Small to medium (1 - 1,000 active nodes per single Go server instance).

## 9. Required Stack
- Server Backend: Go 1.23+ (`net/http`, router Chi, `gorilla/websocket` / `nhooyr.io/websocket`).
- Server Database: SQLite3 (WAL mode via `modernc.org/sqlite` untuk pure Go / CGO-free).
- Server UI/Assets: Embedded Web SPA/HTML5 (`embed.FS`), Tailwind CSS, xterm.js.
- Agent: Go 1.23+ binary (creack/pty untuk Linux/macOS, Windows ConPTY/winpty untuk Windows terminal, native screen capture).
- Auth & Security: JWT token untuk web session, mutual Pre-Shared Key (PSK) / Device Token untuk Agent connection.

## 10. Constraints & Boundaries
- Reverse Connection: Semua koneksi jaringan dari Agent ke Server bersifat outbound (klien tidak pernah membuka port listen).
- Zero CGO requirement jika memungkinkan agar kompilasi cross-platform Go (`GOOS=windows GOARCH=amd64 go build`) semudah mungkin.
- Memory agent tidak boleh melebihi 25MB RAM saat idle.

## 11. UX & Operational Expectations
- Web UI harus bersih, responsif, cepat (sat set), dan tidak membutuhkan build pipeline rumit saat runtime.
- Latensi terminal harus di bawah 50ms di jaringan stabil.
- Transmisi data terminal dan desktop harus terenkripsi TLS (WSS).

## 12. Data & Security Assumptions
- TLS wajib diaktifkan pada Central Server (bisa via built-in ACME/Let's Encrypt atau di balik reverse proxy Caddy/Nginx).
- Token registrasi agen digenerate dari server dan terikat pada device UUID.
- Sesi remote di-audit ke dalam SQLite database.

## 13. Success Criteria
- Agent berhasil terkoneksi ke server dalam waktu < 2 detik setelah service aktif.
- Web terminal dapat menjalankan bash/sh/PowerShell interaktif dengan resize window yang sinkron.
- File explorer dapat melihat direktori root, mengunduh file, dan mengunggah file tanpa korupsi stream.
- Server berjalan stabil sebagai single executable.

## 14. Open Decisions
- [OPEN DECISION] Mekanisme screen capture lintas OS: Apakah menggunakan library pure Go seperti `kbinani/screenshot` untuk frame streaming awal atau langsung H.264/WebRTC? (Rekomendasi awal: JPEG/WebP frame stream via WebSocket untuk kesederhanaan CGO-free, lalu evolusi ke WebRTC jika performa membutuhkan).
- [OPEN DECISION] Format otentikasi user: Local SQLite user table dengan bcrypt vs OIDC/LDAP. (Rekomendasi awal: Local SQLite auth dengan RBAC admin/operator).
