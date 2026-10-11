package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	envBase          = "MCP_FILE_BASE"
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
)

// config memuat pengaturan server. Semua nilainya tetap selama server berjalan.
type config struct {
	baseDir       string   // folder pertama (dasar path relatif), absolut dan bebas symlink
	otherDirs     []string // folder dasar tambahan, kosong bila hanya satu folder
	maxWriteBytes int64
	maxReadChars  int
	maxEntries    int
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

// loadConfig membaca pengaturan dari variabel lingkungan. MCP_FILE_BASE wajib
// diisi dan harus menunjuk ke folder yang ada: tidak ada nilai bawaan agar
// cakupan akses selalu dipilih secara sadar.
func loadConfig(getenv func(string) string) (*config, error) {
	raw := strings.TrimSpace(getenv(envBase))
	if raw == "" {
		return nil, fmt.Errorf("%s belum diisi: isi dengan folder yang boleh diakses server (contoh: C:/Users/nama/Dokumen)", envBase)
	}
	if strings.ContainsRune(raw, 0) {
		return nil, fmt.Errorf("%s mengandung null byte", envBase)
	}
	var entries []string
	for _, e := range strings.Split(raw, string(os.PathListSeparator)) {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s belum diisi: isi dengan folder yang boleh diakses server", envBase)
	}
	var bases []string
	var firstErr error
	for _, e := range entries {
		real, err := resolveBase(e)
		if err != nil {
			if len(entries) == 1 {
				return nil, err
			}
			slog.Warn("folder dilewati", "folder", e, "alasan", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, b := range bases {
			if within(b, real) || within(real, b) {
				return nil, fmt.Errorf("folder di %s tumpang tindih: %s dan %s", envBase, b, real)
			}
		}
		bases = append(bases, real)
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("tidak ada folder yang tersedia di %s: %w", envBase, firstErr)
	}

	cfg := &config{
		baseDir:       bases[0],
		otherDirs:     bases[1:],
		maxWriteBytes: defaultMaxWriteBytes,
		maxReadChars:  defaultMaxReadChars,
		maxEntries:    defaultMaxEntries,
	}
	if raw := strings.TrimSpace(getenv(envMaxWriteBytes)); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			slog.Warn("batas tulis tidak valid, memakai nilai bawaan", "variabel", envMaxWriteBytes, "nilai", raw, "bawaan", defaultMaxWriteBytes)
		} else {
			cfg.maxWriteBytes = n
		}
	}
	return cfg, nil
}

// resolveBase mengubah satu entri MCP_FILE_BASE menjadi path folder absolut
// bebas symlink. Gagal bila folder tidak ada, bukan folder, atau tidak terbaca.
func resolveBase(raw string) (string, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("%s tidak valid: %w", envBase, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("folder %s tidak ditemukan (%s)", abs, envBase)
		}
		return "", fmt.Errorf("%s tidak bisa dibaca: %w", envBase, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("%s tidak bisa dibaca: %w", envBase, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%s harus menunjuk ke folder, bukan file: %s", envBase, real)
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
