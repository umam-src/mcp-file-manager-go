package main

import (
	"fmt"
	"os"
	"testing"
)

func TestDeleteDirDryRunRecursiveReportsWithoutDeleting(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "d/a.txt", "abc")
	m.raw(t, "d/sub/b.txt", "de")
	m.raw(t, "d/sub/dalam/c.txt", "f")

	r := m.deleteDir(deleteDirInput{Path: "d", Recursive: true, DryRun: true})
	if hasError(r) {
		t.Fatalf("dry_run seharusnya berhasil: %v", r)
	}
	if r["dry_run"] != true || r["deleted"] != false || r["recursive"] != true {
		t.Errorf("penanda dry_run/deleted/recursive salah: %v", r)
	}
	if fmt.Sprint(r["files"]) != "3" || fmt.Sprint(r["dirs"]) != "2" || fmt.Sprint(r["bytes"]) != "6" {
		t.Errorf("hitungan salah (harapan files=3 dirs=2 bytes=6): %v", r)
	}
	for _, rel := range []string{"d/a.txt", "d/sub/b.txt", "d/sub/dalam/c.txt"} {
		if _, err := os.Stat(m.abs(rel)); err != nil {
			t.Errorf("%s hilang setelah dry_run: %v", rel, err)
		}
	}
}

func TestDeleteDirDryRunNonRecursive(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "penuh/x.txt", "x")
	m.createDirectory(pathInput{Path: "kosong"})

	r := m.deleteDir(deleteDirInput{Path: "penuh", DryRun: true})
	if hasError(r) || r["dry_run"] != true || r["deleted"] != false || r["empty"] != false || r["would_delete"] != false {
		t.Errorf("folder tidak kosong tanpa recursive: harapan empty=false would_delete=false tanpa error: %v", r)
	}
	r = m.deleteDir(deleteDirInput{Path: "kosong", DryRun: true})
	if hasError(r) || r["empty"] != true || r["would_delete"] != true || r["deleted"] != false {
		t.Errorf("folder kosong: harapan empty=true would_delete=true: %v", r)
	}
	for _, rel := range []string{"penuh/x.txt", "kosong"} {
		if _, err := os.Stat(m.abs(rel)); err != nil {
			t.Errorf("%s hilang setelah dry_run: %v", rel, err)
		}
	}
}

func TestDeleteDirDryRunOnLinkReportsOnlyLink(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "asli/data.txt", "penting")
	linkDirOrSkip(t, m.abs("asli"), m.abs("tautan"))

	r := m.deleteDir(deleteDirInput{Path: "tautan", Recursive: true, DryRun: true})
	if hasError(r) || r["link"] != true || r["deleted"] != false || r["dry_run"] != true {
		t.Errorf("tautan: harapan link=true deleted=false dry_run=true: %v", r)
	}
	if _, ok := r["files"]; ok {
		t.Errorf("isi target tautan tidak boleh dihitung: %v", r)
	}
	if _, err := os.Lstat(m.abs("tautan")); err != nil {
		t.Error("tautan hilang setelah dry_run")
	}
	if got := m.content(t, "asli/data.txt"); got != "penting" {
		t.Errorf("isi target berubah: %q", got)
	}
}

func TestDeleteDirDryRunKeepsGuards(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "f.txt", "x")
	for _, in := range []deleteDirInput{
		{Path: ".", Recursive: true, DryRun: true},
		{Path: "tidak-ada", DryRun: true},
		{Path: "f.txt", DryRun: true},
		{Path: "../luar", Recursive: true, DryRun: true},
	} {
		if r := m.deleteDir(in); !hasError(r) {
			t.Errorf("%+v seharusnya error: %v", in, r)
		}
	}
}

func TestDeleteDirDryRunCapsCount(t *testing.T) {
	m := newTestManager(t)
	old := maxDryRunEntries
	maxDryRunEntries = 5
	t.Cleanup(func() { maxDryRunEntries = old })
	for i := 0; i < 9; i++ {
		m.raw(t, fmt.Sprintf("besar/f%d.txt", i), "x")
	}
	r := m.deleteDir(deleteDirInput{Path: "besar", Recursive: true, DryRun: true})
	if hasError(r) || r["truncated"] != true || r["deleted"] != false {
		t.Errorf("harapan truncated=true tanpa error: %v", r)
	}
	if n := len(entryCountCheck(t, m, "besar")); n != 9 {
		t.Errorf("isi folder berubah: %d file", n)
	}
}

func entryCountCheck(t *testing.T, m *manager, rel string) []os.DirEntry {
	t.Helper()
	es, err := os.ReadDir(m.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return es
}
