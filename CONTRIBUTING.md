# Berkontribusi

Terima kasih sudah ingin membantu. Proyek ini kecil dan sengaja dijaga ringan.

## Sebelum mengirim perubahan

Jalankan dari folder ini; semuanya harus bersih dan lulus:

```
gofmt -l .
go vet ./...
go test ./...
```

## Aturan singkat

- Kontrak tool (nama, parameter, bentuk hasil) harus sama dengan versi Python di `../mcp-file-manager`. Bila berubah, kedua sisi diubah bersama.
- Semua akses file dari input pemanggil wajib lewat `safe()` di `safepath.go`. Jangan memanggil `os.Open` atau `os.ReadFile` langsung pada input pemanggil.
- Jangan menulis ke stdout selain protokol; log ke stderr.
- Tambah dependensi hanya bila benar-benar perlu. Server harus tetap berjalan penuh tanpa jaringan.
- Setiap perbaikan atau tool baru disertai test, termasuk kasus gagal (di luar `BASE_DIR`, tidak ditemukan, null byte, symlink atau junction keluar).
- Perubahan yang terlihat oleh pemanggil perlu naik versi dan entri `CHANGELOG.md` dalam satu commit (format [Keep a Changelog](https://keepachangelog.com/), [SemVer](https://semver.org/)). Test `TestVersiSamaDenganChangelog` menjaganya.
- Jangan memasukkan kredensial, token, atau data pribadi ke kode, catatan, maupun commit.
