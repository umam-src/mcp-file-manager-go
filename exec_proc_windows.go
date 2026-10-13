//go:build exec && windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000
)

// setupProc menyembunyikan jendela konsol dan memisahkan grup proses.
func setupProc(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = createNewProcessGroup | createNoWindow
}

// shellCommand menjalankan satu baris perintah lewat cmd.exe, dengan halaman
// kode UTF-8 agar keluaran terbaca benar. Baris diteruskan apa adanya.
func shellCommand(line string) *exec.Cmd {
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /d /c chcp 65001>nul & ` + line}
	return cmd
}

// killTree mematikan proses beserta seluruh anak prosesnya.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
