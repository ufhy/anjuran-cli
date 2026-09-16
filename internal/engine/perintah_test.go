package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/spec"
)

// Di posisi nama perintah, engine meminta daftar perintah — bukan diam.
//
// Diam di sini adalah perbedaan paling terasa dengan sebuah editor: mengetik
// nama sesuatu harus memunculkan daftarnya.
func TestPosisiPerintahMemintaDaftarPerintah(t *testing.T) {
	e := New(spec.NewRegistry("../testdata/specs"))

	for _, line := range []string{"gi", "docker ps | gi", "make; gi"} {
		res, err := e.Complete(line, len(line))
		if err != nil {
			t.Fatal(err)
		}
		if !berkasAtau(res.Templates, "commands") {
			t.Errorf("%q: Templates = %v, mau memuat \"commands\"", line, res.Templates)
		}
	}
}

func berkasAtau(ts []string, want string) bool {
	for _, t := range ts {
		if t == want {
			return true
		}
	}
	return false
}

// Nama perintah yang sudah lengkap langsung menawarkan isinya: itu yang membuat
// "ketik git, lihat git bisa apa" bekerja tanpa menekan spasi lebih dulu.
func TestNamaPerintahLengkapMenawarkanSubcommand(t *testing.T) {
	e := New(spec.NewRegistry("../testdata/specs"))

	res, err := e.Complete("git", 3)
	if err != nil {
		t.Fatal(err)
	}

	var commit *Candidate
	for i := range res.Candidates {
		if res.Candidates[i].Name == "commit" {
			commit = &res.Candidates[i]
		}
	}
	if commit == nil {
		t.Fatalf("subcommand git tidak ditawarkan; dapat %v", names(res.Candidates))
	}

	// Ditampilkan tanpa mengulang nama perintahnya, tetapi disisipkan utuh —
	// yang diganti adalah kata "git" itu sendiri.
	if commit.Insert != "git commit" {
		t.Errorf("Insert = %q, mau %q", commit.Insert, "git commit")
	}
	if res.ReplaceStart != 0 || res.ReplaceEnd != 3 {
		t.Errorf("rentang ganti = %d..%d, mau 0..3", res.ReplaceStart, res.ReplaceEnd)
	}
	// Tidak ada lagi yang perlu disaring; menyaring dengan "git" membuat
	// panjang nama menjadi satu-satunya pembeda.
	if !res.FilterPrefixSet || res.FilterPrefix != "" {
		t.Errorf("FilterPrefix = %q (set=%v), mau kosong dan set",
			res.FilterPrefix, res.FilterPrefixSet)
	}
}

// Nama yang belum lengkap tidak menawarkan subcommand siapa pun.
func TestNamaPerintahBelumLengkapTidakMenawarkanSubcommand(t *testing.T) {
	e := New(spec.NewRegistry("../testdata/specs"))

	res, err := e.Complete("gi", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("mau tidak ada kandidat dari spec, dapat %v", names(res.Candidates))
	}
	if res.FilterPrefixSet {
		t.Error("penyaringan tidak boleh dimatikan saat namanya belum lengkap")
	}
}

// Perintah yang tidak punya spec tetap sah sebagai nama perintah; yang tidak
// ada hanyalah isinya.
func TestPerintahTanpaSpecTidakJatuh(t *testing.T) {
	e := New(spec.NewRegistry("../testdata/specs"))

	res, err := e.Complete("perintahkarangan", 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("mau tidak ada kandidat, dapat %v", names(res.Candidates))
	}
}

// Jalur ke sebuah berkas bukan nama perintah lagi.
func TestJalurBukanNamaPerintah(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skrip.sh"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	e := New(spec.NewRegistry("../testdata/specs")).InDir(dir)

	res, err := e.Complete("./skr", 5)
	if err != nil {
		t.Fatal(err)
	}
	// Template "commands" boleh diminta, tetapi sumbernya yang menolak jalur;
	// yang penting engine tidak jatuh dan rentang gantinya benar.
	if res.ReplaceStart != 0 || res.ReplaceEnd != 5 {
		t.Errorf("rentang ganti = %d..%d, mau 0..5", res.ReplaceStart, res.ReplaceEnd)
	}
}
