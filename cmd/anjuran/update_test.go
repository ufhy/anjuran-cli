package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/remote"
)

// Binary milik pengelola paket tidak boleh ditimpa dari sini: memperbaruinya
// di belakang punggung pengelola paket membuat kedua pihak berselisih tentang
// apa yang sebenarnya terpasang.
func TestDikelolaPengelolaPaket(t *testing.T) {
	tests := map[string]bool{
		"/usr/bin/anjuran": true,
		"/opt/homebrew/Cellar/anjuran/0.1/bin/anjuran": true,
		"/nix/store/abc-anjuran/bin/anjuran":           true,
		"/home/lab/.local/bin/anjuran":                 false,
		"/Users/a/.local/bin/anjuran":                  false,
		"/usr/local/bin/anjuran":                       false,
	}
	for path, mau := range tests {
		if got := dikelola(path); got != mau {
			t.Errorf("dikelola(%q) = %v, mau %v", path, got, mau)
		}
	}
}

// Tata letak diambil dari letak binary yang sedang berjalan, bukan ditebak.
// Pemasang menaruhnya di <dasar>/bin/anjuran dengan spec di
// <dasar>/share/anjuran, dan itulah yang harus ditimpa.
func TestPasangDiTempatMenimpaSpecLama(t *testing.T) {
	dasar := t.TempDir()
	bin := filepath.Join(dasar, "bin")
	share := filepath.Join(dasar, "share", "anjuran")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(share, "specs"), 0o755)
	exe := filepath.Join(bin, "anjuran")
	os.WriteFile(exe, []byte("lama"), 0o755)
	// Berkas yang sudah tidak ada di rilis baru harus HILANG, bukan tertinggal
	// dan tetap ditawarkan.
	os.WriteFile(filepath.Join(share, "specs", "usang.json"), []byte("{}"), 0o644)

	sumber := t.TempDir()
	os.WriteFile(filepath.Join(sumber, "anjuran"), []byte("baru"), 0o755)
	os.MkdirAll(filepath.Join(sumber, "specs"), 0o755)
	os.WriteFile(filepath.Join(sumber, "specs", "git.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(sumber, "extra"), 0o755)
	os.WriteFile(filepath.Join(sumber, "extra", "cd.json"), []byte("{}"), 0o644)

	src := remote.Source{
		Binary: filepath.Join(sumber, "anjuran"),
		Specs:  filepath.Join(sumber, "specs"),
		Extra:  filepath.Join(sumber, "extra"),
	}
	if err := pasangDiTempat(src, exe); err != nil {
		t.Fatal(err)
	}

	isi, _ := os.ReadFile(exe)
	if string(isi) != "baru" {
		t.Errorf("binary = %q, mau %q", isi, "baru")
	}
	fi, err := os.Stat(exe)
	if err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Error("binary kehilangan bit eksekusinya")
	}
	if _, err := os.Stat(filepath.Join(share, "specs", "usang.json")); err == nil {
		t.Error("spec lama tertinggal")
	}
	for _, mau := range []string{"specs/git.json", "extra/cd.json"} {
		if _, err := os.Stat(filepath.Join(share, filepath.FromSlash(mau))); err != nil {
			t.Errorf("%s tidak ikut terpasang", mau)
		}
	}
}

// Binary yang setengah tertulis tidak boleh pernah terlihat: penggantiannya
// lewat rename, yang atomik dalam satu filesystem.
func TestPasangDiTempatTidakMeninggalkanBerkasSementara(t *testing.T) {
	dasar := t.TempDir()
	bin := filepath.Join(dasar, "bin")
	os.MkdirAll(bin, 0o755)
	exe := filepath.Join(bin, "anjuran")
	os.WriteFile(exe, []byte("lama"), 0o755)

	sumber := t.TempDir()
	os.WriteFile(filepath.Join(sumber, "anjuran"), []byte("baru"), 0o755)

	if err := pasangDiTempat(remote.Source{Binary: filepath.Join(sumber, "anjuran")}, exe); err != nil {
		t.Fatal(err)
	}
	entri, _ := os.ReadDir(bin)
	for _, e := range entri {
		if strings.HasSuffix(e.Name(), ".baru") {
			t.Errorf("berkas sementara tertinggal: %s", e.Name())
		}
	}
}
