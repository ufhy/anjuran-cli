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
)

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

	nameW := 0
	for _, it := range items {
		if n := utf8.RuneCountInString(it.Name); n > nameW {
			nameW = n
		}
	}
	// Kolom nama tidak boleh melahap seluruh lebar terminal.
	if max := r.width / 2; nameW > max {
		nameW = max
	}

	lines := make([]string, 0, len(items)+1)
	for i, it := range items {
		lines = append(lines, r.composeRow(it, nameW, i == selected))
	}

	if hidden := total - len(items); hidden > 0 {
		note := fmt.Sprintf("  … %d lagi", hidden)
		if !r.simple {
			note = escDim + note + escReset
		}
		lines = append(lines, note)
	}
	return lines
}

func (r *Renderer) composeRow(it Item, nameW int, selected bool) string {
	marker := "  "
	if selected {
		marker = "❯ "
	}

	name := truncate(it.Name, nameW)
	pad := strings.Repeat(" ", max(0, nameW-utf8.RuneCountInString(name)))

	// Sisa lebar untuk deskripsi: total dikurangi marker, nama, dan dua spasi.
	descW := r.width - 2 - nameW - 2
	desc := ""
	if descW > 4 && it.Description != "" {
		desc = "  " + truncate(it.Description, descW)
	}

	plain := marker + name + pad + desc
	if r.simple {
		return plain
	}
	if selected {
		// Reverse video berlaku untuk seluruh baris, jadi sorotan per huruf
		// tidak dipasang sama sekali — bukan dipasang lalu dibuang.
		return escReverse + plain + escReset
	}

	// Satu escReset di ujung sudah membatalkan escDim maupun escCyan;
	// menutup keduanya secara terpisah hanya menambah byte yang dikirim.
	color := escCyan
	if it.Dangerous {
		color = escRed
	}
	row := color + marker + highlight(name, it.Highlight) + pad
	if desc != "" {
		row += escDim + desc
	}
	return row + escReset
}

// highlight menebalkan rune yang cocok dengan kueri.
func highlight(name string, positions []int) string {
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
			b.WriteString(escReset + escCyan)
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
