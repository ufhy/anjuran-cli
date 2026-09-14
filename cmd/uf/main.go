// Command uf adalah entry point CLI.
//
// Pada tahap ini hanya ada subperintah `complete`, yang membaca sebuah baris
// perintah dan mencetak kandidatnya. Bentuk ini disengaja: engine bisa diuji
// dan di-benchmark dari shell mana pun sebelum ada satu pun kode terminal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/spec"
)

const usage = `uf - autocomplete lintas platform untuk shell

Penggunaan:
  uf complete --line <baris> [--cursor N] [--json]
  uf widget   --line <baris> --cursor <N>

Opsi:
  --line    baris perintah yang sedang diketik
  --cursor  posisi kursor dalam byte (default: akhir baris)
  --json    keluarkan hasil mentah sebagai JSON
  --specs   direktori spec, boleh beberapa dipisah titik dua
            (default: $UF_SPECS, ~/.config/uf/specs, ./specs, lalu bawaan)

widget adalah mode interaktif yang dipanggil integrasi shell; dropdown digambar
ke /dev/tty dan hasilnya dikembalikan lewat stdout.

Lingkungan:
  UF_SPECS  direktori spec
  UF_SIMPLE bila diisi, matikan warna dan sorotan
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "complete":
		os.Exit(runComplete(os.Args[2:]))
	case "widget":
		os.Exit(runWidget(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "perintah tidak dikenal: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func runComplete(args []string) int {
	fs := flag.NewFlagSet("complete", flag.ExitOnError)
	line := fs.String("line", "", "baris perintah")
	cursor := fs.Int("cursor", -1, "posisi kursor dalam byte")
	asJSON := fs.Bool("json", false, "keluarkan JSON")
	specsDir := fs.String("specs", "", "direktori spec")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *cursor < 0 {
		*cursor = len(*line)
	}

	dirs, err := resolveSpecsDirs(*specsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	eng := engine.New(spec.NewRegistryDirs(dirs...))
	res, err := eng.Complete(*line, *cursor)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintln(os.Stderr, "uf:", err)
			return 1
		}
		return 0
	}

	for _, c := range res.Candidates {
		if c.Description == "" {
			fmt.Printf("%s\t%s\n", c.Name, c.Kind)
			continue
		}
		fmt.Printf("%s\t%s\t%s\n", c.Name, c.Kind, c.Description)
	}
	return 0
}

// resolveSpecsDirs menyusun urutan pencarian spec.
//
// Urutannya: yang disebut pengguna lebih dulu, lalu spec bawaan. Dengan begitu
// sebuah spec buatan sendiri bisa menimpa spec bawaan tanpa menyunting
// direktori yang dihasilkan mesin.
func resolveSpecsDirs(flagValue string) ([]string, error) {
	var dirs []string
	add := func(paths ...string) {
		for _, p := range paths {
			if p == "" {
				continue
			}
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				dirs = append(dirs, p)
			}
		}
	}

	add(filepath.SplitList(flagValue)...)
	add(filepath.SplitList(os.Getenv("UF_SPECS"))...)
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".config", "uf", "specs"))
	}
	add("specs")
	// Spec bawaan yang diletakkan bersebelahan dengan binary, sebagaimana
	// hasil pemasangan dari paket rilis.
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), "specs"),
			filepath.Join(filepath.Dir(exe), "..", "share", "uf", "specs"))
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("direktori spec tidak ditemukan; set UF_SPECS atau pakai --specs")
	}
	return dirs, nil
}
