package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// symlinkOrSkip membuat symlink; di Windows tanpa izin yang cukup test dilewati.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink tidak bisa dibuat di lingkungan ini: %v", err)
	}
}

// linkDirOrSkip membuat tautan ke folder yang sudah ada. Bila symlink tidak
// diizinkan di Windows, dipakai junction (mklink /J) yang tidak butuh hak
// istimewa dan sama-sama bisa menunjuk ke luar BASE_DIR.
func linkDirOrSkip(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr == nil {
			t.Logf("memakai junction karena symlink tidak diizinkan: %s", strings.TrimSpace(string(out)))
			return
		}
	}
	t.Skipf("tautan folder tidak bisa dibuat di lingkungan ini: %v", err)
}

func TestSymlinkOutsideBaseDirBlocked(t *testing.T) {
	m := newTestManager(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkDirOrSkip(t, outside, m.abs("keluar"))

	if r := m.readFile(readInput{Path: "keluar/secret.txt"}); !hasError(r) {
		t.Error("membaca lewat symlink ke luar seharusnya ditolak")
	}
	if r := m.write("keluar/baru.txt", "x"); !hasError(r) {
		t.Error("menulis lewat symlink ke luar seharusnya ditolak")
	}
	if _, err := os.Stat(filepath.Join(outside, "baru.txt")); err == nil {
		t.Error("file bocor ke luar BASE_DIR lewat symlink")
	}
}

func TestDanglingSymlinkOutsideBaseDirBlocked(t *testing.T) {
	m := newTestManager(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "belum-ada.txt")
	symlinkOrSkip(t, target, m.abs("rusak"))

	if r := m.write("rusak", "x"); !hasError(r) {
		t.Error("menulis lewat symlink rusak ke luar seharusnya ditolak")
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("file dibuat di luar BASE_DIR lewat symlink rusak")
	}
	if r := m.writeFile(writeInput{Path: "rusak", Content: "x", Mode: "append"}); !hasError(r) {
		t.Error("append lewat symlink rusak ke luar seharusnya ditolak")
	}
	if r := m.createDirectory(pathInput{Path: "rusak/sub"}); !hasError(r) {
		t.Error("mkdir lewat symlink rusak ke luar seharusnya ditolak")
	}
}

func TestSymlinkInsideBaseDirAllowed(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli/a.txt", "ok")
	linkDirOrSkip(t, m.abs("asli"), m.abs("pintas"))
	if r := m.readFile(readInput{Path: "pintas/a.txt"}); hasError(r) {
		t.Errorf("symlink di dalam BASE_DIR seharusnya boleh: %v", r["error"])
	}
}

func TestSymlinkLoopDoesNotHang(t *testing.T) {
	m := newTestManager(t)
	symlinkOrSkip(t, m.abs("b"), m.abs("a"))
	symlinkOrSkip(t, m.abs("a"), m.abs("b"))
	if r := m.write("a", "x"); !hasError(r) {
		t.Error("symlink melingkar seharusnya ditolak")
	}
}

func TestSafeNonexistentNestedPathStaysInside(t *testing.T) {
	m := newTestManager(t)
	got, err := m.cfg.safe("belum/ada/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !within(m.cfg.baseDir, got) || !strings.HasSuffix(got, filepath.Join("belum", "ada", "file.txt")) {
		t.Errorf("got = %s", got)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	if _, err := loadConfig(env(nil)); err == nil || !strings.Contains(err.Error(), envBase) {
		t.Errorf("tanpa MCP_FILE_BASE seharusnya error yang menyebut variabelnya: %v", err)
	}
	if _, err := loadConfig(env(map[string]string{envBase: filepath.Join(dir, "tidak-ada")})); err == nil {
		t.Error("folder tidak ada seharusnya error")
	}
	if _, err := loadConfig(env(map[string]string{envBase: file})); err == nil {
		t.Error("file (bukan folder) seharusnya error")
	}

	cfg, err := loadConfig(env(map[string]string{envBase: dir, envMaxWriteBytes: "1234"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.maxWriteBytes != 1234 || cfg.maxReadChars != defaultMaxReadChars || cfg.maxEntries != defaultMaxEntries {
		t.Errorf("cfg = %+v", cfg)
	}
	for _, bad := range []string{"abc", "-5", "0"} {
		cfg, err := loadConfig(env(map[string]string{envBase: dir, envMaxWriteBytes: bad}))
		if err != nil || cfg.maxWriteBytes != defaultMaxWriteBytes {
			t.Errorf("nilai %q seharusnya memakai bawaan: cfg=%+v err=%v", bad, cfg, err)
		}
	}
}

// TestVersiSamaDenganChangelog menjaga aturan "jangan bump versi tanpa entri
// CHANGELOG, dan sebaliknya": versi di kode harus sama dengan judul rilis
// teratas di CHANGELOG.md (0.0.0 bila belum pernah dirilis).
func TestVersiSamaDenganChangelog(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`).FindSubmatch(data)
	want := "0.0.0"
	if m != nil {
		want = string(m[1])
	}
	if version != want {
		t.Errorf("version = %s, tetapi rilis teratas di CHANGELOG.md adalah %s", version, want)
	}
}

func TestToolsAnnotatedOverProtocol(t *testing.T) {
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

	if got := cs.InitializeResult().Instructions; !strings.Contains(got, m.cfg.baseDir) {
		t.Errorf("instructions tidak memuat BASE_DIR: %q", got)
	}

	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"list_directory": true, "read_file": true, "read_multiple_files": true, "write_file": true,
		"edit_block": true, "create_directory": true, "move_file": true, "get_file_info": true,
		"start_search": true, "delete_file": true, "delete_dir": true,
	}
	if len(list.Tools) != len(want)+execToolCount {
		t.Errorf("jumlah tool = %d, mau %d", len(list.Tools), len(want)+execToolCount)
	}
	for _, tool := range list.Tools {
		if execToolNames[tool.Name] {
			continue // tool varian exec sengaja berbeda (openWorldHint); diuji di exec_test.go
		}
		if !want[tool.Name] {
			t.Errorf("tool tak terduga: %s", tool.Name)
		}
		a := tool.Annotations
		if a == nil || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s: openWorldHint harus false", tool.Name)
			continue
		}
		switch tool.Name {
		case "read_file", "read_multiple_files", "list_directory", "get_file_info", "start_search":
			if !a.ReadOnlyHint {
				t.Errorf("%s harus read-only", tool.Name)
			}
		case "write_file", "edit_block", "delete_file", "delete_dir", "move_file":
			if a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint {
				t.Errorf("%s harus destruktif", tool.Name)
			}
		}
		t.Logf("skema %s: %v", tool.Name, tool.InputSchema)
	}
}

func TestEndToEndOverProtocol(t *testing.T) {
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

	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: error protokol: %v", name, err)
		}
		return res
	}
	// Hasil hanya dikirim sekali: satu blok teks JSON di content, tanpa
	// structuredContent (syarat utama hemat token).
	structured := func(res *mcp.CallToolResult) map[string]any {
		t.Helper()
		if res.StructuredContent != nil {
			t.Fatalf("StructuredContent harus kosong (hasil dobel), isinya: %#v", res.StructuredContent)
		}
		if len(res.Content) != 1 {
			t.Fatalf("content harus satu blok, dapat %d", len(res.Content))
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("content[0] bukan teks: %#v", res.Content[0])
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			t.Fatalf("content bukan JSON objek: %v: %s", err, tc.Text)
		}
		return out
	}

	if out := structured(call("write_file", map[string]any{"path": "a.txt", "content": "halo\nlagi\n"})); out["written"] != float64(10) {
		t.Errorf("write_file: %v", out)
	}
	// Argumen opsional tidak dikirim: bawaan harus berlaku.
	out := structured(call("read_file", map[string]any{"path": "a.txt"}))
	if out["total_lines"] != float64(2) {
		t.Errorf("read_file: %v", out)
	}
	out = structured(call("edit_block", map[string]any{"file_path": "a.txt", "old_string": "halo", "new_string": "hai"}))
	if out["replacements"] != float64(1) {
		t.Errorf("edit_block: %v", out)
	}
	out = structured(call("write_file", map[string]any{"path": "a.txt", "content": "akhir\n", "mode": "append"}))
	if out["written"] != float64(6) {
		t.Errorf("write_file append: %v", out)
	}
	out = structured(call("read_file", map[string]any{"path": "a.txt", "offset": -1}))
	if lines, _ := out["lines"].([]any); len(lines) != 1 || lines[0] != "akhir\n" {
		t.Errorf("read_file ekor: %v", out)
	}
	out = structured(call("list_directory", map[string]any{}))
	if entries, _ := out["entries"].([]any); len(entries) != 1 || entries[0] != "[FILE] a.txt" {
		t.Errorf("list_directory: %v", out)
	}
	out = structured(call("start_search", map[string]any{"pattern": "hai", "searchType": "content"}))
	if matches, _ := out["matches"].([]any); len(matches) != 1 || matches[0] != "a.txt:1: hai" {
		t.Errorf("start_search: %v", out)
	}
	out = structured(call("get_file_info", map[string]any{"path": "a.txt"}))
	if out["lines"] != float64(3) || out["type"] != "file" {
		t.Errorf("get_file_info: %v", out)
	}
	out = structured(call("read_multiple_files", map[string]any{"paths": []string{"a.txt", "tidak-ada.txt"}}))
	if files, _ := out["files"].([]any); len(files) != 2 {
		t.Errorf("read_multiple_files: %v", out)
	}

	// Kegagalan adalah hasil biasa berisi "error", bukan error protokol.
	res := call("read_file", map[string]any{"path": "../rahasia.txt"})
	if res.IsError {
		t.Error("kegagalan tool tidak boleh ditandai isError; kontraknya {\"error\": ...}")
	}
	if _, ok := structured(res)["error"]; !ok {
		t.Errorf("hasil seharusnya memuat error: %v", res.Content)
	}
}

// Tautan (symlink/junction) ke folder di dalam BASE_DIR: delete dan move
// harus bekerja pada tautannya sendiri, bukan pada isi targetnya.

func TestDeleteDirOnLinkRemovesOnlyLink(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		m := newTestManager(t)
		m.raw(t, "asli/data.txt", "penting")
		linkDirOrSkip(t, m.abs("asli"), m.abs("tautan"))

		r := m.deleteDir(deleteDirInput{Path: "tautan", Recursive: recursive})
		if r["deleted"] != true {
			t.Fatalf("recursive=%v: seharusnya berhasil, hasil: %v", recursive, r)
		}
		if got := m.content(t, "asli/data.txt"); got != "penting" {
			t.Errorf("recursive=%v: isi target berubah: %q", recursive, got)
		}
		if _, err := os.Lstat(m.abs("tautan")); err == nil {
			t.Errorf("recursive=%v: tautan seharusnya terhapus", recursive)
		}
	}
}

func TestDeleteFileOnSymlinkRemovesOnlyLink(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli.txt", "penting")
	symlinkOrSkip(t, m.abs("asli.txt"), m.abs("tautan.txt"))

	if r := m.deleteFile(pathInput{Path: "tautan.txt"}); r["deleted"] != true {
		t.Fatalf("seharusnya berhasil, hasil: %v", r)
	}
	if got := m.content(t, "asli.txt"); got != "penting" {
		t.Errorf("isi target berubah: %q", got)
	}
	if _, err := os.Lstat(m.abs("tautan.txt")); err == nil {
		t.Error("tautan seharusnya terhapus")
	}
}

func TestDeleteFileOnDirLinkStillRejected(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli/data.txt", "penting")
	linkDirOrSkip(t, m.abs("asli"), m.abs("tautan"))

	if r := m.deleteFile(pathInput{Path: "tautan"}); !hasError(r) {
		t.Error("delete_file pada tautan ke folder seharusnya ditolak")
	}
	if got := m.content(t, "asli/data.txt"); got != "penting" {
		t.Errorf("isi target berubah: %q", got)
	}
}

func TestDeleteOnLinkOutsideBaseDirRejected(t *testing.T) {
	m := newTestManager(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkDirOrSkip(t, outside, m.abs("keluar"))

	if r := m.deleteDir(deleteDirInput{Path: "keluar", Recursive: true}); !hasError(r) {
		t.Error("delete_dir pada tautan ke luar BASE_DIR seharusnya ditolak")
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.txt")); err != nil {
		t.Errorf("isi di luar BASE_DIR terhapus: %v", err)
	}
}

func TestMoveFileSourceLinkMovesOnlyLink(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli/data.txt", "penting")
	linkDirOrSkip(t, m.abs("asli"), m.abs("tautan"))

	if r := m.moveFile(moveInput{Source: "tautan", Destination: "tautan2"}); r["moved"] != true {
		t.Fatalf("seharusnya berhasil, hasil: %v", r)
	}
	if got := m.content(t, "asli/data.txt"); got != "penting" {
		t.Errorf("target ikut berpindah atau berubah: %v", got)
	}
	if _, err := os.Lstat(m.abs("tautan")); err == nil {
		t.Error("tautan lama seharusnya hilang")
	}
	if _, err := os.Lstat(m.abs("tautan2")); err != nil {
		t.Errorf("tautan baru seharusnya ada: %v", err)
	}
}

func TestMoveFileOverwriteOntoLinkRemovesOnlyLink(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli/data.txt", "penting")
	m.raw(t, "baru/x.txt", "x")
	linkDirOrSkip(t, m.abs("asli"), m.abs("tautan"))

	if r := m.moveFile(moveInput{Source: "baru", Destination: "tautan", Overwrite: true}); r["moved"] != true {
		t.Fatalf("seharusnya berhasil, hasil: %v", r)
	}
	if got := m.content(t, "asli/data.txt"); got != "penting" {
		t.Errorf("isi target terhapus atau berubah: %v", got)
	}
	if got := m.content(t, "tautan/x.txt"); got != "x" {
		t.Errorf("hasil pindah salah: %v", got)
	}
}
