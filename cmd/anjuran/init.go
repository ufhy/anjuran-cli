package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ufhy/anjuran-cli/internal/config"
	"github.com/ufhy/anjuran-cli/internal/shellinit"
)

// runInit mencetak skrip integrasi untuk sebuah shell.
//
// Skrip disematkan di dalam binary agar pemasangannya tidak bergantung pada
// letak repo — satu baris di berkas konfigurasi, dan berlaku sama di mesin
// lokal maupun host remote yang hanya menerima salinan binary.
func runInit(args []string) int {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	}
	if shell == "" {
		shell = detectShell()
	}
	if shell == "" {
		fmt.Fprintf(os.Stderr, "anjuran: sebutkan shell-nya; pilihan: %s\n",
			strings.Join(shellinit.Shells(), ", "))
		return 2
	}

	script, err := shellinit.Script(shell)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 2
	}
	fmt.Print(pembuka(shell) + script)
	return 0
}

// detectShell menebak shell dari $SHELL. Tebakan hanya dipakai bila pengguna
// tidak menyebutkannya sendiri.
func detectShell() string {
	base := filepath.Base(os.Getenv("SHELL"))
	for _, s := range shellinit.Shells() {
		if base == s {
			return s
		}
	}
	return ""
}

// shellMembaca adalah setelan yang dibaca SKRIP SHELL, bukan kode Go.
//
// Tombol pemicu, mode otomatis, dan ghost text diputuskan di dalam skrip
// integrasi, yang berjalan di shell PENGGUNA. Lingkungan proses anjuran tidak
// pernah sampai ke sana, jadi berkas konfigurasi tidak akan berpengaruh apa
// pun atasnya kecuali nilainya ikut dibawa keluar bersama skripnya.
var shellMembaca = []string{
	"ANJURAN_KEY",
	"ANJURAN_AUTO",
	"ANJURAN_GHOST",
	"ANJURAN_GHOST_STYLE",
}

// pembuka menyusun baris yang memasang nilai dari berkas konfigurasi.
//
// Bentuknya "setel bila BELUM disetel" di setiap shell, bukan penetapan
// langsung. Dengan begitu urutan kewenangan yang berlaku di dalam anjuran
// berlaku juga di dalam shell: apa yang pengguna nyatakan untuk sesi ini
// tetap menang atas apa yang dituliskannya dulu di berkas.
func pembuka(shell string) string {
	cfg, err := config.Muat()
	if err != nil || len(cfg.Nilai) == 0 {
		return ""
	}

	var b strings.Builder
	for _, env := range shellMembaca {
		v, ada := cfg.Nilai[env]
		if !ada {
			continue
		}
		switch shell {
		case "fish":
			// Tanpa -x nilainya tidak terlihat oleh proses anjuran yang
			// dijalankan binding-nya.
			fmt.Fprintf(&b, "set -q %s; or set -gx %s %s\n", env, env, kutipFish(v))
		case "powershell", "pwsh":
			fmt.Fprintf(&b, "if (-not (Test-Path env:%s)) { $env:%s = %s }\n",
				env, env, kutipPowershell(v))
		default:
			// ${VAR=nilai} TANPA titik dua: hanya berlaku bila variabelnya
			// belum ada sama sekali. Bentuk bertitik dua juga akan menimpa
			// nilai kosong, dan kosong adalah cara sah menyatakan "mati".
			fmt.Fprintf(&b, ": \"${%s=%s}\"; export %s\n", env, kutipPosix(v), env)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "# Dipasang dari " + cfg.Berkas + "\n" + b.String() + "\n"
}

// kutipPosix membungkus nilai untuk sh, zsh, dan bash.
func kutipPosix(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// kutipFish membungkus nilai untuk fish, yang tidak mengenal bentuk itu.
func kutipFish(v string) string {
	return "'" + strings.NewReplacer(`\`, `\`, "'", `\'`).Replace(v) + "'"
}

// kutipPowershell membungkus nilai untuk PowerShell, tempat kutip tunggal
// dilipatgandakan untuk melolosnya.
func kutipPowershell(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
