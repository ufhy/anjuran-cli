package remote

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipDiWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uji ini memakai shell POSIX")
	}
}

// sumberPalsu menyiapkan direktori berisi binary dan spec seperti hasil rilis.
func sumberPalsu(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Binary tiruan yang mencetak versinya, supaya pemeriksaan pasca-pasang
	// benar-benar menjalankan berkas yang dikirim.
	bin := filepath.Join(dir, "anjuran")
	script := "#!/bin/sh\necho 'anjuran 9.9.9'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	specs := filepath.Join(dir, "specs")
	if err := os.MkdirAll(filepath.Join(specs, "aws"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"git.json", filepath.Join("aws", "s3.json")} {
		if err := os.WriteFile(filepath.Join(specs, f), []byte(`{"name":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPasangDanPeriksa(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}
	src := sumberPalsu(t)

	opt := Options{From: src, Base: ".local", Out: io.Discard}
	plan, cleanup, err := Prepare(context.Background(), tr, opt)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if plan.Platform.OS != runtime.GOOS {
		t.Errorf("platform = %v, mau %s", plan.Platform, runtime.GOOS)
	}

	if err := Install(context.Background(), tr, plan, opt); err != nil {
		t.Fatal(err)
	}

	// Binary harus mendarat di bawah rumah SUNGGUHAN, bukan di direktori
	// bernama harfiah $HOME.
	bin := filepath.Join(home, ".local", "bin", "anjuran")
	fi, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("binary tidak ada di %s: %v", bin, err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("binary kehilangan bit eksekusinya")
	}
	if _, err := os.Stat(filepath.Join(home, "$HOME")); err == nil {
		t.Error("ada direktori bernama harfiah $HOME; ekspansi shell tidak terjadi")
	}

	// Spec, termasuk yang bersarang, harus ikut terbawa.
	for _, p := range []string{"git.json", "aws/s3.json"} {
		full := filepath.Join(home, ".local", "share", "anjuran", "specs", filepath.FromSlash(p))
		if _, err := os.Stat(full); err != nil {
			t.Errorf("spec %s tidak terbawa", p)
		}
	}
}

// Menyalin bukan berarti bisa menjalankan. Home yang dipasang noexec atau
// berkas yang kehilangan bit eksekusinya baru ketahuan saat diperiksa.
func TestBinaryTidakBisaDijalankanTerdeteksi(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}

	src := t.TempDir()
	// Berkas yang bukan program sama sekali.
	if err := os.WriteFile(filepath.Join(src, "anjuran"), []byte("bukan program"), 0o755); err != nil {
		t.Fatal(err)
	}

	opt := Options{From: src, Base: ".local", Out: io.Discard}
	plan, cleanup, err := Prepare(context.Background(), tr, opt)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	err = Install(context.Background(), tr, plan, opt)
	if err == nil {
		t.Fatal("mau error karena binary tidak bisa dijalankan")
	}
	if !strings.Contains(err.Error(), "noexec") {
		t.Errorf("pesan harus menyebut sebab yang paling mungkin: %v", err)
	}
}

func TestVersiSamaTidakDikirimUlang(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}
	src := sumberPalsu(t)

	opt := Options{From: src, Base: ".local", Version: "9.9.9", Out: io.Discard}
	plan, cleanup, _ := Prepare(context.Background(), tr, opt)
	defer cleanup()
	if err := Install(context.Background(), tr, plan, opt); err != nil {
		t.Fatal(err)
	}

	plan2, cleanup2, err := Prepare(context.Background(), tr, opt)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup2()
	if !plan2.UpToDate {
		t.Errorf("host dengan versi sama seharusnya dilewati; terpasang = %q", plan2.Installed)
	}
}

func TestForceMemasangUlang(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}
	src := sumberPalsu(t)

	opt := Options{From: src, Base: ".local", Version: "9.9.9", Out: io.Discard}
	plan, cleanup, _ := Prepare(context.Background(), tr, opt)
	defer cleanup()
	Install(context.Background(), tr, plan, opt)

	opt.Force = true
	plan2, cleanup2, _ := Prepare(context.Background(), tr, opt)
	defer cleanup2()
	if plan2.UpToDate {
		t.Error("--force harus tetap memasang ulang")
	}
}

// Spec lama harus hilang, kalau tidak perintah yang sudah dicabut dari rilis
// baru akan tetap ditawarkan selamanya.
func TestSpecLamaDibersihkan(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}
	src := sumberPalsu(t)
	opt := Options{From: src, Base: ".local", Out: io.Discard}

	plan, cleanup, _ := Prepare(context.Background(), tr, opt)
	defer cleanup()
	Install(context.Background(), tr, plan, opt)

	usang := filepath.Join(home, ".local", "share", "anjuran", "specs", "usang.json")
	if err := os.WriteFile(usang, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan2, cleanup2, _ := Prepare(context.Background(), tr, opt)
	defer cleanup2()
	Install(context.Background(), tr, plan2, opt)

	if _, err := os.Stat(usang); err == nil {
		t.Error("spec lama seharusnya dibersihkan sebelum yang baru dipasang")
	}
}

func TestBaseDenganKarakterKhusus(t *testing.T) {
	skipDiWindows(t)

	home := t.TempDir()
	tr := &localTransport{home: home}
	src := sumberPalsu(t)

	// Nama direktori yang memuat spasi dan tanda dolar harus tetap harfiah.
	base := "dir dengan spasi"
	opt := Options{From: src, Base: base, Out: io.Discard}

	plan, cleanup, err := Prepare(context.Background(), tr, opt)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := Install(context.Background(), tr, plan, opt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, base, "bin", "anjuran")); err != nil {
		t.Errorf("path dengan spasi tidak ditangani: %v", err)
	}
}

func TestSumberTidakDitemukan(t *testing.T) {
	skipDiWindows(t)

	tr := &localTransport{home: t.TempDir()}
	opt := Options{From: t.TempDir(), Base: ".local", Out: io.Discard}

	_, cleanup, err := Prepare(context.Background(), tr, opt)
	defer cleanup()
	if err == nil {
		t.Fatal("mau error untuk direktori sumber yang kosong")
	}
}

func TestPlatformBerbedaTanpaSumber(t *testing.T) {
	_, cleanup, err := ResolveSource("", Platform{OS: "plan9", Arch: "amd64"}, nil, nil)
	defer cleanup()
	if err == nil {
		t.Fatal("mau error")
	}
	// Pesannya harus memberi tahu jalan keluarnya, bukan sekadar menolak.
	if !strings.Contains(err.Error(), "--from") {
		t.Errorf("pesan harus menyebut jalan keluar: %v", err)
	}
}
