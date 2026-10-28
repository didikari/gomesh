<!-- BEGIN:foundation-pack-rules -->
# Core Foundation & Architecture

Anda beroperasi di dalam workspace dengan aturan arsitektur ketat (*Project Foundation Pack*).

Setiap kali *user* meminta perubahan kode, penambahan fitur, atau debugging, Anda WAJIB memeriksa aturan utama yang terletak di dalam:
`docs/foundation-pack/`

## Instruksi Wajib AI:
1. Jadikan `docs/foundation-pack/` sebagai *Source of Truth* tertinggi.
2. Jangan mengabaikan *guardrails* keamanan dan performa yang sudah tertulis di sana.
3. Jika *user* memberikan perintah yang bertentangan dengan foundation, berikan peringatan terlebih dahulu.
4. **Auto-Record (Sync-to-Code):** Setiap kali Anda selesai mengimplementasikan fitur atau merombak kode, Anda WAJIB secara otomatis memperbarui dokumen terkait di `docs/foundation-pack/` (misal: menandai fitur selesai di `implementation-roadmap.md` atau memperbarui skema di `architecture-blueprint.md`) agar dokumentasi selalu sinkron dengan kode, tanpa perlu diminta.
<!-- END:foundation-pack-rules -->
