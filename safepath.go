package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// pathError adalah kesalahan validasi path. Pesannya aman ditampilkan ke
// pemanggil apa adanya.
type pathError struct{ msg string }

func (e *pathError) Error() string { return e.msg }

// maxSymlinkHops membatasi jumlah symlink yang diikuti, agar rantai symlink
// yang melingkar tidak membuat server berputar tanpa akhir.
const maxSymlinkHops = 40

// within melaporkan apakah p sama dengan base atau berada di dalamnya.
func within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// safe menerjemahkan path dari pemanggil menjadi path absolut yang pasti
// berada di dalam salah satu folder dasar. Path relatif dihitung dari baseDir
// (folder pertama); path absolut diterima selama hasil akhirnya masih di dalam
// salah satu folder dasar.
//
// Yang ditolak: path kosong, null byte, traversal (..), dan symlink (termasuk
// symlink yang targetnya belum ada) yang mengarah keluar semua folder dasar.
// Semua akses file dari input pemanggil WAJIB lewat fungsi ini.
func (c *config) safe(p string) (string, error) {
	if p == "" {
		return "", &pathError{"path tidak boleh kosong"}
	}
	if strings.ContainsRune(p, 0) {
		return "", &pathError{"path mengandung null byte"}
	}
	candidate := p
	if !filepath.IsAbs(p) {
		candidate = filepath.Join(c.baseDir, p)
	}
	linked, err := resolveLinks(candidate)
	if err != nil {
		return "", err
	}
	resolved, err := resolvePath(linked)
	if err != nil {
		return "", err
	}
	if !c.inAnyBase(resolved) {
		where := c.baseDir
		if len(c.otherDirs) > 0 {
			where = strings.Join(c.allBases(), "; ")
		}
		return "", &pathError{fmt.Sprintf("Akses ditolak: '%s' berada di luar %s", p, where)}
	}
	return resolved, nil
}

// resolvePath mengikuti semua symlink pada path, juga untuk bagian path yang
// belum ada (mis. file baru yang akan dibuat). Bagian yang belum ada
// ditempelkan apa adanya setelah bagian yang sudah ada diselesaikan.
//
// filepath.EvalSymlinks gagal untuk symlink yang targetnya belum ada, padahal
// menulis lewat symlink seperti itu akan membuat file di targetnya. Karena itu
// symlink semacam ini diikuti secara manual.
func resolvePath(p string) (string, error) {
	p = filepath.Clean(p)
	for hops := 0; hops <= maxSymlinkHops; hops++ {
		// Cari awalan terpanjang yang sudah ada.
		cur := p
		var missing []string // komponen yang belum ada, dari belakang ke depan
		var real string
		for {
			r, err := filepath.EvalSymlinks(cur)
			if err == nil {
				real = r
				break
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return "", err
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				return "", err
			}
			missing = append(missing, filepath.Base(cur))
			cur = parent
		}
		if len(missing) == 0 {
			return real, nil
		}

		first := filepath.Join(real, missing[len(missing)-1])
		fi, err := os.Lstat(first)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Benar-benar belum ada: tempelkan sisanya.
				return joinReversed(real, missing), nil
			}
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			return "", &pathError{fmt.Sprintf("path tidak bisa diselesaikan: %s", first)}
		}
		// Symlink yang targetnya belum ada: ikuti manual, lalu ulangi.
		target, err := os.Readlink(first)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(real, target)
		}
		p = filepath.Clean(joinReversed(target, missing[:len(missing)-1]))
	}
	return "", &pathError{"terlalu banyak symlink bertingkat"}
}

// joinReversed menyambung base dengan parts yang urutannya terbalik.
func joinReversed(base string, parts []string) string {
	for i := len(parts) - 1; i >= 0; i-- {
		base = filepath.Join(base, parts[i])
	}
	return base
}

// splitParts memecah path menjadi komponen tanpa komponen kosong.
func splitParts(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == os.PathSeparator
	})
}

// resolveLinks menelusuri path komponen demi komponen dan mengganti setiap
// symlink serta junction (di Windows, Go menganggap junction sebagai file
// "irregular" sehingga filepath.EvalSymlinks tidak mengikutinya) dengan
// targetnya. Komponen yang belum ada ditempelkan apa adanya. Hasilnya dipakai
// resolvePath untuk normalisasi akhir.
func resolveLinks(p string) (string, error) {
	sep := string(filepath.Separator)
	p = filepath.Clean(p)
	vol := filepath.VolumeName(p)
	cur := vol + sep
	queue := splitParts(p[len(vol):])
	hops := 0
	missing := false
	for len(queue) > 0 {
		comp := queue[0]
		queue = queue[1:]
		if comp == "." {
			continue
		}
		if comp == ".." {
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, comp)
		if missing {
			cur = next
			continue
		}
		fi, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				missing = true
				cur = next
				continue
			}
			return "", err
		}
		if fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
			cur = next
			continue
		}
		target, err := os.Readlink(next)
		if err != nil {
			if fi.Mode()&fs.ModeSymlink == 0 {
				cur = next // file khusus biasa, bukan tautan
				continue
			}
			return "", err
		}
		hops++
		if hops > maxSymlinkHops {
			return "", &pathError{"terlalu banyak symlink bertingkat"}
		}
		switch {
		case filepath.IsAbs(target):
			tv := filepath.VolumeName(target)
			cur = tv + sep
			target = target[len(tv):]
		case strings.HasPrefix(target, sep):
			cur = filepath.VolumeName(cur) + sep
		}
		queue = append(splitParts(target), queue...)
	}
	return cur, nil
}

// safeLeaf seperti safe, tetapi komponen terakhir path tidak diikuti bila
// berupa tautan (symlink atau junction): hasilnya path tautan itu sendiri dan
// isLink bernilai true. Dipakai delete_file, delete_dir, dan move_file agar
// yang dihapus atau dipindah adalah tautannya, bukan isi targetnya. Semua
// penolakan safe tetap berlaku, termasuk tautan yang mengarah keluar baseDir.
func (c *config) safeLeaf(p string) (full string, isLink bool, err error) {
	resolved, err := c.safe(p)
	if err != nil {
		return "", false, err
	}
	if c.isBase(resolved) {
		return resolved, false, nil
	}
	cand := p
	if !filepath.IsAbs(p) {
		cand = filepath.Join(c.baseDir, p)
	}
	trimmed := strings.TrimRight(cand, `/\`)
	leaf := filepath.Base(trimmed)
	if trimmed == "" || leaf == "." || leaf == ".." || leaf == string(filepath.Separator) {
		return resolved, false, nil // "tautan/." berarti foldernya sendiri
	}
	dir, err := c.safe(filepath.Dir(trimmed))
	if err != nil {
		return "", false, err
	}
	link := filepath.Join(dir, leaf)
	fi, lerr := os.Lstat(link)
	if lerr != nil || fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return resolved, false, nil
	}
	if _, rerr := os.Readlink(link); rerr != nil {
		return resolved, false, nil // file khusus biasa, bukan tautan
	}
	return link, true, nil
}
