package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uf-cli/uf/internal/spec"
)

// Nama berkas berspasi yang belum dikutip sudah dipecah shell menjadi beberapa
// kata sebelum uf melihatnya. Prefix-nya disatukan kembali, supaya yang
// dilengkapi adalah namanya yang utuh — begitulah orang mengetiknya.
func engineDiDir(t *testing.T) (*Engine, string) {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"folder dengan spasi", "berkas"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"catatan.txt", "folder dengan spasi/isi.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return New(spec.NewRegistry("../testdata/specs")).InDir(dir), dir
}

func TestPrefixDisatukanMelintasiSpasi(t *testing.T) {
	e, _ := engineDiDir(t)

	for _, u := range []struct {
		nama       string
		line       string
		mauPrefix  string
		mauReplace int
	}{
		{"di tengah nama", "gzip folder de", "folder de", 5},
		{"tepat setelah spasi", "gzip folder ", "folder ", 5},
		{"sudah melewati garis miring", "gzip folder dengan spasi/i", "folder dengan spasi/i", 5},
	} {
		t.Run(u.nama, func(t *testing.T) {
			res, err := e.Complete(u.line, len(u.line))
			if err != nil {
				t.Fatal(err)
			}
			if res.Prefix != u.mauPrefix {
				t.Errorf("Prefix = %q, mau %q", res.Prefix, u.mauPrefix)
			}
			if res.ReplaceStart != u.mauReplace {
				t.Errorf("ReplaceStart = %d, mau %d", res.ReplaceStart, u.mauReplace)
			}
		})
	}
}

// Dua argumen yang memang terpisah tidak boleh ikut disatukan. Tanpa syarat
// "harus cocok dengan sesuatu di disk", "gzip berkas catatan" akan berubah
// menjadi satu nama yang tidak pernah ada, dan kandidatnya hilang semua.
func TestDuaArgumenTerpisahTidakDisatukan(t *testing.T) {
	e, _ := engineDiDir(t)

	res, err := e.Complete("gzip berkas catatan", len("gzip berkas catatan"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Prefix != "catatan" {
		t.Errorf("Prefix = %q, mau %q", res.Prefix, "catatan")
	}
}

// Yang sudah dikutip tidak disatukan lagi: kutipnya sudah menyatakan batas
// namanya, dan menggabungnya justru merusak.
func TestTokenTerkutipTidakDisatukan(t *testing.T) {
	e, _ := engineDiDir(t)

	line := `gzip berkas 'folder de`
	res, err := e.Complete(line, len(line))
	if err != nil {
		t.Fatal(err)
	}
	if res.Prefix != "folder de" {
		t.Errorf("Prefix = %q, mau %q", res.Prefix, "folder de")
	}
}

// Penggabungan hanya sah di posisi yang memang meminta nama berkas. Posisi
// subcommand tidak pernah digabung, walaupun teks gabungannya kebetulan cocok
// dengan sebuah folder di disk.
func TestPenggabunganTidakDiPosisiSubcommand(t *testing.T) {
	e, _ := engineDiDir(t)

	line := "git folder de"
	res, err := e.Complete(line, len(line))
	if err != nil {
		t.Fatal(err)
	}
	if res.Prefix != "de" {
		t.Errorf("Prefix = %q, mau %q", res.Prefix, "de")
	}
}

// Opsi yang sedang diketik tidak pernah digabung dengan kata sebelumnya: tanda
// minus menyatakan dengan jelas bahwa ini bukan nama berkas.
func TestOpsiTidakDisatukan(t *testing.T) {
	e, _ := engineDiDir(t)

	line := "gzip folder -d"
	res, err := e.Complete(line, len(line))
	if err != nil {
		t.Fatal(err)
	}
	if res.Prefix != "-d" {
		t.Errorf("Prefix = %q, mau %q", res.Prefix, "-d")
	}
}

// IsDir tetap mengenali folder yang namanya dikutip; tanpa itu folder berspasi
// kehilangan seluruh perlakuan folder — tidak bisa ditelusuri, tanpa petunjuk
// tombol.
func TestIsDirMengenaliNamaTerkutip(t *testing.T) {
	for _, u := range []struct {
		c   Candidate
		mau bool
	}{
		{Candidate{Name: "proyek/", Insert: "proyek/"}, true},
		{Candidate{Name: "folder dengan spasi/", Insert: `'folder dengan spasi/'`}, true},
		{Candidate{Name: "catatan.txt", Insert: "catatan.txt"}, false},
		{Candidate{Name: "berkas dengan spasi.txt", Insert: `'berkas dengan spasi.txt'`}, false},
	} {
		if got := u.c.IsDir(); got != u.mau {
			t.Errorf("IsDir(%q) = %v, mau %v", u.c.Insert, got, u.mau)
		}
	}
}
