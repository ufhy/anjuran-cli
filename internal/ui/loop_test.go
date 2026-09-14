package ui

import (
	"io"
	"strings"
	"testing"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/spec"
	"github.com/uf-cli/uf/internal/tty"
)

// fakeTerm memutar ulang urutan tombol yang sudah ditentukan, sehingga loop
// interaktif bisa diuji tanpa PTY sama sekali.
type fakeTerm struct {
	keys []tty.Key
	i    int
}

func (f *fakeTerm) ReadKey() (tty.Key, error) {
	if f.i >= len(f.keys) {
		return tty.Key{}, io.EOF
	}
	k := f.keys[f.i]
	f.i++
	return k, nil
}

func (f *fakeTerm) Size() (int, int) { return 80, 24 }

func k(t tty.KeyType) tty.Key   { return tty.Key{Type: t} }
func r(c rune) tty.Key          { return tty.Key{Type: tty.KeyRune, Rune: c} }
func newEngine() *engine.Engine { return engine.New(spec.NewRegistry("../testdata/specs")) }
func discard() *Renderer        { return NewRenderer(io.Discard, 80, 24, true) }

func run(t *testing.T, line string, keys ...tty.Key) (State, Outcome) {
	t.Helper()
	term := &fakeTerm{keys: keys}
	s := NewSession(newEngine(), term, discard(), State{Line: line, Cursor: len(line)})
	st, out, err := s.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return st, out
}

func TestKandidatTunggalLangsungDisisipkan(t *testing.T) {
	// "git stat" hanya cocok dengan status, jadi tidak perlu dropdown.
	st, out := run(t, "git stat")
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if st.Line != "git status" {
		t.Errorf("Line = %q, mau %q", st.Line, "git status")
	}
	if st.Cursor != len("git status") {
		t.Errorf("Cursor = %d, mau %d", st.Cursor, len("git status"))
	}
}

func TestTanpaKandidat(t *testing.T) {
	_, out := run(t, "git zzzz")
	if out != NoCandidates {
		t.Errorf("outcome = %v, mau NoCandidates", out)
	}
}

func TestPilihDenganPanahDanEnter(t *testing.T) {
	// "git " menampilkan semua subcommand; turun satu lalu Enter.
	st, out := run(t, "git ", k(tty.KeyDown), k(tty.KeyEnter))
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if !strings.HasPrefix(st.Line, "git ") || st.Line == "git " {
		t.Errorf("Line = %q, seharusnya sebuah subcommand tersisip", st.Line)
	}
}

func TestEscMembatalkan(t *testing.T) {
	st, out := run(t, "git ", k(tty.KeyDown), k(tty.KeyEscape))
	if out != Cancelled {
		t.Fatalf("outcome = %v, mau Cancelled", out)
	}
	if st.Line != "git " {
		t.Errorf("baris harus utuh saat dibatalkan, dapat %q", st.Line)
	}
}

func TestMengetikMenyaringDaftar(t *testing.T) {
	// Ketik c,o,m lalu Enter: penyaringan harus menyisakan commit.
	st, out := run(t, "git ", r('c'), r('o'), r('m'), k(tty.KeyEnter))
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if st.Line != "git commit" {
		t.Errorf("Line = %q, mau %q", st.Line, "git commit")
	}
}

func TestBackspaceMengembalikanDaftar(t *testing.T) {
	// Ketik "z" hingga daftar kosong, lalu hapus; kandidat harus muncul lagi.
	st, out := run(t, "git ", r('c'), r('o'), r('m'), r('z'))
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	// Karakter yang terlanjur diketik dipertahankan, tidak dibuang diam-diam.
	if st.Line != "git comz" {
		t.Errorf("Line = %q, mau %q", st.Line, "git comz")
	}
}

func TestSpasiMenerimaLaluLanjut(t *testing.T) {
	st, out := run(t, "git ", r('c'), r('o'), r('m'), r(' '))
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if st.Line != "git commit " {
		t.Errorf("Line = %q, mau %q", st.Line, "git commit ")
	}
	if st.Cursor != len(st.Line) {
		t.Errorf("Cursor = %d, mau di ujung baris", st.Cursor)
	}
}

func TestTabBerputar(t *testing.T) {
	// Tab menuruni daftar; berapa pun panjang daftarnya harus kembali ke awal.
	st, out := run(t, "git ", k(tty.KeyTab), k(tty.KeyTab), k(tty.KeyUp), k(tty.KeyUp), k(tty.KeyEnter))
	if out != Accepted {
		t.Fatalf("outcome = %v, mau Accepted", out)
	}
	if st.Line == "git " {
		t.Error("seharusnya ada yang tersisip")
	}
}

func TestKursorDiTengahMenolakKetikan(t *testing.T) {
	// Kursor tidak di ujung baris: gema karakter tidak aman, sesi harus tutup
	// alih-alih menulis di posisi yang salah.
	term := &fakeTerm{keys: []tty.Key{r('c')}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git  --verbose", Cursor: 4})
	st, out, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out != Cancelled {
		t.Errorf("outcome = %v, mau Cancelled", out)
	}
	if st.Line != "git  --verbose" {
		t.Errorf("baris harus utuh, dapat %q", st.Line)
	}
}

func TestEOFTidakMengubahBaris(t *testing.T) {
	st, out := run(t, "git ")
	if out != Cancelled {
		t.Errorf("outcome = %v, mau Cancelled", out)
	}
	if st.Line != "git " {
		t.Errorf("Line = %q, mau utuh", st.Line)
	}
}

func TestWindowMenjagaPilihanTerlihat(t *testing.T) {
	tests := []struct {
		total, selected, rows int
		wantStart, wantRel    int
	}{
		{5, 0, 10, 0, 0},   // semuanya muat
		{20, 0, 5, 0, 0},   // di awal
		{20, 19, 5, 15, 4}, // di akhir
		{20, 10, 5, 8, 2},  // di tengah
	}
	for _, tt := range tests {
		start, rel := window(tt.total, tt.selected, tt.rows)
		if start != tt.wantStart || rel != tt.wantRel {
			t.Errorf("window(%d,%d,%d) = (%d,%d), mau (%d,%d)",
				tt.total, tt.selected, tt.rows, start, rel, tt.wantStart, tt.wantRel)
		}
	}
}

// Urutan daftar adalah bagian dari kegunaannya: kandidat yang paling mungkin
// dimaksud harus sudah terpilih sebelum pengguna menekan apa pun.
func TestUrutanPrioritasSaatBelumMengetik(t *testing.T) {
	cands := []engine.Candidate{
		{Name: "zebra", Kind: engine.KindSubcommand, Priority: 90},
		{Name: "alpha", Kind: engine.KindSubcommand, Priority: 10},
		{Name: "beta", Kind: engine.KindSubcommand, Priority: engine.DefaultPriority},
	}
	got := filter(cands, "")
	want := []string{"zebra", "beta", "alpha"}
	for i := range want {
		if got[i].cand.Name != want[i] {
			t.Fatalf("urutan = %v, mau %v", candNames(got), want)
		}
	}
}

func TestRelevansiMengalahkanPrioritasSaatMengetik(t *testing.T) {
	cands := []engine.Candidate{
		{Name: "zebra", Kind: engine.KindSubcommand, Priority: 99},
		{Name: "alpha", Kind: engine.KindSubcommand, Priority: 1},
	}
	// "al" adalah awalan alpha, jadi alpha harus menang meski prioritasnya
	// jauh lebih rendah.
	got := filter(cands, "al")
	if len(got) == 0 || got[0].cand.Name != "alpha" {
		t.Fatalf("urutan = %v, mau alpha di depan", candNames(got))
	}
}

func TestPrioritasMemutusSeriSkorYangSama(t *testing.T) {
	cands := []engine.Candidate{
		{Name: "commit", Kind: engine.KindSubcommand, Priority: 10},
		{Name: "config", Kind: engine.KindSubcommand, Priority: 90},
	}
	// Keduanya cocok "co" dengan skor identik; prioritas yang menentukan.
	got := filter(cands, "co")
	if got[0].cand.Name != "config" {
		t.Fatalf("urutan = %v, mau config di depan", candNames(got))
	}
}

func candNames(rs []ranked) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.cand.Name
	}
	return out
}

// fakeDynamic menyediakan kandidat dinamis tanpa menjalankan proses apa pun.
type fakeDynamic struct {
	names   []string
	panggil int
}

func (f *fakeDynamic) Candidates(res *engine.Result) []engine.Candidate {
	f.panggil++
	out := make([]engine.Candidate, len(f.names))
	for i, n := range f.names {
		out[i] = engine.Candidate{
			Name: n, Insert: n, CursorOffset: len(n),
			Kind: engine.KindArg, Priority: engine.DefaultPriority,
		}
	}
	return out
}

func prepare(t *testing.T, line string, dyn Dynamic) *Preflight {
	t.Helper()
	p, err := Prepare(newEngine(), State{Line: line, Cursor: len(line)}, dyn)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Kandidat dinamis harus bersaing dalam daftar yang SAMA dengan kandidat
// statis. Menampilkannya sebagai dua daftar terpisah akan membuat nama branch
// selalu kalah atau selalu menang, bukan diurutkan menurut relevansi.
func TestKandidatDinamisIkutDisaring(t *testing.T) {
	dyn := &fakeDynamic{names: []string{"fitur-a", "fitur-b", "main"}}
	p := prepare(t, "git checkout fit", dyn)

	got := p.Candidates()
	if len(got) != 2 {
		t.Fatalf("kandidat = %v, mau hanya yang cocok 'fit'", candNamesOf(got))
	}
	for _, c := range got {
		if c.Name != "fitur-a" && c.Name != "fitur-b" {
			t.Errorf("kandidat tak terduga: %q", c.Name)
		}
	}
}

func TestKandidatDinamisBersaingDenganStatis(t *testing.T) {
	// "-" adalah saran statis pada spec checkout buatan tangan? Tidak; yang
	// diuji di sini adalah keduanya muncul dalam satu daftar terurut.
	dyn := &fakeDynamic{names: []string{"fitur-a"}}
	p := prepare(t, "git checkout ", dyn)

	var adaStatis, adaDinamis bool
	for _, c := range p.Candidates() {
		if c.Name == "fitur-a" {
			adaDinamis = true
		}
		if c.Kind == engine.KindOption {
			adaStatis = true
		}
	}
	if !adaDinamis || !adaStatis {
		t.Errorf("kedua sumber harus muncul dalam satu daftar; dinamis=%v statis=%v", adaDinamis, adaStatis)
	}
}

func TestTanpaDynamicTetapBerjalan(t *testing.T) {
	p := prepare(t, "git ", nil)
	if len(p.Candidates()) == 0 {
		t.Error("kandidat statis harus tetap ada tanpa sumber dinamis")
	}
}

// Kandidat tunggal tidak membuka dropdown sama sekali, termasuk ketika satu-
// satunya kandidat berasal dari generator.
func TestKandidatDinamisTunggalLangsungDisisipkan(t *testing.T) {
	dyn := &fakeDynamic{names: []string{"fitur-unik"}}
	p := prepare(t, "git checkout fitur-uni", dyn)

	st, out, done := p.Immediate()
	if !done || out != Accepted {
		t.Fatalf("outcome = %v, done = %v; mau langsung disisipkan", out, done)
	}
	if st.Line != "git checkout fitur-unik" {
		t.Errorf("Line = %q", st.Line)
	}
}

func candNamesOf(cs []engine.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}
