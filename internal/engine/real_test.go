package engine

import (
	"os"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/spec"
)

// realSpecs adalah direktori spec hasil transpile. Berkas ini dihasilkan oleh
// `make specs`, jadi seluruh pengujian di sini dilewati bila belum dibangun.
const realSpecs = "../../specs"

func realEngine(tb testing.TB) *Engine {
	tb.Helper()
	if _, err := os.Stat(realSpecs); err != nil {
		tb.Skip("direktori specs/ belum dibangun; jalankan `make specs`")
	}
	return New(spec.NewRegistry(realSpecs))
}

func TestSpecAsliTerbaca(t *testing.T) {
	e := realEngine(t)
	tests := []struct {
		line, want string
	}{
		{"git ch", "checkout"},
		{"docker comp", "compose"},
		{"terraform ap", "apply"},
		{"systemctl resta", "restart"},
		{"npm ins", "install"},
		{"ssh -", "-p"},
	}
	for _, tt := range tests {
		res, err := e.Complete(tt.line, len(tt.line))
		if err != nil {
			t.Fatalf("Complete(%q): %v", tt.line, err)
		}
		if !has(res.Candidates, tt.want) {
			t.Errorf("Complete(%q) tidak memuat %q; dapat %v", tt.line, tt.want, names(res.Candidates))
		}
	}
}

// loadSpec adalah bagian yang membuat aws, az, dan gcloud bisa dipakai sama
// sekali: isinya tersebar di ratusan berkas terpisah.
func TestLoadSpecDimuatSaatDibutuhkan(t *testing.T) {
	e := realEngine(t)
	res, err := e.Complete("aws s3 ", 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cp", "ls", "sync"} {
		if !has(res.Candidates, want) {
			t.Errorf("mau subcommand %q dari aws/s3, dapat %v", want, names(res.Candidates))
		}
	}
}

func TestLoadSpecBersarangDua(t *testing.T) {
	e := realEngine(t)
	// az memecah spec-nya lebih dalam lagi.
	res, err := e.Complete("az vm ", 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) == 0 {
		t.Error("mau subcommand az vm, tidak dapat apa-apa")
	}
}

func BenchmarkSpecKecil(b *testing.B) {
	e := realEngine(b)
	line := "git checkout --fo"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Complete(line, len(line))
	}
}

func BenchmarkSpecBesarLewatLoadSpec(b *testing.B) {
	e := realEngine(b)
	line := "aws s3 cp --rec"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Complete(line, len(line))
	}
}

// BenchmarkMuatDingin mengukur biaya sebenarnya yang dirasakan pengguna:
// registry kosong, jadi berkas gzip harus dibuka dan diurai.
func BenchmarkMuatDingin(b *testing.B) {
	if _, err := os.Stat(realSpecs); err != nil {
		b.Skip("direktori specs/ belum dibangun")
	}
	line := "aws s3 cp --rec"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := New(spec.NewRegistry(realSpecs))
		e.Complete(line, len(line))
	}
}
