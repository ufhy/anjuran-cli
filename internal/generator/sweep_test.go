package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/parser"
	"github.com/ufhy/anjuran-cli/internal/spec"
)

// Penyapuan korpus.
//
// Berbeda dari uji cakupan, yang memeriksa daftar perintah pilihan, berkas ini
// menjalankan SELURUH spec yang terpasang dan memeriksa sifat yang harus benar
// untuk semuanya. Contoh yang dipilih tangan hanya menemukan yang sudah
// terpikirkan; penyapuan menemukan yang tidak.
//
// Generator sengaja TIDAK dijalankan di sini: yang diuji adalah engine dan
// spec, dan menumbuhkan ribuan proses hanya akan membuat uji ini lambat serta
// bergantung pada mesin tempatnya berjalan.

func sweepEngine(t *testing.T) (*engine.Engine, []string) {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	specs := filepath.Join(root, "specs")
	entries, err := os.ReadDir(specs)
	if err != nil {
		t.Skip("direktori specs/ belum dibangun; jalankan `make specs`")
	}

	extra := filepath.Join(root, "extra")
	r := spec.NewRegistryDirs(extra, specs)
	r.Trust(extra)

	var names []string
	for _, e := range entries {
		n := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".gz"), ".json")
		if e.IsDir() || n == e.Name() || strings.HasPrefix(n, ".") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return engine.New(r).InDir(root), names
}

// masalah mengumpulkan temuan agar dilaporkan sekaligus, bukan berhenti di
// yang pertama: satu penyapuan harus memberi gambaran utuh.
type masalah struct {
	jenis  string
	contoh []string
	jumlah int
}

func (m *masalah) catat(s string) {
	m.jumlah++
	if len(m.contoh) < 5 {
		m.contoh = append(m.contoh, s)
	}
}

func (m *masalah) lapor(t *testing.T) {
	t.Helper()
	if m.jumlah == 0 {
		return
	}
	t.Errorf("%s: %d kejadian\n  %s", m.jenis, m.jumlah, strings.Join(m.contoh, "\n  "))
}

// TestSapuKandidatBisaDisisipkan memeriksa bahwa setiap kandidat benar-benar
// bisa dimasukkan ke baris perintah.
//
// Yang diperiksa bukan "tidak ada spasi" — sebagian kandidat memang berisi
// spasi dan sah selama dikutip. Yang diperiksa adalah hasilnya tetap terurai
// utuh oleh tokenizer shell: kutip yang tidak tertutup akan menggantung baris
// perintah pengguna, dan karakter kendali merusak tampilannya.
func TestSapuKandidatBisaDisisipkan(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "kandidat tidak bisa disisipkan"}

	for _, name := range names {
		for _, suffix := range []string{" ", " -", " --"} {
			line := name + suffix
			res, err := eng.Complete(line, len(line))
			if err != nil {
				t.Fatalf("Complete(%q): %v", line, err)
			}
			for _, c := range res.Candidates {
				if c.Insert == "" {
					m.catat(fmt.Sprintf("%q: kandidat %q menyisipkan teks kosong", line, c.Name))
					continue
				}
				for _, r := range c.Insert {
					if unicode.IsControl(r) {
						m.catat(fmt.Sprintf("%q: %q memuat karakter kendali %q", line, c.Insert, r))
						break
					}
				}

				// Kutip yang tidak tertutup menggantung baris perintah.
				for _, tok := range parser.Parse(c.Insert, len(c.Insert)).Tokens {
					if !tok.Terminated {
						m.catat(fmt.Sprintf("%q: %q punya kutip yang tidak tertutup", line, c.Insert))
						break
					}
				}

				// Kandidat tanpa insertValue harus menjadi SATU kata; kalau
				// tidak, shell memecahnya dan perintahnya rusak.
				if c.Insert == engine.Quote(c.Name) {
					if n := len(parser.Parse(c.Insert, 0).Tokens); n != 1 {
						m.catat(fmt.Sprintf("%q: %q terurai menjadi %d kata", line, c.Insert, n))
					}
				}
			}
		}
	}
	m.lapor(t)
}

// TestSapuRentangPenggantian memeriksa rentang byte yang akan diganti.
//
// Rentang di luar batas akan memotong baris di tempat yang salah, atau membuat
// shell menyisipkan di posisi yang tidak masuk akal.
func TestSapuRentangPenggantian(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "rentang penggantian tidak sah"}

	for _, name := range names {
		for _, line := range []string{name + " ", name + " --ver", name + " sub arg"} {
			for _, cursor := range []int{len(line), len(line) / 2, 0} {
				res, err := eng.Complete(line, cursor)
				if err != nil {
					t.Fatalf("Complete(%q, %d): %v", line, cursor, err)
				}
				switch {
				case res.ReplaceStart < 0 || res.ReplaceEnd < 0:
					m.catat(fmt.Sprintf("%q@%d: rentang negatif [%d,%d)", line, cursor, res.ReplaceStart, res.ReplaceEnd))
				case res.ReplaceStart > res.ReplaceEnd:
					m.catat(fmt.Sprintf("%q@%d: awal melewati akhir [%d,%d)", line, cursor, res.ReplaceStart, res.ReplaceEnd))
				case res.ReplaceEnd > len(line):
					m.catat(fmt.Sprintf("%q@%d: akhir melewati panjang baris [%d,%d)", line, cursor, res.ReplaceStart, res.ReplaceEnd))
				}
			}
		}
	}
	m.lapor(t)
}

// TestSapuPenyaringanPrefix memeriksa bahwa prefix benar-benar menyaring.
//
// Kandidat yang tidak cocok dengan yang sudah diketik adalah kandidat yang
// tidak mungkin dimaksud pengguna.
func TestSapuPenyaringanPrefix(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "kandidat tidak cocok dengan prefix"}

	for _, name := range names {
		line := name + " --zzq"
		res, err := eng.Complete(line, len(line))
		if err != nil {
			t.Fatalf("Complete(%q): %v", line, err)
		}
		for _, c := range res.Candidates {
			if !strings.HasPrefix(strings.ToLower(c.Name), "--zzq") {
				m.catat(fmt.Sprintf("%q: %q tidak berawalan prefix", line, c.Name))
			}
		}
	}
	m.lapor(t)
}

// TestSapuTanpaKandidatGanda memeriksa tidak ada entri kembar dalam satu
// daftar. Entri kembar membuat panah terasa macet: menekan bawah tidak
// mengubah apa pun yang terlihat.
func TestSapuTanpaKandidatGanda(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "kandidat kembar"}

	for _, name := range names {
		line := name + " "
		res, err := eng.Complete(line, len(line))
		if err != nil {
			t.Fatalf("Complete(%q): %v", line, err)
		}
		seen := map[string]bool{}
		for _, c := range res.Candidates {
			key := string(c.Kind) + "\x00" + c.Name
			if seen[key] {
				m.catat(fmt.Sprintf("%q: %q (%s) muncul lebih dari sekali", line, c.Name, c.Kind))
			}
			seen[key] = true
		}
	}
	m.lapor(t)
}

// TestSapuMasukanAneh memastikan engine tidak pernah panik atau gagal pada
// baris yang tidak lazim. Pengguna mengetik hal-hal aneh setiap hari: kutip
// yang belum ditutup, pipa berganda, karakter non-ASCII.
func TestSapuMasukanAneh(t *testing.T) {
	eng, names := sweepEngine(t)

	pola := []string{
		"%s", "%s ", "%s  ", "%s -", "%s --", "%s ---", "%s -x -x -x",
		`%s "`, `%s '`, `%s "belum ditutup`, `%s \`, `%s a\ b `,
		"%s | ", "%s || ", "%s && ", "%s ; ", "%s |||",
		"%s café ", "%s 日本語 ", "%s \U0001F680 ",
		"%s =", "%s --opt=", "%s --opt=nilai ", "%s -- ",
		"%s ~", "%s ~/", "%s /", "%s ./", "%s ../",
		"%s $(", "%s `", "%s *", "%s ?",
	}

	for _, name := range names {
		for _, p := range pola {
			line := fmt.Sprintf(p, name)
			for _, cursor := range []int{0, len(line) / 3, len(line) / 2, len(line), len(line) + 5, -1} {
				res, err := eng.Complete(line, cursor)
				if err != nil {
					t.Fatalf("Complete(%q, %d): %v", line, cursor, err)
				}
				if res == nil {
					t.Fatalf("Complete(%q, %d) mengembalikan nil", line, cursor)
				}
			}
		}
	}
}

// TestSapuSisipanStabil memastikan menyisipkan sebuah kandidat menghasilkan
// baris yang masih bisa dilengkapi lagi.
//
// Ini menutup lingkaran yang sebenarnya dialami pengguna: pilih, lanjutkan
// mengetik, pilih lagi. Rentang penggantian yang keliru baru terlihat di sini.
func TestSapuSisipanStabil(t *testing.T) {
	eng, names := sweepEngine(t)
	m := &masalah{jenis: "sisipan menghasilkan baris yang rusak"}

	for _, name := range names {
		line := name + " "
		res, err := eng.Complete(line, len(line))
		if err != nil || len(res.Candidates) == 0 {
			continue
		}

		for _, c := range res.Candidates[:min(3, len(res.Candidates))] {
			baru := line[:res.ReplaceStart] + c.Insert + line[res.ReplaceEnd:]
			kursor := res.ReplaceStart + c.CursorOffset

			if kursor < 0 || kursor > len(baru) {
				m.catat(fmt.Sprintf("%q + %q: kursor %d di luar baris %q", line, c.Insert, kursor, baru))
				continue
			}
			// Baris hasilnya harus tetap memuat nama perintahnya.
			if !strings.HasPrefix(baru, name) {
				m.catat(fmt.Sprintf("%q + %q menghasilkan %q", line, c.Insert, baru))
				continue
			}
			if _, err := eng.Complete(baru, kursor); err != nil {
				m.catat(fmt.Sprintf("%q gagal dilengkapi lagi: %v", baru, err))
			}
		}
	}
	m.lapor(t)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
