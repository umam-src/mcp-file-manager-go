# mcp-file-manager-go

Alat bantu agar asisten AI (misalnya Claude) dapat melihat, membaca, dan mengelola file di **satu folder yang Anda tentukan**. Berupa satu file `.exe`, tidak perlu memasang Python, dan tidak memakai internet.

## Yang bisa dilakukan

- Melihat isi folder dan membaca file teks, bahkan yang besar.
- Membuat, mengubah, menambah, memindah, dan menghapus file atau folder.
- Menjaga format baris file tetap sama, sehingga file yang diubah tidak terlihat berubah seluruhnya.

## Aman karena dibatasi

Asisten hanya bisa bekerja di dalam folder yang Anda pilih. Semua upaya keluar dari folder itu ditolak, termasuk lewat jalan pintas folder. Folder utama itu sendiri juga tidak bisa dihapus.

## Pasang di Claude Desktop (Windows)

Cukup empat langkah.

1. **Siapkan file programnya.** Anda butuh file `mcp-file-manager-go.exe`. Versi siap unduh belum diterbitkan, jadi file ini dibuat sendiri dari folder ini (caranya ada di [docs/pemasangan.md](docs/pemasangan.md)). Simpan di tempat tetap yang tidak akan dipindah, misalnya `C:\Alat\`.
2. **Pilih satu folder** yang boleh dilihat dan diubah asisten, misalnya folder proyek Anda. Makin sempit makin aman. Folder itu harus sudah ada.
3. **Daftarkan di Claude Desktop.** Buka berkas pengaturan `%APPDATA%\Claude\claude_desktop_config.json` (ketik alamat itu di kolom alamat File Explorer), lalu tambahkan isi contoh di [claude_desktop_config.json.example](claude_desktop_config.json.example). Ganti dua alamat di dalamnya: lokasi file `.exe` dan folder pilihan Anda. Tulis alamat dengan garis miring biasa (`/`), misalnya `C:/Alat/mcp-file-manager-go.exe`. Bila berkas itu sudah berisi pengaturan lain, tambahkan di dalamnya, jangan buat blok baru.
4. **Mulai ulang Claude Desktop sepenuhnya.** Keluar lewat ikon di pojok kanan bawah layar (bukan hanya menutup jendela), lalu buka lagi. Coba minta Claude, "tampilkan isi folder saya"; hasilnya harus sesuai folder yang Anda pilih.

Bila tidak berjalan, penyebab paling umum adalah alamat `.exe` salah atau folder pilihan belum ada. Cara memeriksanya dan panduan lengkap ada di [docs/pemasangan.md](docs/pemasangan.md).

## Untuk yang ingin tahu lebih dalam

Daftar perintah, isian, dan batasnya ada di [docs/alat.md](docs/alat.md). Riwayat perubahan ada di [CHANGELOG.md](CHANGELOG.md), dan cara ikut berkontribusi di [CONTRIBUTING.md](CONTRIBUTING.md).

## Lisensi

MIT. Hak cipta 2026 Endsho.
