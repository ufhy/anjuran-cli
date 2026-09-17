// Command anjuran adalah entry point CLI.
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

	"github.com/ufhy/anjuran-cli/internal/engine"
	"github.com/ufhy/anjuran-cli/internal/generator"
	"github.com/ufhy/anjuran-cli/internal/spec"
	"github.com/ufhy/anjuran-cli/internal/ui"
)

// version diisi saat build lewat -ldflags. Nilai "dev" berarti binary ini
// dibangun dari pohon kerja, bukan dari sebuah rilis.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

const usage = `anjuran - autocomplete lintas platform untuk shell

Penggunaan:
  anjuran init      <zsh|bash|fish|powershell>
  anjuran bootstrap [user@]host [--from <dir>] [--dry-run]
  anjuran version
  anjuran complete --line <baris> [--cursor N] [--json]
  anjuran widget   --line <baris> --cursor <N>

Opsi:
  --line    baris perintah yang sedang diketik
  --cursor  posisi kursor dalam byte (default: akhir baris)
  --json    keluarkan hasil mentah sebagai JSON
  --no-generators
            jangan jalankan generator; hanya kandidat dari berkas spec
  --specs   direktori spec, boleh beberapa dipisah titik dua
            (default: $ANJURAN_SPECS, ~/.config/anjuran/specs, ./specs, lalu bawaan)

init mencetak skrip integrasi shell. Pasang dengan menambahkan satu baris ke
berkas konfigurasi shell:

  zsh   ~/.zshrc                     eval "$(anjuran init zsh)"
  bash  ~/.bashrc                    eval "$(anjuran init bash)"
  fish  ~/.config/fish/config.fish   anjuran init fish | source

bootstrap memasang anjuran di host lain lewat SSH. Engine harus berjalan di sisi
remote, karena generator seperti "kubectl get pods" hanya menjawab benar di
tempat datanya berada. Perintah ini bukan pembungkus ssh: ia dijalankan sekali,
dengan sadar, lalu selesai.

widget adalah mode interaktif yang dipanggil integrasi shell; dropdown digambar
ke /dev/tty dan hasilnya dikembalikan lewat stdout.

Lingkungan:
  ANJURAN_SPECS      direktori spec
  ANJURAN_SIMPLE     bila diisi, matikan warna dan sorotan
  ANJURAN_KEY        tombol pemicu, dibaca oleh skrip init
  ANJURAN_AUTO       0 untuk mematikan dropdown yang muncul sendiri (bawaan: nyala)
  ANJURAN_LOG        rekam byte yang digambar ke terminal, untuk menyelidiki
                     laporan "layarnya aneh" di mesin lain
  ANJURAN_GHOST      bila diisi, tampilkan saran dari riwayat sebagai teks abu-abu
  ANJURAN_CACHE_DIR  lokasi cache generator dan ingatan pilihan

Generator menjalankan perintah sebagai efek samping mengetik, jadi
kebijakannya ketat secara bawaan dan diatur lewat lingkungan:

  ANJURAN_NO_GENERATORS        bila diisi, matikan seluruh generator
  ANJURAN_GENERATOR_ALLOW      biner tambahan yang boleh dijalankan, dipisah koma
  ANJURAN_GENERATOR_ALLOW_ROOT bila diisi, izinkan generator berjalan sebagai root
  ANJURAN_GENERATOR_TIMEOUT    batas waktu, misalnya 800ms

Secara bawaan anjuran hanya menjalankan perintah yang sedang kamu ketik sendiri,
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
	case "bootstrap":
		os.Exit(runBootstrap(os.Args[2:]))
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
		fmt.Fprintln(os.Stderr, "anjuran:", err)
		return 1
	}

	// Jalur yang dipakai sama persis dengan mode widget — engine, generator,
	// penyaringan, dan pemeringkatan yang sama — supaya hasil pemeriksaan di
	// sini tidak pernah berbeda dari yang muncul saat Tab ditekan.
	eng := engine.New(newRegistry(dirs, *specsDir)).InDir(generator.CurrentDir())

	var src *generator.Source
	var dyn ui.Dynamic
	if !*noGen {
		src = newDynamic()
		dyn = src
	}

	pre, err := ui.Prepare(eng, ui.State{Line: *line, Cursor: *cursor}, dyn, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "anjuran:", err)
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
			fmt.Fprintln(os.Stderr, "anjuran:", err)
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
		fmt.Fprintln(os.Stderr, "anjuran: generator ditolak:", d)
	}
	return 0
}

// resolveSpecsDirs menyusun urutan pencarian spec.
//
// Urutannya: yang disebut pengguna lebih dulu, lalu spec bawaan. Dengan begitu
// sebuah spec buatan sendiri bisa menimpa spec bawaan tanpa menyunting
// direktori yang dihasilkan mesin.
func resolveSpecsDirs(flagValue string) ([]string, error) {
	dirs, _ := resolveSpecsDirsTrust(flagValue)
	if len(dirs) == 0 {
		return nil, fmt.Errorf("direktori spec tidak ditemukan; setel ANJURAN_SPECS atau pakai --specs")
	}
	return dirs, nil
}

// resolveSpecsDirsTrust mengembalikan urutan pencarian beserta direktori mana
// yang isinya ditulis tangan dan ditinjau.
func resolveSpecsDirsTrust(flagValue string) (dirs, trusted []string) {
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

	// Urutannya adalah urutan LAPISAN, dari yang paling menimpa ke yang paling
	// dasar. Spec dengan nama sama digabung, bukan saling menggantikan.
	//
	// Tambalan harus berada di ATAS spec bawaan. Kalau terbalik, generator
	// bawaan yang justru ingin diperbaiki akan menimpa perbaikannya — dan
	// gejalanya diam-diam: fiturnya tampak jalan, isinya saja yang salah.
	// Spec milik pengguna dan tambalan bawaan ditulis tangan satu per satu,
	// jadi generatornya boleh memanggil interpreter. Korpus hasil transpile
	// tidak pernah mendapat kelonggaran itu.
	mark := len(dirs)
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".config", "anjuran", "specs"))
	}
	add("extra")
	add(bundledDirs("extra")...)
	trusted = append(trusted, dirs[mark:]...)

	add(filepath.SplitList(flagValue)...)
	add(filepath.SplitList(os.Getenv("ANJURAN_SPECS"))...)
	add("specs")
	add(bundledDirs("specs")...)

	return dirs, trusted
}

// versionString merangkai keterangan versi. Commit dan tanggal hanya muncul
// bila binary ini memang dibangun oleh proses rilis.
func versionString() string {
	s := "anjuran " + version
	if commit != "" {
		s += " (" + commit
		if date != "" {
			s += ", " + date
		}
		s += ")"
	}
	return s
}

// newRegistry menyusun registry lengkap dengan penandaan direktori tepercaya.
func newRegistry(dirs []string, flagValue string) *spec.Registry {
	r := spec.NewRegistryDirs(dirs...)
	_, trusted := resolveSpecsDirsTrust(flagValue)
	r.Trust(trusted...)
	return r
}

// bundledDirs menyusun lokasi sebuah direktori bawaan relatif terhadap binary.
func bundledDirs(name string) []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return bundledDirsFor(exe, name, filepath.EvalSymlinks)
}

// bundledSpecsDirsFor adalah isi bundledSpecsDirs yang bisa diuji.
//
// Symlink harus diselesaikan lebih dulu. Homebrew — baik formula maupun cask —
// memasang binary sebagai symlink di dalam bin, sementara spec tetap berada di
// direktori aslinya; tanpa langkah ini spec tidak akan pernah ditemukan.
// Kedua lokasi tetap dicoba, karena arsip biasa tidak memakai symlink sama
// sekali dan di sana exe sudah merupakan jalur sebenarnya.
func bundledDirsFor(exe, name string, eval func(string) (string, error)) []string {
	dirs := []string{}
	seen := map[string]bool{}

	addFor := func(path string) {
		base := filepath.Dir(path)
		for _, d := range []string{
			filepath.Join(base, name),                           // arsip rilis, cask, scoop
			filepath.Join(base, "..", "share", "anjuran", name), // deb, rpm, formula
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
