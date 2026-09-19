package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/ufhy/anjuran-cli/internal/config"
)

// contohConfig adalah berkas yang dicetak `anjuran config --contoh`.
//
// Seluruh barisnya dikomentari dengan sengaja. Berkas contoh yang menyalakan
// sesuatu hanya karena disalin adalah jebakan: orang menyalinnya untuk tahu
// apa yang BISA diatur, bukan untuk mengubah perilakunya diam-diam.
const contohConfig = `# Konfigurasi anjuran.
#
# Setiap baris di bawah menunjukkan nilai BAWAANNYA, dan semuanya dikomentari.
# Variabel lingkungan tetap menang atas berkas ini, jadi apa pun yang kamu
# setel untuk satu sesi tidak perlu diubah di sini lebih dulu.

# Direktori spec. Kosong berarti cari di tempat biasa.
# specs = "~/.config/anjuran/specs"

# Matikan warna dan sorotan. Berguna di terminal yang tidak mendukungnya.
# simple = false

# Gaya ikon per baris: "nerd", atau false untuk mematikan.
# ikon = "nerd"

# Tombol pemicu, dibaca skrip init. "\t" adalah Tab.
# key = "\t"

# Dropdown yang muncul sendiri di tombol pemicu. Di Windows bawaannya mati.
# auto = true

# Saran dari riwayat sebagai teks abu-abu.
# ghost = false

# Rekam byte yang digambar ke terminal, untuk menyelidiki laporan
# "layarnya aneh" di mesin lain.
# log = "/tmp/anjuran.log"

# Lokasi cache generator dan ingatan pilihan.
# cache_dir = "~/.cache/anjuran"

[generator]
# Matikan SELURUH generator.
# disable = false

# Biner tambahan yang boleh dijalankan, di luar perintah yang sedang diketik.
# allow = ["kubectl", "docker"]

# Izinkan generator berjalan sebagai root. Mati secara bawaan, dan itu
# disengaja: generator menjalankan perintah sebagai efek samping mengetik.
# allow_root = false

# Batas waktu satu generator.
# timeout = "800ms"
`

// runConfig menjawab satu pertanyaan yang tidak punya jawaban sebelum ini:
// "di mana berkasnya, dan apa yang sebenarnya terbaca?"
//
// Konfigurasi yang tidak bisa diperiksa akan selalu menghasilkan tebakan saat
// ada yang tidak sesuai harapan — dan tebakan itu biasanya salah tempat.
func runConfig(args []string) int {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	contoh := fs.Bool("contoh", false, "cetak berkas contoh yang bisa disalin")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *contoh {
		fmt.Print(contohConfig)
		return 0
	}

	p := config.Lokasi()
	if p == "" {
		fmt.Fprintln(os.Stderr, "anjuran: lokasi konfigurasi tidak bisa ditentukan")
		return 1
	}
	fmt.Println(p)

	cfg, err := config.Muat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	if cfg.Berkas == "" {
		fmt.Println("\n(belum ada; anjuran berjalan dengan bawaan)")
		fmt.Println("Buat dengan: anjuran config --contoh > " + p)
		return 0
	}
	if len(cfg.Nilai) == 0 {
		fmt.Println("\n(ada, tetapi tidak menyetel apa pun)")
		return 0
	}

	// Yang ditampilkan nama LINGKUNGANNYA, bukan nama di berkas: itu yang
	// akan dicari orang saat membandingkan dengan apa yang sedang berjalan.
	fmt.Println()
	nama := make([]string, 0, len(cfg.Nilai))
	for k := range cfg.Nilai {
		nama = append(nama, k)
	}
	sort.Strings(nama)
	for _, k := range nama {
		v := cfg.Nilai[k]
		tanda := ""
		// Nilai yang ditimpa lingkungan ditandai, karena di situlah kebingungan
		// lahir: berkasnya benar, tetapi bukan berkas itu yang berlaku.
		if env, ada := os.LookupEnv(k); ada && env != v {
			tanda = fmt.Sprintf("  (ditimpa lingkungan: %q)", env)
		}
		fmt.Printf("  %-28s %q%s\n", k, v, tanda)
	}
	return 0
}
