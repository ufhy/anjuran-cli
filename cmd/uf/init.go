package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uf-cli/uf/internal/shellinit"
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
		fmt.Fprintf(os.Stderr, "uf: sebutkan shell-nya; pilihan: %s\n",
			strings.Join(shellinit.Shells(), ", "))
		return 2
	}

	script, err := shellinit.Script(shell)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 2
	}
	fmt.Print(script)
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
