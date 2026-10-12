package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// manager menampung pengaturan dan menjadi penerima semua handler tool.
type manager struct{ cfg *config }

// instructions dikirim ke client saat inisialisasi (Claude Desktop
// memasukkannya ke konteks model), supaya model tahu batas akses dan cara pakai.
func instructions(bases []string) string {
	scope := fmt.Sprintf("Semua path relatif terhadap BASE_DIR = %s. "+
		"Path absolut di dalam BASE_DIR juga diterima; path di luarnya ditolak. ", bases[0])
	if len(bases) > 1 {
		scope = fmt.Sprintf("Path relatif terhadap BASE_DIR = %s (folder pertama). "+
			"Folder lain: %s. Path absolut di salah satu folder diterima; path di luar semuanya ditolak. ",
			bases[0], strings.Join(bases[1:], "; "))
	}
	return "Server file manager lokal. " + scope +
		"Untuk melihat struktur pakai list_directory(depth); folder berat " +
		"(.git, .venv, node_modules, dll.) tidak dimasuki. " +
		"Untuk file besar pakai read_file dengan length dan offset (lanjutkan dari " +
		"next_offset; offset negatif membaca ekor file). Pakai edit_block untuk " +
		"perubahan kecil; write_file hanya untuk file baru, tulis ulang penuh, atau " +
		"mode append. Line ending file yang sudah ada (CRLF/LF) dipertahankan. " +
		"Hapus, timpa, dan pindah tidak bisa dibatalkan."
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

// slimSchema membangkitkan skema input dari struct In seperti SDK, lalu
// mengubah tipe union ["null", T] pada parameter opsional (field pointer)
// menjadi T saja. Parameter opsional cukup tidak dikirim; "null" hanya
// menambah token di tools/list (syarat utama proyek: hemat token). Struct,
// nilai bawaan, dan daftar required tidak berubah.
func slimSchema[In any]() *jsonschema.Schema {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	for _, prop := range s.Properties {
		if len(prop.Types) != 2 {
			continue
		}
		for i, typ := range prop.Types {
			if typ == "null" {
				prop.Type, prop.Types = prop.Types[1-i], nil
				break
			}
		}
	}
	return s
}

// dropNullArgs membuang argumen bernilai null dari panggilan tool sebelum
// divalidasi skema. Skema ramping tidak lagi menerima "null", sedangkan
// sebagian client mengirim null untuk parameter yang dikosongkan; tanpa ini
// panggilan semacam itu yang sebelumnya berhasil akan ditolak.
func dropNullArgs(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if r, ok := req.(*mcp.CallToolRequest); ok && method == "tools/call" && r.Params != nil && len(r.Params.Arguments) > 0 {
			var args map[string]json.RawMessage
			if json.Unmarshal(r.Params.Arguments, &args) == nil {
				changed := false
				for k, v := range args {
					if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
						delete(args, k)
						changed = true
					}
				}
				if changed {
					if b, err := json.Marshal(args); err == nil {
						r.Params.Arguments = b
					}
				}
			}
		}
		return next(ctx, method, req)
	}
}

// slimListTools membuang "additionalProperties":false dari skema yang dikirim
// di tools/list (hemat token: syarat utama proyek). Skema yang terdaftar di
// server tidak diubah, jadi parameter yang tidak dikenal tetap ditolak saat
// tools/call dengan pesan galat yang sama; hanya salinan untuk client yang
// dirapikan. Salinan dibuat agar skema terdaftar dan Tool bersama tidak
// ikut berubah.
func slimListTools(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		if lr, ok := res.(*mcp.ListToolsResult); ok && err == nil && method == "tools/list" {
			tools := make([]*mcp.Tool, len(lr.Tools))
			for i, t := range lr.Tools {
				tools[i] = t
				if s, ok := t.InputSchema.(*jsonschema.Schema); ok && s != nil && s.AdditionalProperties != nil {
					sc, tc := *s, *t
					sc.AdditionalProperties = nil
					tc.InputSchema = &sc
					tools[i] = &tc
				}
			}
			out := *lr
			out.Tools = tools
			return &out, nil
		}
		return res, err
	}
}

// addTool mendaftarkan tool dengan skema input yang sudah dirapikan.
func addTool[In any](srv *mcp.Server, t *mcp.Tool, f func(In) result) {
	t.InputSchema = slimSchema[In]()
	mcp.AddTool(srv, t, wrap(f))
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
		&mcp.ServerOptions{Instructions: instructions(cfg.allBases())},
	)
	srv.AddReceivingMiddleware(dropNullArgs, slimListTools)

	// Deskripsi tool: maksimal dua kalimat (aturan "Desain MCP hemat token").
	// Nama, urutan, dan teksnya disamakan dengan versi Python.
	addTool(srv, &mcp.Tool{
		Name: "list_directory",
		Description: "Daftar isi folder sampai kedalaman depth, diberi awalan [DIR] atau [FILE]; exclude melewati nama (boleh * dan ?). " +
			"Folder berat (.git, .venv, node_modules, dll.) tidak dimasuki; hasil dibatasi 1000 entri ('truncated'), dapat diturunkan lewat max_entries.",
		Annotations: annotations("Daftar folder", true, false, true),
	}, m.listDirectory)

	addTool(srv, &mcp.Tool{
		Name: "read_file",
		Description: "Baca file teks (UTF-8) per baris; offset negatif membaca baris dari akhir file, dan file besar dilanjutkan dari 'next_offset'. " +
			"Hasil dibatasi karakternya ('truncated'); 'line_ending' melaporkan LF, CRLF, CR, atau mixed.",
		Annotations: annotations("Baca file", true, false, true),
	}, m.readFile)

	addTool(srv, &mcp.Tool{
		Name: "read_multiple_files",
		Description: "Baca beberapa file (maks 20) sekaligus dengan aturan read_file; batas karakter dibagi untuk semua file. " +
			"File yang gagal dibaca memuat 'error' tanpa menggagalkan yang lain.",
		Annotations: annotations("Baca banyak file", true, false, true),
	}, m.readMultipleFiles)

	addTool(srv, &mcp.Tool{
		Name: "write_file",
		Description: "Tulis seluruh file (mode rewrite, bawaan) atau tambah di akhir (mode append); folder induk dibuat bila perlu. " +
			"Line ending file lama dipertahankan, file baru memakai LF; ada batas ukuran.",
		Annotations: annotations("Tulis file", false, true, false),
	}, m.writeFile)

	addTool(srv, &mcp.Tool{
		Name: "edit_block",
		Description: "Ganti old_string dengan new_string; gagal bila jumlah kemunculannya berbeda dari expected_replacements (bawaan 1). " +
			"old_string tidak boleh kosong; CRLF dicocokkan otomatis dan line ending dipertahankan.",
		Annotations: annotations("Edit file", false, true, false),
	}, m.editBlock)

	addTool(srv, &mcp.Tool{
		Name:        "create_directory",
		Description: "Buat folder (rekursif; tidak error bila sudah ada).",
		Annotations: annotations("Buat direktori", false, false, true),
	}, m.createDirectory)

	addTool(srv, &mcp.Tool{
		Name:        "move_file",
		Description: "Pindah atau ganti nama file/folder dari source ke destination; overwrite=true menimpa destination.",
		Annotations: annotations("Pindah/ganti nama", false, true, false),
	}, m.moveFile)

	addTool(srv, &mcp.Tool{
		Name:        "get_file_info",
		Description: "Info file atau folder: tipe, ukuran, waktu ubah (UTC), dan jumlah baris untuk file teks.",
		Annotations: annotations("Info file", true, false, true),
	}, m.getFileInfo)

	addTool(srv, &mcp.Tool{
		Name: "start_search",
		Description: "Cari nama file/folder (searchType=files, bawaan; pattern teks biasa atau wildcard * ?) atau teks di dalam file (searchType=content, teks biasa). " +
			"Hasil langsung dikembalikan dan dibatasi maxResults ('truncated'); filePattern membatasi nama file pada pencarian isi.",
		Annotations: annotations("Cari", true, false, true),
	}, m.startSearch)

	addTool(srv, &mcp.Tool{
		Name:        "delete_file",
		Description: "Hapus file (bukan folder).",
		Annotations: annotations("Hapus file", false, true, true),
	}, m.deleteFile)

	addTool(srv, &mcp.Tool{
		Name:        "delete_dir",
		Description: "Hapus folder. Bawaannya hanya bila kosong; recursive=true menghapus beserta isinya; dry_run=true hanya melaporkan isinya tanpa menghapus.",
		Annotations: annotations("Hapus direktori", false, true, true),
	}, m.deleteDir)

	return srv
}
