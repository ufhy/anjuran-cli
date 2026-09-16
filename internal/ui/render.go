package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/ufhy/anjuran-cli/internal/engine"
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
	// Hint adalah petunjuk tombol yang digambar di tepi kanan baris, hanya
	// pada baris yang sedang terpilih.
	//
	// Tanpa ini tidak ada yang memberi tahu bahwa sebuah folder bisa dimasuki
	// DAN bisa dipakai apa adanya; keduanya tombol yang berbeda, dan menebak
	// tombol adalah pekerjaan yang tidak seharusnya dibebankan ke pengguna.
	Hint string
}

// Escape sequence yang dipakai. Sengaja dikumpulkan di satu tempat agar
// mode sederhana bisa mengosongkan yang bersifat dekoratif.
const (
	// escIndex menurunkan kursor satu baris tanpa mengubah kolomnya, dan
	// menggulung layar bila sudah di baris terakhir.
	escIndex         = "\x1bD"
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
	boxTeeDown     = "┬"
	boxTeeUp       = "┴"
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
	// echoed adalah jumlah KOLOM yang kita gemakan sendiri ke baris prompt.
	//
	// Dicatat karena shell tidak tahu tentangnya: ia masih mengira barisnya
	// seperti saat sesi dibuka. Bila gema itu dibiarkan, penggambaran ulang
	// milik shell dimulai dari kolom yang salah dan barisnya tampak berganda.
	echoed int
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

// Render menggambar items dengan baris ke-selected disorot.
//
// selected adalah indeks di dalam items — yaitu di dalam jendela yang terlihat —
// sedangkan position adalah nomor urut sebenarnya di seluruh daftar, dimulai
// dari 1. Keduanya dipisah karena daftar bisa tergulung: yang disorot adalah
// baris di layar, tetapi yang dilaporkan harus posisi sesungguhnya. Menyamakan
// keduanya membuat penghitung salah pada setiap daftar yang lebih panjang dari
// layar. position bernilai 0 berarti belum ada yang terpilih.
func (r *Renderer) Render(items []Item, selected, position, total int) error {
	lines := r.compose(items, selected, position, total)

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
	// escIndex dipakai, BUKAN "\n".
	//
	// Di luar mode raw, terminal mengubah "\n" menjadi CR+LF, sehingga kursor
	// ikut melompat ke kolom 0. Posisi yang disimpan sesudahnya lalu salah,
	// dan baris prompt tertimpa saat kursor dipulihkan. Mode Tab tidak
	// terkena karena di sana terminal sedang raw; mode gambar-saja terkena,
	// karena ia sengaja tidak mengubah keadaan terminal sama sekali.
	s := strings.Repeat(escIndex, need) + fmt.Sprintf("\x1b[%dA", need)
	if _, err := io.WriteString(r.w, s); err != nil {
		return err
	}
	r.reserved = n
	return nil
}

// Adopt memberi tahu renderer bahwa n baris di bawah kursor SUDAH terpakai dan
// sudah diamankan oleh proses lain.
//
// Ini yang membuat penggambaran ulang antar proses mungkin. Saat dropdown
// muncul otomatis, setiap ketikan menjalankan proses anjuran yang baru dan tidak
// mewarisi apa pun; zsh yang menyimpan jumlah barisnya lalu menyerahkannya
// kembali ke sini. Isi baris lama sengaja diisi penanda yang tidak mungkin
// cocok, supaya seluruhnya digambar ulang dan sisanya dibersihkan.
func (r *Renderer) Adopt(n int) {
	if n <= 0 {
		return
	}
	r.reserved = n
	r.prev = make([]string, n)
	for i := range r.prev {
		r.prev[i] = "\x00"
	}
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
	r.echoed += runeWidth(c)
	_, err := io.WriteString(r.w, string(c))
	return err
}

// EchoBackspace menghapus satu karakter di baris prompt.
func (r *Renderer) EchoBackspace() error {
	if r.echoed > 0 {
		r.echoed--
	}
	_, err := io.WriteString(r.w, "\b \b")
	return err
}

// UnEcho menghapus seluruh karakter yang kita gemakan sendiri.
//
// Dipanggil sebelum sesi berakhir, supaya baris yang terlihat kembali persis
// seperti yang terakhir digambar shell. Setelah itu shell menggambar ulang
// dari buffer barunya dan hasilnya tepat — tanpa ini, gema kita dan gambar
// shell saling menumpuk.
func (r *Renderer) UnEcho() error {
	if r.echoed <= 0 {
		return nil
	}
	n := r.echoed
	r.echoed = 0
	_, err := io.WriteString(r.w, strings.Repeat("\b \b", n))
	return err
}

// compose membentuk seluruh baris dropdown sebagai string siap kirim.
func (r *Renderer) compose(items []Item, selected, position, total int) []string {
	if len(items) == 0 {
		return nil
	}

	c := r.columns(items)

	// Mode sederhana tidak pernah berbingkai: terminal yang memerlukannya
	// biasanya juga tidak menggambar karakter kotak dengan benar.
	if r.simple || r.width < minBoxWidth {
		lines := make([]string, 0, len(items)+1)
		for i, it := range items {
			lines = append(lines, r.plainRow(it, c, i == selected))
		}
		if hidden := total - len(items); hidden > 0 {
			lines = append(lines, fmt.Sprintf("  … %d lagi", hidden))
		}
		return lines
	}

	lines := make([]string, 0, len(items)+2)
	lines = append(lines, escDim+c.topBorder()+escReset)
	for i, it := range items {
		lines = append(lines, r.boxedRow(it, c, i == selected))
	}
	lines = append(lines, escDim+c.bottomBorder(position, total)+escReset)
	return lines
}

// layout menyimpan lebar kedua kolom.
//
// Nama dan keterangan dipisah garis tegak, bukan sekadar dijajarkan dengan
// spasi: pada daftar yang panjang nama-namanya, mata butuh satu titik tetap
// untuk tahu di mana keterangan dimulai.
type layout struct {
	nameW int
	descW int
	// left dan right adalah lebar isi masing-masing sel, sudah termasuk
	// satu spasi di tiap tepinya.
	left, right int
	// split bernilai false bila tidak ada keterangan sama sekali; di situ
	// garis pemisah hanya akan memenggal kotak tanpa memisahkan apa pun.
	split bool
}

func (c layout) topBorder() string {
	if !c.split {
		return boxTopLeft + strings.Repeat(boxHorizontal, c.left) + boxTopRight
	}
	return boxTopLeft + strings.Repeat(boxHorizontal, c.left) +
		boxTeeDown + strings.Repeat(boxHorizontal, c.right) + boxTopRight
}

// bottomBorder menyisipkan penghitung ke dalam garis bawah, di ujung kanan,
// sehingga tidak memakan satu baris layar untuk dirinya sendiri.
func (c layout) bottomBorder(position, total int) string {
	label := fmt.Sprintf(" %d ", total)
	if position > 0 {
		label = fmt.Sprintf(" %d/%d ", position, total)
	}

	tail := c.right
	if !c.split {
		tail = c.left
	}
	n := textWidth(label)
	if n+2 > tail {
		label = ""
		n = 0
	}
	right := strings.Repeat(boxHorizontal, tail-n-1) + label + boxHorizontal

	if !c.split {
		return boxBottomLeft + right + boxBottomRight
	}
	return boxBottomLeft + strings.Repeat(boxHorizontal, c.left) +
		boxTeeUp + right + boxBottomRight
}

// columns menghitung lebar kedua kolom agar muat di lebar terminal.
func (r *Renderer) columns(items []Item) layout {
	var c layout
	hintW := 0
	for _, it := range items {
		if n := textWidth(it.Name); n > c.nameW {
			c.nameW = n
		}
		if n := textWidth(it.Description); n > c.descW {
			c.descW = n
		}
		if n := textWidth(it.Hint); n > hintW {
			hintW = n
		}
	}
	// Kolom nama tidak boleh melahap seluruh lebar terminal.
	if max := r.width / 2; c.nameW > max {
		c.nameW = max
	}

	c.split = c.descW > 0

	// Ruang untuk petunjuk tombol pada baris terpilih.
	//
	// Tanpa cadangan ini nama terpanjang mengisi kolomnya sampai habis, dan
	// petunjuknya tidak pernah muat — justru pada daftar folder, satu-satunya
	// tempat petunjuk itu diperlukan.
	if hintW > 0 {
		if c.split {
			c.descW += hintW + 1
		} else {
			c.nameW += hintW + 1
		}
	}

	// Sel kiri: spasi + penanda + nama + spasi.
	c.left = 1 + 2 + c.nameW + 1

	if !c.split {
		if max := r.width - 2; c.left > max {
			c.left = max
		}
		return c
	}

	// Sel kanan: spasi + keterangan + spasi, sisa lebar setelah sel kiri
	// dan garis pemisahnya.
	avail := r.width - 2 - c.left - 1 - 2
	if avail < 4 {
		// Tidak cukup ruang untuk kolom keterangan yang berguna.
		c.split = false
		c.descW = 0
		return c
	}
	if c.descW > avail {
		c.descW = avail
	}
	c.right = c.descW + 2
	return c
}

// rowCells menyusun isi kedua sel tanpa warna, sudah rata kolom.
func rowCells(it Item, c layout, selected bool) (left, right string) {
	marker := "  "
	if selected {
		marker = "❯ "
	}
	name := truncateWidth(it.Name, c.nameW)
	left = " " + marker + name + strings.Repeat(" ", max(0, c.nameW-textWidth(name))) + " "

	if !c.split {
		if selected {
			left = tempelPetunjuk(left, it.Hint)
		}
		return left, ""
	}
	desc := truncateWidth(it.Description, c.descW)
	right = " " + desc + strings.Repeat(" ", max(0, c.descW-textWidth(desc))) + " "
	if selected {
		right = tempelPetunjuk(right, it.Hint)
	}
	return left, right
}

// tempelPetunjuk menaruh petunjuk tombol di tepi kanan sel, MENGGANTIKAN ruang
// kosongnya.
//
// Lebar selnya tidak boleh berubah: sel inilah yang menentukan di mana bingkai
// kanan digambar, dan satu kolom saja meleset sudah cukup membuat kotaknya
// terlihat patah. Bila ruang kosongnya tidak cukup, petunjuknya yang dibuang —
// bukan barisnya yang dibiarkan melebar.
func tempelPetunjuk(sel, hint string) string {
	if hint == "" {
		return sel
	}
	isi := strings.TrimRight(sel, " ")
	kosong := textWidth(sel) - textWidth(isi)
	n := textWidth(hint)
	if kosong < n+2 {
		return sel
	}
	return isi + strings.Repeat(" ", kosong-n-1) + hint + " "
}

// plainRow menggambar satu baris tanpa bingkai dan tanpa warna.
func (r *Renderer) plainRow(it Item, c layout, selected bool) string {
	left, right := rowCells(it, c, selected)
	if right == "" {
		return strings.TrimRight(left, " ")
	}
	return left + " " + strings.TrimRight(right, " ")
}

// boxedRow menggambar satu baris di dalam bingkai berkolom.
func (r *Renderer) boxedRow(it Item, c layout, selected bool) string {
	left, right := rowCells(it, c, selected)
	edge := escDim + boxVertical + escReset

	// Baris terpilih disorot SELEBAR isi kotak, garis pemisah ikut di
	// dalamnya. Sorotan selebar teks saja terbaca sebagai potongan teks
	// berwarna terbalik, bukan sebagai pilihan aktif pada sebuah menu.
	if selected {
		body := left
		if c.split {
			body += boxVertical + right
		}
		return edge + escReverse + body + escReset + edge
	}

	color := colorFor(it.Kind, it.Dangerous)
	marker := left[:3] // spasi + penanda dua karakter
	rest := left[3:]
	name := strings.TrimRight(rest, " ")
	pad := rest[len(name):]

	row := edge + color + marker + highlight(name, it.Highlight, color) + escReset + pad
	if c.split {
		row += escDim + boxVertical + right + escReset
	}
	return row + edge
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Show menggambar daftar kandidat tanpa ada yang terpilih, lalu mengembalikan
// jumlah baris yang terpakai.
//
// Dipakai oleh mode gambar-saja, di mana belum ada pilihan aktif: pengguna
// masih mengetik, dan menyorot salah satu baris akan menyarankan bahwa Enter
// akan memilihnya — padahal Enter di situ menjalankan perintah.
//
// Kandidat tunggal TETAP digambar. Justru di situlah pengguna paling dekat
// dengan jawabannya, dan menghilangkan kotaknya terasa seperti fiturnya mati.
// Yang disembunyikan hanya satu keadaan: kandidat tunggal yang teksnya sudah
// persis sama dengan yang diketik, karena di situ memang tidak ada lagi yang
// bisa ditawarkan.
func (r *Renderer) Show(cands []engine.Candidate, prefix string) int {
	if len(cands) == 0 ||
		(len(cands) == 1 && strings.EqualFold(cands[0].Name, prefix)) {
		r.Clear()
		return 0
	}

	rows := r.MaxRows()
	if len(cands) < rows {
		rows = len(cands)
	}

	items := make([]Item, rows)
	for i := 0; i < rows; i++ {
		items[i] = Item{
			Name:        cands[i].Label(),
			Description: cands[i].Description,
			Kind:        string(cands[i].Kind),
			Dangerous:   cands[i].Dangerous,
		}
	}

	// selected di luar rentang berarti tidak ada baris yang tersorot.
	lines := r.compose(items, -1, 0, len(cands))
	if err := r.Render(items, -1, 0, len(cands)); err != nil {
		return 0
	}
	return len(lines)
}
