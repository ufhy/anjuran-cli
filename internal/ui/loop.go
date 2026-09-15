package ui

import (
	"io"

	"github.com/uf-cli/uf/internal/engine"
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
	eng *engine.Engine
	dyn Dynamic
	res *engine.Result
	rs  []ranked
	st  State
}

// Prepare menghitung dan memeringkat kandidat untuk sebuah state.
//
// dyn boleh nil; tanpa itu hanya kandidat statis dari spec yang dipakai.
func Prepare(eng *engine.Engine, st State, dyn Dynamic) (*Preflight, error) {
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

	return &Preflight{eng: eng, dyn: dyn, res: res, rs: filter(res.Candidates, res.Prefix), st: st}, nil
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

// Immediate menangani kasus yang tidak memerlukan terminal. Nilai ketiga
// bernilai false bila dropdown memang harus ditampilkan.
func (p *Preflight) Immediate() (State, Outcome, bool) {
	switch len(p.rs) {
	case 0:
		return p.st, NoCandidates, true
	case 1:
		return apply(p.st, p.res, p.rs[0].cand), Accepted, true
	}
	return p.st, Cancelled, false
}

// Session menjalankan interaksi dropdown untuk satu penekanan tombol pelengkap.
type Session struct {
	eng  *engine.Engine
	dyn  Dynamic
	term Terminal
	rend *Renderer

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
		eng:      p.eng,
		dyn:      p.dyn,
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

	switch len(rs) {
	case 0:
		return s.st, NoCandidates, nil
	case 1:
		// Satu kandidat tidak perlu dropdown: langsung sisipkan.
		return apply(s.st, res, rs[0].cand), Accepted, nil
	}

	selected := s.start
	if selected < 0 {
		selected = len(rs) - 1
	}
	if selected >= len(rs) {
		selected = 0
	}
	defer s.rend.Clear()

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
		case KeyCtrlC, KeyEscape, KeyCtrlD:
			return s.st, Cancelled, nil

		case KeyEnter:
			return apply(s.st, res, rs[selected].cand), Accepted, nil

		case KeyDown, KeyTab:
			selected = (selected + 1) % len(rs)

		case KeyUp, KeyShiftTab:
			selected = (selected - 1 + len(rs)) % len(rs)

		case KeyPageDown:
			selected = min(selected+s.rend.MaxRows(), len(rs)-1)

		case KeyPageUp:
			selected = max(selected-s.rend.MaxRows(), 0)

		case KeyRune:
			if key.Rune == ' ' {
				// Spasi menerima pilihan lalu menutup dropdown, sehingga
				// pengguna bisa langsung lanjut mengetik argumen berikutnya.
				st := apply(s.st, res, rs[selected].cand)
				st.Line += " "
				st.Cursor = len(st.Line)
				return st, Accepted, nil
			}
			if !s.typable {
				return s.st, Cancelled, nil
			}
			s.insert(key.Rune)
			if err := s.rend.EchoRune(key.Rune); err != nil {
				return s.st, Cancelled, err
			}
			if res, rs, selected, err = s.refresh(); err != nil {
				return s.st, Cancelled, err
			} else if len(rs) == 0 {
				// Tidak ada lagi yang cocok: tutup, tetapi pertahankan
				// karakter yang sudah terlanjur diketik pengguna.
				return s.st, Accepted, nil
			}

		case KeyBackspace:
			if !s.typable || len(s.st.Line) == 0 {
				return s.st, Cancelled, nil
			}
			s.deleteBack()
			if err := s.rend.EchoBackspace(); err != nil {
				return s.st, Cancelled, err
			}
			if res, rs, selected, err = s.refresh(); err != nil {
				return s.st, Cancelled, err
			} else if len(rs) == 0 {
				return s.st, Accepted, nil
			}

		default:
			// Tombol yang tidak ditangani menutup dropdown tanpa mengubah apa
			// pun, agar tidak ada tombol yang "tertelan" diam-diam.
			return s.st, Cancelled, nil
		}
	}
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
	return res, filter(res.Candidates, res.Prefix), nil
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
	start, rel := window(len(rs), selected, rows)
	end := min(start+rows, len(rs))
	// rel menyorot baris di layar; selected+1 melaporkan posisi sebenarnya.
	return s.rend.Render(items(rs[start:end]), rel, selected+1, len(rs))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
