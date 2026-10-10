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

### Saran optimasi (dikerjakan setelah disetujui)
- [ ] Test junction hanya berjalan di Windows (junction dipakai sebagai cadangan symlink); pastikan symlink asli juga teruji di Linux lewat CI.
- [x] 0.2.1: bug data hilang pada tautan diperbaiki (`delete_file`, `delete_dir`, `move_file` bekerja pada tautannya sendiri, `safeLeaf()` di `safepath.go`, 6 test regresi). Test `delete_file` pada symlink file dilewati di Windows tanpa hak symlink; baru terbukti jalan lewat CI Linux. Python belum diperbaiki (dikerjakan setelah Go selesai).
- [x] 0.2.2: `move_file` dengan `overwrite=true` tidak lagi menghapus tujuan sebelum `rename` berhasil (cadangan sementara; test regresi gagal dulu, lalu lulus di Windows).
- [x] 0.2.3: `start_search` (content) tidak lagi membaca file di luar `BASE_DIR` lewat symlink file. Test regresi memakai `symlinkOrSkip`, jadi baru terbukti jalan lewat CI Linux atau Windows dengan hak symlink; Python belum diperiksa.
- [x] 0.2.4: ganti huruf besar/kecil nama lewat `move_file` (mis. `a.txt` ke `A.txt`) kini berhasil di Windows (`caseOnlyTarget`, 4 test); di Linux perilaku tidak berubah. Python menyusul.
- [x] Ukuran `.exe` 12 MB menjadi 8,5 MB (windows) dan 8,3 MB (linux) dengan `-trimpath -ldflags "-s -w"`; dipakai di workflow rilis.
- [x] 0.1.1: hemat token tahap 1 (`outputSchema` dibuang, deskripsi tool dipangkas dan disamakan dengan Python): `tools/list` 6.341 menjadi 4.140 karakter.
- [ ] Hemat token tahap 2: sederhanakan tipe union `null` pada parameter opsional (`path`, `depth`, `length`, `expected_replacements`, `searchType`, dll.) dan hapus deskripsi parameter yang sudah jelas. Ukur ulang setelahnya.
