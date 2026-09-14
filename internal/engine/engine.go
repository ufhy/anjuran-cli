// Package engine menerjemahkan sebuah baris perintah menjadi daftar kandidat.
//
// Seluruh isi paket ini murni: masukannya string dan posisi kursor, keluarannya
// struct. Tidak ada PTY, tidak ada escape sequence, tidak ada state global.
// Renderer dan integrasi shell dibangun di atasnya, bukan sebaliknya.
package engine

import (
	"sort"
	"strings"

	"github.com/uf-cli/uf/internal/parser"
	"github.com/uf-cli/uf/internal/spec"
)

// Kind mengelompokkan asal sebuah kandidat, dipakai renderer untuk ikon/warna.
type Kind string

const (
	KindSubcommand Kind = "subcommand"
	KindOption     Kind = "option"
	KindArg        Kind = "arg"
)

// Candidate adalah satu entri yang ditawarkan ke pengguna.
type Candidate struct {
	// Name adalah teks yang ditampilkan.
	Name string `json:"name"`
	// Insert adalah teks yang benar-benar disisipkan; biasanya sama dengan
	// Name, berbeda pada kasus seperti --opt=value.
	Insert      string `json:"insert"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind"`
}

// Result adalah jawaban lengkap engine untuk satu posisi kursor.
type Result struct {
	// Prefix adalah teks yang sudah diketik pada token kursor.
	Prefix string `json:"prefix"`
	// ReplaceStart dan ReplaceEnd adalah rentang byte pada baris asli yang
	// harus digantikan saat sebuah kandidat dipilih.
	ReplaceStart int         `json:"replaceStart"`
	ReplaceEnd   int         `json:"replaceEnd"`
	Candidates   []Candidate `json:"candidates"`
	// Generators berisi sumber dinamis yang RELEVAN di posisi ini namun
	// belum dieksekusi. Eksekusi ditunda ke layer terpisah supaya kebijakan
	// keamanannya (allowlist, timeout, mati saat UID 0) punya satu tempat.
	Generators []spec.Generator `json:"generators,omitempty"`
}

// Engine memegang registry spec.
type Engine struct {
	registry *spec.Registry
}

func New(r *spec.Registry) *Engine {
	return &Engine{registry: r}
}

// Complete adalah satu-satunya entry point paket ini.
func (e *Engine) Complete(line string, cursor int) (*Result, error) {
	l := parser.Parse(line, cursor)
	res := &Result{Prefix: l.Prefix}
	res.ReplaceStart, res.ReplaceEnd = replaceRange(l)

	words, _ := l.Words()
	if len(words) == 0 {
		// Kursor berada di posisi nama perintah. Melengkapi biner di PATH
		// adalah pekerjaan tahap lain; di sini kita berhenti.
		return res, nil
	}

	root, err := e.registry.Load(words[0].Value)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return res, nil // perintah tanpa spec: bukan error
	}

	st := walk(root, words[1:])
	e.suggest(res, st, l.Prefix)
	return res, nil
}

// state adalah posisi engine di dalam pohon spec setelah membaca seluruh
// token yang sudah selesai diketik.
type state struct {
	// current adalah subcommand terdalam yang sudah dimasuki.
	current *spec.Subcommand
	// persistent mengumpulkan opsi ber-isPersistent dari semua leluhur,
	// karena opsi seperti `git --git-dir` tetap berlaku di subcommand.
	persistent []spec.Option
	// pendingArg terisi bila token di posisi kursor adalah argumen milik
	// sebuah opsi, misalnya kursor tepat setelah `git commit -m `.
	pendingArg *spec.Arg
	// argIndex menghitung argumen posisional current yang sudah terisi.
	argIndex int
	// usedOptions menandai opsi yang sudah dipakai, untuk exclusiveOn
	// dan untuk menyembunyikan opsi non-repeatable.
	usedOptions map[string]bool
}

// walk mengeksekusi token-token yang sudah selesai diketik, menghasilkan
// state di posisi kursor.
func walk(root *spec.Subcommand, words []parser.Token) *state {
	st := &state{current: root, usedOptions: map[string]bool{}}

	i := 0
	for i < len(words) {
		tok := words[i].Value

		if isOptionToken(tok, words[i].Quote) {
			opt := st.lookupOption(tok)
			if opt != nil {
				for _, n := range opt.Name {
					st.usedOptions[n] = true
				}
			}

			// Bentuk --opt=value sudah membawa argumennya sendiri.
			if opt != nil && !strings.Contains(tok, "=") {
				for ai := range opt.Args {
					if opt.Args[ai].IsOptional {
						break
					}
					i++
					if i >= len(words) {
						// Slot argumen ini persis di posisi kursor.
						st.pendingArg = &opt.Args[ai]
						break
					}
				}
			}
			i++
			continue
		}

		if sub := st.current.FindSubcommand(tok); sub != nil {
			// Bawa turun opsi persisten milik induk sebelum berpindah.
			for _, o := range st.current.Options {
				if o.IsPersistent {
					st.persistent = append(st.persistent, o)
				}
			}
			st.current = sub
			st.argIndex = 0
			i++
			continue
		}

		// Bukan opsi, bukan subcommand: sebuah argumen posisional terisi.
		st.argIndex++
		i++
	}

	return st
}

// lookupOption mencari opsi di subcommand aktif, lalu di opsi persisten.
func (st *state) lookupOption(name string) *spec.Option {
	if o := st.current.FindOption(name); o != nil {
		return o
	}
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	for i := range st.persistent {
		if st.persistent[i].Name.Has(name) {
			return &st.persistent[i]
		}
	}
	return nil
}

// suggest mengisi res dengan kandidat yang cocok untuk prefix.
func (e *Engine) suggest(res *Result, st *state, prefix string) {
	// Kasus --opt=<kursor>: yang diminta adalah NILAI opsi tersebut.
	if eq := strings.IndexByte(prefix, '='); eq >= 0 && strings.HasPrefix(prefix, "-") {
		optName, valPrefix := prefix[:eq], prefix[eq+1:]
		if opt := st.lookupOption(optName); opt != nil && len(opt.Args) > 0 {
			e.addArg(res, &opt.Args[0], valPrefix, optName+"=")
		}
		return
	}

	// Kursor berada di slot argumen milik sebuah opsi.
	if st.pendingArg != nil {
		e.addArg(res, st.pendingArg, prefix, "")
		return
	}

	// Prefix diawali tanda minus: pengguna jelas sedang mencari opsi.
	if strings.HasPrefix(prefix, "-") && prefix != "-" {
		e.addOptions(res, st, prefix)
		return
	}

	// Konteks umum: subcommand, lalu argumen posisional, lalu opsi.
	for i := range st.current.Subcommands {
		sub := &st.current.Subcommands[i]
		if sub.Hidden {
			continue
		}
		for _, name := range sub.Name {
			if matches(name, prefix) {
				res.Candidates = append(res.Candidates, Candidate{
					Name:        name,
					Insert:      name,
					Description: sub.Description,
					Kind:        KindSubcommand,
				})
				break // satu entri per subcommand, pakai alias yang cocok
			}
		}
	}

	if st.argIndex < len(st.current.Args) {
		e.addArg(res, &st.current.Args[st.argIndex], prefix, "")
	} else if n := len(st.current.Args); n > 0 && st.current.Args[n-1].IsVariadic {
		// Argumen variadic menerima token tak terbatas.
		e.addArg(res, &st.current.Args[n-1], prefix, "")
	}

	e.addOptions(res, st, prefix)
	sortCandidates(res.Candidates)
}

// addOptions menambahkan opsi subcommand aktif dan opsi persisten.
func (e *Engine) addOptions(res *Result, st *state, prefix string) {
	add := func(opts []spec.Option) {
		for i := range opts {
			o := &opts[i]
			if o.Hidden || st.isExcluded(o) {
				continue
			}
			for _, name := range o.Name {
				if matches(name, prefix) {
					res.Candidates = append(res.Candidates, Candidate{
						Name:        name,
						Insert:      name,
						Description: o.Description,
						Kind:        KindOption,
					})
				}
			}
		}
	}
	add(st.current.Options)
	add(st.persistent)
	sortCandidates(res.Candidates)
}

// isExcluded menyembunyikan opsi yang sudah dipakai (dan tidak repeatable)
// atau yang bentrok dengan opsi lain yang sudah ada di baris.
func (st *state) isExcluded(o *spec.Option) bool {
	if !o.IsRepeatable {
		for _, n := range o.Name {
			if st.usedOptions[n] {
				return true
			}
		}
	}
	for _, x := range o.ExclusiveOn {
		if st.usedOptions[x] {
			return true
		}
	}
	return false
}

// addArg menawarkan suggestion statis milik sebuah argumen, dan mencatat
// generator-nya untuk dieksekusi oleh layer di atas engine.
func (e *Engine) addArg(res *Result, a *spec.Arg, prefix, insertPrefix string) {
	for i := range a.Suggestions {
		s := &a.Suggestions[i]
		for _, name := range s.Name {
			if s.Hidden && name != prefix {
				continue
			}
			if matches(name, prefix) {
				res.Candidates = append(res.Candidates, Candidate{
					Name:        name,
					Insert:      insertPrefix + name,
					Description: s.Description,
					Kind:        KindArg,
				})
				break
			}
		}
	}
	res.Generators = append(res.Generators, a.Generators...)
	sortCandidates(res.Candidates)
}

// isOptionToken membedakan flag dari argumen biasa. Token yang dikutip
// tidak pernah dianggap flag, sehingga `grep "-v"` tetap jadi argumen.
func isOptionToken(tok string, q parser.Quote) bool {
	if q != parser.QuoteNone {
		return false
	}
	return len(tok) > 1 && tok[0] == '-' && tok != "--"
}

// matches memakai pencocokan awalan tanpa membedakan huruf besar-kecil.
// Fuzzy matching menyusul di tahap renderer, di mana peringkatnya terlihat.
func matches(name, prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

// sortCandidates mengurutkan subcommand lebih dulu, lalu argumen, lalu opsi;
// masing-masing alfabetis. Opsi ditaruh terakhir karena saat prefix kosong
// yang dicari pengguna hampir selalu subcommand.
func sortCandidates(c []Candidate) {
	rank := map[Kind]int{KindSubcommand: 0, KindArg: 1, KindOption: 2}
	sort.SliceStable(c, func(i, j int) bool {
		if rank[c[i].Kind] != rank[c[j].Kind] {
			return rank[c[i].Kind] < rank[c[j].Kind]
		}
		return c[i].Name < c[j].Name
	})
}

// replaceRange menentukan rentang byte yang digantikan saat kandidat dipilih.
func replaceRange(l *parser.Line) (int, int) {
	if l.NewToken || l.CursorIndex >= len(l.Tokens) {
		return l.Cursor, l.Cursor
	}
	t := l.Tokens[l.CursorIndex]
	return t.Start, t.End
}
