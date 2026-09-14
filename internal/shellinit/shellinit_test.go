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

// READLINE_POINT milik bash dihitung dalam byte, tidak seperti shell lain.
func TestSkripBashMemintaSatuanByte(t *testing.T) {
	s, _ := Script("bash")
	if !strings.Contains(s, "--cursor-unit byte") {
		t.Error("skrip bash harus meminta satuan byte untuk READLINE_POINT")
	}
}

func TestSkripSelainBashMemakaiSatuanRune(t *testing.T) {
	for _, sh := range []string{"zsh", "fish"} {
		s, _ := Script(sh)
		if strings.Contains(s, "--cursor-unit byte") {
			t.Errorf("skrip %s tidak boleh meminta satuan byte", sh)
		}
	}
}
