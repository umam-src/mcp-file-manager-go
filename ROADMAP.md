# Roadmap — mcp-file-manager-go

Padanan Go dari `mcp-file-manager` (Python): satu `.exe`, tanpa Python. Kontraknya (nama tool, parameter, bentuk hasil) sama dengan versi Python.

## Status saat ini

- Versi 0.2.1 (2026-10-10): tool mengikuti Desktop Commander (11 tool); perilaku tautan pada `delete_file`, `delete_dir`, `move_file` sudah diperbaiki di Go dan belum di Python, sehingga paritas dengan Python 0.4.0 sementara berbeda di bagian itu. CI dan workflow rilis sudah ditulis tetapi belum dijalankan; belum diuji di aplikasi Claude Desktop.
- Acuan paritas: versi Python 0.4.0 (menurut `CHANGELOG.md` Python; `pyproject.toml` masih 0.3.1). Pengerjaan Python menunggu Go selesai.

## Cara membaca roadmap ini

Sesuai aturan "MINOR ditentukan roadmap" di `CLAUDE.md`, setiap milestone di bawah adalah satu rilis MINOR. Perubahan lain yang terlihat oleh pemanggil dan tidak tercantum di sini dirilis sebagai PATCH. Nomor versi adalah usulan dan boleh disesuaikan sebelum milestone dikerjakan.

## 0.1.0 — Paritas dengan Python 0.3.0 (selesai 2026-10-10)

- [x] Server stdio memakai SDK resmi `github.com/modelcontextprotocol/go-sdk`. Stdout hanya untuk protokol; semua log ke stderr.
- [x] `instructions` server (memuat `BASE_DIR` dan cara pakai tool) dan annotations pada semua tool.
- [x] 9 tool (kemudian diganti di 0.2.0): `list_files`, `read_file`, `write_file`, `append_file`, `edit_file`, `move_file`, `delete_file`, `delete_dir`, `mkdir`.
- [x] `list_files`: rekursif, filter pola, direktori berat (`.git`, `.venv`, `node_modules`, dll.) dilewati, jumlah entri dibatasi.
- [x] `read_file`: streaming dengan `offset`, `next_offset`, `line_ending`, dan batas 100.000 karakter.
- [x] Validasi path setara `_safe()`: tolak traversal, symlink dan junction keluar `BASE_DIR`, path kosong, dan null byte. `delete_dir` tidak boleh menghapus `BASE_DIR`.
- [x] Batas tulis (default 5 MiB, dapat diubah lewat `MCP_FILE_MAX_WRITE_BYTES`) dan pelestarian line ending CRLF/LF. `old_string` kosong pada `edit_file` ditolak.
- [x] Kegagalan dikembalikan sebagai `{"error": ...}`, bukan error protokol.
- [x] `go test` yang setara dengan test Python, termasuk kasus gagal.
- [x] Uji end-to-end lewat stdio, dan `.exe` Windows dicoba pada data nyata (hasil sama dengan Python).
- [x] Sumber versi tunggal: konstanta `version` di `version.go`, dijaga test terhadap `CHANGELOG.md`.

## 0.2.0 — Adopsi tool Desktop Commander (selesai 2026-10-10)

Keputusan "Adopsi tool Desktop Commander" di `Keputusan.md`; kontrak identik dengan Python 0.4.0.

- [x] 11 tool: `list_directory`, `read_file`, `read_multiple_files`, `write_file`, `edit_block`, `create_directory`, `move_file`, `get_file_info`, `start_search`, `delete_file`, `delete_dir`.
- [x] Menggantikan `list_files`, `append_file`, `edit_file`, dan `mkdir` (perubahan merusak kompatibilitas, ditandai **BREAKING** di CHANGELOG).
- [x] `read_file` dengan `offset` negatif (ekor file), `write_file` dengan `mode`, `get_file_info`, dan pencarian nama/isi.

## 0.3.0 — Siap dipasang dan dirilis
- [x] `README.md`, `docs/alat.md` (daftar tool dan konfigurasi), `CONTRIBUTING.md`, `LICENSE` (dikerjakan bersama 0.1.0).
- [x] Contoh `claude_desktop_config.json` untuk `.exe` dan panduan pemasangan (`docs/pemasangan.md`).
- [ ] CI Go: `go vet` dan `go test` di Windows dan Linux, hemat menit (`paths`, `concurrency`, cache). Ditulis di `.github/workflows/ci.yml`; menunggu dijalankan.
- [ ] Build rilis `windows/amd64` dan `linux/amd64`. Ditulis di `.github/workflows/release.yml`; cross-build lokal berhasil, menunggu tag.
- [ ] Diuji di aplikasi Claude Desktop sungguhan.

## 0.4.0 — Fitur lanjutan (mengikuti TODO Python)

Butir di bawah berasal dari bagian "Masih terbuka" di `TODO.md` versi Python. Kontrak harus sama dengan Python; bila Python mengerjakannya lebih dulu, Go menyusul dengan kontrak yang sama.

- [ ] `list_directory`: parameter opsional `exclude` dan `max_entries`.
- [ ] `delete_dir`: dry-run/preview sebelum hapus rekursif.
- [ ] Tool `search_files` (cari teks di banyak file) dan `get_file_info` (ukuran, waktu ubah).

## Di luar cakupan

- Mengubah atau menghapus versi Python.
- Menambah tool di luar 11 tool di atas sebelum 0.3.0 selesai.
- Transport selain stdio (versi Python juga hanya stdio).

## Keputusan yang sudah diambil

Penomoran, `MCP_FILE_BASE` wajib tanpa default, tanpa dukungan `.env`, SDK sebagai dependensi langsung, dan sumber versi tunggal sudah diputuskan; lihat `Keputusan.md` di catatan proyek.
