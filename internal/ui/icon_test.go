package ui

import (
	"strings"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/engine"
)

func TestIkonUntukTiapJenis(t *testing.T) {
	for _, u := range []struct {
		nama string
		c    engine.Candidate
		mode string
		mau  string
	}{
		{"subcommand", engine.Candidate{Kind: engine.KindSubcommand}, IkonAman, "▪"},
		{"opsi", engine.Candidate{Kind: engine.KindOption}, IkonAman, "▫"},
		{"berkas", engine.Candidate{Kind: engine.KindArg}, IkonAman, "·"},
		{"direktori", engine.Candidate{Kind: engine.KindArg, Name: "proyek/"}, IkonAman, "▸"},
		{"direktori nerd", engine.Candidate{Kind: engine.KindArg, Name: "proyek/"}, IkonNerd, ""},
		// Baris "berhenti" namanya SUDAH berupa ikon; menggambarnya dua kali
		// hanya mengulang hal yang sama.
		{"berhenti", engine.Candidate{Kind: engine.KindBerhenti}, IkonAman, ""},
		{"dimatikan", engine.Candidate{Kind: engine.KindSubcommand}, IkonMati, ""},
	} {
		if got := ikonUntuk(u.c, u.mode); got != u.mau {
			t.Errorf("%s: ikon = %q, mau %q", u.nama, got, u.mau)
		}
	}
}

// Setiap ikon harus selebar SATU kolom.
//
// Lebar yang meleset satu kolom saja sudah cukup mematahkan bingkai kotaknya,
// dan itu berlaku untuk seluruh baris di bawahnya.
func TestIkonSelaluSatuKolom(t *testing.T) {
	semua := []string{direktoriAman, direktoriNerd}
	for _, m := range []map[engine.Kind]string{ikonAman, ikonNerd} {
		for _, v := range m {
			semua = append(semua, v)
		}
	}
	for _, ikon := range semua {
		if n := textWidth(ikon); n != 1 {
			t.Errorf("ikon %q selebar %d kolom, mau 1", ikon, n)
		}
	}
}

// Ikon ikut diperhitungkan saat menyusun kolom, sehingga nama terpanjang tetap
// muat utuh dan bingkainya tetap lurus.
func TestIkonTidakMemotongNama(t *testing.T) {
	var b strings.Builder
	r := NewRenderer(&b, 80, 24, false)
	if err := r.Render([]Item{
		{Name: "commit", Icon: "▪"},
		{Name: "cherry-pick", Icon: "▪"},
	}, 0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "cherry-pick") {
		t.Errorf("nama terpanjang terpotong:\n%s", b.String())
	}
	// Seluruh baris kotak harus sama panjangnya.
	var lebar []int
	for _, baris := range strings.Split(b.String(), "\x1b8") {
		if i := strings.Index(baris, "│"); i >= 0 {
			lebar = append(lebar, textWidth(buangEsc(baris)))
		}
	}
	for i := 1; i < len(lebar); i++ {
		if lebar[i] != lebar[0] {
			t.Errorf("lebar baris tidak seragam: %v", lebar)
			break
		}
	}
}

// buangEsc menyisakan karakter yang benar-benar tergambar.
func buangEsc(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && !(s[i] >= '@' && s[i] <= '~' && i > 0 && s[i-1] != 0x1b) {
				i++
			}
			continue
		}
		if s[i] == '\r' {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
