# Changelog

Semua perubahan penting dicatat di file ini.
Format mengikuti [Keep a Changelog](https://keepachangelog.com/), versi mengikuti [SemVer](https://semver.org/).

Milestone berikutnya mengikuti [ROADMAP.md](ROADMAP.md).

## [Unreleased]

## [0.5.0] - 2026-10-13

### Added
- Config terpusat: berkas JSON lewat `MCP_FILE_CONFIG` (atau berkas bawaan di folder config pengguna bila ada) berisi folder dasar dengan mode `rw`/`ro` (bawaan `ro`) dan batas `max_write_bytes`, `max_read_chars`, `max_entries`. Berkas harus di luar semua folder dasar.
- Muat ulang tanpa restart; berkas rusak: config valid terakhir tetap dipakai dan peringatan ke stderr.
- Folder `ro` menolak tulis, hapus, dan pindah.

### Changed
- `MCP_FILE_BASE` kini cadangan bila berkas config tidak ada atau tidak valid (semua `rw`); tanpa keduanya server tidak jalan. Bagi yang hanya memakai `MCP_FILE_BASE`, perilaku tidak berubah.

## [0.4.0] - 2026-10-12

### Added
- `list_directory`: parameter opsional `exclude` (daftar nama yang dilewati sepenuhnya; nama persis, atau pola dengan `*` dan `?`; huruf besar/kecil diabaikan di Windows) dan `max_entries` (menurunkan batas 1000 entri; nilai di atas batas server dipotong ke batas server, nilai kurang dari 1 ditolak). Tanpa kedua parameter hasil tidak berubah. `tools/list` 5.150 menjadi 5.308 karakter (lihat `Keputusan.md` proyek, 2026-10-12). Isu G-6.
## [0.3.0] - 2026-10-12

### Added
- `MCP_FILE_BASE` boleh berisi beberapa folder (dipisah `;` di Windows, `:` di Linux). Path relatif dihitung dari folder pertama yang tersedia; path absolut di salah satu folder diterima; symlink dan junction diperiksa terhadap semua folder. Folder tak tersedia dilewati dengan peringatan selama masih ada satu yang valid; folder bertumpang tindih ditolak saat mulai; `delete_dir` tidak menghapus folder dasar mana pun. Satu folder berjalan seperti sebelumnya; `tools/list` tetap 5.116 karakter, `instructions` hanya bertambah bila ada lebih dari satu folder. Folder jarak jauh (UNC, CIFS, NFS) belum diuji.
- `delete_dir`: parameter opsional `dry_run`; pratinjau hapus rekursif tanpa menghapus (`files`, `dirs`, `bytes`, `truncated`; tidak rekursif: `empty`, `would_delete`). `tools/list` 5.067 menjadi 5.116 karakter. Isu G-7.
- `claude_desktop_config.json.example` dan `docs/pemasangan.md` (panduan pasang di Claude Desktop).
- CI (`.github/workflows/ci.yml`): `gofmt`, `go vet`, dan `go test` di Linux dan Windows; jalan hanya saat kode Go, modul, `CHANGELOG.md`, atau workflow berubah, membatalkan run lama, dan memakai cache.
- Workflow rilis (`.github/workflows/release.yml`): tag `vX.Y.Z` mencocokkan tag dengan `version.go`, menjalankan test, membangun `windows/amd64` dan `linux/amd64` (`-trimpath -ldflags "-s -w"`), lalu melampirkan biner dan `SHA256SUMS.txt` ke Release; tag `vX.Y.Z-rc.N` menghasilkan pre-release. Pull request dan build manual (Run workflow) di branch mana pun hanya menghasilkan artifact 7 hari (Windows saja), tanpa tag dan Release. Sudah dicoba lewat tag `v0.2.6-rc.1` (pre-release).
- `.gitignore` (menutup `.env`, `*.exe`, `dist/`).

## [0.2.7] - 2026-10-11

### Changed
- Hemat token tahap 3: `additionalProperties:false` tidak lagi dikirim di skema `tools/list` (11 kali); `tools/list` turun dari 5.386 menjadi 5.067 karakter. Keamanan tidak berubah: parameter yang tidak dikenal tetap ditolak saat `tools/call` dengan pesan galat yang sama, karena skema yang terdaftar di server tidak dipangkas, hanya salinan yang dikirim ke client. Nama, nilai bawaan, dan hasil tool tidak berubah.

## [0.2.6] - 2026-10-11

### Changed
- Hemat token tahap 2: skema input di `tools/list` tidak lagi memuat tipe union `null` pada 10 parameter opsional (mis. `["null","string"]` menjadi `"string"`); `tools/list` turun dari 5.476 menjadi 5.386 karakter. Parameter opsional cukup tidak dikirim, dan argumen yang dikirim `null` tetap diterima dan diperlakukan sama dengan tidak dikirim. Nama, nilai bawaan, dan hasil tool tidak berubah.

## [0.2.5] - 2026-10-10

### Changed
- Hasil tool kini dikirim satu kali saja: satu blok teks JSON ringkas di `content`. Sebelumnya isi yang sama juga dikirim ulang di `structuredContent`, sehingga setiap hasil memakan token hampir dua kali lipat. Isi JSON tidak berubah; client yang membaca `structuredContent` harus membaca `content[0].text`. Versi Python belum mendapat perubahan ini.

## [0.2.4] - 2026-10-10

### Fixed
- `move_file` bisa mengganti huruf besar/kecil nama (mis. `a.txt` ke `A.txt`) di Windows; sebelumnya ditolak "Sumber dan tujuan sama". Linux tidak berubah.

## [0.2.3] - 2026-10-10

### Security
- `start_search` (`searchType=content`) tidak lagi membaca isi file di luar `BASE_DIR` lewat symlink file; tautan keluar `BASE_DIR` atau rusak dilewati.

## [0.2.2] - 2026-10-10

### Fixed
- `move_file` dengan `overwrite=true` tidak lagi menghapus tujuan sebelum pemindahan berhasil; tujuan dicadangkan sementara dan dikembalikan bila `rename` gagal.

## [0.2.1] - 2026-10-10

### Fixed
- Data hilang pada tautan (symlink atau junction Windows) ke folder di dalam `BASE_DIR`: `delete_file`, `delete_dir`, dan `move_file` kini bekerja pada tautannya sendiri, bukan isi targetnya. `delete_dir` pada tautan hanya melepas tautan (hasil memuat `link: true`). Tautan keluar `BASE_DIR` tetap ditolak.

## [0.2.0] - 2026-10-10

Tool mengikuti nama dan parameter Desktop Commander (khusus file); kontrak identik dengan Python 0.4.0.

### Added
- `read_multiple_files`, `get_file_info`, dan `start_search` (nama file atau isi, `maxResults`); `read_file` dengan `offset` negatif; `write_file` dengan `mode` (`rewrite` atau `append`).

### Changed
- **BREAKING:** `list_files` menjadi `list_directory(path, depth)`, `edit_file` menjadi `edit_block(file_path, old_string, new_string, expected_replacements)`, `mkdir` menjadi `create_directory`; `read_file` memakai `length`, `move_file` memakai `source` dan `destination`; hasil tanpa field `ok`. Total 11 tool.

### Removed
- **BREAKING:** `append_file` (pakai `write_file` dengan `mode` `append`).

## [0.1.1] - 2026-10-10

### Changed
- Hemat token tahap 1: `tools/list` turun dari 6.341 menjadi 4.140 karakter (`outputSchema` kosong dibuang, deskripsi tool dipangkas dan disamakan dengan Python).

## [0.1.0] - 2026-10-10

Rilis pertama: padanan Go dari `mcp-file-manager` Python 0.3.0. Satu `.exe`, tanpa Python, tanpa panggilan jaringan.

### Added
- Server MCP (stdio) dengan SDK resmi `go-sdk`, `instructions`, annotations, dan sembilan tool file; konfigurasi lewat `MCP_FILE_BASE` (wajib), `MCP_FILE_MAX_WRITE_BYTES`, dan `MCP_FILE_LOG_LEVEL`.

### Security
- Path kosong, null byte, traversal (`..`), serta symlink dan junction Windows yang mengarah keluar `BASE_DIR` ditolak. `delete_dir` tidak dapat menghapus `BASE_DIR`.
