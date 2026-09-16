package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Escape sequence harus terbaca sebagai teks, bukan dijalankan oleh terminal
// yang kebetulan membuka berkas rekamannya.
func TestRekamMenulisEscapeSebagaiTeks(t *testing.T) {
	var b strings.Builder
	w := &sebagaiTeks{&b}

	n, err := w.Write([]byte("a\x1b[2mb\r\n\bc\x07\x00"))
	if err != nil {
		t.Fatal(err)
	}
	// Panjang yang dilaporkan adalah panjang MASUKAN, bukan hasil
	// penulisannya; io.MultiWriter memeriksanya dan akan menganggap penulisan
	// gagal bila keduanya berbeda.
	if n != len("a\x1b[2mb\r\n\bc\x07\x00") {
		t.Errorf("n = %d, mau %d", n, len("a\x1b[2mb\r\n\bc\x07\x00"))
	}
	for _, want := range []string{"<ESC>", "<CR>", "<LF>", "<BS>", "<BEL>", "<00>"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("rekaman tidak memuat %q: %q", want, b.String())
		}
	}
}

// Tanpa ANJURAN_LOG, tujuan gambar dikembalikan apa adanya: perekaman tidak
// boleh membebani jalur biasa.
func TestRekamMatiTanpaEnv(t *testing.T) {
	t.Setenv(EnvRekam, "")
	var b strings.Builder
	if got := rekam(&b); got != (&b) {
		t.Error("tanpa ANJURAN_LOG, penulis harus dikembalikan apa adanya")
	}
}

func TestRekamMenulisKeBerkas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.log")
	t.Setenv(EnvRekam, path)

	var b strings.Builder
	w := rekam(&b)
	if _, err := w.Write([]byte("halo\x1b[K")); err != nil {
		t.Fatal(err)
	}

	isi, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(isi), "halo<ESC>[K") {
		t.Errorf("berkas rekaman = %q", isi)
	}
	// Terminal tetap menerima byte aslinya, bukan bentuk terbacanya.
	if b.String() != "halo\x1b[K" {
		t.Errorf("terminal menerima %q, mau byte asli", b.String())
	}
}

// Berkas rekaman yang tidak bisa dibuka tidak boleh menghalangi pekerjaan.
func TestRekamGagalTidakMenghalangi(t *testing.T) {
	t.Setenv(EnvRekam, filepath.Join(t.TempDir(), "tidak-ada", "a.log"))
	var b strings.Builder
	if got := rekam(&b); got != (&b) {
		t.Error("kegagalan membuka berkas harus mengembalikan penulis apa adanya")
	}
}
