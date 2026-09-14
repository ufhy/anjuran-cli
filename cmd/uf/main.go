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
	"github.com/uf-cli/uf/internal/generator"
	"github.com/uf-cli/uf/internal/spec"
	"github.com/uf-cli/uf/internal/ui"
)

// version diisi saat build lewat -ldflags. Nilai "dev" berarti binary ini
// dibangun dari pohon kerja, bukan dari sebuah rilis.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

const usage = `uf - autocomplete lintas platform untuk shell

Penggunaan:
  uf init     <zsh|bash|fish|powershell>
  uf version
  uf complete --line <baris> [--cursor N] [--json]
  uf widget   --line <baris> --cursor <N>

Opsi:
  --line    baris perintah yang sedang diketik
  --cursor  posisi kursor dalam byte (default: akhir baris)
  --json    keluarkan hasil mentah sebagai JSON
  --no-generators
            jangan jalankan generator; hanya kandidat dari berkas spec
  --specs   direktori spec, boleh beberapa dipisah titik dua
            (default: $UF_SPECS, ~/.config/uf/specs, ./specs, lalu bawaan)

init mencetak skrip integrasi shell. Pasang dengan menambahkan satu baris ke
berkas konfigurasi shell:

  zsh   ~/.zshrc                     eval "$(uf init zsh)"
  bash  ~/.bashrc                    eval "$(uf init bash)"
  fish  ~/.config/fish/config.fish   uf init fish | source

widget adalah mode interaktif yang dipanggil integrasi shell; dropdown digambar
ke /dev/tty dan hasilnya dikembalikan lewat stdout.

Lingkungan:
  UF_SPECS  direktori spec
  UF_SIMPLE bila diisi, matikan warna dan sorotan
  UF_KEY    tombol pemicu, dibaca oleh skrip init

Generator menjalankan perintah sebagai efek samping mengetik, jadi
kebijakannya ketat secara bawaan dan diatur lewat lingkungan:

  UF_NO_GENERATORS        bila diisi, matikan seluruh generator
  UF_GENERATOR_ALLOW      biner tambahan yang boleh dijalankan, dipisah koma
  UF_GENERATOR_ALLOW_ROOT bila diisi, izinkan generator berjalan sebagai root
  UF_GENERATOR_TIMEOUT    batas waktu, misalnya 800ms

Secara bawaan uf hanya menjalankan perintah yang sedang kamu ketik sendiri,
dan tidak pernah menjalankan interpreter seperti bash, python, atau sudo.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "complete":
		os.Exit(runComplete(os.Args[2:]))
	case "widget":
		os.Exit(runWidget(os.Args[2:]))
	case "version", "-v", "--version":
		fmt.Println(versionString())
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
	noGen := fs.Bool("no-generators", false, "jangan jalankan generator")
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

	// Jalur yang dipakai sama persis dengan mode widget — engine, generator,
	// penyaringan, dan pemeringkatan yang sama — supaya hasil pemeriksaan di
	// sini tidak pernah berbeda dari yang muncul saat Tab ditekan.
	eng := engine.New(spec.NewRegistryDirs(dirs...))

	var src *generator.Source
	var dyn ui.Dynamic
	if !*noGen {
		src = newDynamic()
		dyn = src
	}

	pre, err := ui.Prepare(eng, ui.State{Line: *line, Cursor: *cursor}, dyn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uf:", err)
		return 1
	}

	var denied []string
	if src != nil {
		denied = src.Denied
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Candidates []engine.Candidate `json:"candidates"`
			Denied     []string           `json:"denied,omitempty"`
		}{pre.Candidates(), denied}); err != nil {
			fmt.Fprintln(os.Stderr, "uf:", err)
			return 1
		}
		return 0
	}

	for _, c := range pre.Candidates() {
		if c.Description == "" {
			fmt.Printf("%s\t%s\n", c.Name, c.Kind)
			continue
		}
		fmt.Printf("%s\t%s\t%s\n", c.Name, c.Kind, c.Description)
	}
	for _, d := range denied {
		fmt.Fprintln(os.Stderr, "uf: generator ditolak:", d)
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
	add(bundledSpecsDirs()...)

	if len(dirs) == 0 {
		return nil, fmt.Errorf("direktori spec tidak ditemukan; set UF_SPECS atau pakai --specs")
	}
	return dirs, nil
}

// versionString merangkai keterangan versi. Commit dan tanggal hanya muncul
// bila binary ini memang dibangun oleh proses rilis.
func versionString() string {
	s := "uf " + version
	if commit != "" {
		s += " (" + commit
		if date != "" {
			s += ", " + date
		}
		s += ")"
	}
	return s
}

// bundledSpecsDirs menyusun lokasi spec bawaan relatif terhadap binary.
func bundledSpecsDirs() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return bundledSpecsDirsFor(exe, filepath.EvalSymlinks)
}

// bundledSpecsDirsFor adalah isi bundledSpecsDirs yang bisa diuji.
//
// Symlink harus diselesaikan lebih dulu. Homebrew — baik formula maupun cask —
// memasang binary sebagai symlink di dalam bin, sementara spec tetap berada di
// direktori aslinya; tanpa langkah ini spec tidak akan pernah ditemukan.
// Kedua lokasi tetap dicoba, karena arsip biasa tidak memakai symlink sama
// sekali dan di sana exe sudah merupakan jalur sebenarnya.
func bundledSpecsDirsFor(exe string, eval func(string) (string, error)) []string {
	dirs := []string{}
	seen := map[string]bool{}

	addFor := func(path string) {
		base := filepath.Dir(path)
		for _, d := range []string{
			filepath.Join(base, "specs"),                      // arsip rilis, cask, scoop
			filepath.Join(base, "..", "share", "uf", "specs"), // deb, rpm, formula
		} {
			c := filepath.Clean(d)
			if !seen[c] {
				seen[c] = true
				dirs = append(dirs, c)
			}
		}
	}

	addFor(exe)
	if real, err := eval(exe); err == nil && real != exe {
		addFor(real)
	}
	return dirs
}
