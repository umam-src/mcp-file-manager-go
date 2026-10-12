package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type baseEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode,omitempty"`
}

// tulisConfig menulis berkas config JSON dan menggeser waktu ubahnya agar
// perubahan terdeteksi walau ditulis cepat berturut-turut.
func tulisConfig(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	tulisMentah(t, path, string(b))
}

func tulisMentah(t *testing.T, path, isi string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(isi), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Duration(len(isi)+1) * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

func cfgFile(t *testing.T, v any) (string, func(string) string) {
	t.Helper()
	path := filepath.Join(realTemp(t), "config.json")
	tulisConfig(t, path, v)
	return path, envOf(map[string]string{envConfig: path})
}

func TestConfigFileBasic(t *testing.T) {
	a, b := realTemp(t), realTemp(t)
	_, env := cfgFile(t, map[string]any{
		"bases":           []baseEntry{{a, "rw"}, {b, "ro"}},
		"max_write_bytes": 1234,
		"max_read_chars":  77,
		"max_entries":     5,
	})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.baseDir != a || len(cfg.otherDirs) != 1 || cfg.otherDirs[0] != b {
		t.Errorf("baseDir=%q otherDirs=%v", cfg.baseDir, cfg.otherDirs)
	}
	if cfg.maxWriteBytes != 1234 || cfg.maxReadChars != 77 || cfg.maxEntries != 5 {
		t.Errorf("batas salah: %d %d %d", cfg.maxWriteBytes, cfg.maxReadChars, cfg.maxEntries)
	}
	if err := cfg.checkWritable(filepath.Join(a, "x")); err != nil {
		t.Errorf("folder rw harus boleh ditulis: %v", err)
	}
	if err := cfg.checkWritable(filepath.Join(b, "x")); err == nil || !strings.Contains(err.Error(), "ro") {
		t.Errorf("folder ro harus ditolak dengan pesan jelas, dapat %v", err)
	}
}

func TestConfigFileModeDefaultIsReadOnly(t *testing.T) {
	a := realTemp(t)
	_, env := cfgFile(t, map[string]any{"bases": []baseEntry{{Path: a}}})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.checkWritable(filepath.Join(a, "x")); err == nil {
		t.Error("mode bawaan harus ro (aman)")
	}
}

func TestConfigFileDefaultLimits(t *testing.T) {
	a := realTemp(t)
	_, env := cfgFile(t, map[string]any{"bases": []baseEntry{{a, "rw"}}})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.maxWriteBytes != defaultMaxWriteBytes || cfg.maxReadChars != defaultMaxReadChars || cfg.maxEntries != defaultMaxEntries {
		t.Errorf("batas bawaan salah: %d %d %d", cfg.maxWriteBytes, cfg.maxReadChars, cfg.maxEntries)
	}
}

func TestConfigFileInvalid(t *testing.T) {
	a := filepath.ToSlash(realTemp(t))
	ok := `"bases":[{"path":"` + a + `","mode":"rw"}]`
	cases := map[string]string{
		"json rusak":         `{"bases": [`,
		"field tak dikenal":  `{"bases":[{"path":"` + a + `","mod":"rw"}]}`,
		"mode tak valid":     `{"bases":[{"path":"` + a + `","mode":"rwx"}]}`,
		"bases kosong":       `{"bases":[]}`,
		"path kosong":        `{"bases":[{"path":"","mode":"rw"}]}`,
		"batas negatif":      `{` + ok + `,"max_entries":-1}`,
		"folder tidak ada":   `{"bases":[{"path":"` + a + `/tidak-ada","mode":"rw"}]}`,
		"field level atas":   `{` + ok + `,"extra":1}`,
		"data setelah objek": `{` + ok + `} {}`,
	}
	for name, isi := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(realTemp(t), "config.json")
			tulisMentah(t, path, isi)
			if _, err := loadConfig(envOf(map[string]string{envConfig: path})); err == nil {
				t.Error("harus gagal")
			}
		})
	}
}

func TestConfigFileInsideBaseRejected(t *testing.T) {
	a := realTemp(t)
	path := filepath.Join(a, "config.json")
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "rw"}}})
	_, err := loadConfig(envOf(map[string]string{envConfig: path}))
	if err == nil || !strings.Contains(err.Error(), "di dalam folder dasar") {
		t.Errorf("config di dalam folder dasar harus ditolak, dapat %v", err)
	}
}

func TestConfigFileFallbackToEnv(t *testing.T) {
	a := realTemp(t)
	missing := filepath.Join(realTemp(t), "tidak-ada.json")
	cfg, err := loadConfig(envOf(map[string]string{envConfig: missing, envBase: a}))
	if err != nil {
		t.Fatalf("harus jatuh ke MCP_FILE_BASE: %v", err)
	}
	if cfg.baseDir != a {
		t.Errorf("baseDir=%q", cfg.baseDir)
	}
	if err := cfg.checkWritable(filepath.Join(a, "x")); err != nil {
		t.Errorf("env bersifat rw: %v", err)
	}
	if _, err := loadConfig(envOf(map[string]string{envConfig: missing})); err == nil || !strings.Contains(err.Error(), envConfig) {
		t.Errorf("tanpa cadangan harus gagal menyebut %s, dapat %v", envConfig, err)
	}
}

func TestConfigFileFallbackThenReload(t *testing.T) {
	a := realTemp(t)
	path := filepath.Join(realTemp(t), "config.json")
	cfg, err := loadConfig(envOf(map[string]string{envConfig: path, envBase: a}))
	if err != nil {
		t.Fatal(err)
	}
	b := realTemp(t)
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{b, "ro"}}})
	cfg.refresh()
	if cfg.baseDir != b {
		t.Errorf("config yang baru muncul harus dimuat, baseDir=%q", cfg.baseDir)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	if got := defaultConfigPath(envOf(nil)); got != "" {
		t.Errorf("tanpa lingkungan harus kosong, dapat %q", got)
	}
	got := defaultConfigPath(envOf(map[string]string{"APPDATA": `C:\Users\x\AppData\Roaming`, "XDG_CONFIG_HOME": "/home/x/.config", "HOME": "/home/x"}))
	if got == "" || !strings.HasSuffix(filepath.ToSlash(got), "mcp-file-manager/config.json") {
		t.Errorf("path bawaan salah: %q", got)
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	rw, ro := realTemp(t), realTemp(t)
	if err := os.WriteFile(filepath.Join(ro, "a.txt"), []byte("isi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ro, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, env := cfgFile(t, map[string]any{"bases": []baseEntry{{rw, "rw"}, {ro, "ro"}}})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	m := &manager{cfg: cfg}
	p := func(s string) string { return filepath.Join(ro, s) }

	blocked := map[string]result{
		"write_file":        m.writeFile(writeInput{Path: p("baru.txt"), Content: "x"}),
		"write_file append": m.writeFile(writeInput{Path: p("a.txt"), Content: "x", Mode: "append"}),
		"edit_block":        m.editBlock(editInput{FilePath: p("a.txt"), OldString: "isi", NewString: "ubah"}),
		"delete_file":       m.deleteFile(pathInput{Path: p("a.txt")}),
		"delete_dir":        m.deleteDir(deleteDirInput{Path: p("sub")}),
		"create_directory":  m.createDirectory(pathInput{Path: p("dir-baru")}),
		"move dari ro":      m.moveFile(moveInput{Source: p("a.txt"), Destination: filepath.Join(rw, "a.txt")}),
		"move ke ro":        m.moveFile(moveInput{Source: filepath.Join(rw, "tidak-ada.txt"), Destination: p("b.txt")}),
	}
	for name, res := range blocked {
		msg, _ := res["error"].(string)
		if !strings.Contains(msg, "hanya baca") {
			t.Errorf("%s: harus ditolak (hanya baca), hasil %v", name, res)
		}
	}
	if b, err := os.ReadFile(p("a.txt")); err != nil || string(b) != "isi\n" {
		t.Errorf("isi file di folder ro tidak boleh berubah: %q %v", b, err)
	}
	if _, err := os.Stat(p("baru.txt")); err == nil {
		t.Error("file baru tidak boleh terbentuk di folder ro")
	}
	if _, err := os.Stat(p("sub")); err != nil {
		t.Error("folder di ro tidak boleh terhapus")
	}

	if res := m.readFile(readInput{Path: p("a.txt")}); res["error"] != nil {
		t.Errorf("membaca di folder ro harus boleh: %v", res)
	}
	if res := m.deleteDir(deleteDirInput{Path: p("sub"), DryRun: true}); res["error"] != nil {
		t.Errorf("dry_run di folder ro harus boleh: %v", res)
	}
	if res := m.writeFile(writeInput{Path: filepath.Join(rw, "ok.txt"), Content: "x"}); res["error"] != nil {
		t.Errorf("menulis di folder rw harus boleh: %v", res)
	}
}

func TestReloadAppliesChanges(t *testing.T) {
	a, b := realTemp(t), realTemp(t)
	path, env := cfgFile(t, map[string]any{"bases": []baseEntry{{a, "ro"}}, "max_entries": 3})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	m := &manager{cfg: cfg}
	if res := m.writeFile(writeInput{Path: filepath.Join(a, "x.txt"), Content: "1"}); res["error"] == nil {
		t.Fatal("awalnya ro, tulis harus ditolak")
	}

	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "rw"}, {b, "ro"}}, "max_entries": 9})
	cfg.refresh()
	if res := m.writeFile(writeInput{Path: filepath.Join(a, "x.txt"), Content: "1"}); res["error"] != nil {
		t.Errorf("setelah muat ulang folder rw, tulis harus boleh: %v", res)
	}
	if len(cfg.otherDirs) != 1 || cfg.otherDirs[0] != b || cfg.maxEntries != 9 {
		t.Errorf("perubahan tidak termuat: %v %d", cfg.otherDirs, cfg.maxEntries)
	}
}

func TestReloadKeepsLastValidOnBrokenFile(t *testing.T) {
	a := realTemp(t)
	path, env := cfgFile(t, map[string]any{"bases": []baseEntry{{a, "rw"}}})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	tulisMentah(t, path, `{"bases": [`)
	cfg.refresh()
	if cfg.baseDir != a || cfg.checkWritable(filepath.Join(a, "x")) != nil {
		t.Fatal("file rusak: config valid terakhir harus tetap dipakai")
	}
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "ro"}}})
	cfg.refresh()
	if cfg.checkWritable(filepath.Join(a, "x")) == nil {
		t.Error("setelah file diperbaiki, config baru harus dimuat")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	cfg.refresh()
	if cfg.checkWritable(filepath.Join(a, "x")) == nil {
		t.Error("file hilang: config valid terakhir (ro) harus tetap dipakai")
	}
}

func TestReloadRejectsConfigInsideNewBase(t *testing.T) {
	a := realTemp(t)
	cfgDir := realTemp(t)
	path := filepath.Join(cfgDir, "config.json")
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "rw"}}})
	cfg, err := loadConfig(envOf(map[string]string{envConfig: path}))
	if err != nil {
		t.Fatal(err)
	}
	// Folder dasar baru memuat file config itu sendiri: harus ditolak.
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "rw"}, {cfgDir, "rw"}}})
	cfg.refresh()
	if len(cfg.otherDirs) != 0 {
		t.Errorf("config yang melebarkan akses ke folder berisi file config harus ditolak, otherDirs=%v", cfg.otherDirs)
	}
}

func TestEnvOnlyHasNoReload(t *testing.T) {
	a := realTemp(t)
	cfg, err := loadConfig(envOf(map[string]string{envBase: a}))
	if err != nil {
		t.Fatal(err)
	}
	cfg.refresh() // tidak boleh panik atau mengubah apa pun
	if cfg.baseDir != a {
		t.Errorf("baseDir=%q", cfg.baseDir)
	}
}

func TestReloadThroughProtocol(t *testing.T) {
	a := realTemp(t)
	path, env := cfgFile(t, map[string]any{"bases": []baseEntry{{a, "ro"}}})
	cfg, err := loadConfig(env)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := newServer(cfg).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "uji", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	target := filepath.Join(a, "x.txt")
	write := func() string {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "write_file", Arguments: map[string]any{"path": target, "content": "1"}})
		if err != nil {
			t.Fatal(err)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("hasil bukan teks: %T", res.Content[0])
		}
		return tc.Text
	}
	if got := write(); !strings.Contains(got, "hanya baca") {
		t.Fatalf("folder ro: tulis harus ditolak, hasil %s", got)
	}
	tulisConfig(t, path, map[string]any{"bases": []baseEntry{{a, "rw"}}})
	if got := write(); strings.Contains(got, "error") {
		t.Fatalf("setelah config diubah ke rw, tulis harus berhasil, hasil %s", got)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "1" {
		t.Errorf("file tidak tertulis: %q %v", b, err)
	}
}
