//go:build exec

package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Varian mcp-file-manager-exec (build tag "exec"): menambah tool proses tanpa
// batas folder. HANYA untuk mesin terisolasi (VM, container, Windows Sandbox);
// server menolak jalan tanpa MCP_SANDBOX=ya. Lihat docs/alat.md.
const (
	serverName       = "mcp-file-manager-exec"
	execToolCount    = 4
	execInstructions = " Varian exec: start_process menjalankan program (program + args, atau shell=true dengan satu baris perintah) tanpa batas folder; " +
		"lanjutkan keluaran dengan read_process_output(pid) dan hentikan dengan kill_process(pid). Proses berhenti sendiri setelah timeout_s."

	envSandbox = "MCP_SANDBOX"

	maxRunning  = 8         // proses berjalan bersamaan
	maxKept     = 20        // proses selesai yang disimpan untuk dibaca
	bufCap      = 256 << 10 // byte keluaran per proses (yang lama dibuang)
	readCap     = 20000     // byte keluaran per hasil
	defTimeout  = 120       // detik
	maxTimeout  = 3600
	defWait     = 3000 // ms: tunggu start_process
	defReadWait = 1000 // ms: tunggu read_process_output
	maxWait     = 30000
	maxInput    = 64 << 10 // byte masukan per interact_with_process
	settleGap   = 120 * time.Millisecond
	writeWait   = 5 * time.Second
)

var execToolNames = map[string]bool{"start_process": true, "read_process_output": true, "kill_process": true, "interact_with_process": true}

// execCheck menolak mulai bila pengguna belum menyatakan mesinnya terisolasi.
func execCheck(getenv func(string) string) error {
	if strings.TrimSpace(getenv(envSandbox)) != "ya" {
		return errors.New("varian exec menjalankan perintah bebas dan hanya boleh di mesin terisolasi; isi " + envSandbox + "=ya untuk mengonfirmasi")
	}
	return nil
}

// envAllow adalah satu-satunya variabel lingkungan yang diteruskan ke proses
// (tanpa token atau rahasia host).
var envAllow = []string{"PATH", "PATHEXT", "SYSTEMROOT", "SYSTEMDRIVE", "COMSPEC", "WINDIR", "TEMP", "TMP",
	"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "PROGRAMFILES", "PROGRAMFILES(X86)", "PROGRAMDATA",
	"LANG", "LC_ALL", "TERM", "TZ"}

func cleanEnv() []string {
	out := make([]string, 0, len(envAllow)+1)
	for _, k := range envAllow {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	return append(out, "PYTHONIOENCODING=utf-8")
}

type startInput struct {
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
	Shell   bool     `json:"shell,omitempty"`
	// Interactive membuka stdin agar proses bisa menerima masukan lewat interact_with_process.
	Interactive bool `json:"interactive,omitempty"`
	TimeoutS    int  `json:"timeout_s,omitempty"`
	WaitMs      int  `json:"wait_ms,omitempty"`
}

type interactInput struct {
	Pid    int    `json:"pid"`
	Input  string `json:"input,omitempty"`
	Close  bool   `json:"close,omitempty"`
	WaitMs int    `json:"wait_ms,omitempty"`
}

type readProcInput struct {
	Pid    int    `json:"pid"`
	Offset *int64 `json:"offset,omitempty"`
	Length int    `json:"length,omitempty"`
	WaitMs int    `json:"wait_ms,omitempty"`
}

type killInput struct {
	Pid int `json:"pid"`
}

// proc adalah satu proses yang dimulai server. Keluaran (stdout dan stderr
// digabung) ditampung berbatas bufCap; offset adalah posisi byte sejak awal.
type proc struct {
	pid     int
	cmd     *exec.Cmd
	started time.Time
	done    chan struct{}
	stdin   io.WriteCloser // nil bila tidak interaktif
	stdinMu sync.Mutex

	mu       sync.Mutex
	buf      []byte
	base     int64 // offset byte pertama di buf
	readPos  int64
	exited   bool
	code     int
	timedOut bool
}

func (p *proc) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	if over := len(p.buf) - bufCap; over > 0 {
		p.buf = append(p.buf[:0], p.buf[over:]...)
		p.base += int64(over)
	}
	return len(b), nil
}

func (p *proc) isExited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

// status mengembalikan pid, status, dan (bila selesai) kode keluar.
func (p *proc) statusLocked() result {
	res := result{"pid": p.pid, "status": "running"}
	if p.exited {
		res["status"] = "exited"
		res["exit_code"] = p.code
		if p.timedOut {
			res["timed_out"] = true
		}
	}
	return res
}

// pos mengembalikan offset baca efektif.
func (p *proc) pos(offset *int64) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if offset != nil {
		return *offset
	}
	return p.readPos
}

// waitFor menunggu proses selesai (needOutput=false) atau ada keluaran baru
// setelah from (needOutput=true), paling lama d.
func (p *proc) waitFor(d time.Duration, needOutput bool, from int64) {
	deadline := time.Now().Add(d)
	for {
		p.mu.Lock()
		total := p.base + int64(len(p.buf))
		ex := p.exited
		p.mu.Unlock()
		if ex || (needOutput && total > from) || !time.Now().Before(deadline) {
			return
		}
		select {
		case <-p.done:
			return
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// snapshot membaca keluaran dari offset (bawaan: lanjutan bacaan terakhir).
func (p *proc) snapshot(offset *int64, length int) result {
	p.mu.Lock()
	defer p.mu.Unlock()
	res := p.statusLocked()
	total := p.base + int64(len(p.buf))
	from := p.readPos
	if offset != nil {
		from = *offset
	}
	if from < 0 {
		from = 0
	}
	if from < p.base {
		from = p.base
		res["dropped"] = true
	}
	if from > total {
		from = total
	}
	if length <= 0 || length > readCap {
		length = readCap
	}
	start := from - p.base
	chunk := p.buf[start:]
	if len(chunk) > length {
		chunk = chunk[:length]
		for i := start + int64(len(chunk)); len(chunk) > 0 && !utf8.RuneStart(p.buf[i]); i-- {
			chunk = chunk[:len(chunk)-1]
		}
		res["truncated"] = true
	}
	p.readPos = from + int64(len(chunk))
	if len(chunk) > 0 {
		res["output"] = strings.ToValidUTF8(string(chunk), "?")
	}
	res["next_offset"] = p.readPos
	return res
}

type execMgr struct {
	cfg   *config
	mu    sync.Mutex
	procs map[int]*proc
}

var theExec *execMgr

func clamp(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func (m *execMgr) get(pid int) *proc {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[pid]
}

func (m *execMgr) start(in startInput) result {
	if strings.TrimSpace(in.Program) == "" {
		return fail("program kosong")
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = m.cfg.baseDir
	}
	if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
		return fail("cwd tidak ada atau bukan folder: " + cwd)
	}
	timeout := time.Duration(clamp(in.TimeoutS, defTimeout, maxTimeout)) * time.Second

	var cmd *exec.Cmd
	if in.Shell {
		cmd = shellCommand(in.Program)
	} else {
		cmd = exec.Command(in.Program, in.Args...)
	}
	cmd.Dir = cwd
	cmd.Env = cleanEnv()
	cmd.WaitDelay = 2 * time.Second // tidak menggantung bila anak proses memegang pipa
	setupProc(cmd)
	p := &proc{started: time.Now(), done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = p, p
	if in.Interactive {
		w, err := cmd.StdinPipe()
		if err != nil {
			return fail("gagal membuka stdin: " + err.Error())
		}
		p.stdin = w
	}
	m.mu.Lock()
	running := 0
	for _, q := range m.procs {
		if !q.isExited() {
			running++
		}
	}
	if running >= maxRunning {
		m.mu.Unlock()
		return fail("terlalu banyak proses berjalan (maks 8); hentikan dengan kill_process")
	}
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return fail("gagal menjalankan: " + err.Error())
	}
	p.cmd, p.pid = cmd, cmd.Process.Pid
	m.procs[p.pid] = p
	m.evictLocked()
	m.mu.Unlock()

	slog.Info("exec mulai", "pid", p.pid, "program", in.Program, "args", in.Args, "shell", in.Shell, "cwd", cwd, "timeout", timeout)
	go p.wait(timeout)
	wait := time.Duration(clamp(in.WaitMs, defWait, maxWait)) * time.Millisecond
	if in.Interactive {
		p.waitSettle(wait, 0) // prompt awal sudah tampil: tidak perlu menunggu proses selesai
	} else {
		p.waitFor(wait, false, 0)
	}
	return p.snapshot(nil, 0)
}

// wait menunggu proses selesai; setelah timeout seluruh pohonnya dimatikan.
func (p *proc) wait(timeout time.Duration) {
	timer := time.AfterFunc(timeout, func() {
		p.mu.Lock()
		p.timedOut = true
		p.mu.Unlock()
		killTree(p.cmd)
	})
	_ = p.cmd.Wait()
	timer.Stop()
	p.mu.Lock()
	p.exited = true
	p.code = p.cmd.ProcessState.ExitCode()
	p.mu.Unlock()
	close(p.done)
	slog.Info("exec selesai", "pid", p.pid, "kode", p.code, "timeout", p.timedOut, "durasi", time.Since(p.started).Round(time.Millisecond))
}

// evictLocked membuang proses selesai tertua bila melebihi maxKept.
func (m *execMgr) evictLocked() {
	for {
		var oldest *proc
		finished := 0
		for _, q := range m.procs {
			if q.isExited() {
				finished++
				if oldest == nil || q.started.Before(oldest.started) {
					oldest = q
				}
			}
		}
		if finished <= maxKept || oldest == nil {
			return
		}
		delete(m.procs, oldest.pid)
	}
}

func (m *execMgr) read(in readProcInput) result {
	p := m.get(in.Pid)
	if p == nil {
		return fail("pid tidak dikenal; hanya proses yang dimulai server ini yang bisa dibaca")
	}
	p.waitFor(time.Duration(clamp(in.WaitMs, defReadWait, maxWait))*time.Millisecond, true, p.pos(in.Offset))
	return p.snapshot(in.Offset, in.Length)
}

func (m *execMgr) kill(in killInput) result {
	p := m.get(in.Pid)
	if p == nil {
		return fail("pid tidak dikenal; hanya proses yang dimulai server ini yang bisa dihentikan")
	}
	if !p.isExited() {
		slog.Info("exec dihentikan", "pid", p.pid)
		killTree(p.cmd)
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.statusLocked()
}

// total mengembalikan offset byte keluaran saat ini.
func (p *proc) total() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.base + int64(len(p.buf))
}

// waitSettle menunggu keluaran baru setelah from (paling lama d), lalu sampai
// keluaran diam selama settleGap, supaya balasan satu masukan terbaca utuh.
func (p *proc) waitSettle(d time.Duration, from int64) {
	deadline := time.Now().Add(d)
	p.waitFor(d, true, from)
	last := p.total()
	for time.Now().Before(deadline) && !p.isExited() {
		time.Sleep(settleGap)
		now := p.total()
		if now == last {
			return
		}
		last = now
	}
}

// closeStdin menutup stdin (EOF); aman dipanggil berulang.
func (p *proc) closeStdin() {
	p.stdinMu.Lock()
	defer p.stdinMu.Unlock()
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
}

func (m *execMgr) interact(in interactInput) result {
	p := m.get(in.Pid)
	if p == nil {
		return fail("pid tidak dikenal; hanya proses yang dimulai server ini yang bisa diberi masukan")
	}
	if p.stdin == nil {
		return fail("proses tidak interaktif; mulai ulang start_process dengan interactive=true")
	}
	if p.isExited() {
		return fail("proses sudah selesai; baca sisa keluaran dengan read_process_output")
	}
	if len(in.Input) > maxInput {
		return fail("masukan terlalu besar (maks 64 KiB); kirim bertahap")
	}
	from := p.total()
	if in.Input != "" {
		s := in.Input
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		errc := make(chan error, 1)
		go func() {
			p.stdinMu.Lock()
			defer p.stdinMu.Unlock()
			_, err := io.WriteString(p.stdin, s)
			errc <- err
		}()
		select {
		case err := <-errc:
			if err != nil {
				return fail("gagal menulis ke stdin: " + err.Error())
			}
		case <-time.After(writeWait):
			return fail("stdin penuh: proses tidak membaca masukan; hentikan dengan kill_process bila macet")
		}
	}
	if in.Close {
		p.closeStdin()
	}
	p.waitSettle(time.Duration(clamp(in.WaitMs, defWait, maxWait))*time.Millisecond, from)
	return p.snapshot(nil, 0)
}

// execShutdown mematikan semua proses yang masih berjalan saat server berhenti.
func execShutdown() {
	if theExec == nil {
		return
	}
	theExec.mu.Lock()
	defer theExec.mu.Unlock()
	for _, p := range theExec.procs {
		if !p.isExited() {
			killTree(p.cmd)
		}
	}
}

func registerExec(srv *mcp.Server, cfg *config) {
	m := &execMgr{cfg: cfg, procs: map[int]*proc{}}
	theExec = m
	yes, no := true, false

	addTool(srv, &mcp.Tool{
		Name: "start_process",
		Description: "Jalankan program (program + args; shell=true untuk satu baris perintah) tanpa batas folder, lalu kembalikan pid dan keluaran awal (menunggu hingga wait_ms). " +
			"Berhenti sendiri setelah timeout_s (bawaan 120, maks 3600); status 'running' atau 'exited' dengan exit_code.",
		Annotations: &mcp.ToolAnnotations{Title: "Jalankan proses", DestructiveHint: &yes, OpenWorldHint: &yes},
	}, m.start)

	addTool(srv, &mcp.Tool{
		Name: "read_process_output",
		Description: "Baca keluaran proses dari offset (bawaan: lanjutan bacaan terakhir); menunggu hingga wait_ms bila belum ada keluaran baru. " +
			"Hasil dibatasi ('truncated'; lanjutkan dari 'next_offset'); keluaran lama yang terbuang ditandai 'dropped'.",
		Annotations: &mcp.ToolAnnotations{Title: "Baca keluaran proses", ReadOnlyHint: true, DestructiveHint: &no, OpenWorldHint: &no},
	}, m.read)

	addTool(srv, &mcp.Tool{
		Name:        "kill_process",
		Description: "Hentikan proses beserta seluruh anak prosesnya.",
		Annotations: &mcp.ToolAnnotations{Title: "Hentikan proses", DestructiveHint: &yes, IdempotentHint: true, OpenWorldHint: &no},
	}, m.kill)

	addTool(srv, &mcp.Tool{
		Name: "interact_with_process",
		Description: "Kirim satu baris masukan ke stdin proses yang dimulai dengan interactive=true (REPL, prompt), lalu kembalikan keluaran baru setelah diam (hingga wait_ms). " +
			"close=true menutup stdin (EOF).",
		Annotations: &mcp.ToolAnnotations{Title: "Beri masukan ke proses", DestructiveHint: &yes, OpenWorldHint: &yes},
	}, m.interact)
}
