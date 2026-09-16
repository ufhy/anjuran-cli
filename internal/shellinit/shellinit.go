// Package shellinit menyimpan skrip integrasi shell di dalam binary.
//
// Skrip disematkan, bukan dibaca dari disk, sehingga pemasangannya cukup satu
// baris di berkas konfigurasi dan tidak bergantung pada letak repo. Ini juga
// yang membuat pemasangan di host remote lewat SSH hanya berarti menyalin satu
// berkas biner.
package shellinit

import (
	_ "embed"
	"fmt"
	"sort"
)

//go:embed anjuran.zsh
var zshScript string

//go:embed anjuran.bash
var bashScript string

//go:embed anjuran.fish
var fishScript string

//go:embed anjuran.ps1
var powershellScript string

var scripts = map[string]string{
	"zsh":        zshScript,
	"bash":       bashScript,
	"fish":       fishScript,
	"powershell": powershellScript,
	// pwsh adalah nama biner PowerShell 6 ke atas; deteksi dari $SHELL
	// akan menemukan nama itu, bukan "powershell".
	"pwsh": powershellScript,
}

// Script mengembalikan skrip integrasi untuk sebuah shell.
func Script(shell string) (string, error) {
	s, ok := scripts[shell]
	if !ok {
		return "", fmt.Errorf("shell %q tidak dikenal; pilihan: %v", shell, Shells())
	}
	return s, nil
}

// Shells mengembalikan daftar shell yang didukung, terurut. Alias "pwsh"
// tidak ikut ditampilkan agar daftarnya tidak memuat dua nama untuk hal sama.
func Shells() []string {
	out := make([]string, 0, len(scripts))
	for k := range scripts {
		if k == "pwsh" {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
