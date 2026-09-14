package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/spec"
	"github.com/uf-cli/uf/internal/tty"
	"github.com/uf-cli/uf/internal/ui"
)

// runWidget adalah mode interaktif yang dipanggil oleh integrasi shell.
//
// Protokolnya sengaja sederhana karena harus bisa diurai oleh zsh, bash,
// fish, dan PowerShell tanpa parser JSON:
//
//	baris pertama : "<status> <posisi-kursor>"
//	sisanya       : isi buffer yang baru, apa adanya
//
// status bernilai "ok" bila pengguna memilih sesuatu, "cancel" bila batal,
// dan "none" bila memang tidak ada kandidat — shell lalu boleh jatuh kembali
// ke completion bawaannya.
//
// Dropdown digambar langsung ke /dev/tty, bukan ke stdout, sehingga keluaran
// yang ditangkap shell tetap bersih.
func runWidget(args []string) int {
	fs := flag.NewFlagSet("widget", flag.ExitOnError)
	line := fs.String("line", "", "isi buffer shell")
	cursor := fs.Int("cursor", -1, "posisi kursor dalam jumlah rune")
	specsDir := fs.String("specs", "", "direktori spec")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Shell melaporkan posisi kursor dalam rune; engine bekerja dalam byte.
	runes := []rune(*line)
	if *cursor < 0 || *cursor > len(runes) {
		*cursor = len(runes)
	}
	byteCursor := len(string(runes[:*cursor]))

	dir, err := resolveSpecsDir(*specsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}
	eng := engine.New(spec.NewRegistry(dir))

	st, outcome, err := interact(eng, ui.State{Line: *line, Cursor: byteCursor})
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	switch outcome {
	case ui.Accepted:
		emit("ok", st.Line, st.Cursor)
	case ui.NoCandidates:
		emit("none", *line, byteCursor)
	default:
		emit("cancel", *line, byteCursor)
	}
	return 0
}

// interact membuka terminal, menjalankan sesi, lalu memulihkan mode terminal.
func interact(eng *engine.Engine, st ui.State) (ui.State, ui.Outcome, error) {
	// Kandidat dihitung lebih dulu. Nol atau satu kandidat tidak memerlukan
	// gambar apa pun, jadi terminal tidak perlu dimasukkan ke mode raw.
	pre, err := ui.Prepare(eng, st)
	if err != nil {
		return st, ui.Cancelled, err
	}
	if out, outcome, done := pre.Immediate(); done {
		return out, outcome, nil
	}

	term, err := tty.Open()
	if err != nil {
		// Tanpa terminal interaktif tidak ada yang bisa digambar. Diperlakukan
		// sebagai "tidak ada kandidat" supaya shell jatuh ke completion bawaan.
		return st, ui.NoCandidates, nil
	}
	defer term.Close()

	w, h := term.Size()
	rend := ui.NewRenderer(term.Out(), w, h, simpleMode())

	return pre.Session(term, rend).Run()
}

// simpleMode mematikan warna dan sorotan pada terminal yang terbatas.
func simpleMode() bool {
	if os.Getenv("UF_SIMPLE") != "" {
		return true
	}
	switch strings.ToLower(os.Getenv("TERM")) {
	case "", "dumb", "vt100", "vt102", "ansi":
		return true
	}
	return false
}

// emit menulis hasil dengan posisi kursor dikembalikan ke satuan rune.
func emit(status, line string, byteCursor int) {
	if byteCursor > len(line) {
		byteCursor = len(line)
	}
	runeCursor := len([]rune(line[:byteCursor]))
	fmt.Printf("%s %d\n%s", status, runeCursor, line)
}
