# mcp-file-manager-go

Alat bantu agar asisten AI (misalnya Claude) dapat melihat, membaca, dan mengelola file di **satu folder yang Anda tentukan**. Berupa satu file `.exe`, tidak perlu memasang Python, dan tidak memakai internet.

## Yang bisa dilakukan

- Melihat isi folder dan membaca file teks, bahkan yang besar.
- Membuat, mengubah, menambah, memindah, dan menghapus file atau folder.
- Menjaga format baris file tetap sama, sehingga file yang diubah tidak terlihat berubah seluruhnya.

## Aman karena dibatasi

Asisten hanya bisa bekerja di dalam folder yang Anda pilih. Semua upaya keluar dari folder itu ditolak, termasuk lewat jalan pintas folder. Folder utama itu sendiri juga tidak bisa dihapus.

## Memakai

1. Bangun file `mcp-file-manager-go.exe` dari folder ini (perintahnya ada di panduan di bawah). Versi siap unduh belum diterbitkan.
2. Daftarkan di aplikasi asisten Anda, dan isi `MCP_FILE_BASE` dengan folder yang boleh diakses. Isian ini wajib; tanpa itu program berhenti dengan pesan yang jelas.

Langkah lengkap dan contoh pengaturan ada di [docs/pemasangan.md](docs/pemasangan.md).

## Untuk yang ingin tahu lebih dalam

Daftar perintah, isian, dan batasnya ada di [docs/alat.md](docs/alat.md). Riwayat perubahan ada di [CHANGELOG.md](CHANGELOG.md), dan cara ikut berkontribusi di [CONTRIBUTING.md](CONTRIBUTING.md).

## Lisensi

MIT. Hak cipta 2026 Endsho.
