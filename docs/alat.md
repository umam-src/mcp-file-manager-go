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

## Varian `exec` (`mcp-file-manager-exec`)

Dibangun dengan build tag `exec` (`go build -tags exec`); biner normal tidak memuat kode ini sama sekali (tanpa simbol proses, `tools/list` tidak berubah). Varian ini menambah tiga tool proses yang berjalan **tanpa batas folder dan dengan hak pengguna yang menjalankan server**. Tool file tetap dibatasi folder dasar; hanya proses yang tidak. Cara kerjanya meniru Desktop Commander: `cwd` tidak dibatasi (membatasinya tidak melindungi apa pun, karena program yang dijalankan bisa mengakses path mana pun) dan batas keamanannya adalah isolasi mesin. Pengaturan tambahan di masa depan (bila perlu) memakai berkas config, bukan variabel lingkungan baru.

**Syarat dan risiko.** Hanya untuk mesin terisolasi (VM, container Docker, atau Windows Sandbox untuk aplikasi Windows). Server menolak mulai kecuali `MCP_SANDBOX=ya`; itu pernyataan pengguna, bukan pemeriksaan: server tidak bisa memastikan mesinnya terisolasi. Tidak ada daftar program yang diizinkan, jadi perintah apa pun bisa membaca, menulis, atau menghapus di mana saja yang dapat dijangkau pengguna itu, dan jaringan tidak diblokir. Jangan memasang `.ssh`, token, atau data pribadi ke mesin itu; folder proyek yang dipasang tetap bisa rusak, jadi Git atau snapshot adalah jaring pengaman. Penggunaan di mesin biasa berarti memberi model akses penuh ke akun Anda.

| Tool | Parameter | Keterangan |
|---|---|---|
| `start_process` | `program*`, `args`, `cwd`, `shell` (false), `timeout_s` (120, maks 3600), `wait_ms` (3000, maks 30000) | Jalankan program langsung (`program` + `args`, tanpa shell perantara) atau, dengan `shell=true`, satu baris perintah lewat shell OS. `cwd` bawaan folder dasar pertama; harus folder yang ada. Kembali segera setelah proses selesai atau `wait_ms` habis, dengan `pid` dan keluaran awal. Proses dimatikan sendiri setelah `timeout_s` (`timed_out: true`). |
| `read_process_output` | `pid*`, `offset`, `length` (20000), `wait_ms` (1000) | Baca keluaran bertahap dari `offset` (bawaan: lanjutan bacaan terakhir); menunggu hingga `wait_ms` bila belum ada keluaran baru. |
| `kill_process` | `pid*` | Hentikan proses beserta seluruh anak prosesnya. |

Hasil: `pid`, `status` (`running` atau `exited`), `exit_code` dan `timed_out` (bila selesai), `output`, `next_offset`, `truncated` (hasil dipotong; lanjutkan dari `next_offset`), dan `dropped` (keluaran lama sudah terbuang dari penampung). Hanya proses yang dimulai server ini yang dikenal.

Perilaku: stdout dan stderr digabung; stdin tertutup (tanpa interaktif; `interact_with_process` direncanakan di 0.7.0); penampung 256 KiB per proses (yang tertua dibuang); paling banyak 8 proses berjalan dan 20 proses selesai disimpan; lingkungan proses dibersihkan (hanya variabel sistem dasar seperti `PATH` dan `HOME`; tanpa token atau rahasia host); setiap proses tercatat di log stderr (program, argumen, `cwd`, kode keluar, durasi).

Per OS: Windows menjalankan `cmd /d /c chcp 65001 & <baris>` agar keluaran UTF-8 dan mematikan pohon proses dengan `taskkill /T /F`; Linux memakai `sh -c` dan grup proses (`SIGKILL` ke seluruh grup). Pohon proses ikut mati saat batas waktu, `kill_process`, dan server berhenti. Di Docker, jalankan container dengan `--init`: bila server menjadi PID 1 tanpa init, proses yang sudah dimatikan tetap tertinggal sebagai zombie (tidak berjalan, tetapi tidak dibersihkan); dengan `--init` tidak ada yang tersisa (teruji di Linux, Docker 29).

## Keamanan path

Ditolak: path kosong, null byte, traversal (`..`), serta symlink dan junction Windows yang mengarah keluar semua folder dasar (`BASE_DIR`), termasuk bila targetnya belum ada. Semua akses lewat `safe()` di `safepath.go`. `delete_file`, `delete_dir`, dan `move_file` memakai `safeLeaf()`, yang tidak mengikuti tautan pada komponen terakhir sehingga yang dihapus atau dipindah adalah tautannya, bukan isi targetnya; tautan yang mengarah keluar `BASE_DIR` tetap ditolak. Selebihnya `resolveLinks` mengikuti symlink dan junction komponen demi komponen karena `filepath.EvalSymlinks` di Go (Windows) tidak mengikuti junction.

## Build dan test

```
go build -o mcp-file-manager-go.exe .
gofmt -l . ; go vet ./... ; go test ./...
# varian exec (tool proses; lihat bagian di atas)
go build -tags exec -o mcp-file-manager-exec.exe .
go vet -tags exec ./... ; go test -tags exec ./...
```
