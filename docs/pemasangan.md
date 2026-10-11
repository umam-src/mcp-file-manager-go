# Panduan pemasangan

Panduan memasang `mcp-file-manager-go` di Claude Desktop (Windows). Untuk client MCP lain, isian yang dibutuhkan sama: perintah ke `.exe` dan variabel `MCP_FILE_BASE`.

## 1. Siapkan programnya

Rilis berbentuk biner belum diterbitkan, jadi untuk sekarang bangun sendiri dari folder ini (perlu Go terpasang):

```
go build -trimpath -ldflags "-s -w" -o mcp-file-manager-go.exe .
```

Simpan di folder tetap yang tidak akan dipindah, misalnya `C:\Alat\mcp-file-manager-go.exe`. Untuk Linux, bangun dengan `go build -trimpath -ldflags "-s -w" -o mcp-file-manager-go .` di mesin Linux.

Workflow rilis (`.github/workflows/release.yml`) melampirkan biner Windows dan Linux ke Release saat tag versi `vX.Y.Z` didorong; tag `vX.Y.Z-rc.N` menghasilkan pre-release untuk uji sebelum rilis sungguhan. Untuk mencoba build tanpa merilis, jalankan workflow secara manual di branch mana pun (Actions, Release, Run workflow) atau buka pull request; hasilnya artifact Windows sementara selama 7 hari. Semuanya belum pernah dijalankan.

## 2. Pilih folder yang boleh diakses

Pilih **satu folder** yang boleh dilihat dan diubah asisten. Makin sempit makin aman: lebih baik folder proyek daripada seluruh drive. Folder harus sudah ada.

## 3. Daftarkan di Claude Desktop

Buka berkas pengaturan `%APPDATA%\Claude\claude_desktop_config.json`, lalu tambahkan isi dari [claude_desktop_config.json.example](../claude_desktop_config.json.example):

```json
{
  "mcpServers": {
    "file-manager-go": {
      "command": "C:/Alat/mcp-file-manager-go.exe",
      "env": {
        "MCP_FILE_BASE": "C:/Proyek/folder-saya"
      }
    }
  }
}
```

- Pakai garis miring biasa (`/`) pada path, atau garis miring balik ganda (`\\`).
- `MCP_FILE_BASE` wajib. Bila kosong atau folder tidak ada, program berhenti dengan pesan yang jelas.
- Bila `mcpServers` sudah berisi server lain, tambahkan `"file-manager-go"` di dalamnya, jangan buat blok kedua.
- Nama tool versi Go sama persis dengan versi Python (`file-manager`) dan Desktop Commander. Bila beberapa server dipasang bersamaan, asisten melihat tool bernama sama; pasang salah satu saja untuk folder yang sama, atau pastikan pembagian tugasnya jelas.
- Pengaturan lain (batas tulis, tingkat log) ada di [alat.md](alat.md).

## 4. Mulai ulang dan periksa

Tutup Claude Desktop sepenuhnya (keluar dari ikon di baki sistem, bukan hanya menutup jendela), lalu buka lagi. Lalu minta asisten menampilkan isi folder; hasilnya harus sesuai isi `MCP_FILE_BASE`. Panduan ini belum diuji di aplikasi Claude Desktop sungguhan; yang sudah diuji adalah biner lewat protokol stdio.

## Bila tidak berjalan

- **Server tidak muncul atau berhenti:** buka log Claude Desktop di `%APPDATA%\Claude\logs\` (berkas bernama `mcp-server-` diikuti nama server di config, mis. `mcp-server-file-manager-go.log`). Penyebab umum: path `.exe` salah, `MCP_FILE_BASE` kosong, atau folder tidak ada.
- **"Akses ditolak: ... berada di luar ...":** path yang diminta berada di luar `MCP_FILE_BASE`. Ini disengaja; ubah `MCP_FILE_BASE` bila memang perlu.
