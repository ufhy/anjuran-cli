package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func tulisBerkas(t *testing.T, dir, nama, isi string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nama), []byte(isi), 0o644); err != nil {
		t.Fatal(err)
	}
}

func nama(s []Skrip) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = x.Nama
	}
	return out
}

func samaPersis(t *testing.T, got []string, mau ...string) {
	t.Helper()
	if len(got) != len(mau) {
		t.Fatalf("dapat %v, mau %v", got, mau)
	}
	for i := range mau {
		if got[i] != mau[i] {
			t.Fatalf("dapat %v, mau %v", got, mau)
		}
	}
}

// Isi perintahnya dipakai sebagai keterangan: "dev" tidak memberi tahu apa pun,
// sedangkan "vite --port 3000" langsung menjawab apa yang akan terjadi.
func TestSkripPaketBesertaPerintahnya(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "package.json",
		`{"scripts":{"dev":"vite --port 3000","build":"tsc && vite build"}}`)

	got := SkripDari(TemplateSkripPaket, d)
	samaPersis(t, nama(got), "build", "dev") // terurut
	if got[1].Perintah != "vite --port 3000" {
		t.Errorf("keterangan = %q", got[1].Perintah)
	}
}

// Dicari ke ATAS seperti npm: "bun run dev" bekerja dari subdirektori mana pun.
func TestSkripDicariKeAtas(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "package.json", `{"scripts":{"dev":"vite"}}`)
	dalam := filepath.Join(d, "src", "komponen")
	if err := os.MkdirAll(dalam, 0o755); err != nil {
		t.Fatal(err)
	}

	samaPersis(t, nama(SkripDari(TemplateSkripPaket, dalam)), "dev")
}

// composer.json membolehkan larik perintah; deno yang baru memakai objek.
func TestNilaiSelainString(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "composer.json",
		`{"scripts":{"test":["phpunit","php-cs-fixer fix"]}}`)
	if got := SkripDari(TemplateSkripComposer, d); len(got) != 1 ||
		got[0].Perintah != "phpunit && php-cs-fixer fix" {
		t.Errorf("larik perintah = %+v", got)
	}

	e := t.TempDir()
	tulisBerkas(t, e, "deno.json",
		`{"tasks":{"start":{"command":"deno run -A main.ts","description":"jalankan server"}}}`)
	if got := SkripDari(TemplateTugasDeno, e); len(got) != 1 || got[0].Perintah != "jalankan server" {
		t.Errorf("objek tugas = %+v", got)
	}
}

// deno.jsonc memuat komentar; garis miring di dalam string bukan komentar.
func TestJSONCBerkomentar(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "deno.jsonc", `{
  // tugas proyek
  "tasks": {
    "start": "deno run -A https://contoh.test/main.ts" /* tetap utuh */
  }
}`)
	got := SkripDari(TemplateTugasDeno, d)
	if len(got) != 1 || got[0].Perintah != "deno run -A https://contoh.test/main.ts" {
		t.Errorf("dapat %+v", got)
	}
}

// Makefile: target khusus, aturan pola, dan penetapan variabel bukan target.
func TestTargetMakefile(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "Makefile", `
VERSI := 1.0
BIN_DIR = bin

build: deps ## bangun binernya
	go build ./...

uji bersih:
	go test ./...

%.o: %.c
	cc -c $<

.PHONY: build uji
`)
	got := SkripDari(TemplateTargetMake, d)
	samaPersis(t, nama(got), "bersih", "build", "uji")
	if got[1].Perintah != "bangun binernya" {
		t.Errorf("keterangan dari '##' = %q", got[1].Perintah)
	}
}

// justfile: keterangan diambil dari komentar tepat di atas resepnya, sama
// seperti yang ditampilkan `just --list`.
func TestResepJustfile(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "justfile", `
# jalankan server pengembangan
dev:
    vite

rilis versi:
    ./release.sh {{versi}}

export PATH := "bin"
`)
	got := SkripDari(TemplateResepJust, d)
	samaPersis(t, nama(got), "dev", "rilis")
	if got[0].Perintah != "jalankan server pengembangan" {
		t.Errorf("keterangan dari komentar = %q", got[0].Perintah)
	}
}

// Manifes yang rusak bukan alasan menggagalkan completion.
func TestManifesRusakTidakMenjatuhkan(t *testing.T) {
	d := t.TempDir()
	tulisBerkas(t, d, "package.json", `{"scripts": {rusak`)
	if got := SkripDari(TemplateSkripPaket, d); got != nil {
		t.Errorf("dapat %+v, mau kosong", got)
	}
}

// Tanpa berkas proyek sama sekali, pencarian berhenti tanpa menawarkan apa pun.
func TestTanpaBerkasProyek(t *testing.T) {
	if got := SkripDari(TemplateSkripPaket, t.TempDir()); got != nil {
		t.Errorf("dapat %+v, mau kosong", got)
	}
}

func TestTemplateTakDikenal(t *testing.T) {
	if AdaSumberSkrip("anjuran:entah") {
		t.Error("template karangan tidak boleh dikenali")
	}
	if got := SkripDari("anjuran:entah", t.TempDir()); got != nil {
		t.Errorf("dapat %+v, mau kosong", got)
	}
}
