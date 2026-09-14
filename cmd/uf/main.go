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

Opsi:
  --line    baris perintah yang sedang diketik
  --cursor  posisi kursor dalam byte (default: akhir baris)
  --json    keluarkan hasil mentah sebagai JSON
  --specs   direktori spec (default: $UF_SPECS, lalu ./specs, lalu ~/.config/uf/specs)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "complete":
		os.Exit(runComplete(os.Args[2:]))
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

	dir, err := resolveSpecsDir(*specsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	eng := engine.New(spec.NewRegistry(dir))
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

// resolveSpecsDir mencari direktori spec dengan urutan: flag eksplisit,
// variabel lingkungan, direktori kerja, lalu konfigurasi user.
func resolveSpecsDir(flagValue string) (string, error) {
	candidates := []string{flagValue, os.Getenv("UF_SPECS"), "specs"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "uf", "specs"))
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("direktori spec tidak ditemukan; set UF_SPECS atau pakai --specs")
}
