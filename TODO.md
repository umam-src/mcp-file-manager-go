# TODO

Milestone dan nomor versi ada di [ROADMAP.md](ROADMAP.md); file ini memecahnya menjadi langkah kerja.

- [x] Go terpasang (go1.27.0 windows/amd64).
- [x] Kerangka modul, `ROADMAP.md`, dan keputusan pertanyaan terbuka.
- [x] 0.1.0: paritas dengan Python 0.3.0 (9 tool, validasi path termasuk junction Windows, batas baca/tulis, line ending, test, uji stdio pada data nyata).
- [x] 0.2.0: adopsi tool Desktop Commander (11 tool: `list_directory`, `read_multiple_files`, `edit_block`, `create_directory`, `get_file_info`, `start_search`, `write_file` dengan `mode`, dan `read_file` dengan `offset` negatif).
- [x] File wajib: `README.md`, `CONTRIBUTING.md`, `LICENSE`, `docs/alat.md`.

## Masih terbuka

### 0.3.0 — Siap dipasang dan dirilis
- [x] Contoh `claude_desktop_config.json` untuk `.exe` dan panduan pemasangan (`docs/pemasangan.md`).
- [ ] CI Go (`go vet`, `go test`) di Windows dan Linux, hemat menit. Workflow `.github/workflows/ci.yml` sudah ditulis; belum dijalankan (belum ada remote).
- [ ] Build rilis Windows dan Linux. Workflow `release.yml` sudah ditulis dan cross-build lokal berhasil; belum diuji lewat tag.
- [ ] Uji di aplikasi Claude Desktop sungguhan.
- [ ] Commit ke Git dan pastikan CI hijau.

### 0.4.0 — Fitur lanjutan
- [ ] `list_directory`: `exclude` dan `max_entries` opsional.
- [ ] `delete_dir`: dry-run/preview.
- [ ] Beberapa folder di `MCP_FILE_BASE` (hemat token tahap 2 sudah selesai di 0.2.6; ukur `tools/list` dan `instructions` sebelum dan sesudah). Syarat:
  - Setiap path lewat `safe()` dan harus berada di salah satu folder; symlink dan junction diperiksa per folder (`resolveLinks`).
  - Path relatif dihitung dari folder pertama; `delete_dir` tidak boleh menghapus folder dasar mana pun; folder dasar yang tumpang tindih ditolak saat mulai.
  - Ukur `tools/list` dan `instructions` sebelum dan sesudah (tidak boleh naik tanpa alasan tertulis); perbarui `docs/alat.md` dan `docs/pemasangan.md`.
  - Uji folder jarak jauh: UNC di Windows (mis. `\\localhost\C$` untuk uji lokal), mount NFS/CIFS di Linux; kasus gagal (share putus, hak akses ditolak).
  - Pertanyaan terbuka ada di `Keputusan.md` (folder tidak tersedia saat mulai, `move_file` antar-folder, batas waktu share yang macet).

### Saran optimasi (dikerjakan setelah disetujui)
- [ ] Test junction hanya berjalan di Windows (junction dipakai sebagai cadangan symlink); pastikan symlink asli juga teruji di Linux lewat CI.
- [x] 0.2.1: bug data hilang pada tautan diperbaiki (`delete_file`, `delete_dir`, `move_file` bekerja pada tautannya sendiri, `safeLeaf()` di `safepath.go`, 6 test regresi). Test `delete_file` pada symlink file dilewati di Windows tanpa hak symlink; baru terbukti jalan lewat CI Linux. Python belum diperbaiki (dikerjakan setelah Go selesai).
- [x] 0.2.2: `move_file` dengan `overwrite=true` tidak lagi menghapus tujuan sebelum `rename` berhasil (cadangan sementara; test regresi gagal dulu, lalu lulus di Windows).
- [x] 0.2.3: `start_search` (content) tidak lagi membaca file di luar `BASE_DIR` lewat symlink file. Test regresi memakai `symlinkOrSkip`, jadi baru terbukti jalan lewat CI Linux atau Windows dengan hak symlink; Python belum diperiksa.
- [x] 0.2.4: ganti huruf besar/kecil nama lewat `move_file` (mis. `a.txt` ke `A.txt`) kini berhasil di Windows (`caseOnlyTarget`, 4 test); di Linux perilaku tidak berubah. Python menyusul.
- [x] Ukuran `.exe` 12 MB menjadi 8,5 MB (windows) dan 8,3 MB (linux) dengan `-trimpath -ldflags "-s -w"`; dipakai di workflow rilis.
- [x] 0.1.1: hemat token tahap 1 (`outputSchema` dibuang, deskripsi tool dipangkas dan disamakan dengan Python): `tools/list` 6.341 menjadi 4.140 karakter.
- [x] 0.2.6: hemat token tahap 2 (union `null` dibuang dari 10 parameter opsional, argumen `null` tetap diterima): `tools/list` 5.476 menjadi 5.386 karakter.
- [ ] Hemat token tahap 3 (kandidat, hematnya kecil): `additionalProperties:false` berulang di tiap skema (sekitar 26 karakter x 11) dan `title` di annotations (sekitar 20 karakter x 11). Perlu keputusan karena memengaruhi kontrak dan kesamaan dengan Python.
