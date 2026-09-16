package shellinit

import (
	"strings"
	"testing"
)

func TestSemuaShellPunyaSkrip(t *testing.T) {
	for _, sh := range Shells() {
		s, err := Script(sh)
		if err != nil {
			t.Errorf("Script(%q): %v", sh, err)
			continue
		}
		if len(s) < 200 {
			t.Errorf("skrip %s terlalu pendek (%d byte), kemungkinan gagal disematkan", sh, len(s))
		}
	}
}

func TestShellTakDikenalMenghasilkanError(t *testing.T) {
	if _, err := Script("tcsh"); err == nil {
		t.Error("mau error untuk shell yang tidak didukung")
	}
}

// Protokol widget hanya ada satu, jadi setiap skrip harus memanggilnya dengan
// cara yang sama. Uji ini menangkap skrip yang tertinggal saat protokolnya
// berubah.
func TestSetiapSkripMematuhiProtokol(t *testing.T) {
	for _, sh := range Shells() {
		s, _ := Script(sh)
		for _, want := range []string{"anjuran widget", "--line", "--cursor"} {
			if !strings.Contains(s, want) {
				t.Errorf("skrip %s tidak memuat %q", sh, want)
			}
		}
		// Ketiga status harus ditangani; "none" khususnya, karena itulah yang
		// mengembalikan tombol ke completion bawaan shell.
		if !strings.Contains(s, "none") {
			t.Errorf("skrip %s tidak menangani status none", sh)
		}
		if !strings.Contains(s, "ANJURAN_KEY") {
			t.Errorf("skrip %s tidak menghormati ANJURAN_KEY", sh)
		}
	}
}

// Setiap keluarga shell melaporkan posisi kursor dengan satuan berbeda, dan
// skrip yang salah meminta satuan akan menyisipkan di posisi keliru begitu
// baris memuat huruf non-ASCII.
func TestSatuanKursorPerShell(t *testing.T) {
	want := map[string]string{
		"bash":       "--cursor-unit byte",  // READLINE_POINT menghitung byte
		"powershell": "--cursor-unit utf16", // indeks string .NET
		"zsh":        "",                    // rune, yaitu nilai bawaan
		"fish":       "",
	}
	for sh, flag := range want {
		s, err := Script(sh)
		if err != nil {
			t.Fatalf("Script(%q): %v", sh, err)
		}
		if flag == "" {
			if strings.Contains(s, "--cursor-unit") {
				t.Errorf("skrip %s memakai satuan rune, seharusnya tidak menyebut --cursor-unit", sh)
			}
			continue
		}
		if !strings.Contains(s, flag) {
			t.Errorf("skrip %s harus memuat %q", sh, flag)
		}
	}
}

// pwsh adalah nama biner PowerShell 6 ke atas dan harus mengarah ke skrip yang
// sama, karena deteksi dari $SHELL akan menemukan nama itu.
func TestPwshAliasKePowershell(t *testing.T) {
	a, err := Script("pwsh")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Script("powershell")
	if a != b {
		t.Error("pwsh dan powershell harus memakai skrip yang sama")
	}
	for _, sh := range Shells() {
		if sh == "pwsh" {
			t.Error("pwsh tidak boleh muncul dua kali di daftar shell")
		}
	}
}

// Pemicunya bukan satu tombol, melainkan titik-titik di mana ada sesuatu yang
// layak ditawarkan — mengikuti cara IDE bekerja.
func TestZshPemicu(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{
		// Nyala secara bawaan, dimatikan dengan ANJURAN_AUTO=0. Sempat harus
		// dinyalakan sendiri, dan itu membuat mode utama alat ini tersembunyi
		// di balik variabel yang harus diketahui namanya lebih dulu.
		`${ANJURAN_AUTO:-1} != (0|no|off|false)`,
		`zle -N self-insert _anjuran_ketik`, // mengetik kata juga membuka kotak
		// Menghapus juga membukanya: sesudah salah ketiklah saran paling
		// dibutuhkan.
		`zle -N backward-delete-char _anjuran_hapus`,
		`bindkey " " _anjuran_spasi`, // spasi
		`bindkey "/" _anjuran_garismiring`,
		`bindkey "=" _anjuran_samadengan`,
		"viins", // mode vi memakai keymap terpisah
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Widget yang sudah terpasang pada tombol pemicu dipanggil lebih dulu, supaya
// perilakunya tetap utuh: oh-my-zsh memetakan spasi ke magic-space.
func TestZshMembungkusWidgetYangAda(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{"bindkey ", "magic-space", "_anjuran_asli"} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Tombol yang bukan urusan dropdown dikembalikan ke antrean masukan zsh.
// Tanpa itu, sesi yang memegang masukan akan menelan tombol seperti Ctrl-A.
func TestZshMengembalikanTombolSisa(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{"zle -U", "_anjuran_kembalikan"} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Shell lain belum punya pemicu otomatis; mereka hanya memakai Tab.
func TestShellLainTanpaPemicuOtomatis(t *testing.T) {
	for _, sh := range []string{"bash", "fish", "powershell"} {
		s, _ := Script(sh)
		if strings.Contains(s, "ANJURAN_AUTO") {
			t.Errorf("skrip %s seharusnya belum punya pemicu otomatis", sh)
		}
	}
}

// Spasi belum tentu terpasang ke self-insert: oh-my-zsh memetakannya ke
// magic-space. Membungkus self-insert saja berarti fitur ini mati diam-diam
// di konfigurasi yang justru paling banyak dipakai.
