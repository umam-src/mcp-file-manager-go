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
- [x] `list_directory`: `exclude` dan `max_entries` opsional (0.4.0; Python menyusul).
- [x] `delete_dir`: dry-run/preview (belum dirilis; lihat `[Unreleased]` di CHANGELOG).
- [x] Beberapa folder di `MCP_FILE_BASE` (selesai 2026-10-11 di `[Unreleased]`, belum dirilis; `tools/list` tetap 5.116). Syarat:
  - Setiap path lewat `safe()` dan harus berada di salah satu folder; symlink dan junction diperiksa per folder (`resolveLinks`).
  - Path relatif dihitung dari folder pertama; `delete_dir` tidak boleh menghapus folder dasar mana pun; folder dasar yang tumpang tindih ditolak saat mulai.
  - Ukur `tools/list` dan `instructions` sebelum dan sesudah (tidak boleh naik tanpa alasan tertulis); perbarui `docs/alat.md` dan `docs/pemasangan.md`.
  - Belum diuji: folder jarak jauh (UNC di Windows, mis. `\\localhost\C$`; mount NFS/CIFS di Linux) dan kasus gagal (share putus, hak akses ditolak); sisa tindak lanjut sebelum rilis.
  - Pertanyaan terbuka ada di `Keputusan.md` (folder tidak tersedia saat mulai, `move_file` antar-folder, batas waktu share yang macet).

### 0.6.0 - Varian `exec` (tool proses, hanya untuk mesin terisolasi)
- [x] Tahap A di kode: `start_process`, `read_process_output`, `kill_process` (build tag `exec`, `MCP_SANDBOX=ya` wajib); test kasus gagal lulus di Windows; `tools/list` normal 5.308 (tidak berubah), exec 6.903; `docs/alat.md` memuat risiko dan syarat isolasi; CI dan workflow rilis diperbarui (CI sudah hijau; alur tag belum dicoba).
- [x] Push (`2a4d7c1`) dan CI hijau di Linux dan Windows (2026-10-13).
- [x] Uji stdio varian `exec` di container Linux nyata (2026-10-13, Docker 29 di VM dev): shell, timeout, `kill_process`, tanpa shell, program tidak ada, tanpa `MCP_SANDBOX` lulus; pohon proses mati. Catatan: pakai `docker run --init` agar tidak ada zombie (sudah di `docs/alat.md`).
- [x] Isolasi dan `cwd` diputuskan 2026-10-13 (meniru Desktop Commander: `cwd` bebas, isolasi di mesin: Docker atau Windows Sandbox sebagai pilihan terdokumentasi; config tambahan lewat config 0.5); lihat `Keputusan.md`.
- [x] Rilis 0.6.0 (bump `version.go` dan CHANGELOG satu commit; tahap A saja, 2026-10-13).
- [x] 0.7.0, tahap B di kode: `interact_with_process` dan `interactive` pada `start_process` (pipe seperti Desktop Commander, bukan PTY); test lulus dengan `-tags exec` di Windows; uji stdio dengan biner sungguhan lulus (2026-10-13). Belum dirilis (`[Unreleased]`).
- [ ] 0.7.0: CI Linux untuk test interaktif, uji dari Claude Desktop, lalu rilis (bump `version.go` dan CHANGELOG satu commit).
- [ ] Opsional: PTY sungguhan (ConPTY di Windows, PTY di Unix) bila pipe tidak cukup untuk REPL yang perlu terminal; butuh pustaka atau kode per OS, putuskan setelah ada kebutuhan nyata.

### Saran optimasi (dikerjakan setelah disetujui)
- [ ] Test junction hanya berjalan di Windows (junction dipakai sebagai cadangan symlink); pastikan symlink asli juga teruji di Linux lewat CI.
- [x] 0.2.1: bug data hilang pada tautan diperbaiki (`delete_file`, `delete_dir`, `move_file` bekerja pada tautannya sendiri, `safeLeaf()` di `safepath.go`, 6 test regresi). Test `delete_file` pada symlink file dilewati di Windows tanpa hak symlink; baru terbukti jalan lewat CI Linux. Python belum diperbaiki (dikerjakan setelah Go selesai).
- [x] 0.2.2: `move_file` dengan `overwrite=true` tidak lagi menghapus tujuan sebelum `rename` berhasil (cadangan sementara; test regresi gagal dulu, lalu lulus di Windows).
- [x] 0.2.3: `start_search` (content) tidak lagi membaca file di luar `BASE_DIR` lewat symlink file. Test regresi memakai `symlinkOrSkip`, jadi baru terbukti jalan lewat CI Linux atau Windows dengan hak symlink; Python belum diperiksa.
- [x] 0.2.4: ganti huruf besar/kecil nama lewat `move_file` (mis. `a.txt` ke `A.txt`) kini berhasil di Windows (`caseOnlyTarget`, 4 test); di Linux perilaku tidak berubah. Python menyusul.
- [x] Ukuran `.exe` 12 MB menjadi 8,5 MB (windows) dan 8,3 MB (linux) dengan `-trimpath -ldflags "-s -w"`; dipakai di workflow rilis.
- [x] 0.1.1: hemat token tahap 1 (`outputSchema` dibuang, deskripsi tool dipangkas dan disamakan dengan Python): `tools/list` 6.341 menjadi 4.140 karakter.
- [x] 0.2.6: hemat token tahap 2 (union `null` dibuang dari 10 parameter opsional, argumen `null` tetap diterima): `tools/list` 5.476 menjadi 5.386 karakter.
- [x] 0.2.7: hemat token tahap 3: `additionalProperties:false` tidak lagi dikirim di `tools/list` (`tools/list` 5.386 menjadi 5.067 karakter); penolakan parameter yang tidak dikenal tetap berjalan di server (`unknown_args_test.go`). `title` di annotations sengaja dipertahankan (hemat kecil, keterbacaan permintaan persetujuan lebih penting); `destructiveHint` tetap eksplisit. Python belum disamakan.
- [ ] Daftar path terlindungi (deny) di config: tolak tulis, hapus, dan pindah ke file yang bisa melebarkan akses atau menjalankan kode, mis. claude_desktop_config.json, kode dan .venv/site-packages server, .git/hooks, folder Startup, dan .exe server sendiri. Usulan untuk 0.5.0 (config terpusat); Python menyusul. Belum disetujui.
- [ ] Mode o per folder dasar (folder rujukan seperti profil pengguna atau SMB hanya dibaca), di file config yang sama. Usulan untuk 0.5.0. Belum disetujui.
- [ ] Tulis folder jaringan di config dengan garis miring (//server/share) agar JSON tidak rusak; contohnya dimasukkan ke docs/pemasangan.md, termasuk catatan bahwa folder yang tidak terjangkau saat mulai dilewati dan baru aktif setelah restart.
