package ui

import (
	"bytes"
	"strings"
	"testing"
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

	if err := r.Render(sample(), 0, 3); err != nil {
		t.Fatal(err)
	}
	first := buf.Len()
	if first == 0 {
		t.Fatal("render pertama seharusnya menulis sesuatu")
	}

	buf.Reset()
	if err := r.Render(sample(), 0, 3); err != nil {
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
	if err := r.Render(sample(), 0, 3); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := r.Render(sample(), 1, 3); err != nil {
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
	if err := r.Render(sample(), 0, 3); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := r.Render(sample()[:1], 0, 1); err != nil {
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
	if err := r.Render(sample(), 0, 3); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Newline pemesan ruang harus mendahului penyimpanan kursor, supaya
	// posisi tersimpan tidak basi bila terminal ikut menggulung.
	nl := strings.Index(out, "\n\n\n")
	save := strings.Index(out, escSaveCursor)
	if nl < 0 || save < 0 {
		t.Fatalf("mau pemesanan ruang dan simpan kursor, dapat %q", out)
	}
	if nl > save {
		t.Error("ruang harus dipesan SEBELUM posisi kursor disimpan")
	}
}

func TestClearMengembalikanKeKondisiAwal(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf, 80, 24, true)
	if err := r.Render(sample(), 0, 3); err != nil {
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
	if err := r.Render(sample(), 0, 3); err != nil {
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
	lines := r.compose(items, 0, 1)
	for _, l := range lines {
		if n := len([]rune(stripStyles(l))); n > 30 {
			t.Errorf("baris %d rune melebihi lebar 30: %q", n, l)
		}
	}
}

func TestSisaKandidatDilaporkan(t *testing.T) {
	r := NewRenderer(nil, 80, 24, true)
	lines := r.compose(sample(), 0, 12)
	last := lines[len(lines)-1]
	if !strings.Contains(last, "9 lagi") {
		t.Errorf("mau catatan sisa kandidat, dapat %q", last)
	}
}

func TestModeSederhanaTanpaWarna(t *testing.T) {
	r := NewRenderer(nil, 80, 24, true)
	for _, l := range r.compose(sample(), 0, 3) {
		if strings.Contains(l, "\x1b[") {
			t.Errorf("mode sederhana tidak boleh mengandung escape warna: %q", l)
		}
	}
}

func TestModeBerwarnaMenyorotHurufCocok(t *testing.T) {
	r := NewRenderer(nil, 80, 24, false)
	items := []Item{{Name: "commit", Highlight: []int{0, 1, 2}}}
	// Baris tidak terpilih mempertahankan sorotan; baris terpilih memakai
	// reverse video sehingga sorotannya sengaja dilepas.
	lines := r.compose(items, 1, 1)
	if !strings.Contains(lines[0], escBold) {
		t.Errorf("mau sorotan tebal pada huruf yang cocok, dapat %q", lines[0])
	}
}

func TestMaxRowsMengikutiTinggiTerminal(t *testing.T) {
	if got := NewRenderer(nil, 80, 24, true).MaxRows(); got != 10 {
		t.Errorf("terminal tinggi = %d baris, mau dibatasi 10", got)
	}
	if got := NewRenderer(nil, 80, 6, true).MaxRows(); got != 4 {
		t.Errorf("terminal pendek = %d baris, mau 4", got)
	}
	if got := NewRenderer(nil, 80, 1, true).MaxRows(); got < 1 {
		t.Errorf("terminal sangat pendek tetap harus menyisakan 1 baris, dapat %d", got)
	}
}
