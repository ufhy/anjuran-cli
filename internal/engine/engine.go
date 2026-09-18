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
	"unicode"

	"github.com/ufhy/anjuran-cli/internal/parser"
	"github.com/ufhy/anjuran-cli/internal/spec"
)

// Kind mengelompokkan asal sebuah kandidat, dipakai renderer untuk ikon/warna.
type Kind string

const (
	KindSubcommand Kind = "subcommand"
	KindOption     Kind = "option"
	KindArg        Kind = "arg"
	// KindBerhenti adalah baris "cukup, pakai yang sudah ada" — bukan kandidat
	// yang berasal dari spec, melainkan tindakan yang ditawarkan UI sendiri.
	KindBerhenti Kind = "berhenti"
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

// TemplateQuery adalah teks yang dipakai menyaring kandidat template.
func (r *Result) TemplateQuery() string {
	if r.Lead != "" {
		return ""
	}
	return r.Prefix
}

// Match mengembalikan teks yang dipakai menyaring kandidat.
func (r *Result) Match() string {
	if r.FilterPrefixSet {
		return r.FilterPrefix
	}
	return r.Prefix
}

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

	// insertValue yang menyebut posisi kursornya sendiri dibiarkan apa adanya:
	// penulisnya sudah menentukan bentuk persisnya, termasuk tanda kutip.
	if i := strings.Index(text, cursorMarker); i >= 0 {
		text = text[:i] + text[i+len(cursorMarker):]
		if seimbang(text) {
			return prefix + text, len(prefix) + i
		}
		// Korpus spec berasal dari pihak ketiga dan bisa cacat: mysql memuat
		// insertValue "{cursor}'", yang menyisakan satu kutip menggantung dan
		// akan menggantung baris perintah pengguna. Dalam keadaan itu nama
		// kandidatnya dipakai apa adanya.
		text = name
	}

	text = prefix + Quote(text)
	return text, len(text)
}

// seimbang menjawab apakah sebuah teks terurai utuh sebagai kata shell.
func seimbang(text string) bool {
	for _, tok := range parser.Parse(text, 0).Tokens {
		if !tok.Terminated {
			return false
		}
	}
	return true
}

// bersihkan membuang karakter kendali dari teks yang akan digambar.
//
// Keterangan berasal dari korpus pihak ketiga dan bisa memuat apa saja: spec
// "ag" menyimpan byte NUL di dalam keterangannya. Menuliskannya apa adanya ke
// terminal merusak kotak yang sedang digambar — baris meleset, bingkai patah,
// dan sisa gambar tertinggal di layar.
func bersihkan(s string) string {
	if strings.IndexFunc(s, unicode.IsControl) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			// Tab dan baris baru menjadi spasi supaya kata tidak menyatu;
			// sisanya dibuang.
			if r == '\t' || r == '\n' || r == '\r' {
				return ' '
			}
			return -1
		}
		return r
	}, s)
}

// perluDikutip menyebut karakter yang mengubah arti sebuah kata di shell.
//
// Garis miring dan tilde sengaja TIDAK termasuk: keduanya justru harus tetap
// bermakna, supaya "~/berkas" tetap menunjuk direktori rumah dan path tetap
// berupa path.
const perluDikutip = " \t\n\"'$`\\|&;<>()*?[]{}!#"

// Quote membungkus teks agar shell memperlakukannya sebagai satu kata.
//
// Tanpa ini, kandidat berisi spasi — nama berkas, dan 131 saran di korpus Fig
// seperti "generate install.sh > install.sh" — akan disisipkan apa adanya dan
// menghasilkan perintah yang rusak. Itu kegagalan yang jauh lebih buruk
// daripada tidak ada kandidat.
func Quote(s string) string {
	if s == "" || !strings.ContainsAny(s, perluDikutip) {
		return s
	}
	// Kutip tunggal mematikan seluruh arti khusus; satu-satunya yang perlu
	// ditangani adalah kutip tunggal itu sendiri.
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// IsDir menjawab apakah kandidat ini sebuah direktori, yaitu sesuatu yang
// masih bisa ditelusuri lebih dalam.
//
// Tidak cukup memeriksa Insert: nama berkas berspasi dikutip, sehingga
// "folder dengan spasi/" disisipkan sebagai "\'folder dengan spasi/\'" dan
// garis miringnya tidak lagi berada di ujung. Memeriksa Insert saja membuat
// folder berspasi kehilangan seluruh perlakuan folder — tidak bisa ditelusuri,
// dan tidak menampilkan petunjuk tombolnya.
// Kedua pemisah diterima. Di Windows kandidat direktori berakhiran "\", dan
// memeriksa "/" saja membuat seluruh perlakuan folder hilang di sana: ikonnya
// salah, panah kanan tidak masuk ke dalamnya, dan penelusuran path berhenti
// di tingkat pertama.
func (c Candidate) IsDir() bool {
	return berakhirPemisah(c.Name) ||
		berakhirPemisah(strings.TrimRight(c.Insert, `'"`))
}

func berakhirPemisah(s string) bool {
	return strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\`)
}

// Result adalah jawaban lengkap engine untuk satu posisi kursor.
type Result struct {
	// Command adalah nama perintah yang sedang dilengkapi. Dibawa keluar
	// karena kebijakan generator memutuskan berdasarkan nama itu.
	Command string `json:"command,omitempty"`
	// Prefix adalah teks yang sudah diketik pada token kursor.
	Prefix string `json:"prefix"`
	// FilterPrefix adalah bagian dari Prefix yang dipakai MENYARING kandidat.
	//
	// Keduanya berbeda saat token kursor memuat lebih dari nilai yang sedang
	// dilengkapi: pada "--output=", yang diketik adalah seluruh token itu,
	// tetapi yang dicocokkan dengan "json" hanyalah bagian sesudah tanda sama
	// dengan. Menyamakan keduanya membuat seluruh kandidat tersaring habis.
	//
	// FilterPrefixSet membedakan "belum ditentukan" dari "memang kosong".
	// Tanpa itu, "--output=" tanpa nilai akan tersaring oleh nama opsinya
	// sendiri dan tidak pernah menampilkan apa pun.
	FilterPrefix    string `json:"filterPrefix,omitempty"`
	FilterPrefixSet bool   `json:"-"`
	// Lead adalah teks yang MENDAHULUI setiap kandidat dinamis saat
	// disisipkan, dan sekaligus menandakan bahwa kandidat template tidak
	// disaring dengan Prefix.
	//
	// Terisi saat nama perintah sudah lengkap tetapi spasinya belum diketik.
	// Mengetik "cd" harus menawarkan isi direktori, dan yang disisipkan adalah
	// "cd proyek/" — bukan "proyek/", karena yang diganti adalah kata "cd"
	// itu sendiri. Tanpa penanda ini daftar foldernya akan disaring dengan
	// "cd" dan selalu kosong.
	Lead string `json:"lead,omitempty"`
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
	res, err := e.hitung(l)
	if err != nil {
		return nil, err
	}

	// Nama berkas berspasi yang belum dikutip sudah dipecah shell menjadi
	// beberapa kata. Bila penggabungannya masuk akal, seluruh perhitungan
	// diulang di atas baris yang sudah disatukan.
	lebar := e.lebarkan(l)
	if lebar == nil {
		return res, nil
	}
	res2, err := e.hitung(lebar)
	if err != nil || !berkasDilengkapi(res2) {
		// Penggabungan hanya sah bila posisi itu memang meminta nama berkas.
		// "git commit -m pesan pan" kebetulan bisa cocok dengan sebuah berkas
		// di disk, dan menggabungkannya di situ hanya merusak.
		return res, nil
	}
	return res2, nil
}

func (e *Engine) hitung(l *parser.Line) (*Result, error) {
	res := &Result{Prefix: l.Prefix}
	res.ReplaceStart, res.ReplaceEnd = replaceRange(l)

	words, _ := l.Words()
	if len(words) > 0 {
		res.Command = words[0].Value
	}
	if len(words) == 0 {
		// Kursor berada di posisi nama PERINTAH — di awal baris, atau sesudah
		// pipa dan titik koma.
		e.saranPerintah(res, l.Prefix)
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
	res.Candidates = dedup(res.Candidates)
	return res, nil
}

// lebarkan memperluas prefix melintasi SPASI yang belum dikutip.
//
// "cd folder de" sudah dipecah shell menjadi dua kata sebelum anjuran melihatnya,
// jadi yang dilengkapi hanya "de" dan nama seperti "folder dengan spasi/"
// tidak pernah muncul. Padahal begitulah orang mengetiknya: tanda kutip
// dipasang belakangan, kalau ingat — dan zsh sendiri melengkapinya.
//
// Perluasan hanya dilakukan bila memang MENGHASILKAN sesuatu: ada entri di
// disk yang berawalan teks gabungan itu. Tanpa syarat itu "ls berkas catatan"
// yang benar-benar dua argumen akan ikut digabung menjadi satu nama yang tidak
// pernah ada.
// Mengembalikan nil bila tidak ada yang layak digabung.
func (e *Engine) lebarkan(l *parser.Line) *parser.Line {
	// Token yang sudah dikutip tidak perlu diperluas, dan memperluasnya justru
	// merusak kutipnya.
	if l.CursorIndex < len(l.Tokens) {
		t := l.Tokens[l.CursorIndex]
		// Token terkutip sudah menyatakan batas namanya sendiri, dan token
		// berawalan minus menyatakan dengan jelas bahwa ini bukan nama berkas.
		if t.Quote != parser.QuoteNone || strings.HasPrefix(t.Value, "-") {
			return nil
		}
	}

	mulai := min(l.CursorIndex, len(l.Tokens))
	terbaik, awal := "", 0
	// Indeks 0 adalah nama perintah; ia tidak pernah ikut digabung.
	for i := mulai - 1; i >= 1; i-- {
		t := l.Tokens[i]
		if t.IsSeparator || t.Quote != parser.QuoteNone || strings.HasPrefix(t.Value, "-") {
			break
		}
		berikut := l.Cursor
		if i+1 < len(l.Tokens) {
			berikut = l.Tokens[i+1].Start
		}
		// Tepat satu spasi pemisah. Apa pun selain itu bukan nama berkas yang
		// terpecah, melainkan dua kata yang memang berbeda.
		if t.End > berikut || berikut > len(l.Raw) || l.Raw[t.End:berikut] != " " {
			break
		}
		if gabung := l.Raw[t.Start:l.Cursor]; e.adaBerawalan(gabung) {
			terbaik, awal = gabung, t.Start
		}
	}
	if terbaik == "" {
		return nil
	}

	// Token-token yang digabung diganti satu token tunggal yang mencakup
	// seluruh rentangnya, sehingga sisa engine melihat satu nama berkas —
	// persis seperti bila pengguna mengutipnya sejak awal.
	gabungan := parser.Token{
		Value: terbaik,
		Start: awal,
		End:   l.Tokens[mulai-1].End,
	}
	if mulai < len(l.Tokens) {
		gabungan.End = l.Tokens[mulai].End
	}
	var tokens []parser.Token
	for i, t := range l.Tokens {
		if t.Start >= awal && i <= mulai {
			continue
		}
		tokens = append(tokens, t)
	}
	sisip := len(tokens)
	for i, t := range tokens {
		if t.Start > gabungan.Start {
			sisip = i
			break
		}
	}
	tokens = append(tokens[:sisip], append([]parser.Token{gabungan}, tokens[sisip:]...)...)

	return &parser.Line{
		Raw:         l.Raw,
		Cursor:      l.Cursor,
		Tokens:      tokens,
		CursorIndex: sisip,
		Prefix:      terbaik,
	}
}

// berkasDilengkapi menjawab apakah posisi ini memang meminta nama berkas.
func berkasDilengkapi(res *Result) bool {
	for _, t := range res.Templates {
		if t == "filepaths" || t == "folders" {
			return true
		}
	}
	return false
}

// adaBerawalan menjawab apakah ada entri di disk yang berawalan teks ini.
func (e *Engine) adaBerawalan(prefix string) bool {
	dir, base := filepath.Split(prefix)
	akar := dir
	if !filepath.IsAbs(akar) {
		akar = filepath.Join(e.Dir, dir)
	}
	entri, err := os.ReadDir(akar)
	if err != nil {
		return false
	}
	base = strings.ToLower(base)
	for _, en := range entri {
		if strings.HasPrefix(strings.ToLower(en.Name()), base) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// saranPerintah melengkapi nama perintah, lalu langsung menawarkan isi
// perintah itu begitu namanya lengkap.
//
// Diam di posisi ini adalah perbedaan paling terasa antara alat ini dan sebuah
// editor: di editor, mengetik nama fungsi langsung memunculkan daftarnya, dan
// begitu namanya lengkap ia menunjukkan apa yang bisa dilakukan dengannya.
// Mengharuskan pengguna menekan spasi lebih dulu hanya untuk melihat "git bisa
// apa" membuat pengetahuan 716 spec itu tersembunyi di balik satu tombol yang
// harus ditebak.
func (e *Engine) saranPerintah(res *Result, prefix string) {
	if prefix == "" {
		res.Templates = append(res.Templates, "commands")
		return
	}

	root, err := e.registry.Load(prefix)
	if err != nil || root == nil {
		// Nama yang belum lengkap, atau perintah tanpa spec: yang ditawarkan
		// adalah nama perintah lain yang berawalan sama.
		res.Templates = append(res.Templates, "commands")
		return
	}
	if root, err = e.registry.Resolve(root); err != nil || root == nil {
		res.Templates = append(res.Templates, "commands")
		return
	}

	// Namanya cocok PERSIS dengan sebuah perintah yang dikenal, jadi kata itu
	// sudah selesai: yang ditawarkan adalah isi perintah itu, bukan nama
	// perintah lain yang kebetulan berawalan sama. Begitu pula editor —
	// begitu sebuah nama terselesaikan, yang ditampilkan adalah isinya.
	res.Lead = prefix + " "

	// Nama perintahnya sudah selesai diketik, jadi tidak ada lagi yang perlu
	// disaring: seluruh isinya ditampilkan. Tanpa ini kandidatnya disaring
	// dengan "git", dan karena SETIAP subcommand ikut membawa awalan itu,
	// panjang nama menjadi satu-satunya pembeda — "git mv" dan "git rm" naik
	// ke puncak sementara "git commit" terlempar ke bawah.
	res.FilterPrefix, res.FilterPrefixSet = "", true

	for i := range root.Subcommands {
		sub := &root.Subcommands[i]
		if sub.Hidden || sub.Deprecated || !e.available(sub.WhenFile) {
			continue
		}
		for _, name := range sub.Name {
			// Ditampilkan sebagai "commit", disisipkan sebagai "git commit":
			// yang diganti adalah kata perintahnya, jadi nama perintah harus
			// ikut dibawa — tetapi mengulanginya di layar hanya kebisingan,
			// sebagaimana editor menampilkan anggota tanpa mengulang nama
			// objeknya.
			insert := prefix + " " + name
			res.Candidates = append(res.Candidates, Candidate{
				Name:         name,
				Insert:       insert,
				CursorOffset: len(insert),
				Description:  bersihkan(sub.Description),
				Kind:         KindSubcommand,
				Priority:     priorityOr(sub.Priority),
				Dangerous:    sub.IsDangerous,
			})
			break
		}
	}

	// Perintah tanpa subcommand — cd, ls, cat — isinya adalah argumennya.
	// Mengetik "cd" harus langsung menawarkan direktori; menunggu spasi lebih
	// dulu berarti pengetahuan itu tetap tersembunyi di balik satu tombol.
	if len(res.Candidates) == 0 && len(root.Args) > 0 {
		e.addArg(res, &root.Args[0], "", res.Lead)
	}
	if len(res.Candidates) == 0 && len(res.Templates) == 0 && len(res.Generators) == 0 {
		// Tidak ada yang bisa ditawarkan dari perintah itu sendiri; kembali
		// menawarkan nama perintah.
		//
		// Generator ikut dihitung: host ssh datang dari sana, bukan dari
		// template maupun suggestion, dan melupakannya membuat "ssh" jatuh
		// kembali ke daftar nama perintah padahal ada jawaban yang lebih baik.
		res.Lead = ""
		res.FilterPrefix, res.FilterPrefixSet = "", false
		res.Templates = append(res.Templates, "commands")
	}
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
			if opt == nil {
				opt = st.lookupBundle(tok)
			}
			if opt != nil {
				for _, n := range opt.Name {
					st.usedOptions[n] = true
				}
			}

			// Bentuk --opt=value sudah membawa argumennya sendiri, begitu
			// pula opsi yang memang mewajibkan nilainya menempel.
			if opt != nil && !strings.Contains(tok, "=") && !opt.RequiresSeparator.Required {
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

// lookupBundle memecah opsi pendek yang ditulis bergabung.
//
// "tar -xf berkas.tar" berarti -x dan -f, dan -f-lah yang menerima nama
// berkasnya. Tanpa pemecahan ini seluruh "-xf" dianggap satu opsi yang tidak
// dikenal, sehingga argumennya tidak pernah dilengkapi — padahal bentuk
// bergabung justru yang paling lazim dipakai orang.
//
// Mengembalikan opsi TERAKHIR dalam gabungan, karena hanya yang terakhir yang
// bisa menerima argumen. Nil bila ada satu huruf pun yang tidak dikenali;
// menebak sebagian hanya akan salah menelan token berikutnya.
func (st *state) lookupBundle(tok string) *spec.Option {
	if len(tok) < 3 || tok[0] != '-' || tok[1] == '-' {
		return nil
	}

	var last *spec.Option
	for _, r := range tok[1:] {
		o := st.lookupOption("-" + string(r))
		if o == nil {
			return nil
		}
		for _, n := range o.Name {
			st.usedOptions[n] = true
		}
		last = o
	}
	return last
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
			res.FilterPrefix, res.FilterPrefixSet = valPrefix, true
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
					Description:  bersihkan(sub.Description),
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

// dedup membuang kandidat kembar.
//
// Satu opsi bisa muncul dua kali karena terdaftar di subcommand sekaligus
// diwarisi sebagai opsi persisten. Di layar itu terlihat sebagai dua baris
// identik, dan panah terasa macet: menekan bawah tidak mengubah apa pun.
func dedup(cands []Candidate) []Candidate {
	seen := make(map[string]bool, len(cands))
	out := cands[:0]
	for _, c := range cands {
		key := string(c.Kind) + "\x00" + c.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
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
				if insertValue == "" && o.RequiresSeparator.Required && len(o.Args) > 0 {
					insertValue = name + o.RequiresSeparator.Rune() + cursorMarker
				}
				insert, offset := makeInsert(name, insertValue, "")
				res.Candidates = append(res.Candidates, Candidate{
					Name:         name,
					Display:      o.DisplayName,
					Insert:       insert,
					CursorOffset: offset,
					Description:  bersihkan(o.Description),
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
					Description:  bersihkan(s.Description),
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
