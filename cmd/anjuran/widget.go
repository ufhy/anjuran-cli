package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/generator"
	"github.com/ufhy/anjuran-cli/internal/recall"
	"github.com/ufhy/anjuran-cli/internal/tty"
	"github.com/ufhy/anjuran-cli/internal/ui"
)

// runWidget adalah mode interaktif yang dipanggil oleh integrasi shell.
//
// Protokolnya sengaja sederhana karena harus bisa diurai oleh zsh, bash,
// fish, dan PowerShell tanpa parser JSON:
//
//	baris pertama : "<status> <posisi-kursor> <tombol-sisa-heksadesimal>"
//	sisanya       : isi buffer yang baru, apa adanya
//
// Tombol sisa adalah tombol yang mengakhiri sesi tetapi bukan urusan dropdown —
// Ctrl-A, Home, panah kiri. Shell mengembalikannya ke antrean masukannya
// sendiri, sehingga tidak ada tombol yang tertelan. Dikirim sebagai
// heksadesimal karena isinya byte kendali yang tidak aman dilewatkan apa adanya
// di dalam satu baris teks.
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
	sel := fs.String("select", "first", "baris yang tersorot saat dibuka: first, last, atau none")
	alias := fs.String("alias", "", "pemekaran alias untuk kata pertama")
	trigger := fs.String("trigger", "manual", "asal pemicu: manual (Tab) atau auto (karakter pemicu)")
	specsDir := fs.String("specs", "", "direktori spec")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	byteCursor := toByteCursor(*line, *cursor, *unit)

	dirs, err := resolveSpecsDirs(*specsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	eng := engine.New(newRegistry(dirs, *specsDir)).InDir(generator.CurrentDir())

	start := mulaiDari(*sel, *line)

	// manual berarti "sisipkan kandidat tunggal tanpa bertanya". Menelusuri ke
	// dalam direktori memakai --select none justru untuk BERHENTI menyisipkan
	// sendiri, jadi di sana haknya dicabut walaupun pemicunya Tab.
	manual := *trigger != "auto" && start != ui.NoSelection

	// Perhitungan memakai bentuk yang sudah dimekarkan; hasilnya dipetakan
	// kembali ke baris asli sebelum diserahkan ke shell.
	ax := newAliasExpansion(*line, byteCursor, *alias)
	st, outcome, sisa, err := interact(eng,
		ui.State{Line: ax.Line(*line), Cursor: ax.Cursor(byteCursor)}, start, manual)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}

	switch outcome {
	case ui.Accepted:
		restored, c := ax.Restore(st.Line, st.Cursor)
		emit("ok", restored, c, *unit, sisa)
	case ui.NoCandidates:
		emit("none", *line, byteCursor, *unit, sisa)
	default:
		emit("cancel", *line, byteCursor, *unit, sisa)
	}
	return 0
}

// mulaiDari menentukan baris mana yang tersorot saat kotak dibuka.
//
// Baris perintah yang masih KOSONG dibuka tanpa ada yang tersorot. Yang
// ditawarkan di situ adalah seluruh isi PATH, dan yang tersorot akan menjadi
// nama pertama menurut abjad — sesuatu yang tidak pernah diketik siapa pun.
// Enter sesudahnya menyisipkannya, dan Enter berikutnya MENJALANKANNYA:
// menekan Tab lalu Enter pada baris kosong sudah cukup untuk menjalankan biner
// asing. Daftarnya tetap ditampilkan — itu memang gunanya Tab di sana —
// tetapi memilih harus menjadi tindakan yang disengaja.
func mulaiDari(sel, line string) int {
	switch sel {
	case "last":
		// Dibuka dengan panah ATAS: yang diminta memang baris terakhir.
		return -1
	case "none":
		return ui.NoSelection
	}
	if strings.TrimSpace(line) == "" {
		return ui.NoSelection
	}
	return 0
}

// EnvIkon memilih gaya ikon per baris.
const EnvIkon = "ANJURAN_IKON"

// modeIkon membaca gaya ikon dari lingkungan.
//
// Bawaannya bentuk geometris yang ada di hampir semua font. Glyph Nerd Font
// jauh lebih terbaca, tetapi harus diminta: font tidak bisa ditanya, dan glyph
// yang tidak dimiliki tergambar sebagai kotak kosong dengan lebar yang bisa
// meleset — cukup untuk mematahkan bingkai kotaknya.
func modeIkon() string {
	switch strings.ToLower(os.Getenv(EnvIkon)) {
	case "nerd":
		return ui.IkonNerd
	case "0", "no", "off", "false", "mati":
		return ui.IkonMati
	default:
		return ui.IkonAman
	}
}

// interact membuka terminal, menjalankan sesi, lalu memulihkan mode terminal.
func interact(eng *engine.Engine, st ui.State, start int, manual bool) (ui.State, ui.Outcome, ui.Leftover, error) {
	// Kandidat dihitung lebih dulu. Nol atau satu kandidat tidak memerlukan
	// gambar apa pun, jadi terminal tidak perlu dimasukkan ke mode raw.
	// Ingatan pilihan disimpan di disk: setiap penekanan tombol pemicu adalah
	// proses anjuran yang baru, jadi ingatan dalam memori tidak akan pernah terpakai.
	rec := recall.Open()
	defer rec.Save()

	pre, err := ui.Prepare(eng, st, newDynamic(), rec)
	if err != nil {
		return st, ui.Cancelled, nil, err
	}
	if out, outcome, done := pre.Manual(manual).Immediate(manual); done {
		return out, outcome, nil, nil
	}

	term, err := tty.Open()
	if err != nil {
		// Tanpa terminal interaktif tidak ada yang bisa digambar. Diperlakukan
		// sebagai "tidak ada kandidat" supaya shell jatuh ke completion bawaan.
		return st, ui.NoCandidates, nil, nil
	}
	defer term.Close()

	w, h := term.Size()
	gambar, tutupRekaman := rekam(term.Out())
	defer tutupRekaman()
	rend := ui.NewRenderer(gambar, w, h, simpleMode())
	rend.Ikon(modeIkon())

	sesi := pre.Session(term, rend).StartAt(start)
	st2, outcome, err := sesi.Run()
	return st2, outcome, sesi.Leftover(), err
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

// simpleMode mematikan warna dan sorotan pada terminal yang terbatas.
func simpleMode() bool {
	if os.Getenv("ANJURAN_SIMPLE") != "" {
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
func emit(status, line string, byteCursor int, unit string, sisa ui.Leftover) {
	fmt.Printf("%s %d %s\n%s", status, fromByteCursor(line, byteCursor, unit),
		hex.EncodeToString(sisa), line)
}
