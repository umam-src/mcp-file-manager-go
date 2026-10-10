package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// visitFunc dipanggil untuk tiap entri. rel memakai '/' di semua platform.
// Kembalikan false untuk menghentikan penelusuran.
type visitFunc func(rel string, isDir bool, de fs.DirEntry, full string) bool

// walkTree menelusuri dir secara depth-first dengan urutan nama. Folder berat
// dan symlink ke folder dicantumkan tetapi tidak dimasuki; folder yang tidak
// terbaca dilewati. Mengembalikan false bila visit meminta berhenti.
func walkTree(dir, rel string, depth, maxDepth int, visit visitFunc) bool {
	des, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	sort.Slice(des, func(i, j int) bool { return nameLess(des[i].Name(), des[j].Name()) })
	for _, de := range des {
		name := de.Name()
		relName := name
		if rel != "" {
			relName = rel + "/" + name
		}
		child := filepath.Join(dir, name)
		isDir := false
		if fi, err := os.Stat(child); err == nil {
			isDir = fi.IsDir()
		}
		if !visit(relName, isDir, de, child) {
			return false
		}
		if de.IsDir() && depth < maxDepth && !excluded(name) {
			if !walkTree(child, relName, depth+1, maxDepth, visit) {
				return false
			}
		}
	}
	return true
}

// statDir memeriksa path hasil safe() yang harus berupa folder yang ada.
func statDir(full, p string) *result {
	fi, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		r := fail("Path tidak ditemukan: " + p)
		return &r
	}
	if err != nil {
		r := failErr(err)
		return &r
	}
	if !fi.IsDir() {
		r := fail("Path bukan direktori: " + p)
		return &r
	}
	return nil
}

type listInput struct {
	Path  *string `json:"path,omitempty"`
	Depth *int    `json:"depth,omitempty"`
}

func (m *manager) listDirectory(in listInput) result {
	p := "."
	if in.Path != nil {
		p = *in.Path
	}
	depth := 2
	if in.Depth != nil {
		depth = *in.Depth
	}
	if depth < 1 {
		return fail("depth harus lebih besar dari 0")
	}
	full, err := m.cfg.safe(p)
	if err != nil {
		return failErr(err)
	}
	if bad := statDir(full, p); bad != nil {
		return *bad
	}

	entries := make([]string, 0)
	truncated := false
	walkTree(full, "", 1, depth, func(rel string, isDir bool, _ fs.DirEntry, _ string) bool {
		if len(entries) >= m.cfg.maxEntries {
			truncated = true
			return false
		}
		prefix := "[FILE] "
		if isDir {
			prefix = "[DIR] "
		}
		entries = append(entries, prefix+rel)
		return true
	})

	res := result{"path": p, "entries": entries}
	if truncated {
		res["truncated"] = true
	}
	return res
}

// wild mencocokkan text dengan pattern berwildcard: '*' = urutan apa saja,
// '?' = satu karakter.
func wild(pattern, text string) bool {
	pr, tr := []rune(pattern), []rune(text)
	p, t, star, mark := 0, 0, -1, 0
	for t < len(tr) {
		switch {
		case p < len(pr) && (pr[p] == '?' || pr[p] == tr[t]):
			p++
			t++
		case p < len(pr) && pr[p] == '*':
			star, mark = p, t
			p++
		case star != -1:
			p = star + 1
			mark++
			t = mark
		default:
			return false
		}
	}
	for p < len(pr) && pr[p] == '*' {
		p++
	}
	return p == len(pr)
}

// nameMatches: nama cocok bila memuat pattern, atau cocok penuh bila pattern
// memuat '*' atau '?'.
func nameMatches(pattern, name string, ignoreCase bool) bool {
	if ignoreCase {
		pattern, name = strings.ToLower(pattern), strings.ToLower(name)
	}
	if strings.ContainsAny(pattern, "*?") {
		return wild(pattern, name)
	}
	return strings.Contains(name, pattern)
}

type searchInput struct {
	Pattern     string  `json:"pattern"`
	Path        *string `json:"path,omitempty"`
	SearchType  *string `json:"searchType,omitempty"`
	FilePattern *string `json:"filePattern,omitempty"`
	IgnoreCase  *bool   `json:"ignoreCase,omitempty"`
	MaxResults  *int    `json:"maxResults,omitempty"`
}

func (m *manager) startSearch(in searchInput) result {
	p, searchType, filePattern := ".", "files", ""
	ignoreCase, maxResults := true, 100
	if in.Path != nil {
		p = *in.Path
	}
	if in.SearchType != nil {
		searchType = *in.SearchType
	}
	if in.FilePattern != nil {
		filePattern = *in.FilePattern
	}
	if in.IgnoreCase != nil {
		ignoreCase = *in.IgnoreCase
	}
	if in.MaxResults != nil {
		maxResults = *in.MaxResults
	}
	if searchType != "files" && searchType != "content" {
		return fail("searchType harus 'files' atau 'content'")
	}
	if in.Pattern == "" {
		return fail("pattern tidak boleh kosong")
	}
	if maxResults < 1 {
		return fail("maxResults harus lebih besar dari 0")
	}
	limit := min(maxResults, maxSearchResults)
	full, err := m.cfg.safe(p)
	if err != nil {
		return failErr(err)
	}
	if bad := statDir(full, p); bad != nil {
		return *bad
	}

	needle := in.Pattern
	if ignoreCase {
		needle = strings.ToLower(needle)
	}
	matches := make([]string, 0)
	truncated := false
	add := func(item string) bool {
		if len(matches) >= limit {
			truncated = true
			return false
		}
		matches = append(matches, item)
		return true
	}

	walkTree(full, "", 1, 1_000_000, func(rel string, isDir bool, de fs.DirEntry, child string) bool {
		if searchType == "files" {
			if !nameMatches(in.Pattern, de.Name(), ignoreCase) {
				return true
			}
			if isDir {
				rel += "/"
			}
			return add(rel)
		}
		if isDir {
			return true
		}
		fi, err := os.Stat(child)
		if err != nil || !fi.Mode().IsRegular() {
			return true
		}
		if filePattern != "" && !nameMatches(filePattern, de.Name(), ignoreCase) {
			return true
		}
		if fi.Size() > maxSearchFileBytes {
			return true
		}
		data, err := os.ReadFile(child)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
			return true
		}
		for i, raw := range bytes.Split(data, []byte("\n")) {
			text := strings.TrimRight(decodeReplace(raw), "\r")
			hay := text
			if ignoreCase {
				hay = strings.ToLower(text)
			}
			if strings.Contains(hay, needle) {
				if !add(fmt.Sprintf("%s:%d: %s", rel, i+1, firstRunes(text, maxMatchChars))) {
					return false
				}
			}
		}
		return true
	})

	res := result{"path": p, "matches": matches}
	if truncated {
		res["truncated"] = true
	}
	return res
}

// countLines menghitung baris file teks; false bila file tampak biner.
func countLines(full string) (int, bool) {
	f, err := os.Open(full)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	head := make([]byte, 8192)
	n, _ := io.ReadFull(f, head)
	if bytes.IndexByte(head[:n], 0) >= 0 {
		return 0, false
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, false
	}
	sr, err := scanLines(f, 0, 0, 0)
	if err != nil {
		return 0, false
	}
	return sr.total, true
}

func (m *manager) getFileInfo(in pathInput) result {
	full, err := m.cfg.safe(in.Path)
	if err != nil {
		return failErr(err)
	}
	fi, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return fail("Path tidak ditemukan: " + in.Path)
	}
	if err != nil {
		return failErr(err)
	}
	info := result{
		"path":     in.Path,
		"type":     "file",
		"modified": fi.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if fi.IsDir() {
		info["type"] = "dir"
	} else if fi.Mode().IsRegular() {
		info["size"] = fi.Size()
		if fi.Size() <= maxInfoLinesBytes {
			if n, ok := countLines(full); ok {
				info["lines"] = n
			}
		}
	}
	return info
}
