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
	baseDir       string // sudah absolut dan bebas symlink
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
	abs, err := filepath.Abs(raw)
	if err != nil {
		return nil, fmt.Errorf("%s tidak valid: %w", envBase, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("folder %s tidak ditemukan (%s)", abs, envBase)
		}
		return nil, fmt.Errorf("%s tidak bisa dibaca: %w", envBase, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return nil, fmt.Errorf("%s tidak bisa dibaca: %w", envBase, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s harus menunjuk ke folder, bukan file: %s", envBase, real)
	}

	cfg := &config{
		baseDir:       real,
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
