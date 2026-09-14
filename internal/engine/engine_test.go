package engine

import (
	"strings"
	"testing"

	"github.com/uf-cli/uf/internal/spec"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	return New(spec.NewRegistry("../../specs"))
}

// names mengambil nama kandidat agar assertion mudah dibaca.
func names(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func has(cs []Candidate, name string) bool {
	for _, c := range cs {
		if c.Name == name {
			return true
		}
	}
	return false
}

func complete(t *testing.T, e *Engine, line string) *Result {
	t.Helper()
	res, err := e.Complete(line, len(line))
	if err != nil {
		t.Fatalf("Complete(%q): %v", line, err)
	}
	return res
}

func TestSubcommandLevelSatu(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "git comm")
	if !has(res.Candidates, "commit") {
		t.Fatalf("mau commit, dapat %v", names(res.Candidates))
	}
	if has(res.Candidates, "checkout") {
		t.Errorf("checkout tidak boleh cocok dengan prefix 'comm'")
	}
}

func TestAliasSubcommand(t *testing.T) {
	e := newTestEngine(t)
	// "co" adalah alias checkout; alias yang cocok yang harus ditampilkan.
	res := complete(t, e, "git co")
	if !has(res.Candidates, "co") {
		t.Fatalf("mau alias co, dapat %v", names(res.Candidates))
	}
	// Setelah alias dipakai, engine harus tetap masuk ke subcommand checkout.
	res = complete(t, e, "git co -")
	if !has(res.Candidates, "--force") {
		t.Fatalf("alias harus mengarah ke spec checkout, dapat %v", names(res.Candidates))
	}
}

func TestSubcommandBersarang(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "docker compose ")
	for _, want := range []string{"up", "down", "logs"} {
		if !has(res.Candidates, want) {
			t.Errorf("mau %s, dapat %v", want, names(res.Candidates))
		}
	}
}

func TestOpsiHanyaSaatPrefixMinus(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "git commit --am")
	if !has(res.Candidates, "--amend") {
		t.Fatalf("mau --amend, dapat %v", names(res.Candidates))
	}
	for _, c := range res.Candidates {
		if c.Kind != KindOption {
			t.Errorf("prefix '--am' seharusnya hanya menghasilkan opsi, dapat %s (%s)", c.Name, c.Kind)
		}
	}
}

func TestOpsiPersistenTurunKeSubcommand(t *testing.T) {
	e := newTestEngine(t)
	// --namespace didefinisikan di akar kubectl dengan isPersistent.
	res := complete(t, e, "kubectl get pods --name")
	if !has(res.Candidates, "--namespace") {
		t.Fatalf("opsi persisten harus tersedia di subcommand, dapat %v", names(res.Candidates))
	}
}

func TestOpsiTerpakaiDisembunyikan(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "git commit --amend --am")
	if has(res.Candidates, "--amend") {
		t.Errorf("--amend non-repeatable dan sudah dipakai, tidak boleh muncul lagi")
	}
}

func TestOpsiRepeatableTetapMuncul(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "ssh -o ControlMaster=auto -o")
	if !has(res.Candidates, "-o") {
		t.Errorf("-o bersifat repeatable, harus tetap ditawarkan; dapat %v", names(res.Candidates))
	}
}

func TestExclusiveOn(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "git push --force-with-lease -")
	if has(res.Candidates, "--force") {
		t.Errorf("--force eksklusif terhadap --force-with-lease, tidak boleh muncul")
	}
}

func TestArgumenMilikOpsi(t *testing.T) {
	e := newTestEngine(t)
	// Kursor tepat di slot argumen --output.
	res := complete(t, e, "kubectl get pods -o ")
	if !has(res.Candidates, "json") || !has(res.Candidates, "yaml") {
		t.Fatalf("mau nilai output json/yaml, dapat %v", names(res.Candidates))
	}
	for _, c := range res.Candidates {
		if c.Kind == KindSubcommand {
			t.Errorf("slot argumen opsi tidak boleh menawarkan subcommand: %s", c.Name)
		}
	}
}

func TestBentukSamaDengan(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "kubectl get pods --output=ya")
	if len(res.Candidates) != 1 || res.Candidates[0].Name != "yaml" {
		t.Fatalf("mau yaml, dapat %v", names(res.Candidates))
	}
	// Teks yang disisipkan harus membawa kembali bagian --output=.
	if got := res.Candidates[0].Insert; got != "--output=yaml" {
		t.Errorf("Insert = %q, mau %q", got, "--output=yaml")
	}
}

func TestArgumenPosisional(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "kubectl get po")
	if !has(res.Candidates, "pods") {
		t.Fatalf("mau resource pods, dapat %v", names(res.Candidates))
	}
}

func TestGeneratorDikumpulkanTanpaDieksekusi(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "git checkout ")
	if len(res.Generators) == 0 {
		t.Fatal("generator untuk branch harus dilaporkan")
	}
	want := "git branch --format=%(refname:short)"
	got := strings.Join(res.Generators[0].Script, " ")
	if got != want {
		t.Errorf("script generator = %q, mau %q", got, want)
	}
}

func TestPerintahTanpaSpecBukanError(t *testing.T) {
	e := newTestEngine(t)
	res := complete(t, e, "perintah-yang-tidak-ada foo")
	if len(res.Candidates) != 0 {
		t.Errorf("mau nol kandidat, dapat %v", names(res.Candidates))
	}
}

func TestRentangPenggantian(t *testing.T) {
	e := newTestEngine(t)

	// Kursor di tengah token: seluruh token yang diganti, bukan hanya prefix.
	res, err := e.Complete("git checkout", 7)
	if err != nil {
		t.Fatal(err)
	}
	if res.ReplaceStart != 4 || res.ReplaceEnd != 12 {
		t.Errorf("rentang = [%d,%d), mau [4,12)", res.ReplaceStart, res.ReplaceEnd)
	}

	// Kursor di whitespace: sisipan kosong di posisi kursor.
	res = complete(t, e, "git ")
	if res.ReplaceStart != 4 || res.ReplaceEnd != 4 {
		t.Errorf("rentang = [%d,%d), mau [4,4)", res.ReplaceStart, res.ReplaceEnd)
	}
}

func TestTokenDikutipBukanOpsi(t *testing.T) {
	e := newTestEngine(t)
	// "-a" dalam kutip adalah argumen biasa, bukan flag. Karena itu -a/--all
	// belum terpakai dan harus tetap ditawarkan.
	res := complete(t, e, `git commit "-a" -`)
	if !has(res.Candidates, "--all") {
		t.Fatalf("token berkutip tidak boleh dihitung sebagai flag terpakai, dapat %v", names(res.Candidates))
	}

	// Pembanding: tanpa kutip, -a memang terpakai dan harus hilang.
	res = complete(t, e, "git commit -a -")
	if has(res.Candidates, "--all") {
		t.Errorf("-a tanpa kutip sudah terpakai, --all tidak boleh muncul")
	}
}

func TestCompletionSetelahPipe(t *testing.T) {
	e := newTestEngine(t)
	// Setelah pipe, konteksnya perintah baru yang tidak punya spec.
	res := complete(t, e, "docker ps | git comm")
	if !has(res.Candidates, "commit") {
		t.Fatalf("konteks harus di-reset ke git, dapat %v", names(res.Candidates))
	}
}

func BenchmarkComplete(b *testing.B) {
	e := New(spec.NewRegistry("../../specs"))
	line := "kubectl get pods --namespace kube-system -o wi"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Complete(line, len(line)); err != nil {
			b.Fatal(err)
		}
	}
}
