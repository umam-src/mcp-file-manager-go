// Command mcp-file-manager-go adalah MCP server (transport stdio) untuk
// mengelola file lokal. Semua akses dibatasi ke folder dasar yang diatur lewat
// berkas config JSON atau MCP_FILE_BASE.
//
// Variabel lingkungan:
//
//	MCP_FILE_CONFIG          berkas config JSON (folder dasar dengan mode rw/ro dan batas hasil),
//	                         dimuat ulang tanpa restart; harus di luar semua folder dasar. Bila
//	                         kosong, dipakai berkas bawaan di folder config pengguna (jika ada)
//	MCP_FILE_BASE            cadangan bila config tidak ada atau tidak valid: folder yang boleh
//	                         diakses (semua rw); beberapa folder dipisah ';' (Windows) atau ':'
//	                         (Linux), path relatif dari folder pertama
//	MCP_FILE_MAX_WRITE_BYTES batas ukuran tulis per operasi (bawaan 5 MiB)
//	MCP_FILE_LOG_LEVEL       level log ke stderr: DEBUG, INFO, WARN, ERROR (bawaan INFO)
//
// PENTING: stdout dipakai protokol MCP. Jangan menulis apa pun ke stdout;
// semua log lewat slog ke stderr.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func slogError(err error) { slog.Error("tool gagal", "error", err) }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-file-manager:", err)
		os.Exit(1)
	}
}

func run() error {
	setupLogging(os.Getenv)
	if err := execCheck(os.Getenv); err != nil {
		return err
	}
	defer execShutdown()
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	slog.Info("mcp-file-manager mulai", "versi", version, "base_dir", cfg.baseDir, "folder_lain", cfg.otherDirs, "hanya_baca", cfg.roDirs, "config", cfg.path)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = newServer(cfg).Run(ctx, &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
