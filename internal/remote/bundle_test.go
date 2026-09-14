package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleIsi(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "uf")
	os.WriteFile(bin, []byte("BINARY"), 0o755)

	specs := filepath.Join(dir, "specs")
	os.MkdirAll(filepath.Join(specs, "aws"), 0o755)
	os.WriteFile(filepath.Join(specs, "git.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(specs, "aws", "s3.json"), []byte("{}"), 0o644)

	var buf bytes.Buffer
	n, err := Bundle(bin, specs, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Errorf("jumlah byte dilaporkan %d, sebenarnya %d", n, buf.Len())
	}

	isi := map[string]int64{}
	mode := map[string]int64{}
	zr, _ := gzip.NewReader(&buf)
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		isi[h.Name] = h.Size
		mode[h.Name] = h.Mode
	}

	// Tata letak harus sesuai yang dicari uf saat mencari spec bawaan.
	if _, ok := isi["bin/uf"]; !ok {
		t.Errorf("binary harus di bin/uf; isi = %v", keys(isi))
	}
	if mode["bin/uf"]&0o111 == 0 {
		t.Error("binary harus punya bit eksekusi")
	}
	for _, want := range []string{"share/uf/specs/git.json", "share/uf/specs/aws/s3.json"} {
		if _, ok := isi[want]; !ok {
			t.Errorf("mau %s; isi = %v", want, keys(isi))
		}
	}
}

func TestBundleTanpaSpec(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "uf")
	os.WriteFile(bin, []byte("BINARY"), 0o755)

	var buf bytes.Buffer
	if _, err := Bundle(bin, "", &buf); err != nil {
		t.Fatalf("spec kosong bukan kondisi kesalahan: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("arsip seharusnya tetap memuat binary")
	}
}

func TestBundleBinaryTidakAda(t *testing.T) {
	var buf bytes.Buffer
	if _, err := Bundle(filepath.Join(t.TempDir(), "tidak-ada"), "", &buf); err == nil {
		t.Error("mau error untuk binary yang tidak ada")
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 KB"},
		{6_600_000, "6.3 MB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.n); got != tt.want {
			t.Errorf("humanBytes(%d) = %s, mau %s", tt.n, got, tt.want)
		}
	}
}

// Entri arsip bisa disusun pihak lain, jadi path yang menunjuk keluar
// direktori tujuan tidak boleh pernah ditulis.
func TestUntarMenolakPathKeluar(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, name := range []string{"../keluar.txt", "/absolut.txt", "aman.txt"} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
		tw.Write([]byte("x"))
	}
	tw.Close()
	zw.Close()

	arc := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(arc, buf.Bytes(), 0o644)

	dst := t.TempDir()
	if err := untar(arc, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "keluar.txt")); err == nil {
		t.Error("entri ../ berhasil menulis di luar direktori tujuan")
	}
	if _, err := os.Stat(filepath.Join(dst, "aman.txt")); err != nil {
		t.Error("entri yang aman seharusnya tetap dibongkar")
	}
}

func keys(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
