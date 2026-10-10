package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func newTestManager(t *testing.T) *manager {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &manager{cfg: &config{
		baseDir:       base,
		maxWriteBytes: defaultMaxWriteBytes,
		maxReadChars:  defaultMaxReadChars,
		maxEntries:    defaultMaxEntries,
	}}
}

func (m *manager) abs(rel string) string { return filepath.Join(m.cfg.baseDir, rel) }

func (m *manager) raw(t *testing.T, rel string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(m.abs(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.abs(rel), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (m *manager) content(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(m.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (m *manager) write(rel, content string) result {
	return m.writeFile(writeInput{Path: rel, Content: content})
}

func (m *manager) lines(t *testing.T, rel string) []string {
	t.Helper()
	r := m.readFile(readInput{Path: rel})
	if hasError(r) {
		t.Fatalf("read_file %s: %v", rel, r["error"])
	}
	return r["lines"].([]string)
}

func hasError(r result) bool { _, ok := r["error"]; return ok }

func equalLines(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00") && len(a) == len(b)
}

func entryNames(r result) []string { return r["entries"].([]string) }

func TestWriteAndReadRoundtrip(t *testing.T) {
	m := newTestManager(t)
	if r := m.write("hello.txt", "halo dunia"); hasError(r) {
		t.Fatalf("write gagal: %v", r)
	}
	r := m.readFile(readInput{Path: "hello.txt"})
	if got := strings.Join(r["lines"].([]string), ""); got != "halo dunia" {
		t.Errorf("isi = %q", got)
	}
	if r["total_lines"] != 1 {
		t.Errorf("total_lines = %v", r["total_lines"])
	}
}

func TestAppendMode(t *testing.T) {
	m := newTestManager(t)
	m.write("log.txt", "baris1\n")
	if r := m.writeFile(writeInput{Path: "log.txt", Content: "baris2\n", Mode: "append"}); hasError(r) {
		t.Fatalf("append gagal: %v", r)
	}
	if got := m.lines(t, "log.txt"); !equalLines(got, []string{"baris1\n", "baris2\n"}) {
		t.Errorf("lines = %q", got)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	m := newTestManager(t)
	for _, p := range []string{"../secret.txt", "sub/../../secret.txt"} {
		if r := m.readFile(readInput{Path: p}); !hasError(r) {
			t.Errorf("%q seharusnya ditolak", p)
		}
	}
}

func TestAbsolutePathEscapeBlocked(t *testing.T) {
	m := newTestManager(t)
	outside := filepath.Join(filepath.Dir(m.cfg.baseDir), "outside.txt")
	if err := os.WriteFile(outside, []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })
	if r := m.readFile(readInput{Path: outside}); !hasError(r) {
		t.Error("path absolut di luar BASE_DIR seharusnya ditolak")
	}
}

func TestAbsolutePathInsideAccepted(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "ok")
	if r := m.readFile(readInput{Path: m.abs("a.txt")}); hasError(r) {
		t.Errorf("path absolut di dalam BASE_DIR seharusnya diterima: %v", r["error"])
	}
}

func TestNullByteAndEmptyPathRejected(t *testing.T) {
	m := newTestManager(t)
	if r := m.readFile(readInput{Path: "bad\x00name.txt"}); !hasError(r) {
		t.Error("null byte seharusnya ditolak")
	}
	if r := m.readFile(readInput{Path: ""}); !hasError(r) {
		t.Error("path kosong seharusnya ditolak")
	}
}

func TestListDirectoryDepthAndPrefixes(t *testing.T) {
	m := newTestManager(t)
	m.write("sub/deep/a.py", "print(1)")
	m.write("top.txt", "x")

	r := m.listDirectory(listInput{Depth: ptr(1)})
	if got := strings.Join(entryNames(r), ","); got != "[DIR] sub,[FILE] top.txt" {
		t.Errorf("depth 1: %s", got)
	}
	r = m.listDirectory(listInput{})
	if got := strings.Join(entryNames(r), ","); got != "[DIR] sub,[DIR] sub/deep,[FILE] top.txt" {
		t.Errorf("depth 2: %s", got)
	}
	r = m.listDirectory(listInput{Depth: ptr(3)})
	if got := strings.Join(entryNames(r), ","); !strings.Contains(got, "[FILE] sub/deep/a.py") {
		t.Errorf("depth 3: %s", got)
	}
	if _, ok := r["truncated"]; ok {
		t.Error("truncated tidak boleh ada bila hasil utuh")
	}
}

func TestListDirectoryListsButSkipsHeavyDirs(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, ".git/config", "junk")
	m.raw(t, ".venv/lib/pkg.py", "junk")
	m.raw(t, "node_modules/x/index.js", "junk")
	m.raw(t, "real.py", "print('ok')")

	names := entryNames(m.listDirectory(listInput{Depth: ptr(5)}))
	want := "[DIR] .git,[DIR] .venv,[DIR] node_modules,[FILE] real.py"
	if strings.Join(names, ",") != want {
		t.Errorf("entries = %v, seharusnya %s", names, want)
	}
}

func TestListDirectoryCapsEntryCount(t *testing.T) {
	m := newTestManager(t)
	m.cfg.maxEntries = 3
	for _, n := range []string{"f0", "f1", "f2", "f3", "f4", "f5"} {
		m.write(n+".txt", "x")
	}
	r := m.listDirectory(listInput{})
	if len(entryNames(r)) != 3 || r["truncated"] != true {
		t.Errorf("entries=%v truncated=%v", entryNames(r), r["truncated"])
	}
}

func TestListDirectoryErrors(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "f.txt", "x")
	for name, in := range map[string]listInput{
		"path tidak ada":    {Path: ptr("nope")},
		"file bukan folder": {Path: ptr("f.txt")},
		"depth 0":           {Depth: ptr(0)},
		"di luar BASE_DIR":  {Path: ptr("../x")},
	} {
		if !hasError(m.listDirectory(in)) {
			t.Errorf("%s: seharusnya error", name)
		}
	}
}

func TestReadFileNegativeOffsetReadsTail(t *testing.T) {
	m := newTestManager(t)
	m.write("n.txt", "1\n2\n3\n4\n5\n")
	r := m.readFile(readInput{Path: "n.txt", Offset: -2})
	if got := r["lines"].([]string); !equalLines(got, []string{"4\n", "5\n"}) {
		t.Errorf("lines = %q", got)
	}
	if r["offset"] != 3 || r["total_lines"] != 5 || r["truncated"] != false {
		t.Errorf("r = %v", r)
	}
	// offset negatif lebih besar dari jumlah baris: baca dari awal.
	r = m.readFile(readInput{Path: "n.txt", Offset: -50})
	if len(r["lines"].([]string)) != 5 || r["offset"] != 0 {
		t.Errorf("r = %v", r)
	}
	// length diabaikan untuk offset negatif.
	r = m.readFile(readInput{Path: "n.txt", Offset: -1, Length: ptr(0)})
	if hasError(r) || len(r["lines"].([]string)) != 1 {
		t.Errorf("r = %v", r)
	}
}

func TestReadMultipleFiles(t *testing.T) {
	m := newTestManager(t)
	m.write("a.txt", "A\n")
	m.write("b.txt", "B\n")
	r := m.readMultipleFiles(multiInput{Paths: []string{"a.txt", "nope.txt", "b.txt"}})
	files := r["files"].([]result)
	if len(files) != 3 {
		t.Fatalf("files = %v", files)
	}
	if hasError(files[0]) || files[0]["path"] != "a.txt" || hasError(files[2]) {
		t.Errorf("file valid seharusnya terbaca: %v", files)
	}
	if !hasError(files[1]) || files[1]["path"] != "nope.txt" {
		t.Errorf("file hilang seharusnya memuat error dan path: %v", files[1])
	}
}

func TestReadMultipleFilesLimitsAndBudget(t *testing.T) {
	m := newTestManager(t)
	if !hasError(m.readMultipleFiles(multiInput{})) {
		t.Error("paths kosong seharusnya error")
	}
	many := make([]string, maxReadFiles+1)
	for i := range many {
		many[i] = "x.txt"
	}
	if !hasError(m.readMultipleFiles(multiInput{Paths: many})) {
		t.Error("lebih dari 20 file seharusnya error")
	}
	m.cfg.maxReadChars = 5
	m.write("a.txt", "aaaaa\n")
	m.write("b.txt", "bbb\n")
	files := m.readMultipleFiles(multiInput{Paths: []string{"a.txt", "b.txt"}})["files"].([]result)
	if files[0]["truncated"] != true || !hasError(files[1]) {
		t.Errorf("batas karakter dibagi antar file: %v", files)
	}
}

func TestGetFileInfo(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "satu\ndua\ntiga\n")
	m.raw(t, "bin.dat", "ab\x00cd")
	m.createDirectory(pathInput{Path: "d"})

	r := m.getFileInfo(pathInput{Path: "a.txt"})
	if r["type"] != "file" || r["size"] != int64(14) || r["lines"] != 3 {
		t.Errorf("a.txt = %v", r)
	}
	if mod, _ := r["modified"].(string); !strings.HasSuffix(mod, "Z") || len(mod) != 20 {
		t.Errorf("modified = %v", r["modified"])
	}
	if _, ok := m.getFileInfo(pathInput{Path: "bin.dat"})["lines"]; ok {
		t.Error("file biner tidak boleh punya lines")
	}
	if d := m.getFileInfo(pathInput{Path: "d"}); d["type"] != "dir" || d["size"] != nil {
		t.Errorf("d = %v", d)
	}
	if !hasError(m.getFileInfo(pathInput{Path: "nope"})) || !hasError(m.getFileInfo(pathInput{Path: "../x"})) {
		t.Error("path tidak ada dan di luar BASE_DIR seharusnya error")
	}
}

func TestStartSearchFiles(t *testing.T) {
	m := newTestManager(t)
	m.write("src/Main.py", "x")
	m.write("src/util.py", "x")
	m.write("src/notes.txt", "x")
	m.raw(t, ".git/main.py", "x")

	search := func(in searchInput) []string { return m.startSearch(in)["matches"].([]string) }
	if got := strings.Join(search(searchInput{Pattern: "main"}), ","); got != "src/Main.py" {
		t.Errorf("substring tanpa beda huruf: %s", got)
	}
	if got := strings.Join(search(searchInput{Pattern: "*.py"}), ","); got != "src/Main.py,src/util.py" {
		t.Errorf("wildcard: %s", got)
	}
	if got := search(searchInput{Pattern: "main", IgnoreCase: ptr(false)}); len(got) != 0 {
		t.Errorf("ignoreCase=false: %v", got)
	}
	if got := strings.Join(search(searchInput{Pattern: "src"}), ","); got != "src/" {
		t.Errorf("folder diberi akhiran /: %s", got)
	}
	if got := search(searchInput{Pattern: "config"}); len(got) != 0 {
		t.Errorf("isi folder berat tidak dicari: %v", got)
	}
}

func TestStartSearchContent(t *testing.T) {
	m := newTestManager(t)
	m.write("a.py", "import os\nHalo Dunia\n")
	m.write("b.txt", "halo lagi\r\n")
	m.raw(t, "bin.dat", "halo\x00biner")
	m.raw(t, ".git/x", "halo")

	r := m.startSearch(searchInput{Pattern: "halo", SearchType: ptr("content")})
	if got := strings.Join(r["matches"].([]string), "|"); got != "a.py:2: Halo Dunia|b.txt:1: halo lagi" {
		t.Errorf("matches = %s", got)
	}
	r = m.startSearch(searchInput{Pattern: "halo", SearchType: ptr("content"), FilePattern: ptr("*.py")})
	if got := r["matches"].([]string); len(got) != 1 || !strings.HasPrefix(got[0], "a.py:2") {
		t.Errorf("filePattern: %v", got)
	}
	r = m.startSearch(searchInput{Pattern: "halo", SearchType: ptr("content"), IgnoreCase: ptr(false)})
	if got := r["matches"].([]string); len(got) != 1 || !strings.HasPrefix(got[0], "b.txt:1") {
		t.Errorf("ignoreCase=false: %v", got)
	}
}

func TestStartSearchLimitsAndErrors(t *testing.T) {
	m := newTestManager(t)
	for _, n := range []string{"a1", "a2", "a3", "a4"} {
		m.write(n+".txt", "x")
	}
	r := m.startSearch(searchInput{Pattern: "a", MaxResults: ptr(2)})
	if len(r["matches"].([]string)) != 2 || r["truncated"] != true {
		t.Errorf("r = %v", r)
	}
	long := strings.Repeat("y", 300)
	m.write("long.txt", "zzz"+long+"\n")
	r = m.startSearch(searchInput{Pattern: "zzz", SearchType: ptr("content")})
	if line := r["matches"].([]string)[0]; len([]rune(line)) != len("long.txt:1: ")+maxMatchChars {
		t.Errorf("teks baris harus dipotong %d karakter: %d", maxMatchChars, len([]rune(line)))
	}
	for name, in := range map[string]searchInput{
		"pattern kosong":    {Pattern: ""},
		"searchType salah":  {Pattern: "a", SearchType: ptr("regex")},
		"maxResults 0":      {Pattern: "a", MaxResults: ptr(0)},
		"path tidak ada":    {Pattern: "a", Path: ptr("nope")},
		"path di luar BASE": {Pattern: "a", Path: ptr("../x")},
	} {
		if !hasError(m.startSearch(in)) {
			t.Errorf("%s: seharusnya error", name)
		}
	}
}

func TestWildcard(t *testing.T) {
	cases := []struct {
		pattern, text string
		want          bool
	}{
		{"*.py", "a.py", true}, {"*.py", "a.pyc", false}, {"a?c", "abc", true},
		{"a?c", "ac", false}, {"*", "", true}, {"a*b*c", "aXXbYYc", true}, {"a*b", "abX", false},
	}
	for _, c := range cases {
		if got := wild(c.pattern, c.text); got != c.want {
			t.Errorf("wild(%q, %q) = %v", c.pattern, c.text, got)
		}
	}
}

func TestMoveFile(t *testing.T) {
	m := newTestManager(t)
	m.write("old.txt", "isi")
	if r := m.moveFile(moveInput{Source: "old.txt", Destination: "new.txt"}); r["moved"] != true {
		t.Fatalf("move gagal: %v", r)
	}
	if got := m.lines(t, "new.txt"); !equalLines(got, []string{"isi"}) {
		t.Errorf("lines = %q", got)
	}
	if r := m.readFile(readInput{Path: "old.txt"}); !hasError(r) {
		t.Error("sumber seharusnya sudah tidak ada")
	}
}

func TestMoveFileNoOverwriteFails(t *testing.T) {
	m := newTestManager(t)
	m.write("a.txt", "A")
	m.write("b.txt", "B")
	if r := m.moveFile(moveInput{Source: "a.txt", Destination: "b.txt"}); !hasError(r) {
		t.Error("tujuan ada tanpa overwrite seharusnya error")
	}
	if r := m.moveFile(moveInput{Source: "a.txt", Destination: "b.txt", Overwrite: true}); r["moved"] != true {
		t.Errorf("overwrite seharusnya berhasil: %v", r)
	}
	if got := m.content(t, "b.txt"); got != "A" {
		t.Errorf("isi b.txt = %q", got)
	}
}

func TestMoveFileGuards(t *testing.T) {
	m := newTestManager(t)
	m.write("dir/a.txt", "A")
	cases := []struct {
		name string
		in   moveInput
	}{
		{"sumber sama dengan tujuan", moveInput{Source: "dir/a.txt", Destination: "dir/a.txt", Overwrite: true}},
		{"tujuan di dalam sumber", moveInput{Source: "dir", Destination: "dir/sub", Overwrite: true}},
		{"tujuan induk sumber", moveInput{Source: "dir/a.txt", Destination: "dir", Overwrite: true}},
	}
	for _, c := range cases {
		if r := m.moveFile(c.in); !hasError(r) {
			t.Errorf("%s: seharusnya ditolak", c.name)
		}
	}
	if got := m.content(t, "dir/a.txt"); got != "A" {
		t.Errorf("sumber rusak setelah penjaga: %q", got)
	}
}

func TestEditBlockNoMatch(t *testing.T) {
	m := newTestManager(t)
	m.write("f.txt", "abc")
	r := m.editBlock(editInput{FilePath: "f.txt", OldString: "xyz", NewString: "123"})
	if !hasError(r) {
		t.Errorf("r = %v", r)
	}
	if got := m.content(t, "f.txt"); got != "abc" {
		t.Errorf("isi berubah: %q", got)
	}
}

func TestEditBlockExpectedReplacements(t *testing.T) {
	m := newTestManager(t)
	m.write("f.txt", "aa aa aa")
	// Bawaan 1: tiga kemunculan ditolak dan file tidak berubah.
	if r := m.editBlock(editInput{FilePath: "f.txt", OldString: "aa", NewString: "bb"}); !hasError(r) {
		t.Fatalf("kemunculan ganda tanpa expected_replacements seharusnya error: %v", r)
	}
	if got := m.content(t, "f.txt"); got != "aa aa aa" {
		t.Errorf("isi berubah: %q", got)
	}
	r := m.editBlock(editInput{FilePath: "f.txt", OldString: "aa", NewString: "bb", ExpectedReplacements: ptr(3)})
	if hasError(r) || r["replacements"] != 3 || m.content(t, "f.txt") != "bb bb bb" {
		t.Errorf("r = %v isi = %q", r, m.content(t, "f.txt"))
	}
	m.write("g.txt", "aa aa")
	if r := m.editBlock(editInput{FilePath: "g.txt", OldString: "aa", NewString: "x", ExpectedReplacements: ptr(3)}); !hasError(r) {
		t.Error("jumlah tidak cocok seharusnya error")
	}
	if r := m.editBlock(editInput{FilePath: "g.txt", OldString: "aa", NewString: "x", ExpectedReplacements: ptr(0)}); !hasError(r) {
		t.Error("expected_replacements 0 seharusnya error")
	}
}

func TestWriteFileModes(t *testing.T) {
	m := newTestManager(t)
	m.write("a.txt", "satu\n")
	if r := m.writeFile(writeInput{Path: "a.txt", Content: "dua\n", Mode: "append"}); hasError(r) || r["written"] != 4 {
		t.Fatalf("append gagal: %v", r)
	}
	if got := m.content(t, "a.txt"); got != "satu\ndua\n" {
		t.Errorf("isi = %q", got)
	}
	if r := m.writeFile(writeInput{Path: "a.txt", Content: "baru", Mode: "rewrite"}); hasError(r) {
		t.Fatalf("rewrite gagal: %v", r)
	}
	if got := m.content(t, "a.txt"); got != "baru" {
		t.Errorf("isi = %q", got)
	}
	// append ke file yang belum ada membuat file (dan folder induknya).
	if r := m.writeFile(writeInput{Path: "x/y.txt", Content: "z", Mode: "append"}); hasError(r) {
		t.Errorf("append file baru gagal: %v", r)
	}
	if r := m.writeFile(writeInput{Path: "a.txt", Content: "q", Mode: "tambah"}); !hasError(r) {
		t.Error("mode tidak dikenal seharusnya error")
	}
}

func TestDeleteDirRequiresRecursiveForNonEmpty(t *testing.T) {
	m := newTestManager(t)
	m.createDirectory(pathInput{Path: "d"})
	m.write("d/x.txt", "x")
	if r := m.deleteDir(deleteDirInput{Path: "d"}); !hasError(r) {
		t.Error("folder tidak kosong tanpa recursive seharusnya error")
	}
	if r := m.deleteDir(deleteDirInput{Path: "d", Recursive: true}); r["deleted"] != true {
		t.Errorf("recursive seharusnya berhasil: %v", r)
	}
}

func TestDeleteBaseDirBlocked(t *testing.T) {
	m := newTestManager(t)
	for _, p := range []string{".", m.cfg.baseDir} {
		if r := m.deleteDir(deleteDirInput{Path: p, Recursive: true}); !hasError(r) {
			t.Errorf("menghapus BASE_DIR lewat %q seharusnya ditolak", p)
		}
	}
	if _, err := os.Stat(m.cfg.baseDir); err != nil {
		t.Fatal("BASE_DIR hilang")
	}
}

func TestDeleteFile(t *testing.T) {
	m := newTestManager(t)
	m.write("a.txt", "x")
	m.createDirectory(pathInput{Path: "d"})
	if r := m.deleteFile(pathInput{Path: "d"}); !hasError(r) {
		t.Error("delete_file pada folder seharusnya error")
	}
	if r := m.deleteFile(pathInput{Path: "nope.txt"}); !hasError(r) {
		t.Error("file tidak ada seharusnya error")
	}
	if r := m.deleteFile(pathInput{Path: "a.txt"}); r["deleted"] != true {
		t.Errorf("r = %v", r)
	}
}

func TestMkdirIdempotent(t *testing.T) {
	m := newTestManager(t)
	for i := 0; i < 2; i++ {
		if r := m.createDirectory(pathInput{Path: "a/b/c"}); r["created"] != true {
			t.Fatalf("mkdir ke-%d gagal: %v", i, r)
		}
	}
}
