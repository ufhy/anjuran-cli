package spec

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// writeSpec menulis spec JSON polos ke dalam dir.
func writeSpec(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel)+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSpecGz menulis spec dalam bentuk terkompresi.
func writeSpecGz(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel)+".json.gz")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	if _, err := zw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNamaBolehStringAtauArray(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "a", `{"name":"a","subcommands":[{"name":"satu"},{"name":["dua","d"]}]}`)

	sc, err := NewRegistry(dir).Load("a")
	if err != nil {
		t.Fatal(err)
	}
	if got := sc.Subcommands[0].Name.Primary(); got != "satu" {
		t.Errorf("name string = %q, mau satu", got)
	}
	if !sc.Subcommands[1].Name.Has("d") {
		t.Error("alias dari name array tidak terbaca")
	}
}

func TestMembacaBentukGzip(t *testing.T) {
	dir := t.TempDir()
	writeSpecGz(t, dir, "z", `{"name":"z","description":"terkompresi"}`)

	sc, err := NewRegistry(dir).Load("z")
	if err != nil {
		t.Fatal(err)
	}
	if sc == nil || sc.Description != "terkompresi" {
		t.Fatalf("spec gzip tidak terbaca: %+v", sc)
	}
}

func TestGzipDiutamakanDaripadaJSON(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "p", `{"name":"p","description":"polos"}`)
	writeSpecGz(t, dir, "p", `{"name":"p","description":"gzip"}`)

	sc, _ := NewRegistry(dir).Load("p")
	if sc.Description != "gzip" {
		t.Errorf("mau bentuk gzip yang menang, dapat %q", sc.Description)
	}
}

func TestPerintahTanpaSpecBukanError(t *testing.T) {
	sc, err := NewRegistry(t.TempDir()).Load("tidak-ada")
	if err != nil {
		t.Fatalf("mau nil tanpa error, dapat %v", err)
	}
	if sc != nil {
		t.Error("mau nil")
	}
}

func TestLoadSpecMenggantiIsiSubcommand(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "cloud", `{"name":"cloud","subcommands":[{"name":"store","description":"asli","loadSpec":"cloud/store"}]}`)
	writeSpec(t, dir, "cloud/store", `{"name":"store","subcommands":[{"name":"ls"},{"name":"cp"}]}`)

	r := NewRegistry(dir)
	root, _ := r.Load("cloud")
	sub, err := r.Resolve(root.FindSubcommand("store"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sub.Subcommands) != 2 {
		t.Fatalf("mau 2 subcommand dari berkas rujukan, dapat %d", len(sub.Subcommands))
	}
	// Deskripsi milik simpul pemanggil dipertahankan; itulah yang dikenal
	// pengguna saat mengetik.
	if sub.Description != "asli" {
		t.Errorf("deskripsi = %q, mau dipertahankan dari pemanggil", sub.Description)
	}
	if sub.LoadSpec != "" {
		t.Error("loadSpec harus dikosongkan setelah diselesaikan")
	}
}

func TestLoadSpecHilangTidakMematikanCompletion(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "x", `{"name":"x","subcommands":[{"name":"y","loadSpec":"x/tidak-ada","options":[{"name":"--tetap"}]}]}`)

	r := NewRegistry(dir)
	root, _ := r.Load("x")
	sub, err := r.Resolve(root.FindSubcommand("y"))
	if err != nil {
		t.Fatal(err)
	}
	if sub.FindOption("--tetap") == nil {
		t.Error("rujukan yang hilang harus jatuh kembali ke simpul apa adanya")
	}
}

// Berkas spec berasal dari pihak ketiga, jadi rujukan loadSpec tidak boleh
// bisa membaca berkas di luar direktori spec.
func TestLoadSpecTidakBisaKeluarDirektori(t *testing.T) {
	dir := t.TempDir()
	rahasia := filepath.Join(filepath.Dir(dir), "rahasia.json")
	os.WriteFile(rahasia, []byte(`{"name":"rahasia"}`), 0o644)
	defer os.Remove(rahasia)

	writeSpec(t, dir, "jahat", `{"name":"jahat","subcommands":[{"name":"a","loadSpec":"../rahasia"}]}`)

	r := NewRegistry(dir)
	root, _ := r.Load("jahat")
	sub, _ := r.Resolve(root.FindSubcommand("a"))
	if sub.Name.Primary() == "rahasia" {
		t.Fatal("rujukan keluar direktori spec seharusnya ditolak")
	}
}

func TestNamaPerintahBerisiPemisahPathDitolak(t *testing.T) {
	r := NewRegistry(t.TempDir())
	for _, bad := range []string{"../etc/passwd", "a/b", `a\b`} {
		if sc, _ := r.Load(bad); sc != nil {
			t.Errorf("Load(%q) seharusnya nil", bad)
		}
	}
}

func TestSpecBerversiMemilihYangTertinggi(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "tool/1.9.0", `{"name":"tool","description":"lama"}`)
	writeSpec(t, dir, "tool/2.0.0", `{"name":"tool","description":"baru"}`)
	writeSpec(t, dir, "tool/10.0.0", `{"name":"tool","description":"terbaru"}`)

	sc, err := NewRegistry(dir).Load("tool")
	if err != nil {
		t.Fatal(err)
	}
	if sc == nil {
		t.Fatal("spec berversi tidak ditemukan")
	}
	// Perbandingan harus numerik, bukan leksikografis: 10 lebih baru dari 2.
	if sc.Description != "terbaru" {
		t.Errorf("versi terpilih = %q, mau terbaru", sc.Description)
	}
}

func TestSpecAkarMengalahkanSpecBerversi(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "tool", `{"name":"tool","description":"akar"}`)
	writeSpec(t, dir, "tool/9.0.0", `{"name":"tool","description":"berversi"}`)

	sc, _ := NewRegistry(dir).Load("tool")
	if sc.Description != "akar" {
		t.Errorf("mau spec akar yang dipakai, dapat %q", sc.Description)
	}
}

func TestDirektoriPertamaMenang(t *testing.T) {
	atas, bawah := t.TempDir(), t.TempDir()
	writeSpec(t, atas, "g", `{"name":"g","description":"milik pengguna"}`)
	writeSpec(t, bawah, "g", `{"name":"g","description":"bawaan"}`)

	sc, _ := NewRegistryDirs(atas, bawah).Load("g")
	if sc.Description != "milik pengguna" {
		t.Errorf("mau direktori pertama yang menang, dapat %q", sc.Description)
	}
}

func TestJatuhKeDirektoriBerikutnya(t *testing.T) {
	atas, bawah := t.TempDir(), t.TempDir()
	writeSpec(t, bawah, "h", `{"name":"h","description":"bawaan"}`)

	sc, _ := NewRegistryDirs(atas, bawah).Load("h")
	if sc == nil || sc.Description != "bawaan" {
		t.Errorf("mau jatuh ke direktori berikutnya, dapat %+v", sc)
	}
}

func TestCacheMenghindariPembacaanUlang(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "c", `{"name":"c","description":"pertama"}`)

	r := NewRegistry(dir)
	first, _ := r.Load("c")

	// Berkas diubah setelah pembacaan pertama; cache harus tetap dipakai.
	writeSpec(t, dir, "c", `{"name":"c","description":"kedua"}`)
	second, _ := r.Load("c")

	if first != second {
		t.Error("mau objek yang sama dari cache")
	}
}

func TestJSONRusakMenghasilkanError(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "r", `{"name":`)
	if _, err := NewRegistry(dir).Load("r"); err == nil {
		t.Error("mau error untuk JSON rusak")
	}
}

func TestFindOptionMengenaliBentukSamaDengan(t *testing.T) {
	sc := &Subcommand{Options: []Option{{Name: Names{"--jobs"}}}}
	if sc.FindOption("--jobs=4") == nil {
		t.Error("--jobs=4 harus mengenali opsi --jobs")
	}
}
