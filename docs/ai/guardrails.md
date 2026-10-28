# AI Guardrails — GoMesh

## 1. Purpose
Dokumen ini mengatur tata kelola dan batasan bagi AI Coding Agent yang beroperasi di repositori GoMesh agar seluruh implementasi tetap selaras dengan arsitektur Go yang idiomatik, efisien, dan aman.

## 2. Default Mode
- Default mode: Planning first untuk fitur baru, execute directly untuk bugfix terisolasi dan task kecil.
- Fitur besar (Remote Desktop, PTY implementation, File Transfer): Rancang protokol pesan di `internal/protocol/` sebelum mengimplementasikan handler.
- Pertanyaan & Ambiguitas: Terapkan rekomendasi arsitektur terbaik berdasarkan standar Go, hanya tanyakan pada user jika berdampak signifikan pada arsitektur sistem.

## 3. Before Touching Code
AI wajib:
- Memeriksa file spesifikasi terkait di `docs/foundation-pack/` dan `docs/`.
- Memeriksa apakah tipe pesan atau konstanta sudah didefinisikan di `internal/protocol/`.
- Memastikan tidak ada penambahan CGO yang merusak portabilitas cross-compilation Go.

## 4. Hard Guardrails
- Dilarang menambahkan dependensi CGO tanpa persetujuan eksplisit.
- Dilarang menggunakan global variable untuk state koneksi WebSocket atau session hub.
- Dilarang mengabaikan error (`_ = ...`) pada operasi network atau filesystem.
- Dilarang membuat arsitektur inbound connection dari server ke agen (wajib reverse outbound connection dari agen ke server).
- Dilarang meninggalkan file demo/scaffold atau placeholder komentar `// TODO nanti diisi`.
- Dilarang mengompresi binary agent dengan UPX packer (memicu deteksi false-positive antivirus). Wajib menggunakan `-ldflags="-s -w" -trimpath` dan metadata Windows Manifest resmi.

## 5. Scaffolding Rules
- Menjaga struktur direktori Go standar: `cmd/`, `internal/`, `web/`, `docs/`.
- Gunakan nama package yang pendek, relevan, dan huruf kecil semua (`hub`, `pty`, `auth`).
- File web frontend harus ditempatkan di `web/` dan diekspos melalui `embed.FS`.

## 6. Editing Rules
- Selalu baca file sebelum melakukan patch.
- Patch sespesifik mungkin tanpa merusak struktur sekitarnya.
- Bila memodifikasi handler WebSocket, periksa penanganan error penutupan koneksi (`websocket.IsCloseError`).

## 7. Planning Rules
Sebelum mengimplementasikan modul besar, AI wajib menyajikan:
- Ringkasan scope dan flow data
- Perubahan pada protokol pesan (`internal/protocol/`)
- Daftar file yang akan dibuat/dimodifikasi
- Rencana verifikasi (build, test, vet)

## 8. Verification Rules
AI wajib memverifikasi setiap perubahan menggunakan perintah berikut:

Checklist verifikasi Go:
- Formatter: `go fmt ./...`
- Linter / Vet: `go vet ./...`
- Testing: `go test -v -race ./...`
- Server Build: `go build -o bin/gomesh ./cmd/gomesh`
- Agent Build (Linux): `GOOS=linux GOARCH=amd64 go build -o bin/gomesh-agent-linux ./cmd/gomesh-agent`
- Agent Build (Windows): `GOOS=windows GOARCH=amd64 go build -o bin/gomesh-agent.exe ./cmd/gomesh-agent`

Jika perintah gagal dijalankan, jelaskan error secara tepat dan perbaiki akar masalahnya sebelum menandai task selesai.

## 9. Communication Rules
- Berikan respons yang ringkas dan terstruktur (GitHub markdown).
- Selalu cantumkan tautan ke file yang dimodifikasi (`[path/file.go](file:///...)`).
- Bedakan secara eksplisit mana data fakta, mana asumsi teknis, dan mana rekomendasi.

## 10. Review Rules
Sebelum finalisasi, periksa:
- Apakah goroutine tertutup dengan benar saat context dibatalkan?
- Apakah mutex lock dilepas (`defer mu.Unlock()`)?
- Apakah file descriptor OS (seperti PTY master/slave) ditutup cleanly?

## 11. Refactor Scope Rules
- Refactor lokal yang berdekatan dengan task diizinkan.
- Perubahan kontrak protokol WebSocket antar modul memerlukan konfirmasi terlebih dahulu.

## 12. Cleanup Rules
- Pastikan tidak ada console log/print debugging liar (gunakan `slog.Debug` terstruktur).
- Pastikan binary hasil build lokal masuk ke `.gitignore` (`bin/`).

## 13. Output Expectations for AI
Setiap penyelesaian task harus menyertakan:
1. Ringkasan fitur yang diimplementasikan.
2. Daftar file yang dibuat atau diubah.
3. Hasil eksekusi perintah verifikasi (`go test`, `go build`).
4. Langkah selanjutnya sesuai roadmap.

## 14. Project-Specific Overrides
- Prioritaskan kecepatan performa (*sat-set*) dan minimalisasi ukuran binary.
- Gunakan fitur native Go 1.22+ / 1.23+ semaksimal mungkin sebelum mengambil 3rd party packages.
