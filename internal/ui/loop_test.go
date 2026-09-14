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
func newEngine() *engine.Engine { return engine.New(spec.NewRegistry("../../specs")) }
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
