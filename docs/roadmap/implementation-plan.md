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

### Phase 0 — Project Initialization & Protocol Definition [SELESAI]
Goal: Inisialisasi struktur repo Go, embedded web assets dasar, dan kontrak protokol WebSocket.
- [x] Setup `go.mod` dan folder layout (`cmd/`, `internal/`, `web/`).
- [x] Definisi tipe pesan framing di `internal/protocol/messages.go`.
- [x] Server HTTP skeleton dengan embedded static HTML dasar (`web/embed.go`).
Success criteria:
- [x] Server berhasil dijalankan dan menyajikan halaman landing status di browser.
- [x] Protokol framing memiliki unit test validasi encoder/decoder (`PASS`).

### Phase 1 — Agent Reverse Connection & Device Presence [SELESAI]
Goal: Hubungan dua arah antara Agent dan Server dengan heartbeat realtime.
- [x] Implementasi `internal/server/hub` (WebSocket handler untuk agen).
- [x] Implementasi `internal/agent/client.go` (Agent reverse WSS dialer dengan auto-reconnect).
- [x] SQLite store untuk persistence metadata device (hostname, IP, OS, status).
- [x] Web dashboard menampilkan tabel device online/offline realtime.
Success criteria:
- [x] Agent binary berhasil terkoneksi ke Server dan status di web dashboard berubah menjadi "Online" dalam < 2 detik.
- [x] Saat agen dihentikan, server mendeteksi status "Offline" via timeout heartbeat / disconnect.

### Phase 2 — Web Terminal (Interactive PTY Relay) [SELESAI]
Goal: Akses shell interaktif dari browser ke komputer target.
- [x] Integrasi `xterm.js` di web dashboard (`web/index.html`).
- [x] Implementasi PTY spawner di agen (`creack/pty` untuk Linux/Mac, Windows fallback di `internal/agent/pty/`).
- [x] Relay data PTY dua arah di server hub (`internal/server/hub/`).
- [x] Penanganan resize window terminal dinamis (`cols`/`rows`).
Success criteria:
- [x] Teknisi dapat mengetik perintah shell di browser dan melihat output secara interaktif dan real-time (`PASS` pada unit/integration test).

### Phase 3 — Remote File Explorer & Transfer [SELESAI]
Goal: Penjelajahan direktori dan transfer file jarak jauh.
- [x] Endpoint protokol RPC file: ListDir, DownloadChunk, UploadChunk, DeleteFile (`internal/protocol/messages.go`).
- [x] Subsystem file manager di agen (`internal/agent/filemgr/`).
- [x] UI file browser di web console dengan breadcrumb, upload, download, delete, dan mkdir modal (`web/index.html`).
- [x] Binary chunk streaming aman via REST relay dan WebSocket RPC.
Success criteria:
- [x] Dapat melihat struktur file remote, men-download file log, dan mengunggah file uji coba tanpa korupsi stream (`PASS` pada `TestFileExplorerRPC`).

### Phase 4 — Remote Desktop (Screen View & Input Injection) [SELESAI]
Goal: Menampilkan layar desktop komputer target dan kendali mouse/keyboard.
- [x] Subsystem screen capture di agen dengan kompresi frame JPEG adaptif (`internal/agent/desktop/`).
- [x] HTML5 Canvas renderer & event listener di web dashboard (`web/index.html`).
- [x] Input event relay (mouse move, click, keystroke, wheel) dari browser ke agen (`internal/server/hub/`).
- [x] Mode kontrol View-Only vs Interactive Control serta tombol fullscreen.
Success criteria:
- [x] Layar remote tampil di canvas browser dengan update frame rate stabil (> 10-15 FPS) (`PASS` pada `TestRemoteDesktopStream`).
- [x] Input mouse & keyboard tersambung dan terefleksi ke komputer target.

### Phase 5 — Hardening, Auth RBAC & Production Readiness [SELESAI]
Goal: Pengamanan sistem dan deployment readiness.
- [x] Autentikasi user web (bcrypt password hashing, HTTP-only session cookie `gomesh_session`, session manager in-memory dengan auto-purge).
- [x] RBAC Middleware (`RequireAuth`, `RequireAdmin`) melindungi seluruh endpoint REST API dan WebSocket relay.
- [x] Manajemen Enrollment Token untuk pendaftaran agen baru (validasi token di handshake hello, bypass jika device sudah terdaftar).
- [x] Service installer script:
  - Linux `systemd`: `scripts/install-linux.sh`
  - Windows `sc.exe`: `scripts/install-windows.ps1`
  - macOS `LaunchDaemon`: `scripts/install-macos.sh`
- [x] Audit logging interaksi remote (`session_audits` tabel SQLite untuk terminal dan desktop streaming).
- [x] Antivirus False-Positive Hardening: Tanpa packer (anti-UPX), build flags `-trimpath -ldflags="-s -w"`, pendaftaran service Windows via `sc.exe` standar.
- [x] Add Device / Deployment UI Modal dengan 1-click copy command dan download binary standalone.
- [x] Device Grouping & Filtering: Kolom `group_name` di database, tab filter group dinamis di web dashboard, dialog pemindahan group instan, dan flag `-group` pada agen (`PASS` pada `TestDeviceGrouping`).
- [x] Direct Web Installer Downloads: Endpoint download installer 1-klik langsung dari browser (`/api/download/installer/windows-bat`, `/api/download/installer/linux-sh`, `/api/download/installer/macos-sh`) serta download langsung binary standalone (`gomesh-agent.exe`, `gomesh-agent-linux`, `gomesh-agent`).
- [x] Modern UI Toast & Custom Modal System: Menggantikan seluruh `window.alert()`, `window.confirm()`, dan `window.prompt()` bawaan browser dengan custom floating Toast notifications (`showToast()`), custom confirmation modal (`showConfirmModal()`), dan custom prompt modal (`showPromptModal()`) berdesain dark-mode modern Tailwind CSS.
- [x] Two-Factor Authentication (RFC 6238 TOTP 2FA): Autentikasi 2FA murni standar Go (`crypto/hmac`, `crypto/sha1`) tanpa dependensi berat pihak ketiga. Kompatibel penuh dengan Google Authenticator, Authy, dan Microsoft Authenticator, lengkap dengan QR Code setup (offline data URI PNG), input verifikasi 6 digit, pending 2FA token temporary session, serta UI aktivasi/deaktivasi di Web Console (`PASS` pada `TestTwoFactorAuthenticationFlow`).
- [x] Zero-Config Auto-Running Agent (Binary Overlay): Pengunduhan binary agent langsung dari Web Console dengan konfigurasi server, token, dan group yang otomatis disematkan (*baked-in configuration overlay*). Client cukup men-download dan melakukan double-click pada file `.exe` (atau binary Linux/Mac) tanpa perlu install, tanpa terminal, dan tanpa parameter flags (`PASS` pada `TestOverlayConfigAppendAndRead` & `TestDirectDownloadsAndEmbeddedScripts`).

### Phase 6 — Smart Roaming Dual-Network & Zero-Install Auto-Run [SELESAI]
Goal: Roaming otomatis cerdas antara jaringan lokal kantor (LAN) dan luar kantor (Cloudflare Tunnel / IP Publik).
- [x] Multi-Endpoint Failover Engine (`internal/agent/client.go`): Agen menyimpan daftar endpoint terurut (Priority 1: LAN kantor, Priority 2+: Cloudflare/IP Publik).
- [x] Fast-Failover Handshake (3 detik timeout): Saat agen di rumah/luar kantor, koneksi ke IP LAN lokal kantor gagal cepat dalam 3 detik dan langsung melompat ke Cloudflare/IP Publik tanpa freeze.
- [x] Background Prober Otomatis: Saat agen terhubung via internet (WAN), prober di latar belakang memeriksa `GET /api/health` ke endpoint LAN kantor setiap 30 detik. Saat user membawa laptop kembali ke kantor, agen secara otomatis mendeteksi LAN lokal dan beralih langsung ke mode LAN berkecepatan tinggi (60 FPS & hemat kuota).
- [x] Network Discovery API (`GET /api/server/network` & `GET /api/health`): Server otomatis mengenali alamat IP LAN non-loopback dan mengeksposnya ke Web Console.
- [x] Dynamic Dual-Network Binary Embedding: Binary `.exe` yang diunduh langsung disematkan endpoint LAN dan endpoint Cloudflare sekaligus.
- [x] Web Console Monitoring Badge (`[LAN]` vs `[WAN]`): Tabel perangkat secara real-time menampilkan badge koneksi (`🟢 LAN` jika terhubung langsung dari kantor, `🌐 WAN` jika terhubung dari luar kantor via internet).

Success criteria:
- [x] Multi-user RBAC aktif (`PASS` pada `TestSessionManagerLifecycle` & `TestAuthMiddleware`).
- [x] Pendaftaran agen dan validasi enrollment token terverifikasi (`PASS` pada `TestAuthAndTokenEnrollment`).
- [x] Smart Roaming dual-network teruji dan tervalidasi (`PASS` pada `TestClientEndpointFailover` & `TestDirectDownloadsAndEmbeddedScripts`).
- [x] Agent dapat langsung dijalankan dengan double click `.exe` tanpa CMD, otomatis beralih LAN/WAN sesuai lokasi user.

### Phase 7 — Zero-Go Server Deployment & Production Packaging [SELESAI]
Goal: Memungkinkan deployment GoMesh Central Server di VPS Linux / Docker tanpa perlu meng-install compiler Go sama sekali.
- [x] 1-Line Automated Server Installer (`scripts/install-server.sh`): Auto-detect OS/Arch, unduh binary pre-compiled, auto-setup systemd service, auto-start on boot, dan print link dashboard dalam < 5 detik.
- [x] Multi-Stage Dockerfile & docker-compose.yml: Image container ultra-ringan berbasis Alpine (< 20MB) dengan auto-restart dan volume SQLite persistent.
- [x] GitHub Actions CI/CD Pipeline (`.github/workflows/release.yml`): Otomatisasi cross-compile multi-platform binary (Linux amd64/arm64, Windows amd64, macOS arm64/amd64) dan build/push container image multi-arch ke GitHub Container Registry (`ghcr.io/didikari/gomesh`).
- [x] Cross-Compilation Tooling (`scripts/build-all.sh`): Skrip 1-klik untuk mengompilasi seluruh varian Server dan Agent ke folder `dist/` lengkap dengan checksum SHA256.

Success criteria:
- [x] User Linux VPS dapat menjalankan server dengan 1 baris perintah `curl ... | sudo bash` tanpa menyentuh compiler Go.
- [x] User Docker dapat menjalankan `docker run` atau `docker compose up -d` langsung dari image container.

### Phase 8 — High-Performance Remote Desktop ("Sat-Set" Engine) [SELESAI]
Goal: Mengoptimalkan performa remote desktop agar sangat responsif, lancar, dan mendukung native input di Windows, Linux, dan macOS.
- [x] FNV-1a Fast Frame Differencing (`internal/agent/desktop/capture.go`): Melewatkan frame yang identik (< 0.2ms overhead), memangkas beban CPU dan bandwidth jaringan hingga 80%.
- [x] Dynamic Frame Pacing & Quality Tuning: Penurunan encode quality ke 45 dan target 25 FPS dengan sleep dinamis 10-15ms agar I/O socket dan handling input tidak terblokir.
- [x] Off-Thread GPU Image Decoding (`web/index.html`): Menggantikan `img.src = URL.createObjectURL` dengan `createImageBitmap(blob)` untuk rendering asinkronus langsung di kartu grafis tanpa jank di main UI thread.
- [x] Cross-Platform Calibrated Native Input:
  - Windows: Perbaikan scroll wheel (`WHEEL_DELTA` 120, DWORD casting, sinkronisasi `SetCursorPos` sebelum wheel) dan implementasi injeksi keyboard via Win32 `KEYEVENTF_UNICODE` & Virtual Key mapping untuk seluruh tombol khusus.
  - Linux: Pure-Go X11 / XTest input injector via `github.com/jezek/xgb/xtest` dengan fallback CLI `xdotool` termasuk penanganan keyboard keydown/keyup.
  - macOS: Driver AppleScript (`osascript`) dan CLI `cliclick` untuk native mouse, keyboard, dan scroll injection.
- [x] Default Natural Pointer & Throttled Mousemove: Mengubah kursor canvas dari `+` (crosshair) menjadi arrow standar, membatasi frekuensi mousemove ke 50ms dengan koordinat integer bulat, dan menambahkan toast reminder jika mengklik saat mode View-Only.
- [x] Windows Path Normalization in File Explorer: Normalisasi path Windows ke forward slash (`filepath.ToSlash`) dan escaping backslash di browser UI untuk mengatasi string escape bug saat membuka direktori Windows.

### Phase 9 — Device Lifecycle & Anti-Duplicate Hardware Identity [SELESAI]
Goal: Manajemen siklus hidup perangkat lengkap dengan penghapusan perangkat dan pencegahan duplikasi komputer target.
- [x] Device Deletion Subsystem:
  - SQLite Store: `DeleteDevice(ctx, id)` menghapus rekaman perangkat dan cascade membersihkan data `session_audits`.
  - Hub Session Teardown: `DisconnectDevice(deviceID)` menutup paksa seluruh koneksi WebSocket aktif (agent, terminal, desktop).
  - REST Endpoint: `DELETE /api/devices/{id}` terproteksi RBAC middleware.
  - Web Console UI: Tombol hapus (icon trash merah) di setiap baris tabel perangkat dengan modal konfirmasi interaktif Tailwind (`showConfirmModal`) dan notifikasi toast.
- [x] Deterministic OS Hardware Identity (`internal/agent/machineid/`):
  - Windows: Membaca `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`.
  - Linux: Membaca `/etc/machine-id` atau `/var/lib/dbus/machine-id`.
  - macOS: Membaca `IOPlatformUUID` via IORegistry.
  - Menjamin satu komputer fisik selalu memiliki satu `HardwareUUID` unik, terlepas dari direktori atau cara agent dijalankan.
- [x] Single-Instance Process Mutex:
  - Windows: Mutex tingkat sistem `Global\GoMeshAgentSingleInstanceLock` via Win32 `CreateMutexW`.
  - Unix: Advisory file lock (`flock`) pada `/tmp/gomesh-agent.lock`.
  - Mencegah lebih dari 1 proses agent berjalan bersamaan di komputer klien yang sama.
- [x] Windows Interactive Session Auto-Start:
  - Mengalihkan auto-start dari Session 0 Service (`sc.exe`) ke Task Scheduler (`schtasks.exe /sc onlogon`) dan Registry Run key, sehingga screen capture display aktif dan Remote Desktop tidak lagi terhalang isolasi Session 0.
- [x] Agent Uninstall Command (`-uninstall` flag):
  - Perintah 1-klik `gomesh-agent.exe -uninstall` untuk membersihkan seluruh service, scheduled task, registry key, dan file konfigurasi secara otomatis dari komputer target.

### Phase 10 — Security Hardening: Dynamic Credentials & Self-Service Password Management [SELESAI]
Goal: Menghilangkan hardcoded credentials default pada UI login dan menyediakan self-service ganti password.
- [x] Eliminasi kredensial login statis di frontend (`web/index.html`):
  - Menghapus atribut `value="admin"` dan `value="admin123"` pada input form login.
  - Menambahkan `autocomplete="username"` dan `autocomplete="current-password"`.
- [x] Backend API Ganti Password (`POST /api/auth/password`):
  - Memverifikasi kata sandi lama via bcrypt sebelum mengizinkan pembaruan.
  - Memvalidasi panjang kata sandi baru (minimal 6 karakter).
  - Mengupdate hash password di database SQLite (`UpdateUserPassword`).
- [x] UI Modal Ganti Password di Web Console:
  - Tombol akses cepat (ikon kunci) di header navigation.
  - Modal interaktif dengan validasi kesesuaian kata sandi baru dan umpan balik realtime.
  - Integrasi listener keyboard shortcut (`Escape` close).
- [x] Unit Testing & Regresi Autentikasi:
  - Menambahkan test suite `TestChangePasswordFlow` untuk memvalidasi proteksi unauthorized, invalid current password, dan keberhasilan rotasi credential.
- [x] Eliminasi Flash of Unauthenticated Content (FOUC):
  - Fast pre-render auth state detection di `<head>` via `localStorage` dan CSS root class (`html.has-auth`).
  - Mencegah layar login berkedip saat dashboard di-reload atau di-refresh oleh pengguna yang telah login.

### Phase 11 — Native Go Modular Multi-Page Web Console Refactor [SELESAI]
Goal: Merombak file monolitik 2.600+ baris `web/index.html` menjadi arsitektur modular multi-page dengan server-side session guard dan zero reload flicker.
- [x] Pemisahan Halaman (Multi-Page Server Routing):
  - `web/login.html`: Halaman autentikasi khusus (login form, TOTP 2FA step, error banner).
  - `web/dashboard.html`: Halaman dashboard operasional khusus (navbar, stat cards, device table, dialog modals). Bebas dari tag login screen.
- [x] Server-Side Authentication Guard di Go (`cmd/gomesh/main.go`):
  - `GET /`: Jika sesi belum ada/tidak valid, otomatis redirect HTTP 302 ke `/login`. Jika sesi valid, menyajikan `dashboard.html`. Menjamin **0% kemungkinan kedipan login screen saat dashboard di-reload**.
  - `GET /login`: Jika pengguna sudah login, otomatis redirect HTTP 302 ke `/`.
- [x] Modularisasi Aset Statis (`web/static/`):
  - `css/app.css`: Custom animations and xterm viewport styling.
  - `js/utils.js`: Toast notifications, custom confirm & prompt dialogs, formatting helpers (`formatBytes`, `timeAgo`).
  - `js/auth.js`: Logout, change password modal, and 2FA TOTP setup/disable.
  - `js/terminal.js`: Interactive terminal lifecycle (xterm.js instance, PTY WebSocket relay, dynamic window resizing).
  - `js/desktop.js`: Remote desktop streaming (canvas, image decoding, GPU bitmap acceleration, mouse/keyboard input injection).
  - `js/filemgr.js`: Remote file explorer (safe index-based directory navigation, breadcrumb, chunked upload/download, mkdir, delete).
  - `js/app.js`: Main dashboard controller, periodic polling, device filtering & grouping, token installer helpers, and global shortcut listeners.
- [x] Pure Go Embedding:
  - `web/embed.go` menggunakan `//go:embed *.html static/*` sehingga seluruh halaman dan file statis tetap menyatu dalam single binary executable tanpa dependensi luar.
- [x] Integration Testing:
  - Menambahkan test suite `TestMultiPageRoutingAndStaticAssets` untuk memvalidasi proteksi redirect 302 dan penyajian file statis.

### Phase 12 — Mobile & Responsive Cross-Device UI Modernization [SELESAI]
Goal: Menghadirkan antarmuka pengguna yang sepenuhnya responsif dan intuitif pada berbagai ukuran layar (ponsel pintar/smartphone <640px, tablet 640px–1024px, hingga layar monitor desktop lebar >1024px).
- [x] Top Navigation Bar Responsiveness (`web/dashboard.html`):
  - Label teks panjang tombol tindakan ("Download Agent", "Add Device", "Audit Logs") otomatis dikondensasikan menjadi ikon tombol sentuh yang rapi pada layar kecil (`hidden md:inline`, `hidden sm:inline`, `hidden lg:inline`).
  - Badge versi dan subtitle deskripsi otomatis tersembunyi secara adaptif agar tidak menyebabkan *horizontal overflow*.
  - Username dan user menu dioptimalkan dengan pemangkasan teks (`truncate`) dan *touch target* minimal 36px–44px.
- [x] Grid Metrik Adaptif 2x2 pada Mobile:
  - Grid statistik menggunakan tata letak `grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4`, menghemat ruang vertikal layar smartphone dan meningkatkan densitas informasi tanpa scrolling berlebih.
- [x] Horizontal Touch Scroll Tabs & Responsif Search:
  - Tab navigasi filter grup perangkat menggunakan kontainer swipe sentuh horizontal (`overflow-x-auto no-scrollbar whitespace-nowrap`).
  - Input pencarian perangkat otomatis melebar 100% pada mobile dan mengalir rapi di bawah tab filter.
  - Ukuran font input disesuaikan menjadi `text-base sm:text-xs` untuk mencegah iOS Safari melakukan auto-zoom saat fokus.
- [x] Progressive Disclosure Table Kolom Perangkat (`web/dashboard.html` & `web/static/js/app.js`):
  - Pada layar ponsel kecil, kolom-kolom sekunder seperti Architecture (`hidden lg:table-cell`), Network IP (`hidden md:table-cell`), Telemetry (`hidden sm:table-cell`), dan Last Seen (`hidden xl:table-cell`) disembunyikan secara bersih.
  - Informasi penting (IP internal dan badge group) ditampilkan secara cerdas di bawah nama Hostname pada tampilan seluler sehingga tidak ada informasi yang hilang.
  - Tombol aksi remote (Desktop, Files, Shell, Delete) beradaptasi menjadi tombol ikon ergonomis dengan *touch targets* yang nyaman di jari.
- [x] Dialog & Modal Responsif:
  - Terminal interaktif dan File Explorer otomatis beralih menjadi tampilan layar penuh (`h-full sm:h-[80vh] rounded-none sm:rounded-xl`) pada smartphone untuk memaksimalkan area kerja teknisi remote.
  - Modal remote desktop canvas menyesuaikan rasio aspek dan touch container secara fleksibel.
  - Banner notifikasi toast otomatis melebar proporsional di sisi atas layar ponsel (`fixed top-3 right-3 left-3 sm:left-auto sm:right-5`).
- [x] Clean CSS Styling:
  - Menambahkan kelas utilitas `.no-scrollbar` dan penghapusan `-webkit-tap-highlight-color` di `web/static/css/app.css` untuk sentuhan responsif alami pada perangkat seluler.

### Phase 13 — Dynamic Release Versioning & Build Injection [SELESAI]
Goal: Menghilangkan hardcode versi statis di HTML dan API, menghubungkan nomor rilis aplikasi secara dinamis dari Git tag / GitHub Releases / Go build ldflags ke tampilan web console dan agen.
- [x] Modul Sentral [`internal/version`](file:///Users/didikariyanto/Projects/Mesh/internal/version/version.go):
  - Mendeteksi versi secara hierarkis: `-ldflags "-X gomesh/internal/version.Version=vX.Y.Z"` (prioritas utama saat rilis CI/CD) -> `runtime/debug.ReadBuildInfo()` -> `git describe --tags --always` saat development lokal -> fallback version.
  - Menyediakan helper `version.Get()` dan `version.Info()` untuk server dan agent node.
- [x] Injeksi Dinamis ke HTML Web Console (`web/dashboard.html` & `web/login.html`):
  - Fungsi `serveHTML()` pada Go HTTP server secara otomatis menggantikan placeholder `{{APP_VERSION}}` dengan versi aktif saat menyajikan file HTML ke browser tanpa overhead template engine berat.
  - Navbar dan footer dashboard serta kartu login menampilkan badge versi dinamis yang selalu sinkron dengan rilis resmi.
- [x] Integrasi Endpoint API:
  - `GET /api/version`: Mengembalikan metadata versi, hash commit git, dan build timestamp dalam format JSON.
  - `GET /api/health`: Memperbarui field `"version"` menggunakan `version.Get()`.
  - Agen node (`gomesh-agent`) otomatis mengirimkan `version.Get()` saat handshake koneksi WebSocket, memastikan tabel database perangkat merekam versi agent yang akurat.
- [x] GitHub Actions Workflow CI/CD (`.github/workflows/release.yml`):
  - Memperbarui build flags kompilasi rilis multi-arsitektur dengan `-X gomesh/internal/version.Version=${RELEASE_TAG} -X gomesh/internal/version.GitCommit=${GIT_COMMIT} -X gomesh/internal/version.BuildDate=${BUILD_DATE}` sehingga binary rilis GitHub selalu memiliki tag versi rilis yang tepat secara otomatis.
- [x] CLI Flag `-version`:
  - Perintah `./gomesh -version` dan `./gomesh-agent -version` mencetak versi aktif aplikasi ke terminal.

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
