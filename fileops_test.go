package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReadFileOffsetAndLimit(t *testing.T) {
	m := newTestManager(t)
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&sb, "baris%d\n", i)
	}
	m.write("n.txt", sb.String())

	r := m.readFile(readInput{Path: "n.txt", Length: ptr(3), Offset: 2})
	if !equalLines(r["lines"].([]string), []string{"baris2\n", "baris3\n", "baris4\n"}) {
		t.Errorf("lines = %q", r["lines"])
	}
	if r["offset"] != 2 || r["total_lines"] != 10 || r["truncated"] != true || r["next_offset"] != 5 {
		t.Errorf("meta = %v", r)
	}

	last := m.readFile(readInput{Path: "n.txt", Length: ptr(100), Offset: r["next_offset"].(int)})
	if last["lines"].([]string)[0] != "baris5\n" || last["truncated"] != false {
		t.Errorf("last = %v", last)
	}
	if _, ok := last["next_offset"]; ok {
		t.Error("next_offset seharusnya tidak ada di halaman terakhir")
	}
}

func TestReadFileRejectsBadArguments(t *testing.T) {
	m := newTestManager(t)
	m.write("x.txt", "x")
	if hasError(m.readFile(readInput{Path: "x.txt", Offset: -1})) {
		t.Error("offset negatif (baca ekor file) seharusnya diterima")
	}
	if !hasError(m.readFile(readInput{Path: "x.txt", Length: ptr(0)})) {
		t.Error("length 0 seharusnya ditolak")
	}
	if !hasError(m.readFile(readInput{Path: "tidak-ada.txt"})) {
		t.Error("file tidak ada seharusnya error")
	}
	m.createDirectory(pathInput{Path: "d"})
	if !hasError(m.readFile(readInput{Path: "d"})) {
		t.Error("membaca folder seharusnya error")
	}
}

func TestReadFileCharCap(t *testing.T) {
	m := newTestManager(t)
	m.cfg.maxReadChars = 10
	m.write("big.txt", "aaaaaa\nbbbbbb\ncccccc\n")
	r := m.readFile(readInput{Path: "big.txt"})
	if got := strings.Join(r["lines"].([]string), ""); got != "aaaaaa\nbbb" {
		t.Errorf("isi = %q", got)
	}
	if r["truncated"] != true || r["total_lines"] != 3 {
		t.Errorf("meta = %v", r)
	}
}

func TestReadFileReportsLineEnding(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "crlf.txt", "a\r\nb\r\n")
	m.raw(t, "lf.txt", "a\nb\n")
	m.raw(t, "cr.txt", "a\rb\r")
	m.raw(t, "mixed.txt", "a\r\nb\nc\n")
	m.raw(t, "none.txt", "satu baris")
	m.raw(t, "empty.txt", "")

	crlf := m.readFile(readInput{Path: "crlf.txt"})
	if *crlf["line_ending"].(*string) != "CRLF" || !equalLines(crlf["lines"].([]string), []string{"a\n", "b\n"}) {
		t.Errorf("crlf = %v", crlf)
	}
	want := map[string]string{"lf.txt": "LF", "cr.txt": "CR", "mixed.txt": "mixed"}
	for file, eol := range want {
		r := m.readFile(readInput{Path: file})
		if got := r["line_ending"].(*string); got == nil || *got != eol {
			t.Errorf("%s: line_ending = %v, mau %s", file, got, eol)
		}
	}
	cr := m.readFile(readInput{Path: "cr.txt"})
	if !equalLines(cr["lines"].([]string), []string{"a\n", "b\n"}) {
		t.Errorf("CR-only seharusnya dinormalisasi: %q", cr["lines"])
	}
	for _, file := range []string{"none.txt", "empty.txt"} {
		r := m.readFile(readInput{Path: file})
		if r["line_ending"].(*string) != nil {
			t.Errorf("%s: line_ending seharusnya nil", file)
		}
	}
	if r := m.readFile(readInput{Path: "empty.txt"}); r["total_lines"] != 0 {
		t.Errorf("file kosong: total_lines = %v", r["total_lines"])
	}
}

func TestReadFileLongLineAcrossBuffer(t *testing.T) {
	m := newTestManager(t)
	long := strings.Repeat("x", 200_000)
	m.raw(t, "long.txt", long+"\r\nkedua\r\n")
	r := m.readFile(readInput{Path: "long.txt", Offset: 1})
	if !equalLines(r["lines"].([]string), []string{"kedua\n"}) || r["total_lines"] != 2 {
		t.Errorf("r = %v", r)
	}
	// Batas karakter menahan baris panjang tanpa memuat semuanya.
	r = m.readFile(readInput{Path: "long.txt"})
	if n := len(r["lines"].([]string)[0]); n != defaultMaxReadChars || r["truncated"] != true {
		t.Errorf("len = %d truncated = %v", n, r["truncated"])
	}
}

func TestReadFileInvalidUTF8Replaced(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "bin.txt", "a\xffb\n")
	r := m.readFile(readInput{Path: "bin.txt"})
	if got := r["lines"].([]string)[0]; got != "a\uFFFDb\n" {
		t.Errorf("lines = %q", got)
	}
}

func TestWriteNewFileUsesLF(t *testing.T) {
	m := newTestManager(t)
	m.write("new.txt", "x\ny\n")
	if got := m.content(t, "new.txt"); got != "x\ny\n" {
		t.Errorf("isi = %q", got)
	}
}

func TestWritePreservesCRLFOfExistingFile(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "w.txt", "a\r\nb\r\n")
	m.write("w.txt", "x\ny\n")
	if got := m.content(t, "w.txt"); got != "x\r\ny\r\n" {
		t.Errorf("isi = %q", got)
	}
}

func TestWriteFileOnDirectoryRejected(t *testing.T) {
	m := newTestManager(t)
	m.createDirectory(pathInput{Path: "d"})
	if !hasError(m.write("d", "x")) {
		t.Error("menulis ke folder seharusnya error")
	}
}

func TestWriteCreatesParentDirs(t *testing.T) {
	m := newTestManager(t)
	if r := m.write("a/b/c.txt", "ok"); hasError(r) {
		t.Fatalf("r = %v", r)
	}
}

func TestWriteCountsCharactersNotBytes(t *testing.T) {
	m := newTestManager(t)
	if r := m.write("u.txt", "héllo"); r["written"] != 5 {
		t.Errorf("written = %v, mau 5 karakter", r["written"])
	}
}

func TestAppendPreservesCRLF(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "a\r\n")
	m.writeFile(writeInput{Path: "a.txt", Content: "b\n", Mode: "append"})
	if got := m.content(t, "a.txt"); got != "a\r\nb\r\n" {
		t.Errorf("isi = %q", got)
	}
}

func TestEditPreservesCRLFWithMultilineMatch(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "e.txt", "one\r\ntwo\r\nthree\r\n")
	r := m.editBlock(editInput{FilePath: "e.txt", OldString: "one\ntwo", NewString: "uno\ndos"})
	if hasError(r) {
		t.Fatalf("r = %v", r)
	}
	if got := m.content(t, "e.txt"); got != "uno\r\ndos\r\nthree\r\n" {
		t.Errorf("isi = %q", got)
	}
}

func TestEditPreservesLFFile(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "e.txt", "one\ntwo\n")
	m.editBlock(editInput{FilePath: "e.txt", OldString: "two", NewString: "dua"})
	if got := m.content(t, "e.txt"); got != "one\ndua\n" {
		t.Errorf("isi = %q", got)
	}
}

func TestEditRejectsEmptyOldString(t *testing.T) {
	m := newTestManager(t)
	m.write("e.txt", "abc")
	if !hasError(m.editBlock(editInput{FilePath: "e.txt", OldString: "", NewString: "x"})) {
		t.Error("old_string kosong seharusnya ditolak")
	}
	if got := m.content(t, "e.txt"); got != "abc" {
		t.Errorf("isi berubah: %q", got)
	}
}

func TestEditRejectsNonUTF8(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "bin.dat", "a\xffb")
	if !hasError(m.editBlock(editInput{FilePath: "bin.dat", OldString: "a", NewString: "x"})) {
		t.Error("file bukan UTF-8 seharusnya ditolak agar tidak rusak")
	}
	if got := m.content(t, "bin.dat"); got != "a\xffb" {
		t.Errorf("isi berubah: %q", got)
	}
}

func TestWriteSizeLimit(t *testing.T) {
	m := newTestManager(t)
	m.cfg.maxWriteBytes = 10
	if !hasError(m.write("big.txt", strings.Repeat("x", 11))) {
		t.Error("tulis melebihi batas seharusnya error")
	}
	if _, err := os.Stat(m.abs("big.txt")); err == nil {
		t.Error("file tidak boleh dibuat saat melebihi batas")
	}
	if r := m.write("ok.txt", strings.Repeat("x", 10)); hasError(r) {
		t.Errorf("tepat di batas seharusnya berhasil: %v", r)
	}
}

func TestAppendSizeLimitCountsExistingContent(t *testing.T) {
	m := newTestManager(t)
	m.cfg.maxWriteBytes = 10
	m.write("a.txt", strings.Repeat("x", 8))
	if !hasError(m.writeFile(writeInput{Path: "a.txt", Content: "yyy", Mode: "append"})) {
		t.Error("append melebihi batas total seharusnya error")
	}
	if got := m.content(t, "a.txt"); got != strings.Repeat("x", 8) {
		t.Errorf("isi berubah: %q", got)
	}
}

func TestEditSizeLimit(t *testing.T) {
	m := newTestManager(t)
	m.cfg.maxWriteBytes = 10
	m.write("e.txt", "abc")
	if !hasError(m.editBlock(editInput{FilePath: "e.txt", OldString: "abc", NewString: strings.Repeat("x", 11)})) {
		t.Error("edit melebihi batas seharusnya error")
	}
	if got := m.content(t, "e.txt"); got != "abc" {
		t.Errorf("isi berubah: %q", got)
	}
}
