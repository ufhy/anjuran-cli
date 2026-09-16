package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Daftar perintah yang didefinisikan pengguna di dalam berkas proyek.
//
// Semuanya satu keluarga: temukan berkas proyek TERDEKAT ke atas, lalu baca
// nama-nama yang didefinisikan di dalamnya. Yang berbeda hanya nama berkas dan
// bentuknya.
const (
	TemplateSkripPaket    = "anjuran:skrip-paket"    // package.json  -> scripts
	TemplateTugasDeno     = "anjuran:tugas-deno"     // deno.json(c)  -> tasks
	TemplateSkripComposer = "anjuran:skrip-composer" // composer.json -> scripts
	TemplateTargetMake    = "anjuran:target-make"    // Makefile
	TemplateResepJust     = "anjuran:resep-just"     // justfile
)

// naikMaks membatasi berapa tingkat direktori ditelusuri ke atas.
//
// npm dan kerabatnya mencari berkas proyek TERDEKAT ke atas, sehingga
// "bun run dev" bekerja dari subdirektori mana pun. Batas ini menjaga agar
// pencarian berhenti bila ternyata tidak ada proyek sama sekali, alih-alih
// menelusuri sampai akar pada setiap penekanan tombol.
const naikMaks = 40

// maksSkrip membatasi jumlah entri yang ditawarkan.
const maksSkrip = 500

// maksUkuranBerkas membatasi berkas yang dibaca. Manifes proyek yang waras
// berukuran kilobyte; apa pun di atas ini bukan sesuatu yang perlu dibaca pada
// setiap penekanan tombol.
const maksUkuranBerkas = 4 << 20

// Skrip adalah satu perintah yang didefinisikan di berkas proyek.
type Skrip struct {
	Nama string
	// Perintah adalah isinya, dipakai sebagai keterangan. Di situlah nilainya:
	// "dev" tidak memberi tahu apa pun, sedangkan "vite --port 3000" langsung
	// menjawab apa yang akan terjadi.
	Perintah string
}

type sumberSkrip struct {
	berkas []string
	baca   func([]byte) []Skrip
}

var sumber = map[string]sumberSkrip{
	TemplateSkripPaket:    {[]string{"package.json"}, dariJSON("scripts")},
	TemplateTugasDeno:     {[]string{"deno.json", "deno.jsonc"}, dariJSON("tasks")},
	TemplateSkripComposer: {[]string{"composer.json"}, dariJSON("scripts")},
	TemplateTargetMake:    {[]string{"Makefile", "makefile", "GNUmakefile"}, dariMakefile},
	TemplateResepJust:     {[]string{"justfile", "Justfile", ".justfile"}, dariJustfile},
}

// AdaSumberSkrip menjawab apakah sebuah template dilayani di sini.
func AdaSumberSkrip(template string) bool {
	_, ok := sumber[template]
	return ok
}

// SkripDari membaca perintah yang didefinisikan pengguna untuk sebuah template.
//
// Dikerjakan langsung di sini, BUKAN dengan menjalankan npm, bun, atau make:
// spec Fig melakukannya lewat `bash -c` yang menaiki direktori lalu membaca
// berkasnya, dan kebijakan generator menolak seluruh interpreter — argumennya
// adalah kode, bukan data. Membacanya sendiri lebih cepat, tidak menjalankan
// apa pun, dan tetap bekerja saat generator dimatikan.
func SkripDari(template, dir string) []Skrip {
	s, ok := sumber[template]
	if !ok {
		return nil
	}
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return nil
		}
	}

	b, ok := bacaTerdekat(dir, s.berkas)
	if !ok {
		return nil
	}
	out := s.baca(b)
	sort.Slice(out, func(i, j int) bool { return out[i].Nama < out[j].Nama })
	if len(out) > maksSkrip {
		out = out[:maksSkrip]
	}
	return out
}

// bacaTerdekat menaiki direktori mencari berkas pertama yang cocok.
func bacaTerdekat(dir string, nama []string) ([]byte, bool) {
	for i := 0; i < naikMaks; i++ {
		for _, n := range nama {
			p := filepath.Join(dir, n)
			info, err := os.Stat(p)
			if err != nil || info.IsDir() || info.Size() > maksUkuranBerkas {
				continue
			}
			if b, err := os.ReadFile(p); err == nil {
				return b, true
			}
		}
		induk := filepath.Dir(dir)
		if induk == dir {
			return nil, false // sudah di akar
		}
		dir = induk
	}
	return nil, false
}

// dariJSON membaca satu objek bernama field dari berkas JSON.
func dariJSON(field string) func([]byte) []Skrip {
	return func(b []byte) []Skrip {
		var isi map[string]json.RawMessage
		if err := json.Unmarshal(buangKomentar(b), &isi); err != nil {
			// Manifes yang rusak bukan alasan menggagalkan completion; yang
			// terjadi hanyalah tidak ada yang ditawarkan.
			return nil
		}
		var bagian map[string]json.RawMessage
		if err := json.Unmarshal(isi[field], &bagian); err != nil {
			return nil
		}

		out := make([]Skrip, 0, len(bagian))
		for nama, nilai := range bagian {
			if nama != "" {
				out = append(out, Skrip{Nama: nama, Perintah: ringkasNilai(nilai)})
			}
		}
		return out
	}
}

// ringkasNilai membentuk keterangan dari nilai yang bentuknya berbeda-beda.
//
// package.json memakai string, composer.json boleh memakai larik string, dan
// deno yang baru memakai objek dengan kunci "command". Yang tidak dikenali
// dilewati keterangannya saja — namanya tetap ditawarkan.
func ringkasNilai(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return satuBaris(s)
	}
	var larik []string
	if json.Unmarshal(raw, &larik) == nil {
		return satuBaris(strings.Join(larik, " && "))
	}
	var obj struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if obj.Description != "" {
			return satuBaris(obj.Description)
		}
		return satuBaris(obj.Command)
	}
	return ""
}

// targetMake cocok dengan baris aturan: nama di kolom pertama, lalu titik dua.
var targetMake = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_.\-/ ]*):(?:[^=]|$)`)

// dariMakefile membaca nama target dari sebuah Makefile.
//
// Yang dilewati: target khusus berawalan titik (.PHONY, .SUFFIXES), aturan pola
// yang memuat "%", dan penetapan variabel — "VAR := x" juga memuat titik dua,
// tetapi bukan target.
func dariMakefile(b []byte) []Skrip {
	var out []Skrip
	seen := map[string]bool{}
	for _, baris := range strings.Split(string(b), "\n") {
		if baris == "" || baris[0] == '\t' || baris[0] == '#' || baris[0] == '.' {
			continue
		}
		m := targetMake.FindStringSubmatch(baris)
		if m == nil || strings.ContainsAny(m[1], "%$") {
			continue
		}
		// "## keterangan" di baris yang sama adalah kebiasaan yang sudah
		// umum; dipakai bila ada.
		ket := ""
		if i := strings.Index(baris, "## "); i >= 0 {
			ket = satuBaris(baris[i+3:])
		}
		// Satu baris boleh menyebut beberapa target sekaligus.
		for _, nama := range strings.Fields(m[1]) {
			if !seen[nama] {
				seen[nama] = true
				out = append(out, Skrip{Nama: nama, Perintah: ket})
			}
		}
	}
	return out
}

// resepJust cocok dengan baris resep just: nama, parameter opsional, titik dua.
var resepJust = regexp.MustCompile(`^@?([A-Za-z0-9_][A-Za-z0-9_-]*)(\s+[^:=]*)?:(?:[^=]|$)`)

// dariJustfile membaca nama resep dari sebuah justfile.
//
// Keterangannya diambil dari komentar tepat DI ATAS resepnya — itulah cara
// just sendiri menampilkannya pada `just --list`.
func dariJustfile(b []byte) []Skrip {
	var out []Skrip
	seen := map[string]bool{}
	komentar := ""
	for _, baris := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(baris, "#") {
			komentar = satuBaris(strings.TrimLeft(baris, "# "))
			continue
		}
		m := resepJust.FindStringSubmatch(baris)
		if m == nil {
			komentar = ""
			continue
		}
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, Skrip{Nama: m[1], Perintah: komentar})
		}
		komentar = ""
	}
	return out
}

// buangKomentar membuang komentar gaya JSONC, yang dipakai deno.jsonc.
//
// Isi string dibiarkan utuh: "https://contoh" memuat dua garis miring dan
// bukan komentar.
func buangKomentar(b []byte) []byte {
	out := make([]byte, 0, len(b))
	dalamString, lolos := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if dalamString {
			out = append(out, c)
			switch {
			case lolos:
				lolos = false
			case c == '\\':
				lolos = true
			case c == '"':
				dalamString = false
			}
			continue
		}
		switch {
		case c == '"':
			dalamString = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return out
}

// satuBaris merapikan teks agar muat sebagai keterangan satu baris.
func satuBaris(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
