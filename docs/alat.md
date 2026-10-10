# Daftar tool dan konfigurasi

Kontrak mengikuti `mcp-file-manager` Python 0.4.0, kecuali perilaku tautan pada `delete_file`, `delete_dir`, dan `move_file` (lihat "Keamanan path"; Python menyusul). Nama dan parameter tool mengikuti Desktop Commander (khusus file) dan berbahasa Inggris; deskripsi dan pesan error berbahasa Indonesia. Parameter bertanda `*` wajib. Semua path boleh relatif terhadap `BASE_DIR` atau absolut selama hasil akhirnya masih di dalam `BASE_DIR`. Kegagalan dikembalikan sebagai `{"error": "..."}`, bukan error protokol.

## Tool

| Tool | Parameter | Keterangan |
|---|---|---|
| `list_directory` | `path`, `depth` (2) | Daftar isi folder, tiap entri diawali `[DIR]` atau `[FILE]` dengan path relatif. Folder berat (`.git`, `.venv`, `node_modules`, `__pycache__`, dll.) dicantumkan tetapi tidak dimasuki; hasil maksimal 1000 entri (`truncated`). Hanya baca. |
| `read_file` | `path*`, `offset` (0), `length` (2000) | Baca per baris, streaming. `offset` negatif membaca ekor file (`length` diabaikan). Hasil: `offset`, `next_offset` (bila masih ada baris), `total_lines`, `line_ending` (`LF`, `CRLF`, `CR`, `mixed`), `truncated`. Maksimal 100.000 karakter. Hanya baca. |
| `read_multiple_files` | `paths*` | Baca sampai 20 file dengan aturan `read_file`; batas karakter dibagi untuk semua file. File yang gagal memuat `error` tanpa menggagalkan yang lain. Hanya baca. |
| `write_file` | `path*`, `content*`, `mode` (`rewrite`) | `rewrite` menulis ulang seluruh file, `append` menambah di akhir; folder induk dibuat bila perlu. Line ending file lama dipertahankan, file baru memakai LF. |
| `edit_block` | `file_path*`, `old_string*`, `new_string*`, `expected_replacements` (1) | Ganti `old_string`; gagal bila jumlah kemunculannya berbeda dari `expected_replacements`. `old_string` kosong ditolak. |
| `create_directory` | `path*` | Buat folder (rekursif, tidak error bila sudah ada). |
| `move_file` | `source*`, `destination*`, `overwrite` (false) | Pindah atau ganti nama. Tautan dipindah sebagai tautan; menimpa tautan hanya melepas tautannya. |
| `get_file_info` | `path*` | Tipe, ukuran, waktu ubah (UTC), dan jumlah baris untuk file teks (tidak untuk file biner atau di atas 10 MiB). Hanya baca. |
| `start_search` | `pattern*`, `path`, `searchType` (`files`), `filePattern`, `ignoreCase` (true), `maxResults` (100) | `files` mencari nama file/folder (teks biasa, atau wildcard `*` `?`); `content` mencari teks biasa di dalam file (file biner dan di atas 5 MiB dilewati; `filePattern` membatasi nama file). Hasil dibatasi `maxResults` (maksimal 1000, `truncated`). Hanya baca. |
| `delete_file` | `path*` | Hapus file (bukan folder). Pada tautan, yang dihapus tautannya. |
| `delete_dir` | `path*`, `recursive` (false) | Hapus folder; bawaannya hanya bila kosong. `BASE_DIR` tidak dapat dihapus. Pada tautan ke folder, hanya tautannya yang dilepas (hasil memuat `link: true`; `recursive` tidak berpengaruh). |

Annotations: `list_directory`, `read_file`, `read_multiple_files`, `get_file_info`, dan `start_search` bertanda hanya baca; `write_file`, `edit_block`, `move_file`, `delete_file`, dan `delete_dir` bertanda destruktif.

## Konfigurasi (variabel lingkungan)

| Variabel | Bawaan | Keterangan |
|---|---|---|
| `MCP_FILE_BASE` | tidak ada (wajib) | Folder yang boleh diakses; harus ada. Bila kosong, server berhenti dengan pesan yang jelas. |
| `MCP_FILE_MAX_WRITE_BYTES` | 5 MiB | Batas ukuran tulis. Nilai tidak valid (bukan angka, nol, negatif) memakai bawaan dan dicatat sebagai peringatan. |
| `MCP_FILE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, atau `error`; log ke stderr. Nilai tidak valid memakai `info`. |

Tidak ada dukungan berkas `.env`; isi variabel lewat pengaturan client MCP.

## Keamanan path

Ditolak: path kosong, null byte, traversal (`..`), serta symlink dan junction Windows yang mengarah keluar `BASE_DIR`, termasuk bila targetnya belum ada. Semua akses lewat `safe()` di `safepath.go`. `delete_file`, `delete_dir`, dan `move_file` memakai `safeLeaf()`, yang tidak mengikuti tautan pada komponen terakhir sehingga yang dihapus atau dipindah adalah tautannya, bukan isi targetnya; tautan yang mengarah keluar `BASE_DIR` tetap ditolak. Selebihnya `resolveLinks` mengikuti symlink dan junction komponen demi komponen karena `filepath.EvalSymlinks` di Go (Windows) tidak mengikuti junction.

## Build dan test

```
go build -o mcp-file-manager-go.exe .
gofmt -l . ; go vet ./... ; go test ./...
```
