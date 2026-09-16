package generator

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/ui"
)

// Penyapuan lanjutan: sifat-sifat yang belum diperiksa penyapuan pertama.

// TestSapuLoadSpecUtuh memeriksa setiap rujukan loadSpec benar-benar ada.
//
// Ada 750 rujukan semacam itu. Satu saja yang menunjuk berkas yang tidak ada
// membuat cabang perintahnya diam, dan diamnya tidak menghasilkan pesan apa
// pun — persis kegagalan yang paling sulit disadari.
func TestSapuLoadSpecUtuh(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "cabang loadSpec tidak menghasilkan apa pun"}

	for _, name := range names {
		line := name + " "
		res, err := eng.Complete(line, len(line))
		if err != nil {
			t.Fatalf("Complete(%q): %v", line, err)
		}

		// Telusuri setiap subcommand satu tingkat; yang memakai loadSpec akan
		// memuat berkasnya di sini.
		for _, c := range res.Candidates {
			if c.Kind != engine.KindSubcommand {
				continue
			}
			sub := name + " " + c.Name + " "
			r2, err := eng.Complete(sub, len(sub))
			if err != nil {
				m.catat(fmt.Sprintf("%q: %v", sub, err))
				continue
			}
			if len(r2.Candidates) == 0 && len(r2.Generators) == 0 && len(r2.Templates) == 0 {
				m.catat(fmt.Sprintf("%q tidak menghasilkan apa pun", sub))
			}
		}
	}
	m.lapor(t)
}

// TestSapuDeskripsiAman memeriksa keterangan yang akan digambar di dropdown.
//
// Newline atau karakter kendali di dalam keterangan akan merusak kotaknya:
// baris meleset, bingkai patah, dan sisa gambar tertinggal di layar.
func TestSapuDeskripsiAman(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "keterangan memuat karakter yang merusak gambar"}

	for _, name := range names {
		for _, suffix := range []string{" ", " -"} {
			line := name + suffix
			res, _ := eng.Complete(line, len(line))
			for _, c := range res.Candidates {
				for _, r := range c.Description {
					if unicode.IsControl(r) {
						m.catat(fmt.Sprintf("%q: keterangan %q memuat %q", line, c.Name, r))
						break
					}
				}
			}
		}
	}
	m.lapor(t)
}

// TestSapuNamaKandidatMasukAkal memeriksa nama yang akan ditampilkan.
func TestSapuNamaKandidatMasukAkal(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "nama kandidat tidak masuk akal"}

	for _, name := range names {
		line := name + " "
		res, _ := eng.Complete(line, len(line))
		for _, c := range res.Candidates {
			switch {
			case strings.TrimSpace(c.Name) == "":
				m.catat(fmt.Sprintf("%q: ada kandidat tanpa nama", line))
			case strings.ContainsAny(c.Name, "\n\r\t"):
				m.catat(fmt.Sprintf("%q: nama %q memuat baris baru", line, c.Name))
			}
		}
	}
	m.lapor(t)
}

// TestSapuArgumenOpsi memeriksa posisi tepat SESUDAH sebuah opsi berargumen.
//
// Di situ yang diharapkan adalah nilai untuk opsi itu, bukan daftar subcommand.
// Salah di sini membuat pengguna memilih sesuatu yang tidak pernah sah.
func TestSapuArgumenOpsi(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "posisi argumen opsi menawarkan subcommand"}

	for _, name := range names {
		line := name + " "
		res, _ := eng.Complete(line, len(line))

		// Cari satu opsi yang jelas menerima argumen.
		var opsi string
		for _, c := range res.Candidates {
			if c.Kind == engine.KindOption && strings.HasSuffix(c.Insert, "=") {
				opsi = c.Name
				break
			}
		}
		if opsi == "" {
			continue
		}

		probe := name + " " + opsi + "="
		r2, err := eng.Complete(probe, len(probe))
		if err != nil {
			m.catat(fmt.Sprintf("%q: %v", probe, err))
			continue
		}
		for _, c := range r2.Candidates {
			if c.Kind == engine.KindSubcommand {
				m.catat(fmt.Sprintf("%q menawarkan subcommand %q", probe, c.Name))
				break
			}
		}
	}
	m.lapor(t)
}

// TestSapuKursorDiTengah menjalankan completion dengan kursor di SETIAP posisi
// baris, bukan hanya di ujungnya.
//
// Pengguna sering kembali ke tengah baris untuk memperbaiki sesuatu, dan di
// situlah perhitungan rentang penggantian paling mudah meleset.
func TestSapuKursorDiTengah(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "kursor di tengah baris menghasilkan rentang salah"}

	for _, name := range names {
		line := name + " sub --opt nilai --lain"
		for cursor := 0; cursor <= len(line); cursor++ {
			res, err := eng.Complete(line, cursor)
			if err != nil {
				m.catat(fmt.Sprintf("%q@%d: %v", line, cursor, err))
				break
			}
			if res.ReplaceStart > cursor || res.ReplaceEnd < cursor {
				m.catat(fmt.Sprintf("%q@%d: rentang [%d,%d) tidak memuat kursor",
					line, cursor, res.ReplaceStart, res.ReplaceEnd))
				break
			}
		}
	}
	m.lapor(t)
}

// TestSapuAcak melempar masukan acak ke engine.
//
// Pengguna menempelkan segala macam ke baris perintah. Yang diuji di sini
// bukan hasilnya, melainkan bahwa tidak ada masukan yang bisa menjatuhkan
// proses anjuran di tengah pengetikan.
func TestSapuAcak(t *testing.T) {
	eng, names := sweepEngine(t)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	huruf := []rune(` "'\|&;<>()*?[]{}$` + "`" + `~/-=abc日本語` + "\U0001F680\t")

	for i := 0; i < 4000; i++ {
		var b strings.Builder
		b.WriteString(names[rng.Intn(len(names))])
		b.WriteByte(' ')
		for n := rng.Intn(24); n > 0; n-- {
			b.WriteRune(huruf[rng.Intn(len(huruf))])
		}
		line := b.String()

		cursor := rng.Intn(len(line) + 3)
		res, err := eng.Complete(line, cursor)
		if err != nil {
			t.Fatalf("Complete(%q, %d): %v", line, cursor, err)
		}
		if res.ReplaceStart < 0 || res.ReplaceEnd > len(line) || res.ReplaceStart > res.ReplaceEnd {
			t.Fatalf("Complete(%q, %d): rentang [%d,%d) tidak sah",
				line, cursor, res.ReplaceStart, res.ReplaceEnd)
		}
	}
}

// TestSapuKotakRapi menggambar kotak dari kandidat SUNGGUHAN milik setiap spec
// lalu memeriksa seluruh barisnya sama lebar di layar.
//
// Keterangan berasal dari korpus pihak ketiga dan memuat apa saja — CJK,
// emoji, tanda gabung. Contoh yang dipilih tangan tidak akan menemukan yang
// mana; menggambar semuanya akan.
func TestSapuKotakRapi(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "baris kotak tidak sama lebar"}

	for _, name := range names {
		line := name + " "
		res, err := eng.Complete(line, len(line))
		if err != nil || len(res.Candidates) == 0 {
			continue
		}

		items := make([]ui.Item, 0, 10)
		for _, c := range res.Candidates {
			if len(items) == 10 {
				break
			}
			items = append(items, ui.Item{
				Name:        c.Label(),
				Description: c.Description,
				Kind:        string(c.Kind),
			})
		}

		for _, width := range []int{60, 100} {
			lines := ui.ComposeForTest(width, items, 0, 1, len(res.Candidates))
			if len(lines) == 0 {
				continue
			}
			want := ui.DisplayWidth(lines[0])
			for i, l := range lines {
				if got := ui.DisplayWidth(l); got != want {
					m.catat(fmt.Sprintf("%q lebar %d: baris %d selebar %d kolom, mau %d",
						line, width, i, got, want))
					break
				}
			}
			if want > width {
				m.catat(fmt.Sprintf("%q: kotak selebar %d melebihi terminal %d", line, want, width))
			}
		}
	}
	m.lapor(t)
}
