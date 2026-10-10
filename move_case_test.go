package main

import (
	"os"
	"testing"
)

// names mengembalikan nama entri langsung di folder rel (dengan huruf aslinya).
func names(t *testing.T, m *manager, rel string) map[string]bool {
	t.Helper()
	des, err := os.ReadDir(m.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, de := range des {
		out[de.Name()] = true
	}
	return out
}

// Mengganti huruf besar/kecil nama harus berhasil. Di Windows (nama tidak
// membedakan huruf) a.txt dan A.txt adalah file yang sama; di Linux A.txt
// hanyalah nama baru.
func TestMoveCaseOnlyRenameFile(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "isi")
	r := m.moveFile(moveInput{Source: "a.txt", Destination: "A.txt"})
	if r["moved"] != true {
		t.Fatalf("ganti huruf seharusnya berhasil: %v", r)
	}
	got := names(t, m, ".")
	if !got["A.txt"] || got["a.txt"] {
		t.Errorf("nama di disk = %v, ingin hanya A.txt", got)
	}
	if c := m.content(t, "A.txt"); c != "isi" {
		t.Errorf("isi = %q", c)
	}
}

func TestMoveCaseOnlyRenameDir(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "dir/x.txt", "X")
	if r := m.moveFile(moveInput{Source: "dir", Destination: "Dir"}); r["moved"] != true {
		t.Fatalf("ganti huruf folder: %v", r)
	}
	got := names(t, m, ".")
	if !got["Dir"] || got["dir"] {
		t.Errorf("nama di disk = %v, ingin hanya Dir", got)
	}
	if c := m.content(t, "Dir/x.txt"); c != "X" {
		t.Errorf("isi = %q", c)
	}
}

// Pindah ke folder lain dengan nama beda huruf tetap pindah ke folder itu,
// bukan ganti nama di tempat.
func TestMoveDifferentCaseIntoOtherDir(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "isi")
	if r := m.moveFile(moveInput{Source: "a.txt", Destination: "sub/A.txt"}); r["moved"] != true {
		t.Fatalf("pindah: %v", r)
	}
	if got := names(t, m, "sub"); !got["A.txt"] {
		t.Errorf("isi sub = %v", got)
	}
	if got := names(t, m, "."); got["a.txt"] || got["A.txt"] {
		t.Errorf("sumber seharusnya sudah tidak ada di akar: %v", got)
	}
}

// Nama yang sama persis tetap ditolak.
func TestMoveSameNameStillRejected(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "isi")
	if r := m.moveFile(moveInput{Source: "a.txt", Destination: "a.txt"}); !hasError(r) {
		t.Error("nama sama persis seharusnya ditolak")
	}
}
