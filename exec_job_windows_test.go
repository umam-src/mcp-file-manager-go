//go:build exec && windows

package main

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestHelperJob adalah proses bantu: masuk job, menjalankan ping panjang, mencetak pid-nya, lalu menunggu.
func TestHelperJob(t *testing.T) {
	if os.Getenv("GO_HELPER_JOB") != "1" {
		t.Skip("proses bantu")
	}
	guardChildren()
	c := exec.Command("ping", "-n", "120", "127.0.0.1")
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString(strconv.Itoa(c.Process.Pid) + "\n")
	time.Sleep(time.Minute)
}

func TestServerDimatikanPaksaAnakProsesIkutMati(t *testing.T) {
	h := exec.Command(os.Args[0], "-test.run=^TestHelperJob$")
	h.Env = append(os.Environ(), "GO_HELPER_JOB=1")
	out, err := h.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		_ = h.Process.Kill()
		t.Fatalf("pid anak tidak terbaca: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		_ = h.Process.Kill()
		t.Fatalf("pid tidak valid %q", line)
	}
	child, err := os.FindProcess(pid) // memegang handle: pid tidak bisa dipakai ulang
	if err != nil {
		_ = h.Process.Kill()
		t.Fatalf("anak proses tidak ditemukan: %v", err)
	}
	_ = h.Process.Kill() // mati paksa: tanpa execShutdown
	_, _ = h.Process.Wait()
	done := make(chan struct{})
	go func() { _, _ = child.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = child.Kill()
		t.Fatal("anak proses masih hidup setelah server dimatikan paksa")
	}
}
