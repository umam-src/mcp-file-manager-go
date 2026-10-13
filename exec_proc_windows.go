//go:build exec && windows

package main

import (
	"log/slog"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000

	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
)

// Tata letak JOBOBJECT_EXTENDED_LIMIT_INFORMATION (Win32).
type jobBasicLimit struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobExtLimit struct {
	Basic                 jobBasicLimit
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// jobHandle dipegang selama server hidup; Windows menutupnya saat server mati.
var jobHandle syscall.Handle

// guardChildren memasukkan server ke Job Object dengan KILL_ON_JOB_CLOSE. Proses
// yang dijalankan server mewarisi job itu, jadi bila server dimatikan paksa
// (execShutdown tidak sempat jalan) Windows ikut mematikan seluruh proses turunan.
func guardChildren() {
	k := syscall.NewLazyDLL("kernel32.dll")
	h, _, err := k.NewProc("CreateJobObjectW").Call(0, 0)
	if h == 0 {
		slog.Warn("job object gagal dibuat; proses turunan bisa tertinggal bila server dimatikan paksa", "err", err)
		return
	}
	var info jobExtLimit
	info.Basic.LimitFlags = jobObjectLimitKillOnJobClose
	r, _, err := k.NewProc("SetInformationJobObject").Call(h, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r != 0 {
		var cur syscall.Handle
		if cur, err = syscall.GetCurrentProcess(); err == nil {
			r, _, err = k.NewProc("AssignProcessToJobObject").Call(h, uintptr(cur))
		} else {
			r = 0
		}
	}
	if r == 0 {
		_ = syscall.CloseHandle(syscall.Handle(h))
		slog.Warn("job object gagal dipasang; proses turunan bisa tertinggal bila server dimatikan paksa", "err", err)
		return
	}
	jobHandle = syscall.Handle(h)
}

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
