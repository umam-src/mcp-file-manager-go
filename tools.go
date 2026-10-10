package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// manager menampung pengaturan dan menjadi penerima semua handler tool.
type manager struct{ cfg *config }

// instructions dikirim ke client saat inisialisasi (Claude Desktop
// memasukkannya ke konteks model), supaya model tahu batas akses dan cara pakai.
func instructions(baseDir string) string {
	return fmt.Sprintf("Server file manager lokal. Semua path relatif terhadap BASE_DIR = %s. "+
		"Path absolut di dalam BASE_DIR juga diterima; path di luarnya ditolak. "+
		"Untuk melihat struktur pakai list_directory(depth); folder berat "+
		"(.git, .venv, node_modules, dll.) tidak dimasuki. "+
		"Untuk file besar pakai read_file dengan length dan offset (lanjutkan dari "+
		"next_offset; offset negatif membaca ekor file). Pakai edit_block untuk "+
		"perubahan kecil; write_file hanya untuk file baru, tulis ulang penuh, atau "+
		"mode append. Line ending file yang sudah ada (CRLF/LF) dipertahankan. "+
		"Hapus, timpa, dan pindah tidak bisa dibatalkan.", baseDir)
}

// wrap menyesuaikan fungsi tool sederhana dengan bentuk handler SDK. Hasil
// selalu dikembalikan sebagai nilai biasa, tidak pernah sebagai error protokol.
//
// Hasil dikirim satu kali saja, sebagai satu blok teks JSON ringkas di
// content. Tipe hasil sengaja any dan structuredContent sengaja tidak diisi:
// SDK akan menggandakan isi yang sama ke structuredContent bila handler
// mengembalikan nilai hasil, dan skema generik outputSchema hanya menambah
// token di tools/list (syarat utama proyek: hemat token).
func wrap[In any](f func(In) result) mcp.ToolHandlerFor[In, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		b, err := json.Marshal(f(in))
		if err != nil {
			b = []byte(`{"error":"hasil tool gagal diubah ke JSON"}`)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
	}
}

// annotations memberi tahu client mana tool yang aman (read-only) dan mana
// yang destruktif, supaya client bisa meminta persetujuan pengguna.
func annotations(title string, readOnly, destructive, idempotent bool) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   &closed,
	}
}

func newServer(cfg *config) *mcp.Server {
	m := &manager{cfg: cfg}
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "mcp-file-manager", Version: version},
		&mcp.ServerOptions{Instructions: instructions(cfg.baseDir)},
	)

	// Deskripsi tool: maksimal dua kalimat (aturan "Desain MCP hemat token").
	// Nama, urutan, dan teksnya disamakan dengan versi Python.
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_directory",
		Description: "Daftar isi folder sampai kedalaman depth, diberi awalan [DIR] atau [FILE]. " +
			"Folder berat (.git, .venv, node_modules, dll.) tidak dimasuki dan hasil dibatasi 1000 entri ('truncated').",
		Annotations: annotations("Daftar folder", true, false, true),
	}, wrap(m.listDirectory))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "read_file",
		Description: "Baca file teks (UTF-8) per baris; offset negatif membaca baris dari akhir file, dan file besar dilanjutkan dari 'next_offset'. " +
			"Hasil dibatasi karakternya ('truncated'); 'line_ending' melaporkan LF, CRLF, CR, atau mixed.",
		Annotations: annotations("Baca file", true, false, true),
	}, wrap(m.readFile))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "read_multiple_files",
		Description: "Baca beberapa file (maks 20) sekaligus dengan aturan read_file; batas karakter dibagi untuk semua file. " +
			"File yang gagal dibaca memuat 'error' tanpa menggagalkan yang lain.",
		Annotations: annotations("Baca banyak file", true, false, true),
	}, wrap(m.readMultipleFiles))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "write_file",
		Description: "Tulis seluruh file (mode rewrite, bawaan) atau tambah di akhir (mode append); folder induk dibuat bila perlu. " +
			"Line ending file lama dipertahankan, file baru memakai LF; ada batas ukuran.",
		Annotations: annotations("Tulis file", false, true, false),
	}, wrap(m.writeFile))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "edit_block",
		Description: "Ganti old_string dengan new_string; gagal bila jumlah kemunculannya berbeda dari expected_replacements (bawaan 1). " +
			"old_string tidak boleh kosong; CRLF dicocokkan otomatis dan line ending dipertahankan.",
		Annotations: annotations("Edit file", false, true, false),
	}, wrap(m.editBlock))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_directory",
		Description: "Buat folder (rekursif; tidak error bila sudah ada).",
		Annotations: annotations("Buat direktori", false, false, true),
	}, wrap(m.createDirectory))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "move_file",
		Description: "Pindah atau ganti nama file/folder dari source ke destination; overwrite=true menimpa destination.",
		Annotations: annotations("Pindah/ganti nama", false, true, false),
	}, wrap(m.moveFile))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_file_info",
		Description: "Info file atau folder: tipe, ukuran, waktu ubah (UTC), dan jumlah baris untuk file teks.",
		Annotations: annotations("Info file", true, false, true),
	}, wrap(m.getFileInfo))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "start_search",
		Description: "Cari nama file/folder (searchType=files, bawaan; pattern teks biasa atau wildcard * ?) atau teks di dalam file (searchType=content, teks biasa). " +
			"Hasil langsung dikembalikan dan dibatasi maxResults ('truncated'); filePattern membatasi nama file pada pencarian isi.",
		Annotations: annotations("Cari", true, false, true),
	}, wrap(m.startSearch))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_file",
		Description: "Hapus file (bukan folder).",
		Annotations: annotations("Hapus file", false, true, true),
	}, wrap(m.deleteFile))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_dir",
		Description: "Hapus folder. Bawaannya hanya bila kosong; recursive=true menghapus beserta isinya.",
		Annotations: annotations("Hapus direktori", false, true, true),
	}, wrap(m.deleteDir))

	return srv
}
