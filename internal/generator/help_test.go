package generator

import (
	"os/exec"
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

	s := &Source{Dir: t.TempDir(), Timeout: 5 * time.Second}
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
