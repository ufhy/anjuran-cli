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
// Kunci urutannya sengaja dipisah, bukan digabung jadi satu skor berbobot:
//
//   - Saat pengguna belum mengetik apa pun, seluruh skor fuzzy bernilai sama,
//     sehingga prioritas dari spec yang menentukan. Itulah yang membuat
//     "git " menaruh commit dan status di atas, bukan urutan alfabet.
//   - Begitu ada huruf yang diketik, relevansi yang memimpin, dan prioritas
//     turun menjadi pemutus seri.
//
// Bobot gabungan sempat dicoba tetapi menghasilkan urutan yang sulit dinalar:
// sebuah entri berprioritas tinggi bisa mengalahkan kecocokan awalan yang
// jelas lebih tepat.
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
	less := func(i, j int) bool {
		a, b := out[i], out[j]
		if query != "" && a.match.Score != b.match.Score {
			return a.match.Score > b.match.Score
		}
		if a.cand.Priority != b.cand.Priority {
			return a.cand.Priority > b.cand.Priority
		}
		if ra, rb := rank[a.cand.Kind], rank[b.cand.Kind]; ra != rb {
			return ra < rb
		}
		return a.cand.Name < b.cand.Name
	}
	sort.SliceStable(out, less)
	return out
}

// apply menyisipkan kandidat terpilih ke dalam baris.
//
// Kursor akhir mengikuti CursorOffset, bukan selalu ujung sisipan, sehingga
// bentuk seperti "--jobs=" meninggalkan kursor tepat sesudah tanda sama dengan.
func apply(st State, res *engine.Result, c engine.Candidate) State {
	line := st.Line[:res.ReplaceStart] + c.Insert + st.Line[res.ReplaceEnd:]
	return State{Line: line, Cursor: res.ReplaceStart + c.CursorOffset}
}

// items mengubah kandidat berperingkat menjadi baris yang bisa digambar.
func items(rs []ranked) []Item {
	out := make([]Item, len(rs))
	for i, r := range rs {
		out[i] = Item{
			Name:        r.cand.Label(),
			Description: r.cand.Description,
			Kind:        string(r.cand.Kind),
			Dangerous:   r.cand.Dangerous,
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
