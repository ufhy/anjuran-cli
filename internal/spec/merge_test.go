package spec

import (
	"path/filepath"
	"testing"
)

// Tambalan harus MENAMBAH, bukan menggantikan. Spec kubectl bawaan berisi
// ribuan opsi; menukarnya dengan berkas tambalan kecil akan menghilangkan
// semuanya, dan gejalanya diam-diam.
func TestTambalanTidakMenghapusIsiDasar(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()

	writeSpec(t, dasar, "k", `{"name":"k",
		"options":[{"name":"--verbose"},{"name":"--quiet"}],
		"subcommands":[
			{"name":"get","options":[{"name":"--watch"}],"args":[{"name":"resource"}]},
			{"name":"apply"}]}`)

	// Tambalan hanya mengisi argumen "get" yang kosong.
	writeSpec(t, tambalan, "k", `{"name":"k",
		"subcommands":[{"name":"get","args":[{"name":"resource",
			"suggestions":[{"name":"pods"},{"name":"services"}]}]}]}`)

	sc, err := NewRegistryDirs(tambalan, dasar).Load("k")
	if err != nil {
		t.Fatal(err)
	}

	if len(sc.Options) != 2 {
		t.Errorf("opsi akar = %d, mau 2 tetap utuh", len(sc.Options))
	}
	if len(sc.Subcommands) != 2 {
		t.Errorf("subcommand = %d, mau 2 tetap utuh", len(sc.Subcommands))
	}

	get := sc.FindSubcommand("get")
	if get == nil {
		t.Fatal("subcommand get hilang")
	}
	if get.FindOption("--watch") == nil {
		t.Error("opsi milik subcommand yang ditambal ikut hilang")
	}
	if len(get.Args) != 1 || len(get.Args[0].Suggestions) != 2 {
		t.Errorf("saran dari tambalan tidak terpasang: %+v", get.Args)
	}
}

// Sumber kandidat DIGANTI, bukan digabung: yang ada di dasar justru yang ingin
// diperbaiki. Menggabungnya akan menyisakan generator lama yang tidak jalan.
func TestSumberKandidatDiganti(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()
	writeSpec(t, dasar, "s", `{"name":"s","args":[{"name":"x",
		"generators":[{"template":["history"]}]}]}`)
	writeSpec(t, tambalan, "s", `{"name":"s","args":[{"name":"x",
		"generators":[{"template":["anjuran:hosts"]}]}]}`)

	sc, _ := NewRegistryDirs(tambalan, dasar).Load("s")
	gens := sc.Args[0].Generators
	if len(gens) != 1 {
		t.Fatalf("generator = %d, mau tepat 1 dari tambalan", len(gens))
	}
	if gens[0].Template[0] != "anjuran:hosts" {
		t.Errorf("generator = %v, mau milik tambalan", gens[0].Template)
	}
}

// Urutan lapisan menentukan siapa yang menimpa. Kalau terbalik, perbaikan
// justru ditimpa oleh yang ingin diperbaiki.
func TestLapisanPertamaMenimpa(t *testing.T) {
	atas, bawah := t.TempDir(), t.TempDir()
	writeSpec(t, atas, "x", `{"name":"x","description":"dari atas"}`)
	writeSpec(t, bawah, "x", `{"name":"x","description":"dari bawah","options":[{"name":"-v"}]}`)

	sc, _ := NewRegistryDirs(atas, bawah).Load("x")
	if sc.Description != "dari atas" {
		t.Errorf("deskripsi = %q, mau dari lapisan teratas", sc.Description)
	}
	if sc.FindOption("-v") == nil {
		t.Error("opsi dari lapisan bawah seharusnya tetap terbawa")
	}
}

func TestSubcommandBaruDitambahkan(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()
	writeSpec(t, dasar, "y", `{"name":"y","subcommands":[{"name":"lama"}]}`)
	writeSpec(t, tambalan, "y", `{"name":"y","subcommands":[{"name":"baru"}]}`)

	sc, _ := NewRegistryDirs(tambalan, dasar).Load("y")
	if sc.FindSubcommand("lama") == nil || sc.FindSubcommand("baru") == nil {
		t.Errorf("keduanya harus ada, dapat %d subcommand", len(sc.Subcommands))
	}
}

func TestArgumenDicocokkanBerdasarkanUrutan(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()
	writeSpec(t, dasar, "z", `{"name":"z","args":[{"name":"pertama"},{"name":"kedua"}]}`)
	writeSpec(t, tambalan, "z", `{"name":"z","args":[{},{"suggestions":[{"name":"b"}]}]}`)

	sc, _ := NewRegistryDirs(tambalan, dasar).Load("z")
	if len(sc.Args) != 2 {
		t.Fatalf("argumen = %d, mau 2", len(sc.Args))
	}
	if sc.Args[0].Name != "pertama" {
		t.Errorf("argumen pertama tertimpa: %q", sc.Args[0].Name)
	}
	if len(sc.Args[1].Suggestions) != 1 {
		t.Error("saran untuk argumen kedua tidak terpasang")
	}
	if sc.Args[1].Name != "kedua" {
		t.Errorf("nama argumen kedua hilang: %q", sc.Args[1].Name)
	}
}

func TestTanpaTambalanTidakBerubah(t *testing.T) {
	dasar := t.TempDir()
	writeSpec(t, dasar, "w", `{"name":"w","options":[{"name":"-a"},{"name":"-b"}]}`)

	sc, _ := NewRegistryDirs(t.TempDir(), dasar).Load("w")
	if len(sc.Options) != 2 {
		t.Errorf("opsi = %d, mau utuh", len(sc.Options))
	}
}

// Tambalan boleh ada untuk perintah yang tidak punya spec dasar sama sekali.
func TestTambalanTanpaDasar(t *testing.T) {
	tambalan := t.TempDir()
	writeSpec(t, tambalan, "baru", `{"name":"baru","args":[{"template":["folders"]}]}`)

	sc, _ := NewRegistryDirs(tambalan, t.TempDir()).Load("baru")
	if sc == nil || len(sc.Args) != 1 {
		t.Fatalf("spec tambalan mandiri harus tetap terbaca: %+v", sc)
	}
}

// Berkas tambalan yang rusak harus terlihat sebagai error, bukan diam-diam
// membuat perintahnya kehilangan spec.
func TestTambalanRusakMenghasilkanError(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()
	writeSpec(t, dasar, "r", `{"name":"r"}`)
	writeSpec(t, tambalan, "r", `{"name":`)

	if _, err := NewRegistryDirs(tambalan, dasar).Load("r"); err == nil {
		t.Error("mau error untuk tambalan yang rusak")
	}
}

func TestTambalanTerkompresi(t *testing.T) {
	dasar, tambalan := t.TempDir(), t.TempDir()
	writeSpec(t, dasar, "g", `{"name":"g","options":[{"name":"-x"}]}`)
	writeSpecGz(t, tambalan, "g", `{"name":"g","description":"ditambal"}`)

	sc, _ := NewRegistryDirs(tambalan, dasar).Load("g")
	if sc.Description != "ditambal" || sc.FindOption("-x") == nil {
		t.Errorf("tambalan gzip tidak digabung dengan benar: %+v", sc)
	}
}

// Rujukan loadSpec tetap tidak boleh keluar dari direktori spec, juga saat
// digabung dari beberapa lapisan.
func TestGabunganTetapMenolakPathKeluar(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "j", `{"name":"j"}`)
	r := NewRegistryDirs(dir)
	if sc, _ := r.Load(filepath.Join("..", "j")); sc != nil {
		t.Error("path keluar direktori seharusnya ditolak")
	}
}

// Tambalan yang menyebut satu sumber kandidat mengganti SELURUHNYA.
//
// Mengganti per bidang menyisakan sumber lama: "bun run" tetap membawa
// generator `bash -c ... cat package.json` milik spec Fig walaupun tambalannya
// sudah memberi template pengganti, dan generator itu lalu ditolak kebijakan
// pada setiap penekanan tombol.
func TestTambalanMenggantiSumberKandidatSeluruhnya(t *testing.T) {
	base := &Subcommand{
		Name: []string{"bun"},
		Args: []Arg{{
			Name:       "script",
			Generators: []Generator{{Script: []string{"bash", "-c", "cat package.json"}}},
		}},
	}
	overlay := &Subcommand{
		Name: []string{"bun"},
		Args: []Arg{{Name: "script", Template: []string{"anjuran:skrip-paket"}}},
	}

	got := merge(base, overlay)
	if len(got.Args) != 1 {
		t.Fatalf("Args = %+v", got.Args)
	}
	if len(got.Args[0].Generators) != 0 {
		t.Errorf("generator lama masih terbawa: %+v", got.Args[0].Generators)
	}
	if len(got.Args[0].Template) != 1 || got.Args[0].Template[0] != "anjuran:skrip-paket" {
		t.Errorf("Template = %v", got.Args[0].Template)
	}
}
