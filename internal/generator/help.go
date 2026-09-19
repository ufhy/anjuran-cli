package generator

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ufhy/anjuran-cli/internal/engine"
)

// Pengetahuan dari `--help`.
//
// Korpus spec yang dipakai anjuran berhenti dirawat pada Mei 2025, dan
// perintah tidak berhenti berubah bersamanya: flag baru docker, kubectl, dan
// gh tidak akan pernah datang dari sana. Yang tetap ikut berubah adalah
// keluaran `--help` milik biner yang benar-benar terpasang di mesin ini.
//
// Ini bukan pengganti spec tulisan tangan. Keluaran --help tidak punya bentuk
// baku, sehingga penguraiannya selalu berupa tebakan terdidik: ia menangkap
// flag dengan baik, subcommand dengan cukup baik, dan tipe argumen tidak sama
// sekali. Karena itu ia hanya dipakai DI TEMPAT KORPUSNYA DIAM — bukan
// menimpa apa yang sudah diketahui.

// bantuanTTL panjang dengan sengaja.
//
// Keluaran --help sebuah biner hanya berubah saat binernya diganti, dan itu
// jarang. Menjalankannya berulang kali hanya menambah proses pada jalur yang
// paling sering dipakai.
const bantuanTTL = 30 * time.Minute

// bantuanMaks membatasi berapa banyak baris keluaran yang diurai. Halaman
// bantuan yang sangat panjang biasanya berupa contoh dan catatan, bukan
// daftar flag — dan mengurai puluhan ribu baris di jalur yang dipanggil tiap
// ketikan bukan pertukaran yang sepadan.
const bantuanMaks = 400

var (
	// Nama flag panjang dan pendek, dicari di dalam bagian KIRI sebuah baris.
	//
	// Sengaja bukan satu regex besar untuk seluruh baris. Bentuk argumennya
	// berbeda-beda — "--file string" di cobra, "--jobs <N>" di clap,
	// "--color=WHEN" di getopt — dan membedakan argumen dari kata pertama
	// keterangan menuntut lookahead, yang tidak dimiliki RE2. Memisahkan
	// keterangan lebih dulu membuat pembedaan itu tidak diperlukan.
	polaPanjang = regexp.MustCompile(`--[A-Za-z0-9][A-Za-z0-9-]*`)
	polaPendek  = regexp.MustCompile(`(?:^|[\s,])(-[A-Za-z0-9?])(?:[\s,=]|$)`)

	// Judul bagian yang mengawali daftar subcommand.
	polaJudulSub = regexp.MustCompile(`(?i)^[A-Za-z][A-Za-z ]*(commands|subcommands|perintah)[A-Za-z ]*:?\s*$`)

	// Baris subcommand: spasi di depan, satu kata, lalu keterangan.
	polaSub = regexp.MustCompile(`^\s+([a-z][a-z0-9][a-z0-9_-]*)(\s\s+(.*))?$`)
)

// Bantuan menjalankan `--help` untuk sebuah perintah dan menguraikan hasilnya.
//
// argv adalah perintah beserta subcommand yang sudah diketik, misalnya
// ["docker", "compose"]. Keluarannya kandidat flag dan subcommand.
func (s *Source) Bantuan(argv []string, mauFlag bool) []engine.Candidate {
	if len(argv) == 0 || argv[0] == "" {
		return nil
	}

	teks, ok := s.jalankanBantuan(argv)
	if !ok {
		return nil
	}
	return uraiBantuan(teks, mauFlag)
}

// jalankanBantuan mencoba bentuk permintaan bantuan yang lazim.
//
// Urutannya disengaja: --help paling luas didukung, -h dipakai alat yang
// lebih tua, dan `help <sub>` dipakai alat bergaya git. Yang pertama
// menghasilkan sesuatu yang bisa diurai dipakai; sisanya tidak dicoba, karena
// setiap percobaan berarti satu proses lagi.
func (s *Source) jalankanBantuan(argv []string) (string, bool) {
	bentuk := [][]string{
		append(append([]string{}, argv...), "--help"),
		append(append([]string{}, argv...), "-h"),
	}

	pol := PolicyFromEnv(argv[0])
	for _, cmd := range bentuk {
		if d := pol.Check(cmd, false); !d.Allowed {
			// Ditolak kebijakan yang sama dengan generator lain: interpreter
			// tidak pernah dijalankan, dan tidak ada yang berjalan sebagai
			// root. Mencoba bentuk berikutnya hanya akan ditolak juga.
			s.Denied = append(s.Denied, strings.Join(cmd, " ")+": "+d.Reason)
			return "", false
		}

		if s.Cache != nil {
			if out, ok := s.Cache.Get(cmd, s.Dir); ok {
				return out, out != ""
			}
		}

		out, err := jalankanPerintah(cmd, s.Dir, s.Timeout)
		// Banyak alat mencetak bantuannya ke stderr dan keluar dengan status
		// bukan nol; yang menentukan isinya, bukan status keluarnya.
		if strings.TrimSpace(out) == "" && err != nil {
			continue
		}
		if s.Cache != nil {
			s.Cache.Put(cmd, s.Dir, out, bantuanTTL)
		}
		if strings.TrimSpace(out) != "" {
			return out, true
		}
	}
	return "", false
}

// jalankanPerintah menjalankan satu perintah dan mengambil seluruh
// keluarannya.
//
// Berbeda dari generator biasa, stderr IKUT dibaca: banyak alat mencetak
// bantuannya ke sana — git di antaranya — dan membuangnya berarti jalur ini
// diam justru pada perintah yang paling sering dipakai.
func jalankanPerintah(argv []string, dir string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	// Bantuan tidak pernah membaca masukan. Membiarkan stdin terbuka membuat
	// alat yang salah menebak mode interaktifnya menggantung sampai batas
	// waktu — dan itu terasa sebagai shell yang membeku.
	cmd.Stdin = nil

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	// Seluruh keturunannya ikut mati saat waktu habis; tanpa ini cucu proses
	// bisa tertinggal hidup.
	setProcessGroup(cmd)
	err := cmd.Run()
	return buf.String(), err
}

// uraiBantuan menarik flag dan subcommand dari teks bantuan.
func uraiBantuan(teks string, mauFlag bool) []engine.Candidate {
	var out []engine.Candidate
	seen := map[string]bool{}
	tambah := func(nama, ket string, kind engine.Kind) {
		if nama == "" || seen[nama] {
			return
		}
		seen[nama] = true
		out = append(out, engine.Candidate{
			Name:         nama,
			Insert:       nama,
			CursorOffset: len(nama),
			Description:  rapikanKeterangan(ket),
			Kind:         kind,
			// Di bawah kandidat dari spec: yang ditulis tangan selalu lebih
			// tepat daripada yang ditebak dari teks bebas.
			Priority: engine.DefaultPriority - 1,
		})
	}

	dalamSub := false
	for i, baris := range strings.Split(teks, "\n") {
		if i >= bantuanMaks {
			break
		}
		baris = strings.TrimRight(baris, "\r")
		if strings.TrimSpace(baris) == "" {
			continue
		}

		// Judul bagian menandai awal daftar subcommand, dan judul lain
		// mengakhirinya. Tanpa batas ini, contoh pemakaian ikut terbaca
		// sebagai subcommand.
		if !strings.HasPrefix(baris, " ") && !strings.HasPrefix(baris, "\t") {
			dalamSub = polaJudulSub.MatchString(baris)
			continue
		}

		if mauFlag {
			kiri, ket := belahKeterangan(baris)
			if !strings.HasPrefix(strings.TrimSpace(kiri), "-") {
				continue
			}
			for _, nama := range polaPanjang.FindAllString(kiri, -1) {
				tambah(nama, ket, engine.KindOption)
			}
			for _, m := range polaPendek.FindAllStringSubmatch(kiri, -1) {
				tambah(m[1], ket, engine.KindOption)
			}
			continue
		}

		if dalamSub {
			if m := polaSub.FindStringSubmatch(baris); m != nil {
				tambah(m[1], m[3], engine.KindSubcommand)
			}
		}
	}
	return out
}

// belahKeterangan memisahkan bagian flag dari keterangannya.
//
// Pemisahnya DUA spasi atau lebih. Itu kebiasaan yang dipatuhi hampir semua
// pustaka argumen, dan satu-satunya petunjuk yang tersedia: nama argumen dan
// kata pertama keterangan sama-sama sepotong teks biasa, dan hanya jarak
// antar keduanya yang membedakan.
func belahKeterangan(baris string) (kiri, ket string) {
	for i := 0; i+1 < len(baris); i++ {
		// Dua spasi pertama SESUDAH ada isi; jarak menjorok di awal baris
		// bukan pemisah.
		if baris[i] == ' ' && baris[i+1] == ' ' && strings.TrimSpace(baris[:i]) != "" {
			return baris[:i], strings.TrimSpace(baris[i:])
		}
	}
	return baris, ""
}

// rapikanKeterangan memangkas keterangan agar muat di kolom kanan.
func rapikanKeterangan(s string) string {
	s = strings.TrimSpace(s)
	// Keterangan berbaris banyak disambung menjadi satu di sumbernya; yang
	// dipakai cukup kalimat pertamanya.
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	const maks = 120
	if len(s) > maks {
		s = strings.TrimSpace(s[:maks])
	}
	return s
}
