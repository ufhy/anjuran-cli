// Package config membaca keinginan tetap pengguna dari sebuah berkas.
//
// Variabel lingkungan bagus untuk sekali pakai — mematikan sesuatu di satu
// sesi, mencoba sebuah nilai — tetapi buruk sebagai tempat menyimpan keputusan
// yang bertahan: ia tidak bisa diberi komentar, tidak bisa ditinjau, dan harus
// diulang di setiap berkas rc pada setiap mesin. Dua belas variabel sudah
// melewati batas nyaman untuk itu.
//
// Berkasnya TIDAK PERNAH menjadi syarat. Tanpa berkas apa pun, anjuran
// berjalan persis seperti sebelum paket ini ada — dan itu bukan kemurahan
// hati, melainkan syarat: alat yang menuntut konfigurasi sebelum bisa dipakai
// sudah gagal sebelum dicoba.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Nama berkasnya. Ekstensinya toml karena bentuknya memang TOML, walau yang
// diurai hanya bagian yang dipakai di sini.
const NamaBerkas = "config.toml"

// Config memetakan kunci berkas ke variabel lingkungan yang sudah ada.
//
// Dipetakan begitu, bukan menjadi struct bidang-per-bidang, karena seluruh
// pembacaan setelan di dalam anjuran sudah melewati os.Getenv. Menambahkan
// jalur kedua berarti setiap tempat harus tahu tentang keduanya, dan tempat
// yang terlupa akan diam-diam mengabaikan berkasnya.
type Config struct {
	// Nilai berisi pasangan NAMA_ENV -> nilai, hanya untuk kunci yang benar
	// benar ada di berkas.
	Nilai map[string]string
	// Berkas adalah lokasi yang benar-benar dibaca; kosong bila tidak ada.
	Berkas string
}

// kunci memetakan nama di dalam berkas ke nama variabel lingkungan.
//
// Nama di berkas sengaja lebih pendek dan tanpa awalan: di dalam berkas
// bernama anjuran, mengulang kata "anjuran" di setiap baris hanya kebisingan.
var kunci = map[string]string{
	"specs":     "ANJURAN_SPECS",
	"simple":    "ANJURAN_SIMPLE",
	"ikon":      "ANJURAN_IKON",
	"key":       "ANJURAN_KEY",
	"auto":      "ANJURAN_AUTO",
	"ghost":     "ANJURAN_GHOST",
	"log":       "ANJURAN_LOG",
	"cache_dir": "ANJURAN_CACHE_DIR",

	"generator.disable":    "ANJURAN_NO_GENERATORS",
	"generator.allow":      "ANJURAN_GENERATOR_ALLOW",
	"generator.allow_root": "ANJURAN_GENERATOR_ALLOW_ROOT",
	"generator.timeout":    "ANJURAN_GENERATOR_TIMEOUT",

	"release_url": "ANJURAN_RELEASE_URL",
}

// matiJadiNol menandai setelan yang BAWAANNYA nyala.
//
// Dua gaya hidup berdampingan di antara variabel lingkungan anjuran, dan
// keduanya harus dihormati apa adanya. Sebagian besar bermakna "bila diisi":
// ANJURAN_SIMPLE=apa pun menyalakannya, jadi menuliskan `simple = false`
// harus berarti TIDAK MENYETELNYA sama sekali — menyetelnya menjadi kata
// "false" justru menyalakannya, kebalikan dari yang ditulis.
//
// Yang di bawah ini sebaliknya: ia menyala tanpa disetel, dan hanya nilai
// yang jelas mematikannya yang berpengaruh. Untuk itu `false` harus benar
// benar sampai ke lingkungan.
var matiJadiNol = map[string]bool{
	"ANJURAN_AUTO": true,
	"ANJURAN_IKON": true,
}

// Lokasi mengembalikan tempat berkas konfigurasi dicari.
//
// Mengikuti kebiasaan tiap sistem, bukan menyeragamkannya: pengguna Linux
// mencari di ~/.config, pengguna Windows di %AppData%. Alat yang meletakkan
// berkasnya di tempat yang tidak diduga akan dicari dengan `find`.
func Lokasi() string {
	if d := os.Getenv("ANJURAN_CONFIG"); d != "" {
		return d
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		if runtime.GOOS == "windows" {
			return filepath.Join(home, "anjuran", NamaBerkas)
		}
		return filepath.Join(home, ".config", "anjuran", NamaBerkas)
	}
	return filepath.Join(dir, "anjuran", NamaBerkas)
}

// Muat membaca berkas konfigurasi bila ada.
//
// Berkas yang tidak ada BUKAN kesalahan: itu keadaan normal, dan keadaan
// normal tidak boleh mencetak apa pun. Berkas yang ada tetapi rusak adalah
// hal lain — di situ pengguna sudah menyatakan niat, dan diam berarti
// niatnya hilang tanpa sepengetahuannya.
func Muat() (*Config, error) {
	p := Lokasi()
	if p == "" {
		return &Config{Nilai: map[string]string{}}, nil
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Nilai: map[string]string{}}, nil
		}
		return &Config{Nilai: map[string]string{}}, err
	}
	defer f.Close()

	nilai, err := urai(f)
	if err != nil {
		return &Config{Nilai: map[string]string{}}, fmt.Errorf("%s: %w", p, err)
	}
	return &Config{Nilai: nilai, Berkas: p}, nil
}

// Terapkan memasang nilai berkas ke lingkungan proses ini, TANPA menimpa apa
// pun yang sudah disetel.
//
// Di situlah urutan kewenangannya terwujud, dan hanya di satu tempat: bawaan,
// lalu berkas, lalu lingkungan, lalu flag. Variabel yang sudah ada berarti
// pengguna menyatakannya untuk sesi ini, dan yang dinyatakan untuk sesi ini
// selalu menang atas yang dituliskan dulu.
func (c *Config) Terapkan() {
	for env, v := range c.Nilai {
		if _, ada := os.LookupEnv(env); ada {
			continue
		}
		os.Setenv(env, v)
	}
}

// urai membaca TOML secukupnya.
//
// Hanya bagian yang benar-benar dipakai: komentar, judul bagian, dan
// penetapan berisi teks, benar/salah, angka, atau daftar teks. Menarik
// pustaka TOML penuh demi itu berarti menambah ketergantungan yang jauh lebih
// besar daripada yang dibutuhkan — dan proyek ini baru punya dua, keduanya
// milik Go sendiri.
//
// Yang TIDAK didukung ditolak dengan berisik, bukan diabaikan diam-diam:
// pengguna yang menulis sesuatu yang tidak dimengerti harus diberi tahu,
// bukan dibiarkan mengira setelannya berlaku.
func urai(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	bagian := ""

	for baris := 1; sc.Scan(); baris++ {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}

		if strings.HasPrefix(t, "[") {
			if !strings.HasSuffix(t, "]") {
				return nil, fmt.Errorf("baris %d: judul bagian tidak ditutup", baris)
			}
			bagian = strings.TrimSpace(t[1 : len(t)-1])
			continue
		}

		nama, isi, ok := strings.Cut(t, "=")
		if !ok {
			return nil, fmt.Errorf("baris %d: bukan penetapan %q", baris, t)
		}
		nama = strings.TrimSpace(nama)
		if bagian != "" {
			nama = bagian + "." + nama
		}

		env, dikenal := kunci[nama]
		if !dikenal {
			return nil, fmt.Errorf("baris %d: setelan %q tidak dikenal", baris, nama)
		}

		v, mati, err := nilaiDari(strings.TrimSpace(isi))
		if err != nil {
			return nil, fmt.Errorf("baris %d: %w", baris, err)
		}
		if mati && !matiJadiNol[env] {
			// Lihat catatan pada matiJadiNol: untuk setelan bergaya "bila
			// diisi", cara mematikannya adalah tidak menyetelnya.
			delete(out, env)
			continue
		}
		out[env] = v
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// nilaiDari menerjemahkan sisi kanan sebuah penetapan menjadi teks.
//
// Semuanya berakhir sebagai teks karena tujuannya variabel lingkungan, dan
// lingkungan hanya mengenal teks. Yang dijaga di sini bentuk yang DITULIS
// pengguna: false menjadi kosong, bukan "false", supaya `simple = false`
// berarti mati dan bukan "diisi dengan kata false" — yang justru menyalakannya.
func nilaiDari(s string) (nilai string, mati bool, err error) {
	// Komentar di ujung baris dipotong, kecuali di dalam kutip.
	if !strings.HasPrefix(s, "\"") && !strings.HasPrefix(s, "'") {
		if i := strings.Index(s, "#"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	if s == "" {
		return "", false, fmt.Errorf("nilai kosong")
	}

	switch s {
	case "true":
		return "1", false, nil
	case "false":
		return "0", true, nil
	}

	// Daftar teks menjadi satu baris berpisah koma, bentuk yang sudah dipakai
	// ANJURAN_GENERATOR_ALLOW.
	if strings.HasPrefix(s, "[") {
		if !strings.HasSuffix(s, "]") {
			return "", false, fmt.Errorf("daftar tidak ditutup")
		}
		var bagian []string
		for _, p := range strings.Split(s[1:len(s)-1], ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			teks, err := lepasKutip(p)
			if err != nil {
				return "", false, err
			}
			bagian = append(bagian, teks)
		}
		return strings.Join(bagian, ","), false, nil
	}

	teks, err := lepasKutip(s)
	return teks, false, err
}

// lepasKutip membuka kutip sebuah teks, dan menerima angka apa adanya.
func lepasKutip(s string) (string, error) {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		// Kutip tunggal TOML bersifat harfiah: isinya tidak ditafsirkan.
		return s[1 : len(s)-1], nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		// Hanya pelolosan yang benar-benar dipakai di sini. \t penting: ia
		// cara menuliskan Tab sebagai tombol pemicu.
		r := strings.NewReplacer(`\t`, "\t", `\n`, "\n", `\\`, `\`, `\"`, `"`)
		return r.Replace(s[1 : len(s)-1]), nil
	}
	// Angka dan durasi ditulis tanpa kutip di TOML; keduanya dipakai apa
	// adanya sebagai teks.
	if strings.ContainsAny(s, " \t") {
		return "", fmt.Errorf("teks berspasi harus dikutip: %q", s)
	}
	return s, nil
}
