//go:build windows

package tty

import (
	"fmt"
	"os"
	"testing"
)

// Jalur konsol Windows tidak pernah dijalankan sampai hari ini.
//
// Seluruh pengujian sebelumnya berlangsung di Linux dan macOS, tempat
// openDevice cukup membuka /dev/tty. Di Windows ia membuka CONIN$ dan CONOUT$
// sebagai dua handle terpisah, lalu menyalakan ENABLE_VIRTUAL_TERMINAL —
// tanpanya seluruh escape ANSI tercetak sebagai teks mentah alih-alih
// menggambar apa pun.
//
// Uji ini dilewati bila memang tidak ada konsol, misalnya di runner CI yang
// menjalankan langkahnya lewat pipa. Yang tidak boleh terjadi adalah gagal
// diam-diam: bila konsolnya ADA tetapi tidak bisa dibuka, itu kabar penting.
func TestBukaKonsolWindows(t *testing.T) {
	term, err := Open()
	if err != nil {
		if _, e := os.Open("CONOUT$"); e != nil {
			t.Skip("tidak ada konsol di sesi ini:", err)
		}
		t.Fatalf("konsol ada tetapi tidak bisa dibuka: %v", err)
	}
	defer term.Close()

	w, h := term.Size()
	if w <= 0 || h <= 0 {
		t.Errorf("ukuran konsol = %dx%d, mau keduanya positif", w, h)
	}
	t.Logf("konsol terbuka, ukuran %dx%d", w, h)

	// Menulis lewat Out() memastikan handle keluarannya benar-benar bisa
	// dipakai, bukan sekadar terbuka.
	if _, err := fmt.Fprint(term.Out(), "\x1b[0m"); err != nil {
		t.Errorf("menulis ke konsol: %v", err)
	}
}
