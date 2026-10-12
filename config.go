package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	envBase          = "MCP_FILE_BASE"
	envConfig        = "MCP_FILE_CONFIG"
	envMaxWriteBytes = "MCP_FILE_MAX_WRITE_BYTES"
	envLogLevel      = "MCP_FILE_LOG_LEVEL"

	defaultMaxWriteBytes = 5 * 1024 * 1024
	// Batas karakter isi yang dikembalikan read_file per panggilan.
	defaultMaxReadChars = 100_000
	// Batas jumlah entri list_directory.
	defaultMaxEntries = 1000

	// Batas tetap yang sama dengan versi Python.
	maxReadFiles       = 20               // file per read_multiple_files
	maxSearchResults   = 1000             // hasil start_search
	maxSearchFileBytes = 5 * 1024 * 1024  // file lebih besar dilewati pencarian isi
	maxInfoLinesBytes  = 10 * 1024 * 1024 // get_file_info tidak menghitung baris di atas ini
	maxMatchChars      = 200              // panjang teks baris pada hasil pencarian isi

	// maxConfigBytes membatasi ukuran berkas config JSON.
	maxConfigBytes = 64 * 1024
)

// config memuat pengaturan server. Bila berasal dari berkas config (JSON),
// nilainya dimuat ulang oleh refresh saat berkas berubah; selama sebuah
// panggilan tool berjalan nilainya tidak berubah (lihat guard).
type config struct {
	baseDir       string   // folder pertama (dasar path relatif), absolut dan bebas symlink
	otherDirs     []string // folder dasar tambahan, kosong bila hanya satu folder
	roDirs        []string // folder dasar (subset dari semua folder) yang hanya boleh dibaca
	maxWriteBytes int64
	maxReadChars  int
	maxEntries    int

	mu     sync.RWMutex
	path   string              // berkas config; kosong bila hanya memakai env
	sig    fileSig             // tanda berkas config saat terakhir diperiksa
	getenv func(string) string // untuk nilai env yang ikut menentukan batas
}

// fileSig adalah tanda sederhana perubahan berkas config.
type fileSig struct {
	exists bool
	mod    time.Time
	size   int64
}

func statSig(path string) fileSig {
	fi, err := os.Stat(path)
	if err != nil {
		return fileSig{}
	}
	return fileSig{exists: true, mod: fi.ModTime(), size: fi.Size()}
}

// setupLogging mengatur logger bawaan agar menulis ke stderr. Stdout dipakai
// protokol MCP, jadi tidak boleh ada log ke sana.
func setupLogging(getenv func(string) string) {
	level := slog.LevelInfo
	invalid := ""
	if raw := strings.TrimSpace(getenv(envLogLevel)); raw != "" {
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			level = slog.LevelInfo
			invalid = raw
		}
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
	if invalid != "" {
		slog.Warn("level log tidak valid, memakai INFO", "nilai", invalid)
	}
}

// baseSpec adalah satu folder dasar beserta modenya.
type baseSpec struct {
	path string
	ro   bool
}

// defaultConfigPath mengembalikan lokasi bawaan berkas config di folder
// config pengguna, atau kosong bila folder itu tidak diketahui. Lokasi ini
// sengaja di luar folder kerja agar server tidak bisa mengubahnya sendiri.
func defaultConfigPath(getenv func(string) string) string {
	dir := ""
	switch {
	case runtime.GOOS == "windows":
		dir = getenv("APPDATA")
	case getenv("XDG_CONFIG_HOME") != "":
		dir = getenv("XDG_CONFIG_HOME")
	case getenv("HOME") != "":
		dir = filepath.Join(getenv("HOME"), ".config")
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mcp-file-manager", "config.json")
}

// loadConfig membaca pengaturan. Urutan: berkas config JSON dari
// MCP_FILE_CONFIG, atau berkas di lokasi bawaan bila ada; bila berkas tidak
// bisa dipakai, MCP_FILE_BASE menjadi cadangan. Tanpa keduanya server tidak
// jalan: tidak ada nilai bawaan agar cakupan akses selalu dipilih secara sadar.
func loadConfig(getenv func(string) string) (*config, error) {
	path := strings.TrimSpace(getenv(envConfig))
	if path == "" {
		if def := defaultConfigPath(getenv); def != "" {
			if _, err := os.Stat(def); err == nil {
				path = def
			}
		}
	}
	if path == "" {
		return loadEnvConfig(getenv)
	}
	cfg, err := loadFileConfig(path, getenv)
	if err == nil {
		return cfg, nil
	}
	if strings.TrimSpace(getenv(envBase)) == "" {
		return nil, fmt.Errorf("config %s tidak bisa dipakai (%s): %w; isi %s sebagai cadangan", path, envConfig, err, envBase)
	}
	slog.Warn("config tidak bisa dipakai, memakai MCP_FILE_BASE", "config", path, "alasan", err)
	cfg, err = loadEnvConfig(getenv)
	if err != nil {
		return nil, err
	}
	// Tetap dipantau: begitu berkas config valid muncul, dipakai tanpa restart.
	cfg.path, cfg.sig, cfg.getenv = absPath(path), statSig(absPath(path)), getenv
	return cfg, nil
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// envLimit membaca batas tulis dari env; nilai tidak valid memakai bawaan.
func envLimit(getenv func(string) string) int64 {
	if raw := strings.TrimSpace(getenv(envMaxWriteBytes)); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			slog.Warn("batas tulis tidak valid, memakai nilai bawaan", "variabel", envMaxWriteBytes, "nilai", raw, "bawaan", defaultMaxWriteBytes)
			return defaultMaxWriteBytes
		}
		return n
	}
	return defaultMaxWriteBytes
}

// loadEnvConfig membaca pengaturan dari variabel lingkungan. MCP_FILE_BASE
// wajib diisi dan harus menunjuk ke folder yang ada. Semua folder bermode rw.
func loadEnvConfig(getenv func(string) string) (*config, error) {
	raw := strings.TrimSpace(getenv(envBase))
	if raw == "" {
		return nil, fmt.Errorf("%s belum diisi: isi dengan folder yang boleh diakses server (contoh: C:/Users/nama/Dokumen)", envBase)
	}
	if strings.ContainsRune(raw, 0) {
		return nil, fmt.Errorf("%s mengandung null byte", envBase)
	}
	var specs []baseSpec
	for _, e := range strings.Split(raw, string(os.PathListSeparator)) {
		if e = strings.TrimSpace(e); e != "" {
			specs = append(specs, baseSpec{path: e})
		}
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("%s belum diisi: isi dengan folder yang boleh diakses server", envBase)
	}
	bases, err := pickBases(specs, envBase)
	if err != nil {
		return nil, err
	}
	cfg := &config{maxReadChars: defaultMaxReadChars, maxEntries: defaultMaxEntries, maxWriteBytes: envLimit(getenv)}
	cfg.setBases(bases)
	return cfg, nil
}

// pickBases menyelesaikan folder dasar dan memeriksa tumpang tindih. Dengan
// satu entri, kegagalan langsung jadi galat; dengan beberapa entri, folder
// yang tidak tersedia dilewati (dengan peringatan) selama masih ada satu yang
// valid.
func pickBases(specs []baseSpec, label string) ([]baseSpec, error) {
	var bases []baseSpec
	var firstErr error
	for _, s := range specs {
		real, err := resolveBase(s.path)
		if err != nil {
			if len(specs) == 1 {
				return nil, fmt.Errorf("%s: %w", label, err)
			}
			slog.Warn("folder dilewati", "folder", s.path, "alasan", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, b := range bases {
			if within(b.path, real) || within(real, b.path) {
				return nil, fmt.Errorf("folder di %s tumpang tindih: %s dan %s", label, b.path, real)
			}
		}
		bases = append(bases, baseSpec{path: real, ro: s.ro})
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("tidak ada folder yang tersedia di %s: %w", label, firstErr)
	}
	return bases, nil
}

// setBases memasang daftar folder dasar yang sudah valid dan tidak kosong.
func (c *config) setBases(bases []baseSpec) {
	c.baseDir = bases[0].path
	c.otherDirs, c.roDirs = nil, nil
	for i, b := range bases {
		if i > 0 {
			c.otherDirs = append(c.otherDirs, b.path)
		}
		if b.ro {
			c.roDirs = append(c.roDirs, b.path)
		}
	}
}

// fileConfig adalah isi berkas config JSON.
type fileConfig struct {
	Bases []struct {
		Path string `json:"path"`
		Mode string `json:"mode"` // "rw" atau "ro"; bawaan "ro"
	} `json:"bases"`
	MaxWriteBytes int64 `json:"max_write_bytes"`
	MaxReadChars  int   `json:"max_read_chars"`
	MaxEntries    int   `json:"max_entries"`
}

// loadFileConfig membaca berkas config JSON. Field yang tidak dikenal
// ditolak agar salah ketik tidak diam-diam melonggarkan akses, dan berkas
// ditolak bila berada di dalam salah satu folder dasar (jika tidak, agen bisa
// melebarkan aksesnya sendiri lewat write_file).
func loadFileConfig(path string, getenv func(string) string) (*config, error) {
	abs := absPath(path)
	sig := statSig(abs)
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("berkas tidak ditemukan atau tidak terbaca: %w", err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("harus berupa berkas biasa")
	}
	if fi.Size() > maxConfigBytes {
		return nil, fmt.Errorf("terlalu besar (maksimal %d byte)", maxConfigBytes)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil {
		return nil, fmt.Errorf("JSON tidak valid: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON tidak valid: ada isi setelah objek utama")
	}
	if len(fc.Bases) == 0 {
		return nil, errors.New(`"bases" harus berisi minimal satu folder`)
	}
	if fc.MaxWriteBytes < 0 || fc.MaxReadChars < 0 || fc.MaxEntries < 0 {
		return nil, errors.New("batas tidak boleh negatif")
	}
	specs := make([]baseSpec, 0, len(fc.Bases))
	for _, b := range fc.Bases {
		p := strings.TrimSpace(b.Path)
		if p == "" || strings.ContainsRune(p, 0) {
			return nil, errors.New(`setiap "path" harus diisi dan tanpa null byte`)
		}
		switch b.Mode {
		case "", "ro", "rw":
		default:
			return nil, fmt.Errorf(`mode "%s" tidak dikenal untuk %s (pilih "rw" atau "ro")`, b.Mode, p)
		}
		specs = append(specs, baseSpec{path: p, ro: b.Mode != "rw"})
	}
	bases, err := pickBases(specs, "config")
	if err != nil {
		return nil, err
	}
	for _, b := range bases {
		if within(b.path, real) {
			return nil, fmt.Errorf("berkas config %s berada di dalam folder dasar %s; pindahkan ke luar folder dasar", real, b.path)
		}
	}
	cfg := &config{
		maxWriteBytes: envLimit(getenv),
		maxReadChars:  defaultMaxReadChars,
		maxEntries:    defaultMaxEntries,
		path:          abs,
		sig:           sig,
		getenv:        getenv,
	}
	if fc.MaxWriteBytes > 0 {
		cfg.maxWriteBytes = fc.MaxWriteBytes
	}
	if fc.MaxReadChars > 0 {
		cfg.maxReadChars = fc.MaxReadChars
	}
	if fc.MaxEntries > 0 {
		cfg.maxEntries = fc.MaxEntries
	}
	cfg.setBases(bases)
	return cfg, nil
}

// refresh memuat ulang berkas config bila berubah sejak diperiksa terakhir.
// Berkas rusak, hilang, atau tidak lolos pemeriksaan: config valid terakhir
// tetap dipakai dan peringatan ditulis ke stderr (sekali per perubahan).
func (c *config) refresh() {
	if c.path == "" {
		return
	}
	sig := statSig(c.path)
	c.mu.RLock()
	same := sig == c.sig
	c.mu.RUnlock()
	if same {
		return
	}
	next, err := loadFileConfig(c.path, c.getenv)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sig = sig
	if err != nil {
		slog.Warn("config tidak valid, memakai config valid terakhir", "config", c.path, "alasan", err)
		return
	}
	c.baseDir, c.otherDirs, c.roDirs = next.baseDir, next.otherDirs, next.roDirs
	c.maxWriteBytes, c.maxReadChars, c.maxEntries = next.maxWriteBytes, next.maxReadChars, next.maxEntries
	slog.Info("config dimuat ulang", "config", c.path, "base_dir", c.baseDir, "folder_lain", c.otherDirs, "hanya_baca", c.roDirs)
}

// resolveBase mengubah satu path folder dasar menjadi path folder absolut
// bebas symlink. Gagal bila folder tidak ada, bukan folder, atau tidak terbaca.
func resolveBase(raw string) (string, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("path tidak valid: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("folder %s tidak ditemukan", abs)
		}
		return "", fmt.Errorf("folder %s tidak bisa dibaca: %w", abs, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("folder %s tidak bisa dibaca: %w", real, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("harus menunjuk ke folder, bukan file: %s", real)
	}
	return real, nil
}

// allBases mengembalikan semua folder dasar; yang pertama adalah baseDir.
func (c *config) allBases() []string {
	return append([]string{c.baseDir}, c.otherDirs...)
}

// inAnyBase melaporkan apakah p (sudah diselesaikan) berada di salah satu
// folder dasar.
func (c *config) inAnyBase(p string) bool {
	if within(c.baseDir, p) {
		return true
	}
	for _, b := range c.otherDirs {
		if within(b, p) {
			return true
		}
	}
	return false
}

// isBase melaporkan apakah p adalah folder dasar itu sendiri.
func (c *config) isBase(p string) bool {
	for _, b := range c.allBases() {
		if rel, err := filepath.Rel(b, p); err == nil && rel == "." {
			return true
		}
	}
	return false
}

// checkWritable menolak penulisan di folder dasar bermode ro. full sudah
// diselesaikan oleh safe atau safeLeaf.
func (c *config) checkWritable(full string) error {
	for _, b := range c.roDirs {
		if within(b, full) {
			return &pathError{fmt.Sprintf("Folder hanya baca (mode ro di config): %s", b)}
		}
	}
	return nil
}

// safeW seperti safe, tetapi juga menolak folder bermode ro. Dipakai semua
// tool yang mengubah isi folder.
func (c *config) safeW(p string) (string, error) {
	full, err := c.safe(p)
	if err != nil {
		return "", err
	}
	if err := c.checkWritable(full); err != nil {
		return "", err
	}
	return full, nil
}

// safeLeafW seperti safeLeaf, tetapi juga menolak folder bermode ro.
func (c *config) safeLeafW(p string) (string, bool, error) {
	full, isLink, err := c.safeLeaf(p)
	if err != nil {
		return "", false, err
	}
	if err := c.checkWritable(full); err != nil {
		return "", false, err
	}
	return full, isLink, nil
}

// guard memuat ulang config bila berubah, lalu menahan pemuatan ulang
// berikutnya sampai panggilan tool ini selesai, supaya satu panggilan melihat
// config yang konsisten.
func (c *config) guard(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "tools/call" {
			return next(ctx, method, req)
		}
		c.refresh()
		c.mu.RLock()
		defer c.mu.RUnlock()
		return next(ctx, method, req)
	}
}
