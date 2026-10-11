package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Parameter yang tidak dikenal harus ditolak pada semua tool, dan operasinya
// tidak boleh berjalan. Penolakan ini adalah pengaman: parameter salah ketik
// (mis. "recursve") tidak boleh diabaikan diam-diam. Test ini berlaku apa pun
// cara penolakannya diimplementasikan (skema atau pemeriksaan di server).
func TestParameterTakDikenalDitolak(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	srv := newServer(m.cfg)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "uji", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Skema yang dikirim ke client tidak memuat additionalProperties (hemat
	// token), dan daftar tool diminta dulu agar terbukti pemangkasan itu tidak
	// melonggarkan validasi saat tools/call di bawah.
	for i := 0; i < 2; i++ {
		tools, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range tools.Tools {
			b, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "additionalProperties") {
				t.Errorf("%s: skema terkirim masih memuat additionalProperties: %s", tool.Name, b)
			}
		}
	}

	m.raw(t, "a.txt", "isi\n")
	m.raw(t, "dd/b.txt", "isi\n")

	cases := map[string]map[string]any{
		"list_directory":      {},
		"read_file":           {"path": "a.txt"},
		"read_multiple_files": {"paths": []string{"a.txt"}},
		"write_file":          {"path": "baru.txt", "content": "x"},
		"edit_block":          {"file_path": "a.txt", "old_string": "isi", "new_string": "ubah"},
		"create_directory":    {"path": "folder-baru"},
		"move_file":           {"source": "a.txt", "destination": "dipindah.txt"},
		"get_file_info":       {"path": "a.txt"},
		"start_search":        {"pattern": "isi"},
		"delete_file":         {"path": "a.txt"},
		"delete_dir":          {"path": "dd", "recursive": true},
	}
	if len(cases) != 11 {
		t.Fatalf("jumlah kasus = %d, harapan 11 tool", len(cases))
	}
	for name, args := range cases {
		bad := map[string]any{"zzz_tak_dikenal": 1}
		for k, v := range args {
			bad[k] = v
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: bad})
		switch {
		case err != nil:
			t.Logf("%s: ditolak (protokol): %v", name, err)
		case res != nil && res.IsError:
			msg := ""
			if len(res.Content) > 0 {
				if tc, ok := res.Content[0].(*mcp.TextContent); ok {
					msg = tc.Text
				}
			}
			t.Logf("%s: ditolak (isError): %s", name, msg)
		default:
			t.Errorf("%s: parameter tak dikenal diterima: %v", name, res.Content)
		}
	}

	// Tidak ada operasi yang berjalan.
	for _, p := range []string{"a.txt", "dd/b.txt"} {
		if _, err := os.Stat(filepath.Join(m.cfg.baseDir, p)); err != nil {
			t.Errorf("%s hilang: %v", p, err)
		}
	}
	for _, p := range []string{"baru.txt", "folder-baru", "dipindah.txt"} {
		if _, err := os.Stat(filepath.Join(m.cfg.baseDir, p)); err == nil {
			t.Errorf("%s terbentuk padahal panggilan harus ditolak", p)
		}
	}
	if b, err := os.ReadFile(filepath.Join(m.cfg.baseDir, "a.txt")); err != nil || string(b) != "isi\n" {
		t.Errorf("isi a.txt berubah: %q %v", b, err)
	}
}
