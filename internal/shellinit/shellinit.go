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

//go:embed uf.zsh
var zshScript string

//go:embed uf.bash
var bashScript string

//go:embed uf.fish
var fishScript string

var scripts = map[string]string{
	"zsh":  zshScript,
	"bash": bashScript,
	"fish": fishScript,
}

// Script mengembalikan skrip integrasi untuk sebuah shell.
func Script(shell string) (string, error) {
	s, ok := scripts[shell]
	if !ok {
		return "", fmt.Errorf("shell %q tidak dikenal; pilihan: %v", shell, Shells())
	}
	return s, nil
}

// Shells mengembalikan daftar shell yang didukung, terurut.
func Shells() []string {
	out := make([]string, 0, len(scripts))
	for k := range scripts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
