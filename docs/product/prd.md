# PRD — GoMesh

## 1. Document Info
- Product: GoMesh
- Owner: Didik Ariyanto / Solution Architect
- Status: approved
- Version: v0.1.0
- Last updated: 2026-10-28

## 2. Product Summary
GoMesh adalah platform open-source untuk *remote computer management & monitoring* berbasis web yang cepat, ringan, dan mandiri, terinspirasi dari MeshCentral. Menggunakan bahasa pemrograman Go untuk Central Server dan Device Agent, sistem ini menghubungkan web console teknisi dengan komputer target melalui tunnel WebSocket dua arah yang aman (outbound-only reverse connection), menembus NAT/firewall tanpa konfigurasi port forwarding.

## 3. Background / Context
- Kondisi saat ini: Pengelolaan perangkat remote di lapangan atau di jaringan privat membutuhkan solusi pihak ketiga berbayar (TeamViewer, AnyDesk) atau konfigurasi VPN/SSH tunnel yang rumit. MeshCentral (Node.js) sangat populer tetapi memiliki dependency footprint besar dan arsitektur legacy.
- Masalah yang ingin diselesaikan: Kebutuhan akan software remote management mandiri (self-hosted), ringan (*low resource footprint*), single binary, cepat (*sat set*), dan handal.
- Kenapa ini penting sekarang: Meningkatkan kemandirian infrastruktur IT, keamanan data internal, dan kecepatan respons teknisi dalam troubleshooting server maupun endpoint karyawan.

## 4. Objectives
- Business objectives:
  - Menyediakan solusi remote management open-source berkinerja tinggi tanpa dependensi eksternal.
  - Menghilangkan ketergantungan pada vendor SaaS komersial pihak ketiga.
- User objectives:
  - Teknisi/SysAdmin dapat mengakses terminal, remote screen, dan file manager dari browser dalam satu klik.
  - Zero-config di sisi target device: cukup pasang binary agent.
- Operational objectives:
  - Distribusi agent sebagai *single static binary* (< 15MB).
  - Server yang hemat memori (< 50MB RAM pada beban normal) dengan database embedded SQLite.

## 5. Non-Objectives
- Hardware-level Intel AMT/vPro out-of-band management di fase v0.1.0.
- Audio redirection / streaming suara perangkat target di fase awal.
- Aplikasi mobile native (Android/iOS) di fase awal (web console responsif diakses dari mobile browser).

## 6. Personas
### Persona 1: Reza (DevOps & Linux Administrator)
- Name/role: Reza / DevOps Engineer
- Needs: Akses shell interaktif ke server headless Linux di AWS/Private Cloud tanpa membuka port SSH publik.
- Pain points: Sering terhambat VPN kantor yang putus-putus atau konfigurasi bastion host yang rumit.
- Success looks like: Membuka GoMesh web dashboard, klik node, dan langsung berinteraksi dengan terminal bash.

### Persona 2: Budi (IT Helpdesk Support)
- Name/role: Budi / Desktop Support Specialist
- Needs: Mengendalikan desktop Windows/macOS user untuk menyelesaikan issue software, mengirim file patch, dan melihat status sistem.
- Pain points: User non-teknis kesulitan mengonfigurasi aplikasi remote atau menyebutkan IP publik mereka.
- Success looks like: Agen sudah berjalan sebagai background service, teknisi langsung terhubung ke layar user.

## 7. Core User Journeys
### Journey 1: Deployment & Registrasi Agen
1. Admin membuka GoMesh Web Console dan menyalin installation command dengan registration token.
2. Admin/Script mengeksekusi installer di perangkat target.
3. Agen Go otomatis men-download binary, membuat system service (systemd / Windows service), dan menginisiasi koneksi outbound TLS WebSocket ke server.
4. Perangkat langsung muncul di daftar "Active Devices" di dashboard.

### Journey 2: Interaksi Remote Terminal
1. Teknisi memilih device online di dashboard dan memilih tab "Terminal".
2. Web UI menginisiasi sesi PTY over WebSocket ke Central Server; Server merutekan ke koneksi agen.
3. Agen membuka PTY (bash/cmd/PowerShell) dan mengalirkan input/output interaktif secara realtime dengan xterm.js di browser.

### Journey 3: Remote File Management
1. Teknisi membuka tab "Files" pada device.
2. Web UI meminta daftar folder/file dari agent.
3. Teknisi dapat menavigasi struktur folder, mengunduh file log, atau mengunggah file update ke direktori target.

## 8. Functional Requirements
### Module: Device Management & Presence
- FR-01: Agen wajib mengirim heartbeat tiap 10 detik via WebSocket.
- FR-02: Server menandai status offline jika heartbeat hilang > 30 detik.
- FR-03: Dashboard menampilkan metadata perangkat: hostname, OS type, architecture, IP internal/eksternal, uptime, dan versi agent.

### Module: Web Terminal (PTY Relay)
- FR-04: Streaming pseudo-terminal dua arah antara browser (xterm.js) dan shell OS target (`/bin/bash` / `powershell.exe`).
- FR-05: Penyesuaian ukuran terminal otomatis (window resize event) dari browser ke agent PTY.
- FR-06: Clean up otomatis resource proses PTY saat sesi ditutup.

### Module: Remote Desktop (Screen & Input)
- FR-07: Agen menangkap frame layar utama (JPEG/WebP stream) dan mengirimkannya ke browser via WebSocket.
- FR-08: Browser merender frame di HTML5 Canvas secara fluid.
- FR-09: Browser menangkap input keyboard & mouse dan meneruskannya ke agent untuk disimulasikan di OS target.

### Module: File Explorer
- FR-10: List directory (nama file, ukuran, permission, tanggal modifikasi).
- FR-11: Download file dari agent ke browser secara streaming chunked binary.
- FR-12: Upload file dari browser ke agent dengan verifikasi integritas.
- FR-13: Operasi file dasar (buat direktori, hapus file/folder).

### Module: Authentication & RBAC
- FR-14: Autentikasi lokal untuk admin dan teknisi menggunakan password hashed (bcrypt).
- FR-15: Pengelolaan token registrasi agen (Agent Key / Enrollment Token).

## 9. Non-Functional Requirements
- Performance:
  - Agent RAM idle < 25MB, CPU idle < 1%.
  - Terminal latency < 50ms pada LAN / < 150ms pada koneksi internet publik.
- Reliability:
  - Auto-reconnect exponential backoff pada agen saat koneksi terputus.
  - Server stateless session multiplexing di memory.
- Security:
  - Wajib enkripsi TLS 1.3 (WSS) untuk semua komunikasi.
  - Proteksi CSRF & JWT yang aman pada web dashboard.
- Accessibility:
  - UI responsif, kontras warna yang nyaman, dukungan keyboard shortcut.
- Auditability:
  - Pencatatan log sesi: waktu sesi, identitas teknisi, target node, dan durasi.
- Maintainability:
  - Pure Go (idiomatic Go, standard library net/http, single binary build).

## 10. Roles and Permissions
- Administrator: Mengelola pengguna, konfigurasi sistem, generate token, dan akses penuh ke seluruh node.
- Operator: Mengakses terminal, desktop, file explorer, dan telemetri perangkat tanpa akses manajemen user.

## 11. Success Metrics
- 100% single binary server & single binary agent tanpa dependency runtime eksternal.
- Agen mampu menembus NAT/firewall secara stabil via outbound WSS.
- Waktu startup server < 1 detik; waktu koneksi awal agen < 2 detik.

## 12. Risks and Constraints
- Risk: Variasi API screen capture dan PTY pada berbagai OS (Linux vs Windows vs macOS).
  - Mitigation: Isolasi logic per OS menggunakan build tags Go (`//go:build windows`, `//go:build linux`, `//go:build darwin`).
- Risk: Bandwidth konsumsi tinggi saat remote desktop.
  - Mitigation: Kompresi JPEG dinamis dan throttling framerate adaptif (10-30 FPS).

## 13. Assumptions
- Agen dijalankan dengan hak akses yang memadai (root / Administrator / user aktif) untuk menjalankan shell dan menangkap layar.
- Server dipasang pada host dengan port HTTP/HTTPS yang dapat dijangkau oleh agen.

## 14. Out of Scope
- Direct peer-to-peer WebRTC mesh connection tanpa relay server (fase MVP fokus pada reliable server relay).
- Manajemen hardware Intel AMT/vPro.

## 15. Acceptance Criteria Summary
- Server Go melayani Web Dashboard dan menerima koneksi WebSocket agent.
- Device agent dapat di-build untuk Linux dan Windows.
- Sesi terminal web xterm.js interaktif berjalan lancar.
- File explorer dapat melihat direktori dan transfer file.

## 16. Open Questions
- [OPEN DECISION] Format default kompresi remote desktop: JPEG stream vs MJPEG vs H.264 WebRTC. (Rekomendasi awal: JPEG chunk over WebSocket untuk kesederhanaan implementasi pure Go).
