//go:build exec && !windows

package main

import (
	"os/exec"
	"syscall"
)

// setupProc menjadikan proses pemimpin grup baru agar seluruh pohonnya bisa dimatikan.
func setupProc(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// shellCommand menjalankan satu baris perintah lewat sh.
func shellCommand(line string) *exec.Cmd {
	return exec.Command("sh", "-c", line)
}

// killTree mematikan seluruh grup proses (proses dan anak-anaknya).
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
