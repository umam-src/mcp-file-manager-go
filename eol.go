package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// detectEOL menentukan line ending file yang sudah ada: CRLF bila mayoritas
// CRLF, selain itu LF. File yang belum ada (atau tanpa baris baru) dianggap LF.
func detectEOL(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "\n"
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return "\n"
	}
	head, _ := io.ReadAll(io.LimitReader(f, 65536))
	crlf := bytes.Count(head, []byte("\r\n"))
	lf := bytes.Count(head, []byte("\n")) - crlf
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

// convertEOL mengubah "\n" pada text menjadi CRLF bila file tujuan memakai
// CRLF. Text yang sudah memuat "\r" dibiarkan (pemanggil mengatur sendiri).
func convertEOL(text, eol string) string {
	if eol == "\r\n" && !strings.Contains(text, "\r") {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// sizeError mengembalikan pesan kesalahan bila ukuran hasil tulis melebihi
// batas, atau string kosong bila masih aman.
func (c *config) sizeError(data string, existing int64) string {
	size := existing + int64(len(data))
	if size > c.maxWriteBytes {
		return fmt.Sprintf("Ukuran tulis %d byte melebihi batas %d byte (atur %s bila memang perlu)",
			size, c.maxWriteBytes, envMaxWriteBytes)
	}
	return ""
}
