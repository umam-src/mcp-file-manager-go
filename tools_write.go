package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// statKind memeriksa path hasil safe(). exists=false bila belum ada.
func statKind(full string) (exists, isDir bool, size int64, err error) {
	fi, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, 0, nil
	}
	if err != nil {
		return false, false, 0, err
	}
	return true, fi.IsDir(), fi.Size(), nil
}

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
}

// writeFile menulis ulang file (mode "rewrite", bawaan) atau menambah di
// akhirnya (mode "append").
func (m *manager) writeFile(in writeInput) result {
	mode := in.Mode
	if mode == "" {
		mode = "rewrite"
	}
	if mode != "rewrite" && mode != "append" {
		return fail("mode harus 'rewrite' atau 'append'")
	}
	full, err := m.cfg.safe(in.Path)
	if err != nil {
		return failErr(err)
	}
	exists, isDir, size, err := statKind(full)
	if err != nil {
		return failErr(err)
	}
	if isDir {
		return fail("Path adalah direktori, bukan file: " + in.Path)
	}
	data := convertEOL(in.Content, detectEOL(full))
	var existing int64
	if mode == "append" && exists {
		existing = size
	}
	if msg := m.cfg.sizeError(data, existing); msg != "" {
		return fail(msg)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return failErr(err)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if mode == "append" {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(full, flags, 0o644)
	if err != nil {
		return failErr(err)
	}
	if _, err := f.WriteString(data); err != nil {
		f.Close()
		return failErr(err)
	}
	if err := f.Close(); err != nil {
		return failErr(err)
	}
	return result{"path": in.Path, "written": utf8.RuneCountInString(in.Content)}
}

type editInput struct {
	FilePath             string `json:"file_path"`
	OldString            string `json:"old_string"`
	NewString            string `json:"new_string"`
	ExpectedReplacements *int   `json:"expected_replacements,omitempty"`
}

// editBlock mengganti old_string dengan new_string; gagal bila jumlah
// kemunculannya berbeda dari expected_replacements (bawaan 1).
func (m *manager) editBlock(in editInput) result {
	if in.OldString == "" {
		return fail("old_string tidak boleh kosong")
	}
	expected := 1
	if in.ExpectedReplacements != nil {
		expected = *in.ExpectedReplacements
	}
	if expected < 1 {
		return fail("expected_replacements harus lebih besar dari 0")
	}
	full, err := m.cfg.safe(in.FilePath)
	if err != nil {
		return failErr(err)
	}
	exists, isDir, _, err := statKind(full)
	if err != nil {
		return failErr(err)
	}
	if !exists {
		return fail("File tidak ditemukan: " + in.FilePath)
	}
	if isDir {
		return fail("Path adalah direktori, bukan file: " + in.FilePath)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return failErr(err)
	}
	if !utf8.Valid(raw) {
		return fail("File bukan teks UTF-8 yang valid, tidak bisa diedit: " + in.FilePath)
	}
	text := string(raw)

	eol := "\n"
	if strings.Count(text, "\r\n")*2 > strings.Count(text, "\n") {
		eol = "\r\n"
	}
	old := convertEOL(in.OldString, eol)
	repl := convertEOL(in.NewString, eol)
	found := strings.Count(text, old)
	if found == 0 {
		return fail("old_string tidak ditemukan")
	}
	if found != expected {
		return fail(fmt.Sprintf("old_string ditemukan %d kali, bukan %d; sesuaikan expected_replacements atau perluas old_string", found, expected))
	}
	text = strings.ReplaceAll(text, old, repl)
	if msg := m.cfg.sizeError(text, 0); msg != "" {
		return fail(msg)
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		return failErr(err)
	}
	return result{"path": in.FilePath, "replacements": found}
}

type pathInput struct {
	Path string `json:"path"`
}

func (m *manager) deleteFile(in pathInput) result {
	full, isLink, err := m.cfg.safeLeaf(in.Path)
	if err != nil {
		return failErr(err)
	}
	exists, isDir, _, err := statKind(full)
	if err != nil {
		return failErr(err)
	}
	if isLink {
		exists = true // tautan rusak tetap bisa dihapus
	}
	if !exists {
		return fail("File tidak ditemukan: " + in.Path)
	}
	if isDir {
		return fail("path adalah direktori, gunakan tool delete_dir")
	}
	if err := os.Remove(full); err != nil {
		return failErr(err)
	}
	return result{"path": in.Path, "deleted": true}
}

type deleteDirInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

func (m *manager) deleteDir(in deleteDirInput) result {
	full, isLink, err := m.cfg.safeLeaf(in.Path)
	if err != nil {
		return failErr(err)
	}
	if full == m.cfg.baseDir {
		return fail("tidak boleh menghapus BASE_DIR itu sendiri")
	}
	exists, isDir, _, err := statKind(full)
	if err != nil {
		return failErr(err)
	}
	if !exists {
		return fail("Direktori tidak ditemukan: " + in.Path)
	}
	if !isDir {
		return fail("path bukan direktori, gunakan tool delete_file")
	}
	if isLink {
		// Hanya tautannya yang dihapus; isi folder targetnya tidak disentuh.
		if err := os.Remove(full); err != nil {
			return failErr(err)
		}
		return result{"path": in.Path, "deleted": true, "link": true}
	}
	if in.Recursive {
		err = os.RemoveAll(full)
	} else {
		err = os.Remove(full) // gagal otomatis bila folder tidak kosong
	}
	if err != nil {
		return fail(fmt.Sprintf("Direktori tidak kosong atau gagal dihapus: %v", err))
	}
	return result{"path": in.Path, "deleted": true, "recursive": in.Recursive}
}

// backupPrefix adalah awalan nama cadangan sementara tujuan yang akan ditimpa
// oleh move_file; cadangan dihapus (atau dikembalikan) sebelum tool selesai.
const backupPrefix = ".mfm-lama-"

// caseOnlyTarget mengenali rename yang hanya mengubah huruf besar/kecil nama
// pada file yang sama di folder yang sama. Hasilnya path tujuan dengan huruf
// seperti yang diketik pemanggil. Di sistem yang membedakan huruf (Linux),
// A.txt bukan file yang sama dengan a.txt sehingga fungsi ini tidak aktif dan
// rename berjalan seperti biasa.
func caseOnlyTarget(src, dst, typedDst string) (string, bool) {
	leaf := filepath.Base(strings.TrimRight(typedDst, `/\`))
	srcLeaf := filepath.Base(src)
	if leaf == "." || leaf == "" || leaf == srcLeaf || !strings.EqualFold(leaf, srcLeaf) {
		return "", false
	}
	if foldName(filepath.Dir(src)) != foldName(filepath.Dir(dst)) {
		return "", false
	}
	cand := filepath.Join(filepath.Dir(src), leaf)
	si, err1 := os.Lstat(src)
	ci, err2 := os.Lstat(cand)
	if err1 != nil || err2 != nil || !os.SameFile(si, ci) {
		return "", false
	}
	return cand, true
}

type moveInput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Overwrite   bool   `json:"overwrite,omitempty"`
}

func (m *manager) moveFile(in moveInput) result {
	src, srcLink, err := m.cfg.safeLeaf(in.Source)
	if err != nil {
		return failErr(err)
	}
	dst, dstLink, err := m.cfg.safeLeaf(in.Destination)
	if err != nil {
		return failErr(err)
	}
	srcExists, _, _, err := statKind(src)
	if err != nil {
		return failErr(err)
	}
	if srcLink {
		srcExists = true // tautan rusak tetap bisa dipindah
	}
	if !srcExists {
		return fail("Sumber tidak ditemukan: " + in.Source)
	}
	// Ganti huruf besar/kecil nama pada file yang sama (Windows: a.txt ke
	// A.txt). safe() menormalkan huruf ke nama yang ada di disk, sehingga
	// sumber dan tujuan tampak sama; rename dilakukan ke nama yang diketik.
	if cand, ok := caseOnlyTarget(src, dst, in.Destination); ok {
		if err := os.Rename(src, cand); err != nil {
			return failErr(err)
		}
		return result{"destination": in.Destination, "moved": true}
	}
	// Penjaga yang tidak ada di versi Python: tanpa ini, overwrite=true bisa
	// menghapus sumber sendiri bila tujuan sama dengan atau induk dari sumber.
	switch {
	case within(src, dst):
		if filepath.Clean(src) == filepath.Clean(dst) {
			return fail("Sumber dan tujuan sama: " + in.Source)
		}
		return fail("Tujuan berada di dalam sumber: " + in.Destination)
	case within(dst, src):
		return fail("Tujuan adalah folder induk dari sumber, memindahkannya akan menghapus sumber: " + in.Destination)
	}
	dstExists, dstIsDir, _, err := statKind(dst)
	if err != nil {
		return failErr(err)
	}
	if dstLink {
		dstExists = true
	}
	// Tujuan yang akan ditimpa dipindah dulu ke cadangan sementara dan baru
	// dihapus setelah rename berhasil; bila rename gagal, cadangan dikembalikan
	// sehingga tujuan tidak hilang.
	backup := ""
	if dstExists {
		if !in.Overwrite {
			return fail(fmt.Sprintf("Tujuan sudah ada: %s (gunakan overwrite=True untuk menimpa)", in.Destination))
		}
		backup = filepath.Join(filepath.Dir(dst), backupPrefix+strconv.FormatInt(time.Now().UnixNano(), 36))
		if err := os.Rename(dst, backup); err != nil {
			return failErr(err)
		}
	}
	restore := func() {
		if backup == "" {
			return
		}
		if err := os.Rename(backup, dst); err != nil {
			slogError(fmt.Errorf("gagal mengembalikan tujuan dari cadangan %s: %w", backup, err))
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		restore()
		return failErr(err)
	}
	if err := os.Rename(src, dst); err != nil {
		restore()
		return failErr(err)
	}
	if backup != "" {
		if dstIsDir && !dstLink { // tautan cukup dilepas, isi targetnya jangan dihapus
			err = os.RemoveAll(backup)
		} else {
			err = os.Remove(backup)
		}
		if err != nil {
			slogError(fmt.Errorf("move berhasil, tetapi cadangan %s gagal dihapus: %w", backup, err))
		}
	}
	return result{"destination": in.Destination, "moved": true}
}

func (m *manager) createDirectory(in pathInput) result {
	full, err := m.cfg.safe(in.Path)
	if err != nil {
		return failErr(err)
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return failErr(err)
	}
	return result{"path": in.Path, "created": true}
}
