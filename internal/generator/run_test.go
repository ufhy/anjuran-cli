package generator

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// skipTanpaShellUnix melewati uji yang menjalankan program POSIX.
func skipTanpaShellUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uji ini memakai program POSIX")
	}
}

// runnerUntuk membuat runner yang mengizinkan satu biner tertentu.
func runnerUntuk(nama string) *Runner {
	return &Runner{
		Policy:  Policy{Command: nama, Allow: map[string]bool{}, Enabled: true},
		Timeout: 2 * time.Second,
	}
}

func namaDari(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestMenjalankanDanMengurai(t *testing.T) {
	skipTanpaShellUnix(t)
	r := runnerUntuk("printf")
	got := r.Run(context.Background(), Spec{
		Script: []string{"/usr/bin/printf", "satu\ndua\ntiga\n"},
	})
	want := []string{"satu", "dua", "tiga"}
	if len(got) != len(want) {
		t.Fatalf("kandidat = %v, mau %v", namaDari(got), want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("kandidat[%d] = %q, mau %q", i, got[i].Name, want[i])
		}
	}
}

func TestBarisGandaDibuang(t *testing.T) {
	skipTanpaShellUnix(t)
	r := runnerUntuk("printf")
	got := r.Run(context.Background(), Spec{
		Script: []string{"/usr/bin/printf", "a\nb\na\nb\nc\n"},
	})
	if len(got) != 3 {
		t.Errorf("kandidat = %v, mau 3 entri unik", namaDari(got))
	}
}

// Generator yang lambat tidak boleh menahan tombol Tab.
func TestBatasWaktuDipatuhi(t *testing.T) {
	skipTanpaShellUnix(t)
	r := runnerUntuk("sleep")
	r.Timeout = 150 * time.Millisecond

	mulai := time.Now()
	got := r.Run(context.Background(), Spec{Script: []string{"/bin/sleep", "10"}})
	lama := time.Since(mulai)

	if len(got) != 0 {
		t.Errorf("generator yang habis waktu harus menghasilkan daftar kosong, dapat %v", namaDari(got))
	}
	if lama > 2*time.Second {
		t.Errorf("butuh %v; batas waktu tidak dipatuhi", lama)
	}
}

// Proses cucu harus ikut mati, kalau tidak ia menumpuk pada tombol yang
// ditekan puluhan kali per menit.
func TestKeturunanIkutMati(t *testing.T) {
	skipTanpaShellUnix(t)

	penanda := filepath.Join(t.TempDir(), "hidup")
	// Kebijakan menolak shell, jadi di sini Runner dipakai langsung tanpa
	// melewati Check — yang diuji adalah mekanisme mematikannya.
	r := &Runner{Timeout: 200 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = r.exec(ctx, []string{"/bin/sh", "-c",
		"(sleep 5; touch " + penanda + ") & wait"})

	time.Sleep(600 * time.Millisecond)
	if _, err := os.Stat(penanda); err == nil {
		t.Error("proses keturunan masih hidup setelah waktunya habis")
	}
}

func TestProgramTidakAdaBukanError(t *testing.T) {
	r := runnerUntuk("program-yang-pasti-tidak-ada")
	got := r.Run(context.Background(), Spec{
		Script: []string{"program-yang-pasti-tidak-ada"},
	})
	if len(got) != 0 {
		t.Errorf("mau daftar kosong, dapat %v", namaDari(got))
	}
}

func TestGeneratorDitolakDicatat(t *testing.T) {
	r := runnerUntuk("git")
	got := r.Run(context.Background(), Spec{Script: []string{"curl", "https://contoh.test"}})
	if len(got) != 0 {
		t.Errorf("mau daftar kosong, dapat %v", namaDari(got))
	}
	if len(r.Denied) != 1 {
		t.Fatalf("penolakan harus dicatat, dapat %v", r.Denied)
	}
	if !strings.Contains(r.Denied[0], "curl") {
		t.Errorf("catatan penolakan harus menyebut perintahnya: %q", r.Denied[0])
	}
}

func TestSplitOnKhusus(t *testing.T) {
	skipTanpaShellUnix(t)
	r := runnerUntuk("printf")
	got := r.Run(context.Background(), Spec{
		Script:  []string{"/usr/bin/printf", "a,b,c"},
		SplitOn: ",",
	})
	if len(got) != 3 {
		t.Errorf("kandidat = %v, mau 3", namaDari(got))
	}
}

// Seluruh generator pada satu posisi kursor berbagi satu tenggat, sehingga
// total waktunya tetap terbatas berapa pun jumlahnya.
func TestRunAllBerbagiTenggat(t *testing.T) {
	skipTanpaShellUnix(t)
	r := runnerUntuk("sleep")
	r.Timeout = 200 * time.Millisecond

	specs := []Spec{
		{Script: []string{"/bin/sleep", "5"}},
		{Script: []string{"/bin/sleep", "5"}},
		{Script: []string{"/bin/sleep", "5"}},
	}
	mulai := time.Now()
	r.RunAll(specs)
	if lama := time.Since(mulai); lama > 2*time.Second {
		t.Errorf("tiga generator lambat butuh %v; tenggat harus dibagi bersama", lama)
	}
}

func TestScriptKosongDiabaikan(t *testing.T) {
	r := runnerUntuk("git")
	if got := r.Run(context.Background(), Spec{}); got != nil {
		t.Errorf("mau nil, dapat %v", namaDari(got))
	}
}

// Generator yang gagal adalah yang PALING mahal: biayanya dibayar penuh setiap
// kali, tanpa pernah menghasilkan apa pun. Saat dropdown digambar ulang tiap
// ketikan, itu berarti proses gagal yang ditumbuhkan terus-menerus.
func TestKegagalanIkutDiCache(t *testing.T) {
	skipTanpaShellUnix(t)

	c := &Cache{Dir: t.TempDir(), Now: time.Now}
	r := runnerUntuk("program-yang-pasti-tidak-ada")
	r.Cache = c

	spec := Spec{Script: []string{"program-yang-pasti-tidak-ada"}, CacheTTL: time.Minute}
	if got := r.Run(context.Background(), spec); len(got) != 0 {
		t.Fatalf("mau kosong, dapat %v", namaDari(got))
	}

	if _, ok := c.Get(spec.Script, ""); !ok {
		t.Error("kegagalan harus ikut tercatat di cache")
	}
}

// Masa berlaku cache kegagalan harus pendek: keadaan yang membuatnya gagal
// bisa berubah kapan saja, misalnya setelah `git init`.
func TestCacheKegagalanBerumurPendek(t *testing.T) {
	skipTanpaShellUnix(t)

	now := time.Now()
	c := &Cache{Dir: t.TempDir(), Now: func() time.Time { return now }}
	r := runnerUntuk("program-yang-pasti-tidak-ada")
	r.Cache = c

	spec := Spec{Script: []string{"program-yang-pasti-tidak-ada"}, CacheTTL: time.Hour}
	r.Run(context.Background(), spec)

	now = now.Add(failureTTL + time.Second)
	if _, ok := c.Get(spec.Script, ""); ok {
		t.Error("cache kegagalan seharusnya sudah kedaluwarsa")
	}
}
