package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// TemplateCommands melengkapi nama perintah dari PATH.
const TemplateCommands = "commands"

// maxPerintah membatasi jumlah nama yang dikembalikan.
//
// PATH sebuah mesin pengembangan biasa memuat beberapa ribu biner, dan awalan
// yang sangat pendek — satu huruf — akan cocok dengan ratusan di antaranya.
// Daftar sepanjang itu tidak menolong siapa pun, dan hanya membuat penyaringan
// serta penggambaran membayar harga yang tidak ada gunanya.
const maxPerintah = 200

// maxEntriPath membatasi entri yang dibaca dari SATU direktori PATH.
//
// Jauh lebih besar daripada maxEntries, dan itu bukan kelonggaran melainkan
// koreksi: batas 2000 yang dipakai untuk melengkapi nama berkas salah
// diterapkan di sini. os.ReadDir mengembalikan entrinya TERURUT, sehingga
// batas itu memotong ekor abjadnya — di /usr/bin runner Ubuntu yang memuat
// lebih dari dua ribu biner, `zsh` hilang sementara perintah berhuruf awal
// hilang. Bukan daftar yang dipendekkan, melainkan perintah yang tidak
// pernah bisa dilengkapi sama sekali, tanpa satu pun tanda.
//
// Penjagaan terhadap direktori yang benar-benar patologis tetap ada; yang
// berubah hanya letaknya, dari "sering tercapai" menjadi "tidak pernah".
const maxEntriPath = 100000

var (
	sekaliPath = onceBaru()
	daftarPath []string
)

// onceBaru ada supaya pengujian bisa memaksa pemindaian diulang; di dalam satu
// proses pengujian ada banyak PATH, sementara di luar sana hanya ada satu.
func onceBaru() *sync.Once { return new(sync.Once) }

// Commands mengembalikan nama biner di PATH yang berawalan prefix.
//
// Hasil pemindaian disimpan untuk seumur proses. Satu proses adalah satu
// interaksi — sesi memegang seluruh ketikan sampai dropdown tertutup —
// sehingga PATH dipindai paling banyak sekali per dropdown, bukan sekali per
// huruf yang diketik.
func Commands(prefix string) []string {
	// Prefix yang memuat pemisah jalur bukan nama perintah lagi, melainkan
	// jalur ke sebuah berkas; melengkapinya dari PATH tidak masuk akal.
	if strings.ContainsAny(prefix, "/\\") {
		return nil
	}

	sekaliPath.Do(muatPath)

	lower := strings.ToLower(prefix)
	out := make([]string, 0, 64)
	for _, nama := range daftarPath {
		if strings.HasPrefix(strings.ToLower(nama), lower) {
			out = append(out, nama)
			if len(out) >= maxPerintah {
				break
			}
		}
	}
	return out
}

// muatPath memindai seluruh direktori di PATH satu kali.
func muatPath() {
	seen := map[string]bool{}
	var out []string

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entri, err := os.ReadDir(dir)
		if err != nil {
			// Direktori PATH yang tidak ada atau tidak terbaca adalah hal
			// biasa; ia dilewati, bukan menggagalkan seluruh pemindaian.
			continue
		}
		for i, en := range entri {
			if i >= maxEntriPath {
				break
			}
			nama := en.Name()
			if en.IsDir() || seen[nama] {
				continue
			}
			if !bisaDijalankan(dir, en) {
				continue
			}
			seen[nama] = true
			out = append(out, nama)
		}
	}
	daftarPath = out
}

// bisaDijalankan menilai apakah sebuah entri layak ditawarkan sebagai perintah.
func bisaDijalankan(dir string, en os.DirEntry) bool {
	if runtime.GOOS == "windows" {
		// Windows tidak punya bit eksekusi; yang menentukan adalah
		// akhirannya, dan daftar yang sah ada di PATHEXT.
		ext := strings.ToLower(filepath.Ext(en.Name()))
		if ext == "" {
			return false
		}
		pathext := os.Getenv("PATHEXT")
		if pathext == "" {
			pathext = ".COM;.EXE;.BAT;.CMD"
		}
		for _, e := range strings.Split(strings.ToLower(pathext), ";") {
			if ext == e {
				return true
			}
		}
		return false
	}

	info, err := en.Info()
	if err != nil {
		return false
	}
	// Symlink menunjuk ke luar direktori ini — Homebrew memasang hampir
	// seluruh binernya begitu — jadi modenya harus ditanyakan ke sasarannya.
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(filepath.Join(dir, en.Name()))
		if err != nil {
			return false
		}
	}
	return !info.IsDir() && info.Mode()&0o111 != 0
}
