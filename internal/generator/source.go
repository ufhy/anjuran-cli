package generator

import (
	"strings"
	"time"

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/spec"
)

// defaultTTL dipakai untuk generator yang tidak menyebut masa berlakunya.
//
// Cukup panjang untuk membuat Tab berulang terasa instan, cukup pendek untuk
// tidak menyesatkan: menampilkan nama pod yang sudah mati selama setengah
// menit lebih buruk daripada menunggu sebentar.
const defaultTTL = 5 * time.Second

// Source menjalankan generator dan template untuk sebuah hasil engine.
type Source struct {
	// Dir adalah direktori kerja; keluaran banyak generator bergantung padanya.
	Dir string
	// Timeout membatasi total waktu seluruh generator pada satu posisi kursor.
	Timeout time.Duration
	// Cache boleh nil, yang berarti jalan tanpa cache.
	Cache *Cache
	// Denied menampung alasan penolakan dari pemanggilan terakhir.
	Denied []string
}

// Candidates memenuhi antarmuka Dynamic milik paket ui.
func (s *Source) Candidates(res *engine.Result) []engine.Candidate {
	if res == nil {
		return nil
	}

	var out []engine.Candidate
	seen := map[string]bool{}
	add := func(name, desc string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		// Nama berkas berspasi harus dikutip; tanpa itu shell memecahnya
		// menjadi dua kata dan perintahnya rusak begitu dipilih.
		insert := res.Lead + engine.Quote(name)
		out = append(out, engine.Candidate{
			Name:         name,
			Insert:       insert,
			CursorOffset: len(insert),
			Description:  desc,
			Kind:         engine.KindArg,
			Priority:     engine.DefaultPriority,
		})
	}

	// Template dikerjakan lebih dulu karena tidak menjalankan proses sama
	// sekali, sehingga selalu tersedia bahkan saat generator dimatikan.
	for _, name := range FromTemplates(res.Templates, res.TemplateQuery(), s.Dir, res.Command) {
		add(name, "")
	}

	// Perintah yang didefinisikan pengguna di berkas proyek ditangani terpisah
	// dari template lain karena ia membawa KETERANGAN: isi perintahnya. "dev"
	// tidak memberi tahu apa pun, sedangkan "vite --port 3000" langsung
	// menjawab apa yang akan terjadi.
	for _, t := range res.Templates {
		if !AdaSumberSkrip(t) {
			continue
		}
		for _, sk := range SkripDari(t, s.Dir) {
			add(sk.Nama, sk.Perintah)
		}
	}

	// Bantuan menentukan PERINGKAT; spec tetap menentukan perilaku.
	//
	// Korpus spec berhenti dirawat pada Mei 2025, sehingga isinya menggambarkan
	// versi perintah yang belum tentu terpasang di sini: flag yang sudah dibuang
	// masih tercantum, flag yang baru tidak ada sama sekali. Keluaran `--help`
	// milik biner yang benar-benar ada di mesin ini selalu menggambarkan versi
	// yang benar, dan karena itu ia yang didahulukan.
	//
	// Yang didahulukan hanya urutannya. Kandidat yang dibenarkan --help naik ke
	// atas, dan yang hanya ada di spec turun ke bawah — tetapi entri spec-nya
	// sendiri tidak diganti, karena di sanalah tipe argumen dan generator
	// tersimpan: menukar `git checkout` versi spec dengan versi --help berarti
	// kehilangan daftar branch-nya.
	if len(res.Path) > 0 {
		// Ditanyakan pada POSISI kursor, bukan pada nama perintahnya saja:
		// `git commit -h` menyebut --amend, `git -h` tidak pernah.
		//
		// Path hanya memuat subcommand yang benar-benar dikenali spec. Untuk
		// perintah tanpa spec ia berisi nama perintahnya saja, dan itu
		// disengaja: menebak mana di antara kata yang sudah diketik adalah
		// subcommand berarti sesekali menjalankan `<alat> <nama-berkas> -h`.
		bantuan := s.Bantuan(res.Path, strings.HasPrefix(res.Prefix, "-"))

		dibenarkan := make(map[string]bool, len(bantuan))
		for _, c := range bantuan {
			dibenarkan[c.Name] = true
		}

		// Diubah di tempat, bukan lewat penggantian slice: pemanggil sudah
		// memegang slice yang sama.
		for i := range res.Candidates {
			if dibenarkan[res.Candidates[i].Name] {
				res.Candidates[i].Priority = BantuanPrioritas
				seen[res.Candidates[i].Name] = true
			}
		}

		// Sisanya adalah yang TIDAK diketahui spec sama sekali — justru inilah
		// bagian yang hilang selama korpusnya beku.
		//
		// Dimasukkan langsung, bukan lewat add(): yang dari bantuan sudah
		// membawa jenisnya sendiri — flag adalah flag, subcommand adalah
		// subcommand — dan nama flag tidak boleh dikutip seperti nama berkas.
		for _, c := range bantuan {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			out = append(out, c)
		}
	}

	if len(res.Generators) == 0 {
		return out
	}

	r := &Runner{
		Policy:  PolicyFromEnv(res.Command),
		Timeout: s.Timeout,
		Cache:   s.Cache,
		Dir:     s.Dir,
	}
	for _, c := range r.RunAll(toSpecs(res.Generators)) {
		add(c.Name, c.Description)
	}
	s.Denied = r.Denied

	return out
}

// toSpecs menerjemahkan generator dari skema berkas ke bentuk yang dijalankan.
func toSpecs(gs []spec.Generator) []Spec {
	out := make([]Spec, 0, len(gs))
	for _, g := range gs {
		if len(g.Script) == 0 {
			continue
		}
		ttl := defaultTTL
		if g.CacheTTL > 0 {
			ttl = time.Duration(g.CacheTTL) * time.Second
		}
		out = append(out, Spec{
			Script:   g.Script,
			SplitOn:  g.SplitOn,
			Trim:     g.Trim,
			CacheTTL: ttl,
			Trusted:  g.Trusted,
		})
	}
	return out
}
