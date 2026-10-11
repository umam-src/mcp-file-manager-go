package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envOf(vals map[string]string) func(string) string {
	return func(k string) string { return vals[k] }
}

func realTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// newMultiManager membuat manager dengan beberapa folder dasar lewat
// loadConfig, jalur yang sama dengan server sungguhan.
func newMultiManager(t *testing.T, n int) (*manager, []string) {
	t.Helper()
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = realTemp(t)
	}
	cfg, err := loadConfig(envOf(map[string]string{envBase: strings.Join(dirs, string(os.PathListSeparator))}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	return &manager{cfg: cfg}, dirs
}

func TestLoadConfigMultipleFolders(t *testing.T) {
	a, b := realTemp(t), realTemp(t)
	cfg, err := loadConfig(envOf(map[string]string{envBase: a + string(os.PathListSeparator) + b}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.baseDir != a || len(cfg.otherDirs) != 1 || cfg.otherDirs[0] != b {
		t.Errorf("baseDir=%q otherDirs=%v, harapan %q dan [%q]", cfg.baseDir, cfg.otherDirs, a, b)
	}
}

func TestLoadConfigSingleFolderUnchanged(t *testing.T) {
	a := realTemp(t)
	for _, raw := range []string{a, " " + a + " ", a + string(os.PathListSeparator)} {
		cfg, err := loadConfig(envOf(map[string]string{envBase: raw}))
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if cfg.baseDir != a || len(cfg.otherDirs) != 0 {
			t.Errorf("%q: baseDir=%q otherDirs=%v", raw, cfg.baseDir, cfg.otherDirs)
		}
	}
}

func TestLoadConfigRejectsOverlap(t *testing.T) {
	a := realTemp(t)
	sub := filepath.Join(a, "dalam")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	for name, raw := range map[string]string{
		"di dalam folder lain": a + sep + sub,
		"urutan terbalik":      sub + sep + a,
		"folder sama dua kali": a + sep + a,
	} {
		if _, err := loadConfig(envOf(map[string]string{envBase: raw})); err == nil || !strings.Contains(err.Error(), "tumpang tindih") {
			t.Errorf("%s: seharusnya ditolak karena tumpang tindih, dapat %v", name, err)
		}
	}
}

func TestLoadConfigSkipsUnavailableFolder(t *testing.T) {
	good, missing := realTemp(t), filepath.Join(realTemp(t), "tidak-ada")
	sep := string(os.PathListSeparator)

	cfg, err := loadConfig(envOf(map[string]string{envBase: missing + sep + good}))
	if err != nil {
		t.Fatalf("folder pertama hilang tetapi ada yang valid: %v", err)
	}
	if cfg.baseDir != good || len(cfg.otherDirs) != 0 {
		t.Errorf("baseDir=%q otherDirs=%v, harapan hanya %q", cfg.baseDir, cfg.otherDirs, good)
	}

	if _, err := loadConfig(envOf(map[string]string{envBase: missing + sep + filepath.Join(good, "juga-tidak-ada")})); err == nil || !strings.Contains(err.Error(), envBase) {
		t.Errorf("semua folder tidak tersedia seharusnya error yang menyebut variabelnya: %v", err)
	}

	file := filepath.Join(good, "berkas.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig(envOf(map[string]string{envBase: file + sep + good}))
	if err != nil || cfg.baseDir != good {
		t.Errorf("file bukan folder harus dilewati selama ada yang valid: cfg=%v err=%v", cfg, err)
	}
}

func TestSafeAcceptsAnyBaseAndRejectsOutside(t *testing.T) {
	m, dirs := newMultiManager(t, 2)
	a, b := dirs[0], dirs[1]

	if got, err := m.cfg.safe("rel.txt"); err != nil || got != filepath.Join(a, "rel.txt") {
		t.Errorf("path relatif harus dari folder pertama: got=%q err=%v", got, err)
	}
	for _, d := range []string{a, b} {
		p := filepath.Join(d, "x", "baru.txt")
		if got, err := m.cfg.safe(p); err != nil || got != p {
			t.Errorf("path absolut di %s harus diterima: got=%q err=%v", d, got, err)
		}
	}
	outside := filepath.Join(realTemp(t), "luar.txt")
	if _, err := m.cfg.safe(outside); err == nil || !strings.Contains(err.Error(), "Akses ditolak") {
		t.Errorf("di luar semua folder harus ditolak: %v", err)
	}
	if _, err := m.cfg.safe(filepath.Join(a, "..", filepath.Base(b), "x")); err != nil {
		t.Errorf("traversal yang berakhir di folder dasar lain boleh karena hasil akhirnya valid: %v", err)
	}
	if _, err := m.cfg.safe(filepath.Join("..", "luar.txt")); err == nil {
		t.Error("traversal keluar dari semua folder harus ditolak")
	}
}

func TestToolsWorkInSecondBase(t *testing.T) {
	m, dirs := newMultiManager(t, 2)
	f := filepath.Join(dirs[1], "sub", "catatan.txt")
	if r := m.writeFile(writeInput{Path: f, Content: "isi kedua\n"}); hasError(r) {
		t.Fatalf("write di folder kedua: %v", r)
	}
	if b, err := os.ReadFile(f); err != nil || string(b) != "isi kedua\n" {
		t.Fatalf("file tidak tertulis di folder kedua: %v %q", err, b)
	}
	if r := m.readFile(readInput{Path: f}); hasError(r) {
		t.Errorf("read di folder kedua: %v", r)
	}
	if r := m.listDirectory(listInput{Path: ptr(dirs[1]), Depth: ptr(2)}); hasError(r) {
		t.Errorf("list di folder kedua: %v", r)
	}
}

func TestDeleteDirRefusesEveryBase(t *testing.T) {
	m, dirs := newMultiManager(t, 3)
	for _, d := range dirs {
		if err := os.WriteFile(filepath.Join(d, "jangan-hilang.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if r := m.deleteDir(deleteDirInput{Path: d, Recursive: true}); !hasError(r) {
			t.Errorf("delete_dir pada folder dasar %s seharusnya ditolak: %v", d, r)
		}
		if _, err := os.Stat(filepath.Join(d, "jangan-hilang.txt")); err != nil {
			t.Errorf("isi %s hilang: %v", d, err)
		}
	}
	sub := filepath.Join(dirs[1], "boleh")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if r := m.deleteDir(deleteDirInput{Path: sub}); hasError(r) {
		t.Errorf("subfolder di folder kedua boleh dihapus: %v", r)
	}
}

func TestMoveBetweenBases(t *testing.T) {
	m, dirs := newMultiManager(t, 2)
	src := filepath.Join(dirs[0], "a.txt")
	dst := filepath.Join(dirs[1], "b.txt")
	if err := os.WriteFile(src, []byte("pindah"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := m.moveFile(moveInput{Source: src, Destination: dst})
	if hasError(r) {
		t.Fatalf("move antar-folder pada volume yang sama: %v", r)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "pindah" {
		t.Errorf("tujuan: %v %q", err, b)
	}
	if _, err := os.Stat(src); err == nil {
		t.Error("sumber seharusnya sudah tidak ada")
	}
	outside := filepath.Join(realTemp(t), "luar.txt")
	if r := m.moveFile(moveInput{Source: dst, Destination: outside}); !hasError(r) {
		t.Errorf("move ke luar semua folder harus ditolak: %v", r)
	}
}

func TestLinkInSecondBaseChecked(t *testing.T) {
	m, dirs := newMultiManager(t, 2)
	outside := realTemp(t)
	if err := os.WriteFile(filepath.Join(outside, "rahasia.txt"), []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkDirOrSkip(t, outside, filepath.Join(dirs[1], "keluar"))
	if r := m.readFile(readInput{Path: filepath.Join(dirs[1], "keluar", "rahasia.txt")}); !hasError(r) {
		t.Errorf("tautan di folder kedua ke luar semua folder harus ditolak: %v", r)
	}

	inFirst := filepath.Join(dirs[0], "dalam.txt")
	if err := os.WriteFile(inFirst, []byte("aman"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkDirOrSkip(t, dirs[0], filepath.Join(dirs[1], "ke-pertama"))
	if r := m.readFile(readInput{Path: filepath.Join(dirs[1], "ke-pertama", "dalam.txt")}); hasError(r) {
		t.Errorf("tautan ke folder dasar lain boleh karena tujuannya masih di dalam: %v", r)
	}
}

func TestInstructionsListFoldersOnlyWhenMultiple(t *testing.T) {
	one := realTemp(t)
	single := instructions([]string{one})
	if !strings.Contains(single, "BASE_DIR = "+one) {
		t.Errorf("satu folder: instructions harus menyebut BASE_DIR = %s: %s", one, single)
	}
	if strings.Contains(single, "pertama") {
		t.Errorf("satu folder: instructions tidak boleh menambah penjelasan folder lain: %s", single)
	}

	m, dirs := newMultiManager(t, 2)
	multi := instructions(m.cfg.allBases())
	for _, d := range dirs {
		if !strings.Contains(multi, d) {
			t.Errorf("beberapa folder: instructions harus menyebut %s: %s", d, multi)
		}
	}
	if !strings.Contains(multi, "pertama") {
		t.Errorf("beberapa folder: instructions harus menyebut path relatif dari folder pertama: %s", multi)
	}
}
