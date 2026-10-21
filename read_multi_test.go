package main

import (
	"strings"
	"testing"
)

// G-12 (a): offset dan length berlaku untuk semua berkas.
func TestReadMultipleFilesOffsetLength(t *testing.T) {
	m := newTestManager(t)
	m.raw(t, "a.txt", "a1\na2\na3\na4\n")
	m.raw(t, "b.txt", "b1\nb2\nb3\nb4\n")

	join := func(r result) string {
		return strings.Join(r["lines"].([]string), "")
	}
	files := func(in multiInput) []result {
		t.Helper()
		out := m.readMultipleFiles(in)
		if msg, bad := out["error"]; bad {
			t.Fatalf("error: %v", msg)
		}
		return out["files"].([]result)
	}

	got := files(multiInput{Paths: []string{"a.txt", "b.txt"}, Offset: 1, Length: ptr(2)})
	if join(got[0]) != "a2\na3\n" || join(got[1]) != "b2\nb3\n" {
		t.Errorf("offset 1, length 2: %q / %q", join(got[0]), join(got[1]))
	}
	if got[0]["next_offset"] != 3 {
		t.Errorf("next_offset: %v", got[0]["next_offset"])
	}

	got = files(multiInput{Paths: []string{"a.txt", "b.txt"}, Offset: -1})
	if join(got[0]) != "a4\n" || join(got[1]) != "b4\n" {
		t.Errorf("ekor: %q / %q", join(got[0]), join(got[1]))
	}

	got = files(multiInput{Paths: []string{"a.txt"}})
	if join(got[0]) != "a1\na2\na3\na4\n" {
		t.Errorf("tanpa offset/length harus utuh: %q", join(got[0]))
	}

	out := m.readMultipleFiles(multiInput{Paths: []string{"a.txt"}, Length: ptr(0)})
	if r := out["files"].([]result); r[0]["error"] == nil {
		t.Errorf("length 0 harus galat: %v", r[0])
	}
}
