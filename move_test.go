package main

import (
	"os"
	"runtime"
	"testing"
)

// Bila rename gagal, tujuan yang akan ditimpa tidak boleh sudah terhapus.
// Di Windows file yang sedang terbuka (tanpa FILE_SHARE_DELETE) tidak bisa
// di-rename, sehingga kegagalan itu bisa dibuat secara andal.
func TestMoveOverwriteKeepsDestinationWhenRenameFails(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kegagalan rename hanya bisa dipicu andal di Windows")
	}
	m := newTestManager(t)
	m.raw(t, "a.txt", "A")
	m.raw(t, "b.txt", "B")
	f, err := os.Open(m.abs("a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := m.moveFile(moveInput{Source: "a.txt", Destination: "b.txt", Overwrite: true})
	if !hasError(r) {
		t.Skip("rename berhasil walau sumber terbuka; kegagalan tidak bisa dipicu")
	}
	if got := m.content(t, "b.txt"); got != "B" {
		t.Errorf("tujuan hilang atau berubah setelah move gagal: %q", got)
	}
	if got := m.content(t, "a.txt"); got != "A" {
		t.Errorf("sumber berubah setelah move gagal: %q", got)
	}
	assertNoBackups(t, m)
}

func TestMoveOverwriteReplacesFileAndDir(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "A")
	m.raw(t, "b.txt", "B")
	if r := m.moveFile(moveInput{Source: "a.txt", Destination: "b.txt", Overwrite: true}); r["moved"] != true {
		t.Fatalf("file ke file: %v", r)
	}
	if got := m.content(t, "b.txt"); got != "A" {
		t.Errorf("isi b.txt = %q", got)
	}

	m.raw(t, "src/x.txt", "X")
	m.raw(t, "dst/lama.txt", "lama")
	if r := m.moveFile(moveInput{Source: "src", Destination: "dst", Overwrite: true}); r["moved"] != true {
		t.Fatalf("folder ke folder: %v", r)
	}
	if got := m.content(t, "dst/x.txt"); got != "X" {
		t.Errorf("isi dst/x.txt = %q", got)
	}
	if _, err := os.Stat(m.abs("dst/lama.txt")); err == nil {
		t.Error("isi lama folder tujuan seharusnya sudah hilang")
	}
	assertNoBackups(t, m)
}

// assertNoBackups memastikan tidak ada file cadangan sementara yang tertinggal
// di BASE_DIR.
func assertNoBackups(t *testing.T, m *manager) {
	t.Helper()
	des, err := os.ReadDir(m.cfg.baseDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range des {
		if len(de.Name()) > len(backupPrefix) && de.Name()[:len(backupPrefix)] == backupPrefix {
			t.Errorf("cadangan sementara tertinggal: %s", de.Name())
		}
	}
}
