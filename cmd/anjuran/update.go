package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ufhy/anjuran-cli/internal/remote"
	"golang.org/x/term"
)

// runUpdate memperbarui anjuran ke rilis terbaru.
//
// Ada karena jalur pemasangan yang paling banyak dipakai — satu baris curl —
// tidak meninggalkan pengelola paket apa pun yang bisa memperbarui nanti.
// Tanpa perintah ini, satu-satunya cara adalah mengingat kembali perintah
// curl itu; dan yang tidak diingat tidak akan pernah dijalankan.
//
// Yang diperbarui BUKAN hanya binary. Spec dan tambalan ikut, karena
// keduanya berpasangan dengan versinya: binary baru dengan spec lama adalah
// kombinasi yang tidak pernah diuji siapa pun.
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	cek := fs.Bool("check", false, "hanya laporkan apakah ada versi baru")
	pra := fs.Bool("pre", false, "terima juga rilis prarilis")
	ke := fs.String("version", "", "pasang tag tertentu, misalnya v0.1.0")
	paksa := fs.Bool("force", false, "pasang ulang meski versinya sudah sama")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	if nyata, err := filepath.EvalSymlinks(exe); err == nil {
		exe = nyata
	}

	// Pemasangan lewat pengelola paket diperbarui oleh pengelola paketnya, dan
	// menimpanya dari sini akan membuat kedua pihak berselisih tentang apa yang
	// sebenarnya terpasang.
	if dikelola(exe) {
		fmt.Fprintf(os.Stderr,
			"anjuran: %s dipasang lewat pengelola paket; perbarui dengan pengelola itu\n", exe)
		return 1
	}

	tag := *ke
	pakaiPra := false
	if tag == "" {
		tag, pakaiPra, err = remote.RilisTerbaru(*pra)
		if err != nil {
			fmt.Fprintln(os.Stderr, "anjuran:", err)
			return 1
		}
	}

	sekarang := version
	baru := strings.TrimPrefix(tag, "v")
	fmt.Printf("  terpasang : %s\n", sekarang)
	fmt.Printf("  tersedia  : %s\n", tag)
	if pakaiPra {
		fmt.Println("  catatan   : belum ada rilis stabil; yang tersedia sebuah prarilis")
	}

	if sekarang == baru && !*paksa {
		fmt.Println("\nSudah versi terbaru.")
		return 0
	}
	// Versi "dev" berarti binary ini dibangun dari pohon kerja. Menimpanya
	// dengan rilis akan membuang perubahan yang sedang dikerjakan orangnya,
	// tanpa cara mengembalikannya.
	if sekarang == "dev" && !*paksa {
		fmt.Fprintln(os.Stderr,
			"\nanjuran: binary ini dibangun dari pohon kerja (versi dev).\n"+
				"  Menimpanya dengan rilis akan membuang build lokalmu.\n"+
				"  Pakai --force bila memang itu yang kamu mau.")
		return 1
	}
	if *cek {
		fmt.Println("\nAda versi baru. Jalankan `anjuran update` untuk memasangnya.")
		return 0
	}

	plat := remote.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}

	// Unduhannya 7 MB. Tanpa tanda kehidupan, sambungan yang lambat tidak bisa
	// dibedakan dari perintah yang menggantung — dan yang tampak menggantung
	// akan ditekan Ctrl-C, di tengah penggantian binary.
	remote.LaporkanKemajuan(kemajuanKeTerminal())
	src, bersihkan, err := remote.UnduhRilis(plat, tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	defer bersihkan()

	if err := pasangDiTempat(src, exe); err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}

	fmt.Printf("\nanjuran %s terpasang di %s\n", baru, exe)
	// Integrasi shell dimuat SEKALI saat shell dimulai, jadi sesi yang sedang
	// berjalan masih memakai skrip versi lama walau binernya sudah baru.
	// Tanpa disebut, perbaikan yang baru saja dipasang tampak tidak terjadi.
	fmt.Println("Mulai shell baru — `exec $SHELL` — supaya integrasinya ikut diperbarui.")
	return 0
}

// dikelola menebak apakah binary ini milik sebuah pengelola paket.
func dikelola(exe string) bool {
	for _, awalan := range []string{
		"/usr/bin/", "/usr/local/Cellar/", "/opt/homebrew/Cellar/",
		"/nix/store/", "/snap/", "/var/lib/flatpak/",
	} {
		if strings.HasPrefix(exe, awalan) {
			return true
		}
	}
	return false
}

// pasangDiTempat mengganti binary beserta spec dan tambalannya.
//
// Tata letaknya diambil dari letak binary yang sedang berjalan, bukan ditebak:
// pemasang menaruhnya di <dasar>/bin/anjuran dengan spec di
// <dasar>/share/anjuran, dan itulah yang harus ditimpa — bukan sebuah
// direktori bawaan yang mungkin bukan milik pemasangan ini.
func pasangDiTempat(src remote.Source, exe string) error {
	dasar := filepath.Dir(filepath.Dir(exe)) // <dasar>/bin/anjuran -> <dasar>

	// Binary ditulis ke berkas sementara di direktori yang SAMA lalu
	// di-rename. Rename dalam satu filesystem bersifat atomik, sehingga tidak
	// pernah ada saat di mana anjuran setengah tertulis — dan di Unix, rename
	// atas berkas yang sedang berjalan bekerja tanpa mengganggu proses ini.
	tmp := exe + ".baru"
	if err := salinBerkas(src.Binary, tmp, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}

	// Spec lama dibuang lebih dulu supaya berkas yang sudah tidak ada di rilis
	// baru tidak tertinggal dan tetap ditawarkan.
	for nama, asal := range map[string]string{"specs": src.Specs, "extra": src.Extra} {
		if asal == "" {
			continue
		}
		tujuan := filepath.Join(dasar, "share", "anjuran", nama)
		if err := os.MkdirAll(filepath.Dir(tujuan), 0o755); err != nil {
			return err
		}
		if err := os.RemoveAll(tujuan); err != nil {
			return err
		}
		if err := salinPohon(asal, tujuan); err != nil {
			return err
		}
	}
	return nil
}

// salinBerkas menyalin satu berkas beserta bit modenya.
func salinBerkas(asal, tujuan string, mode os.FileMode) error {
	isi, err := os.ReadFile(asal)
	if err != nil {
		return err
	}
	return os.WriteFile(tujuan, isi, mode)
}

// salinPohon menyalin sebuah direktori secara rekursif.
func salinPohon(asal, tujuan string) error {
	return filepath.Walk(asal, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(asal, p)
		if err != nil {
			return err
		}
		ke := filepath.Join(tujuan, rel)
		if info.IsDir() {
			return os.MkdirAll(ke, 0o755)
		}
		return salinBerkas(p, ke, info.Mode().Perm())
	})
}

// kemajuanKeTerminal menggambar batang kemajuan, atau tidak sama sekali.
//
// Di luar terminal — pipa, CI, cron — batang yang digambar ulang dengan \r
// menjadi ribuan baris sampah di dalam log. Di sana satu baris ringkas saat
// selesai jauh lebih berguna.
func kemajuanKeTerminal() remote.Kemajuan {
	kaTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	selesai := false
	return func(sudah, total int64) {
		if selesai {
			return
		}
		if !kaTerminal {
			if sudah >= total {
				selesai = true
				fmt.Printf("  diunduh   : %s\n", ukuran(total))
			}
			return
		}
		lebar := 24
		isi := 0
		if total > 0 {
			isi = int(int64(lebar) * sudah / total)
		}
		fmt.Printf("\r  mengunduh : [%s%s] %s / %s",
			strings.Repeat("█", isi), strings.Repeat("░", lebar-isi),
			ukuran(sudah), ukuran(total))
		if sudah >= total {
			selesai = true
			fmt.Println()
		}
	}
}

// ukuran memformat jumlah byte agar mudah dibaca.
func ukuran(n int64) string {
	const satuan = 1024
	if n < satuan {
		return fmt.Sprintf("%d B", n)
	}
	bagi, exp := int64(satuan), 0
	for n/bagi >= satuan && exp < 3 {
		bagi *= satuan
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(bagi), "KMGT"[exp])
}
