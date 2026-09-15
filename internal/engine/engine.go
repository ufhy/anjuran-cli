// Package engine menerjemahkan sebuah baris perintah menjadi daftar kandidat.
//
// Seluruh isi paket ini murni: masukannya string dan posisi kursor, keluarannya
// struct. Tidak ada PTY, tidak ada escape sequence, tidak ada state global.
// Renderer dan integrasi shell dibangun di atasnya, bukan sebaliknya.
package engine

import (
	"os"
	"path/filepath"
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
	// Name adalah nama kanonik kandidat.
	Name string `json:"name"`
	// Display adalah teks yang ditampilkan bila berbeda dari Name.
	Display string `json:"display,omitempty"`
	// Insert adalah teks yang benar-benar disisipkan; biasanya sama dengan
	// Name, berbeda pada kasus seperti --opt=value.
	Insert string `json:"insert"`
	// CursorOffset adalah posisi kursor akhir, dihitung dari awal Insert.
	// Spec Fig memakai penanda {cursor} untuk ini, misalnya pada opsi yang
	// nilainya harus menempel: menyisipkan "--jobs=" sebaiknya meletakkan
	// kursor sesudah tanda sama dengan, bukan di ujung kata berikutnya.
	CursorOffset int    `json:"cursorOffset,omitempty"`
	Description  string `json:"description,omitempty"`
	Kind         Kind   `json:"kind"`
	// Priority mengikuti konvensi Fig: 50 adalah nilai bawaan.
	Priority int `json:"priority,omitempty"`
	// Dangerous menandai kandidat yang merusak bila salah pilih, misalnya
	// --force pada git push.
	Dangerous bool `json:"dangerous,omitempty"`
}

// DefaultPriority adalah nilai yang dipakai Fig bila sebuah entri tidak
// menyebut prioritasnya sendiri.
const DefaultPriority = 50

// Label mengembalikan teks yang seharusnya ditampilkan.
func (c Candidate) Label() string {
	if c.Display != "" {
		return c.Display
	}
	return c.Name
}

// cursorMarker adalah penanda posisi kursor di dalam insertValue milik Fig.
const cursorMarker = "{cursor}"

// makeInsert menerjemahkan insertValue menjadi teks sisipan dan posisi kursor.
func makeInsert(name, insertValue, prefix string) (string, int) {
	text := name
	if insertValue != "" {
		text = insertValue
	}
	offset := -1
	if i := strings.Index(text, cursorMarker); i >= 0 {
		text = text[:i] + text[i+len(cursorMarker):]
		offset = i
	}
	text = prefix + text
	if offset < 0 {
		return text, len(text)
	}
	return text, len(prefix) + offset
}

// Result adalah jawaban lengkap engine untuk satu posisi kursor.
type Result struct {
	// Command adalah nama perintah yang sedang dilengkapi. Dibawa keluar
	// karena kebijakan generator memutuskan berdasarkan nama itu.
	Command string `json:"command,omitempty"`
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
	// Templates adalah sumber bawaan seperti filepaths dan folders, yang
	// dikerjakan tanpa menjalankan proses apa pun.
	Templates []string `json:"templates,omitempty"`
}

// Engine memegang registry spec.
type Engine struct {
	registry *spec.Registry
	// Dir adalah direktori kerja, dipakai menilai syarat whenFile.
	Dir string
	// Exists bisa diganti saat pengujian; nil berarti memeriksa berkas
	// sungguhan.
	Exists func(path string) bool
}

func New(r *spec.Registry) *Engine {
	return &Engine{registry: r}
}

// InDir menyetel direktori kerja yang dipakai menilai syarat whenFile.
func (e *Engine) InDir(dir string) *Engine {
	e.Dir = dir
	return e
}

// available menilai syarat whenFile sebuah entri.
//
// Syarat kosong berarti selalu tersedia. Berkasnya dicari relatif terhadap
// direktori kerja, karena itulah tempat perintahnya akan dijalankan.
func (e *Engine) available(whenFile string) bool {
	if whenFile == "" {
		return true
	}
	path := whenFile
	if !filepath.IsAbs(path) && e.Dir != "" {
		path = filepath.Join(e.Dir, whenFile)
	}
	if e.Exists != nil {
		return e.Exists(path)
	}
	_, err := os.Stat(path)
	return err == nil
}

// Complete adalah satu-satunya entry point paket ini.
func (e *Engine) Complete(line string, cursor int) (*Result, error) {
	l := parser.Parse(line, cursor)
	res := &Result{Prefix: l.Prefix}
	res.ReplaceStart, res.ReplaceEnd = replaceRange(l)

	words, _ := l.Words()
	if len(words) > 0 {
		res.Command = words[0].Value
	}
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
		// Perintah tanpa spec tetap melengkapi nama berkas.
		//
		// Korpus Fig memuat 716 perintah; sisanya — gzip, awk, openssl,
		// perintah internal perusahaan, skrip apa pun di PATH — tidak ada di
		// sana. Diam total di situ salah: shell mana pun melengkapi path untuk
		// perintah yang tidak dikenalnya, dan itulah yang paling sering
		// dibutuhkan.
		res.Templates = append(res.Templates, "filepaths")
		return res, nil
	}

	root, err = e.registry.Resolve(root)
	if err != nil {
		return nil, err
	}

	st, err := e.walk(root, words[1:])
	if err != nil {
		return nil, err
	}
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
func (e *Engine) walk(root *spec.Subcommand, words []parser.Token) (*state, error) {
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

			// Bentuk --opt=value sudah membawa argumennya sendiri, begitu
			// pula opsi yang memang mewajibkan nilainya menempel.
			if opt != nil && !strings.Contains(tok, "=") && !opt.RequiresSeparator {
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
			resolved, err := e.registry.Resolve(sub)
			if err != nil {
				return nil, err
			}
			// Bawa turun opsi persisten milik induk sebelum berpindah.
			for _, o := range st.current.Options {
				if o.IsPersistent {
					st.persistent = append(st.persistent, o)
				}
			}
			st.current = resolved
			st.argIndex = 0
			i++
			continue
		}

		// Bukan opsi, bukan subcommand: sebuah argumen posisional terisi.
		st.argIndex++
		i++
	}

	return st, nil
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
		if sub.Hidden || sub.Deprecated || !e.available(sub.WhenFile) {
			continue
		}
		for _, name := range sub.Name {
			if matches(name, prefix) {
				insert, offset := makeInsert(name, sub.InsertValue, "")
				res.Candidates = append(res.Candidates, Candidate{
					Name:         name,
					Display:      sub.DisplayName,
					Insert:       insert,
					CursorOffset: offset,
					Description:  sub.Description,
					Kind:         KindSubcommand,
					Priority:     priorityOr(sub.Priority),
					Dangerous:    sub.IsDangerous,
				})
				break // satu entri per subcommand, pakai alias yang cocok
			}
		}
	}

	switch n := len(st.current.Args); {
	case st.argIndex < n:
		e.addArg(res, &st.current.Args[st.argIndex], prefix, "")
	case n > 0 && st.current.Args[n-1].IsVariadic:
		// Argumen variadic menerima token tak terbatas.
		e.addArg(res, &st.current.Args[n-1], prefix, "")
	case n == 0 && len(st.current.Subcommands) == 0:
		// Perintah yang spec-nya tidak menyebutkan argumen apa pun, dan tidak
		// punya subcommand untuk ditawarkan. Ratusan spec berhenti di daftar
		// opsi saja; melengkapi berkas adalah tebakan terbaik yang tersedia.
		res.Templates = append(res.Templates, "filepaths")
	}

	e.addOptions(res, st, prefix)
	sortCandidates(res.Candidates)
}

// addOptions menambahkan opsi subcommand aktif dan opsi persisten.
func (e *Engine) addOptions(res *Result, st *state, prefix string) {
	add := func(opts []spec.Option) {
		for i := range opts {
			o := &opts[i]
			if o.Hidden || o.Deprecated || st.isExcluded(o) {
				continue
			}
			for _, name := range o.Name {
				if !matches(name, prefix) {
					continue
				}
				// Opsi yang nilainya wajib menempel disisipkan sekalian dengan
				// tanda sama dengan, supaya pengguna tidak menulis bentuk yang
				// justru ditolak perintahnya.
				insertValue := o.InsertValue
				if insertValue == "" && o.RequiresSeparator && len(o.Args) > 0 {
					insertValue = name + "=" + cursorMarker
				}
				insert, offset := makeInsert(name, insertValue, "")
				res.Candidates = append(res.Candidates, Candidate{
					Name:         name,
					Display:      o.DisplayName,
					Insert:       insert,
					CursorOffset: offset,
					Description:  o.Description,
					Kind:         KindOption,
					Priority:     priorityOr(o.Priority),
					Dangerous:    o.IsDangerous,
				})
			}
		}
	}
	add(st.current.Options)
	add(st.persistent)
	sortCandidates(res.Candidates)
}

// isExcluded menyembunyikan opsi yang sudah dipakai (dan tidak repeatable),
// yang bentrok dengan opsi lain di baris, atau yang syaratnya belum terpenuhi.
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
	// dependsOn: opsi baru masuk akal setelah opsi lain hadir, misalnya
	// --set-upstream yang hanya berguna bersama nama remote.
	for _, d := range o.DependsOn {
		if !st.usedOptions[d] {
			return true
		}
	}
	return false
}

// priorityOr menerapkan nilai bawaan Fig untuk entri tanpa prioritas.
func priorityOr(p int) int {
	if p == 0 {
		return DefaultPriority
	}
	return p
}

// addArg menawarkan suggestion statis milik sebuah argumen, dan mencatat
// generator-nya untuk dieksekusi oleh layer di atas engine.
func (e *Engine) addArg(res *Result, a *spec.Arg, prefix, insertPrefix string) {
	for i := range a.Suggestions {
		s := &a.Suggestions[i]
		if s.Deprecated || !e.available(s.WhenFile) {
			continue
		}
		for _, name := range s.Name {
			if s.Hidden && name != prefix {
				continue
			}
			if matches(name, prefix) {
				insert, offset := makeInsert(name, s.InsertValue, insertPrefix)
				res.Candidates = append(res.Candidates, Candidate{
					Name:         name,
					Display:      s.DisplayName,
					Insert:       insert,
					CursorOffset: offset,
					Description:  s.Description,
					Kind:         KindArg,
					Priority:     priorityOr(s.Priority),
				})
				break
			}
		}
	}
	res.Generators = append(res.Generators, a.Generators...)
	res.Templates = append(res.Templates, a.Template...)
	for _, g := range a.Generators {
		res.Templates = append(res.Templates, g.Template...)
	}

	// Argumen tanpa sumber kandidat apa pun dilengkapi sebagai nama berkas.
	//
	// Ratusan spec hanya menyebut nama argumennya — "python script", "node
	// script" — tanpa menyebut isinya dari mana. Tanpa aturan ini, Tab di situ
	// diam sama sekali. Melengkapi berkas adalah tebakan terbaik yang
	// tersedia, dan itu pula yang dilakukan shell tanpa completion khusus.
	if !hasSource(a) {
		res.Templates = append(res.Templates, "filepaths")
	}

	sortCandidates(res.Candidates)
}

// hasSource menjawab apakah sebuah argumen menyebutkan dari mana kandidatnya
// berasal.
func hasSource(a *spec.Arg) bool {
	if len(a.Suggestions) > 0 || len(a.Template) > 0 {
		return true
	}
	for _, g := range a.Generators {
		if len(g.Script) > 0 || len(g.Template) > 0 {
			return true
		}
	}
	return false
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
