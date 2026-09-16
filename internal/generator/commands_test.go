package generator

import (
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
