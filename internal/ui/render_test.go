package ui

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ufhy/anjuran-cli/internal/engine"
)

func sample() []Item {
	return []Item{
		{Name: "add", Description: "Tambahkan file"},
		{Name: "commit", Description: "Rekam perubahan"},
		{Name: "push", Description: "Kirim ke remote"},
	}
}

// Inilah alasan renderer dibuat diff-based: pada SSH ber-RTT tinggi, byte yang
// tidak perlu dikirim adalah lag yang tidak perlu dirasakan.
func TestRenderUlangIdentikTidakMengirimApaPun(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)

	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	first := buf.Len()
	if first == 0 {
		t.Fatal("render pertama seharusnya menulis sesuatu")
	}

	buf.Reset()
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}

	// Yang tersisa hanya pasangan simpan/pulihkan kursor, bukan isi baris.
	out := buf.String()
	if strings.Contains(out, "commit") {
		t.Errorf("baris identik tidak boleh dikirim ulang, dapat %q", out)
	}
	if buf.Len() > 8 {
		t.Errorf("render identik menulis %d byte, seharusnya mendekati nol", buf.Len())
	}
}

func TestPindahPilihanHanyaMenggambarUlangDuaBaris(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := r.Render(sample(), 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Baris 0 kehilangan penanda dan baris 1 mendapatkannya; baris 2 diam.
	if !strings.Contains(out, "add") || !strings.Contains(out, "commit") {
		t.Errorf("dua baris yang berubah harus digambar ulang, dapat %q", out)
	}
	if strings.Contains(out, "push") {
		t.Errorf("baris ketiga tidak berubah, tidak boleh dikirim; dapat %q", out)
	}
}

func TestDropdownMenyusutMembersihkanSisa(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := r.Render(sample()[:1], 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	// Dua baris sisa harus dihapus, ditandai escape clear-line.
	if n := strings.Count(buf.String(), escClearLine); n < 2 {
		t.Errorf("mau minimal 2 pembersihan baris, dapat %d: %q", n, buf.String())
	}
}

func TestRuangDipesanSebelumMenggambar(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Pemesan ruang harus mendahului penyimpanan kursor, supaya posisi
	// tersimpan tidak basi bila terminal ikut menggulung.
	nl := strings.Index(out, escIndex+escIndex+escIndex)
	save := strings.Index(out, escSaveCursor)
	if nl < 0 || save < 0 {
		t.Fatalf("mau pemesanan ruang dan simpan kursor, dapat %q", out)
	}
	// "\n" tidak boleh dipakai: di luar mode raw ia menjadi CR+LF dan
	// memindahkan kursor ke kolom 0.
	if strings.Contains(out, "\n") {
		t.Errorf("pemesanan ruang tidak boleh memakai newline mentah: %q", out)
	}
	if nl > save {
		t.Error("ruang harus dipesan SEBELUM posisi kursor disimpan")
	}
}

func TestClearMengembalikanKeKondisiAwal(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.Clear(); err != nil {
		t.Fatal(err)
	}
	if r.reserved != 0 || r.prev != nil {
		t.Error("Clear harus melupakan seluruh state gambar")
	}

	// Setelah Clear, render berikutnya harus menggambar penuh lagi.
	buf.Reset()
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "commit") {
		t.Error("render setelah Clear harus menggambar ulang seluruh isi")
	}
}

func TestBarisDipotongSesuaiLebar(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 30, 24, true)
	items := []Item{{
		Name:        "sebuah-nama-subcommand-yang-sangat-panjang",
		Description: "deskripsi yang juga panjang sekali sampai tidak muat",
	}}
	lines := r.compose(items, 0, 1, 1)
	for _, l := range lines {
		if n := len([]rune(stripStyles(l))); n > 30 {
			t.Errorf("baris %d rune melebihi lebar 30: %q", n, l)
		}
	}
}

func TestSisaKandidatDilaporkan(t *testing.T) {
	r := NewRenderer(nil, 80, 24, true)
	lines := r.compose(sample(), 0, 1, 12)
	last := lines[len(lines)-1]
	if !strings.Contains(last, "9 lagi") {
		t.Errorf("mau catatan sisa kandidat, dapat %q", last)
	}
}

func TestModeSederhanaTanpaWarna(t *testing.T) {
	r := NewRenderer(nil, 80, 24, true)
	for _, l := range r.compose(sample(), 0, 1, 3) {
		if strings.Contains(l, "\x1b[") {
			t.Errorf("mode sederhana tidak boleh mengandung escape warna: %q", l)
		}
	}
}

func TestModeBerwarnaMenyorotHurufCocok(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{{Name: "commit", Kind: "subcommand", Highlight: []int{0, 1, 2}}}
	// Baris pertama adalah bingkai atas; baris kedua barulah isinya.
	// selected = 1 berarti tidak ada baris yang terpilih di sini, sehingga
	// sorotan per huruf tetap terpasang.
	lines := r.compose(items, 1, 2, 1)
	if !strings.Contains(lines[1], escBold) {
		t.Errorf("mau sorotan tebal pada huruf yang cocok, dapat %q", lines[1])
	}
}

// Bingkai membuat dropdown terbaca sebagai satu benda. Garisnya tidak pernah
// berubah antar penekanan tombol, jadi renderer diff hanya mengirimnya sekali.
func TestBingkaiDigambar(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	lines := r.compose(sample(), 0, 1, 3)

	if len(lines) != len(sample())+2 {
		t.Fatalf("mau %d baris isi ditambah dua garis bingkai, dapat %d", len(sample()), len(lines))
	}
	if !strings.Contains(lines[0], boxTopLeft) || !strings.Contains(lines[0], boxTopRight) {
		t.Errorf("baris pertama harus bingkai atas, dapat %q", lines[0])
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, boxBottomLeft) || !strings.Contains(last, boxBottomRight) {
		t.Errorf("baris terakhir harus bingkai bawah, dapat %q", last)
	}
	// Dua garis tepi, ditambah satu pemisah kolom di tengah.
	for _, l := range lines[1 : len(lines)-1] {
		if n := strings.Count(stripStyles(l), boxVertical); n != 3 {
			t.Errorf("baris isi harus punya dua tepi dan satu pemisah, dapat %d: %q", n, stripStyles(l))
		}
	}
	if !strings.Contains(stripStyles(lines[0]), boxTeeDown) {
		t.Errorf("bingkai atas harus memuat pertemuan kolom, dapat %q", stripStyles(lines[0]))
	}
	if !strings.Contains(stripStyles(lines[len(lines)-1]), boxTeeUp) {
		t.Errorf("bingkai bawah harus memuat pertemuan kolom")
	}
}

func TestSemuaBarisSamaLebar(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{
		{Name: "a", Description: "pendek"},
		{Name: "nama-yang-jauh-lebih-panjang", Description: "keterangan yang juga panjang sekali"},
		{Name: "bb"},
	}
	lines := r.compose(items, 0, 1, 3)

	want := utf8.RuneCountInString(stripStyles(lines[0]))
	for i, l := range lines {
		if got := utf8.RuneCountInString(stripStyles(l)); got != want {
			t.Errorf("baris %d selebar %d, mau %d: %q", i, got, want, stripStyles(l))
		}
	}
}

// Baris terpilih harus tersorot SELEBAR isi kotak. Sorotan selebar teks saja
// terbaca sebagai potongan teks berwarna, bukan sebagai pilihan aktif.
func TestBarisTerpilihTersorotPenuh(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{
		{Name: "a", Description: "pendek"},
		{Name: "nama-panjang", Description: "keterangan panjang sekali"},
	}
	lines := r.compose(items, 0, 1, 2)

	baris := lines[1] // baris pertama isi, yang terpilih
	if !strings.Contains(baris, escReverse) {
		t.Fatalf("baris terpilih harus memakai reverse video: %q", baris)
	}

	// Ambil teks di antara reverse dan reset; panjangnya harus mengisi kotak.
	mulai := strings.Index(baris, escReverse) + len(escReverse)
	selesai := strings.Index(baris[mulai:], escReset)
	isi := baris[mulai : mulai+selesai]

	innerWidth := utf8.RuneCountInString(stripStyles(lines[0])) - 2
	if got := utf8.RuneCountInString(isi); got != innerWidth {
		t.Errorf("sorotan selebar %d, mau selebar isi kotak %d: %q", got, innerWidth, isi)
	}
}

func TestPenghitungPosisiDiGarisBawah(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	lines := r.compose(sample(), 1, 2, 13)
	last := stripStyles(lines[len(lines)-1])
	if !strings.Contains(last, "2/13") {
		t.Errorf("garis bawah harus memuat posisi, dapat %q", last)
	}
}

// Terminal sempit lebih butuh kolomnya untuk teks daripada untuk bingkai.
func TestTerminalSempitTanpaBingkai(t *testing.T) {
	r := NewRenderer(nil, 30, 24, false)
	for _, l := range r.compose(sample(), 0, 1, 3) {
		if strings.Contains(l, boxVertical) || strings.Contains(l, boxTopLeft) {
			t.Errorf("terminal sempit tidak boleh berbingkai: %q", l)
		}
	}
}

func TestWarnaBerbedaPerJenis(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{
		{Name: "commit", Kind: "subcommand"},
		{Name: "--force", Kind: "option"},
		{Name: "main", Kind: "arg"},
	}
	// selected = 3 berarti tidak ada yang terpilih, sehingga setiap baris
	// memakai warna jenisnya sendiri.
	lines := r.compose(items, 3, 4, 3)
	warna := map[string]string{
		"commit":  escCyan,
		"--force": escBlue,
		"main":    escGreen,
	}
	for i, it := range items {
		if !strings.Contains(lines[i+1], warna[it.Name]) {
			t.Errorf("%s (%s) tidak memakai warna jenisnya: %q", it.Name, it.Kind, lines[i+1])
		}
	}
}

func TestKandidatBerbahayaBerwarnaMerah(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{{Name: "--force", Kind: "option", Dangerous: true}}
	lines := r.compose(items, 1, 2, 1)
	if !strings.Contains(lines[1], escRed) {
		t.Errorf("kandidat berbahaya harus merah, dapat %q", lines[1])
	}
}

func TestMaxRowsMengikutiTinggiTerminal(t *testing.T) {
	if got := NewRenderer(nil, 80, 24, true).MaxRows(); got != 10 {
		t.Errorf("terminal tinggi = %d baris, mau dibatasi 10", got)
	}
	// Tiga baris disisakan, bukan dua: baris perintah itu sendiri, dan dua
	// baris bingkai kotak.
	if got := NewRenderer(nil, 80, 6, true).MaxRows(); got != 3 {
		t.Errorf("terminal pendek = %d baris, mau 3", got)
	}
	if got := NewRenderer(nil, 80, 1, true).MaxRows(); got < 1 {
		t.Errorf("terminal sangat pendek tetap harus menyisakan 1 baris, dapat %d", got)
	}
}

// Kotak tidak boleh menuntut ruang sebanyak tinggi layar.
//
// Bila ia menuntutnya, pemesanan ruang menggulung layar sampai baris perintah
// terdorong keluar — dan `ESC [ nA` sesudahnya mentok di baris nol alih-alih
// mengikuti isinya. Yang terlihat pengguna adalah prompt beserta awal
// perintahnya lenyap, padahal perintahnya tetap berjalan dengan benar.
func TestKotakTidakPernahSetinggiLayar(t *testing.T) {
	for _, tinggi := range []int{4, 6, 8, 10, 14, 24, 60} {
		r := NewRenderer(nil, 80, tinggi, false)
		// Dua baris bingkai di atas baris kandidat.
		if tingi := r.MaxRows() + 2; tingi > tinggi-1 {
			t.Errorf("tinggi layar %d: kotak memakai %d baris, maksimal %d",
				tinggi, tingi, tinggi-1)
		}
	}
}

// Adopt memungkinkan proses baru melanjutkan gambar proses sebelumnya. Tanpa
// itu, dropdown yang muncul sambil mengetik tidak mungkin: setiap ketikan
// menjalankan anjuran yang baru dan tidak mewarisi apa pun.
func TestAdoptMenggambarUlangSemuanya(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	r.Adopt(5)

	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Tidak boleh ada pemesanan ruang: lima baris sudah diamankan proses lain.
	if strings.Contains(out, escIndex) {
		t.Errorf("baris yang sudah diamankan tidak boleh dipesan ulang: %q", out)
	}
	// Seluruh baris digambar ulang, karena isi lama tidak diketahui.
	for _, want := range []string{"add", "commit", "push"} {
		if !strings.Contains(out, want) {
			t.Errorf("mau %q digambar ulang", want)
		}
	}
	// Dua baris sisa dari gambar sebelumnya harus dibersihkan.
	if n := strings.Count(out, escClearLine); n < 5 {
		t.Errorf("mau minimal 5 pembersihan baris, dapat %d", n)
	}
}

func TestAdoptNolTidakBerpengaruh(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	r.Adopt(0)
	if err := r.Render(sample(), 0, 1, 3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), escIndex) {
		t.Error("tanpa baris yang diadopsi, ruang tetap harus dipesan")
	}
}

// Show dipakai mode gambar-saja: belum ada pilihan aktif, karena pengguna
// masih mengetik dan Enter di situ menjalankan perintah.
func TestShowTanpaBarisTerpilih(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, false)

	cands := []engine.Candidate{
		{Name: "add", Kind: engine.KindSubcommand},
		{Name: "commit", Kind: engine.KindSubcommand},
		{Name: "push", Kind: engine.KindSubcommand},
	}
	n := r.Show(cands, "")
	if n != len(cands)+2 {
		t.Errorf("baris terpakai = %d, mau %d isi ditambah dua bingkai", n, len(cands))
	}
	if strings.Contains(buf.String(), escReverse) {
		t.Error("tidak boleh ada baris tersorot saat pengguna masih mengetik")
	}
	if !strings.Contains(stripStyles(buf.String()), " 3 ") {
		t.Errorf("garis bawah harus memuat jumlah kandidat: %q", stripStyles(buf.String()))
	}
}

// Kandidat tunggal justru saat pengguna paling dekat dengan jawabannya.
// Menghilangkan kotaknya di situ terasa seperti fiturnya mati.
func TestShowMenggambarKandidatTunggal(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, false)
	n := r.Show([]engine.Candidate{{Name: "add"}}, "ad")
	if n == 0 {
		t.Fatal("kandidat tunggal harus tetap digambar")
	}
	if !strings.Contains(buf.String(), "add") {
		t.Error("isinya harus tergambar")
	}
}

// Satu-satunya yang disembunyikan: kandidat tunggal yang sudah diketik penuh,
// karena di situ memang tidak ada lagi yang bisa ditawarkan.
func TestShowMenyembunyikanYangSudahDiketikPenuh(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, false)
	if n := r.Show([]engine.Candidate{{Name: "add"}}, "add"); n != 0 {
		t.Errorf("baris terpakai = %d, mau 0", n)
	}
	if n := r.Show([]engine.Candidate{{Name: "add"}}, "ADD"); n != 0 {
		t.Errorf("perbandingan harus mengabaikan besar-kecil huruf, dapat %d baris", n)
	}
}

func TestShowMembersihkanSaatKosong(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, false)
	r.Adopt(6)
	if n := r.Show(nil, ""); n != 0 {
		t.Errorf("baris terpakai = %d, mau 0", n)
	}
	if !strings.Contains(buf.String(), escClearLine) {
		t.Error("kotak lama harus dibersihkan saat tidak ada kandidat")
	}
}

// Penghitung harus melaporkan posisi SEBENARNYA, bukan posisi di dalam jendela
// yang terlihat. Menyamakan keduanya membuat angkanya salah pada setiap daftar
// yang lebih panjang dari layar — dan daftar seperti itu justru yang biasa.
func TestPenghitungMemakaiPosisiSebenarnya(t *testing.T) {
	items := sample() // tiga baris terlihat
	r := NewRenderer(nil, 80, 24, false)

	// Baris ke-2 di layar, tetapi kandidat ke-48 dari 56.
	lines := r.compose(items, 1, 48, 56)
	last := stripStyles(lines[len(lines)-1])
	if !strings.Contains(last, "48/56") {
		t.Errorf("garis bawah = %q, mau memuat 48/56", last)
	}

	// Yang tersorot tetap baris kedua di layar.
	if !strings.Contains(lines[2], escReverse) {
		t.Error("baris kedua di layar seharusnya yang tersorot")
	}
}

// Tanpa keterangan sama sekali, garis pemisah hanya akan memenggal kotak tanpa
// memisahkan apa pun.
func TestTanpaKeteranganJadiKolomTunggal(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{{Name: "fitur-a"}, {Name: "fitur-b"}}
	lines := r.compose(items, 0, 1, 2)

	for i, l := range lines {
		if strings.Contains(stripStyles(l), boxTeeDown) || strings.Contains(stripStyles(l), boxTeeUp) {
			t.Errorf("baris %d tidak boleh punya pemisah kolom: %q", i, stripStyles(l))
		}
	}
	if n := strings.Count(stripStyles(lines[1]), boxVertical); n != 2 {
		t.Errorf("baris isi hanya boleh punya dua tepi, dapat %d", n)
	}
}

// Kolom keterangan yang terlalu sempit tidak berguna; lebih baik kembali ke
// satu kolom daripada menampilkan potongan dua huruf.
func TestKolomKeteranganTerlaluSempitDibuang(t *testing.T) {
	items := []Item{{
		Name:        "nama-subcommand-yang-panjang",
		Description: "keterangan",
	}}

	// Pada lebar wajar, kolom keterangan masih berguna dan dipertahankan.
	if c := NewRenderer(nil, 72, 24, false).columns(items); !c.split {
		t.Error("kolom keterangan seharusnya muat di lebar 72")
	}

	// Pada terminal yang sangat sempit, sisanya tidak cukup untuk apa pun.
	if c := NewRenderer(nil, 20, 24, false).columns(items); c.split {
		t.Error("kolom keterangan yang tidak muat seharusnya dibuang")
	}
}

// Garis pemisah ikut tersorot, supaya baris terpilih tetap terbaca sebagai
// satu blok yang utuh.
func TestPemisahIkutTersorot(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	lines := r.compose(sample(), 0, 1, 3)

	baris := lines[1]
	mulai := strings.Index(baris, escReverse)
	if mulai < 0 {
		t.Fatal("baris terpilih harus memakai reverse video")
	}
	selesai := strings.Index(baris[mulai:], escReset)
	isi := baris[mulai+len(escReverse) : mulai+selesai]
	if !strings.Contains(isi, boxVertical) {
		t.Errorf("pemisah kolom harus berada di dalam sorotan: %q", isi)
	}
}

// Setiap baris kotak harus sama lebarnya DI LAYAR, bukan sama jumlah rune-nya.
//
// Huruf CJK dan emoji memakan dua kolom. Menghitungnya sebagai satu membuat
// bingkai patah begitu ada satu nama berkas berbahasa Jepang atau satu emoji
// di dalam keterangan — dan nama seperti itu bukan hal langka.
func TestSemuaBarisSamaLebarDiLayar(t *testing.T) {
	kasus := [][]Item{
		{{Name: "biasa", Description: "keterangan biasa"}},
		{
			{Name: "biasa", Description: "keterangan"},
			{Name: "日本語コマンド", Description: "perintah bahasa Jepang"},
			{Name: "emoji", Description: "roket \U0001F680 di tengah"},
			{Name: "campur\U0001F525an", Description: "api di tengah nama"},
		},
		{
			{Name: "한글", Description: "Hangul"},
			{Name: "中文", Description: "中文说明"},
			{Name: "é", Description: "huruf bertanda gabung"},
		},
	}

	for i, items := range kasus {
		for _, width := range []int{50, 60, 80, 120} {
			r := NewRenderer(nil, width, 24, false)
			lines := r.compose(items, 0, 1, len(items))

			want := textWidth(stripStyles(lines[0]))
			for j, l := range lines {
				if got := textWidth(stripStyles(l)); got != want {
					t.Errorf("kasus %d lebar %d: baris %d selebar %d kolom, mau %d: %q",
						i, width, j, got, want, stripStyles(l))
				}
			}
			if want > width {
				t.Errorf("kasus %d: kotak selebar %d kolom melebihi terminal %d", i, want, width)
			}
		}
	}
}

// Kandidat yang ditawarkan UI sendiri tidak berasal dari penyaringan, sehingga
// tidak punya posisi cocok.
//
// Sempat membuat anjuran panik di tengah menggambar. Akibatnya jauh lebih
// buruk daripada kotak yang tidak muncul: prosesnya mati tanpa keluaran, dan
// shell menganggapnya "tidak ada jawaban" lalu menyisipkan hasil completion-nya
// sendiri — perubahan baris yang tidak pernah dipilih siapa pun.
func TestItemsTanpaMatchTidakPanik(t *testing.T) {
	rs := []ranked{{cand: engine.Candidate{Name: "\u23ce", Kind: engine.KindBerhenti}}}
	got := items(rs, IkonAman)
	if len(got) != 1 || got[0].Name != "\u23ce" {
		t.Fatalf("items = %+v", got)
	}
	if got[0].Hint != "" {
		t.Errorf("Hint = %q, mau kosong — ikonnya sudah menjadi namanya", got[0].Hint)
	}
}
