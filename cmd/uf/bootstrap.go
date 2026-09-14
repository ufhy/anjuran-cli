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

	"github.com/uf-cli/uf/internal/remote"
	"golang.org/x/term"
)

// runBootstrap memasang uf di host lain lewat SSH.
//
// Ini bukan pembungkus ssh. uf tidak pernah menyisip di antara kamu dan
// koneksimu; perintah ini dijalankan sekali, dengan sadar, lalu selesai.
func runBootstrap(args []string) int {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	from := fs.String("from", "", "direktori sumber: berisi uf dan specs, atau arsip rilis")
	base := fs.String("base", remote.RemoteBase, "direktori tujuan di host, relatif terhadap rumah pengguna")
	force := fs.Bool("force", false, "pasang ulang meski versinya sudah sama")
	dryRun := fs.Bool("dry-run", false, "tampilkan rencananya tanpa mengirim apa pun")
	yes := fs.Bool("yes", false, "lanjutkan tanpa bertanya")
	timeout := fs.Duration("timeout", 5*time.Minute, "batas waktu seluruh proses")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "uf: sebutkan host tujuan, misalnya: uf bootstrap deploy@web-01")
		return 2
	}
	host, sshArgs := rest[0], rest[1:]

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	t, err := remote.NewSSH(host, sshArgs...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}
	defer t.Close()

	specsDirs, _ := resolveSpecsDirs("")
	opt := remote.Options{
		From:       *from,
		LocalSpecs: specsDirs,
		Base:       *base,
		Force:      *force,
		DryRun:     *dryRun,
		Version:    version,
		Out:        os.Stdout,
	}

	plan, cleanup, err := remote.Prepare(ctx, t, opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}
	defer cleanup()

	if plan.UpToDate {
		fmt.Printf("%s sudah memakai %s; tidak ada yang perlu dikirim.\n", host, plan.Installed)
		return 0
	}

	fmt.Printf("Akan memasang uf di host lain:\n\n%s\n", plan)

	if *dryRun {
		fmt.Println("Mode dry-run; tidak ada yang dikirim.")
		return 0
	}
	if !*yes && !confirm(host) {
		fmt.Println("Dibatalkan.")
		return 1
	}

	if err := remote.Install(ctx, t, plan, opt); err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	fmt.Printf("\n%s\n", remote.ShellHint(*base))
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
			"uf: bukan terminal interaktif; pakai --yes, atau daftarkan host di %s\n",
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
	return filepath.Join(home, ".config", "uf", "hosts")
}
