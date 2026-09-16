package generator

import (
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
		insert := engine.Quote(name)
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
	for _, name := range FromTemplates(res.Templates, res.Prefix, s.Dir, res.Command) {
		add(name, "")
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
