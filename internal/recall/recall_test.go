package recall

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func storeUji(t *testing.T) *Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return Open()
}

func TestMengingatPilihan(t *testing.T) {
	s := storeUji(t)
	key := Key("git", "c")

	if got := s.Preferred(key); got != "" {
		t.Errorf("belum ada ingatan, dapat %q", got)
	}

	s.Record(key, "commit")
	if got := s.Preferred(key); got != "commit" {
		t.Errorf("Preferred = %q, mau commit", got)
	}
}

// Ingatannya sengaja sempit: kebiasaan melekat pada perintah DAN awalan.
// "git c" yang biasanya berakhir di commit tidak boleh mengubah "docker c".
func TestIngatanTerpisahPerPerintahDanAwalan(t *testing.T) {
	s := storeUji(t)
	s.Record(Key("git", "c"), "commit")

	if got := s.Preferred(Key("docker", "c")); got != "" {
		t.Errorf("perintah lain ikut terpengaruh: %q", got)
	}
	if got := s.Preferred(Key("git", "ch")); got != "" {
		t.Errorf("awalan lain ikut terpengaruh: %q", got)
	}
}

func TestPilihanTerbaruMenggantikan(t *testing.T) {
	s := storeUji(t)
	key := Key("git", "c")
	s.Record(key, "commit")
	s.Record(key, "checkout")

	if got := s.Preferred(key); got != "checkout" {
		t.Errorf("Preferred = %q, mau pilihan terbaru", got)
	}
}

func TestTersimpanAntarProses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)

	// anjuran adalah proses baru setiap kali; ingatan harus melewati batas proses.
	a := Open()
	a.Record(Key("kubectl", "g"), "get")
	a.Save()

	b := Open()
	if got := b.Preferred(Key("kubectl", "g")); got != "get" {
		t.Errorf("Preferred = %q setelah dibaca ulang, mau get", got)
	}
}

func TestTanpaPerubahanTidakMenulis(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)

	s := Open()
	s.Save()

	if s.path != "" {
		if _, err := os.Stat(s.path); err == nil {
			t.Error("tidak ada yang berubah, seharusnya tidak menulis berkas")
		}
	}
}

// Yang paling lama tidak dipakai dibuang lebih dulu; berkasnya dibaca pada
// setiap penekanan tombol pemicu, jadi ia harus tetap kecil.
func TestIngatanDipangkas(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)

	s := Open()
	now := time.Now()
	s.Now = func() time.Time { return now }

	for i := 0; i < maxEntri+50; i++ {
		now = now.Add(time.Second)
		s.Record(Key("cmd", string(rune('a'+i%26))+string(rune(i))), "pilihan")
	}
	s.Save()

	b := Open()
	if len(b.data) > maxEntri {
		t.Errorf("tersimpan %d ingatan, mau paling banyak %d", len(b.data), maxEntri)
	}
}

func TestMasukanKosongDiabaikan(t *testing.T) {
	s := storeUji(t)
	s.Record("", "commit")
	s.Record(Key("git", "c"), "")
	if len(s.data) != 0 {
		t.Errorf("masukan kosong seharusnya diabaikan, dapat %v", s.data)
	}
	if got := Key("", "c"); got != "" {
		t.Errorf("Key tanpa perintah = %q, mau kosong", got)
	}
}

// Berkas yang rusak tidak boleh menjatuhkan apa pun: ingatan yang hilang hanya
// berarti urutan kembali ke bawaan.
func TestBerkasRusakDiabaikan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)

	s := Open()
	if s.path == "" {
		t.Skip("direktori cache tidak tersedia")
	}
	os.MkdirAll(filepath.Dir(s.path), 0o700)
	os.WriteFile(s.path, []byte("bukan json"), 0o600)

	b := Open()
	if got := b.Preferred(Key("git", "c")); got != "" {
		t.Errorf("mau kosong, dapat %q", got)
	}
	b.Record(Key("git", "c"), "commit")
	b.Save() // tidak boleh panik
}

// Store nil tetap aman dipakai: lupa sama sekali adalah keadaan yang sah.
func TestStoreNilAman(t *testing.T) {
	var s *Store
	if got := s.Preferred("x"); got != "" {
		t.Errorf("mau kosong, dapat %q", got)
	}
	s.Record("x", "y")
	s.Save()
}
