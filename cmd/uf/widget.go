package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/generator"
	"github.com/uf-cli/uf/internal/spec"
	"github.com/uf-cli/uf/internal/tty"
	"github.com/uf-cli/uf/internal/ui"
	"golang.org/x/term"
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
	cursor := fs.Int("cursor", -1, "posisi kursor")
	unit := fs.String("cursor-unit", "rune", "satuan posisi kursor: rune, byte, atau utf16")
	prev := fs.Int("prev-lines", 0, "baris yang sudah digambar mode render")
	sel := fs.String("select", "first", "baris yang tersorot saat dibuka: first atau last")
	alias := fs.String("alias", "", "pemekaran alias untuk kata pertama")
	specsDir := fs.String("specs", "", "direktori spec")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	byteCursor := toByteCursor(*line, *cursor, *unit)

	dirs, err := resolveSpecsDirs(*specsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}
	eng := engine.New(spec.NewRegistryDirs(dirs...))

	start := 0
	if *sel == "last" {
		start = -1
	}

	// Perhitungan memakai bentuk yang sudah dimekarkan; hasilnya dipetakan
	// kembali ke baris asli sebelum diserahkan ke shell.
	ax := newAliasExpansion(*line, byteCursor, *alias)
	st, outcome, err := interact(eng,
		ui.State{Line: ax.Line(*line), Cursor: ax.Cursor(byteCursor)}, *prev, start)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	switch outcome {
	case ui.Accepted:
		restored, c := ax.Restore(st.Line, st.Cursor)
		emit("ok", restored, c, *unit)
	case ui.NoCandidates:
		emit("none", *line, byteCursor, *unit)
	default:
		emit("cancel", *line, byteCursor, *unit)
	}
	return 0
}

// interact membuka terminal, menjalankan sesi, lalu memulihkan mode terminal.
func interact(eng *engine.Engine, st ui.State, prevLines, start int) (ui.State, ui.Outcome, error) {
	// Kandidat dihitung lebih dulu. Nol atau satu kandidat tidak memerlukan
	// gambar apa pun, jadi terminal tidak perlu dimasukkan ke mode raw.
	pre, err := ui.Prepare(eng, st, newDynamic())
	if err != nil {
		return st, ui.Cancelled, err
	}
	if out, outcome, done := pre.Immediate(); done {
		// Dropdown yang sedang tergambar oleh mode otomatis harus tetap
		// dibersihkan, meski sesi interaktif tidak jadi dibuka.
		clearLeftover(prevLines)
		return out, outcome, nil
	}

	term, err := tty.Open()
	if err != nil {
		clearLeftover(prevLines)
		// Tanpa terminal interaktif tidak ada yang bisa digambar. Diperlakukan
		// sebagai "tidak ada kandidat" supaya shell jatuh ke completion bawaan.
		return st, ui.NoCandidates, nil
	}
	defer term.Close()

	w, h := term.Size()
	rend := ui.NewRenderer(term.Out(), w, h, simpleMode())
	// Baris yang sudah digambar mode otomatis diambil alih, bukan ditumpuk.
	rend.Adopt(prevLines)

	return pre.Session(term, rend).StartAt(start).Run()
}

// clearLeftover menghapus dropdown yang tertinggal dari mode otomatis.
func clearLeftover(lines int) {
	if lines <= 0 {
		return
	}
	f, err := os.OpenFile(ttyDevice, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()

	w, h := 80, 24
	if tw, th, err := term.GetSize(int(f.Fd())); err == nil && tw > 0 && th > 0 {
		w, h = tw, th
	}
	r := ui.NewRenderer(f, w, h, simpleMode())
	r.Adopt(lines)
	r.Clear()
}

// newDynamic menyiapkan sumber kandidat dinamis.
//
// Generator MENJALANKAN PERINTAH sebagai efek samping mengetik, jadi
// kebijakannya dibaca dari lingkungan setiap kali dipanggil, bukan disimpan
// sebagai state: pengguna harus bisa mematikannya untuk satu sesi tanpa
// memasang ulang apa pun.
func newDynamic() *generator.Source {
	return &generator.Source{
		Dir:     generator.CurrentDir(),
		Timeout: generatorTimeout(),
		Cache:   generator.NewCache(),
	}
}

// generatorTimeout membaca batas waktu dari lingkungan.
func generatorTimeout() time.Duration {
	v := os.Getenv(generator.EnvTimeout)
	if v == "" {
		return generator.DefaultTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return generator.DefaultTimeout
	}
	return d
}

// ttyDevice adalah terminal pengendali pada sistem mirip Unix.
const ttyDevice = "/dev/tty"

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

// Satuan posisi kursor yang dipahami mode widget.
//
// Tiga satuan, bukan karena kelebihan pilihan melainkan karena setiap keluarga
// shell memang melaporkannya berbeda:
//
//	rune   zsh ($CURSOR) dan fish (commandline -C) menghitung karakter
//	byte   READLINE_POINT milik bash menghitung byte
//	utf16  PSReadLine memakai indeks string .NET, yaitu UTF-16 code unit
//
// Perbedaannya tidak terlihat sama sekali pada baris ASCII. Pada huruf beraksen
// dan CJK, rune dan byte berpisah; pada emoji dan karakter di luar BMP, rune
// dan utf16 juga berpisah karena satu rune di sana memakan dua code unit.
const (
	unitRune  = "rune"
	unitByte  = "byte"
	unitUTF16 = "utf16"
)

// toByteCursor menerjemahkan posisi kursor yang dilaporkan shell menjadi
// offset byte yang dipakai engine.
func toByteCursor(line string, cursor int, unit string) int {
	switch unit {
	case unitByte:
		if cursor < 0 || cursor > len(line) {
			return len(line)
		}
		return cursor

	case unitUTF16:
		if cursor < 0 {
			return len(line)
		}
		units := 0
		for i, r := range line {
			if units >= cursor {
				return i
			}
			// Kursor yang jatuh di TENGAH pasangan surrogate bukan posisi yang
			// sah. Dibulatkan ke bawah, karena kursor berarti "sebelum karakter
			// ini": membulatkan ke atas akan melewati karakter yang belum
			// dilewati pengguna.
			if units+utf16Len(r) > cursor {
				return i
			}
			units += utf16Len(r)
		}
		return len(line)

	default:
		runes := []rune(line)
		if cursor < 0 || cursor > len(runes) {
			return len(line)
		}
		return len(string(runes[:cursor]))
	}
}

// fromByteCursor mengembalikan posisi kursor ke satuan yang diminta pemanggil.
func fromByteCursor(line string, byteCursor int, unit string) int {
	if byteCursor > len(line) {
		byteCursor = len(line)
	}
	switch unit {
	case unitByte:
		return byteCursor
	case unitUTF16:
		units := 0
		for _, r := range line[:byteCursor] {
			units += utf16Len(r)
		}
		return units
	default:
		return len([]rune(line[:byteCursor]))
	}
}

// utf16Len adalah jumlah code unit UTF-16 yang dipakai sebuah rune.
func utf16Len(r rune) int {
	if r > 0xFFFF {
		return 2 // pasangan surrogate
	}
	return 1
}

// emit menulis hasil dengan posisi kursor dikembalikan ke satuan yang diminta.
func emit(status, line string, byteCursor int, unit string) {
	fmt.Printf("%s %d\n%s", status, fromByteCursor(line, byteCursor, unit), line)
}
