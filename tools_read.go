package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"
)

// result adalah hasil setiap tool. Kegagalan dikembalikan sebagai
// {"error": "..."} (bukan error protokol), sama dengan versi Python.
type result = map[string]any

func fail(msg string) result { return result{"error": msg} }

// failErr mengubah error menjadi hasil kegagalan. Hanya error tak terduga
// yang dicatat di log; kesalahan validasi path sudah jelas bagi pemanggil.
func failErr(err error) result {
	var pe *pathError
	if !errors.As(err, &pe) {
		slogError(err)
	}
	return fail(err.Error())
}

// defaultExcludeDirs adalah folder besar/tidak relevan yang dilewati saat
// recursive=true, agar hasil list_files tidak meledak ukurannya.
var defaultExcludeDirs = []string{
	".git", ".venv", "venv", "node_modules", "__pycache__",
	".egg-info", "dist", "build", ".mypy_cache", ".pytest_cache",
}

func excluded(name string) bool {
	if strings.HasSuffix(name, ".egg-info") {
		return true
	}
	for _, d := range defaultExcludeDirs {
		if name == d {
			return true
		}
	}
	return false
}

// foldName menyamakan huruf besar/kecil di Windows, tempat nama file tidak
// membedakannya.
func foldName(s string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(s)
	}
	return s
}

func nameLess(a, b string) bool { return foldName(a) < foldName(b) }

type scanResult struct {
	lines      []string
	total      int
	cutByChars bool
	lineEnding *string
}

// scanLines membaca r per baris (bergaya universal newline: \n, \r\n, dan \r
// semuanya dianggap akhir baris) tanpa memuat seluruh isi ke memori. Hanya
// baris pada rentang [offset, offset+limit) yang disimpan, dibatasi maxChars
// karakter. Seluruh isi tetap dibaca agar total baris akurat.
func scanLines(r io.Reader, offset, limit, maxChars int) (scanResult, error) {
	res := scanResult{lines: []string{}}
	var (
		cur                      []byte
		chars, lineLen           int
		seenLF, seenCRLF, seenCR bool
		pendingCR                bool
	)
	collecting := func() bool {
		return res.total >= offset && len(res.lines) < limit && chars < maxChars &&
			len(cur) < (maxChars-chars)*utf8.UTFMax+utf8.UTFMax
	}
	finish := func(hasEOL bool) {
		defer func() { cur = cur[:0]; lineLen = 0 }()
		res.total++
		if res.total <= offset || len(res.lines) >= limit {
			return
		}
		remaining := maxChars - chars
		if remaining <= 0 {
			res.cutByChars = true
			return
		}
		text := decodeReplace(cur)
		if hasEOL {
			text += "\n"
		}
		if utf8.RuneCountInString(text) > remaining {
			text = firstRunes(text, remaining)
			res.cutByChars = true
		}
		res.lines = append(res.lines, text)
		chars += utf8.RuneCountInString(text)
	}

	buf := make([]byte, 64*1024)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if pendingCR {
				pendingCR = false
				if b == '\n' {
					seenCRLF = true
					finish(true)
					continue
				}
				seenCR = true
				finish(true)
			}
			switch b {
			case '\n':
				seenLF = true
				finish(true)
			case '\r':
				pendingCR = true
			default:
				lineLen++
				if collecting() {
					cur = append(cur, b)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, err
		}
	}
	if pendingCR {
		seenCR = true
		finish(true)
	} else if lineLen > 0 {
		finish(false)
	}

	kinds := 0
	name := ""
	for _, k := range []struct {
		seen bool
		name string
	}{{seenLF, "LF"}, {seenCRLF, "CRLF"}, {seenCR, "CR"}} {
		if k.seen {
			kinds++
			name = k.name
		}
	}
	switch {
	case kinds > 1:
		mixed := "mixed"
		res.lineEnding = &mixed
	case kinds == 1:
		res.lineEnding = &name
	}
	return res, nil
}

// decodeReplace mengubah byte tidak valid menjadi U+FFFD, seperti
// errors="replace" pada Python.
func decodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return string([]rune(string(b)))
}

func firstRunes(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

type readInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Length *int   `json:"length,omitempty"`
}

// readPath memvalidasi path lalu membaca baris [offset, offset+length).
// offset negatif membaca |offset| baris terakhir (length diabaikan).
func (m *manager) readPath(p string, offset, length, maxChars int) result {
	if offset >= 0 && length <= 0 {
		return fail("length harus lebih besar dari 0")
	}
	full, err := m.cfg.safe(p)
	if err != nil {
		return failErr(err)
	}
	fi, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return fail("File tidak ditemukan: " + p)
	}
	if err != nil {
		return failErr(err)
	}
	if fi.IsDir() {
		return fail("Path adalah direktori, bukan file: " + p)
	}
	f, err := os.Open(full)
	if err != nil {
		return failErr(err)
	}
	defer f.Close()

	if offset < 0 {
		// Hitung total baris dulu (limit 0: tidak ada baris yang disimpan),
		// lalu baca ulang dari awal mulai dari baris ke-(total+offset).
		counted, err := scanLines(f, 0, 0, maxChars)
		if err != nil {
			return failErr(err)
		}
		length = -offset
		offset = max(0, counted.total+offset)
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return failErr(err)
		}
	}

	sr, err := scanLines(f, offset, length, maxChars)
	if err != nil {
		return failErr(err)
	}
	end := offset + len(sr.lines)
	res := result{
		"path":        p,
		"lines":       sr.lines,
		"offset":      offset,
		"total_lines": sr.total,
		"truncated":   end < sr.total || sr.cutByChars,
		"line_ending": sr.lineEnding,
	}
	if end < sr.total {
		res["next_offset"] = end
	}
	return res
}

func (m *manager) readFile(in readInput) result {
	length := 2000
	if in.Length != nil {
		length = *in.Length
	}
	return m.readPath(in.Path, in.Offset, length, m.cfg.maxReadChars)
}

type multiInput struct {
	Paths []string `json:"paths"`
}

func (m *manager) readMultipleFiles(in multiInput) result {
	if len(in.Paths) == 0 {
		return fail("paths tidak boleh kosong")
	}
	if len(in.Paths) > maxReadFiles {
		return fail(fmt.Sprintf("Maksimal %d file per panggilan", maxReadFiles))
	}
	budget := m.cfg.maxReadChars
	files := make([]result, 0, len(in.Paths))
	for _, p := range in.Paths {
		if budget <= 0 {
			files = append(files, result{"path": p, "error": "batas karakter tercapai; baca file ini terpisah"})
			continue
		}
		r := m.readPath(p, 0, 2000, budget)
		if msg, bad := r["error"]; bad {
			r = result{"path": p, "error": msg}
		} else {
			for _, line := range r["lines"].([]string) {
				budget -= utf8.RuneCountInString(line)
			}
		}
		files = append(files, r)
	}
	return result{"files": files}
}
