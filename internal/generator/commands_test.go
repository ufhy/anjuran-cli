package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// pathSementara mengisi PATH dengan direktori sekali pakai, lalu mengosongkan
// hasil pemindaian yang tersimpan supaya pemindaian benar-benar diulang.
func pathSementara(t *testing.T, berkas map[string]os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	for nama, mode := range berkas {
		if err := os.WriteFile(filepath.Join(dir, nama), nil, mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	// Hasil pemindaian sengaja disimpan seumur proses; di dalam pengujian
	// beberapa PATH hidup dalam satu proses yang sama.
	sekaliPath = onceBaru()
	daftarPath = nil
	return dir
}

func TestCommandsMenyaringDenganAwalan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bit eksekusi tidak berlaku di Windows")
	}
	pathSementara(t, map[string]os.FileMode{
		"kubectl":     0o755,
		"kubeseal":    0o755,
		"git":         0o755,
		"catatan.txt": 0o644, // tidak bisa dijalankan
	})

	got := Commands("kube")
	if !punya(got, "kubectl") || !punya(got, "kubeseal") {
		t.Errorf("mau kubectl dan kubeseal, dapat %v", got)
	}
	if punya(got, "git") {
		t.Errorf("git tidak berawalan kube, dapat %v", got)
	}
}

// Berkas tanpa bit eksekusi bukan perintah; menawarkannya hanya menghasilkan
// baris yang gagal dijalankan.
func TestCommandsMelewatiYangTidakBisaDijalankan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bit eksekusi tidak berlaku di Windows")
	}
	pathSementara(t, map[string]os.FileMode{"catatan.txt": 0o644})

	if got := Commands("cat"); punya(got, "catatan.txt") {
		t.Errorf("berkas tanpa bit eksekusi ditawarkan: %v", got)
	}
}

// Awalan yang memuat pemisah jalur adalah jalur ke sebuah berkas, bukan nama
// perintah; melengkapinya dari PATH tidak masuk akal.
func TestCommandsMenolakJalur(t *testing.T) {
	pathSementara(t, map[string]os.FileMode{"git": 0o755})

	for _, p := range []string{"./gi", "/usr/bin/gi", `dir\gi`} {
		if got := Commands(p); len(got) != 0 {
			t.Errorf("Commands(%q) = %v, mau kosong", p, got)
		}
	}
}

// Awalan kosong mengembalikan seluruh isi PATH, dibatasi maxPerintah.
func TestCommandsAwalanKosong(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bit eksekusi tidak berlaku di Windows")
	}
	pathSementara(t, map[string]os.FileMode{"git": 0o755, "ls": 0o755})

	if got := Commands(""); len(got) != 2 {
		t.Errorf("Commands(\"\") = %v, mau 2 entri", got)
	}
}

// Huruf besar-kecil tidak menentukan: pengguna mengetik apa adanya.
func TestCommandsTidakPeduliHurufBesar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bit eksekusi tidak berlaku di Windows")
	}
	pathSementara(t, map[string]os.FileMode{"Docker": 0o755})

	if got := Commands("doc"); !punya(got, "Docker") {
		t.Errorf("mau Docker, dapat %v", got)
	}
}

// Direktori PATH yang lebih besar daripada batas berkas biasa tetap terbaca
// SELURUHNYA.
//
// os.ReadDir mengembalikan entrinya terurut, sehingga batas apa pun memotong
// ekor abjadnya. Pada /usr/bin runner Ubuntu — lebih dari dua ribu biner —
// batas lama membuat `zsh` tidak pernah muncul sebagai kandidat, sementara
// perintah berhuruf awal tetap muncul. Yang hilang bukan panjang daftarnya,
// melainkan perintahnya sendiri, tanpa satu pun tanda.
func TestPerintahDiUjungAbjadTetapTerbaca(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bit eksekusi tidak berlaku di Windows")
	}
	dir := t.TempDir()
	// Lebih banyak daripada batas berkas biasa (2000), dengan yang dicari
	// berada SESUDAHNYA dalam urutan abjad.
	for i := 0; i < 2100; i++ {
		tulisBiner(t, dir, fmt.Sprintf("aaa-%04d", i))
	}
	tulisBiner(t, dir, "zzz-paling-akhir")

	t.Setenv("PATH", dir)
	sekaliPath = onceBaru()

	got := Commands("zzz")
	if len(got) != 1 || got[0] != "zzz-paling-akhir" {
		t.Errorf("Commands(\"zzz\") = %v, mau [zzz-paling-akhir]", got)
	}
}

func tulisBiner(t *testing.T, dir, nama string) {
	t.Helper()
	p := filepath.Join(dir, nama)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
