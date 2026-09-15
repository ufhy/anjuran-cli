package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uf-cli/uf/internal/parser"
	"github.com/uf-cli/uf/internal/spec"
)

// engineFor membuat engine dari satu spec inline, agar perilaku setiap field
// bisa diuji tanpa bergantung pada spec Fig yang besar.
func engineFor(t *testing.T, name, body string) *Engine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(spec.NewRegistry(dir))
}

func find(cs []Candidate, name string) *Candidate {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

// requiresSeparator menentukan bentuk penulisan yang diterima perintahnya.
// Tanpa ini, penelusuran akan menelan token berikutnya sebagai nilai opsi.
func TestRequiresSeparatorTidakMenelanTokenBerikutnya(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--jobs","requiresSeparator":true,"args":[{"name":"n"}]},
		{"name":"--verbose"}],
		"subcommands":[{"name":"build"}]}`)

	// "--jobs" tidak menempel dengan nilainya, jadi token berikutnya adalah
	// subcommand biasa, bukan argumen milik --jobs.
	res, err := e.Complete("t --jobs bui", 12)
	if err != nil {
		t.Fatal(err)
	}
	if find(res.Candidates, "build") == nil {
		t.Errorf("mau subcommand build, dapat %v", names(res.Candidates))
	}
}

func TestRequiresSeparatorMenyisipkanTandaSamaDengan(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--jobs","requiresSeparator":true,"args":[{"name":"n"}]}]}`)

	res, _ := e.Complete("t --jo", 6)
	c := find(res.Candidates, "--jobs")
	if c == nil {
		t.Fatal("mau --jobs")
	}
	if c.Insert != "--jobs=" {
		t.Errorf("Insert = %q, mau %q", c.Insert, "--jobs=")
	}
	if c.CursorOffset != len("--jobs=") {
		t.Errorf("CursorOffset = %d, mau %d", c.CursorOffset, len("--jobs="))
	}
}

func TestInsertValueMenentukanPosisiKursor(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--tag","insertValue":"--tag='{cursor}'"}]}`)

	res, _ := e.Complete("t --ta", 6)
	c := find(res.Candidates, "--tag")
	if c == nil {
		t.Fatal("mau --tag")
	}
	if c.Insert != "--tag=''" {
		t.Errorf("Insert = %q, mau %q", c.Insert, "--tag=''")
	}
	// Kursor harus berada di antara kedua tanda kutip.
	if c.CursorOffset != len("--tag='") {
		t.Errorf("CursorOffset = %d, mau %d", c.CursorOffset, len("--tag='"))
	}
}

func TestDisplayNameDipakaiUntukTampilan(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","subcommands":[
		{"name":"i","displayName":"install"}]}`)

	res, _ := e.Complete("t ", 2)
	c := find(res.Candidates, "i")
	if c == nil {
		t.Fatal("mau subcommand i")
	}
	if c.Label() != "install" {
		t.Errorf("Label = %q, mau install", c.Label())
	}
	// Yang disisipkan tetap nama sebenarnya, bukan teks tampilan.
	if c.Insert != "i" {
		t.Errorf("Insert = %q, mau i", c.Insert)
	}
}

func TestDeprecatedDisembunyikan(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--lama","deprecated":true},{"name":"--baru"}]}`)

	res, _ := e.Complete("t --", 4)
	if find(res.Candidates, "--lama") != nil {
		t.Error("opsi deprecated tidak boleh ditawarkan")
	}
	if find(res.Candidates, "--baru") == nil {
		t.Error("opsi biasa harus tetap ada")
	}
}

func TestDependsOnMenundaOpsi(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--remote","args":[{"name":"r"}]},
		{"name":"--set-upstream","dependsOn":["--remote"]}]}`)

	res, _ := e.Complete("t --s", 5)
	if find(res.Candidates, "--set-upstream") != nil {
		t.Error("opsi dengan dependsOn belum boleh muncul")
	}

	res, _ = e.Complete("t --remote asal --s", 19)
	if find(res.Candidates, "--set-upstream") == nil {
		t.Errorf("setelah syaratnya ada, opsi harus muncul; dapat %v", names(res.Candidates))
	}
}

func TestPriorityDibawaKeKandidat(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","subcommands":[
		{"name":"jarang","priority":10},{"name":"sering","priority":90},{"name":"biasa"}]}`)

	res, _ := e.Complete("t ", 2)
	if got := find(res.Candidates, "sering").Priority; got != 90 {
		t.Errorf("priority = %d, mau 90", got)
	}
	// Entri tanpa prioritas memakai nilai bawaan Fig.
	if got := find(res.Candidates, "biasa").Priority; got != DefaultPriority {
		t.Errorf("priority bawaan = %d, mau %d", got, DefaultPriority)
	}
}

func TestIsDangerousDibawaKeKandidat(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[{"name":"--force","isDangerous":true}]}`)
	res, _ := e.Complete("t --f", 5)
	c := find(res.Candidates, "--force")
	if c == nil || !c.Dangerous {
		t.Errorf("mau --force ditandai berbahaya, dapat %+v", c)
	}
}

// Opsi pendek yang ditulis bergabung adalah bentuk yang paling lazim dipakai:
// "tar -xzf berkas.tar.gz". Tanpa dipecah, seluruhnya dianggap satu opsi tak
// dikenal dan argumennya tidak pernah dilengkapi.
func TestOpsiPendekBergabung(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"-x"},{"name":"-z"},
		{"name":"-f","args":[{"name":"archive","suggestions":[{"name":"arsip.tar"}]}]},
		{"name":"-C","args":[{"name":"dir","suggestions":[{"name":"tujuan"}]}]}]}`)

	// Huruf terakhir dalam gabungan yang menerima argumennya.
	for _, line := range []string{"t -xzf ", "t -xf ", "t -zxf "} {
		res, _ := e.Complete(line, len(line))
		if find(res.Candidates, "arsip.tar") == nil {
			t.Errorf("%q tidak menawarkan argumen milik -f; dapat %v", line, names(res.Candidates))
		}
	}

	// Opsi yang sudah dipakai di dalam gabungan tidak ditawarkan lagi.
	res, _ := e.Complete("t -xz -", 7)
	for _, n := range []string{"-x", "-z"} {
		if find(res.Candidates, n) != nil {
			t.Errorf("%s sudah dipakai di dalam gabungan, tidak boleh ditawarkan lagi", n)
		}
	}
}

// Gabungan yang memuat huruf tak dikenal tidak boleh ditebak sebagian:
// menebaknya hanya akan salah menelan token berikutnya sebagai argumen.
func TestGabunganTakDikenalTidakDitebak(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t",
		"options":[{"name":"-x"},{"name":"-f","args":[{"name":"a"}]}],
		"subcommands":[{"name":"jalan"}]}`)

	res, _ := e.Complete("t -xq jal", 9)
	if find(res.Candidates, "jalan") == nil {
		t.Errorf("token sesudah gabungan tak dikenal harus tetap dianggap subcommand; dapat %v", names(res.Candidates))
	}
}

// Satu opsi bisa terdaftar di subcommand sekaligus diwarisi sebagai opsi
// persisten. Di layar itu terlihat sebagai dua baris identik.
func TestKandidatKembarDibuang(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t",
		"options":[{"name":"--scan","isPersistent":true}],
		"subcommands":[{"name":"build","options":[{"name":"--scan"}]}]}`)

	res, _ := e.Complete("t build --sc", 12)
	n := 0
	for _, c := range res.Candidates {
		if c.Name == "--scan" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("--scan muncul %d kali, mau tepat sekali", n)
	}
}

// Kandidat berisi spasi harus dikutip; tanpa itu shell memecahnya menjadi dua
// kata dan perintahnya rusak begitu dipilih.
func TestKandidatBerspasiDikutip(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","args":[{"suggestions":[
		{"name":"dua kata"},{"name":"biasa"},{"name":"ada'kutip"}]}]}`)

	res, _ := e.Complete("t ", 2)

	c := find(res.Candidates, "dua kata")
	if c == nil || c.Insert != "'dua kata'" {
		t.Errorf("Insert = %q, mau %q", c.Insert, "'dua kata'")
	}
	// Yang tidak perlu dikutip dibiarkan apa adanya.
	if b := find(res.Candidates, "biasa"); b == nil || b.Insert != "biasa" {
		t.Errorf("kandidat biasa tidak boleh dikutip: %q", b.Insert)
	}
	// Kutip tunggal di dalam teks harus dilolos, bukan merusak kutipnya.
	k := find(res.Candidates, "ada'kutip")
	if k == nil || k.Insert != `'ada'\''kutip'` {
		t.Errorf("Insert = %q, mau %q", k.Insert, `'ada'\''kutip'`)
	}
}

// Korpus spec berasal dari pihak ketiga dan bisa cacat: mysql memuat
// insertValue "{cursor}'", yang menyisakan kutip menggantung.
func TestInsertValueCacatTidakDipakai(t *testing.T) {
	e := engineFor(t, "t", `{"name":"t","options":[
		{"name":"--init","insertValue":"{cursor}'"}]}`)

	res, _ := e.Complete("t --in", 6)
	c := find(res.Candidates, "--init")
	if c == nil {
		t.Fatal("mau --init")
	}
	if c.Insert != "--init" {
		t.Errorf("Insert = %q, mau kembali ke nama kandidatnya", c.Insert)
	}
}

// Sifat yang harus benar untuk teks apa pun: mengutip lalu mengurai kembali
// menghasilkan teks yang sama persis, sebagai SATU kata.
//
// Ini menutup seluruh kelasnya sekaligus, bukan contoh per contoh — dan
// memang menemukan pelolosan kutip tunggal yang salah, yang justru
// menghasilkan kutip tidak seimbang.
func TestQuoteBolakBalik(t *testing.T) {
	kasus := []string{
		"biasa", "dua kata", "tiga  spasi  beruntun",
		"ada'kutip", "'diawali kutip", "diakhiri kutip'", "''", "'",
		`ada"kutip ganda`, "ada$dolar", "ada`backtick`", `ada\backslash`,
		"ada|pipa", "ada&ampersand", "ada;titikkoma", "ada>redirect",
		"ada*bintang", "ada?tanya", "ada[kurung]", "ada{kurawal}",
		"ada!seru", "ada#pagar", "ada(kurung)",
		"café dengan aksen", "日本語 spasi", "emoji \U0001F680 di tengah",
		"berkas dengan spasi.txt", "-diawali minus", "--terlihat seperti opsi",
	}

	for _, s := range kasus {
		q := Quote(s)
		l := parser.Parse(q, len(q))

		if len(l.Tokens) != 1 {
			t.Errorf("Quote(%q) = %q terurai menjadi %d kata", s, q, len(l.Tokens))
			continue
		}
		if !l.Tokens[0].Terminated {
			t.Errorf("Quote(%q) = %q punya kutip yang tidak tertutup", s, q)
			continue
		}
		if got := l.Tokens[0].Value; got != s {
			t.Errorf("Quote(%q) = %q terurai kembali menjadi %q", s, q, got)
		}
	}
}

// Teks yang tidak butuh kutip dibiarkan apa adanya, supaya path tetap terbaca
// sebagai path dan tilde tetap dipekarkan shell.
func TestQuoteTidakMengubahYangAman(t *testing.T) {
	for _, s := range []string{
		"biasa", "internal/engine/", "./berkas.txt", "../naik", "/abs/path",
		"~/rumah", "--opt=nilai", "-x", "nama-dengan-tanda_hubung.txt", "v1.2.3",
	} {
		if got := Quote(s); got != s {
			t.Errorf("Quote(%q) = %q, mau tidak berubah", s, got)
		}
	}
}
