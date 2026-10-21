package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Hasil tool tidak boleh meng-escape <, >, dan & (G-12): \u003c dst. hanya
// menambah token.
func TestEncodeResultTanpaEscapeHTML(t *testing.T) {
	in := result{"lines": []string{"if a < b && c > d {", "<div>&amp;</div>"}}
	got := encodeResult(in)
	if strings.Contains(got, `\u00`) {
		t.Errorf("masih ada escape HTML: %s", got)
	}
	want := `{"lines":["if a < b && c > d {","<div>&amp;</div>"]}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("hasil tidak boleh berakhir newline: %q", got)
	}
	var back result
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("hasil bukan JSON sah: %v", err)
	}
}

func TestEncodeResultGagalDiubah(t *testing.T) {
	got := encodeResult(result{"x": make(chan int)})
	if !strings.Contains(got, `"error"`) {
		t.Errorf("kegagalan harus berbentuk {\"error\": ...}: %s", got)
	}
}
