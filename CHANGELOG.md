# Changelog

Semua perubahan penting dicatat di file ini.
Format mengikuti [Keep a Changelog](https://keepachangelog.com/), versi mengikuti [SemVer](https://semver.org/).

Milestone berikutnya mengikuti [ROADMAP.md](ROADMAP.md).

## [Unreleased]

### Added
- `claude_desktop_config.json.example` dan `docs/pemasangan.md` (panduan pasang di Claude Desktop).
- CI (`.github/workflows/ci.yml`): `gofmt`, `go vet`, dan `go test` di Linux dan Windows; jalan hanya saat kode Go, modul, `CHANGELOG.md`, atau workflow berubah, membatalkan run lama, dan memakai cache.
- Workflow rilis (`.github/workflows/release.yml`): pada tag `v*` mencocokkan tag dengan `version.go`, menjalankan test, membangun `windows/amd64` dan `linux/amd64` (`-trimpath -ldflags "-s -w"`), lalu melampirkan biner dan `SHA256SUMS.txt` ke Release. Kedua workflow belum pernah dijalankan.
- `.gitignore` (menutup `.env`, `*.exe`, `dist/`).

## [0.2.1] - 2026-10-10

### Fixed
- Data hilang pada tautan (symlink atau junction Windows) ke folder di dalam `BASE_DIR`: `delete_dir` dengan `recursive=true` menghapus isi folder targetnya dan menyisakan tautannya, `move_file` memindahkan target (bukan tautannya), dan `move_file` dengan `overwrite=true` ke tautan menghapus isi targetnya. Kini `delete_file`, `delete_dir`, dan `move_file` bekerja pada tautannya sendiri. `delete_dir` pada tautan hanya melepas tautan (hasil memuat `link: true`, `recursive` tidak berpengaruh); tautan rusak tetap bisa dihapus atau dipindah. Tautan yang mengarah keluar `BASE_DIR` tetap ditolak. Versi Python belum mendapat perbaikan ini.

## [0.2.0] - 2026-10-10

Tool mengikuti nama dan parameter Desktop Commander (khusus file); kontrak tetap identik dengan Python 0.4.0.

### Added
- `read_multiple_files`: baca sampai 20 file sekali panggil; batas karakter dibagi antar file dan file yang gagal memuat `error` tanpa menggagalkan yang lain.
- `get_file_info`: tipe, ukuran, waktu ubah (UTC), dan jumlah baris untuk file teks.
- `start_search`: cari nama file/folder (teks biasa atau wildcard `*` `?`) atau teks di dalam file (`searchType=content`), dibatasi `maxResults` (`truncated`).
- `read_file`: `offset` negatif membaca ekor file.
- `write_file`: parameter `mode` (`rewrite` bawaan atau `append`).

### Changed
- **BREAKING:** `list_files` diganti `list_directory(path, depth)`; hasilnya daftar `[DIR] nama` dan `[FILE] nama` (path relatif dengan `/`), folder berat dicantumkan tetapi tidak dimasuki, dan `truncated` hanya muncul bila hasil terpotong.
- **BREAKING:** `edit_file(path, old_string, new_string, count)` diganti `edit_block(file_path, old_string, new_string, expected_replacements)`. Perubahan dibatalkan dengan `error` bila jumlah kemunculan berbeda dari `expected_replacements` (bawaan 1).
- **BREAKING:** `read_file` memakai `length` (sebelumnya `limit`); `move_file` memakai `source` dan `destination` (sebelumnya `src` dan `dst`); `mkdir` diganti `create_directory`.
- **BREAKING:** Hasil lebih ringkas: `write_file` hanya `path` dan `written`, `move_file` hanya `destination` dan `moved`, tanpa field `ok`.
- `tools/list` kini memuat 11 tool.

### Removed
- **BREAKING:** `append_file` (pakai `write_file` dengan `mode` `append`).

## [0.1.1] - 2026-10-10

### Changed
- Hemat token: `tools/list` turun dari 6.341 menjadi 4.140 karakter. Skema hasil generik kosong (`outputSchema`) dibuang dari semua tool, deskripsi tool dipangkas menjadi maksimal dua kalimat dan disamakan dengan versi Python, dan deskripsi parameter dikurangi menjadi tiga. Kontrak tool dan bentuk hasil tidak berubah.

## [0.1.0] - 2026-10-10

Rilis pertama: padanan Go dari `mcp-file-manager` Python 0.3.0. Satu `.exe`, tanpa Python, tanpa panggilan jaringan.

### Added
- Server MCP (stdio) dengan SDK resmi `go-sdk`, `instructions` (memuat `BASE_DIR`), dan annotations pada semua tool. Log hanya ke stderr.
- Sembilan tool dengan kontrak sama seperti Python 0.3.0: `list_files`, `read_file`, `write_file`, `append_file`, `edit_file`, `move_file`, `delete_file`, `delete_dir`, `mkdir`.
- `list_files` rekursif dengan filter pola; direktori berat dilewati (`excluded_dirs`) dan hasil dibatasi 1000 entri (`truncated`).
- `read_file` streaming dengan `offset`, `next_offset`, `line_ending`, dan batas 100.000 karakter.
- Batas tulis `MCP_FILE_MAX_WRITE_BYTES` (default 5 MiB). Line ending file yang ada dipertahankan, file baru memakai LF, dan `old_string` kosong pada `edit_file` ditolak.
- Konfigurasi lewat variabel lingkungan: `MCP_FILE_BASE` (wajib, tanpa default), `MCP_FILE_MAX_WRITE_BYTES`, `MCP_FILE_LOG_LEVEL`.

### Security
- Semua path divalidasi: path kosong, null byte, traversal (`..`), serta symlink dan junction Windows yang mengarah keluar `BASE_DIR` ditolak, termasuk bila target belum ada. `delete_dir` tidak dapat menghapus `BASE_DIR`.
