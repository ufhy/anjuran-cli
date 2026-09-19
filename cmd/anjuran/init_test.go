package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/config"
)

// configUji menaruh berkas konfigurasi dan mengarahkan anjuran ke sana.
func configUji(t *testing.T, isi string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(isi), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANJURAN_CONFIG", p)
}

// Setelan yang dibaca SKRIP SHELL harus ikut keluar bersama skripnya.
//
// Tombol pemicu dan mode otomatis diputuskan di dalam skrip integrasi, yang
// berjalan di shell pengguna. Lingkungan proses anjuran tidak pernah sampai
// ke sana, jadi tanpa ini berkas konfigurasi tidak berpengaruh apa pun atas
// keduanya — dan itu justru dua setelan yang paling sering diubah orang.
func TestSetelanShellIkutDibawaKeSkrip(t *testing.T) {
	configUji(t, "auto = false\nkey = \"\t\"\n")

	for _, sh := range []string{"zsh", "bash", "fish", "powershell"} {
		got := pembuka(sh)
		if !strings.Contains(got, "ANJURAN_AUTO") {
			t.Errorf("%s: mode otomatis dari berkas tidak dibawa:\n%s", sh, got)
		}
		if !strings.Contains(got, "ANJURAN_KEY") {
			t.Errorf("%s: tombol pemicu dari berkas tidak dibawa:\n%s", sh, got)
		}
	}
}

// Yang dinyatakan pengguna untuk SESI INI tetap menang di dalam shell juga.
//
// Karena itu bentuknya selalu "setel bila belum disetel", bukan penetapan
// langsung. Urutan kewenangan yang berlaku di dalam anjuran tidak boleh
// berhenti di batas prosesnya.
func TestSkripTidakMenimpaYangSudahDisetel(t *testing.T) {
	configUji(t, "auto = false\n")

	tanda := map[string]string{
		"zsh":        `${ANJURAN_AUTO=`,
		"bash":       `${ANJURAN_AUTO=`,
		"fish":       "set -q ANJURAN_AUTO",
		"powershell": "Test-Path env:ANJURAN_AUTO",
	}
	for sh, mau := range tanda {
		got := pembuka(sh)
		if !strings.Contains(got, mau) {
			t.Errorf("%s: tidak memakai bentuk bersyarat %q:\n%s", sh, mau, got)
		}
	}
	// Bentuk bertitik dua juga menimpa nilai KOSONG, dan kosong adalah cara
	// sah menyatakan "mati".
	if got := pembuka("bash"); strings.Contains(got, `${ANJURAN_AUTO:=`) {
		t.Errorf("bash memakai bentuk yang ikut menimpa nilai kosong:\n%s", got)
	}
}

// Setelan yang dibaca Go TIDAK ikut dibawa ke skrip.
//
// Ia sudah sampai lewat lingkungan proses anjuran sendiri. Mengekspornya lagi
// ke shell pengguna berarti mengotori lingkungannya dengan hal yang bukan
// urusannya.
func TestSetelanGoTidakDieksporKeShell(t *testing.T) {
	configUji(t, "specs = \"/di-mana-saja\"\nikon = nerd\n")

	got := pembuka("bash")
	if strings.Contains(got, "ANJURAN_SPECS") || strings.Contains(got, "ANJURAN_IKON") {
		t.Errorf("setelan yang dibaca Go ikut diekspor ke shell:\n%s", got)
	}
}

// Tanpa berkas, skripnya keluar persis seperti sebelum berkas konfigurasi ada.
func TestTanpaBerkasSkripTidakBerubah(t *testing.T) {
	t.Setenv("ANJURAN_CONFIG", filepath.Join(t.TempDir(), "tidak-ada.toml"))

	if got := pembuka("bash"); got != "" {
		t.Errorf("skrip mendapat tambahan tanpa berkas konfigurasi:\n%s", got)
	}
}

// Berkas contoh yang DICETAK anjuran harus bisa diurai anjuran.
//
// Contoh yang tidak bisa dibaca alatnya sendiri adalah jebakan: orang
// menyalinnya lebih dulu, lalu menghabiskan waktu mencari kesalahan di
// tempat yang salah. Diuji terhadap teks yang benar-benar dicetak, bukan
// terhadap tiruannya — tiruan hanya membuktikan bahwa tiruannya benar.
func TestContohYangDicetakBisaDiurai(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(contohConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANJURAN_CONFIG", p)

	cfg, err := config.Muat()
	if err != nil {
		t.Fatalf("contoh yang dicetak anjuran tidak bisa diurai anjuran: %v", err)
	}
	// Seluruh barisnya memang dikomentari, dan itu bagian dari maksudnya:
	// menyalin contohnya tidak boleh mengubah perilaku apa pun.
	if len(cfg.Nilai) != 0 {
		t.Errorf("contoh menyalakan sesuatu hanya karena disalin: %v", cfg.Nilai)
	}
}
