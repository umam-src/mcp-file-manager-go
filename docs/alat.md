# Daftar tool dan konfigurasi

Kontrak mengikuti `mcp-file-manager` Python 0.4.0, kecuali perilaku tautan pada `delete_file`, `delete_dir`, dan `move_file` (lihat "Keamanan path"; Python menyusul). Nama dan parameter tool mengikuti Desktop Commander (khusus file) dan berbahasa Inggris; deskripsi dan pesan error berbahasa Indonesia. Parameter bertanda `*` wajib. Semua path boleh relatif terhadap `BASE_DIR` atau absolut selama hasil akhirnya masih di dalam `BASE_DIR`. Kegagalan dikembalikan sebagai `{"error": "..."}`, bukan error protokol.

## Tool

| Tool | Parameter | Keterangan |
|---|---|---|
| `list_directory` | `path`, `depth` (2), `exclude`, `max_entries` | Daftar isi folder, tiap entri diawali `[DIR]` atau `[FILE]` dengan path relatif. Folder berat (`.git`, `.venv`, `node_modules`, `__pycache__`, dll.) dicantumkan tetapi tidak dimasuki. `exclude` (daftar nama; persis, atau pola `*` dan `?`) melewati entri sepenuhnya. Hasil maksimal 1000 entri (`truncated`); `max_entries` menurunkan batas itu (nilai lebih besar dipotong ke batas server). Hanya baca. |
| `read_file` | `path*`, `offset` (0), `length` (2000) | Baca per baris, streaming. `offset` negatif membaca ekor file (`length` diabaikan). Hasil: `offset`, `next_offset` (bila masih ada baris), `total_lines`, `line_ending` (`LF`, `CRLF`, `CR`, `mixed`), `truncated`. Maksimal 100.000 karakter. Hanya baca. |
| `read_multiple_files` | `paths*` | Baca sampai 20 file dengan aturan `read_file`; batas karakter dibagi untuk semua file. File yang gagal memuat `error` tanpa menggagalkan yang lain. Hanya baca. |
| `write_file` | `path*`, `content*`, `mode` (`rewrite`) | `rewrite` menulis ulang seluruh file, `append` menambah di akhir; folder induk dibuat bila perlu. Line ending file lama dipertahankan, file baru memakai LF. |
| `edit_block` | `file_path*`, `old_string*`, `new_string*`, `expected_replacements` (1) | Ganti `old_string`; gagal bila jumlah kemunculannya berbeda dari `expected_replacements`. `old_string` kosong ditolak. |
| `create_directory` | `path*` | Buat folder (rekursif, tidak error bila sudah ada). |
| `move_file` | `source*`, `destination*`, `overwrite` (false) | Pindah atau ganti nama. Tautan dipindah sebagai tautan; menimpa tautan hanya melepas tautannya. Tujuan yang ditimpa dihapus hanya setelah pemindahan berhasil; bila gagal, tujuan tetap utuh. Di Windows, mengganti huruf besar/kecil nama saja (`a.txt` ke `A.txt`) diperbolehkan; di Linux huruf dibedakan seperti biasa. |
| `get_file_info` | `path*` | Tipe, ukuran, waktu ubah (UTC), dan jumlah baris untuk file teks (tidak untuk file biner atau di atas 10 MiB). Hanya baca. |
| `start_search` | `pattern*`, `path`, `searchType` (`files`), `filePattern`, `ignoreCase` (true), `maxResults` (100) | `files` mencari nama file/folder (teks biasa, atau wildcard `*` `?`); `content` mencari teks biasa di dalam file (file biner dan di atas 5 MiB dilewati; tautan yang keluar `BASE_DIR` atau rusak dilewati; `filePattern` membatasi nama file). Hasil dibatasi `maxResults` (maksimal 1000, `truncated`). Hanya baca. |
| `delete_file` | `path*` | Hapus file (bukan folder). Pada tautan, yang dihapus tautannya. |
| `delete_dir` | `path*`, `recursive` (false), `dry_run` (false) | Hapus folder; bawaannya hanya bila kosong. `BASE_DIR` mana pun tidak dapat dihapus. Pada tautan ke folder, hanya tautannya yang dilepas (hasil memuat `link: true`; `recursive` tidak berpengaruh). Dengan `dry_run=true` tidak ada yang dihapus (`deleted: false`): hasil rekursif melaporkan `files`, `dirs`, `bytes`, dan `truncated: true` bila lebih dari 100.000 entri (hitungan sebagian); hasil tidak rekursif melaporkan `empty` dan `would_delete`; tautan di dalam pohon dihitung sebagai file dan tidak diikuti. |

Annotations: `list_directory`, `read_file`, `read_multiple_files`, `get_file_info`, dan `start_search` bertanda hanya baca; `write_file`, `edit_block`, `move_file`, `delete_file`, dan `delete_dir` bertanda destruktif.

## Konfigurasi (variabel lingkungan)

| Variabel | Bawaan | Keterangan |
|---|---|---|
| `MCP_FILE_CONFIG` | berkas bawaan di folder config pengguna (`%APPDATA%\mcp-file-manager\config.json`; Linux `$XDG_CONFIG_HOME` atau `~/.config`), dipakai hanya bila ada | Path berkas config JSON (lihat "Berkas config"). Dimuat ulang tanpa restart. Tanpa variabel ini dan tanpa berkas bawaan, server memakai `MCP_FILE_BASE`. |
| `MCP_FILE_BASE` | tidak ada (wajib bila tidak ada config) | Cadangan bila berkas config tidak ada atau tidak valid saat mulai; semua folder bermode `rw`. Folder yang boleh diakses; harus ada. Beberapa folder dipisah `;` (Windows) atau `:` (Linux): path relatif dihitung dari folder pertama yang tersedia, path absolut di salah satu folder diterima. Folder yang tidak ada atau bukan folder dilewati dengan peringatan di stderr selama masih ada satu yang valid; folder yang bertumpang tindih (sama, atau satu di dalam yang lain) ditolak saat mulai. Folder jarak jauh (UNC `\\server\share`, mount CIFS/NFS) boleh sebagai path yang dipasang sistem operasi. `move_file` antar-folder memakai rename sistem operasi dan gagal dengan pesan galat bila berbeda volume. Bila kosong, server berhenti dengan pesan yang jelas. |
| `MCP_FILE_MAX_WRITE_BYTES` | 5 MiB | Batas ukuran tulis. Nilai tidak valid (bukan angka, nol, negatif) memakai bawaan dan dicatat sebagai peringatan. |
| `MCP_FILE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, atau `error`; log ke stderr. Nilai tidak valid memakai `info`. |

### Berkas config

Berkas JSON di luar semua folder dasar (ditolak bila berada di dalamnya, agar server tidak bisa mengubah pengaturannya sendiri). Contoh:

```json
{
  "bases": [
    {"path": "C:/Proyek", "mode": "rw"},
    {"path": "//server/share", "mode": "ro"}
  ],
  "max_write_bytes": 5242880,
  "max_read_chars": 100000,
  "max_entries": 1000
}
```

- `mode`: `rw` atau `ro`; bawaan `ro` (aman). Tulis, hapus, dan pindah ke folder `ro` ditolak dengan pesan galat.
- Batas yang dihilangkan memakai bawaan. Gunakan garis miring (`//server/share`) agar JSON tidak rusak oleh `\`.
- Dimuat ulang tanpa restart: waktu ubah berkas diperiksa tiap panggilan. Berkas rusak atau tidak valid: config valid terakhir tetap dipakai dan peringatan ditulis ke stderr.
- Bila berkas tidak bisa dipakai saat mulai, `MCP_FILE_BASE` menjadi cadangan (semua `rw`) dan berkas tetap dipantau.

Tidak ada dukungan berkas `.env`; isi variabel lewat pengaturan client MCP.

## Keamanan path

Ditolak: path kosong, null byte, traversal (`..`), serta symlink dan junction Windows yang mengarah keluar semua folder dasar (`BASE_DIR`), termasuk bila targetnya belum ada. Semua akses lewat `safe()` di `safepath.go`. `delete_file`, `delete_dir`, dan `move_file` memakai `safeLeaf()`, yang tidak mengikuti tautan pada komponen terakhir sehingga yang dihapus atau dipindah adalah tautannya, bukan isi targetnya; tautan yang mengarah keluar `BASE_DIR` tetap ditolak. Selebihnya `resolveLinks` mengikuti symlink dan junction komponen demi komponen karena `filepath.EvalSymlinks` di Go (Windows) tidak mengikuti junction.

## Build dan test

```
go build -o mcp-file-manager-go.exe .
gofmt -l . ; go vet ./... ; go test ./...
```
