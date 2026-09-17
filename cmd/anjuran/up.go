package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ufhy/anjuran-cli/internal/remote"
	"golang.org/x/term"
)

// runUp memasang anjuran di host lain, supaya completion ikut jalan saat
// pengguna SSH ke sana.
//
// Ini bukan pembungkus ssh. anjuran tidak pernah menyisip di antara kamu dan
// koneksimu; perintah ini dijalankan sekali, dengan sadar, lalu selesai.
// opsiAnjuranSalahTempat mencari opsi milik anjuran di antara argumen ssh.
//
// Daftarnya dibaca dari FlagSet-nya sendiri, bukan ditulis ulang: opsi baru
// ikut terjaga tanpa ada yang perlu mengingat tempat kedua.
func opsiAnjuranSalahTempat(fs *flag.FlagSet, args []string) string {
	milikKami := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { milikKami[f.Name] = true })

	for _, a := range args {
		nama := strings.TrimLeft(a, "-")
		if i := strings.IndexByte(nama, '='); i >= 0 {
			nama = nama[:i]
		}
		if strings.HasPrefix(a, "-") && milikKami[nama] {
			return a
		}
	}
	return ""
}

func runUp(args []string) int {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	from := fs.String("from", "", "direktori sumber: berisi anjuran dan specs, atau arsip rilis")
	base := fs.String("base", remote.RemoteBase, "direktori tujuan di host, relatif terhadap rumah pengguna")
	force := fs.Bool("force", false, "pasang ulang meski versinya sudah sama")
	dryRun := fs.Bool("dry-run", false, "tampilkan rencananya tanpa mengirim apa pun")
	noShell := fs.Bool("no-shell", false, "jangan sentuh berkas konfigurasi shell di host")
	yes := fs.Bool("yes", false, "lanjutkan tanpa bertanya")
	timeout := fs.Duration("timeout", 5*time.Minute, "batas waktu seluruh proses")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "anjuran: sebutkan host tujuan, misalnya: anjuran up deploy@web-01")
		return 2
	}
	host, sshArgs := rest[0], rest[1:]

	// Opsi anjuran yang tertulis SESUDAH nama host diteruskan ke ssh apa
	// adanya — dan ssh menjawabnya dengan memuntahkan seluruh pesan
	// penggunaannya, yang sama sekali tidak menjelaskan apa yang keliru.
	//
	// "anjuran bootstrap lab --dry-run" adalah bentuk yang paling wajar
	// ditulis orang, jadi ia pantas dijawab dengan kalimat, bukan dengan
	// halaman bantuan ssh.
	if salah := opsiAnjuranSalahTempat(fs, sshArgs); salah != "" {
		fmt.Fprintf(os.Stderr,
			"anjuran: %s adalah opsi anjuran, bukan opsi ssh, dan harus ditulis SEBELUM nama host:\n\n"+
				"  anjuran up %s %s\n\n"+
				"Apa pun sesudah nama host diteruskan ke ssh apa adanya.\n",
			salah, salah, host)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	t, err := remote.NewSSH(host, sshArgs...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	defer t.Close()

	specsDirs, _ := resolveSpecsDirs("")
	// Tambalan buatan tangan ikut dikirim: di situlah pengetahuan yang tidak
	// ada di korpus Fig. Spec milik pengguna di ~/.config TIDAK ikut — itu
	// miliknya sendiri, dan menyalinnya ke mesin orang lain tanpa diminta
	// bukan keputusan yang boleh diambil alat ini.
	extraDirs := append([]string{"extra"}, bundledDirs("extra")...)
	opt := remote.Options{
		From:       *from,
		LocalSpecs: specsDirs,
		LocalExtra: extraDirs,
		Base:       *base,
		Force:      *force,
		DryRun:     *dryRun,
		NoShell:    *noShell,
		Version:    version,
		Out:        os.Stdout,
	}

	plan, cleanup, err := remote.Prepare(ctx, t, opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}
	defer cleanup()

	if plan.UpToDate {
		fmt.Printf("%s sudah memakai %s; tidak ada yang perlu dikirim.\n", host, plan.Installed)
		// Binary yang sudah mutakhir tidak berarti shell-nya sudah disetel.
		// Keduanya urusan terpisah, dan yang belum selesai tetap diselesaikan.
		if !*noShell {
			if rc, status, err := remote.PasangShell(ctx, t, *base); err != nil {
				fmt.Printf("  shell     : gagal disetel (%v)\n", err)
			} else {
				fmt.Printf("  shell     : %s (%s)\n", rc, status)
			}
		}
		return 0
	}

	fmt.Printf("Akan memasang anjuran di host lain:\n\n%s\n", plan)

	if *dryRun {
		fmt.Println("Mode dry-run; tidak ada yang dikirim.")
		return 0
	}
	if !*yes && !confirm(host) {
		fmt.Println("Dibatalkan.")
		return 1
	}

	if err := remote.Install(ctx, t, plan, opt); err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}

	// Petunjuk manual hanya berguna bila memang tidak ada yang disetel sendiri.
	if *noShell {
		fmt.Printf("\n%s\n", remote.ShellHint(*base))
	} else {
		fmt.Printf("\nBuka sesi baru ke host itu, atau muat ulang konfigurasi shell-nya.\n")
	}
	return 0
}

// confirm meminta persetujuan sebelum menulis ke mesin orang lain.
//
// Host yang sudah terdaftar di berkas allowlist tidak ditanyai: itu bentuk
// persetujuan yang bisa ditinjau dan disimpan di kendali versi. Di luar
// terminal interaktif, jawabannya selalu tidak — sebuah skrip tidak boleh
// mendapat izin hanya karena tidak ada yang menjawab.
func confirm(host string) bool {
	if allowFile := allowlistPath(); allowFile != "" && remote.Allowed(host, allowFile) {
		fmt.Printf("%s sudah terdaftar di %s.\n", host, allowFile)
		return true
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr,
			"anjuran: bukan terminal interaktif; pakai --yes, atau daftarkan host di %s\n",
			allowlistPath())
		return false
	}

	fmt.Print("Lanjutkan? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "ya", "yes":
		return true
	}
	return false
}

// allowlistPath adalah berkas daftar host yang sudah disetujui.
func allowlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "anjuran", "hosts")
}
