package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pencarian isi tidak boleh membaca file di luar BASE_DIR lewat symlink file.
// Dilewati di Windows tanpa hak symlink (jalan di Linux dan Windows berhak).
func TestSearchContentDoesNotFollowSymlinkOutsideBaseDir(t *testing.T) {
	m := newTestManager(t)
	outside := filepath.Join(t.TempDir(), "rahasia.txt")
	if err := os.WriteFile(outside, []byte("kata-rahasia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, m.abs("pintas.txt"))
	m.write("biasa.txt", "kata-rahasia di dalam\n")

	r := m.startSearch(searchInput{Pattern: "kata-rahasia", SearchType: ptr("content")})
	got := strings.Join(r["matches"].([]string), "|")
	if strings.Contains(got, "pintas.txt") {
		t.Errorf("isi file di luar BASE_DIR terbaca lewat symlink: %s", got)
	}
	if !strings.Contains(got, "biasa.txt:1:") {
		t.Errorf("file biasa seharusnya tetap ditemukan: %s", got)
	}
}

func TestSearchContentFollowsSymlinkInsideBaseDir(t *testing.T) {
	m := newTestManager(t)
	m.write("asli.txt", "kata-unik\n")
	symlinkOrSkip(t, m.abs("asli.txt"), m.abs("pintas.txt"))

	r := m.startSearch(searchInput{Pattern: "kata-unik", SearchType: ptr("content")})
	got := strings.Join(r["matches"].([]string), "|")
	if !strings.Contains(got, "pintas.txt:1:") || !strings.Contains(got, "asli.txt:1:") {
		t.Errorf("symlink di dalam BASE_DIR seharusnya ikut dicari: %s", got)
	}
}
