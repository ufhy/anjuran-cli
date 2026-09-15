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
		for _, want := range []string{"uf widget", "--line", "--cursor"} {
			if !strings.Contains(s, want) {
				t.Errorf("skrip %s tidak memuat %q", sh, want)
			}
		}
		// Ketiga status harus ditangani; "none" khususnya, karena itulah yang
		// mengembalikan tombol ke completion bawaan shell.
		if !strings.Contains(s, "none") {
			t.Errorf("skrip %s tidak menangani status none", sh)
		}
		if !strings.Contains(s, "UF_KEY") {
			t.Errorf("skrip %s tidak menghormati UF_KEY", sh)
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

// Dropdown otomatis dipicu SPASI, bukan setiap huruf. Setelah sebuah kata
// selesai barulah ada yang bisa ditawarkan, dan biayanya hanya dibayar di
// tempat yang jarang ditekan.
func TestZshOtomatisDipicuSpasi(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{
		"UF_AUTO",      // harus opt-in
		"uf render",    // memakai mode gambar-saja
		"--prev-lines", // zsh yang menyimpan jumlah barisnya
		"zle .self-insert",
		"TRAPINT", // Ctrl-C tidak pernah sampai ke widget
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Shell lain tidak punya hook per-ketikan yang layak, jadi tidak boleh
// berpura-pura punya mode otomatis.
func TestShellLainTanpaModeOtomatis(t *testing.T) {
	for _, sh := range []string{"bash", "fish", "powershell"} {
		s, _ := Script(sh)
		if strings.Contains(s, "uf render") {
			t.Errorf("skrip %s seharusnya belum memakai mode gambar-saja", sh)
		}
	}
}

// Spasi belum tentu terpasang ke self-insert: oh-my-zsh memetakannya ke
// magic-space. Membungkus self-insert saja berarti fitur ini mati diam-diam
// di konfigurasi yang justru paling banyak dipakai.
func TestZshMembungkusWidgetSpasiYangAda(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{
		`bindkey ' '`,           // menanyakan widget yang sedang terpasang
		`bindkey " " _uf_space`, // memasang pembungkusnya
		"magic-space",           // alasannya ditulis, bukan sekadar dikerjakan
		"viins",                 // mode vi memakai keymap terpisah
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Alias harus diteruskan ke uf, kalau tidak "gco" tidak menghasilkan apa pun.
func TestZshMeneruskanAlias(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{
		"--alias",       // diteruskan ke uf
		"${aliases[",    // dibaca dari tabel alias zsh
		"_uf_alias_exp", // lewat variabel, bukan subshell
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
	// Dipakai oleh kedua jalur: Tab dan dropdown otomatis.
	if n := strings.Count(s, "--alias"); n < 2 {
		t.Errorf("--alias dipakai %d kali, mau di jalur widget dan render", n)
	}
}
