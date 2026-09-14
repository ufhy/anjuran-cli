package engine

import (
	"os"
	"path/filepath"
	"testing"

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
