package remote

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// zipUji menyusun arsip zip berisi entri yang diminta.
func zipUji(t *testing.T, isi map[string]string, mode os.FileMode) string {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nama, teks := range isi {
		h := &zip.FileHeader{Name: nama, Method: zip.Deflate}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(teks)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(t.TempDir(), "rilis.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Rilis Windows dikemas zip. Tanpa pembongkarnya, `anjuran up` ke host
// Windows berhenti sebelum mulai — padahal seluruh langkah lainnya sudah
// bekerja di sana.
func TestUnzipMembongkarArsipRilis(t *testing.T) {
	arc := zipUji(t, map[string]string{
		"anjuran.exe":    "biner",
		"specs/git.json": "{}",
		"extra/cd.json":  "{}",
		"README.md":      "baca",
	}, 0o755)

	dir := t.TempDir()
	if err := unzip(arc, dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"anjuran.exe", "specs/git.json", "extra/cd.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s tidak dibongkar: %v", rel, err)
		}
	}
}

// Entri arsip bisa disusun pihak lain, jadi path yang menunjuk keluar
// direktori tujuan tidak boleh pernah ditulis.
func TestUnzipMenolakPathKeluar(t *testing.T) {
	arc := zipUji(t, map[string]string{
		"../keluar.txt": "jahat",
		"aman.txt":      "baik",
	}, 0o644)

	dir := t.TempDir()
	if err := unzip(arc, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "keluar.txt")); err == nil {
		t.Error("entri ../ berhasil menulis di luar direktori tujuan")
	}
	if _, err := os.Stat(filepath.Join(dir, "aman.txt")); err != nil {
		t.Error("entri yang aman seharusnya tetap dibongkar")
	}
}

// Berkas rilis Windows dibongkar utuh lewat jalur yang sama dengan Unix.
//
// Bit eksekusi dipertahankan bila arsipnya menyimpannya: zip buatan
// goreleaser di Linux menyimpannya, dan binary yang kehilangan bitnya tidak
// bisa dijalankan di host Unix mana pun.
func TestUnzipMempertahankanBitEksekusi(t *testing.T) {
	arc := zipUji(t, map[string]string{"anjuran": "biner"}, 0o755)

	dir := t.TempDir()
	if err := unzip(arc, dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "anjuran"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows tidak punya bit eksekusi sama sekali; memeriksanya di sana
	// bukan menguji apa pun.
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode = %v, bit eksekusinya hilang", fi.Mode().Perm())
	}
}
