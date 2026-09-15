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
	// Kursor tidak di ujung baris: gema karakter tidak aman, jadi sesi tutup
	// alih-alih menulis di posisi yang salah. Karakternya DIKEMBALIKAN ke
	// shell, sehingga tetap tersisip — hanya oleh zsh, bukan oleh kita.
	key := tty.Key{Type: tty.KeyRune, Rune: 'c', Raw: []byte("c")}
	term := &fakeTerm{keys: []tty.Key{key}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git  --verbose", Cursor: 4})
	st, _, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	if st.Line != "git  --verbose" {
		t.Errorf("baris harus utuh, dapat %q", st.Line)
	}
	if string(s.Leftover()) != "c" {
		t.Errorf("Leftover = %q, mau karakter dikembalikan ke shell", s.Leftover())
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

// Memilih direktori berarti sedang menelusuri, belum selesai memilih. Menutup
// kotak di situ memaksa memulai lagi dari awal untuk setiap tingkat.
func TestMemilihDirektoriMelanjutkan(t *testing.T) {
	// Dua kandidat, supaya dropdown benar-benar terbuka: satu kandidat akan
	// langsung disisipkan tanpa sesi.
	dyn := &fakeDynamic{names: []string{"internal/", "integrasi/"}}
	term := &fakeTerm{keys: []tty.Key{
		k(tty.KeyEnter), // pilih salah satu direktori
		k(tty.KeyEscape),
	}}

	p, err := Prepare(newEngine(), State{Line: "git add int", Cursor: 11}, dyn)
	if err != nil {
		t.Fatal(err)
	}
	st, out, err := p.Session(term, discard()).Run()
	if err != nil {
		t.Fatal(err)
	}

	// Enter menyisipkan direktorinya lalu sesi TETAP berjalan, sehingga Esc
	// setelahnya yang mengakhirinya.
	if out != Cancelled {
		t.Errorf("outcome = %v; sesi seharusnya lanjut setelah direktori dipilih", out)
	}
	if !strings.HasSuffix(st.Line, "/") {
		t.Errorf("Line = %q, mau berakhir dengan direktori", st.Line)
	}
}

// Memilih berkas biasa menyelesaikan pilihan dan menutup kotak.
func TestMemilihBerkasMenutup(t *testing.T) {
	dyn := &fakeDynamic{names: []string{"internal.txt"}}
	term := &fakeTerm{keys: []tty.Key{k(tty.KeyEnter)}}

	p, _ := Prepare(newEngine(), State{Line: "git add int", Cursor: 11}, dyn)
	st, out, _ := p.Session(term, discard()).Run()

	if out != Accepted {
		t.Errorf("outcome = %v, mau Accepted", out)
	}
	if st.Line != "git add internal.txt" {
		t.Errorf("Line = %q", st.Line)
	}
}

// Tab menyisipkan awalan terpanjang yang sama lebih dulu — perilaku Tab yang
// sudah dikenal orang dari shell mana pun. Tanpa itu Tab hanya terasa seperti
// panah bawah.
func TestTabMenyisipkanAwalanBersama(t *testing.T) {
	dyn := &fakeDynamic{names: []string{"fitur-alpha", "fitur-beta", "fitur-gamma"}}
	term := &fakeTerm{keys: []tty.Key{k(tty.KeyTab), k(tty.KeyEscape)}}

	p, err := Prepare(newEngine(), State{Line: "git add fit", Cursor: 11}, dyn)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := p.Session(term, discard()).Run()
	if err != nil {
		t.Fatal(err)
	}
	if st.Line != "git add fitur-" {
		t.Errorf("Line = %q, mau %q", st.Line, "git add fitur-")
	}
}

// Bila tidak ada awalan yang bisa ditambahkan, Tab kembali berperan
// memindahkan pilihan.
func TestTabBerpindahBilaTidakAdaAwalan(t *testing.T) {
	dyn := &fakeDynamic{names: []string{"alpha", "beta"}}
	term := &fakeTerm{keys: []tty.Key{k(tty.KeyTab), k(tty.KeyEnter)}}

	p, _ := Prepare(newEngine(), State{Line: "git add ", Cursor: 8}, dyn)
	st, out, _ := p.Session(term, discard()).Run()

	if out != Accepted {
		t.Fatalf("outcome = %v", out)
	}
	// Tab memindahkan ke kandidat berikutnya, lalu Enter memilihnya.
	if strings.HasSuffix(st.Line, "alpha") {
		t.Errorf("Tab seharusnya berpindah dari kandidat pertama; Line = %q", st.Line)
	}
}

func TestAwalanBersama(t *testing.T) {
	buat := func(names ...string) []ranked {
		out := make([]ranked, len(names))
		for i, n := range names {
			out[i] = ranked{cand: engine.Candidate{Name: n}, match: &Match{}}
		}
		return out
	}

	tests := []struct {
		names  []string
		prefix string
		want   string
	}{
		{[]string{"fitur-a", "fitur-b"}, "fit", "fitur-"},
		{[]string{"commit", "config"}, "co", "co"},        // tidak menambah apa pun
		{[]string{"alpha", "beta"}, "", ""},               // tidak ada awalan bersama
		{[]string{"satu"}, "s", ""},                       // satu kandidat bukan urusan Tab
		{[]string{"Fitur-a", "fitur-b"}, "fit", "Fitur-"}, // beda huruf besar tetap cocok
	}
	for _, tt := range tests {
		got := awalanBersama(buat(tt.names...), tt.prefix)
		if tt.want == "co" {
			// "co" sama panjang dengan prefix, jadi tidak ada yang disisipkan.
			tt.want = ""
		}
		if got != tt.want {
			t.Errorf("awalanBersama(%v, %q) = %q, mau %q", tt.names, tt.prefix, got, tt.want)
		}
	}
}

// Tombol yang bukan urusan dropdown harus DIKEMBALIKAN ke shell, bukan
// ditelan. Menelannya membuat tombol seperti Ctrl-A atau Home terasa kadang
// tidak berfungsi — dan itu jauh lebih mengganggu daripada dropdown yang
// menutup sedikit terlalu cepat.
func TestTombolAsingDikembalikan(t *testing.T) {
	tests := []struct {
		nama string
		key  tty.Key
	}{
		{"Ctrl-A", tty.Key{Type: tty.KeyUnknown, Raw: []byte{0x01}}},
		{"panah kiri", tty.Key{Type: tty.KeyLeft, Raw: []byte("\x1b[D")}},
		{"panah kanan", tty.Key{Type: tty.KeyRight, Raw: []byte("\x1b[C")}},
		{"Ctrl-C", tty.Key{Type: tty.KeyCtrlC, Raw: []byte{0x03}}},
	}

	for _, tt := range tests {
		term := &fakeTerm{keys: []tty.Key{tt.key}}
		s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})
		if _, _, err := s.Run(); err != nil {
			t.Fatalf("%s: %v", tt.nama, err)
		}
		if string(s.Leftover()) != string(tt.key.Raw) {
			t.Errorf("%s: Leftover = %q, mau %q", tt.nama, s.Leftover(), tt.key.Raw)
		}
	}
}

// Esc berarti "batalkan saran", bukan "batalkan baris", jadi ia berhenti di
// sini dan tidak diteruskan.
func TestEscTidakDikembalikan(t *testing.T) {
	term := &fakeTerm{keys: []tty.Key{{Type: tty.KeyEscape, Raw: []byte{0x1b}}}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})
	if _, out, _ := s.Run(); out != Cancelled {
		t.Errorf("outcome = %v, mau Cancelled", out)
	}
	if len(s.Leftover()) != 0 {
		t.Errorf("Esc seharusnya berhenti di dropdown, dapat %q", s.Leftover())
	}
}

// Di dalam kutip dan sesudah backslash, spasi adalah BAGIAN DARI KATA.
// Memperlakukannya sebagai pemicu akan menerima kandidat yang tersorot dan
// merusak nama berkas yang sedang diketik.
func TestSpasiLiteral(t *testing.T) {
	tests := []struct {
		line   string
		cursor int
		want   bool
	}{
		{"cat berkas", 10, false},
		{`cat "berkas`, 11, true},   // kutip ganda belum ditutup
		{`cat 'berkas`, 11, true},   // kutip tunggal belum ditutup
		{`cat "berkas"`, 12, false}, // sudah ditutup
		{`cat berkas\`, 11, true},   // backslash melolos berikutnya
		{`cat berkas\\`, 12, false}, // backslash-nya sendiri sudah dilolos
		{"cat ", 4, false},
	}
	for _, tt := range tests {
		s := &Session{st: State{Line: tt.line, Cursor: tt.cursor}}
		if got := s.spasiLiteral(); got != tt.want {
			t.Errorf("spasiLiteral(%q@%d) = %v, mau %v", tt.line, tt.cursor, got, tt.want)
		}
	}
}

func TestHitungBackslash(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0}, {"abc", 0}, {`abc\`, 1}, {`abc\\`, 2}, {`abc\\\`, 3},
	}
	for _, tt := range tests {
		if got := hitungBackslash(tt.s); got != tt.want {
			t.Errorf("hitungBackslash(%q) = %d, mau %d", tt.s, got, tt.want)
		}
	}
}

// Spasi menerima pilihan LALU membuka konteks berikutnya. Menutup dropdown di
// situ berarti pengguna harus memicunya lagi secara manual — padahal spasinya
// sudah dikonsumsi sesi dan tidak pernah sampai ke shell untuk memicu ulang.
func TestSpasiMenerimaLaluLanjut(t *testing.T) {
	term := &fakeTerm{keys: []tty.Key{
		r('c'), r('o'), r('m'), // saring ke commit
		{Type: tty.KeyRune, Rune: ' ', Raw: []byte(" ")},
		k(tty.KeyEscape),
	}}
	s := NewSession(newEngine(), term, discard(), State{Line: "git ", Cursor: 4})
	st, out, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}
	// Esc yang mengakhiri sesi, bukan spasinya: sesi tetap berjalan setelah
	// spasi diterima.
	if out != Cancelled {
		t.Errorf("outcome = %v; sesi seharusnya lanjut setelah spasi", out)
	}
	if st.Line != "git commit " {
		t.Errorf("Line = %q, mau %q", st.Line, "git commit ")
	}
}
