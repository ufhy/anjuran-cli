package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ufhy/anjuran-cli/internal/engine"
)

func namaKandidat(cs []engine.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func ketDari(cs []engine.Candidate, nama string) string {
	for _, c := range cs {
		if c.Name == nama {
			return c.Description
		}
	}
	return ""
}

// Bentuk keluaran --help berbeda-beda, dan justru itu masalahnya. Tiga gaya
// di bawah mewakili hampir semua alat yang dipakai sehari-hari: getopt gaya
// GNU, cobra gaya Go, dan clap gaya Rust.
func TestUraiFlagGayaUmum(t *testing.T) {
	tests := []struct {
		nama string
		teks string
		mau  []string
	}{
		{
			nama: "GNU",
			teks: "Usage: ls [OPTION]... [FILE]...\n" +
				"  -a, --all                  do not ignore entries starting with .\n" +
				"  -l                         use a long listing format\n" +
				"      --color=WHEN           colorize the output\n",
			mau: []string{"--all", "-a", "-l", "--color"},
		},
		{
			nama: "cobra",
			teks: "Flags:\n" +
				"  -f, --file string   berkas masukan\n" +
				"      --verbose       keluaran rinci\n",
			mau: []string{"--file", "-f", "--verbose"},
		},
		{
			nama: "clap",
			teks: "OPTIONS:\n" +
				"    -j, --jobs <N>        jumlah pekerjaan paralel\n" +
				"        --offline         jangan akses jaringan\n",
			mau: []string{"--jobs", "-j", "--offline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			got := namaKandidat(uraiBantuan(tt.teks, true))
			for _, mau := range tt.mau {
				if !has(got, mau) {
					t.Errorf("tidak memuat %q; dapat %v", mau, got)
				}
			}
		})
	}
}

// Keterangan flag adalah separuh nilainya: "--color" sendirian tidak memberi
// tahu apa pun.
func TestKeteranganFlagIkutTerbaca(t *testing.T) {
	teks := "  -a, --all                  do not ignore entries starting with .\n"
	cs := uraiBantuan(teks, true)
	if ket := ketDari(cs, "--all"); ket != "do not ignore entries starting with ." {
		t.Errorf("keterangan --all = %q", ket)
	}
	// Bentuk pendeknya berbagi keterangan yang sama; keduanya baris yang sama.
	if ket := ketDari(cs, "-a"); ket == "" {
		t.Error("bentuk pendek kehilangan keterangannya")
	}
}

// Subcommand hanya dibaca DI DALAM bagiannya.
//
// Tanpa batas itu, contoh pemakaian di bagian atas halaman — yang juga
// berupa baris menjorok berisi satu kata — ikut terbaca sebagai subcommand.
func TestSubcommandHanyaDariBagiannya(t *testing.T) {
	teks := "Usage: docker [OPTIONS] COMMAND\n" +
		"\n" +
		"Examples:\n" +
		"  contoh-yang-bukan-subcommand\n" +
		"\n" +
		"Management Commands:\n" +
		"  builder     Manage builds\n" +
		"  compose     Docker Compose\n" +
		"\n" +
		"Global Options:\n" +
		"  --debug     Enable debug mode\n"

	got := namaKandidat(uraiBantuan(teks, false))
	for _, mau := range []string{"builder", "compose"} {
		if !has(got, mau) {
			t.Errorf("tidak memuat %q; dapat %v", mau, got)
		}
	}
	if has(got, "contoh-yang-bukan-subcommand") {
		t.Errorf("contoh pemakaian ikut terbaca sebagai subcommand: %v", got)
	}
	if has(got, "--debug") {
		t.Errorf("flag ikut terbaca sebagai subcommand: %v", got)
	}
}

// Kandidat dari --help berada DI ATAS kandidat yang hanya diketahui spec.
//
// Spec menggambarkan versi perintah pada Mei 2025, ketika korpusnya berhenti
// dirawat. --help menggambarkan versi yang benar-benar terpasang. Bila
// keduanya berbeda, yang terpasanglah yang benar.
func TestPrioritasDiAtasSpec(t *testing.T) {
	cs := uraiBantuan("  --verbose   rinci\n", true)
	if len(cs) == 0 {
		t.Fatal("tidak menghasilkan kandidat")
	}
	if cs[0].Priority <= engine.DefaultPriority {
		t.Errorf("Priority = %d, harus di atas %d", cs[0].Priority, engine.DefaultPriority)
	}
}

// Keterangan yang sangat panjang dipangkas: kolom kanan punya lebar terbatas,
// dan baris yang meluber merusak bingkai kotaknya.
func TestKeteranganPanjangDipangkas(t *testing.T) {
	panjang := ""
	for i := 0; i < 40; i++ {
		panjang += "katapanjang "
	}
	cs := uraiBantuan("  --x  "+panjang+"\n", true)
	if len(cs) == 0 {
		t.Fatal("tidak menghasilkan kandidat")
	}
	if len(cs[0].Description) > 120 {
		t.Errorf("keterangan %d karakter, harus dipangkas", len(cs[0].Description))
	}
}

// Entri spec yang dibenarkan --help naik peringkat, TETAPI entrinya sendiri
// tidak diganti.
//
// Di entri spec itulah tipe argumen dan generator tersimpan. Menukarnya
// dengan hasil uraian teks bebas berarti `git checkout ` kehilangan daftar
// branch-nya — jadi yang didahulukan hanya urutannya, bukan isinya.
func TestBantuanMengangkatEntriSpecTanpaMenggantinya(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("perlu `go` di PATH")
	}

	res := &engine.Result{
		Command: "go",
		Path:    []string{"go"},
		Candidates: []engine.Candidate{
			// `go help` menyebut "build"; ini yang harus terangkat.
			{Name: "build", Insert: "build", Description: "dari spec", Priority: engine.DefaultPriority},
			// Tidak disebut `go help`: peringkatnya tidak boleh berubah.
			{Name: "hanya-di-spec", Insert: "hanya-di-spec", Priority: engine.DefaultPriority},
		},
	}

	// Kebijakan generator mematikan dirinya saat berjalan sebagai root, dan
	// container pengembangan biasanya root. Tanpa baris ini uji ini LOLOS
	// tanpa pernah menjalankan satu proses pun — lolos yang tidak membuktikan
	// apa-apa, yang lebih buruk daripada gagal.
	t.Setenv("ANJURAN_GENERATOR_ALLOW_ROOT", "1")

	// Dipanggil DUA KALI, dan itu bagian dari kontraknya.
	//
	// Bantuan tidak pernah menahan jalur panas: prosesnya dilepas ke latar dan
	// hasilnya dipanen ketikan berikutnya, saat jawabannya sudah ada di cache.
	// Panggilan pertama di sini hanya menghangatkannya.
	s := &Source{Dir: t.TempDir(), Cache: &Cache{Dir: t.TempDir()}}
	s.Candidates(res)
	tungguCache(t, s, []string{"go", "--help"})

	res.Candidates[0].Priority = engine.DefaultPriority
	out := s.Candidates(res)

	if got := res.Candidates[0].Priority; got != BantuanPrioritas {
		t.Errorf("entri spec yang dibenarkan --help: Priority = %d, mau %d", got, BantuanPrioritas)
	}
	if got := res.Candidates[0].Description; got != "dari spec" {
		t.Errorf("entri spec diganti isinya: Description = %q", got)
	}
	if got := res.Candidates[1].Priority; got != engine.DefaultPriority {
		t.Errorf("entri yang tak disebut --help ikut terangkat: Priority = %d", got)
	}
	for _, c := range out {
		if c.Name == "build" {
			t.Error("nama yang sudah ada di spec ditambahkan lagi sebagai kandidat kedua")
		}
	}
}

// Biner yang bantuannya lambat TIDAK BOLEH menahan jalur panas.
//
// Karakter yang diketik digemakan ke layar sebelum kandidatnya dihitung
// ulang, jadi menunggu proses asing di sini menghasilkan baris yang terus
// terisi di atas kotak yang membeku. Itu terlihat persis seperti alat yang
// rusak — dan memang pernah terjadi: `kubectl get -h` pada mesin tanpa
// kubeconfig menghabiskan detik, dan seluruh kotaknya ikut berhenti.
func TestBantuanLambatTidakMenahanJalurPanas(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skrip sh tidak berlaku di Windows")
	}
	// Lihat catatan yang sama di uji di atas: tanpa ini, kebijakan menolak
	// lebih dulu dan uji ini lolos tanpa menunggu apa pun.
	t.Setenv("ANJURAN_GENERATOR_ALLOW_ROOT", "1")

	dir := t.TempDir()
	lambat := filepath.Join(dir, "lambat")
	if err := os.WriteFile(lambat, []byte("#!/bin/sh\nsleep 5\necho '  --x  y'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	s := &Source{Dir: dir, Cache: &Cache{Dir: t.TempDir()}}

	mulai := time.Now()
	s.Bantuan([]string{"lambat"}, true)
	lama := time.Since(mulai)

	// Dua bentuk dicoba, masing-masing menunggu paling lama sekejap.
	batas := 3 * bantuanTungguDepan
	if lama > batas {
		t.Errorf("menunggu %v, batasnya %v — jalur panas tertahan proses asing", lama, batas)
	}
}

// tungguCache menanti proses latar selesai mengisi cache.
//
// Menunggu KEADAAN, bukan angka: tidur dengan durasi tetap akan lolos di
// mesin lengang dan gagal di mesin sibuk, dan uji yang gagal sesekali
// berhenti dipercaya.
func tungguCache(t *testing.T, s *Source, cmd []string) {
	t.Helper()
	batas := time.Now().Add(10 * time.Second)
	for time.Now().Before(batas) {
		if _, ok := s.Cache.Get(cmd, ""); ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("cache bantuan untuk %v tidak pernah terisi", cmd)
}
