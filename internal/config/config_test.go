package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tulisConfig menaruh sebuah berkas konfigurasi dan mengarahkan anjuran ke sana.
func tulisConfig(t *testing.T, isi string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), NamaBerkas)
	if err := os.WriteFile(p, []byte(isi), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANJURAN_CONFIG", p)
}

// Tanpa berkas apa pun, tidak ada yang berubah dan tidak ada yang mengeluh.
//
// Ini syarat, bukan kemurahan hati: alat yang menuntut konfigurasi sebelum
// bisa dipakai sudah gagal sebelum dicoba.
func TestTanpaBerkasBukanKesalahan(t *testing.T) {
	t.Setenv("ANJURAN_CONFIG", filepath.Join(t.TempDir(), "tidak-ada.toml"))

	c, err := Muat()
	if err != nil {
		t.Fatalf("berkas yang tidak ada dilaporkan sebagai kesalahan: %v", err)
	}
	if len(c.Nilai) != 0 {
		t.Errorf("menghasilkan nilai dari ketiadaan: %v", c.Nilai)
	}
}

// Lingkungan MENANG atas berkas. Itu seluruh urutan kewenangannya.
func TestLingkunganMenangAtasBerkas(t *testing.T) {
	tulisConfig(t, "specs = \"/dari-berkas\"\n")
	t.Setenv("ANJURAN_SPECS", "/dari-lingkungan")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	c.Terapkan()

	if got := os.Getenv("ANJURAN_SPECS"); got != "/dari-lingkungan" {
		t.Errorf("ANJURAN_SPECS = %q, berkas menimpa lingkungan", got)
	}
}

// Yang tidak dinyatakan di lingkungan diambil dari berkas.
func TestBerkasMengisiYangBelumAda(t *testing.T) {
	tulisConfig(t, "specs = \"/dari-berkas\"\n")
	os.Unsetenv("ANJURAN_SPECS")
	t.Cleanup(func() { os.Unsetenv("ANJURAN_SPECS") })

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	c.Terapkan()

	if got := os.Getenv("ANJURAN_SPECS"); got != "/dari-berkas" {
		t.Errorf("ANJURAN_SPECS = %q, mau /dari-berkas", got)
	}
}

// false harus berarti MATI, dan dua gaya variabel menuntut dua perlakuan.
//
// Sebagian besar bermakna "bila diisi": menyetel ANJURAN_SIMPLE menjadi kata
// "false" justru menyalakannya — kebalikan dari yang ditulis. Sebaliknya
// ANJURAN_AUTO menyala tanpa disetel, jadi nilainya harus benar-benar sampai.
func TestFalseMematikanKeduaGayaVariabel(t *testing.T) {
	tulisConfig(t, "simple = false\nauto = false\n")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}

	if v, ada := c.Nilai["ANJURAN_SIMPLE"]; ada {
		t.Errorf("ANJURAN_SIMPLE disetel %q; menyetelnya berarti MENYALAKANnya", v)
	}
	if got := c.Nilai["ANJURAN_AUTO"]; got != "0" {
		t.Errorf("ANJURAN_AUTO = %q, mau 0", got)
	}
}

// Daftar menjadi bentuk berpisah koma, yang sudah dipakai variabelnya.
func TestDaftarMenjadiPisahKoma(t *testing.T) {
	tulisConfig(t, "[generator]\nallow = [\"kubectl\", \"docker\"]\n")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Nilai["ANJURAN_GENERATOR_ALLOW"]; got != "kubectl,docker" {
		t.Errorf("allow = %q, mau kubectl,docker", got)
	}
}

// Tab ditulis sebagai \t, dan itu satu-satunya cara menuliskan tombol pemicu
// bawaan di dalam berkas teks.
func TestTabBisaDituliskan(t *testing.T) {
	tulisConfig(t, "key = \"\\t\"\n")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Nilai["ANJURAN_KEY"]; got != "\t" {
		t.Errorf("key = %q, mau sebuah tab", got)
	}
}

// Setelan yang salah ketik DITOLAK, tidak diabaikan diam-diam.
//
// Pengguna yang menulis "atuo = false" sudah menyatakan niat. Mengabaikannya
// tanpa sepatah kata berarti ia akan mengira setelannya berlaku, lalu mencari
// penyebabnya di tempat yang salah.
func TestSetelanTakDikenalDitolak(t *testing.T) {
	tulisConfig(t, "atuo = false\n")

	_, err := Muat()
	if err == nil {
		t.Fatal("setelan yang tidak dikenal diterima diam-diam")
	}
	if !strings.Contains(err.Error(), "atuo") {
		t.Errorf("pesan galat tidak menyebut setelan yang salah: %v", err)
	}
}

// Komentar dan baris kosong tidak mengganggu, dan komentar di ujung baris
// bukan bagian dari nilainya.
func TestKomentarDiabaikan(t *testing.T) {
	tulisConfig(t, "# catatan\n\nikon = nerd  # gaya ikon\n")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Nilai["ANJURAN_IKON"]; got != "nerd" {
		t.Errorf("ikon = %q, komentar ikut terbaca sebagai nilai", got)
	}
}

// Tanda pagar DI DALAM kutip adalah isi, bukan komentar.
func TestPagarDalamKutipBukanKomentar(t *testing.T) {
	tulisConfig(t, "specs = \"/jalur/dengan#pagar\"\n")

	c, err := Muat()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Nilai["ANJURAN_SPECS"]; got != "/jalur/dengan#pagar" {
		t.Errorf("specs = %q, pagar di dalam kutip ikut terpotong", got)
	}
}
