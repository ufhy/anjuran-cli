package ui

import (
	"sort"

	"github.com/uf-cli/uf/internal/engine"
)

// State adalah isi baris perintah beserta posisi kursornya.
type State struct {
	Line   string
	Cursor int
}

// Terminal adalah kemampuan terminal yang dibutuhkan sesi. Dibuat sebagai
// interface supaya loop di bawah bisa diuji tanpa PTY sungguhan.
type Terminal interface {
	ReadKey() (Key, error)
	Size() (int, int)
}

// Key dan KeyType diekspor ulang agar paket ui tidak memaksa pemanggilnya
// mengimpor paket tty hanya untuk mendeklarasikan sebuah tombol.
type (
	Key     = keyAlias
	KeyType = keyTypeAlias
)

// ranked adalah kandidat yang sudah dinilai.
type ranked struct {
	cand  engine.Candidate
	match *Match
}

// filter mencocokkan seluruh kandidat terhadap prefix lalu mengurutkannya.
//
// Urutannya: skor fuzzy menurun, lalu jenis (subcommand sebelum opsi), lalu
// alfabetis. Pengurutan terakhir penting agar daftar tidak bergoyang saat
// dua kandidat punya skor sama.
func filter(cands []engine.Candidate, query string) []ranked {
	out := make([]ranked, 0, len(cands))
	for _, c := range cands {
		m := FuzzyMatch(c.Name, query)
		if m == nil {
			continue
		}
		out = append(out, ranked{cand: c, match: m})
	}

	rank := map[engine.Kind]int{
		engine.KindSubcommand: 0,
		engine.KindArg:        1,
		engine.KindOption:     2,
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].match.Score != out[j].match.Score {
			return out[i].match.Score > out[j].match.Score
		}
		if ri, rj := rank[out[i].cand.Kind], rank[out[j].cand.Kind]; ri != rj {
			return ri < rj
		}
		return out[i].cand.Name < out[j].cand.Name
	})
	return out
}

// apply menyisipkan kandidat terpilih ke dalam baris.
func apply(st State, res *engine.Result, c engine.Candidate) State {
	line := st.Line[:res.ReplaceStart] + c.Insert + st.Line[res.ReplaceEnd:]
	return State{Line: line, Cursor: res.ReplaceStart + len(c.Insert)}
}

// items mengubah kandidat berperingkat menjadi baris yang bisa digambar.
func items(rs []ranked) []Item {
	out := make([]Item, len(rs))
	for i, r := range rs {
		out[i] = Item{
			Name:        r.cand.Name,
			Description: r.cand.Description,
			Kind:        string(r.cand.Kind),
			Highlight:   r.match.Positions,
		}
	}
	return out
}

// window memilih potongan daftar yang terlihat sehingga baris terpilih selalu
// berada di dalamnya, dan mengembalikan indeks terpilih relatif terhadapnya.
func window(total, selected, rows int) (start, relSelected int) {
	if total <= rows {
		return 0, selected
	}
	start = selected - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > total {
		start = total - rows
	}
	return start, selected - start
}
