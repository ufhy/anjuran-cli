package ui

import (
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/parser"
	"github.com/ufhy/anjuran-cli/internal/recall"
)

// Outcome membedakan tiga akhir sesi yang perlu ditangani shell secara
// berbeda. Membatalkan dan tidak-ada-kandidat tampak sama di layar, tetapi
// pada kasus kedua shell sebaiknya jatuh kembali ke completion bawaannya.
type Outcome int

const (
	// Accepted berarti pengguna memilih sebuah kandidat.
	Accepted Outcome = iota
	// Cancelled berarti pengguna menekan Esc atau Ctrl-C.
	Cancelled
	// NoCandidates berarti tidak ada yang bisa ditawarkan sama sekali.
	NoCandidates
)

// Preflight adalah hasil perhitungan kandidat SEBELUM terminal disentuh.
//
// Pemisahan ini bukan sekadar kerapian: sebagian besar penekanan tombol
// berakhir tanpa dropdown sama sekali (nol atau satu kandidat). Menghitung
// lebih dulu berarti kasus-kasus itu tidak pernah masuk mode raw — pada sesi
// SSH, itu beberapa round-trip yang tidak jadi terjadi setiap kali Tab.
type Preflight struct {
	eng    *engine.Engine
	dyn    Dynamic
	recall Recall
	res    *engine.Result
	rs     []ranked
	st     State
	manual bool
}

// Recall mengingat kandidat yang pernah dipilih pengguna.
//
// Interface, bukan tipe konkret, supaya seluruh pengujian sesi bisa berjalan
// tanpa menyentuh disk — dan supaya lupa sama sekali tetap menjadi keadaan
// yang sah, bukan kegagalan.
type Recall interface {
	Preferred(key string) string
	Record(key, name string)
}

// recallKey merangkai kunci ingatan untuk sebuah hasil.
func recallKey(res *engine.Result) string {
	if res == nil {
		return ""
	}
	return recall.Key(res.Command, res.Prefix)
}

// Prepare menghitung dan memeringkat kandidat untuk sebuah state.
//
// dyn boleh nil; tanpa itu hanya kandidat statis dari spec yang dipakai.
func Prepare(eng *engine.Engine, st State, dyn Dynamic, rec Recall) (*Preflight, error) {
	res, err := eng.Complete(st.Line, st.Cursor)
	if err != nil {
		return nil, err
	}

	if dyn != nil {
		// Kandidat dinamis ditambahkan SETELAH yang statis, lalu penyaringan
		// dan pemeringkatan berjalan atas keduanya sekaligus. Dengan begitu
		// nama branch dan nama subcommand bersaing dengan aturan yang sama,
		// bukan tampil sebagai dua daftar terpisah.
		res.Candidates = append(res.Candidates, dyn.Candidates(res)...)
	}

	p := &Preflight{eng: eng, dyn: dyn, recall: rec, res: res, st: st}
	p.rs = filter(res.Candidates, res.Match(), p.preferred(res))
	return p, nil
}

// preferred mengembalikan kandidat yang terakhir dipilih untuk konteks ini.
func (p *Preflight) preferred(res *engine.Result) string {
	if p.recall == nil {
		return ""
	}
	return p.recall.Preferred(recallKey(res))
}

// Candidates mengembalikan kandidat dalam urutan yang akan ditampilkan,
// sudah disaring terhadap prefix. Dipakai oleh mode pemeriksaan agar yang
// terlihat di sana sama persis dengan yang muncul saat Tab ditekan.
func (p *Preflight) Candidates() []engine.Candidate {
	out := make([]engine.Candidate, len(p.rs))
	for i, r := range p.rs {
		out[i] = r.cand
	}
	return out
}

// Prefix adalah teks yang sudah diketik pada token kursor.
func (p *Preflight) Prefix() string {
	if p.res == nil {
		return ""
	}
	return p.res.Prefix
}

// Manual menandakan pengguna meminta completion dengan menekan tombolnya
// sendiri, bukan sekadar mengetik karakter pemicu.
func (p *Preflight) Manual(v bool) *Preflight {
	p.manual = v
	return p
}

// Immediate menangani kasus yang tidak memerlukan terminal. Nilai ketiga
// bernilai false bila dropdown memang harus ditampilkan.
//
// manual menandakan pengguna MEMINTA completion — menekan Tab — dan bukan
// sekadar mengetik karakter pemicu. Bedanya menentukan: menekan Tab dengan
// satu kandidat memang berarti "sisipkan itu", sedangkan mengetik spasi tidak
// pernah berarti demikian. Menyisipkan sendiri di sana mengubah baris perintah
// tanpa diminta, dan karakter yang diketik sesudahnya menempel di tempat yang
// salah.
func (p *Preflight) Immediate(manual bool) (State, Outcome, bool) {
	p.manual = manual
	switch {
	case len(p.rs) == 0:
		return p.st, NoCandidates, true
	case len(p.rs) == 1 && manual:
		if p.recall != nil {
			p.recall.Record(recallKey(p.res), p.rs[0].cand.Name)
		}
		return apply(p.st, p.res, p.rs[0].cand), Accepted, true
	}
	return p.st, Cancelled, false
}

// NoSelection membuka sesi TANPA ada yang tersorot.
//
// Dipakai saat menelusuri ke dalam sebuah direktori: isinya ditampilkan supaya
// pengguna bisa melihat ada apa di sana, tetapi tidak ada yang dipilihkan
// untuknya. Tanpa keadaan ini, menelusuri selalu menyorot anak pertama, dan
// Enter — satu-satunya cara berhenti — justru turun satu tingkat lagi.
const NoSelection = -2

// Leftover adalah tombol yang mengakhiri sesi tetapi BUKAN urusan sesi.
//
// Dikembalikan ke shell alih-alih ditelan. Sebelumnya setiap tombol yang tidak
// dikenali sesi hilang tanpa jejak — Ctrl-A, Home, panah kiri — dan pengguna
// merasakannya sebagai tombol yang kadang tidak berfungsi. Inilah yang membuat
// sesi boleh memegang seluruh interaksi tanpa merampas apa pun dari shell.
type Leftover []byte

// Session menjalankan interaksi dropdown untuk satu penekanan tombol pelengkap.
type Session struct {
	eng    *engine.Engine
	dyn    Dynamic
	recall Recall
	term   Terminal
	rend   *Renderer

	st State
	// typable menandakan gema karakter aman dilakukan. Sesi hanya bisa
	// menerima ketikan baru bila kursor berada di ujung baris; bila tidak,
	// karakter yang digemakan akan muncul di tempat yang salah karena baris
	// prompt dimiliki shell, bukan kita.
	typable bool

	// res dan rs berisi hasil preflight bila sesi dibangun lewat Preflight,
	// sehingga kandidat tidak dihitung ulang saat Run.
	res      *engine.Result
	rs       []ranked
	prepared bool

	// start adalah baris yang tersorot saat sesi dibuka. Bernilai -1 berarti
	// baris terakhir — dipakai saat sesi dibuka dengan panah ATAS, supaya
	// arah tekanannya terasa sebagaimana mestinya.
	start int

	// awal adalah bagian baris yang TERAKHIR digambar shell.
	//
	// Sesi tidak memiliki baris prompt dan tidak bisa menggambar ulang
	// seluruhnya; yang bisa dilakukannya hanyalah mengganti ekor sesudah
	// bagian ini. Menyusut bila pengguna menghapus sampai melewatinya.
	awal string

	// leftover menampung tombol yang harus dikembalikan ke shell.
	leftover Leftover

	// sisaDidukung menandai shell pemanggil bisa menerima tombol yang
	// dikembalikan. Hanya zsh yang bisa; di shell lain sisa itu hilang, jadi
	// huruf yang terbaca disisipkan ke dalam baris alih-alih dikembalikan.
	sisaDidukung bool

	// manual menandakan sesi dibuka karena pengguna menekan tombol completion,
	// bukan karena mengetik karakter pemicu.
	manual bool
}

// Leftover mengembalikan tombol yang belum ditangani sesi, bila ada.
func (s *Session) Leftover() Leftover { return s.leftover }

// selesai mengakhiri sesi, mengembalikan tombol yang mengakhirinya BESERTA
// seluruh ketikan yang sudah telanjur terbaca.
//
// Ketikan cepat dan tempelan teks tiba dalam satu bongkahan. Tanpa
// mengembalikannya, karakter yang menyusul di bongkahan yang sama hilang
// bersama proses ini — dan itu terasa sebagai karakter yang kadang tidak
// muncul, kegagalan yang jauh lebih mengganggu daripada dropdown yang menutup.
func (s *Session) selesai(raw []byte, st State, out Outcome) (State, Outcome, error) {
	sisa := append(append([]byte(nil), raw...), s.term.Drain()...)

	// Shell yang tidak bisa menerima tombol kembali mendapat perlakuan lain.
	//
	// Hanya zsh punya cara mengembalikan tombol ke antrean masukannya
	// (`zle -U`). Di bash, fish, dan PowerShell, apa pun yang dikembalikan
	// HILANG — dan yang hilang adalah huruf yang baru saja diketik orangnya.
	// Mengetik "git zzqq" cepat-cepat berakhir menjadi "git z".
	//
	// Huruf yang bisa diselamatkan disisipkan langsung ke dalam baris, karena
	// di situlah tempatnya seharusnya. Tombol kendali memang tidak bisa
	// diselamatkan dengan cara ini, dan dibuang — tetapi kehilangan satu
	// Ctrl-A jauh lebih ringan daripada kehilangan ketikan.
	if !s.sisaDidukung {
		st = s.sisipkanTerbaca(st, sisa)
		s.leftover = nil
		return st, out, nil
	}

	s.leftover = sisa
	return st, out, nil
}

// sisipkanTerbaca menyisipkan huruf yang bisa dicetak dari sisa masukan.
func (s *Session) sisipkanTerbaca(st State, sisa []byte) State {
	var b strings.Builder
	for _, r := range string(sisa) {
		// Tombol kendali dan escape sequence tidak punya wujud di dalam baris
		// perintah; memasukkannya justru mengotori perintah yang akan
		// dijalankan.
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return st
	}

	c := st.Cursor
	if c < 0 || c > len(st.Line) {
		c = len(st.Line)
	}
	st.Line = st.Line[:c] + b.String() + st.Line[c:]
	st.Cursor = c + b.Len()
	return st
}

// DukungSisa menandai bahwa shell pemanggil bisa menerima tombol yang
// dikembalikan. Bawaannya tidak: hanya zsh yang mampu.
func (s *Session) DukungSisa(v bool) *Session {
	s.sisaDidukung = v
	return s
}

// StartAt menentukan baris yang tersorot saat sesi dibuka.
func (s *Session) StartAt(i int) *Session {
	s.start = i
	return s
}

// Session membangun sesi interaktif dari hasil preflight, sehingga kandidat
// tidak dihitung dua kali.
func (p *Preflight) Session(term Terminal, rend *Renderer) *Session {
	return &Session{
		start:    0,
		awal:     p.st.Line,
		manual:   p.manual,
		eng:      p.eng,
		dyn:      p.dyn,
		recall:   p.recall,
		term:     term,
		rend:     rend,
		st:       p.st,
		typable:  p.st.Cursor == len(p.st.Line),
		res:      p.res,
		rs:       p.rs,
		prepared: true,
	}
}

// NewSession menyiapkan sesi tanpa preflight; kandidat dihitung saat Run.
func NewSession(eng *engine.Engine, term Terminal, rend *Renderer, st State) *Session {
	return &Session{
		eng:     eng,
		term:    term,
		rend:    rend,
		st:      st,
		awal:    st.Line,
		typable: st.Cursor == len(st.Line),
	}
}

// Run menampilkan dropdown dan mengembalikan state akhir baris perintah.
func (s *Session) Run() (State, Outcome, error) {
	res, rs := s.res, s.rs
	if !s.prepared {
		var err error
		res, rs, err = s.recompute()
		if err != nil {
			return s.st, Cancelled, err
		}
	}

	switch {
	case len(rs) == 0:
		return s.st, NoCandidates, nil
	case len(rs) == 1 && s.manual && s.start != NoSelection:
		// Menekan tombol completion dengan satu kandidat memang berarti
		// "sisipkan itu". Mengetik karakter pemicu tidak pernah berarti
		// demikian, jadi di sana kandidatnya ditampilkan, bukan disisipkan.
		s.ingat(res, rs[0].cand)
		return apply(s.st, res, rs[0].cand), Accepted, nil
	}

	// Baris "berhenti" ditawarkan secara TERLIHAT saat kotak dibuka tanpa
	// sorotan sesudah menelusuri sebuah direktori.
	//
	// Sebelumnya berhenti memang bisa — Enter tanpa sorotan menerima baris apa
	// adanya — tetapi tidak ada apa pun di layar yang mengatakannya. Yang
	// terlihat pengguna hanyalah daftar isi direktori tanpa penanda, dan tidak
	// ada cara menduga bahwa Enter berarti "cukup". Kini ia sebuah baris yang
	// bisa dilihat dan dipilih, lengkap dengan ikon tombolnya.
	//
	// Yang disisipkannya adalah teks yang MEMANG SUDAH ADA di posisi itu,
	// sehingga memilihnya benar-benar tidak mengubah apa pun — dan seluruh
	// jalur Enter, Tab, serta penyisipan lain bekerja tanpa perlakuan khusus.
	if s.start == NoSelection && strings.TrimSpace(s.st.Line) != "" && len(rs) > 0 {
		teks := ""
		if res.ReplaceStart >= 0 && res.ReplaceEnd <= len(s.st.Line) && res.ReplaceStart <= res.ReplaceEnd {
			teks = s.st.Line[res.ReplaceStart:res.ReplaceEnd]
		}
		rs = append([]ranked{{cand: engine.Candidate{
			// Ikon saja, tanpa kalimat: kolom ini milik isi direktori, dan
			// satu baris teks penjelas di tengahnya justru mengganggu
			// pembacaan daftar. Tombolnya sendiri sudah menjelaskan
			// tindakannya, sebagaimana "→ ⏎" pada baris folder.
			Name:         "\u23ce",
			Insert:       teks,
			CursorOffset: len(teks),
			Kind:         engine.KindBerhenti,
		}}}, rs...)
		s.start = 0
	}

	selected := s.start
	switch {
	case s.start == NoSelection:
		selected = -1
	case selected < 0:
		selected = len(rs) - 1
	case selected >= len(rs):
		selected = 0
	}
	defer func() {
		// Urutannya penting: gema dihapus lebih dulu, baru kotaknya, supaya
		// kursor kembali tepat ke tempat shell terakhir meninggalkannya.
		s.rend.UnEcho()
		s.rend.Clear()
	}()

	for {
		if err := s.draw(rs, selected); err != nil {
			return s.st, Cancelled, err
		}

		key, err := s.term.ReadKey()
		if err == io.EOF {
			return s.st, Cancelled, nil
		}
		if err != nil {
			return s.st, Cancelled, err
		}

		switch key.Type {
		case KeyEscape:
			// Esc membatalkan SARAN, bukan ketikan.
			//
			// Karakter yang diketik pengguna di dalam sesi tetap miliknya;
			// membuangnya berarti Esc menghapus pekerjaan yang baru saja
			// dilakukan. Karena itu hasilnya dikembalikan sebagai Accepted
			// dengan baris apa adanya — yang dibatalkan hanya kotaknya.
			return s.selesai(nil, s.st, Accepted)

		case KeyCtrlC, KeyCtrlD:
			// Keduanya punya arti bagi shell — membatalkan baris, menutup
			// sesi — jadi diteruskan alih-alih ditelan.
			return s.selesai(key.Raw, s.st, Cancelled)

		case KeyEnter:
			// Enter MENERIMA lalu menutup, selalu.
			//
			// Sempat dibuat menelusuri lebih dalam saat yang dipilih sebuah
			// direktori, dan itu keliru: Enter tidak pernah sampai ke shell,
			// sehingga "cd proyek/" tidak bisa dijalankan sama sekali —
			// setiap Enter hanya turun satu tingkat lagi. Telusur lanjut
			// dilakukan dengan Tab, yang memang berarti "lengkapi lagi".
			// Tidak ada yang tersorot berarti pengguna sudah puas dengan
			// barisnya sendiri — inilah cara berhenti setelah menelusuri ke
			// dalam sebuah direktori.
			if selected < 0 {
				return s.selesai(nil, s.st, Accepted)
			}
			cand := rs[selected].cand
			s.ingat(res, cand)
			s.st = apply(s.st, res, cand)
			return s.st, Accepted, nil

		case KeyTab:
			// Tab menyisipkan AWALAN TERPANJANG YANG SAMA lebih dulu.
			//
			// Mengetik "git com" lalu Tab seharusnya langsung membawa ke
			// "git commit" bila seluruh kandidat yang tersisa berawalan sama —
			// itu perilaku Tab yang sudah dikenal orang dari shell mana pun,
			// dan tanpanya Tab hanya terasa seperti panah bawah.
			if ext := awalanBersama(rs, res.Match()); ext != "" {
				s.st = apply(s.st, res, engine.Candidate{
					Name: ext, Insert: ext, CursorOffset: len(ext), Kind: engine.KindArg,
				})
				if err := s.gambarUlang(); err != nil {
					return s.st, Cancelled, err
				}
				if res, rs, selected, err = s.refresh(); err != nil {
					return s.st, Cancelled, err
				}
				if len(rs) == 0 {
					return s.selesai(nil, s.st, Accepted)
				}
				continue
			}
			// Satu kandidat: Tab berarti "lengkapi itu". Memutar pilihan di
			// situ tidak mengubah apa pun dan terasa seperti tombol yang mati.
			if len(rs) == 1 {
				cand := rs[0].cand
				s.ingat(res, cand)
				s.st = apply(s.st, res, cand)
				if err := s.gambarUlang(); err != nil {
					return s.st, Cancelled, err
				}

				// Direktori dibuka isinya: Tab memang berarti "lengkapi lagi",
				// dan ini permintaan eksplisit — berbeda dari Enter, yang harus
				// menutup supaya perintahnya bisa dijalankan.
				if cand.IsDir() {
					if res, rs, err = s.recompute(); err != nil {
						return s.st, Cancelled, err
					}
					if len(rs) > 0 {
						// Isinya ditampilkan, tidak dipilihkan: Tab lagi untuk
						// masuk ke daftarnya, Enter untuk berhenti di sini.
						selected = -1
						continue
					}
				}
				return s.st, Accepted, nil
			}
			selected = (selected + 1) % len(rs)

		case KeyDown:
			selected = (selected + 1) % len(rs)

		case KeyUp, KeyShiftTab:
			if selected < 0 {
				selected = len(rs) - 1
				break
			}
			selected = (selected - 1 + len(rs)) % len(rs)

		case KeyPageDown:
			selected = min(max(selected, 0)+s.rend.MaxRows(), len(rs)-1)

		case KeyPageUp:
			selected = max(selected-s.rend.MaxRows(), 0)

		case KeyRune:
			// Spasi MENGETIK SPASI, bukan menerima pilihan.
			//
			// Sempat dibuat menerima kandidat yang sedang tersorot, dan itu
			// mengejutkan: pengguna mengetik spasi untuk melanjutkan kalimat
			// perintahnya, bukan untuk memilih sesuatu yang kebetulan berada di
			// baris teratas. Menerima harus selalu berupa tindakan yang
			// disengaja — Enter atau Tab.

			if !s.typable {
				return s.selesai(key.Raw, s.st, Accepted)
			}
			s.insert(key.Rune)
			if err := s.rend.EchoRune(key.Rune); err != nil {
				return s.st, Cancelled, err
			}

			// Backslash di ujung adalah pelolosan yang BELUM SELESAI. Menyaring
			// ulang di situ selalu menghasilkan nol kandidat — tidak ada nama
			// berkas yang berakhir dengan backslash — sehingga sesi menutup
			// tepat sebelum karakter yang dilolos sempat diketik.
			if hitungBackslash(s.st.Line[:s.st.Cursor])%2 == 1 {
				continue
			}

			if res, rs, selected, err = s.refresh(); err != nil {
				return s.st, Cancelled, err
			}
			if len(rs) == 0 {
				// Tidak ada lagi yang cocok. Sesi ditutup, tetapi karakter
				// yang sudah diketik tetap dibawa — itu milik pengguna.
				return s.selesai(nil, s.st, Accepted)
			}

		case KeyBackspace:
			// Backspace MENUTUP kotaknya, lalu tombolnya dikembalikan ke shell.
			//
			// Menghapus adalah cara pengguna mundur dari apa yang sedang
			// ditawarkan; menyaring ulang di situ menahan kotak tetap terbuka
			// justru saat ia sedang berusaha menyingkirkannya. Kotaknya akan
			// muncul lagi pada spasi berikutnya, atau dengan Tab.
			//
			// Karakternya dihapus SHELL, bukan kami: shell yang memiliki baris
			// prompt, sehingga ia menggambarnya ulang dengan benar tanpa kami
			// perlu menghitung kolom sama sekali. Seluruh pembukuan gema untuk
			// menghapus — berikut jaring pengaman agar tidak menembus prompt —
			// menjadi tidak diperlukan bersama ini.
			//
			// Itu hanya berlaku bila shell-nya BISA menerima tombol kembali.
			// Di bash, fish, dan PowerShell backspace yang dikembalikan hilang,
			// sehingga menghapus terasa melewatkan satu tekanan: "cd " yang
			// dihapus tiga kali menyisakan "c". Di sana anjuran menghapusnya
			// sendiri dan menyerahkan baris yang sudah benar.
			if !s.sisaDidukung {
				st := s.st
				if st.Cursor > 0 {
					_, lebar := utf8.DecodeLastRuneInString(st.Line[:st.Cursor])
					st.Line = st.Line[:st.Cursor-lebar] + st.Line[st.Cursor:]
					st.Cursor -= lebar
				}
				return s.selesai(nil, st, Accepted)
			}
			return s.selesai(key.Raw, s.st, Accepted)

		case KeyRight:
			// Panah kanan MASUK ke dalam folder yang sedang tersorot.
			//
			// Folder punya dua tindakan, dan memisahkannya ke dua tombol
			// membuat keduanya bisa dipakai tanpa memilih: → untuk melihat isi
			// lebih dalam, Enter untuk berhenti dan memakai path itu. Ikon di
			// tepi kanan baris memberi tahu keduanya ada.
			if selected >= 0 && rs[selected].cand.IsDir() {
				cand := rs[selected].cand
				s.ingat(res, cand)
				s.st = apply(s.st, res, cand)
				if err := s.gambarUlang(); err != nil {
					return s.st, Cancelled, err
				}
				if res, rs, err = s.recompute(); err != nil {
					return s.st, Cancelled, err
				}
				if len(rs) > 0 {
					selected = -1
					continue
				}
				return s.st, Accepted, nil
			}
			// Di baris yang bukan folder, → hanya menggeser kursor.
			return s.geserKursor(+1)

		case KeyLeft:
			// Pergerakan kursor DIKERJAKAN di sini, bukan diteruskan.
			//
			// Dulu tombolnya dikembalikan ke shell lewat Leftover. Itu hanya
			// pernah bekerja di zsh, karena hanya zsh punya `zle -U` untuk
			// menerima tombol yang dikembalikan; di bash, fish, dan PowerShell
			// tombolnya tertelan dan terasa seperti keyboard yang kadang mati.
			// Mengerjakannya di sini membuat satu jalur yang sama untuk
			// keempatnya, dan baris yang dikembalikan sudah memuat kursor yang
			// benar — sesuatu yang setiap shell memang sudah tahu cara memakai.
			return s.geserKursor(-1)

		case KeyHome:
			return s.pindahKursor(0)

		case KeyEnd:
			return s.pindahKursor(len(s.st.Line))

		default:
			// Tombol yang tidak ditangani menutup dropdown, lalu DIKEMBALIKAN
			// ke shell. Menelannya membuat tombol seperti Ctrl-A atau Home
			// terasa kadang tidak berfungsi.
			return s.selesai(key.Raw, s.st, Accepted)
		}
	}
}

// gambarUlang menampilkan baris yang baru saja diubah SESI sendiri.
//
// Hanya ekor sesudah bagian yang terakhir digambar shell yang bisa diganti;
// bila baris barunya tidak lagi berawalan itu, tidak ada yang bisa dikerjakan
// tanpa menggambar ulang prompt — dan prompt itu milik shell.
func (s *Session) gambarUlang() error {
	if !s.typable || !strings.HasPrefix(s.st.Line, s.awal) {
		return nil
	}
	return s.rend.EchoLine(s.st.Line[len(s.awal):])
}

// awalanBersama mencari awalan yang dimiliki SELURUH kandidat dan lebih
// panjang daripada yang sudah diketik.
//
// Mengembalikan string kosong bila tidak ada tambahan yang bisa disisipkan —
// di situ Tab kembali berperan memindahkan pilihan.
func awalanBersama(rs []ranked, prefix string) string {
	if len(rs) < 2 {
		return ""
	}

	common := rs[0].cand.Name
	for _, r := range rs[1:] {
		common = awalanDua(common, r.cand.Name)
		if len(common) <= len(prefix) {
			return ""
		}
	}
	// Harus benar-benar memperpanjang apa yang sudah diketik, dan tetap
	// konsisten dengannya: kandidat dicocokkan secara fuzzy, jadi awalan
	// bersama belum tentu diawali teks yang diketik.
	if len(common) <= len(prefix) || !strings.EqualFold(common[:len(prefix)], prefix) {
		return ""
	}
	return common
}

// awalanDua mengembalikan awalan bersama dua teks, mengabaikan besar-kecil
// huruf saat membandingkan tetapi mempertahankan bentuk aslinya.
func awalanDua(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && unicode.ToLower(rune(a[i])) == unicode.ToLower(rune(b[i])) {
		i++
	}
	return a[:i]
}

// spasiLiteral menjawab apakah spasi di posisi ini adalah BAGIAN DARI KATA,
// bukan pemisah kata.
//
// "cat \"berkas d" masih berada di dalam kutip yang belum ditutup, dan
// "cat berkas\ d" baru saja dilolos dengan backslash. Di kedua tempat itu
// spasi tidak mengakhiri apa pun — memperlakukannya sebagai pemicu akan
// menerima kandidat yang sedang tersorot dan merusak nama yang sedang diketik.
func (s *Session) spasiLiteral() bool {
	// Backslash tepat sebelum kursor melolos karakter berikutnya, kecuali
	// backslash itu sendiri sudah dilolos.
	if n := hitungBackslash(s.st.Line[:s.st.Cursor]); n%2 == 1 {
		return true
	}

	l := parser.Parse(s.st.Line, s.st.Cursor)
	if l.CursorIndex < len(l.Tokens) {
		t := l.Tokens[l.CursorIndex]
		return t.Quote != parser.QuoteNone && !t.Terminated
	}
	return false
}

// hitungBackslash menghitung backslash beruntun di ujung teks.
func hitungBackslash(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n
}

// refresh menghitung ulang kandidat setelah baris berubah.
func (s *Session) refresh() (*engine.Result, []ranked, int, error) {
	res, rs, err := s.recompute()
	return res, rs, 0, err
}

func (s *Session) recompute() (*engine.Result, []ranked, error) {
	res, err := s.eng.Complete(s.st.Line, s.st.Cursor)
	if err != nil {
		return nil, nil, err
	}
	if s.dyn != nil {
		res.Candidates = append(res.Candidates, s.dyn.Candidates(res)...)
	}
	preferred := ""
	if s.recall != nil {
		preferred = s.recall.Preferred(recallKey(res))
	}
	return res, filter(res.Candidates, res.Match(), preferred), nil
}

// ingat mencatat pilihan pengguna untuk konteks tempat ia memilihnya.
func (s *Session) ingat(res *engine.Result, c engine.Candidate) {
	// Baris "berhenti" bukan pilihan yang layak diingat: ia tindakan UI, bukan
	// kandidat yang berasal dari spec.
	if c.Kind == engine.KindBerhenti {
		return
	}
	if s.recall != nil {
		s.recall.Record(recallKey(res), c.Name)
	}
}

// insert menyisipkan satu rune di posisi kursor.
func (s *Session) insert(r rune) {
	s.st.Line = s.st.Line[:s.st.Cursor] + string(r) + s.st.Line[s.st.Cursor:]
	s.st.Cursor += len(string(r))
}

// deleteBack menghapus satu rune sebelum kursor.
func (s *Session) deleteBack() {
	if s.st.Cursor == 0 {
		return
	}
	// Mundur sampai awal rune, bukan sekadar satu byte.
	i := s.st.Cursor - 1
	for i > 0 && s.st.Line[i]&0xC0 == 0x80 {
		i--
	}
	s.st.Line = s.st.Line[:i] + s.st.Line[s.st.Cursor:]
	s.st.Cursor = i
}

// draw menggambar potongan daftar yang terlihat.
func (s *Session) draw(rs []ranked, selected int) error {
	rows := s.rend.MaxRows()
	// selected < 0 berarti belum ada yang dipilih: daftar digambar dari awal
	// tanpa sorotan, dan penghitung hanya melaporkan jumlahnya.
	start, rel := window(len(rs), max(selected, 0), rows)
	if selected < 0 {
		rel = -1
	}
	end := min(start+rows, len(rs))
	// rel menyorot baris di layar; selected+1 melaporkan posisi sebenarnya.
	return s.rend.Render(items(rs[start:end], s.rend.ikon), rel, selected+1, len(rs))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// geserKursor menutup sesi dengan kursor bergeser satu karakter.
//
// Satuannya RUNE, bukan byte: menggeser satu byte di tengah huruf beraksen
// atau CJK meninggalkan kursor di tengah karakter, dan shell yang menerimanya
// akan menyisipkan di tempat yang salah pada ketikan berikutnya.
func (s *Session) geserKursor(arah int) (State, Outcome, error) {
	st := s.st
	if arah < 0 {
		if st.Cursor > 0 {
			_, lebar := utf8.DecodeLastRuneInString(st.Line[:st.Cursor])
			st.Cursor -= lebar
		}
	} else if st.Cursor < len(st.Line) {
		_, lebar := utf8.DecodeRuneInString(st.Line[st.Cursor:])
		st.Cursor += lebar
	}
	return s.selesai(nil, st, Accepted)
}

// pindahKursor menutup sesi dengan kursor di posisi tertentu.
func (s *Session) pindahKursor(pos int) (State, Outcome, error) {
	st := s.st
	if pos < 0 {
		pos = 0
	}
	if pos > len(st.Line) {
		pos = len(st.Line)
	}
	st.Cursor = pos
	return s.selesai(nil, st, Accepted)
}
