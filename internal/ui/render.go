package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Item adalah satu baris yang digambar di dropdown.
type Item struct {
	Name        string
	Description string
	Kind        string
	// Dangerous menandai kandidat yang merusak bila salah pilih, misalnya
	// --force pada git push. Ditandai warna agar terlihat sebelum dipilih.
	Dangerous bool
	// Highlight adalah indeks rune pada Name yang cocok dengan kueri.
	Highlight []int
}

// Escape sequence yang dipakai. Sengaja dikumpulkan di satu tempat agar
// mode sederhana bisa mengosongkan yang bersifat dekoratif.
const (
	escSaveCursor    = "\x1b7"
	escRestoreCursor = "\x1b8"
	escClearLine     = "\x1b[K"
	escReset         = "\x1b[0m"
	escDim           = "\x1b[2m"
	escBold          = "\x1b[1m"
	escReverse       = "\x1b[7m"
	escCyan          = "\x1b[36m"
	escYellow        = "\x1b[33m"
	escRed           = "\x1b[31m"
	escBlue          = "\x1b[34m"
	escGreen         = "\x1b[32m"
)

// Karakter bingkai. Bingkai membuat dropdown terbaca sebagai satu benda,
// bukan sebagai beberapa baris yang kebetulan tercetak.
//
// Biayanya kecil justru pada jalur yang paling kita jaga: garis bingkai tidak
// pernah berubah antar penekanan tombol, sehingga renderer diff hanya
// mengirimnya satu kali untuk seluruh sesi.
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
)

// minBoxWidth adalah lebar terminal terkecil yang masih layak diberi bingkai.
// Di bawah itu, dua kolom yang dimakan bingkai lebih berharga untuk teks.
const minBoxWidth = 44

// colorFor memilih warna menurut jenis kandidat, supaya daftar punya struktur
// yang terbaca sekilas: subcommand, opsi, dan nilai argumen tidak tercampur
// menjadi satu blok teks yang seragam.
func colorFor(kind string, dangerous bool) string {
	if dangerous {
		return escRed
	}
	switch kind {
	case "subcommand":
		return escCyan
	case "option":
		return escBlue
	default:
		return escGreen
	}
}

// Renderer menggambar dropdown tepat di bawah baris prompt.
//
// Setiap Render hanya menulis baris yang BERUBAH sejak render sebelumnya.
// Ini bukan optimasi mikro: pada sesi SSH dengan RTT tinggi, menggambar ulang
// seluruh kotak setiap ketikan terasa nyata sebagai lag.
type Renderer struct {
	w      io.Writer
	width  int
	height int
	// simple mematikan warna dan sorotan untuk terminal terbatas.
	simple bool

	// prev menyimpan baris yang sedang tampil di layar.
	prev []string
	// reserved adalah jumlah baris yang sudah kita amankan di bawah prompt.
	reserved int
}

// NewRenderer membuat renderer. width dan height adalah ukuran terminal.
func NewRenderer(w io.Writer, width, height int, simple bool) *Renderer {
	if width < 20 {
		width = 20
	}
	if height < 3 {
		height = 3
	}
	return &Renderer{w: w, width: width, height: height, simple: simple}
}

// MaxRows adalah jumlah baris kandidat yang muat, menyisakan ruang untuk
// baris prompt itu sendiri dan satu baris status.
func (r *Renderer) MaxRows() int {
	n := r.height - 2
	if n > 10 {
		n = 10
	}
	if n < 1 {
		n = 1
	}
	return n
}

// Render menggambar items dengan baris ke-selected disorot. total adalah
// jumlah kandidat sebenarnya, dipakai untuk menampilkan sisa yang tersembunyi.
func (r *Renderer) Render(items []Item, selected, total int) error {
	lines := r.compose(items, selected, total)

	// Pastikan ada ruang di bawah prompt SEBELUM posisi kursor disimpan.
	// Bila terminal ikut menggulung, penggulungan terjadi di sini, sehingga
	// posisi tersimpan nanti tidak pernah basi.
	if err := r.reserve(len(lines)); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(escSaveCursor)

	for i, line := range lines {
		if i < len(r.prev) && r.prev[i] == line {
			continue // baris tidak berubah: tidak ada byte yang dikirim
		}
		b.WriteString(escRestoreCursor)
		fmt.Fprintf(&b, "\x1b[%dB\r", i+1)
		b.WriteString(line)
		b.WriteString(escClearLine)
	}

	// Bersihkan sisa baris bila dropdown menyusut.
	for i := len(lines); i < len(r.prev); i++ {
		b.WriteString(escRestoreCursor)
		fmt.Fprintf(&b, "\x1b[%dB\r", i+1)
		b.WriteString(escClearLine)
	}

	b.WriteString(escRestoreCursor)
	r.prev = lines

	_, err := io.WriteString(r.w, b.String())
	return err
}

// reserve menjamin tersedia n baris kosong di bawah kursor.
func (r *Renderer) reserve(n int) error {
	if n <= r.reserved {
		return nil
	}
	need := n - r.reserved
	// Newline menggulung layar bila perlu; kursor lalu dikembalikan ke atas.
	s := strings.Repeat("\n", need) + fmt.Sprintf("\x1b[%dA", need)
	if _, err := io.WriteString(r.w, s); err != nil {
		return err
	}
	r.reserved = n
	return nil
}

// Clear menghapus dropdown dan melupakan state gambar.
func (r *Renderer) Clear() error {
	if r.reserved == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(escSaveCursor)
	for i := 0; i < r.reserved; i++ {
		b.WriteString(escRestoreCursor)
		fmt.Fprintf(&b, "\x1b[%dB\r", i+1)
		b.WriteString(escClearLine)
	}
	b.WriteString(escRestoreCursor)

	r.prev = nil
	r.reserved = 0
	_, err := io.WriteString(r.w, b.String())
	return err
}

// EchoRune menampilkan karakter yang baru diketik pengguna di baris prompt.
//
// Dropdown tidak memiliki baris prompt — shell yang memilikinya — sehingga
// gema karakter dilakukan sendiri agar tidak perlu menggambar ulang prompt.
func (r *Renderer) EchoRune(c rune) error {
	_, err := io.WriteString(r.w, string(c))
	return err
}

// EchoBackspace menghapus satu karakter di baris prompt.
func (r *Renderer) EchoBackspace() error {
	_, err := io.WriteString(r.w, "\b \b")
	return err
}

// compose membentuk seluruh baris dropdown sebagai string siap kirim.
func (r *Renderer) compose(items []Item, selected, total int) []string {
	if len(items) == 0 {
		return nil
	}

	nameW, descW, inner := r.columns(items)

	// Mode sederhana tidak pernah berbingkai: terminal yang memerlukannya
	// biasanya juga tidak menggambar karakter kotak dengan benar.
	if r.simple || r.width < minBoxWidth {
		lines := make([]string, 0, len(items)+1)
		for i, it := range items {
			lines = append(lines, r.plainRow(it, nameW, descW, i == selected))
		}
		if hidden := total - len(items); hidden > 0 {
			lines = append(lines, fmt.Sprintf("  … %d lagi", hidden))
		}
		return lines
	}

	lines := make([]string, 0, len(items)+2)
	lines = append(lines, escDim+boxTopLeft+strings.Repeat(boxHorizontal, inner)+boxTopRight+escReset)
	for i, it := range items {
		lines = append(lines, r.boxedRow(it, nameW, descW, inner, i == selected))
	}
	lines = append(lines, r.bottomBorder(inner, selected, total))
	return lines
}

// columns menghitung lebar kolom nama, kolom keterangan, dan lebar dalam kotak.
func (r *Renderer) columns(items []Item) (nameW, descW, inner int) {
	for _, it := range items {
		if n := utf8.RuneCountInString(it.Name); n > nameW {
			nameW = n
		}
	}
	// Kolom nama tidak boleh melahap seluruh lebar terminal.
	if max := r.width / 2; nameW > max {
		nameW = max
	}

	for _, it := range items {
		if n := utf8.RuneCountInString(it.Description); n > descW {
			descW = n
		}
	}

	// marker + nama + jarak + keterangan, ditambah satu spasi di tiap tepi.
	inner = 2 + nameW + 2 + descW + 2
	if maxInner := r.width - 2; inner > maxInner {
		inner = maxInner
		if d := inner - (2 + nameW + 2 + 2); d >= 0 {
			descW = d
		} else {
			descW = 0
		}
	}
	return nameW, descW, inner
}

// rowText menyusun isi satu baris tanpa warna, sudah rata kolom.
func rowText(it Item, nameW, descW int, selected bool) (marker, name, pad, desc string) {
	marker = "  "
	if selected {
		marker = "❯ "
	}
	name = truncate(it.Name, nameW)
	pad = strings.Repeat(" ", max(0, nameW-utf8.RuneCountInString(name)))
	if descW > 3 && it.Description != "" {
		desc = "  " + truncate(it.Description, descW-2)
	}
	return marker, name, pad, desc
}

// plainRow menggambar satu baris tanpa bingkai dan tanpa warna.
func (r *Renderer) plainRow(it Item, nameW, descW int, selected bool) string {
	marker, name, pad, desc := rowText(it, nameW, descW, selected)
	return marker + name + pad + desc
}

// boxedRow menggambar satu baris di dalam bingkai.
func (r *Renderer) boxedRow(it Item, nameW, descW, inner int, selected bool) string {
	marker, name, pad, desc := rowText(it, nameW, descW, selected)

	plain := " " + marker + name + pad + desc
	fill := strings.Repeat(" ", max(0, inner-utf8.RuneCountInString(plain)))
	border := escDim + boxVertical + escReset

	// Baris terpilih disorot SELEBAR isi kotak, bukan selebar teksnya. Inilah
	// yang membuatnya terbaca sebagai pilihan aktif pada sebuah menu alih-alih
	// sebagai sepotong teks yang kebetulan berwarna terbalik.
	if selected {
		return border + escReverse + plain + fill + escReset + border
	}

	color := colorFor(it.Kind, it.Dangerous)
	body := color + " " + marker + highlight(name, it.Highlight, color) + pad + escReset
	if desc != "" {
		body += escDim + desc + escReset
	}
	return border + body + fill + border
}

// bottomBorder menyisipkan penghitung posisi ke dalam garis bawah, sehingga
// tidak memakan satu baris layar sendiri.
func (r *Renderer) bottomBorder(inner, selected, total int) string {
	label := fmt.Sprintf(" %d/%d ", selected+1, total)
	if n := utf8.RuneCountInString(label); n+2 > inner {
		label = ""
	}
	left := 2
	right := inner - left - utf8.RuneCountInString(label)
	if right < 0 {
		right = 0
	}
	return escDim + boxBottomLeft + strings.Repeat(boxHorizontal, left) +
		label + strings.Repeat(boxHorizontal, right) + boxBottomRight + escReset
}

// highlight menebalkan rune yang cocok dengan kueri, lalu mengembalikan
// warna dasarnya agar sisa nama tidak ikut berubah.
func highlight(name string, positions []int, base string) string {
	if len(positions) == 0 {
		return name
	}
	set := make(map[int]bool, len(positions))
	for _, p := range positions {
		set[p] = true
	}

	var b strings.Builder
	for i, c := range []rune(name) {
		if set[i] {
			b.WriteString(escBold + escYellow)
			b.WriteRune(c)
			b.WriteString(escReset + base)
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// stripStyles membuang escape SGR dari sebuah string.
func stripStyles(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// truncate memotong berdasarkan rune, bukan byte, lalu menambahkan elipsis.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(rs[:w-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
