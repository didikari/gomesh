# Engineering Guidelines — GoMesh

## 1. Purpose
Dokumen ini menjadi standar rekayasa (*engineering guidelines*) utama untuk pengembangan GoMesh, memastikan kode berkinerja tinggi, idiomatik, aman, dan mudah dipelihara.

## 2. Repo Principles
- Single Binary Simplicity: Baik Central Server maupun Device Agent harus dapat dikompilasi menjadi satu file biner mandiri tanpa runtime dependencies eksternal (CGO-free bila memungkinkan).
- Outbound-First Networking: Seluruh komunikasi agen ke server wajib diinisiasi secara outbound untuk menjamin tembus firewall/NAT secara alami.
- Sat-Set & Resource Efficient: Zero runtime bloat, konsumsi RAM rendah (< 25MB pada agent), dan respon latensi minimal.

## 3. Architecture Discipline
- Jangan menaruh business logic berat di HTTP controller atau WebSocket packet handler; pisahkan ke dalam domain services.
- Ikuti boundary layer yang disepakati pada Architecture Blueprint.
- Hindari abstraksi dini atau hirarki antarmuka (*interface*) berlebihan; buat interface hanya ketika ada minimal dua implementasi nyata atau untuk kebutuhan unit testing (mocking I/O).
- Dilarang membuat helper/utility generic baru tanpa alasan yang terukur.

## 4. Folder Conventions
- Backend Server: Terletak di `cmd/gomesh` dan `internal/server/`.
- Device Agent: Terletak di `cmd/gomesh-agent` dan `internal/agent/`.
- Shared Protocol: Terletak di `internal/protocol/` (tipe pesan dan framing WebSocket).
- Web Frontend: Terletak di `web/` dan diekspos melalui Go `embed.FS`.

## 5. Naming Conventions
- Packages: Huruf kecil satu kata (`hub`, `pty`, `auth`, `store`).
- Interfaces: Diakhiri dengan suffix "-er" jika memungkinkan (`Reader`, `Relayer`, `Executor`).
- Structs & Functions: Menggunakan CamelCase / PascalCase untuk yang diekspor (`SessionHub`, `StartPTY`), dan camelCase untuk internal package (`handlePacket`).
- Error Variables: Menggunakan prefix `Err` (`ErrDeviceNotFound`, `ErrUnauthorized`).
- Database Tables & Columns: Menggunakan snake_case (`devices`, `created_at`, `hardware_uuid`).

## 6. Backend Rules (Go Best Practices)
- Error Handling Terang-terangan: Dilarang keras mengabaikan error menggunakan blank identifier (`_ = doSomething()`). Setiap error wajib diperiksa dan dibungkus dengan konteks:
  ```go
  if err != nil {
      return fmt.Errorf("gagal menginisialisasi sesi pty: %w", err)
  }
  ```
- Context Passing: Setiap fungsi yang melakukan operasi I/O, database, atau network streaming wajib menerima `ctx context.Context` sebagai argumen pertama.
- Goroutine Safety & Leak Prevention:
  - Setiap goroutine yang dijalankan harus memiliki mekanisme penghentian yang pasti (channel sinyal stop, `ctx.Done()`, atau penutupan koneksi).
  - Gunakan `sync.Mutex` atau `sync.RWMutex` untuk melindungi shared in-memory state pada session hub.
  - Jalankan `go test -race ./...` secara berkala.
- Dependency Injection: Masukkan dependensi (database pool, hub, logger) melalui constructor struct (`NewHub()`), hindari global variables.

## 7. Frontend Rules
- Web assets harus di-minify dan siap di-serve langsung via `embed.FS`.
- UI harus menyediakan state visual yang jelas: Loading state, Offline state, Disconnected banner, dan Error toast.
- Terminal UI wajib menggunakan `xterm.js` dengan addon `fit` dan `web-links`.
- Remote desktop canvas harus menangani event resize dan scaling aspect-ratio secara otomatis.

## 8. Data & Database Rules
- Driver Database: Menggunakan `modernc.org/sqlite` untuk menjaga kompilasi Go tetap pure Go tanpa GCC/CGO.
- Mode WAL: SQLite wajib dijalankan dengan `PRAGMA journal_mode=WAL;` dan `PRAGMA busy_timeout=5000;`.
- Identifier: Gunakan UUID v4 string untuk Device ID dan Session ID untuk mencegah enumerasi ID.
- Migrasi: Gunakan embedded SQL scripts sederhana yang dijalankan saat startup dengan mekanisme versi `schema_version`.

## 9. Dependency Policy
- Diizinkan secara default:
  - Standard library Go (`net/http`, `crypto`, `context`, `sync`, `embed`, `log/slog`).
  - `modernc.org/sqlite` (Pure Go SQLite driver).
  - `github.com/gorilla/websocket` atau `nhooyr.io/websocket`.
  - `github.com/creack/pty` (Unix PTY).
  - `golang.org/x/crypto` (bcrypt untuk hashing password).
- Memerlukan persetujuan eksplisit:
  - Penambahan library screen capture yang membutuhkan CGO.
  - Framework web besar (Gin, Fiber, Echo) jika `net/http` + Chi sudah memadai.

## 10. Testing Policy
- Unit Testing: Seluruh logika parsing protokol dan message framing di `internal/protocol` wajib memiliki unit test.
- Integration Testing: Verifikasi relasi WebSocket hub dan mock agent connection.
- Minimum verification commands:
  - Format: `go fmt ./...`
  - Vet/Lint: `go vet ./...`
  - Test: `go test -v -race ./...`
  - Build Check: `go build ./cmd/gomesh && go build ./cmd/gomesh-agent`

## 11. Code Review Expectations
- Correctness: Apakah koneksi websocket tertutup rapi saat error?
- Leak Check: Apakah goroutine dan PTY child process benar-benar di-kill saat sesi berakhir?
- Performance: Apakah ada pembacaan buffer besar ke memori tanpa streaming?

## 12. Definition of Done
Sebuah task atau modul dianggap selesai jika:
- Berhasil di-build tanpa error di Linux dan Windows.
- Lolos `go vet` dan `go test`.
- Tidak ada memory leak pada pemantauan pprof atau goroutine leak.
- Perubahan arsitektur tercatat di `docs/architecture/overview.md`.

## 13. Explicit Anti-Patterns
- Anti-Pattern 1: Menyimpan state sesi di database untuk interaksi real-time streaming (gunakan in-memory hub dengan sync primitives).
- Anti-Pattern 2: Menggunakan polling HTTP untuk status perangkat (wajib menggunakan WebSocket persistent heartbeat).
- Anti-Pattern 3: Inbound connection dari server ke klien (melanggar prinsip reverse connection GoMesh).
- Anti-Pattern 4 (AV False Positive Trigger): Dilarang menggunakan UPX/packer pada binary Windows agen karena memicu deteksi heuristik antivirus. Wajib menyertakan Windows Manifest & Version Info pada kompilasi Windows.

## 14. Open Decisions
- [OPEN DECISION] Pilihan router HTTP: Standard Library `net/http` bawaan Go 1.22+ (sudah mendukung method & path params) vs `go-chi/chi`. (Rekomendasi awal: Go 1.22+ standard library `net/http` untuk zero-dependency router).
