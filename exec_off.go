//go:build !exec

package main

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Varian normal: tidak ada kode eksekusi proses. Berkas exec_on.go (build tag
// "exec") memuat varian mcp-file-manager-exec; simbol di bawah hanya penahan
// agar kode bersama tetap sama di kedua varian.
const (
	serverName       = "mcp-file-manager"
	execInstructions = ""
	execToolCount    = 0
)

var execToolNames = map[string]bool{}

func execCheck(func(string) string) error { return nil }

func registerExec(*mcp.Server, *config) {}

func execShutdown() {}
