package ui

import (
	"os"
	"testing"
	"time"

	"github.com/ufhy/anjuran-cli/internal/tty"
)

// Terminal palsu mengirim tombol tanpa jeda sama sekali, sehingga setiap
// ketikan di dalam pengujian tampak seperti tempelan.
//
// Waktunya karena itu dipalsukan untuk SELURUH paket: tiap pembacaan maju satu
// detik, jauh di atas ambang tempelan. Uji yang memang ingin menguji tempelan
// memasang percepatannya sendiri lewat denganWaktuCepat.
func TestMain(m *testing.M) {
	palsu := time.Unix(0, 0)
	waktuSekarang = func() time.Time {
		palsu = palsu.Add(time.Second)
		return palsu
	}
	os.Exit(m.Run())
}

// denganWaktuCepat membuat setiap tombol datang lebih rapat daripada ambang
// tempelan, meniru teks yang ditempel.
func denganWaktuCepat(t *testing.T) {
	t.Helper()
	asli := waktuSekarang
	palsu := time.Unix(0, 0)
	waktuSekarang = func() time.Time {
		palsu = palsu.Add(jedaTempelan / 5)
		return palsu
	}
	t.Cleanup(func() { waktuSekarang = asli })
}

func rune_(r rune) tty.Key { return tty.Key{Type: tty.KeyRune, Rune: r} }

// Menempel satu baris perintah tidak boleh berakhir dengan kotak yang menelan
// sisa tempelannya.
//
// Karakter pertama yang mengakhiri kata membuka kotak, dan tanpa penjagaan ini
// seluruh sisa tempelan masuk ke sana sebagai penyaring — pengguna menekan Esc
// berkali-kali hanya untuk menempel satu perintah.
func TestTempelanMenutupKotak(t *testing.T) {
	denganWaktuCepat(t)

	term := &fakeTerm{keys: []tty.Key{rune_('c'), rune_('o'), rune_('m'), rune_('m')}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})

	st, out, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out != Accepted {
		t.Errorf("outcome = %v, mau Accepted", out)
	}

	// Rune PERTAMA tetap diketik: ia belum punya pembanding waktu, dan
	// menolaknya berarti menelan karakter yang memang diketik pengguna.
	if st.Line != "git c" {
		t.Errorf("Line = %q, mau %q — karakter yang sudah diketik harus tersimpan", st.Line, "git c")
	}
	// Sisa tempelannya tidak ikut termakan: ia mendarat di shell sesudah sesi
	// ditutup. Dua tombol terbaca — yang pertama diketik, yang kedua mengenali
	// tempelannya.
	if term.i != 2 {
		t.Errorf("sesi membaca %d tombol; seharusnya berhenti di tombol kedua", term.i)
	}
}

// Mengetik dengan kecepatan manusia TIDAK boleh dianggap tempelan — itu
// justru cara menyaring daftar.
func TestKetikanManusiaBukanTempelan(t *testing.T) {
	term := &fakeTerm{keys: []tty.Key{rune_('c'), rune_('o'), rune_('m'),
		{Type: tty.KeyEnter}}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})

	st, out, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if st.Line == "git c" {
		t.Error("ketikan manusia disalahartikan sebagai tempelan")
	}
}

// Tombol kendali boleh datang beruntun: menahan panah bawah menghasilkan
// rentetan yang wajar, dan menutup kotak di situ merusak pemakaian paling
// biasa.
func TestTombolKendaliBeruntunBukanTempelan(t *testing.T) {
	denganWaktuCepat(t)

	term := &fakeTerm{keys: []tty.Key{
		{Type: tty.KeyDown}, {Type: tty.KeyDown}, {Type: tty.KeyDown},
		{Type: tty.KeyEnter},
	}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})

	_, out, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out != Accepted {
		t.Errorf("outcome = %v, mau Accepted — panah beruntun bukan tempelan", out)
	}
}
