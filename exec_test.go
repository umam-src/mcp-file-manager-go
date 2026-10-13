//go:build exec

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Proses pembantu: biner test ini dijalankan ulang dengan argumen "-- mode".
// Env proses dibersihkan, jadi mode dikirim lewat argumen.
func helperMode() string {
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func TestHelperProses(t *testing.T) {
	switch helperMode() {
	case "":
		return
	case "tidur":
		time.Sleep(time.Minute)
	case "besar":
		fmt.Print(strings.Repeat("x", 100000))
	case "keluar3":
		os.Exit(3)
	case "bertahap":
		fmt.Println("satu")
		time.Sleep(400 * time.Millisecond)
		fmt.Println("dua")
	case "gema":
		// Membalas tiap baris stdin dengan "got:<baris>"; selesai saat EOF atau "quit".
		fmt.Println("siap")
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			if sc.Text() == "quit" {
				break
			}
			fmt.Println("got:" + sc.Text())
		}
		fmt.Println("selesai")
	}
	os.Exit(0)
}

func helperArgs(mode string) startInput {
	return startInput{Program: os.Args[0], Args: []string{"-test.run=^TestHelperProses$", "--", mode}}
}

func newMgr(t *testing.T) *execMgr {
	t.Helper()
	m := &execMgr{cfg: &config{baseDir: t.TempDir()}, procs: map[int]*proc{}}
	t.Cleanup(func() {
		for _, p := range m.procs {
			if !p.isExited() {
				killTree(p.cmd)
			}
		}
	})
	return m
}

func pidOf(t *testing.T, r result) int {
	t.Helper()
	pid, ok := r["pid"].(int)
	if !ok {
		t.Fatalf("tanpa pid: %v", r)
	}
	return pid
}

func TestExecCheck(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	if execCheck(env("")) == nil || execCheck(env("tidak")) == nil {
		t.Fatal("tanpa MCP_SANDBOX=ya harus ditolak")
	}
	if err := execCheck(env("ya")); err != nil {
		t.Fatalf("MCP_SANDBOX=ya harus diterima: %v", err)
	}
}

func TestStartGagal(t *testing.T) {
	m := newMgr(t)
	if r := m.start(startInput{}); r["error"] == nil {
		t.Fatalf("program kosong harus gagal: %v", r)
	}
	if r := m.start(startInput{Program: "program-tidak-ada-xyz"}); r["error"] == nil {
		t.Fatalf("program tidak ada harus gagal: %v", r)
	}
	if r := m.start(startInput{Program: os.Args[0], Cwd: "Z:\\tidak\\ada"}); r["error"] == nil {
		t.Fatalf("cwd tidak ada harus gagal: %v", r)
	}
}

func TestKodeKeluarDanShell(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("keluar3")
	in.WaitMs = 10000
	r := m.start(in)
	if r["status"] != "exited" || r["exit_code"] != 3 {
		t.Fatalf("harap exited kode 3: %v", r)
	}
	r = m.start(startInput{Program: "echo halo", Shell: true, WaitMs: 10000})
	if r["status"] != "exited" || !strings.Contains(fmt.Sprint(r["output"]), "halo") {
		t.Fatalf("shell: %v", r)
	}
}

func TestTimeoutMematikanProses(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("tidur")
	in.TimeoutS, in.WaitMs = 1, 15000
	t0 := time.Now()
	r := m.start(in)
	if r["status"] != "exited" || r["timed_out"] != true {
		t.Fatalf("harap exited dan timed_out: %v", r)
	}
	if time.Since(t0) > 12*time.Second {
		t.Fatalf("timeout terlalu lama: %v", time.Since(t0))
	}
}

func TestKillProses(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("tidur")
	in.WaitMs = 200
	r := m.start(in)
	if r["status"] != "running" {
		t.Fatalf("harap running: %v", r)
	}
	pid := pidOf(t, r)
	if r = m.kill(killInput{Pid: pid}); r["status"] != "exited" {
		t.Fatalf("harap exited setelah kill: %v", r)
	}
	if r = m.kill(killInput{Pid: 999999}); r["error"] == nil {
		t.Fatalf("pid tidak dikenal harus gagal: %v", r)
	}
	if r = m.read(readProcInput{Pid: 999999}); r["error"] == nil {
		t.Fatalf("pid tidak dikenal harus gagal: %v", r)
	}
}

func TestKeluaranBesarBertahap(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("besar")
	in.WaitMs = 10000
	r := m.start(in)
	if r["truncated"] != true {
		t.Fatalf("100.000 byte harus terpotong: len=%d", len(fmt.Sprint(r["output"])))
	}
	total := len(r["output"].(string))
	for i := 0; i < 20 && r["truncated"] == true; i++ {
		r = m.read(readProcInput{Pid: pidOf(t, r), Length: 30000})
		total += len(fmt.Sprint(r["output"]))
	}
	if total != 100000 {
		t.Fatalf("total keluaran = %d, harapan 100000", total)
	}
	if r = m.read(readProcInput{Pid: pidOf(t, r), Offset: ptr(int64(0)), Length: 10}); len(r["output"].(string)) != 10 {
		t.Fatalf("baca ulang dari offset 0: %v", r)
	}
}

func TestBacaBertahapSampaiSelesai(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("bertahap")
	in.WaitMs = 50
	r := m.start(in)
	pid, all := pidOf(t, r), fmt.Sprint(r["output"])
	for i := 0; i < 30 && r["status"] == "running"; i++ {
		r = m.read(readProcInput{Pid: pid, WaitMs: 2000})
		if s, ok := r["output"].(string); ok {
			all += s
		}
	}
	if r["status"] != "exited" || !strings.Contains(all, "satu") || !strings.Contains(all, "dua") {
		t.Fatalf("status=%v keluaran=%q", r["status"], all)
	}
}

func TestBatasProsesBerjalan(t *testing.T) {
	m := newMgr(t)
	for i := 0; i < maxRunning; i++ {
		in := helperArgs("tidur")
		in.WaitMs = 10
		if r := m.start(in); r["error"] != nil {
			t.Fatalf("proses %d: %v", i, r)
		}
	}
	if r := m.start(helperArgs("tidur")); r["error"] == nil {
		t.Fatalf("proses ke-%d harus ditolak: %v", maxRunning+1, r)
	}
}

func outStr(r result) string {
	s, _ := r["output"].(string)
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func TestInteraktifBolakBalik(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("gema")
	in.Interactive = true
	in.WaitMs = 5000
	r := m.start(in)
	if r["error"] != nil || r["status"] != "running" {
		t.Fatalf("harus berjalan: %v", r)
	}
	pid := pidOf(t, r)
	if !strings.Contains(outStr(r), "siap") {
		// keluaran awal bisa belum tiba saat start_process kembali
		r = m.read(readProcInput{Pid: pid, WaitMs: 5000})
		if !strings.Contains(outStr(r), "siap") {
			t.Fatalf("tanpa 'siap': %v", r)
		}
	}
	for _, kata := range []string{"halo", "dunia"} {
		r = m.interact(interactInput{Pid: pid, Input: kata, WaitMs: 5000})
		if r["error"] != nil || !strings.Contains(outStr(r), "got:"+kata) {
			t.Fatalf("balasan %q salah: %v", kata, r)
		}
	}
	// "quit" membuat proses selesai; hasilnya status exited dengan kode 0.
	r = m.interact(interactInput{Pid: pid, Input: "quit", WaitMs: 5000})
	m.get(pid).waitFor(5*time.Second, false, 0)
	r = m.read(readProcInput{Pid: pid, WaitMs: 100})
	if r["status"] != "exited" || r["exit_code"] != 0 {
		t.Fatalf("harus selesai kode 0: %v", r)
	}
}

func TestInteraktifCloseKirimEOF(t *testing.T) {
	m := newMgr(t)
	in := helperArgs("gema")
	in.Interactive = true
	pid := pidOf(t, m.start(in))
	r := m.interact(interactInput{Pid: pid, Input: "satu", Close: true, WaitMs: 5000})
	if r["error"] != nil {
		t.Fatalf("close gagal: %v", r)
	}
	m.get(pid).waitFor(5*time.Second, false, 0)
	r = m.read(readProcInput{Pid: pid, Offset: ptr(int64(0)), WaitMs: 100})
	if r["status"] != "exited" || !strings.Contains(outStr(r), "got:satu") || !strings.Contains(outStr(r), "selesai") {
		t.Fatalf("EOF harus mengakhiri proses setelah membalas: %v", r)
	}
}

func TestInteraktifGagal(t *testing.T) {
	m := newMgr(t)
	if r := m.interact(interactInput{Pid: 999999, Input: "x"}); r["error"] == nil {
		t.Fatalf("pid tidak dikenal harus gagal: %v", r)
	}
	// Proses non-interaktif menolak masukan.
	pid := pidOf(t, m.start(helperArgs("tidur")))
	if r := m.interact(interactInput{Pid: pid, Input: "x"}); r["error"] == nil {
		t.Fatalf("non-interaktif harus ditolak: %v", r)
	}
	// Masukan terlalu besar.
	in := helperArgs("gema")
	in.Interactive = true
	pid2 := pidOf(t, m.start(in))
	if r := m.interact(interactInput{Pid: pid2, Input: strings.Repeat("a", maxInput+1)}); r["error"] == nil {
		t.Fatalf("masukan terlalu besar harus ditolak: %v", r)
	}
	// Proses yang sudah selesai menolak masukan.
	m.interact(interactInput{Pid: pid2, Input: "quit", WaitMs: 5000})
	m.get(pid2).waitFor(5*time.Second, false, 0)
	if r := m.interact(interactInput{Pid: pid2, Input: "x"}); r["error"] == nil {
		t.Fatalf("proses selesai harus ditolak: %v", r)
	}
}

func TestNonInteraktifStdinTertutup(t *testing.T) {
	// Tanpa interactive, stdin tertutup: program yang membaca stdin langsung dapat EOF.
	m := newMgr(t)
	in := helperArgs("gema")
	in.WaitMs = 5000
	r := m.start(in)
	if r["status"] != "exited" || !strings.Contains(outStr(r), "selesai") {
		t.Fatalf("stdin tertutup harus langsung selesai: %v", r)
	}
}

func TestEnvMeneruskanVariabelAman(t *testing.T) {
	t.Setenv("USERNAME", "uji")
	t.Setenv("PGPASSWORD", "rahasia")
	var gotUser, gotSecret bool
	for _, kv := range cleanEnv() {
		gotUser = gotUser || kv == "USERNAME=uji"
		gotSecret = gotSecret || strings.HasPrefix(kv, "PGPASSWORD=")
	}
	if !gotUser {
		t.Error("USERNAME harus diteruskan")
	}
	if gotSecret {
		t.Error("variabel rahasia tidak boleh diteruskan")
	}
}
