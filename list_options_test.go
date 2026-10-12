package main

import "testing"

func TestListDirectoryExclude(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "x")
	m.raw(t, "b.log", "x")
	m.raw(t, "keep/c.txt", "x")
	m.raw(t, "skip/d.txt", "x")
	m.raw(t, "tmp1/e.txt", "x")

	r := m.listDirectory(listInput{Exclude: []string{"skip", "*.log", "tmp?"}})
	if hasError(r) {
		t.Fatal(r["error"])
	}
	want := []string{"[FILE] a.txt", "[DIR] keep", "[FILE] keep/c.txt"}
	if !equalLines(entryNames(r), want) {
		t.Fatalf("entries = %v, want %v", entryNames(r), want)
	}
}

func TestListDirectoryExcludeNameIsExact(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "log.txt", "x")
	m.raw(t, "logs/a.txt", "x")

	r := m.listDirectory(listInput{Exclude: []string{"log"}})
	if hasError(r) {
		t.Fatal(r["error"])
	}
	if got := len(entryNames(r)); got != 3 {
		t.Fatalf("exclude tanpa wildcard harus cocok nama persis; entries = %v", entryNames(r))
	}
}

func TestListDirectoryMaxEntries(t *testing.T) {
	m := newTestManager(t)
	for _, n := range []string{"a", "b", "c", "d"} {
		m.raw(t, n+".txt", "x")
	}

	r := m.listDirectory(listInput{MaxEntries: ptr(2)})
	if hasError(r) {
		t.Fatal(r["error"])
	}
	if got := len(entryNames(r)); got != 2 || r["truncated"] != true {
		t.Fatalf("max_entries=2: entries=%v truncated=%v", entryNames(r), r["truncated"])
	}

	// Di atas batas server: dipotong ke batas server, bukan gagal.
	m.cfg.maxEntries = 3
	r = m.listDirectory(listInput{MaxEntries: ptr(100)})
	if hasError(r) {
		t.Fatal(r["error"])
	}
	if got := len(entryNames(r)); got != 3 || r["truncated"] != true {
		t.Fatalf("max_entries di atas batas server: entries=%v truncated=%v", entryNames(r), r["truncated"])
	}
}

func TestListDirectoryMaxEntriesInvalid(t *testing.T) {
	m := newTestManager(t)
	for _, n := range []int{0, -1} {
		if r := m.listDirectory(listInput{MaxEntries: ptr(n)}); !hasError(r) {
			t.Fatalf("max_entries=%d harus ditolak", n)
		}
	}
}
